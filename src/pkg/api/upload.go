package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type gitHubActionClaims struct {
	Repository  string `json:"repository"`
	Sha         string `json:"sha"`
	Environment string `json:"environment"`
	Ref         string `json:"ref"`
	Actor       string `json:"actor"`
	Event       string `json:"event"`
	Workflow    string `json:"workflow"`
	Job         string `json:"job"`
	RunID       string `json:"run_id"`
	RunNumber   string `json:"run_number"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
}

// UploadHandler handles file upload requests, processes uploaded content, and returns a corresponding HTTP response.
func (r *RestApi) UploadHandler(ct *gin.Context) {
	ctx, span := r.tracer.Start(ct.Request.Context(), "restApi.UploadHandler")
	defer span.End()

	if ctx.Err() != nil {
		otelzap.L().Sugar().Ctx(ctx).Warnw("request context canceled")
		ct.AbortWithStatus(StatusRequestContextCanceled)
		return
	}

	// Get OIDC claims (and verify authentication)
	claims, herr := r.extractAndVerifyAuth(ctx, ct.GetHeader("Authorization"))
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to extract or verify auth", zap.Error(herr), zap.Strings("advice", herr.Advice()), zap.String("cause", herr.Cause().Error()))
		ct.JSON(http.StatusForbidden, gin.H{"error": "invalid authorization header"})
		return
	}

	// Get the Page Configuration
	page := r.extractPagesConfig(claims.Repository)
	if page == nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("repository not authorized", zap.String("repository", claims.Repository))
		ct.JSON(http.StatusForbidden, gin.H{"error": "repository not authorized"})
		return
	}

	// Parse uploaded files
	form, err := ct.MultipartForm()
	if err != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to parse multipart form", zap.Error(err))
		ct.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	span.SetAttributes(attribute.Int("file_count", len(form.File)))

	uploadPath, fileCount, herr := r.saveArtifactsToTemp(ctx, ct, claims.Sha, form)
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to save artifacts", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	herr = r.uploadArtifactsToS3(ctx, uploadPath, claims.Repository, claims.Sha, page.Bucket)
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to upload artifacts to storage backend", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	span.SetStatus(codes.Ok, "")
	ct.JSON(http.StatusOK, gin.H{"status": "upload successful", "file_count": fileCount})
}

func (r *RestApi) extractAndVerifyAuth(ctx context.Context, authHeader string) (*gitHubActionClaims, humane.Error) {
	ctx, span := r.tracer.Start(ctx, "restApi.extractAndVerifyAuth")
	defer span.End()

	// Create OIDC provider
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, humane.Wrap(err, "failed to initialize OIDC provider", "Make sure the OIDC issuer is correctly configured and try again.")
	}

	// Extract and verify Authorization header
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, humane.New("missing or invalid Authorization header", "Make sure the Authorization header is correctly formatted and try again.")
	}
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")

	verifier := provider.VerifierContext(ctx, &oidc.Config{SkipClientIDCheck: true})
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, humane.Wrap(err, "failed to verify OIDC token", "Make sure the Authorization header is correctly formatted and try again.")
	}

	// Extract GitHub Action OIDC Claims
	claims := &gitHubActionClaims{}
	if err := idToken.Claims(claims); err != nil {
		return nil, humane.Wrap(err, "failed to extract claims from token", "Make sure the Authorization header is correctly formatted and try again.")
	}

	return claims, nil
}

func (r *RestApi) saveArtifactsToTemp(ctx context.Context, ct *gin.Context, commitSha string, form *multipart.Form) (string, int, humane.Error) {
	ctx, span := r.tracer.Start(ctx, "restApi.saveArtifactsToTemp")
	defer span.End()

	tempDir := os.TempDir()
	uploadPath := filepath.Join(tempDir, commitSha)

	otelzap.L().Sugar().Ctx(ctx).Debugw("start saving artifacts", zap.String("path", uploadPath))

	fileCount := 0
	for key, files := range form.File {
		relPath, ok := extractRelativePath(key)
		if !ok {
			continue
		}

		for _, file := range files {
			dst := filepath.Join(uploadPath, relPath)

			if err := os.MkdirAll(filepath.Dir(dst), 0o775); err != nil {
				return uploadPath, 0, humane.Wrap(err, "failed to create upload cache directory", "Make sure the upload cache directory is writable and try again.")
			}

			if err := ct.SaveUploadedFile(file, dst); err != nil {
				return uploadPath, 0, humane.Wrap(err, "failed to save file", "Make sure the upload cache directory is writable and try again.")
			}

			fileCount++
		}
	}

	return uploadPath, fileCount, nil
}

func (r *RestApi) uploadArtifactsToS3(ctx context.Context, sourceFolder, repo, commitSha string, bucketConfig config.BucketConfig) humane.Error {
	ctx, span := r.tracer.Start(ctx, "restApi.uploadArtifactsToS3")
	defer span.End()

	bucket := bucketConfig.Name.String()

	otelzap.L().Sugar().Ctx(ctx).Debugw("start uploading artifacts to s3", zap.String("bucket", bucket), zap.String("source_folder", sourceFolder), zap.String("repo", repo), zap.String("commit_sha", commitSha))

	// Walk through directory recursively
	files := []string{}
	err := filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return humane.Wrap(err, "failed to walk upload directory")
	}

	s3Client := createS3Client(bucketConfig)

	// Use a worker pool pattern
	concurrency := 10 // Adjust later...
	semaphore := make(chan struct{}, concurrency)
	errChan := make(chan error, len(files))

	var wg sync.WaitGroup
	for _, filePath := range files {
		wg.Add(1)
		errChan = r.uploadFileToS3(ctx, sourceFolder, repo, commitSha, &wg, semaphore, errChan, s3Client, bucket, filePath)
	}

	// Wait for all uploads to complete
	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return humane.Wrap(err, "failed to upload artifacts to S3")
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *RestApi) uploadFileToS3(ctx context.Context, sourceFolder string, repo string, commitSha string, wg *sync.WaitGroup, semaphore chan struct{}, errChan chan error, s3Client *s3.Client, bucket string, filePath string) chan error {
	go func(file string) {
		defer wg.Done()

		// Acquire semaphore
		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		// Upload file
		relPath, _ := filepath.Rel(sourceFolder, file)
		key := filepath.Join(repo, commitSha, relPath)

		// Open file for reading
		f, err := os.Open(file)
		if err != nil {
			errChan <- err
			return
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				errChan <- err
			}
		}(f)

		// Upload to S3
		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   f,
		})

		if err != nil {
			errChan <- err
		}
	}(filePath)
	return errChan
}

func createS3Client(bucketConfig config.BucketConfig) *s3.Client {
	// Extract credentials from EnvValue (assuming page.Bucket.Secret/ID is already resolved)
	accessKey := bucketConfig.ApplicationID.String()
	secretKey := bucketConfig.Secret.String()
	region := bucketConfig.Region.String()
	endpoint := bucketConfig.URL.String()

	s3Options := s3.Options{
		BaseEndpoint:  &endpoint,
		Region:        region, // required even if arbitrary
		UsePathStyle:  true,   // required for Backblaze B2 compatibility
		Logger:        otelzap.L(),
		Credentials:   aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		UseAccelerate: false,
	}

	return s3.New(s3Options)
}
