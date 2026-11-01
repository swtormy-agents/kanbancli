package cli

import (
	"net/url"
	"path"
	"strings"
)

func normalizeAPIPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", usageErr("path is required")
	}
	if raw[0] != '/' {
		raw = "/" + raw
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		return "", usageErr("path must not contain ? or #; use --query for query parameters")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", usageErr("invalid path encoding: %v", err)
	}
	if strings.ContainsAny(decoded, "?#") {
		return "", usageErr("path must not contain ? or # after decoding")
	}
	if strings.Contains(decoded, "..") {
		return "", usageErr("path must not contain ..")
	}
	if strings.Contains(decoded, "//") {
		return "", usageErr("path must not contain //")
	}
	if strings.Contains(decoded, ".pe.") {
		return "", usageErr("enterprise-only paths are out of scope in this CLI")
	}
	cleaned := path.Clean(decoded)
	if cleaned == "/api" {
		return cleaned, nil
	}
	if !strings.HasPrefix(cleaned, "/api/") {
		return "", usageErr("path must start with /api/")
	}
	return cleaned, nil
}

func parseQueryFlags(pairs []string) (url.Values, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	q := make(url.Values)
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, usageErr("--query must be key=value")
		}
		q.Add(k, v)
	}
	return q, nil
}
