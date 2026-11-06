package mediastate

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// Selection records (L2): absent reads as no selection; a write is checked
// strictly (a shell slot, a package name, a sha256 release, an RFC 3339
// time) and merged per slot into the owner's set (L4: a write for one slot
// keeps the others); choosing the default is a write like any other; another
// owner sees none of it.
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
	// "Use default" for one slot: the base package is selected for it; the
	// write carries only that slot and the other slot's record stays (the
	// answer is the merged set).
	reset := `{"value":{"contextMenu.renderer":{"package":"menu-default","at":"` + at + `"}}}`
	code, got = prefCall(h, http.MethodPut, key, reset, "owner-a")
	if answer, _ := got["value"].(map[string]any); code != 200 || len(answer) != 2 {
		t.Fatalf("reset = %d %v", code, got)
	}
	_, got = prefCall(h, http.MethodGet, key, "", "owner-a")
	value, _ = got["value"].(map[string]any)
	if menu, _ := value["contextMenu.renderer"].(map[string]any); menu["package"] != "menu-default" {
		t.Fatalf("after use default = %v", got)
	}
	if launcher, _ := value["launcher"].(map[string]any); launcher["package"] != "launcher" {
		t.Fatalf("a write for one slot clobbered another: %v", got)
	}
	// Another slot's write later keeps this one.
	other := `{"value":{"launcher":{"package":"launcher-alt","at":"` + at + `"}}}`
	if code, _ := prefCall(h, http.MethodPut, key, other, "owner-a"); code != 200 {
		t.Fatalf("other = %d", code)
	}
	_, got = prefCall(h, http.MethodGet, key, "", "owner-a")
	value, _ = got["value"].(map[string]any)
	menu, _ = value["contextMenu.renderer"].(map[string]any)
	launcher, _ := value["launcher"].(map[string]any)
	if menu["package"] != "menu-default" || launcher["package"] != "launcher-alt" {
		t.Fatalf("after the other slot's write = %v", got)
	}
	if _, other := prefCall(h, http.MethodGet, key, "", "owner-b"); other["default"] != true {
		t.Fatalf("another owner = %v", other)
	}
}

// Concurrent writes for different slots all land (L4, provider-selection P6:
// two windows, or a choice and the yank fallback, writing at once).
func TestSelectionWritesForDifferentSlotsDoNotClobber(t *testing.T) {
	t.Parallel()
	h, _, _ := setupSettings(t, nil)
	key := apppackage.SelectionKey
	at := "2026-11-06T10:00:00Z"
	var wg sync.WaitGroup
	for _, slot := range apppackage.ShellServices {
		wg.Add(1)
		go func(slot string) {
			defer wg.Done()
			body := `{"value":{"` + slot + `":{"package":"pick-one","at":"` + at + `"}}}`
			if code, got := prefCall(h, http.MethodPut, key, body, "owner-a"); code != 200 {
				t.Errorf("%s = %d %v", slot, code, got)
			}
		}(slot)
	}
	wg.Wait()
	_, got := prefCall(h, http.MethodGet, key, "", "owner-a")
	if value, _ := got["value"].(map[string]any); len(value) != len(apppackage.ShellServices) {
		t.Fatalf("after concurrent writes = %v", got)
	}
}
