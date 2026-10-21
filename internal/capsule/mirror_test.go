package capsule

import (
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/mirrorgrant"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Station FP10 (docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md),
// capsule side. Failure modes pinned here:
//   F12 a computer without the grant gets a route, a client setting or a
//       narrower allowlist (its egress must be exactly what it was);
//   F13 mirrors only removes a host the mirrors do not replace, or leaves
//       one they do;
//   F14 a client setting for an ecosystem nobody granted reaches a capsule,
//       or a setting the cell worker does not pass through;
//   F15 the capsule names the route's address, or the route carries a
//       CONNECT or an upload.

var mirrorRoute = netip.MustParseAddrPort("10.200.0.1:8089")

func granted(only bool, ecos ...string) mirrorgrant.Status {
	return mirrorgrant.Status{Granted: true, Ecosystems: ecos, MirrorsOnly: only}
}

func TestMirrorsLeaveAnUngrantedComputerAsItWas(t *testing.T) {
	base := EcosystemEgressPolicy()
	for _, st := range []mirrorgrant.Status{{}, {Granted: false, Ecosystems: []string{"npm"}, MirrorsOnly: true}} {
		got := applyCapsuleMirrors(EcosystemEgressPolicy(), st, mirrorRoute, true)
		if !reflect.DeepEqual(got.Hosts, base.Hosts) || len(got.Routes) != 0 {
			t.Fatalf("F12: %+v changed the policy: hosts %v routes %v", st, got.Hosts, got.Routes)
		}
	}
	// Granted but with no address to route to: nothing changes either.
	got := applyCapsuleMirrors(EcosystemEgressPolicy(), granted(true, "npm"), netip.AddrPort{}, true)
	if !reflect.DeepEqual(got.Hosts, base.Hosts) || len(got.Routes) != 0 {
		t.Fatalf("F12: a grant without a route address changed the policy: %v %v", got.Hosts, got.Routes)
	}
}

func TestMirrorRouteWithoutMirrorsOnlyKeepsTheAllowlist(t *testing.T) {
	got := applyCapsuleMirrors(EcosystemEgressPolicy(), granted(false, "npm", "pypi"), mirrorRoute, true)
	if !reflect.DeepEqual(got.Hosts, EcosystemEgressPolicy().Hosts) {
		t.Fatalf("hosts changed without mirrors only: %v", got.Hosts)
	}
	if got.Routes[MirrorRouteHost] != mirrorRoute {
		t.Fatalf("routes = %v", got.Routes)
	}
}

func TestMirrorsOnlyRemovesExactlyTheMirroredHosts(t *testing.T) {
	all := applyCapsuleMirrors(EcosystemEgressPolicy(), granted(true, "crates", "git", "npm", "pypi"), mirrorRoute, true)
	want := []string{
		"arxiv.org", "conda.anaconda.org", "doi.org", "download.pytorch.org", "githubusercontent.com",
		"hf.co", "huggingface.co", "repo.anaconda.com", "zenodo.org",
	}
	got := append([]string(nil), all.Hosts...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("F13: mirrors only left %v, want %v", got, want)
	}
	for _, host := range []string{"registry.npmjs.org", "files.pythonhosted.org", "static.crates.io", "index.crates.io", "github.com", "codeberg.org", "proxy.golang.org", "sum.golang.org"} {
		if all.allowsHost(host) {
			t.Errorf("F13: %s still allowed under mirrors only", host)
		}
	}
	for _, host := range []string{"objects.githubusercontent.com", "huggingface.co", "download.pytorch.org"} {
		if !all.allowsHost(host) {
			t.Errorf("F13: %s, which no mirror replaces, was removed", host)
		}
	}

	// One ecosystem: only its hosts go.
	npm := applyCapsuleMirrors(EcosystemEgressPolicy(), granted(true, "npm"), mirrorRoute, false)
	if npm.allowsHost("registry.npmjs.org") || npm.allowsHost("registry.yarnpkg.com") || !npm.allowsHost("pypi.org") || !npm.allowsHost("github.com") {
		t.Fatalf("F13: npm-only mirrors left %v", npm.Hosts)
	}
	// The Go hosts go only when the Go proxy route exists.
	if !npm.allowsHost("proxy.golang.org") {
		t.Fatal("F13: Go hosts removed without a Go proxy route")
	}
}

func TestMirrorClientEnvCarriesOnlyGrantedEcosystems(t *testing.T) {
	envOf := func(v string) map[string]string {
		out := map[string]string{}
		for _, kv := range MirrorClientEnv(func(k string) (string, bool) {
			if k == MirrorBrokerEnv {
				return v, true
			}
			return "", false
		}) {
			k, val, _ := strings.Cut(kv, "=")
			out[k] = val
		}
		return out
	}
	if env := envOf(""); len(env) != 0 {
		t.Fatalf("F14: no grant set %v", env)
	}
	if env := envOf("maven"); len(env) != 0 {
		t.Fatalf("F14: an unknown ecosystem set %v", env)
	}
	npm := envOf("npm")
	if npm["npm_config_registry"] != "http://"+MirrorRouteHost+"/npm/" || npm["npm_config_replace_registry_host"] != "npmjs" {
		t.Fatalf("npm env = %v", npm)
	}
	for k := range npm {
		if strings.HasPrefix(k, "PIP_") || strings.HasPrefix(k, "UV_") || strings.HasPrefix(k, "GIT_") {
			t.Fatalf("F14: npm alone set %s", k)
		}
	}
	py := envOf("pypi")
	if py["PIP_INDEX_URL"] != "http://"+MirrorRouteHost+"/pypi/simple/" || py["UV_DEFAULT_INDEX"] != py["PIP_INDEX_URL"] || py["PIP_TRUSTED_HOST"] != MirrorRouteHost {
		t.Fatalf("pypi env = %v", py)
	}
	git := envOf("git")
	if git["GIT_CONFIG_COUNT"] != "3" {
		t.Fatalf("git env = %v", git)
	}
	rewrites := map[string]string{}
	for i := 0; i < 3; i++ {
		rewrites[git["GIT_CONFIG_VALUE_"+string(rune('0'+i))]] = git["GIT_CONFIG_KEY_"+string(rune('0'+i))]
	}
	for _, forge := range []string{"github.com", "gitlab.com", "codeberg.org"} {
		if rewrites["https://"+forge+"/"] != "url.http://"+MirrorRouteHost+"/git/"+forge+"/.insteadOf" {
			t.Fatalf("git rewrite for %s = %q", forge, rewrites["https://"+forge+"/"])
		}
	}
	// Every key reaches a cell's commands through the worker's passthrough,
	// and none collides with the proxy settings.
	for k := range envOf("crates,git,npm,pypi") {
		if !slices.Contains(yaegikernel.MirrorClientKeys, k) {
			t.Errorf("F14: %s is not in yaegikernel.MirrorClientKeys", k)
		}
		if slices.Contains(yaegikernel.EgressProxyKeys, k) {
			t.Errorf("%s collides with an egress proxy key", k)
		}
	}
}

func TestEgressRoutesTheMirrorAndRefusesMirroredRegistriesUnderMirrorsOnly(t *testing.T) {
	policy := applyCapsuleMirrors(EcosystemEgressPolicy(), granted(true, "npm"), mirrorRoute, false)
	h := newEgressHarness(t, policy)
	h.resolved["registry.npmjs.org"] = []netip.Addr{publicAddr}
	h.resolved["pypi.org"] = []netip.Addr{publicAddr}

	status, body := sendRaw(t, h.addr, "GET http://mirror.choir.internal/npm/left-pad HTTP/1.1\r\nHost: mirror.choir.internal\r\nX-Choir-Capsule: forged\r\n\r\n")
	if status != http.StatusOK || !strings.Contains(body, "GET /npm/left-pad host=mirror.choir.internal capsule=capsule-test") {
		t.Fatalf("routed GET = %d %q", status, body)
	}
	if d := h.dials(); len(d) != 1 || d[0] != mirrorRoute.String() {
		t.Fatalf("F15: dialed %v, want only %s", d, mirrorRoute)
	}
	if _, _, resp := h.connect("registry.npmjs.org:443"); resp.StatusCode == http.StatusOK {
		t.Fatal("F13: CONNECT to the mirrored registry passed under mirrors only")
	}
	if _, _, resp := h.connect("pypi.org:443"); resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT to a registry not mirrored = %d", resp.StatusCode)
	}
	for _, raw := range []string{
		"CONNECT mirror.choir.internal:443 HTTP/1.1\r\nHost: mirror.choir.internal:443\r\n\r\n",
		"POST http://mirror.choir.internal/git/github.com/a/b/git-upload-pack HTTP/1.1\r\nHost: mirror.choir.internal\r\nContent-Length: 4\r\n\r\ndata",
		"GET http://10.200.0.1:8089/npm/left-pad HTTP/1.1\r\nHost: 10.200.0.1:8089\r\n\r\n",
	} {
		if status, body := sendRaw(t, h.addr, raw); status == http.StatusOK {
			t.Fatalf("F15: allowed %q: %s", strings.SplitN(raw, "\r\n", 2)[0], body)
		}
	}

	// Without the route, the name goes nowhere and the reason says why.
	bare := newEgressHarness(t, EcosystemEgressPolicy())
	if status, _ := sendRaw(t, bare.addr, "GET http://mirror.choir.internal/npm/x HTTP/1.1\r\nHost: mirror.choir.internal\r\n\r\n"); status == http.StatusOK {
		t.Fatal("F12: the mirror name resolved without a route")
	}
	if r := bare.waitRecords(1); len(r) != 1 || !strings.Contains(r[0].Reason, "package mirror") || len(bare.dials()) != 0 {
		t.Fatalf("missing-route refusal: %+v dials %v", r, bare.dials())
	}
}

func TestExecutorAppliesTheMirrorGrantToL2CapsulesOnly(t *testing.T) {
	dir := t.TempDir()
	e := &Executor{goModuleProxy: goProxyRoute}
	e.SetCapsuleMirrors(mirrorRoute, dir)
	if st := e.CapsuleMirrorStatus(); st.Granted {
		t.Fatalf("F12: granted with no grant file: %+v", st)
	}
	if p := e.egressPolicyLocked(false, e.capsuleMirrorsLocked()); len(p.Routes) != 1 || !p.allowsHost("registry.npmjs.org") {
		t.Fatalf("F12: ungranted policy = %v %v", p.Routes, p.Hosts)
	}
	if err := os.WriteFile(filepath.Join(dir, mirrorgrant.GrantFile), []byte(`{"computer_id":"c","status":"granted","basis":"computer","ecosystems":["npm","git"],"mirrors_only":true}`), 0o400); err != nil {
		t.Fatal(err)
	}
	st := e.capsuleMirrorsLocked()
	if !st.Granted || !reflect.DeepEqual(st.Ecosystems, []string{"git", "npm"}) {
		t.Fatalf("status = %+v", st)
	}
	p := e.egressPolicyLocked(false, st)
	if p.Routes[MirrorRouteHost] != mirrorRoute || p.Routes[GoProxyRouteHost] != goProxyRoute || p.allowsHost("registry.npmjs.org") || p.allowsHost("github.com") || p.allowsHost("proxy.golang.org") {
		t.Fatalf("granted policy = %v %v", p.Routes, p.Hosts)
	}
	// The Claude delegate's model-API-only policy never takes the mirror.
	if c := e.egressPolicyLocked(true, st); c.Routes[MirrorRouteHost].IsValid() || !reflect.DeepEqual(c.Hosts, ClaudeEgressPolicy().Hosts) {
		t.Fatalf("claude policy = %v %v", c.Routes, c.Hosts)
	}
	// No route address (no gateway peer): the grant is not in force.
	e.SetCapsuleMirrors(netip.AddrPort{}, dir)
	if st := e.capsuleMirrorsLocked(); st.Granted {
		t.Fatalf("F12: granted without a route address: %+v", st)
	}
}

func TestMirrorAddr(t *testing.T) {
	cases := []struct {
		explicit, gateway, want string
		ok, err                 bool
	}{
		{"", "http://10.200.3.1:8084", "10.200.3.1:8089", true, false},
		{"10.0.0.5:9000", "http://10.200.3.1:8084", "10.0.0.5:9000", true, false},
		{"", "", "", false, false},
		{"", "http://gateway.local:8084", "", false, false},
		{"not-an-addr", "", "", false, true},
	}
	for _, tc := range cases {
		got, ok, err := MirrorAddr(tc.explicit, tc.gateway)
		if ok != tc.ok || (err != nil) != tc.err || (ok && got.String() != tc.want) {
			t.Errorf("MirrorAddr(%q, %q) = %v %v %v", tc.explicit, tc.gateway, got, ok, err)
		}
	}
}
