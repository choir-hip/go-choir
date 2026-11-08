package yaegikernel

import (
	"strings"
	"testing"
)

// The install review verbs (desk-install-review-2026-11-08 2.3). FM3: a desk
// other than management never holds them.
func TestInstallVerbsAreManagementOnly(t *testing.T) {
	for _, role := range []string{"management", "research", "texture", SessionRoleManagementTask, SessionRoleResearchWeb, SessionRoleResearchTexture, "engineering"} {
		surface := DeskSurface(role, "", false)
		for _, v := range []string{"InstallPacket", "InstallVerdict"} {
			has := strings.Contains(surface, "choir."+v+"(")
			if want := role == "management"; has != want {
				t.Errorf("role %s: choir.%s on surface = %v, want %v", role, v, has, want)
			}
		}
	}
}

func TestInstallVerdictStagesOneIntent(t *testing.T) {
	findings := []InstallFinding{{Concern: "reads notes", Evidence: "App.svelte:12"}}
	var tray Tray
	if err := tray.InstallVerdict("rel-1", 0.9, true, findings, ""); err != nil {
		t.Fatal(err)
	}
	if err := tray.InstallVerdict("rel-1", 0.9, true, findings, ""); err == nil {
		t.Error("a second install verdict in one cell must be refused")
	}
	got := tray.Drain()
	if len(got) != 1 || got[0].Kind != IntentInstallVerdict || got[0].TargetRef != "rel-1" || got[0].Decision != "accept" || got[0].PWithinCeiling != 0.9 || len(got[0].InstallFindings) != 1 {
		t.Fatalf("staged = %+v", got)
	}
	var bad Tray
	if err := bad.InstallVerdict("rel-1", 1.5, true, nil, ""); err == nil {
		t.Error("a probability over 1 must be refused")
	}
	if err := bad.InstallVerdict("rel-1", -0.1, true, nil, ""); err == nil {
		t.Error("a negative probability must be refused")
	}
	if err := bad.InstallVerdict("rel-1", 0.2, false, nil, "  "); err == nil {
		t.Error("a reject needs a reason")
	}
	if err := bad.InstallVerdict("", 0.2, true, nil, ""); err == nil {
		t.Error("a release is required")
	}
}

// The lesson of apply-proof-5 (ShotFinding as a host struct was unconstructable
// in a cell): a cell passes install findings as plain maps, and a typo is
// refused by name.
func TestCellBuildsInstallFindingsAsMaps(t *testing.T) {
	out, staged := managementVerifyCell(t, `fmt.Println(choir.InstallVerdict("rel-1", 0.8, false, []map[string]any{
	{"concern": "posts notes to api.example.com", "evidence": "App.svelte line 12", "blocking": true},
	{"Concern": "reads settings"},
}, "the notes reach a publisher sink"))`)
	if out.Error != "" || len(staged) != 1 || staged[0].Kind != IntentInstallVerdict || len(staged[0].InstallFindings) != 2 {
		t.Fatalf("error %q, stdout %q, staged %+v", out.Error, out.Stdout, staged)
	}
	f := staged[0].InstallFindings[0]
	if f.Concern == "" || f.Evidence != "App.svelte line 12" || !f.Blocking || staged[0].Decision != "reject" || staged[0].PWithinCeiling != 0.8 {
		t.Fatalf("finding = %+v, intent %+v", f, staged[0])
	}
	out, staged = managementVerifyCell(t, `fmt.Println(choir.InstallVerdict("rel-1", 0.8, true, []map[string]any{{"concern": "x", "block": true}}, ""))`)
	if len(staged) != 0 || !strings.Contains(out.Stdout, `unknown key "block"`) {
		t.Fatalf("typo: error %q, stdout %q, staged %+v", out.Error, out.Stdout, staged)
	}
	out, staged = managementVerifyCell(t, `fmt.Println(choir.InstallVerdict("rel-1", 0.8, true, []map[string]any{{"concern": "x", "blocking": "yes"}}, ""))`)
	if len(staged) != 0 || !strings.Contains(out.Stdout, "must be a bool") {
		t.Fatalf("type: error %q, stdout %q, staged %+v", out.Error, out.Stdout, staged)
	}
}
