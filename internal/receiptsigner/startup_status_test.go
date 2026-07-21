package receiptsigner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStartupStagePublishesClosedMode0600Value(t *testing.T) {
	path := filepath.Join(t.TempDir(), "startup-stage")
	for _, stage := range []StartupStage{
		StartupStageStarted,
		StartupStageConfigured,
		StartupStageKeyLoaded,
		StartupStageHandlerConfigured,
		StartupStageSocketListening,
	} {
		if err := WriteStartupStage(path, stage); err != nil {
			t.Fatalf("write %q: %v", stage, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if want := string(stage) + "\n"; string(got) != want {
			t.Fatalf("stage = %q, want %q", got, want)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestWriteStartupStageRejectsUnknownStageAndSymlinkDirectory(t *testing.T) {
	root := t.TempDir()
	if err := WriteStartupStage(filepath.Join(root, "unknown"), StartupStage("raw-error")); err == nil {
		t.Fatal("unknown stage succeeded")
	}
	target := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteStartupStage(filepath.Join(link, "stage"), StartupStageStarted); err == nil {
		t.Fatal("symlink startup directory succeeded")
	}
}
