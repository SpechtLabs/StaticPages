package config

import (
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

// ApplyPageDefaults expands the optional top-level `pageDefaults` block into
// every entry under `pages`, so common settings (bucket, proxy, git, preview)
// can be declared once instead of repeated per page.
//
// Each page is deep-merged over the defaults: a value the page sets wins, and
// anything it omits is inherited — recursively, per leaf field. Lists (such as
// proxy.searchPath) are replaced wholesale rather than concatenated. The merge
// runs on the raw config maps before unmarshalling, so a field that is present
// with a zero value (e.g. `preview.enabled: false`) still overrides a non-zero
// default — something a struct-level merge could not distinguish from "unset".
//
// It is a no-op when `pageDefaults` is absent, leaving behaviour unchanged.
func ApplyPageDefaults(v *viper.Viper) {
	defaults := v.GetStringMap("pageDefaults")
	if len(defaults) == 0 {
		return
	}

	pages, ok := v.Get("pages").([]any)
	if !ok {
		return
	}

	v.Set("pages", mergePageDefaults(defaults, pages))
}

// mergePageDefaults returns a new page list with defaults applied beneath each
// page (the page takes precedence).
func mergePageDefaults(defaults map[string]any, pages []any) []any {
	merged := make([]any, 0, len(pages))
	for _, page := range pages {
		merged = append(merged, deepMergeMaps(defaults, cast.ToStringMap(page)))
	}
	return merged
}

// deepMergeMaps returns a new map combining base and override. Where both hold
// a sub-map for the same key it recurses; otherwise the override value wins.
// base is never mutated.
func deepMergeMaps(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}

	for k, ov := range override {
		if bv, ok := out[k]; ok {
			if bm, bok := asStringMap(bv); bok {
				if om, ook := asStringMap(ov); ook {
					out[k] = deepMergeMaps(bm, om)
					continue
				}
			}
		}
		out[k] = ov
	}

	return out
}

// asStringMap reports whether v is a YAML mapping and, if so, returns it as a
// map[string]any (handling both the map[string]any and map[any]any shapes that
// different YAML decoders produce).
func asStringMap(v any) (map[string]any, bool) {
	switch v.(type) {
	case map[string]any, map[any]any:
		return cast.ToStringMap(v), true
	default:
		return nil, false
	}
}
