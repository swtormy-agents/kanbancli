package kanban

import (
	"strings"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	ok, err := ValidateBaseURL("https://kanban.example.com/")
	if err != nil || ok != "https://kanban.example.com" {
		t.Fatalf("ok=%q err=%v", ok, err)
	}
	for _, raw := range []string{
		"ftp://example.com",
		"https://",
		"https://user:pass@host.example",
	} {
		if _, err := ValidateBaseURL(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestStripURLUserinfo(t *testing.T) {
	in := "https://user:pass@host.example/api"
	out := StripURLUserinfo(in)
	if strings.Contains(out, "user") || strings.Contains(out, "pass") {
		t.Fatalf("userinfo leaked: %q", out)
	}
}
