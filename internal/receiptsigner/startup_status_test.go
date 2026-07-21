package receiptsigner

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
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
		StartupStageServeReturnedClosed,
		StartupStageServeReturnedPermission,
		StartupStageServeReturnedInvalid,
		StartupStageServeReturnedResource,
		StartupStageServeReturnedUnknown,
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

func TestClassifyServeExitUsesClosedNonSecretClasses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want StartupStage
	}{
		{name: "nil", want: StartupStageServeReturnedClosed},
		{name: "server closed", err: fmt.Errorf("wrapped: %w", http.ErrServerClosed), want: StartupStageServeReturnedClosed},
		{name: "listener closed", err: fmt.Errorf("wrapped: %w", net.ErrClosed), want: StartupStageServeReturnedClosed},
		{name: "permission", err: fmt.Errorf("wrapped: %w", syscall.EACCES), want: StartupStageServeReturnedPermission},
		{name: "operation not permitted", err: fmt.Errorf("wrapped: %w", syscall.EPERM), want: StartupStageServeReturnedPermission},
		{name: "bad descriptor", err: fmt.Errorf("wrapped: %w", syscall.EBADF), want: StartupStageServeReturnedInvalid},
		{name: "invalid", err: fmt.Errorf("wrapped: %w", syscall.EINVAL), want: StartupStageServeReturnedInvalid},
		{name: "process descriptors exhausted", err: fmt.Errorf("wrapped: %w", syscall.EMFILE), want: StartupStageServeReturnedResource},
		{name: "system descriptors exhausted", err: fmt.Errorf("wrapped: %w", syscall.ENFILE), want: StartupStageServeReturnedResource},
		{name: "socket buffers exhausted", err: fmt.Errorf("wrapped: %w", syscall.ENOBUFS), want: StartupStageServeReturnedResource},
		{name: "memory exhausted", err: fmt.Errorf("wrapped: %w", syscall.ENOMEM), want: StartupStageServeReturnedResource},
		{name: "unknown", err: fmt.Errorf("opaque"), want: StartupStageServeReturnedUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyServeExit(test.err); got != test.want {
				t.Fatalf("stage = %q, want %q", got, test.want)
			}
		})
	}
}
