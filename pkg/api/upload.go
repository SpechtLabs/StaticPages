package api

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
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
		otelzap.L().Ctx(ctx).Warn("request context canceled", zap.Error(ctx.Err()))
		ct.AbortWithStatus(StatusRequestContextCanceled)
		return
	}

	// Get Repository Metadata claims (and verify authentication)
	metadata, herr := r.extractAndVerifyAuth(ctx, ct.GetHeader("Authorization"))
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to extract or verify auth")
		ct.JSON(http.StatusForbidden, errorBody("invalid authorization header"))
		return
	}

	// Get the Page Configuration
	page, herr := r.extractPagesConfig(ctx, metadata.Repository())
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("repository not authorized", zap.String("repository", metadata.Repository()))
		ct.JSON(http.StatusForbidden, errorBody("repository not authorized"))
		return
	}

	// Parse uploaded files
	form, err := ct.MultipartForm()
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("failed to parse multipart form", zap.String("commit_sha", metadata.SHA()))
		ct.JSON(http.StatusBadRequest, errorBody("invalid multipart form"))
		return
	}

	uploadPath, fileCount, size, herr := r.saveArtifactsToTemp(ctx, form)
	if uploadPath != "" {
		defer func() { _ = os.RemoveAll(uploadPath) }()
	}
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to save artifacts to temp folder", zap.String("commit_sha", metadata.SHA()))
		ct.JSON(http.StatusInternalServerError, errorBody("failed to save artifacts"))
		return
	}

	span.SetAttributes(
		attribute.Int("file_count", fileCount),
		attribute.Int64("file_size", size),
	)

	s3client := s3_client.NewS3PageClient(page)
	herr = s3client.UploadFolder(ctx, uploadPath, filepath.Join(metadata.Repository(), metadata.SHA()))
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to upload artifacts to storage backend")
		ct.JSON(http.StatusInternalServerError, errorBody("failed to save artifacts"))
		return
	}

	if herr := r.addToPageIndex(ctx, s3client, metadata); herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to update page metadata", zap.String("domain", page.Domain.String()))
		ct.JSON(http.StatusInternalServerError, errorBody("failed to update page metadata"))
		return
	}

	// Invalidate the cache immediately (useful if we're running "all in one")
	s3_client.InvalidatePageMetadata(page)

	span.SetStatus(codes.Ok, "")
	ct.JSON(http.StatusOK, gin.H{
		"status":      "upload successful",
		"file_count":  fileCount,
		"url":         fmt.Sprintf("https://%s", page.Domain.String()),
		"preview_url": getPreviewUrls(page, metadata),
	})
}

// addToPageIndex records the uploaded commit in the page's index of published
// commits.
func (r *RestApi) addToPageIndex(ctx context.Context, s3client *s3_client.S3PageClient, metadata *s3_client.PageIndexData) humane.Error {
	ctx, span := r.tracer.Start(ctx, "restApi.addToPageIndex")
	defer span.End()

	pageIndex, herr := s3client.DownloadPageIndex(ctx)
	if herr != nil {
		return herr
	}

	pageIndex[metadata.SHA()] = metadata
	span.SetAttributes(attribute.Int("index_size", len(pageIndex)))

	return s3client.UploadPageIndex(ctx, pageIndex)
}

// saveArtifactsToTemp saves the form's files to a new temporary directory and
// returns its path, which the caller removes once it's done with it.
func (r *RestApi) saveArtifactsToTemp(ctx context.Context, form *multipart.Form) (string, int, int64, humane.Error) {
	ctx, span := r.tracer.Start(ctx, "restApi.saveArtifactsToTemp")
	defer span.End()

	uploadPath, err := os.MkdirTemp("", "staticpages-upload-*")
	if err != nil {
		return "", 0, 0, humane.Wrap(err, "failed to create upload cache directory", "Make sure the upload cache directory is writable and try again.")
	}
	span.SetAttributes(attribute.String("upload_path", uploadPath))

	otelzap.L().Ctx(ctx).Debug("start saving artifacts", zap.String("path", uploadPath))

	root, err := os.OpenRoot(uploadPath)
	if err != nil {
		return uploadPath, 0, 0, humane.Wrap(err, "failed to open upload cache directory", "Make sure the upload cache directory is writable and try again.")
	}
	defer func() { _ = root.Close() }()

	var (
		wg    sync.WaitGroup
		saved savedFiles
	)

	for key, files := range form.File {
		relPath, ok := extractRelativePath(key)
		if !ok {
			continue
		}

		for _, file := range files {
			wg.Go(func() {
				saved.record(file.Size, saveUploadedFile(root, relPath, file))
			})
		}
	}

	wg.Wait()
	return uploadPath, saved.count, saved.size, saved.err
}

// savedFiles tallies the files saveArtifactsToTemp's workers saved, and keeps
// the first error one of them ran into.
type savedFiles struct {
	err   humane.Error
	size  int64
	count int
	mu    sync.Mutex
}

func (s *savedFiles) record(size int64, herr humane.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if herr != nil {
		if s.err == nil {
			s.err = herr
		}
		return
	}

	s.count++
	s.size += size
}

// saveUploadedFile writes the uploaded file to relPath under root. The path
// comes from the form key, and root refuses one that would leave it, so a key
// like files[../../etc/passwd] can't write outside the upload directory.
func saveUploadedFile(root *os.Root, relPath string, file *multipart.FileHeader) (herr humane.Error) {
	if err := root.MkdirAll(filepath.Dir(relPath), 0o750); err != nil {
		return humane.Wrap(err, "failed to create upload cache directory",
			"Make sure every uploaded path stays inside the site directory, and that the upload cache directory is writable.")
	}

	src, err := file.Open()
	if err != nil {
		return humane.Wrap(err, "failed to read uploaded file", "Retry the upload; the request may have been cut short.")
	}
	defer func() { _ = src.Close() }()

	dst, err := root.Create(relPath)
	if err != nil {
		return humane.Wrap(err, "failed to save file",
			"Make sure every uploaded path stays inside the site directory, and that the upload cache directory is writable.")
	}
	defer func() {
		if err := dst.Close(); err != nil && herr == nil {
			herr = humane.Wrap(err, "failed to save file", "Make sure the upload cache directory is writable and try again.")
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return humane.Wrap(err, "failed to save file", "Make sure the upload cache directory is writable and try again.")
	}

	return nil
}

func getPreviewUrls(page *config.Page, metadata *s3_client.PageIndexData) []string {
	previewUrls := make([]string, 0)
	if !page.Preview.Enabled {
		return previewUrls
	}

	if page.Preview.Branch && page.Git.MainBranch != metadata.Branch {
		previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.Branch, page.Domain.String()))
	}

	if page.Preview.CommitSha {
		previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.SHA(), page.Domain.String()))
	}

	if page.Preview.Environments {
		previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.Environment, page.Domain.String()))
	}

	return previewUrls
}

// errorBody is the JSON body of an error response.
func errorBody(message string) gin.H {
	return gin.H{"error": message}
}
