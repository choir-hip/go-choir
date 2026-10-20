package claudedelegate

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// Failure mode F15 (docs/problems/claude-delegate-resume-sonnet-2026-10-20.md,
// Root cause): a run's write is invisible to the desk. The desk's cells run
// at / and the delegate at /workspace/platform, so a relative path in a
// brief names two files. The status must list what the run wrote as
// absolute paths the broker itself stat'ed, and must never stat a path the
// model names outside the working tree.

func TestWrittenPathResolvesAgainstTheWorkdirAndStaysInside(t *testing.T) {
	const wd = "/workspace/platform"
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"docs/probes/x.md", wd + "/docs/probes/x.md", true},
		{"./docs/x.md", wd + "/docs/x.md", true},
		{wd + "/internal/a.go", wd + "/internal/a.go", true},
		{wd + "/internal/../cmd/b.go", wd + "/cmd/b.go", true},
		{"", "", false},
		{"/etc/passwd", "", false},
		{"../etc/passwd", "", false},
		{"docs/../../etc/passwd", "", false},
		{wd, "", false},
		{"/workspace/platform-evil/x", "", false},
		{"/tmp/choir-delegate/x/home/.claude/y", "", false},
	}
	for _, c := range cases {
		got, ok := writtenPath(wd, c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("writtenPath(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// A Write with a relative path lands in the delegate's working tree; the
// status names it as an absolute path with the broker's own stat, lists an
// Edit target that does not exist as exists=false, and ignores paths
// outside the tree and non-writing tools.
func TestServerReportsWrittenFilesAsAbsolutePathsItStatted(t *testing.T) {
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "docs", "probes"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := `
read prompt
cd ` + tree + `
printf 'probe ok\n' > docs/probes/p.md
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"docs/probes/p.md","content":"probe ok"}}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + tree + `/docs/probes/p.md","content":"again"}},{"type":"tool_use","name":"Edit","input":{"file_path":"docs/missing.md"}},{"type":"tool_use","name":"Write","input":{"file_path":"/etc/passwd"}},{"type":"tool_use","name":"Read","input":{"file_path":"docs/read-only.md"}}]}}'
echo '{"type":"result","subtype":"success","is_error":false,"num_turns":2,"session_id":"123e4567-e89b-12d3-a456-426614174000"}'
`
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, script))
	srv.Workdir = tree
	if _, err := srv.Start(Spec{Objective: "write", MaxTurns: 2, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(10 * time.Second)
	if final.State != StateDone {
		t.Fatalf("final = %+v", final)
	}
	want := []WrittenFile{
		{Path: filepath.Join(tree, "docs", "probes", "p.md"), Exists: true, Bytes: int64(len("probe ok\n"))},
		{Path: filepath.Join(tree, "docs", "missing.md"), Exists: false},
	}
	if !slices.Equal(final.WrittenFiles, want) {
		t.Fatalf("written_files = %+v; want %+v", final.WrittenFiles, want)
	}
	onDisk, err := ReadStatus(srv.Root, final.ID)
	if err != nil || !slices.Equal(onDisk.WrittenFiles, want) {
		t.Fatalf("status.json written_files = %+v, %v", onDisk.WrittenFiles, err)
	}
}

// The list is bounded, and the overflow is counted, not dropped silently.
func TestServerWrittenFilesAreBounded(t *testing.T) {
	const total = 200
	tree := t.TempDir()
	script := `
read prompt
n=0
while [ $n -lt ` + strconv.Itoa(total) + ` ]; do
echo "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"tool_use\",\"name\":\"Write\",\"input\":{\"file_path\":\"f$n.txt\"}}]}}"
n=$((n+1))
done
echo '{"type":"result","subtype":"success","is_error":false}'
`
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, script))
	srv.Workdir = tree
	if _, err := srv.Start(Spec{Objective: "many", MaxTurns: 2, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(15 * time.Second)
	if len(final.WrittenFiles) != maxWrittenFiles || final.WrittenFilesTruncated != total-maxWrittenFiles {
		t.Fatalf("len=%d truncated=%d", len(final.WrittenFiles), final.WrittenFilesTruncated)
	}
}
