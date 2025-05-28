package api

import (
	"context"
	"strings"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/sierrasoftworks/humane-errors-go"
)

func (r *RestApi) extractPagesConfig(ctx context.Context, repo string) (*config.Page, humane.Error) {
	_, span := r.tracer.Start(ctx, "restApi.extractPagesConfig")
	defer span.End()

	for _, page := range r.conf.Pages {
		if page.Git.Repository == repo {
			return page, nil
		}
	}

	return nil, humane.New("no matching page found", "Make sure the repository is configured in the config file.")
}

func extractRelativePath(key string) (string, bool) {
	const prefix = "files["
	const suffix = "]"

	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
		return "", false
	}
	return key[len(prefix) : len(key)-len(suffix)], true
}
