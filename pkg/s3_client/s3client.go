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
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// S3PageClient reads and writes one page's objects and index in its bucket.
type S3PageClient struct {
	tracer       trace.Tracer
	client       *s3.Client
	page         *config.Page
	repository   string
	s3Endpoint   string
	s3BucketName string
	s3Options    s3.Options
}

// NewS3PageClient returns a client for the page's bucket and repository; the
// options override either.
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

	// Instrument the AWS SDK so each S3 call to the storage backend (Backblaze)
	// is emitted as a child span under the calling operation. The SDK traces
	// itself through smithy-go; otelaws, which did this before, is deprecated.
	client.s3Options.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())

	client.client = s3.New(client.s3Options)

	return client
}

// S3ClientOption configures an S3PageClient.
type S3ClientOption func(*S3PageClient)

// WithRepository sets the repository whose objects the client reads and writes.
func WithRepository(repository string) S3ClientOption {
	return func(c *S3PageClient) {
		c.repository = repository
	}
}

// WithBucketConf points the client at the bucket.
func WithBucketConf(bucketConf *config.BucketConfig) S3ClientOption {
	return func(c *S3PageClient) {
		c.s3Endpoint = bucketConf.URL.String()
		c.s3BucketName = bucketConf.Name.String()
		c.s3Options = s3.Options{
			BaseEndpoint:  &c.s3Endpoint,
			Region:        bucketConf.Region.String(), // required even if arbitrary
			UsePathStyle:  true,                       // required for Backblaze B2 compatibility
			UseAccelerate: false,                      // TODO(cedi): test whether Backblaze B2 needs this
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

// UploadFolder uploads every file under source to the target prefix in the
// bucket, ten at a time.
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
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return humane.Wrap(err, "failed to walk upload directory",
			"Make sure the uploaded artifacts were extracted completely and are readable.")
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
			return humane.Wrap(err, "failed to upload artifacts to S3",
				"Make sure the bucket exists and the configured credentials may write to it.")
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (c *S3PageClient) uploadFileInFolder(ctx context.Context, source, file, target string) humane.Error {
	relPath, err := filepath.Rel(source, file)
	if err != nil {
		return humane.Wrap(err, "failed to determine relative path for upload",
			"This is a bug in staticpages; please report it.")
	}

	// Open file for reading, refusing anything that resolves outside source
	f, err := os.OpenInRoot(source, relPath)
	if err != nil {
		return humane.Wrap(err, "failed to open file for S3 upload",
			"Make sure the uploaded artifacts are readable.")
	}

	defer func() { _ = f.Close() }()

	// Construct target path
	s3Key := filepath.Join(target, relPath)
	// Convert Windows path separators to forward slashes
	s3Key = filepath.ToSlash(s3Key)

	// Get file size for Content-Length
	fileInfo, err := f.Stat()
	if err != nil {
		return humane.Wrap(err, "failed to get file stats for S3 upload",
			"Make sure the uploaded artifacts are readable.")
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
		return humane.Wrap(err, fmt.Sprintf("failed to upload file %s to S3", file),
			"Make sure the bucket exists and the configured credentials may write to it.")
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

// UploadPageIndex writes the repository's index of published commits.
func (c *S3PageClient) UploadPageIndex(ctx context.Context, metadata PageIndex) humane.Error {
	ctx, span := c.tracer.Start(ctx, "s3Client.UploadPageIndex")
	defer span.End()

	data, err := yaml.Marshal(metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return humane.Wrap(err, "failed to marshal metadata for S3 upload",
			"This is a bug in staticpages; please report it.")
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
		return humane.Wrap(err, "failed to upload metadata to S3",
			"Make sure the bucket exists and the configured credentials may write to it.")
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// DownloadPageIndex reads the repository's index of published commits; a
// repository that has none yet gets an empty index.
func (c *S3PageClient) DownloadPageIndex(ctx context.Context) (PageIndex, humane.Error) {
	ctx, span := c.tracer.Start(ctx, "s3Client.DownloadPageIndex")
	defer span.End()

	// Convert Windows path separators to forward slashes
	s3Key := filepath.ToSlash(path.Join(c.repository, "index.yaml"))

	span.SetAttributes(
		attribute.String("s3.endpoint", c.s3Endpoint),
		attribute.String("s3.bucket", c.s3BucketName),
		attribute.String("s3.key", s3Key),
	)

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
		return nil, humane.Wrap(err, "failed to download metadata from S3",
			"Make sure the bucket exists and the configured credentials may read it.")
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, humane.Wrap(err, "failed to read metadata from S3 response",
			"Check the connection to the storage backend and try again.")
	}

	err = yaml.Unmarshal(data, &metadata)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, humane.Wrap(err, "failed to unmarshal metadata from S3",
			"Make sure index.yaml in the bucket is valid YAML written by staticpages.")
	}

	span.SetAttributes(attribute.Int("page_index.entries", len(metadata)))
	span.SetStatus(codes.Ok, "")
	return metadata, nil
}

func isNotFound(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey"
}
