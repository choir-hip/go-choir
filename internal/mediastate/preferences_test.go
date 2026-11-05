package mediastate

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// S6 failure modes G1 to G4 are listed in internal/apppackage/settings.go.
// Route-level ones:
//
//	R1 an undeclared key, another package's key, or a key of an uninstalled
//	   package is read or written (a package could squat the preference table);
//	R2 a wrong type, an out-of-enum string, a non-finite number, an oversize
//	   value or an unknown field is stored;
//	R3 a refused write emits an event or changes the stored value;
//	R4 one owner reads or changes another's value;
//	R5 a value does not survive a restart of the store;
//	R6 the theme route changes (theme still answers at the same path).

func notes() []apppackage.SettingsSection {
	return []apppackage.SettingsSection{{Section: "Notes", Fields: []apppackage.SettingField{
		{Key: "autosave", Type: "boolean", Default: json.RawMessage(`true`), Label: "Autosave"},
		{Key: "font", Type: "string", Default: json.RawMessage(`"serif"`), Label: "Font", Enum: []string{"serif", "mono"}},
		{Key: "size", Type: "number", Default: json.RawMessage(`14`), Label: "Size"},
		{Key: "motto", Type: "string", Default: json.RawMessage(`""`), Label: "Motto"},
	}}}
}

func setupSettings(t *testing.T, installed map[string][]apppackage.SettingsSection) (*Handler, *events.EventBus, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewEventBus()
	h := NewHandler(s, bus)
	h.SetSettingSource(func(_ context.Context, _ string, pkg string) []apppackage.SettingsSection { return installed[pkg] })
	return h, bus, path
}

func prefCall(h *Handler, method, key, body, owner string) (int, map[string]any) {
	w := mediaStateRequest(h.HandlePreference, method, "/api/preferences/"+key, body, owner, map[string]string{"X-Choir-Device": "device-a"})
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestSettingDefaultsRoundTripAndEvent(t *testing.T) {
	t.Parallel()
	h, bus, _ := setupSettings(t, map[string][]apppackage.SettingsSection{"notes": notes()})
	ch := bus.SubscribeWithBuffer(8)
	defer bus.Unsubscribe(ch)

	code, got := prefCall(h, http.MethodGet, "pkg.notes.autosave", "", "owner-a")
	if code != 200 || got["value"] != true || got["default"] != true {
		t.Fatalf("default = %d %v", code, got)
	}
	code, got = prefCall(h, http.MethodPut, "pkg.notes.font", `{"value":"mono"}`, "owner-a")
	if code != 200 || got["value"] != "mono" || got["default"] != false {
		t.Fatalf("put = %d %v", code, got)
	}
	ev := <-ch
	if ev.Record.Kind != types.EventSettingUpdated || ev.Record.OwnerID != "owner-a" {
		t.Fatalf("event = %s owner %s", ev.Record.Kind, ev.Record.OwnerID)
	}
	var payload map[string]any
	_ = json.Unmarshal(ev.Record.Payload, &payload)
	if payload["key"] != "pkg.notes.font" || payload["value"] != "mono" || payload["source_device_id"] != "device-a" {
		t.Fatalf("payload = %v", payload)
	}
	if code, got = prefCall(h, http.MethodGet, "pkg.notes.font", "", "owner-a"); code != 200 || got["value"] != "mono" || got["default"] != false {
		t.Fatalf("get = %d %v", code, got)
	}
	if code, _ = prefCall(h, http.MethodPut, "pkg.notes.size", `{"value":18.5}`, "owner-a"); code != 200 {
		t.Fatalf("number = %d", code)
	}
	// R4: another owner sees the default.
	if code, got = prefCall(h, http.MethodGet, "pkg.notes.font", "", "owner-b"); code != 200 || got["value"] != "serif" || got["default"] != true {
		t.Fatalf("R4 = %d %v", code, got)
	}
}

func TestSettingRefusals(t *testing.T) {
	t.Parallel()
	h, bus, _ := setupSettings(t, map[string][]apppackage.SettingsSection{"notes": notes()})
	ch := bus.SubscribeWithBuffer(8)
	defer bus.Unsubscribe(ch)
	big := `{"value":"` + strings.Repeat("x", apppackage.MaxSettingStringLen+1) + `"}`
	cases := []struct {
		name, method, key, body, owner string
		want                           int
	}{
		{"R1 undeclared field", "GET", "pkg.notes.nope", "", "o", 404},
		{"R1 undeclared field write", "PUT", "pkg.notes.nope", `{"value":1}`, "o", 404},
		{"R1 uninstalled package", "PUT", "pkg.ghost.autosave", `{"value":true}`, "o", 404},
		{"R1 another package's key", "PUT", "pkg.other.autosave", `{"value":true}`, "o", 404},
		{"R1 not a package key", "GET", "locale", "", "o", 404},
		{"R1 key without a field", "GET", "pkg.notes", "", "o", 404},
		{"R2 wrong type", "PUT", "pkg.notes.autosave", `{"value":"yes"}`, "o", 400},
		{"R2 out of enum", "PUT", "pkg.notes.font", `{"value":"comic"}`, "o", 400},
		{"R2 string where number", "PUT", "pkg.notes.size", `{"value":"9"}`, "o", 400},
		{"R2 overflow number", "PUT", "pkg.notes.size", `{"value":1e999}`, "o", 400},
		{"R2 oversize string", "PUT", "pkg.notes.motto", big, "o", 400},
		{"R2 oversize body", "PUT", "pkg.notes.motto", `{"value":"` + strings.Repeat("x", maxSettingBody) + `"}`, "o", 413},
		{"R2 unknown field", "PUT", "pkg.notes.autosave", `{"value":true,"extra":1}`, "o", 400},
		{"R2 no value", "PUT", "pkg.notes.autosave", `{}`, "o", 400},
		{"R2 null", "PUT", "pkg.notes.autosave", `{"value":null}`, "o", 400},
		{"R2 two values", "PUT", "pkg.notes.autosave", `{"value":true}{"value":false}`, "o", 400},
		{"no owner", "GET", "pkg.notes.autosave", "", "", 401},
		{"method", "DELETE", "pkg.notes.autosave", "", "o", 405},
	}
	for _, c := range cases {
		if code, _ := prefCall(h, c.method, c.key, c.body, c.owner); code != c.want {
			t.Errorf("%s: %d, want %d", c.name, code, c.want)
		}
	}
	// R3: nothing stored, nothing emitted.
	select {
	case ev := <-ch:
		t.Fatalf("R3: a refusal emitted %s", ev.Record.Kind)
	default:
	}
	if _, got := prefCall(h, http.MethodGet, "pkg.notes.autosave", "", "o"); got["default"] != true {
		t.Fatalf("R3: a refusal stored %v", got)
	}
}

// G4: the package updated its field; the stored value no longer fits.
func TestSettingStoredValueThatNoLongerFitsReadsAsTheDefault(t *testing.T) {
	t.Parallel()
	installed := map[string][]apppackage.SettingsSection{"notes": notes()}
	h, _, _ := setupSettings(t, installed)
	if code, _ := prefCall(h, http.MethodPut, "pkg.notes.font", `{"value":"mono"}`, "o"); code != 200 {
		t.Fatal(code)
	}
	installed["notes"] = []apppackage.SettingsSection{{Section: "Notes", Fields: []apppackage.SettingField{
		{Key: "font", Type: "string", Default: json.RawMessage(`"serif"`), Label: "Font", Enum: []string{"serif", "sans"}},
	}}}
	if code, got := prefCall(h, http.MethodGet, "pkg.notes.font", "", "o"); code != 200 || got["value"] != "serif" || got["default"] != true {
		t.Fatalf("G4 = %d %v", code, got)
	}
	// Uninstall keeps the value; the key is refused until the package returns.
	delete(installed, "notes")
	if code, _ := prefCall(h, http.MethodGet, "pkg.notes.font", "", "o"); code != 404 {
		t.Fatalf("uninstalled = %d", code)
	}
}

// R5: the value is a projected tape fact and folds back after a restart.
func TestSettingSurvivesAStoreRestart(t *testing.T) {
	h, _, path := setupSettings(t, map[string][]apppackage.SettingsSection{"notes": notes()})
	if code, _ := prefCall(h, http.MethodPut, "pkg.notes.size", `{"value":21}`, "o"); code != 200 {
		t.Fatal(code)
	}
	if err := h.store.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h2 := NewHandler(s, events.NewEventBus())
	h2.SetSettingSource(func(_ context.Context, _ string, pkg string) []apppackage.SettingsSection { return map[string][]apppackage.SettingsSection{"notes": notes()}[pkg] })
	if code, got := prefCall(h2, http.MethodGet, "pkg.notes.size", "", "o"); code != 200 || got["value"] != float64(21) || got["default"] != false {
		t.Fatalf("R5 = %d %v", code, got)
	}
}

// R1 with no source wired: every package key is refused. R6: the theme path
// is the theme handler.
func TestPreferenceRouteWithoutDeclarationsAndTheme(t *testing.T) {
	t.Parallel()
	_, _, h := testMediaStateSetup(t)
	if w := mediaStateRequest(h.HandlePreference, http.MethodPut, "/api/preferences/pkg.notes.autosave", `{"value":true}`, "o", nil); w.Code != 404 {
		t.Fatalf("no source = %d", w.Code)
	}
	if w := mediaStateRequest(h.HandlePreference, http.MethodPut, "/api/preferences/theme", `{"theme":{"id":"x","schema_version":1}}`, "o", nil); w.Code != 200 {
		t.Fatalf("R6 theme put = %d %s", w.Code, w.Body.String())
	}
	if w := mediaStateRequest(h.HandlePreference, http.MethodGet, "/api/preferences/theme", "", "o", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"x"`) {
		t.Fatalf("R6 theme get = %d %s", w.Code, w.Body.String())
	}
}
