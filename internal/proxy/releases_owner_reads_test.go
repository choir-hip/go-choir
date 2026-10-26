package proxy

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Library S1 at the proxy (docs/design/library-package-propagation-2026-10-26.md
// 4.3, docs/problems/release-reads-not-owner-scoped-2026-10-26.md). Corpusd
// holds the policy (CHOIR_RELEASE_READS); the proxy's part is to name the
// authenticated owner on every release read, and to take that name from
// nowhere else. Failure modes pinned, written before the code:
//
//	P1 the list is relayed with no owner, so corpusd cannot scope it;
//	P2 the single read is relayed as the host with no owner;
//	P3 a caller names an owner of its own in the query (owner_id) and the
//	   proxy forwards it, widening the read to another owner's releases;
//	P4 a second owner's key is forwarded as the first owner;
//	P5 an API key without the read scope reads anyway (unchanged: 403);
//	P6 a release id that is not a sha256 reaches corpusd (unchanged: 404);
//	P7 an admin key of owner B reads as someone other than B: the admin scope
//	   passes the scope check only; it names no other owner, so private means
//	   private even to admin.

func (e *releaseProxyEnv) corpusdReads() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, c := range e.calls {
		if strings.HasPrefix(c, "corpusd GET /internal/platform/releases") {
			out = append(out, c)
		}
	}
	return out
}

func (e *releaseProxyEnv) secretFor(t *testing.T, userID string, scopes ...string) string {
	t.Helper()
	_, secret, err := e.store.CreateComputerScopedAPIKey(context.Background(), userID, "owner-reads-key", scopes, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

// upstreamOwner returns the single owner_id the proxy sent corpusd on a
// recorded read, and whether it sent exactly one.
func upstreamOwner(t *testing.T, call string) (string, bool) {
	t.Helper()
	_, after, found := strings.Cut(call, " query=")
	if !found {
		t.Fatalf("call %q records no query", call)
	}
	raw, _, _ := strings.Cut(after, " ")
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	owners := values["owner_id"]
	if len(owners) != 1 {
		return "", false
	}
	return owners[0], true
}

func TestReleaseReadsCarryTheAuthenticatedOwnerToCorpusd(t *testing.T) {
	env := newReleaseProxyEnv(t)
	env.draft = func() []byte { return []byte(`{}`) }
	other, err := env.store.CreateUser("other-owner", "other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, owner string
	}{{"first owner", env.user}, {"second owner", other.ID}} {
		secret := env.secretFor(t, tc.owner, "computer:self_development:read")
		env.calls = nil
		// P1, P4: the list names the authenticated owner.
		if rec := env.send(t, http.MethodGet, "/api/releases?limit=5", "", secret); rec.Code != http.StatusOK {
			t.Fatalf("%s list = %d %s", tc.name, rec.Code, rec.Body.String())
		}
		// P2, P4: so does the single read.
		if rec := env.send(t, http.MethodGet, "/api/releases/"+id, "", secret); rec.Code != http.StatusOK {
			t.Fatalf("%s show = %d %s", tc.name, rec.Code, rec.Body.String())
		}
		reads := env.corpusdReads()
		if len(reads) != 2 {
			t.Fatalf("%s: corpusd reads = %v, want a list and a single read", tc.name, reads)
		}
		for _, call := range reads {
			if owner, ok := upstreamOwner(t, call); !ok || owner != tc.owner {
				t.Fatalf("%s: upstream %q names owner %q (sent=%v), want %q", tc.name, call, owner, ok, tc.owner)
			}
		}
	}
}

func TestReleaseReadsIgnoreAnOwnerTheCallerNames(t *testing.T) {
	env := newReleaseProxyEnv(t)
	env.draft = func() []byte { return []byte(`{}`) }
	secret := env.secret(t, "computer:self_development:read")
	id := strings.Repeat("a", 64)
	// P3: owner_id and computer_id in the caller's query are never forwarded.
	for _, path := range []string{
		"/api/releases?owner_id=someone-else&limit=5",
		"/api/releases?owner=someone-else",
		"/api/releases/" + id + "?owner_id=someone-else&computer_id=other",
	} {
		env.calls = nil
		if rec := env.send(t, http.MethodGet, path, "", secret); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body.String())
		}
		reads := env.corpusdReads()
		if len(reads) != 1 {
			t.Fatalf("%s: corpusd reads = %v", path, reads)
		}
		if owner, ok := upstreamOwner(t, reads[0]); !ok || owner != env.user {
			t.Fatalf("%s: upstream %q names owner %q, want the authenticated %q", path, reads[0], owner, env.user)
		}
		if strings.Contains(reads[0], "someone-else") || strings.Contains(reads[0], "computer_id=other") {
			t.Fatalf("%s: a caller-named scope reached corpusd: %q", path, reads[0])
		}
	}
}

func TestReleaseReadsKeepTheirScopeAndIdChecks(t *testing.T) {
	env := newReleaseProxyEnv(t)
	env.draft = func() []byte { return []byte(`{}`) }
	// P5: an API key without the read scope is refused before corpusd.
	if rec := env.send(t, http.MethodGet, "/api/releases", "", env.secret(t, "computer:self_development:approve")); rec.Code != http.StatusForbidden || len(env.corpusdReads()) != 0 {
		t.Fatalf("no read scope = %d, reads %v", rec.Code, env.corpusdReads())
	}
	// P6: a release id that is not a sha256 is a 404 before corpusd.
	if rec := env.send(t, http.MethodGet, "/api/releases/not-a-sha", "", env.secret(t, "computer:self_development:read")); rec.Code != http.StatusNotFound || len(env.corpusdReads()) != 0 {
		t.Fatalf("bad id = %d, reads %v", rec.Code, env.corpusdReads())
	}
}

func TestAdminKeyReadsOnlyItsOwnersReleases(t *testing.T) {
	env := newReleaseProxyEnv(t)
	env.draft = func() []byte { return []byte(`{}`) }
	ownerB, err := env.store.CreateUser("owner-b", "b@example.com")
	if err != nil {
		t.Fatal(err)
	}
	admin := env.secretFor(t, ownerB.ID, "admin")
	id := strings.Repeat("a", 64)
	for _, path := range []string{
		"/api/releases?limit=5",
		"/api/releases?owner_id=" + env.user + "&limit=5", // naming the first owner changes nothing
		"/api/releases/" + id,
		"/api/releases/" + id + "?owner_id=" + env.user,
	} {
		env.calls = nil
		if rec := env.send(t, http.MethodGet, path, "", admin); rec.Code != http.StatusOK {
			t.Fatalf("admin %s = %d %s", path, rec.Code, rec.Body.String())
		}
		reads := env.corpusdReads()
		if len(reads) != 1 {
			t.Fatalf("%s: corpusd reads = %v", path, reads)
		}
		if owner, ok := upstreamOwner(t, reads[0]); !ok || owner != ownerB.ID {
			t.Fatalf("admin of owner B, %s: upstream %q names owner %q, want only %q", path, reads[0], owner, ownerB.ID)
		}
		if strings.Contains(reads[0], env.user) {
			t.Fatalf("admin of owner B reached corpusd naming owner A: %q", reads[0])
		}
	}
}
