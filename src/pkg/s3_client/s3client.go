package s3_client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type S3PageClient struct {
	client       *s3.Client
	page         *config.Page
	tracer       trace.Tracer
	repository   string
	s3Options    s3.Options
	s3Endpoint   string
	s3BucketName string
}

func NewS3PageClient(page *config.Page, options ...S3ClientOption) *S3PageClient {
	client := &S3PageClient{
		tracer:       otel.Tracer("StaticPages-S3-Client"),
		page:         page,
		repository:   "",
		s3Options:    s3.Options{},
		s3Endpoint:   "",
		s3BucketName: "",
		client:       nil,
	}

	// By default, we initialize the S3 config with the bucket config of the page
	WithBucketConf(&page.Bucket)(client)

	// By default, we initialize the S3 config with the repository we can extract from the config of the page
	WithRepository(page.Git.Repository)(client)

	for _, option := range options {
		option(client)
	}

	client.client = s3.New(client.s3Options)

	return client
}

type S3ClientOption func(*S3PageClient)

func WithRepository(repository string) S3ClientOption {
	return func(c *S3PageClient) {
		c.repository = repository
	}
}

func WithBucketConf(bucketConf *config.BucketConfig) S3ClientOption {
	return func(c *S3PageClient) {
		c.s3Endpoint = bucketConf.URL.String()
		c.s3BucketName = bucketConf.Name.String()
		c.s3Options = s3.Options{
			BaseEndpoint:  &c.s3Endpoint,
			Region:        bucketConf.Region.String(), // required even if arbitrary
			UsePathStyle:  true,                       // required for Backblaze B2 compatibility
			UseAccelerate: false,                      // maybe required for BackBlaze B2 compatibility? TODO: test
			Logger:        otelzap.L(),
			Credentials: aws.NewCredentialsCache(
				credentials.NewStaticCredentialsProvider(
					bucketConf.ApplicationID.String(),
					bucketConf.Secret.String(),
					"",
				)),
		}
	}
}

func (c *S3PageClient) UploadFolder(ctx context.Context, source, target string) humane.Error {
	ctx, span := c.tracer.Start(ctx, "s3Client.uploadArtifactsToS3")
	defer span.End()

	otelzap.L().Ctx(ctx).Debug("start uploading artifacts to s3",
		zap.String("bucket", c.s3BucketName),
		zap.String("source_folder", source),
		zap.String("target_folder", target),
	)

	// Walk through directory recursively to collect all files
	files := make([]string, 0)
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return humane.Wrap(err, "failed to walk upload directory")
	}

	// Use a worker pool pattern
	concurrency := 10 // Adjust later...
	semaphore := make(chan struct{}, concurrency)
	errChan := make(chan error, len(files))
	var wg sync.WaitGroup

	// Upload files
	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := c.uploadFileInFolder(ctx, source, path, target)
			if err != nil {
				errChan <- err
			}
		}(filePath)
	}

	// Wait for all uploads to complete
	wg.Wait()
	close(errChan)

	// Handle errors (if any)
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

func (c *S3PageClient) uploadFileInFolder(ctx context.Context, source, file, target string) humane.Error {
	// Open file for reading
	f, err := os.Open(file)
	if err != nil {
		return humane.Wrap(err, "failed to open file for S3 upload")
	}

	defer func() { _ = f.Close() }()

	relPath, err := filepath.Rel(source, file)
	if err != nil {
		return humane.Wrap(err, "failed to determine relative path for upload")
	}

	// Construct target path
	s3Key := filepath.Join(target, relPath)
	// Convert Windows path separators to forward slashes
	s3Key = filepath.ToSlash(s3Key)

	// Get file size for Content-Length
	fileInfo, err := f.Stat()
	if err != nil {
		return humane.Wrap(err, "failed to get file stats for S3 upload")
	}

	// Upload the file to S3
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.s3BucketName),
		Key:           aws.String(s3Key),
		Body:          f,
		ContentLength: aws.Int64(fileInfo.Size()),
		ContentType:   aws.String(determineContentType(file)),
	})

	if err != nil {
		return humane.Wrap(err, fmt.Sprintf("failed to upload file %s to S3", file))
	}

	return nil
}

// determineContentType returns the appropriate Content-Type for a file
func determineContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".ico":
		return "image/x-icon"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".yaml", ".yml":
		return "application/x-yaml"
	default:
		return "application/octet-stream"
	}
}

func (c *S3PageClient) UploadPageIndex(ctx context.Context, metadata PageIndex) humane.Error {
	ctx, span := c.tracer.Start(ctx, "s3Client.UploadPageIndex")
	defer span.End()

	data, err := yaml.Marshal(metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return humane.Wrap(err, "failed to marshal metadata for S3 upload")
	}

	s3Key := filepath.ToSlash(path.Join(c.repository, "index.yaml"))

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.s3BucketName),
		Key:           aws.String(s3Key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String("application/x-yaml"),
	})

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return humane.Wrap(err, "failed to upload metadata to S3")
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (c *S3PageClient) DownloadPageIndex(ctx context.Context) (PageIndex, humane.Error) {
	ctx, span := c.tracer.Start(ctx, "s3Client.DownloadPageIndex")
	defer span.End()

	// Convert Windows path separators to forward slashes
	s3Key := filepath.ToSlash(path.Join(c.repository, "index.yaml"))

	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.s3BucketName),
		Key:    aws.String(s3Key),
	})

	metadata := make(PageIndex)

	if err != nil {
		if isNotFound(err) {
			return metadata, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, humane.Wrap(err, "failed to download metadata from S3")
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, humane.Wrap(err, "failed to read metadata from S3 response")
	}

	err = yaml.Unmarshal(data, &metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, humane.Wrap(err, "failed to unmarshal metadata from S3")
	}

	span.SetStatus(codes.Ok, "")
	return metadata, nil
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey"
}
