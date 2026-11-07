//go:build linux

package shotsandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// D-B1. Failure modes, decided before the code:
//
//   - a thread of the stage keeps the id it started with (root of the
//     namespace) after the drop, and something it spawns runs as that id: the
//     stage reads every thread's four uids and four gids from the kernel and
//     refuses unless all are the ones asked;
//   - a refusal that still runs the servers: the stage returns a Result error
//     and starts nothing;
//   - a child of the stage starts with another id than the stage dropped to:
//     the children are started with the credential explicitly.

func fakeTasks(t *testing.T, threads map[string][2]string) {
	t.Helper()
	root := t.TempDir()
	for tid, ids := range threads {
		dir := filepath.Join(root, tid)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("Name:\tx\nUid:\t%s\nGid:\t%s\nGroups:\t\n", ids[0], ids[1])
		if err := os.WriteFile(filepath.Join(dir, "status"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := procTaskDir
	procTaskDir = root
	t.Cleanup(func() { procTaskDir = old })
}

func TestVerifyThreadIDsRequiresEveryThreadAndEveryIDToMatch(t *testing.T) {
	ok := [2]string{"65534\t65534\t65534\t65534", "65534\t65534\t65534\t65534"}
	fakeTasks(t, map[string][2]string{"1": ok, "2": ok})
	if err := verifyThreadIDs(65534, 65534); err != nil {
		t.Fatalf("all threads dropped = %v", err)
	}
	for name, bad := range map[string][2]string{
		"a thread still root":     {"0\t0\t0\t0", ok[1]},
		"a saved uid kept":        {"65534\t65534\t0\t65534", ok[1]},
		"a filesystem uid kept":   {"65534\t65534\t65534\t0", ok[1]},
		"a thread gid kept":       {ok[0], "65534\t0\t65534\t65534"},
		"a line that is not four": {"65534", ok[1]},
	} {
		fakeTasks(t, map[string][2]string{"1": ok, "2": bad})
		if err := verifyThreadIDs(65534, 65534); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	fakeTasks(t, map[string][2]string{})
	if err := verifyThreadIDs(0, 0); err == nil {
		t.Error("no threads listed was accepted")
	}
}

// The real /proc of this process: every thread has the test's own ids.
func TestVerifyThreadIDsReadsTheRealProcess(t *testing.T) {
	if err := verifyThreadIDs(os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("this process's threads: %v", err)
	}
	if err := verifyThreadIDs(os.Getuid()+1, os.Getgid()); err == nil {
		t.Fatal("a wrong uid was accepted for the real threads")
	}
}

func TestChildAttrCarriesTheDroppedCredential(t *testing.T) {
	if a := childAttr(Spec{}); a.Credential != nil || !a.Setpgid {
		t.Fatalf("no drop: %+v", a)
	}
	a := childAttr(Spec{RunAsUID: 65534, RunAsGID: 65534})
	if a.Credential == nil || a.Credential.Uid != 65534 || a.Credential.Gid != 65534 || !a.Credential.NoSetGroups || a.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("drop: %+v", a)
	}
}

// A stage whose confinement check fails reports it and runs nothing: no
// autoputer is started, no shot taken.
func TestStageRefusesWhenTheThreadCheckFails(t *testing.T) {
	spec := stageSpec(t, "ok")
	if err := os.MkdirAll(spec.WorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(spec.WorkDir, "refuse-confine"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	res := launchOrSkip(t, spec)
	if !strings.Contains(res.Error, "could not confine itself") || !strings.Contains(res.Error, "still has Uid") {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Files) != 0 {
		t.Fatal("a stage that failed its check took shots")
	}
	if _, err := os.Stat(filepath.Join(spec.WorkDir, "resets")); err == nil {
		t.Fatal("the servers ran after a failed confinement check")
	}
}
