package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUploadHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	issuer := newFakeIssuer(t)
	backend := s3mem.New()
	require.NoError(t, backend.CreateBucket("test"))
	bucket := httptest.NewServer(gofakes3.New(backend, gofakes3.WithHostBucket(false)).Server())
	t.Cleanup(bucket.Close)

	page := &config.Page{
		Domain: config.FromString("example.com"),
		Bucket: config.BucketConfig{
			URL: config.EnvValue(bucket.URL), Name: "test",
			ApplicationID: "test", Secret: "test", Region: "test",
		},
		Git: config.GitConfig{
			Provider:   "custom",
			Repository: "spechtlabs/site",
			MainBranch: "main",
			Oidc: config.GitProvider{
				Issuer: issuer.server.URL,
				ClaimMappings: config.ClaimMapRaw{
					"repository": "repository", "commit": "sha", "branch": "ref", "environment": "environment",
				},
			},
		},
		Preview: config.PreviewConfig{Enabled: true, CommitSha: true, Branch: true},
	}
	// A page whose bucket doesn't exist, so storing the upload fails.
	broken := *page
	broken.Git.Repository = "spechtlabs/broken"
	broken.Bucket.Name = "missing"
	api := NewRestApi(config.StaticPagesConfig{Pages: []*config.Page{page, &broken}})

	tests := []struct {
		name        string
		claims      map[string]any
		files       map[string]string
		noAuth      bool
		badForm     bool
		canceled    bool
		wantStatus  int
		wantPreview []string
		wantObjects map[string]string
	}{
		{
			name:   "publishes the site and indexes the commit",
			claims: map[string]any{"repository": "spechtlabs/site", "sha": "abc123", "ref": "refs/heads/feature"},
			files: map[string]string{
				"files[index.html]":    "<h1>hello</h1>",
				"files[assets/app.js]": "console.log(1)",
			},
			wantStatus:  http.StatusOK,
			wantPreview: []string{"https://feature.example.com", "https://abc123.example.com"},
			wantObjects: map[string]string{
				"spechtlabs/site/abc123/index.html":    "<h1>hello</h1>",
				"spechtlabs/site/abc123/assets/app.js": "console.log(1)",
			},
		},
		{
			name:   "refuses a path outside the site",
			claims: map[string]any{"repository": "spechtlabs/site", "sha": "def456", "ref": "main"},
			files: map[string]string{
				"files[../escape.html]": "nope",
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "refuses a repository no page is configured for",
			claims:     map[string]any{"repository": "someone/else", "sha": "abc123", "ref": "main"},
			files:      map[string]string{"files[index.html]": "x"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "rejects a malformed form",
			claims:     map[string]any{"repository": "spechtlabs/site", "sha": "abc123", "ref": "main"},
			badForm:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "fails when the bucket can't be written",
			claims:     map[string]any{"repository": "spechtlabs/broken", "sha": "abc123", "ref": "main"},
			files:      map[string]string{"files[index.html]": "x"},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "gives up on a client that went away",
			claims:     map[string]any{"repository": "spechtlabs/site", "sha": "abc123", "ref": "main"},
			files:      map[string]string{"files[index.html]": "x"},
			canceled:   true,
			wantStatus: StatusRequestContextCanceled,
		},
		{
			name:       "refuses a request without a token",
			noAuth:     true,
			files:      map[string]string{"files[index.html]": "x"},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := multipartBody(t, tt.files)
			req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
			req.Header.Set("Content-Type", contentType)
			if tt.badForm {
				req.Header.Set("Content-Type", "multipart/form-data; boundary=nope")
			}
			if tt.canceled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			if !tt.noAuth {
				req.Header.Set("Authorization", "Bearer "+issuer.token(t, tt.claims))
			}

			rr := httptest.NewRecorder()
			api.router.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code, rr.Body.String())
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp struct {
				FileCount  int      `json:"file_count"`
				PreviewURL []string `json:"preview_url"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			assert.Equal(t, len(tt.files), resp.FileCount)
			assert.Equal(t, tt.wantPreview, resp.PreviewURL)

			for key, want := range tt.wantObjects {
				assert.Equal(t, want, readObject(t, backend, key), key)
			}

			var index map[string]map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(readObject(t, backend, "spechtlabs/site/index.yaml")), &index))
			assert.Contains(t, index, tt.claims["sha"])
		})
	}
}

func TestSaveArtifactsToTemp(t *testing.T) {
	api := NewRestApi(config.StaticPagesConfig{})

	tests := []struct {
		name      string
		files     map[string]string
		wantErr   bool
		wantCount int
		wantFiles map[string]string
	}{
		{
			name: "nested files, other fields ignored",
			files: map[string]string{
				"files[index.html]":           "index",
				"files[guides/intro.html]":    "intro",
				"files[guides/img/logo.svg]":  "<svg/>",
				"attachment[not-a-site-file]": "ignored",
			},
			wantCount: 3,
			wantFiles: map[string]string{
				"index.html":          "index",
				"guides/intro.html":   "intro",
				"guides/img/logo.svg": "<svg/>",
			},
		},
		{
			name:    "path escaping the upload directory",
			files:   map[string]string{"files[../../escape.txt]": "nope"},
			wantErr: true,
		},
		{
			name:    "absolute path",
			files:   map[string]string{"files[/etc/escape.txt]": "nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, contentType := multipartBody(t, tt.files)
			_, params, err := mime.ParseMediaType(contentType)
			require.NoError(t, err)
			form, err := multipart.NewReader(body, params["boundary"]).ReadForm(1 << 20)
			require.NoError(t, err)

			uploadPath, count, size, herr := api.saveArtifactsToTemp(context.Background(), form)
			t.Cleanup(func() { _ = os.RemoveAll(uploadPath) })

			if tt.wantErr {
				assert.Error(t, herr)
				_, statErr := os.Stat(filepath.Join(filepath.Dir(uploadPath), "escape.txt"))
				assert.True(t, os.IsNotExist(statErr), "nothing may be written outside the upload directory")
				return
			}

			require.NoError(t, herr)
			assert.Equal(t, tt.wantCount, count)

			var wantSize int64
			for rel, want := range tt.wantFiles {
				got, err := os.ReadFile(filepath.Join(uploadPath, rel))
				require.NoError(t, err)
				assert.Equal(t, want, string(got))
				wantSize += int64(len(want))
			}
			assert.Equal(t, wantSize, size)
		})
	}
}

func multipartBody(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for field, content := range files {
		part, err := w.CreateFormFile(field, filepath.Base(field))
		require.NoError(t, err)
		_, err = io.WriteString(part, content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	return body, w.FormDataContentType()
}

func readObject(t *testing.T, backend *s3mem.Backend, key string) string {
	t.Helper()

	obj, err := backend.GetObject("test", key, nil)
	require.NoError(t, err, key)
	defer func() { _ = obj.Contents.Close() }()

	content, err := io.ReadAll(obj.Contents)
	require.NoError(t, err)
	return string(content)
}
