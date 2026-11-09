package agentcore

// Failure modes (an assignment's PDF and chart never reached the owner's
// Files):
//   - the copy lands outside Reports/<short id>/ (a symlinked Reports
//     directory in Files, a separator in a name);
//   - a re-copy after a crash or a re-driven report duplicates or corrupts;
//   - a crash mid-copy leaves a half file under a final name or a temp file;
//   - a file that fails to copy vanishes without a reason;
//   - the completion does not list the Files paths and the refusals, or lists
//     them twice on a re-drive.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func stagedDeliverables(t *testing.T, files map[string]string) (string, []capsule.DeliverableFile) {
	t.Helper()
	dir := t.TempDir()
	var out []capsule.DeliverableFile
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		out = append(out, capsule.DeliverableFile{Name: name, MediaType: "text/plain", Bytes: int64(len(body))})
	}
	return dir, out
}

func TestShortAssignmentID(t *testing.T) {
	if got := shortAssignmentID("assignment-0123456789abcdef"); got != "01234567" {
		t.Fatalf("got %q", got)
	}
	if got := shortAssignmentID("assignment-../../etc"); strings.ContainsAny(got, "./") || got == "" {
		t.Fatalf("got %q", got)
	}
}

func TestLandDeliverablesFilesUnderReportsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	stage, accepted := stagedDeliverables(t, map[string]string{"a.txt": "one", "b.csv": "x,y\n"})
	id := "assignment-0123456789abcdef"
	for round := 0; round < 2; round++ {
		landed, refused := landDeliverables(root, id, stage, accepted)
		if len(landed) != 2 || len(refused) != 0 {
			t.Fatalf("round %d: landed %v refused %v", round, landed, refused)
		}
	}
	got, err := os.ReadFile(filepath.Join(root, "Reports", "01234567", "a.txt"))
	if err != nil || string(got) != "one" {
		t.Fatalf("read back: %q, %v", got, err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "Reports", "01234567"))
	if len(entries) != 2 {
		t.Fatalf("directory holds %d entries (a temp file stayed?): %v", len(entries), entries)
	}
	// A changed re-run replaces the bytes.
	if err := os.WriteFile(filepath.Join(stage, "a.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	landDeliverables(root, id, stage, accepted)
	got, _ = os.ReadFile(filepath.Join(root, "Reports", "01234567", "a.txt"))
	if string(got) != "two" {
		t.Fatalf("re-copy did not replace: %q", got)
	}
}

func TestLandDeliverablesRefusesSymlinkedReportsAndListsReasons(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "Reports")); err != nil {
		t.Fatal(err)
	}
	stage, accepted := stagedDeliverables(t, map[string]string{"a.txt": "one"})
	landed, refused := landDeliverables(root, "assignment-0123456789abcdef", stage, accepted)
	if len(landed) != 0 || len(refused) != 1 || refused[0].Reason == "" {
		t.Fatalf("landed %v refused %v", landed, refused)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("the copy followed the symlink out of Files: %v", entries)
	}
	// No file tree at all.
	landed, refused = landDeliverables("", "assignment-0123456789abcdef", stage, accepted)
	if len(landed) != 0 || len(refused) != 1 {
		t.Fatalf("landed %v refused %v", landed, refused)
	}
}

func TestLandDeliverablesRefusesAMissingStagedFileWithReason(t *testing.T) {
	root := t.TempDir()
	stage, accepted := stagedDeliverables(t, map[string]string{"a.txt": "one"})
	accepted = append(accepted, capsule.DeliverableFile{Name: "gone.txt"}, capsule.DeliverableFile{Name: "../escape.txt"})
	landed, refused := landDeliverables(root, "assignment-0123456789abcdef", stage, accepted)
	if len(landed) != 1 || len(refused) != 2 {
		t.Fatalf("landed %v refused %v", landed, refused)
	}
	if _, err := os.Stat(filepath.Join(root, "Reports", "escape.txt")); err == nil {
		t.Fatal("a name with .. escaped the assignment directory")
	}
}

func TestDeliverablesSummaryListsPathsAndRefusalsOnce(t *testing.T) {
	root := t.TempDir()
	stage, accepted := stagedDeliverables(t, map[string]string{"chart.png": "png"})
	landed, _ := landDeliverables(root, "assignment-0123456789abcdef", stage, accepted)
	text := deliverablesSummary(landed, []capsule.DeliverableRefusal{{Name: "evil.exe", Reason: "type .exe is not delivered"}})
	for _, want := range []string{"Reports/01234567/chart.png", "evil.exe: type .exe is not delivered"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary lacks %q:\n%s", want, text)
		}
	}
	once := withDeliverablesSummary("Built the report.", text)
	twice := withDeliverablesSummary(once, text)
	if once != twice || !strings.HasPrefix(once, "Built the report.") || once != strings.TrimSpace(once) {
		t.Fatalf("re-drive changed the summary:\n%q\n%q", once, twice)
	}
	if deliverablesSummary(nil, nil) != "" || withDeliverablesSummary("s", "") != "s" {
		t.Fatal("an assignment without deliverables changed its summary")
	}
}

func TestLandAssignmentDeliverablesWithoutExecutorLeavesReport(t *testing.T) {
	rt := &Runtime{}
	report := types.EngineeringAssignmentReport{Summary: "done"}
	if got := rt.landAssignmentDeliverables(t.Context(), types.EngineeringAssignment{}, "run", "handle", report); got.Summary != "done" {
		t.Fatalf("summary %q", got.Summary)
	}
}
