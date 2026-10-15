package yaegikernel

import (
	"crypto/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

// The management task role (management delegation S2a, design round 2
// R2-1/R2-2). Ways the role can fail, written before the code:
//
//   - normalizeDeskRole does not know the role and maps it to engineering,
//     so a task gets WriteFile, Exec, Complete and Freeze (the round-2
//     discovered heresy: unknown roles fail open to engineering);
//   - NewChoirScope gives the task's handle Exec and WriteFile (every
//     non-research role had them) or ProductAPI (management's), so a cell
//     that bypassed the export map could still reach them;
//   - the task exports the parent's verbs: engineering Cast admission,
//     Verify, CancelAssignment, ProductAPI, Delegate, CancelTask, or the
//     ledger verbs (Precommit, Resolve, Escalate, Report);
//   - agentprofile.Canonical rejects the role, so resolveRunIdentity refuses
//     a task run, or the kernel and agentprofile spell it differently.
func TestManagementTaskRoleReverseParity(t *testing.T) {
	broker, issuer := choirRoleFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-task", 1, SessionRoleManagementTask, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := normalizeDeskRole(SessionRoleManagementTask); got != SessionRoleManagementTask {
		t.Fatalf("normalizeDeskRole(%q) = %q", SessionRoleManagementTask, got)
	}
	if got, err := agentprofile.Canonical(SessionRoleManagementTask); err != nil || got != agentprofile.ManagementTask || agentprofile.ManagementTask != SessionRoleManagementTask {
		t.Fatalf("agentprofile.Canonical(%q) = %q, %v; the kernel and agentprofile must spell the role alike", SessionRoleManagementTask, got, err)
	}

	exports := scope.ChoirExports()["choir/choir"]
	var names []string
	for name := range exports {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"CellOut", "Context", "Decline", "Done", "Feed", "Help", "ListDir", "Note", "Pack", "ReadCapture", "ReadFile", "Records", "Stories", "Cast"}
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("management task exports %v, want exactly %v", names, want)
	}
	for _, denied := range []string{"WriteFile", "Exec", "Complete", "Freeze", "Verify", "CancelAssignment", "ProductAPI",
		"Delegate", "CancelTask", "Precommit", "Resolve", "Disagreement", "Escalate", "Report", "ReportPacket", "Reply", "Ask", "CancelAct", "Updates"} {
		if _, ok := exports[denied]; ok {
			t.Errorf("a management task must not export choir.%s", denied)
		}
	}
	// The handle is the durable gate: even a cell that reached the method
	// past the export map is refused at the broker.
	for _, action := range scope.actions {
		switch action {
		case ActionExec, ActionWriteFile, ActionProductAPI, ActionMessage, ActionEmit, ActionResearch:
			t.Errorf("a management task's handle authorizes %s", action)
		}
	}
	hasAction := map[BrokerAction]bool{}
	for _, action := range scope.actions {
		hasAction[action] = true
	}
	for _, need := range []BrokerAction{ActionReadCapture, ActionStories} {
		if !hasAction[need] {
			t.Errorf("a management task's handle lacks %s", need)
		}
	}
	if _, err := scope.ProductAPI("GET", "/api/trajectories", nil); err == nil || !strings.Contains(err.Error(), "product_api") {
		t.Fatalf("a task's ProductAPI must fail handle verification, got %v", err)
	}
	if _, err := scope.Exec("true", nil); err == nil {
		t.Fatal("a task's Exec must fail")
	}
	if _, err := scope.WriteFile("x.txt", "x"); err == nil {
		t.Fatal("a task's WriteFile must fail")
	}

	// The parent keeps its verbs and gains Delegate and CancelTask.
	parent, err := NewChoirScope(broker, issuer, "computer-choir", "activation-parent", 1, "management", "")
	if err != nil {
		t.Fatal(err)
	}
	parentExports := parent.ChoirExports()["choir/choir"]
	for _, need := range []string{"Delegate", "CancelTask", "ProductAPI", "Verify", "Feed", "Decline", "Stories", "ReadCapture"} {
		if _, ok := parentExports[need]; !ok {
			t.Errorf("the parent management desk must export choir.%s", need)
		}
	}
	for _, other := range []string{"engineering", "texture", SessionRoleResearch, SessionRoleResearchChild} {
		s, err := NewChoirScope(broker, issuer, "computer-choir", "activation-other", 1, other, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, verb := range []string{"Delegate", "CancelTask"} {
			if _, ok := s.ChoirExports()["choir/choir"][verb]; ok {
				t.Errorf("%s must not export choir.%s", other, verb)
			}
		}
	}
}

// An unknown role fails closed (R2-2): no module verbs, and a handle with
// no filesystem, assignment or message authority. Before S2a an unknown
// role resolved to the engineering surface.
func TestUnknownDeskRoleFailsClosed(t *testing.T) {
	broker, issuer := choirRoleFixture(t)
	engineeringOnly := []string{"WriteFile", "Exec", "Complete", "Freeze"}
	for _, role := range []string{"", "co-super", "management-task", "Management_Task", "bogus"} {
		if got := normalizeDeskRole(role); got != "" {
			t.Errorf("normalizeDeskRole(%q) = %q, want no desk", role, got)
		}
		scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-unknown", 1, role, "")
		if err != nil {
			t.Fatal(err)
		}
		exports := scope.ChoirExports()["choir/choir"]
		for _, verb := range engineeringOnly {
			if _, ok := exports[verb]; ok {
				t.Errorf("unknown role %q exports engineering's choir.%s", role, verb)
			}
		}
		for name := range exports {
			switch name {
			case "ReadFile", "ListDir", "Context", "Records", "Pack", "Help", "CellOut":
			default:
				t.Errorf("unknown role %q exports choir.%s beyond observation", role, name)
			}
		}
		for _, action := range scope.actions {
			switch action {
			case ActionReadFile, ActionListDir:
			default:
				t.Errorf("unknown role %q's handle authorizes %s", role, action)
			}
		}
	}
	// Every desk the kernel knows is a role agentprofile knows, so a run
	// identity can carry it.
	for desk := range deskModuleSets {
		if desk == SessionRoleResearchChild {
			continue
		}
		if _, err := agentprofile.Canonical(desk); err != nil {
			t.Errorf("deskModuleSets has %q, which agentprofile.Canonical refuses: %v", desk, err)
		}
		if normalizeDeskRole(desk) != desk {
			t.Errorf("normalizeDeskRole(%q) = %q", desk, normalizeDeskRole(desk))
		}
	}
}

// The task's prompt teaches only what the task can call (prompt parity for
// the new role): every choir.X in its overlay is a task export, and the
// parent's verbs are not taught to it.
func TestManagementTaskPromptTeachesOnlyTaskVerbs(t *testing.T) {
	raw, err := os.ReadFile("../runtimeprompts/overlays/rlm_management_task_runtime.yaml")
	if err != nil {
		t.Fatalf("task overlay: %v", err)
	}
	broker, issuer := choirRoleFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-task", 1, SessionRoleManagementTask, "")
	if err != nil {
		t.Fatal(err)
	}
	exports := scope.ChoirExports()["choir/choir"]
	taught := map[string]bool{}
	for _, m := range regexp.MustCompile(`choir\.([A-Z][A-Za-z0-9_]*)`).FindAllStringSubmatch(string(raw), -1) {
		taught[m[1]] = true
		if _, ok := exports[m[1]]; !ok {
			t.Errorf("the task overlay teaches choir.%s, which a task cannot call", m[1])
		}
	}
	for _, verb := range []string{"Done", "Feed", "Decline", "Stories", "Records", "Cast"} {
		if !taught[verb] {
			t.Errorf("the task overlay does not teach choir.%s", verb)
		}
	}
	// The parent's prompt teaches Delegate and CancelTask, which it exports.
	parent, err := NewChoirScope(broker, issuer, "computer-choir", "activation-parent", 1, "management", "")
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := os.ReadFile("../runtimeprompts/overlays/rlm_management_runtime.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"Delegate", "CancelTask"} {
		if !strings.Contains(string(overlay), "choir."+verb+"(") {
			t.Errorf("management's overlay does not teach choir.%s", verb)
		}
		if _, ok := parent.ChoirExports()["choir/choir"][verb]; !ok {
			t.Errorf("management does not export choir.%s", verb)
		}
	}
}

func choirRoleFixture(t *testing.T) (*Broker, *HandleIssuer) {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: t.TempDir()}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	return broker, issuer
}
