package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

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
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to save artifacts to temp folder", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	s3client, herr := s3_client.GetS3Client(page.Domain.String())
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to get s3 client", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	herr = s3client.UploadFolder(ctx, uploadPath, filepath.Join(claims.Repository, claims.Sha))
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to upload artifacts to storage backend", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	metadata, err := s3client.DownloadMetadata(ctx)
	if err != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("unable to get metadata", zap.Error(err), zap.String("domain", page.Domain.String()))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update page metadata"})
		return
	}

	// Update our Page Metadata
	metadata[claims.Sha] = &s3_client.PageCommitMetadata{
		Environment: claims.Environment,
		Branch:      claims.Branch(),
		Date:        time.Now(),
	}

	herr = s3client.UploadMetadata(ctx, metadata)
	if herr != nil {
		otelzap.L().Sugar().Ctx(ctx).Errorw("failed to update page metadata", zap.Error(herr))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update page metadata"})
		return
	}

	span.SetStatus(codes.Ok, "")
	ct.JSON(http.StatusOK, gin.H{"status": "upload successful", "file_count": fileCount})
}

func (r *RestApi) extractAndVerifyAuth(ctx context.Context, authHeader string) (*s3_client.GitHubClaims, humane.Error) {
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
	claims := &s3_client.GitHubClaims{}
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
