package s3_client

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3PageClient(t *testing.T) {
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("test"))
	server := httptest.NewServer(gofakes3.New(backend, gofakes3.WithHostBucket(false)).Server())
	t.Cleanup(server.Close)

	page := func(domain, bucket, repository string) *config.Page {
		return &config.Page{
			Domain: config.FromString(domain),
			Bucket: config.BucketConfig{
				URL: config.EnvValue(server.URL), Name: config.EnvValue(bucket),
				ApplicationID: "test", Secret: "test", Region: "test",
			},
			Git: config.GitConfig{Repository: repository},
		}
	}
	ctx := context.Background()

	t.Run("upload a folder and index it", func(t *testing.T) {
		site := page("site.example", "test", "spechtlabs/site")
		source := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(source, "css"), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(source, "index.html"), []byte("<h1>hi</h1>"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(source, "css", "site.css"), []byte("h1{}"), 0o600))

		client := NewS3PageClient(site)
		require.NoError(t, client.UploadFolder(ctx, source, "spechtlabs/site/abc123"))

		obj, err := backend.HeadObject("test", "spechtlabs/site/abc123/css/site.css")
		require.NoError(t, err)
		assert.Equal(t, "text/css", obj.Metadata["Content-Type"])

		index, herr := client.DownloadPageIndex(ctx)
		require.NoError(t, herr)
		assert.Empty(t, index, "a repository without an index gets an empty one")

		index["abc123"] = NewPageCommitMetadata("spechtlabs/site", "abc123", "main", "", time.Now())
		require.NoError(t, client.UploadPageIndex(ctx, index))

		metadata, herr := GetPageMetadata(ctx, site)
		require.NoError(t, herr)
		assert.Contains(t, metadata, "abc123")
		InvalidatePageMetadata(site)
	})

	t.Run("missing source folder", func(t *testing.T) {
		client := NewS3PageClient(page("site.example", "test", "spechtlabs/site"))
		herr := client.UploadFolder(ctx, filepath.Join(t.TempDir(), "missing"), "x")
		require.Error(t, herr)
		assert.Contains(t, herr.Error(), "failed to walk upload directory")
	})

	t.Run("missing bucket", func(t *testing.T) {
		missing := page("missing.example", "missing", "spechtlabs/site")
		client := NewS3PageClient(missing)

		source := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(source, "index.html"), []byte("x"), 0o600))
		assert.ErrorContains(t, client.UploadFolder(ctx, source, "x"), "failed to upload artifacts to S3")
		assert.ErrorContains(t, client.UploadPageIndex(ctx, PageIndex{}), "failed to upload metadata to S3")

		_, herr := GetPageMetadata(ctx, missing)
		assert.ErrorContains(t, herr, "unable to get page metadata")
	})

	t.Run("corrupt index", func(t *testing.T) {
		_, err := backend.PutObject("test", "spechtlabs/corrupt/index.yaml", nil, bytes.NewReader([]byte("{not yaml")), 9, nil)
		require.NoError(t, err)

		_, herr := NewS3PageClient(page("corrupt.example", "test", "spechtlabs/corrupt")).DownloadPageIndex(ctx)
		assert.ErrorContains(t, herr, "failed to unmarshal metadata from S3")
	})
}

func TestDetermineContentType(t *testing.T) {
	tests := map[string]string{
		"index.html": "text/html", "page.htm": "text/html", "site.css": "text/css",
		"app.js": "application/javascript", "data.json": "application/json",
		"logo.png": "image/png", "photo.jpg": "image/jpeg", "photo.jpeg": "image/jpeg",
		"anim.gif": "image/gif", "logo.svg": "image/svg+xml", "doc.pdf": "application/pdf",
		"feed.xml": "application/xml", "bundle.zip": "application/zip", "favicon.ico": "image/x-icon",
		"robots.txt": "text/plain", "README.md": "text/markdown", "config.yaml": "application/x-yaml",
		"config.yml": "application/x-yaml", "font.woff2": "application/octet-stream",
	}

	for file, want := range tests {
		t.Run(file, func(t *testing.T) {
			assert.Equal(t, want, determineContentType(file))
		})
	}
}
