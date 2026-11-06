package pkgrelease

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
)

// MA2.3 failure modes at the release seam (the manifest classes are pinned in
// internal/apppackage):
//
//	R1  a publisher's manifest states origin, level or a hash, and the release
//	    is accepted (package_publisher_declared).
//	R2  a published manifest asks for a privileged grant (package_privileged_grant;
//	    ui.shell is tier T2: package_platform_review_required, L3).
//	R3  a published manifest carries a desk before the Phase 6 sandbox
//	    (package_desk_needs_sandbox).
//	R4  a record claims a level the platform refuses (privileged, extended
//	    before the review pipeline), a class that does not match its abi and
//	    desk count, or no behaviour surface.
//	R5  a v1 record (no schema, no surface) still validates, or is refused with
//	    a generic code the catalog cannot count (package_manifest_v1_deleted).
//	R6  two checks of the same bytes give two behaviour surfaces.

var appSource = apppackage.MapSource(map[string]string{"App.svelte": "<p>x</p>"})

func readerManifest(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(testManifest), &v); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(v)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCheckManifestSummaryIsDerivedAndStable(t *testing.T) {
	raw := readerManifest(t, nil)
	a, version, err := CheckManifest("reader", raw, appSource)
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := CheckManifest("reader", raw, appSource)
	if a != b || len(a.BehaviourSurface) != 64 || a.Schema != "choir.package/v2" || a.Class != "app" || a.Level != "default" || a.DesksCount != 0 || version != "1.0.1" && version != "1.0.0" {
		t.Fatalf("summary = %+v version %s", a, version)
	}
}

func TestCheckManifestRefusalReasons(t *testing.T) {
	deskPkg := func(v map[string]any) {
		v["contributes"] = map[string]any{"desks": []any{map[string]any{"name": "reader", "prompt": "p.md"}}}
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		reason string
	}{
		{"declared origin", func(v map[string]any) { v["origin"] = "first_party" }, ReasonPublisherDeclared},
		{"declared level", func(v map[string]any) { v["level"] = "default" }, ReasonPublisherDeclared},
		{"declared hash", func(v map[string]any) { v["behaviour_surface"] = strings.Repeat("a", 64) }, ReasonPublisherDeclared},
		{"T2 grant", func(v map[string]any) { v["capabilities"] = []any{map[string]any{"grant": "ui.shell"}} }, ReasonPlatformReviewRequired},
		{"desk", deskPkg, ReasonDeskNeedsSandbox},
		{"desk with privileged grant", func(v map[string]any) {
			deskPkg(v)
			v["capabilities"] = []any{map[string]any{"grant": "desk.capsule.mutate"}}
		}, ReasonPrivilegedGrant},
		{"unknown grant", func(v map[string]any) { v["capabilities"] = []any{map[string]any{"grant": "ui.root"}} }, ReasonManifestInvalid},
		{"v1 body", func(v map[string]any) { v["schema"] = "choir.app-package/v1" }, ReasonManifestInvalid},
		{"candidate suffix", func(v map[string]any) { v["name"] = "reader-candidate" }, ReasonManifestInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name := "reader"
			if c.name == "candidate suffix" {
				name = "reader-candidate"
			}
			_, _, err := CheckManifest(name, readerManifest(t, c.mutate), appSource)
			if got := ReasonOf(err); got != c.reason {
				t.Fatalf("reason = %q (%v), want %q", got, err, c.reason)
			}
		})
	}
	// An extended grant is accepted by the manifest and carried as the level.
	ext, _, err := CheckManifest("reader", readerManifest(t, func(v map[string]any) {
		v["capabilities"] = []any{map[string]any{"grant": "ui.net.egress", "args": []any{"api.example.com"}}}
	}), appSource)
	if err != nil || ext.Level != "extended" {
		t.Fatalf("extended: %+v %v", ext, err)
	}
	base, _, _ := CheckManifest("reader", readerManifest(t, nil), appSource)
	if ext.BehaviourSurface == base.BehaviourSurface {
		t.Fatal("an added capability did not move the behaviour surface")
	}
}

func TestRecordRefusesWhatTheManifestSummaryMayNotClaim(t *testing.T) {
	mod := func(f func(*Manifest)) error {
		r := mediaRecord()
		f(&r.Manifest)
		return r.Validate()
	}
	if err := mod(func(*Manifest) {}); err != nil {
		t.Fatal(err)
	}
	// MA7.5: an extended release is a valid record; its install waits on a
	// sealed install review verdict on the computer (internal/installreview).
	if err := mod(func(m *Manifest) { m.Level = "extended" }); err != nil {
		t.Fatalf("extended level: %v", err)
	}
	cases := map[string]struct {
		f      func(*Manifest)
		reason string
	}{
		"v1 summary":         {func(m *Manifest) { m.Schema, m.BehaviourSurface, m.Class, m.Level = "", "", "", "" }, ReasonManifestV1},
		"no surface":         {func(m *Manifest) { m.BehaviourSurface = "" }, ReasonManifestV1},
		"bad surface":        {func(m *Manifest) { m.BehaviourSurface = "xyz" }, ReasonInvalid},
		"old schema":         {func(m *Manifest) { m.Schema = "choir.app-package/v1" }, ReasonManifestInvalid},
		"privileged level":   {func(m *Manifest) { m.Level = "privileged" }, ReasonPrivilegedGrant},
		"unknown level":      {func(m *Manifest) { m.Level = "root" }, ReasonInvalid},
		"unknown class":      {func(m *Manifest) { m.Class = "binary" }, ReasonInvalid},
		"app without abi":    {func(m *Manifest) { m.ABI = "" }, ReasonInvalid},
		"desk class":         {func(m *Manifest) { m.Class, m.ABI, m.DesksCount = "desk", "", 1 }, ReasonDeskNeedsSandbox},
		"desks, class app":   {func(m *Manifest) { m.DesksCount = 1 }, ReasonInvalid},
		"negative count":     {func(m *Manifest) { m.KindsCount = -1 }, ReasonInvalid},
		"data class has abi": {func(m *Manifest) { m.Class = "data" }, ReasonInvalid},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ReasonOf(mod(c.f)); got != c.reason {
				t.Fatalf("reason = %q, want %q", got, c.reason)
			}
		})
	}
}
