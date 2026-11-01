package cli

import (
	"net/url"
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
	if strings.ContainsAny(raw, "?#\\") {
		return "", usageErr("path must not contain ?, #, or backslashes")
	}
	if containsForbiddenPercentEncoding(raw) {
		return "", usageErr("path must not contain encoded slash, dot, percent, or backslash sequences")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", usageErr("invalid path encoding: %v", err)
	}
	if strings.Contains(decoded, "%") {
		return "", usageErr("path must not contain percent-encoding after decoding")
	}
	if strings.ContainsAny(decoded, "?#\\") {
		return "", usageErr("path must not contain ?, #, or backslashes after decoding")
	}
	if strings.Contains(decoded, "//") {
		return "", usageErr("path must not contain //")
	}
	if strings.Contains(decoded, "..") {
		return "", usageErr("path must not contain ..")
	}
	if err := validateAPISegments(decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

func containsForbiddenPercentEncoding(raw string) bool {
	lower := strings.ToLower(raw)
	for _, seq := range []string{"%2f", "%2e", "%25", "%5c"} {
		if strings.Contains(lower, seq) {
			return true
		}
	}
	return false
}

func validateAPISegments(path string) error {
	if path == "/api" {
		return nil
	}
	if !strings.HasPrefix(path, "/api/") {
		return usageErr("path must start with /api/")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, seg := range parts {
		if seg == "" {
			return usageErr("path must not contain empty path segments")
		}
		if seg == ".." {
			return usageErr("path must not contain ..")
		}
		if strings.Contains(seg, ".pe.") {
			return usageErr("enterprise-only paths are out of scope in this CLI")
		}
	}
	return nil
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
