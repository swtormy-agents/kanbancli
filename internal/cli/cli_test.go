package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const testToken = "kanbancli-test-tk" // 16 chars // gitleaks:allow

func testEnv(url string) func(string) string {
	return func(k string) string {
		switch k {
		case "KANBANCLI_URL":
			return url
		case "KANBANCLI_TOKEN":
			return testToken
		}
		return ""
	}
}

func testEnvLogin(url string) func(string) string {
	return func(k string) string {
		switch k {
		case "KANBANCLI_URL":
			return url
		case "KANBANCLI_EMAIL":
			return "agent@example.com"
		case "KANBANCLI_PASSWORD":
			return "super-secret-password-12345"
		}
		return ""
	}
}

func authHeader(r *http.Request) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	return strings.HasPrefix(h, prefix) && strings.TrimPrefix(h, prefix) == testToken
}

func runCLI(t *testing.T, getenv func(string) string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Execute(args, strings.NewReader(""), &out, &errOut, getenv)
	return code, out.String(), errOut.String()
}

func newAPIMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/me", func(w http.ResponseWriter, r *http.Request) {
		if !authHeader(r) {
			http.Error(w, "nope", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"item":{"id":"1","username":"agent"}}`))
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"item":{"version":"2.0.0"}}`))
	})
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":"10","name":"demo"}]`))
	})
	mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"item":{"id":"10","name":"demo"}}`))
	})
	mux.HandleFunc("/api/boards/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"item":{"id":"2"},"included":{"lists":[{"id":"20","name":"Todo"}],"cards":[{"id":"30","name":"Task"}]}}`))
	})
	mux.HandleFunc("/api/cards/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			if r.Method == http.MethodGet {
				w.Write([]byte(`[{"id":"40","text":"hi"}]`))
				return
			}
			if r.Method == http.MethodPost {
				w.Write([]byte(`{"item":{"id":"41","text":"new"}}`))
				return
			}
		}
		if r.Method == http.MethodPatch {
			w.Write([]byte(`{"item":{"id":"30","listId":"20"}}`))
			return
		}
		w.Write([]byte(`{"item":{"id":"30","name":"Task"}}`))
	})
	mux.HandleFunc("/api/lists/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cards") && r.Method == http.MethodPost {
			w.Write([]byte(`{"item":{"id":"31","name":"new"}}`))
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":"1","username":"u","name":"U","email":"x@y.z"}]`))
	})
	mux.HandleFunc("/api/access-tokens", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["password"] == nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"item":"kanbancli-test-tk"}`))
	})
	return mux
}

func TestHealthAndVersion(t *testing.T) {
	srv := httptest.NewServer(newAPIMux())
	defer srv.Close()
	code, out, _ := runCLI(t, testEnv(srv.URL), "health")
	if code != 0 || !strings.Contains(out, `"ok":true`) {
		t.Fatalf("health code=%d out=%s", code, out)
	}
	code, out, _ = runCLI(t, testEnv(srv.URL), "version")
	if code != 0 || !strings.Contains(out, "2.0.0") {
		t.Fatalf("version code=%d out=%s", code, out)
	}
}

func TestMeProjectsBoardCards(t *testing.T) {
	srv := httptest.NewServer(newAPIMux())
	defer srv.Close()
	if code, out, _ := runCLI(t, testEnv(srv.URL), "me"); code != 0 || !strings.Contains(out, "agent") {
		t.Fatalf("me code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "projects", "list"); code != 0 || !strings.Contains(out, "demo") {
		t.Fatalf("projects code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "project", "get", "--id", "10"); code != 0 || !strings.Contains(out, "demo") {
		t.Fatalf("project get code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "boards", "get", "--id", "2"); code != 0 || !strings.Contains(out, "Todo") {
		t.Fatalf("boards get code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "lists", "list", "--board", "2"); code != 0 || !strings.Contains(out, "Todo") {
		t.Fatalf("lists code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "cards", "list", "--board", "2"); code != 0 || !strings.Contains(out, "Task") {
		t.Fatalf("cards list code=%d out=%s", code, out)
	}
	if code, out, _ := runCLI(t, testEnv(srv.URL), "card", "get", "--id", "30"); code != 0 || !strings.Contains(out, "Task") {
		t.Fatalf("card get code=%d out=%s", code, out)
	}
}

func TestCommentsAndUsers(t *testing.T) {
	srv := httptest.NewServer(newAPIMux())
	defer srv.Close()
	if code, _, _ := runCLI(t, testEnv(srv.URL), "comments", "list", "--card", "30"); code != 0 {
		t.Fatalf("comments list exit %d", code)
	}
	code, out, _ := runCLI(t, testEnv(srv.URL), "users", "list")
	if code != 0 || strings.Contains(out, "x@y.z") || strings.Contains(out, "email") {
		t.Fatalf("users code=%d out=%s", code, out)
	}
}

func TestAuthLoginShowToken(t *testing.T) {
	srv := httptest.NewServer(newAPIMux())
	defer srv.Close()
	code, out, errOut := runCLI(t, testEnvLogin(srv.URL), "auth", "login", "--show-token")
	if code != 0 {
		t.Fatalf("login code=%d err=%s", code, errOut)
	}
	if strings.TrimSpace(out) != testToken {
		t.Fatalf("token out=%q", out)
	}
	if strings.Contains(errOut, "super-secret") {
		t.Fatalf("password leaked: %s", errOut)
	}
}

func TestAuthLoginDryRunRedactsPassword(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	code, out, _ := runCLI(t, testEnvLogin(srv.URL), "auth", "login", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out, "super-secret") {
		t.Fatalf("password in dry-run: %s", out)
	}
	if !strings.Contains(out, "<REDACTED>") {
		t.Fatalf("expected redacted password: %s", out)
	}
}

func TestAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	code, _, errOut := runCLI(t, testEnv(srv.URL), "version")
	if code != 3 {
		t.Fatalf("want exit 3 got %d stderr=%s", code, errOut)
	}
}

func TestAuth401TokenNotInStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		http.Error(w, `{"message":"invalid token `+key+`"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	code, _, errOut := runCLI(t, testEnv(srv.URL), "version")
	if code != 3 {
		t.Fatalf("want exit 3 got %d", code)
	}
	if strings.Contains(errOut, testToken) {
		t.Fatalf("token leaked: %s", errOut)
	}
	if !strings.Contains(errOut, "<API_TOKEN>") {
		t.Fatalf("expected redacted token: %s", errOut)
	}
}

func TestDryRunRedactsToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	code, out, _ := runCLI(t, testEnv(srv.URL), "health", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out, testToken) {
		t.Fatalf("token leaked: %s", out)
	}
	if !strings.Contains(out, "<API_TOKEN>") {
		t.Fatalf("expected placeholder: %s", out)
	}
}

func TestAPICallWithQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("foo") != "bar" {
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, _, _ := runCLI(t, testEnv(srv.URL), "api", "GET", "/api/projects", "--query", "foo=bar")
	if code != 0 {
		t.Fatalf("api exit %d", code)
	}
}

func TestAPIPathValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP request during path validation: %s", r.URL.Path)
	}))
	defer srv.Close()
	for _, args := range [][]string{
		{"api", "GET", "/apix/foo"},
		{"api", "GET", "/api/../etc/passwd"},
		{"api", "GET", "/api/foo//bar"},
		{"api", "GET", "/api/a%2fb"},
		{"api", "GET", "/api/%252e%252e/x"},
		{"api", "GET", "/api/a%252fb"},
	} {
		code, _, errOut := runCLI(t, testEnv(srv.URL), args...)
		if code != 2 {
			t.Fatalf("args %v want exit 2 got %d stderr=%s", args, code, errOut)
		}
	}
}

func TestWriteRequiresWriteFlag(t *testing.T) {
	var errOut bytes.Buffer
	code := Execute([]string{"api", "POST", "/api/projects", "--body", "{}"},
		strings.NewReader(""), io.Discard, &errOut, testEnv("http://example"))
	if code != 3 {
		t.Fatalf("want 3 got %d: %s", code, errOut.String())
	}
}

func TestCardCreateRequiresWrite(t *testing.T) {
	var errOut bytes.Buffer
	code := Execute([]string{"card", "create", "--list", "1", "--name", "x"},
		strings.NewReader(""), io.Discard, &errOut, testEnv("http://example"))
	if code != 3 {
		t.Fatalf("want 3 got %d: %s", code, errOut.String())
	}
}

func TestWriteConfirmationSuccess(t *testing.T) {
	old := isTerminal
	isTerminal = func(ioReader) bool { return true }
	defer func() { isTerminal = old }()

	srv := httptest.NewServer(newAPIMux())
	defer srv.Close()
	var out bytes.Buffer
	code := Execute(
		[]string{"card", "create", "--write", "--list", "20", "--name", "x"},
		strings.NewReader("write\n"),
		&out,
		io.Discard,
		testEnv(srv.URL),
	)
	if code != 0 {
		t.Fatalf("exit %d out=%s", code, out.String())
	}
}

func TestInvalidBaseURLs(t *testing.T) {
	cases := []struct {
		url  string
		code int
	}{
		{"ftp://example.com", 2},
		{"https://", 2},
		{"https://user:pass@host.example", 2},
	}
	for _, tc := range cases {
		getenv := func(k string) string {
			switch k {
			case "KANBANCLI_URL":
				return tc.url
			case "KANBANCLI_TOKEN":
				return testToken
			}
			return ""
		}
		var errOut bytes.Buffer
		code := Execute([]string{"version"}, strings.NewReader(""), io.Discard, &errOut, getenv)
		if code != tc.code {
			t.Fatalf("url %q want exit %d got %d: %s", tc.url, tc.code, code, errOut.String())
		}
	}
}

func TestInvalidIDFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP during invalid ID validation: %s", r.URL.Path)
	}))
	defer srv.Close()
	cases := [][]string{
		{"project", "get", "--id", "0"},
		{"project", "get", "--id", "-1"},
		{"project", "get", "--id", "1a"},
		{"project", "get", "--id", "1/2"},
		{"project", "get", "--id", "%2e"},
		{"project", "get", "--id", "18446744073709551616"},
		{"card", "get", "--id", "0"},
		{"lists", "list", "--board", "-1"},
	}
	for _, args := range cases {
		code, _, errOut := runCLI(t, testEnv(srv.URL), args...)
		if code != 2 {
			t.Fatalf("args %v want exit 2 got %d stderr=%s", args, code, errOut)
		}
	}
}

func TestSkillPrint(t *testing.T) {
	code, out, _ := runCLI(t, testEnv("http://127.0.0.1:1"), "skill", "print")
	if code != 0 || !strings.Contains(out, "name: kanbancli") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestSkillMarkdownIdentical(t *testing.T) {
	a, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../skills/kanbancli/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("SKILL.md files differ; run scripts/sync-skill-md.sh")
	}
	if string(a) != skillMD {
		t.Fatalf("embedded SKILL.md out of sync")
	}
}

func TestReportErrorRedactsToken(t *testing.T) {
	var errOut bytes.Buffer
	a := &app{token: testToken, errOut: &errOut}
	code := a.reportError(runtimeErr(fmt.Errorf("failure mentioning %s", testToken)))
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if strings.Contains(errOut.String(), testToken) {
		t.Fatalf("token leaked")
	}
}

func TestShortTokenRejected(t *testing.T) {
	short := "short-token-15c" // 15 chars // gitleaks:allow
	getenv := func(k string) string {
		switch k {
		case "KANBANCLI_URL":
			return "http://127.0.0.1:1"
		case "KANBANCLI_TOKEN":
			return short
		}
		return ""
	}
	var errOut bytes.Buffer
	code := Execute([]string{"version"}, strings.NewReader(""), io.Discard, &errOut, getenv)
	if code != 2 {
		t.Fatalf("want exit 2 got %d: %s", code, errOut.String())
	}
}

func TestMissingToken(t *testing.T) {
	getenv := func(k string) string {
		if k == "KANBANCLI_URL" {
			return "http://127.0.0.1:1"
		}
		return ""
	}
	var errOut bytes.Buffer
	code := Execute([]string{"health"}, strings.NewReader(""), io.Discard, &errOut, getenv)
	if code != 3 {
		t.Fatalf("want exit 3 got %d", code)
	}
}
