//go:build linux

package capsule

import (
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A choir-shot job with its build cached claims 1536 MiB, not the build
// peak. Failure modes pinned:
//   - D1 a cached shot run, direct or through sh -c, keeps the 3072 claim;
//   - D2 an uncached run (first run builds with vite) is defaulted down and
//     OOMs the capsule;
//   - D3 an explicit memory_mib is overridden;
//   - D4 a compound build-plus-shot command or another script is defaulted;
//   - D5 the Go source hash drifts from choir-shot.mjs sourceHash, so a
//     cached build is never seen (or a stale one is).

func shotCapsule(t *testing.T, cached bool) *Capsule {
	t.Helper()
	merged := t.TempDir()
	fe := filepath.Join(merged, "workspace", "platform", "frontend")
	if err := os.MkdirAll(filepath.Join(fe, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"package.json": "{}", "src/a.ts": "a", "src/b.ts": "b"} {
		if err := os.WriteFile(filepath.Join(fe, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if cached {
		sum, err := shotSourceHash(fe, "/workspace/platform/frontend")
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(merged, "tmp", "choir-shot", sum)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "done"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Capsule{MergedDir: merged}
}

func TestDefaultShotMemory(t *testing.T) {
	cwd := "/workspace/platform"
	script := "/workspace/platform/scripts/choir-shot.mjs"
	for _, tc := range []struct {
		name     string
		cached   bool
		argv     []string
		declared int
		want     int
	}{
		{"cached direct", true, []string{"node", script, "--shots", "/tmp/s.mjs"}, 0, 1536},
		{"cached relative script", true, []string{"node", "scripts/choir-shot.mjs"}, 0, 1536},
		{"cached sh -c", true, []string{"sh", "-c", "node " + script + " --shots /tmp/s.mjs --only a"}, 0, 1536},
		{"cached sh -c quoted", true, []string{"/bin/sh", "-c", "node '" + script + "' --only a"}, 0, 1536},
		{"cd then relative script", true, []string{"sh", "-c", "cd /workspace/platform && node scripts/choir-shot.mjs"}, 0, 0},
		{"cd then absolute script", true, []string{"sh", "-c", "cd /tmp && node " + script}, 0, 0},
		{"git pull then shot", true, []string{"sh", "-c", "git pull; node " + script}, 0, 0},
		{"env -C wrapper", true, []string{"sh", "-c", "env -C /other node scripts/choir-shot.mjs"}, 0, 0},
		{"env prefix", true, []string{"sh", "-c", "NODE_OPTIONS=--import=/tmp/x.mjs node " + script}, 0, 0},
		{"node --import flag", true, []string{"node", "--import=/tmp/b.mjs", script}, 0, 0},
		{"node -r flag", true, []string{"sh", "-c", "node -r /tmp/b.js " + script}, 0, 0},
		{"script suffix", true, []string{"node", script + ".bak"}, 0, 0},
		{"script suffix sh", true, []string{"sh", "-c", "node " + script + ".bak"}, 0, 0},
		{"wrapper executable", true, []string{"sh", "-c", "ninja node " + script}, 0, 0},
		{"piped to tail", true, []string{"sh", "-c", "node " + script + " | tail -3"}, 0, 1536},
		{"uncached", false, []string{"node", script}, 0, 0},
		{"uncached sh -c", false, []string{"sh", "-c", "node " + script}, 0, 0},
		{"explicit wins", true, []string{"node", script}, 2048, 2048},
		{"explicit below wins", true, []string{"node", script}, 900, 900},
		{"build plus shot", true, []string{"sh", "-c", "npm run build && node " + script}, 0, 0},
		{"other script", true, []string{"node", "scripts/other.mjs"}, 0, 0},
		{"cat of the script", true, []string{"cat", script}, 0, 0},
		{"platform root override", true, []string{"sh", "-c", "CHOIR_PLATFORM_ROOT=/x node " + script}, 0, 0},
	} {
		got, note := defaultShotMemory(shotCapsule(t, tc.cached), tc.argv, cwd, tc.declared)
		if tc.declared > 0 {
			if got != tc.want || note != "" {
				t.Errorf("D3 %s: %d %q", tc.name, got, note)
			}
			continue
		}
		if got != tc.want || (tc.want > 0) != (note == shotDefaultNote) {
			t.Errorf("D1/D2/D4 %s: got %d %q, want %d", tc.name, got, note, tc.want)
		}
	}
	// A changed frontend source is a different hash: the old cache is stale.
	caps := shotCapsule(t, true)
	if err := os.WriteFile(filepath.Join(caps.MergedDir, "workspace", "platform", "frontend", "src", "a.ts"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := defaultShotMemory(caps, []string{"node", script}, cwd, 0); got != 0 {
		t.Errorf("a stale cache defaulted to %d", got)
	}
}

func TestJobWaitReasonHintsOnlyOnTheUndeclaredClaim(t *testing.T) {
	facts := HostMemoryFacts{HeadroomBytes: 1 << 30}
	plain := jobWaitReason(&capsuleJob{}, facts, true, 0)
	if !strings.HasSuffix(plain, memoryMiBHint) || !strings.Contains(plain, "3072 MiB") {
		t.Errorf("undeclared wait text: %q", plain)
	}
	def := jobWaitReason(&capsuleJob{memoryNote: shotDefaultNote, spec: specMiB(1536)}, HostMemoryFacts{HeadroomBytes: 100 << 20}, true, 0)
	if !strings.Contains(def, shotDefaultNote) || strings.Contains(def, memoryMiBHint) {
		t.Errorf("defaulted wait text: %q", def)
	}
	if got := jobWaitReason(&capsuleJob{}, HostMemoryFacts{HeadroomBytes: 8 << 30}, true, 0); got != "" {
		t.Errorf("no wait expected: %q", got)
	}
}

// D5: the Go hash is choir-shot.mjs's sourceHash.
func TestShotSourceHashMatchesChoirShot(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{"a.txt": "1", "B/c.txt": "2", "node_modules/x/y.js": "no", "dist/o.js": "no", "src/z.ts": "z", "src/Z.ts": "Z"} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script, _ := filepath.Abs("../../scripts/choir-shot.mjs")
	out, err := exec.Command(node, "--input-type=module", "-e",
		"const m = await import('file://"+script+"'); console.log(m.sourceHash(process.argv[1]))", dir).Output()
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	got, err := shotSourceHash(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != got {
		t.Fatalf("D5: node %q, go %q", out, got)
	}
}

func specMiB(n int) capsulejob.Spec { return capsulejob.Spec{MemoryMiB: n} }
