package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

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
		otelzap.L().Ctx(ctx).Warn("request context canceled")
		ct.AbortWithStatus(StatusRequestContextCanceled)
		return
	}

	// Get Repository Metadata claims (and verify authentication)
	metadata, herr := r.extractAndVerifyAuth(ctx, ct.GetHeader("Authorization"))
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to extract or verify auth")
		ct.JSON(http.StatusForbidden, gin.H{"error": "invalid authorization header"})
		return
	}

	// Get the Page Configuration
	page, herr := r.extractPagesConfig(ctx, metadata.Repository())
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("repository not authorized", zap.String("repository", metadata.Repository()))
		ct.JSON(http.StatusForbidden, gin.H{"error": "repository not authorized"})
		return
	}

	// Parse uploaded files
	uploadPath, fileCount, size, herr := r.saveArtifactsToTemp(ctx, ct, metadata.SHA())
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to save artifacts to temp folder", zap.String("commit_sha", metadata.SHA()))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
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
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save artifacts"})
		return
	}

	pageIndex, err := s3client.DownloadPageIndex(ctx)
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("unable to get metadata", zap.String("domain", page.Domain.String()))
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update page metadata"})
		return
	}

	// Update our Page Metadata
	pageIndex[metadata.SHA()] = metadata

	span.SetAttributes(
		attribute.Int("index_size", len(pageIndex)),
	)

	herr = s3client.UploadPageIndex(ctx, pageIndex)
	if herr != nil {
		otelzap.L().WithError(herr).Ctx(ctx).Error("failed to update page metadata")
		ct.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update page metadata"})
		return
	}

	// Invalidate the cache immediately (useful if we're running "all in one")
	s3_client.InvalidatePageMetadata(page)

	// Get the Preview URL(s)
	domain := page.Domain.String()
	previewUrls := make([]string, 0)
	if page.Preview.Enabled {
		if page.Preview.Branch {
			previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.Branch, domain))
		}

		if page.Preview.CommitSha {
			previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.SHA(), domain))
		}

		if page.Preview.Environments {
			previewUrls = append(previewUrls, fmt.Sprintf("https://%s.%s", metadata.Environment, domain))
		}
	}

	span.SetStatus(codes.Ok, "")
	ct.JSON(http.StatusOK, gin.H{
		"status":      "upload successful",
		"file_count":  fileCount,
		"url":         domain,
		"preview_url": previewUrls,
	})
}

func (r *RestApi) saveArtifactsToTemp(ctx context.Context, ct *gin.Context, commitSha string) (string, int, int64, humane.Error) {
	ctx, span := r.tracer.Start(ctx, "restApi.saveArtifactsToTemp")
	defer span.End()

	tempDir := os.TempDir()
	uploadPath := filepath.Join(tempDir, commitSha)

	otelzap.L().Ctx(ctx).Debug("start saving artifacts", zap.String("path", uploadPath))

	form, err := ct.MultipartForm()
	if err != nil {
		otelzap.L().WithError(err).Ctx(ctx).Error("failed to parse multipart form")
		ct.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return uploadPath, 0, 0, humane.Wrap(err, "failed to parse multipart form", "Make sure the request is correctly formatted and try again.")
	}

	var (
		wg        sync.WaitGroup
		errOnce   sync.Once
		errResult humane.Error
		countMu   sync.Mutex
		fileCount int
		size      int64
	)

	for key, files := range form.File {
		relPath, ok := extractRelativePath(key)
		if !ok {
			continue
		}

		for _, file := range files {
			wg.Add(1)
			file := file // capture loop var

			go func(relPath string) {
				defer wg.Done()

				dst := filepath.Join(uploadPath, relPath)

				if err := os.MkdirAll(filepath.Dir(dst), 0o775); err != nil {
					errOnce.Do(func() {
						errResult = humane.Wrap(err, "failed to create upload cache directory", "Make sure the upload cache directory is writable and try again.")
					})
					return
				}

				if err := ct.SaveUploadedFile(file, dst); err != nil {
					errOnce.Do(func() {
						errResult = humane.Wrap(err, "failed to save file", "Make sure the upload cache directory is writable and try again.")
					})
					return
				}

				countMu.Lock()
				fileCount++
				size += file.Size
				countMu.Unlock()
			}(relPath)
		}
	}

	wg.Wait()
	return uploadPath, fileCount, size, errResult
}
