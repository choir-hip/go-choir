package mediastate

import (
	"context"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The update policy preference (F1): absent reads as manual for every app;
// a write is checked strictly and stored per owner; an invalid value is
// refused, never stored and misread.
func TestUpdatePolicyPreferenceIsManualUntilTheOwnerChooses(t *testing.T) {
	t.Parallel()
	h, _, _ := setupSettings(t, nil)
	key := pkgrelease.UpdatePolicyKey
	code, got := prefCall(h, http.MethodGet, key, "", "owner-a")
	value, _ := got["value"].(map[string]any)
	if code != 200 || got["default"] != true || value["default"] != pkgrelease.UpdateManual {
		t.Fatalf("absent = %d %v", code, got)
	}
	for _, bad := range []string{`{"value":{"default":"always"}}`, `{"value":{"default":"manual","apps":{"x":"later"}}}`, `{"value":"automatic"}`} {
		if code, _ := prefCall(h, http.MethodPut, key, bad, "owner-a"); code != http.StatusBadRequest {
			t.Errorf("%s = %d", bad, code)
		}
	}
	if code, got := prefCall(h, http.MethodPut, key, `{"value":{"default":"manual","apps":{"calendar":"automatic"}}}`, "owner-a"); code != 200 {
		t.Fatalf("put = %d %v", code, got)
	}
	p := ReadUpdatePolicy(context.Background(), h.store, "owner-a")
	if p.For("calendar") != pkgrelease.UpdateAutomatic || p.For("pulse") != pkgrelease.UpdateManual {
		t.Fatalf("read back = %+v", p)
	}
	if other := ReadUpdatePolicy(context.Background(), h.store, "owner-b"); other.For("calendar") != pkgrelease.UpdateManual {
		t.Fatalf("another owner = %+v", other)
	}
}
