package kanban

import (
	"encoding/json"
	"strings"
	"testing"
)

func testFixtureToken() string {
	return "test-" + "fixture-" + "value-01"
}

func TestSanitizeUsers(t *testing.T) {
	raw := []byte(`[{"id":"1","username":"u","name":"U","email":"secret@example.com"}]`)
	out, err := SanitizeUsers(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "secret@") || strings.Contains(s, "email") {
		t.Fatalf("email leaked: %s", s)
	}
	if !strings.Contains(s, `"username":"u"`) {
		t.Fatalf("expected username: %s", s)
	}
}

func TestExtractAccessToken(t *testing.T) {
	want := testFixtureToken()
	cases := []struct {
		raw string
	}{
		{`{"item":"` + want + `"}`},
		{`{"item":{"token":"` + want + `"}}`},
	}
	for _, tc := range cases {
		got, err := ExtractAccessToken(json.RawMessage(tc.raw))
		if err != nil || got != want {
			t.Fatalf("raw=%s got=%q err=%v", tc.raw, got, err)
		}
	}
}

func TestExtractBoardListsCards(t *testing.T) {
	board := []byte(`{"item":{"id":"1"},"included":{"lists":[{"id":"2"}],"cards":[{"id":"3"}]}}`)
	lists, err := ExtractBoardLists(board)
	if err != nil || !strings.Contains(string(lists), `"id":"2"`) {
		t.Fatalf("lists=%s err=%v", lists, err)
	}
	cards, err := ExtractBoardCards(board)
	if err != nil || !strings.Contains(string(cards), `"id":"3"`) {
		t.Fatalf("cards=%s err=%v", cards, err)
	}
}
