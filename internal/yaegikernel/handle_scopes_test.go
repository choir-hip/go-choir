package yaegikernel

import (
	"crypto/rand"
	"sort"
	"testing"
)

func newTestBroker(t *testing.T) (*Broker, *HandleIssuer) {
	t.Helper()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "c1", CurrentEpoch: 1, AllowedRoot: t.TempDir()}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	return broker, issuer
}

// SEC-02 failure mode 5: ActionExec and ActionWriteFile leave every handle but
// engineering's; ActionMessage and ActionEmit (retired verbs) leave all of
// them. The handle scopes below are what a worker's own broker will honor.
//
// Written independently of internal/deskmanifest: this table is the authority
// the batch must not widen, and it is the narrowing it applies.
var pinnedHandleScopes = map[string][]BrokerAction{
	"management":              {ActionReadFile, ActionListDir, ActionProductAPI, ActionReadCapture, ActionStories},
	"texture":                 {ActionReadFile, ActionListDir, ActionReadCapture},
	"research":                append([]BrokerAction{ActionReadFile, ActionListDir}, pinnedFrameActions["research"]...),
	SessionRoleResearchChild:  append([]BrokerAction{ActionReadFile, ActionListDir}, pinnedFrameActions[SessionRoleResearchChild]...),
	SessionRoleManagementTask: {ActionReadFile, ActionListDir, ActionReadCapture, ActionStories},
	"engineering":             {ActionExec, ActionReadFile, ActionWriteFile, ActionListDir},
	"":                        {ActionReadFile, ActionListDir},
	"not-a-desk":              {ActionReadFile, ActionListDir},
}

func actionNames(in []BrokerAction) []string {
	out := make([]string, len(in))
	for i, a := range in {
		out[i] = string(a)
	}
	sort.Strings(out)
	return out
}

func TestHandleScopesAreExactlyTheDesksTable(t *testing.T) {
	for role, want := range pinnedHandleScopes {
		role, want := role, want
		t.Run("role="+role, func(t *testing.T) {
			broker, issuer := newTestBroker(t)
			scope, err := NewChoirScope(broker, issuer, "c1", "act1", 1, role, "")
			if err != nil {
				t.Fatal(err)
			}
			got, w := actionNames(scope.actions), actionNames(want)
			if len(got) != len(w) {
				t.Fatalf("role %q handle scopes = %v, want %v", role, got, w)
			}
			for i := range got {
				if got[i] != w[i] {
					t.Fatalf("role %q handle scopes = %v, want %v", role, got, w)
				}
			}
		})
	}
}

// verbDispatch is what each choir verb sends to the host or its worker
// broker, read from the s.call sites in choir.go, search_store.go and
// system1.go. Verbs absent here stage tray intents or read the cell frame and
// need no action.
var verbDispatch = map[string]BrokerAction{
	"ReadFile": ActionReadFile, "ListDir": ActionListDir, "WriteFile": ActionWriteFile, "Exec": ActionExec,
	"SearchStart": ActionSearchStart, "Read": ActionRead, "PollSource": ActionPollSource, "SourceAPI": ActionSourceAPI,
	"ReadCapture": ActionReadCapture, "SearchWireCorpus": ActionSearchWireCorpus, "SaveEvidence": ActionSaveEvidence,
	"ReadEvidence": ActionReadEvidence, "ListEvidence": ActionListEvidence, "RunMemoryEntry": ActionGetRunMemoryEntry,
	"ProductAPI": ActionProductAPI, "Stories": ActionStories, "Research": ActionResearch, "System1": ActionSystem1,
	"RelateStories": ActionRelateStories,
}

// The table forgot nothing a desk's verbs dispatch (failure mode F8: a real
// turn refused), and holds nothing no verb of the desk dispatches.
func TestFrameTableIsWhatTheDesksVerbsDispatch(t *testing.T) {
	for role, table := range pinnedFrameActions {
		want := map[BrokerAction]bool{}
		for _, verb := range deskModuleSets[role] {
			if a, ok := verbDispatch[verb]; ok && a != ActionExec && a != ActionWriteFile && a != ActionReadFile && a != ActionListDir {
				want[a] = true
			}
		}
		got := map[BrokerAction]bool{}
		for _, a := range table {
			got[a] = true
		}
		for a := range want {
			if !got[a] {
				t.Errorf("desk %q: verb dispatches %s but the frame table omits it (a real turn would be refused)", role, a)
			}
		}
		for a := range got {
			if !want[a] {
				t.Errorf("desk %q: frame table holds %s but no verb of the desk dispatches it", role, a)
			}
		}
	}
}
