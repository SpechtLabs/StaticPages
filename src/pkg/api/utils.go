package api

import (
	"context"
	"strings"

	"github.com/SpechtLabs/StaticPages/pkg/config"
)

func (r *RestApi) extractPagesConfig(ctx context.Context, repo string) *config.Page {
	ctx, span := r.tracer.Start(ctx, "restApi.extractPagesConfig")
	defer span.End()

	for _, page := range r.conf.Pages {
		if page.Git.Repository == repo {
			return page
		}
	}

	return nil
}

func extractRelativePath(key string) (string, bool) {
	const prefix = "files["
	const suffix = "]"

	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
		return "", false
	}
	return key[len(prefix) : len(key)-len(suffix)], true
}
