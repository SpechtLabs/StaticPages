package api

import (
	"strings"

	"github.com/SpechtLabs/StaticPages/pkg/config"
)

func (r *RestApi) extractPagesConfig(repo string) *config.Page {
	for _, page := range r.conf.Pages {
		if page.Auth.Repository == repo {
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
