package mediastate

import (
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// Selection records (L2): absent reads as no selection; a write is checked
// strictly (a shell slot, a package name, a sha256 release, an RFC 3339
// time) and replaces the owner's set; choosing the default is a write like
// any other; another owner sees none of it.
func TestSelectionRecordsAreTheOwnersAndStrict(t *testing.T) {
	t.Parallel()
	h, _, _ := setupSettings(t, nil)
	key := apppackage.SelectionKey
	code, got := prefCall(h, http.MethodGet, key, "", "owner-a")
	if value, _ := got["value"].(map[string]any); code != 200 || got["default"] != true || len(value) != 0 {
		t.Fatalf("absent = %d %v", code, got)
	}
	at := "2026-11-06T10:00:00Z"
	rel := strings.Repeat("a", 64)
	for _, bad := range []string{
		`{"value":{"windows":{"package":"menu-compact","at":"` + at + `"}}}`,
		`{"value":{"launcher":{"package":"Bad Name","at":"` + at + `"}}}`,
		`{"value":{"launcher":{"package":"launcher","release":"r1","at":"` + at + `"}}}`,
		`{"value":{"launcher":{"package":"launcher","at":"yesterday"}}}`,
		`{"value":{"launcher":{"package":"launcher","at":"` + at + `","extra":1}}}`,
		`{"value":"launcher"}`,
	} {
		if code, _ := prefCall(h, http.MethodPut, key, bad, "owner-a"); code != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, code)
		}
	}
	put := `{"value":{"contextMenu.renderer":{"package":"menu-compact","release":"` + rel + `","at":"` + at + `"},"launcher":{"package":"launcher","at":"` + at + `"}}}`
	if code, got := prefCall(h, http.MethodPut, key, put, "owner-a"); code != 200 {
		t.Fatalf("put = %d %v", code, got)
	}
	code, got = prefCall(h, http.MethodGet, key, "", "owner-a")
	value, _ := got["value"].(map[string]any)
	menu, _ := value["contextMenu.renderer"].(map[string]any)
	if code != 200 || got["default"] == true || len(value) != 2 || menu["package"] != "menu-compact" || menu["release"] != rel {
		t.Fatalf("read back = %d %v", code, got)
	}
	// "Use default": the base package is selected; nothing else changes.
	reset := `{"value":{"contextMenu.renderer":{"package":"menu-default","at":"` + at + `"},"launcher":{"package":"launcher","at":"` + at + `"}}}`
	if code, _ := prefCall(h, http.MethodPut, key, reset, "owner-a"); code != 200 {
		t.Fatalf("reset = %d", code)
	}
	_, got = prefCall(h, http.MethodGet, key, "", "owner-a")
	value, _ = got["value"].(map[string]any)
	if menu, _ := value["contextMenu.renderer"].(map[string]any); menu["package"] != "menu-default" {
		t.Fatalf("after use default = %v", got)
	}
	if _, other := prefCall(h, http.MethodGet, key, "", "owner-b"); other["default"] != true {
		t.Fatalf("another owner = %v", other)
	}
}
