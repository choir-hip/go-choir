package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The update sync at the proxy (F1). Failure modes pinned:
//
//	U1  the sync runs without the owner's act (an agent key without
//	    owner:acts);
//	U2  the computer gets a candidate whose record does not hash to its id,
//	    or whose tree the store answers is not the record's;
//	U3  corpusd is asked as anyone but the authenticated owner, or about
//	    anything but the computer's own installs.

func TestPackageUpdateSyncCarriesOnlyVerifiedCandidates(t *testing.T) {
	tree := []byte("diff --git a/frontend/src/packages/demo-next/App.svelte b/frontend/src/packages/demo-next/App.svelte\n")
	var mu sync.Mutex
	var guestIn, corpusdAsked []byte
	var installedID, goodID, badID, user string
	var goodRaw, badRaw []byte

	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/api/packages/installed" {
			_ = json.NewEncoder(w).Encode(map[string]any{"packages": []map[string]any{{"name": "demo-next", "release_id": installedID, "track": user}}})
			return
		}
		mu.Lock()
		guestIn, _ = io.ReadAll(r.Body)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"updates":[]}`))
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/platform/packages/updates":
			mu.Lock()
			corpusdAsked, _ = io.ReadAll(r.Body)
			mu.Unlock()
			old := time.Now().Add(-48 * time.Hour)
			offer := func(id string, raw []byte, seq int) map[string]any {
				return map[string]any{
					"release": pkgrelease.TrackRelease{ReleaseID: id, Package: "demo-next", Track: user, Seq: seq, State: pkgrelease.StateListed, Version: 1, ListedAt: old},
					"view":    map[string]any{"release_id": id, "record": json.RawMessage(raw)},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"updates": []map[string]any{{
				"package": "demo-next", "track": user, "release_id": installedID,
				"installed": pkgrelease.TrackRelease{ReleaseID: installedID, Package: "demo-next", Track: user, Seq: 1, State: pkgrelease.StateListed, Version: 1},
				"newest":    offer(badID, badRaw, 3),
				"soaked":    offer(goodID, goodRaw, 2),
			}}})
		case strings.Contains(r.URL.Path, "/blobs/"):
			if r.URL.Query().Get("owner_id") != user {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(tree)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(corpusd.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	u, err := store.CreateUser("update-user", "update@example.com")
	if err != nil {
		t.Fatal(err)
	}
	user = u.ID
	_, installedID, _ = installRecordNamed(t, append([]byte("v1"), tree...), user, "demo-next")
	_, goodID, goodRaw = installRecord(t, tree, user)
	_, badID, badRaw = installRecordNamed(t, tree, user, "demo-next")
	badRaw = []byte(strings.Replace(string(badRaw), `"operation_id":"op-1"`, `"operation_id":"op-z"`, 1))
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "desktop_id": "primary", "user_id": user, "state": "active", "computer_url": guest.URL})
	}))
	t.Cleanup(ownership.Close)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	key := func(scopes ...string) string {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user, "update-key-"+strings.Join(scopes, "-"), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	send := func(secret string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/packages/updates", strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer "+secret)
		w := httptest.NewRecorder()
		handler.HandleAPI(w, r)
		return w
	}

	// U1
	if rec := send(key("computer:self_development:approve")); rec.Code != http.StatusForbidden || guestIn != nil {
		t.Fatalf("U1 = %d, guest got %s", rec.Code, guestIn)
	}
	rec := send(key("computer:self_development:approve", OwnerActsScope))
	if rec.Code != http.StatusOK {
		t.Fatalf("sync = %d %s", rec.Code, rec.Body.String())
	}
	// U3
	var asked PackageUpdatesAsked
	if err := json.Unmarshal(corpusdAsked, &asked); err != nil || asked.OwnerID != user || len(asked.Installs) != 1 || asked.Installs[0].ReleaseID != installedID {
		t.Fatalf("U3 corpusd asked %s", corpusdAsked)
	}
	// U2: the forged newest is left out, the soaked one carries record and tree.
	var body struct {
		Packages []packageUpdateOffer `json:"packages"`
	}
	if err := json.Unmarshal(guestIn, &body); err != nil || len(body.Packages) != 1 {
		t.Fatalf("guest got %s (%v)", guestIn, err)
	}
	p := body.Packages[0]
	if p.Newest != nil {
		t.Fatalf("U2 a forged candidate reached the computer: %+v", p.Newest.Release)
	}
	if p.Soaked == nil || p.Soaked.Release.ReleaseID != goodID || string(p.Soaked.Record) != string(goodRaw) || string(p.Soaked.TreePatch) != string(tree) {
		t.Fatalf("U2 soaked = %+v", p.Soaked)
	}
}

// PackageUpdatesAsked is what corpusd's update read is asked.
type PackageUpdatesAsked struct {
	OwnerID  string `json:"owner_id"`
	Installs []struct {
		Package   string `json:"package"`
		Track     string `json:"track"`
		ReleaseID string `json:"release_id"`
	} `json:"installs"`
}
