package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"
)

// The shot verbs are management's alone (desk-shots-and-review §3.3).
func TestShotVerbsAreManagementOnly(t *testing.T) {
	verbs := []string{"ShotStart", "ShotWait", "ShotVerdict", "Screens", "VerifyFindings"}
	for _, role := range []string{"management", "research", "texture", SessionRoleManagementTask, SessionRoleResearchWeb, SessionRoleResearchTexture, "engineering"} {
		surface := DeskSurface(role, "", false)
		for _, v := range verbs {
			has := strings.Contains(surface, "choir."+v+"(")
			if want := role == "management"; has != want {
				t.Errorf("role %s: choir.%s on surface = %v, want %v", role, v, has, want)
			}
		}
	}
}

func TestShotVerdictStagesOneIntent(t *testing.T) {
	findings := []ShotFinding{{Pair: "01", BeforeSHA256: "a", AfterSHA256: "b", Answers: map[string]string{"q1": "yes"}}}
	var tray Tray
	if err := tray.ShotVerdict("op-1", "pass", findings, ""); err != nil {
		t.Fatal(err)
	}
	if err := tray.ShotVerdict("op-1", "pass", findings, ""); err == nil {
		t.Error("a second shot verdict in one cell must be refused")
	}
	got := tray.Drain()
	if len(got) != 1 || got[0].Kind != IntentShotVerdict || got[0].TargetRef != "op-1" || got[0].Decision != "pass" || len(got[0].ShotFindings) != 1 {
		t.Fatalf("staged = %+v", got)
	}
	var bad Tray
	if err := bad.ShotVerdict("op-1", "maybe", findings, ""); err == nil {
		t.Error("verdict other than pass/fail must be refused")
	}
	if err := bad.ShotVerdict("op-1", "fail", findings, "  "); err == nil {
		t.Error("a fail needs a reason")
	}
	if err := bad.ShotVerdict("", "pass", findings, ""); err == nil {
		t.Error("an operation id is required")
	}
}

func TestShotPayloadBounds(t *testing.T) {
	if err := ValidatePayload(ActionShotWait, json.RawMessage(`{"job_id":"j","max_seconds":241}`)); err == nil {
		t.Error("max_seconds over 240 must be refused")
	}
	if err := ValidatePayload(ActionShotWait, json.RawMessage(`{"job_id":"j","max_seconds":240}`)); err != nil {
		t.Error(err)
	}
	if err := ValidatePayload(ActionShotStart, json.RawMessage(`{"operation_id":"`+strings.Repeat("x", 129)+`"}`)); err == nil {
		t.Error("operation_id over 128 must be refused")
	}
}

func TestVerifyFindingsAttachToVerify(t *testing.T) {
	f := []ShotFinding{{Pair: "01", Answers: map[string]string{"q1": "yes"}}}
	var tray Tray
	if err := tray.VerifyFindings(f); err == nil {
		t.Error("findings without a Verify must be refused")
	}
	if err := tray.Verify("a-1", "pass", []string{"r"}, "sha256:abc"); err != nil {
		t.Fatal(err)
	}
	if err := tray.VerifyFindings(f); err != nil {
		t.Fatal(err)
	}
	if err := tray.VerifyFindings(f); err == nil {
		t.Error("a second VerifyFindings must be refused")
	}
	got := tray.Drain()
	if len(got) != 1 || len(got[0].ShotFindings) != 1 {
		t.Fatalf("staged = %+v", got)
	}
}
