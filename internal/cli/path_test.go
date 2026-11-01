package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeAPIPath(t *testing.T) {
	ok := map[string]string{
		"/api/projects": "/api/projects",
		"api/projects":  "/api/projects",
		"/api":          "/api",
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
		"/api/a%2fb",
		"/api/a%2Fb",
		"/api/%252e%252e/x",
		"/api/a%252fb",
		"/api/%5cetc",
		"/api/x%25y",
		"/api/back\\slash",
		"/api/foo%2fbar",
	}
	for _, in := range bad {
		if _, err := normalizeAPIPath(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestAPIPathRejectedExit2NoHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP request for rejected path: %s", r.URL.Path)
	}))
	defer srv.Close()

	for _, path := range []string{
		"/api/a%2fb",
		"/api/%252e%252e/x",
		"/api/a%252fb",
		"/api/foo//bar",
		"/api/../secret",
		"/api/%2e%2e/admin",
		"/api/x%25y",
	} {
		code, _, errOut := runCLI(t, testEnv(srv.URL), "api", "GET", path)
		if code != 2 {
			t.Fatalf("path %q want exit 2 got %d stderr=%s", path, code, errOut)
		}
	}
}
