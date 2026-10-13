package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A desk session lives as long as its desk, but its broker handle was issued
// once, at worker start, for an hour; every choir.* verb verifies it, so a
// desk older than the handle lost its verbs (Cut 0 artifact 6). The handle
// is reissued when a cell is bound.
//
// Ways this could fail, written first: the lifetime the test sets never
// elapses, so the test passes on the old code; the reissue happens only at
// scope construction; the reissued handle drops the role's actions; a cell
// started before expiry still fails because the handle is not refreshed at
// Begin.
func TestDeskHandleOutlivesItsFirstIssue(t *testing.T) {
	old := sessionHandleTTL
	sessionHandleTTL = 200 * time.Millisecond
	t.Cleanup(func() { sessionHandleTTL = old })

	issuer, err := NewHandleIssuer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-ttl", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"results":[]}`), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-ttl", "activation-ttl", 1, SessionRoleResearchChild, "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	cell := func() SessionResult {
		t.Helper()
		out, err := serveCell(sess, SessionFrame{ID: "cell", Source: `_, err := choir.WebSearch("q", 1)
if err != nil {
	panic(err)
}`}, nil, &hooks)
		if err != nil {
			t.Fatalf("serve cell: %v", err)
		}
		return out
	}
	if out := cell(); out.Error != "" {
		t.Fatalf("first cell: %s", out.Error)
	}
	time.Sleep(3 * sessionHandleTTL)
	out := cell()
	if strings.Contains(out.Error, "expired") {
		t.Fatalf("a cell after the first handle's lifetime lost its verbs: %s", out.Error)
	}
	if out.Error != "" {
		t.Fatalf("second cell: %s", out.Error)
	}
}
