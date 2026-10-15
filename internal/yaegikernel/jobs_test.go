package yaegikernel

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

func engineeringJobSession(t *testing.T) (*Session, *ChoirScope) {
	t.Helper()
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-eng", 1, "engineering", "implementation")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return sess, scope
}

// A cell starts a job: choir.Start stages one job intent with the id it
// returns, the default cwd and wall clock, and Poll shows it as starting;
// Poll reads a job from the cell-start snapshot; Stop stages a stop.
func TestCellStartsPollsAndStopsAJob(t *testing.T) {
	sess, scope := engineeringJobSession(t)
	hooks := scope.BindCell()
	code := 0
	frame := SessionFrame{ID: "cell", Jobs: []JobView{{ID: "job-earlier", State: capsulejob.StateDone, ExitCode: &code, ReceiptRef: "capsule-exec:sha256:ab", Tail: "PASS"}},
		Source: `id, err := choir.Start([]string{"go", "build", "./cmd/autoputer"}, nil)
fmt.Println("start", id, err)
st, err := choir.Poll(id)
fmt.Println("poll", st.State, st.Cwd, st.WallClockSec, err)
old, err := choir.Poll("job-earlier")
fmt.Println("old", old.State, old.ReceiptRef, old.Tail, err)
fmt.Println("stop", choir.Stop("job-earlier"))
_, err = choir.Poll("job-nope")
fmt.Println("missing", err)`}
	out, err := serveCell(sess, frame, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell: %v %s", err, out.Error)
	}
	for _, want := range []string{"poll starting /workspace/platform 1200 <nil>", "old done capsule-exec:sha256:ab PASS <nil>", "stop <nil>", `missing choir.Poll: no job "job-nope"; jobs: job-earlier (done)`} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout %q lacks %q", out.Stdout, want)
		}
	}
	if len(out.Intents) != 2 {
		t.Fatalf("intents: %+v", out.Intents)
	}
	start, stop := out.Intents[0], out.Intents[1]
	if start.Kind != IntentJob || start.JobOp != JobOpStart || !capsulejob.ValidID(start.JobID) || !strings.Contains(out.Stdout, "start "+start.JobID+" <nil>") ||
		strings.Join(start.Argv, " ") != "go build ./cmd/autoputer" || start.Cwd != capsulejob.DefaultCwd || start.WallClockMS != capsulejob.DefaultWallClock.Milliseconds() {
		t.Fatalf("start intent: %+v", start)
	}
	if stop.Kind != IntentJob || stop.JobOp != JobOpStop || stop.JobID != "job-earlier" {
		t.Fatalf("stop intent: %+v", stop)
	}
}

// Start's bounds, refused in the cell where the model can fix them.
func TestJobStartRefusals(t *testing.T) {
	_, scope := engineeringJobSession(t)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{ID: "cell"})
	for _, tc := range []struct {
		argv []string
		opts map[string]any
		want string
	}{
		{nil, nil, "needs a command"},
		{[]string{"go"}, map[string]any{"minutes": 90}, "wall clock"},
		{[]string{"go"}, map[string]any{"cwd": "workspace"}, "absolute"},
		{[]string{"go"}, map[string]any{"dir": "/x"}, `no option "dir"`},
		{[]string{"go"}, map[string]any{"minutes": "ten"}, "must be a number"},
	} {
		if _, err := scope.Start(tc.argv, tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Start(%v, %v) = %v, want %q", tc.argv, tc.opts, err, tc.want)
		}
	}
	id, err := scope.Start([]string{"go", "test", "./..."}, map[string]any{"minutes": 2.5, "cwd": "/workspace/platform/frontend"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Start([]string{"true"}, nil); err == nil || !strings.Contains(err.Error(), "one job start per cell") {
		t.Fatalf("second start: %v", err)
	}
	if err := scope.Freeze("a", []string{"b"}, []string{"c"}); err == nil || !strings.Contains(err.Error(), "job") {
		t.Fatalf("Freeze beside a job start: %v", err)
	}
	if err := scope.Complete("completed", "none", "s", nil, []string{"x"}); err == nil {
		t.Fatal("Complete beside a job start")
	}
	in := hooks.End()
	if len(in) != 1 || in[0].JobID != id || in[0].WallClockMS != 150000 || in[0].Cwd != "/workspace/platform/frontend" {
		t.Fatalf("intents: %+v", in)
	}

	hooks.Begin(SessionFrame{ID: "cell-2"})
	if err := scope.Freeze("a", []string{"b"}, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Start([]string{"true"}, nil); err == nil || !strings.Contains(err.Error(), "freeze") {
		t.Fatalf("Start beside Freeze: %v", err)
	}
	hooks.End()
}

// Jobs are engineering's: no other desk's choir package has them.
func TestJobsAreEngineeringOnly(t *testing.T) {
	broker, issuer, _, _ := testChoirFixture(t)
	for _, role := range []string{"engineering", "management", "research", "texture"} {
		scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-"+role, 1, role, "")
		if err != nil {
			t.Fatal(err)
		}
		exports := scope.ChoirExports()["choir/choir"]
		for _, verb := range []string{"Start", "Poll", "Stop"} {
			if _, ok := exports[verb]; ok != (role == "engineering") {
				t.Fatalf("%s exports %s: %v", role, verb, ok)
			}
		}
	}
}
