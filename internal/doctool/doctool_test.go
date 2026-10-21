package doctool

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The document-tool sandbox (SDC SEC-15; docs/definitions/choir-launch-fp2-
// desk-boundary-2026-10-21.md, "Second batch", failure modes D1-D11).

func TestModeFromEnv(t *testing.T) {
	cases := map[string]Mode{
		"":          ModeOff,
		"off":       ModeOff,
		"observe":   ModeObserve,
		" OBSERVE ": ModeObserve,
		"required":  ModeRequired,
		// A typo strengthens: it is required, never off.
		"requried": ModeRequired,
		"on":       ModeRequired,
	}
	for raw, want := range cases {
		got := ModeFromEnv(func(k string) string {
			if k != ModeEnv {
				t.Fatalf("read %s", k)
			}
			return raw
		})
		if got != want {
			t.Errorf("%q: mode %q, want %q", raw, got, want)
		}
	}
}

// The image declares the sandbox user for operators (ps, ls -l) with the
// number the spawner uses, and sets the mode it ships in.
func TestGuestImageDeclaresTheSandboxUser(t *testing.T) {
	raw, err := os.ReadFile("../../nix/autoputer-vm.nix")
	if err != nil {
		t.Skipf("nix/autoputer-vm.nix not readable from the package: %v", err)
	}
	nix := string(raw)
	uid := regexp.MustCompile(`users\.users\.choir-doctool = \{[^}]*uid = (\d+);`).FindStringSubmatch(nix)
	gid := regexp.MustCompile(`users\.groups\.choir-doctool = \{ gid = (\d+); \}`).FindStringSubmatch(nix)
	if uid == nil || gid == nil || uid[1] != "61010" || gid[1] != "61010" || SandboxUID != 61010 {
		t.Fatalf("image user %v group %v, spawner uid %d: want 61010 everywhere", uid, gid, SandboxUID)
	}
	if !strings.Contains(nix, `CHOIR_DOCTOOL_CGROUP_ROOT = "/sys/fs/cgroup/choir-doctools";`) {
		t.Fatal("the guest image does not name the document tools' cgroup root")
	}
	if !strings.Contains(nix, `CHOIR_DOCUMENT_TOOL_SANDBOX = "observe";`) {
		t.Fatal(`the guest image does not set CHOIR_DOCUMENT_TOOL_SANDBOX = "observe"`)
	}
}

// Review B2: a tool's cgroup is never under the desks cgroup; the document
// tools have their own root, swept by its own name pattern.
func TestToolCgroupRootIsNotTheDesks(t *testing.T) {
	if cgroupRootEnv == "CHOIR_DESK_CGROUP_ROOT" || cgroupRootEnv != "CHOIR_DOCTOOL_CGROUP_ROOT" {
		t.Fatalf("cgroup root env %q", cgroupRootEnv)
	}
	if strings.HasPrefix(toolCgroupPrefix, "w-") {
		t.Fatalf("tool cgroup prefix %q collides with the desks' w-*", toolCgroupPrefix)
	}
}

// D11: output past the cap is refused, not buffered.
func TestCappedBuffer(t *testing.T) {
	b := &cappedBuffer{limit: 8}
	if n, err := b.Write([]byte("12345")); n != 5 || err != nil {
		t.Fatalf("write under cap: %d %v", n, err)
	}
	if _, err := b.Write([]byte("6789")); err == nil || !b.overflow {
		t.Fatal("write past cap accepted")
	}
	if b.buf.Len() > 8 {
		t.Fatalf("buffered %d bytes past a cap of 8", b.buf.Len())
	}
}
