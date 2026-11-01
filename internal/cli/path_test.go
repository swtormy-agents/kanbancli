package cli

import (
	"testing"
)

func TestNormalizeAPIPath(t *testing.T) {
	ok := map[string]string{
		"/api/projects": "/api/projects",
		"api/projects":  "/api/projects",
	}
	for in, want := range ok {
		got, err := normalizeAPIPath(in)
		if err != nil || got != want {
			t.Fatalf("%q => %q err=%v want %q", in, got, err, want)
		}
	}
	bad := []string{
		"/apix/foo",
		"/api/../etc/passwd",
		"/api/foo//bar",
		"/api/foo?x=1",
		"/api/foo%3Fx=1",
		"/api/%2e%2e/admin",
		"/api/foo%23frag",
		"/api/enterprise.pe.foo",
	}
	for _, in := range bad {
		if _, err := normalizeAPIPath(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}
