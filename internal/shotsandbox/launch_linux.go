//go:build linux

package shotsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// Residual (N1): the stage has no mount namespace, so the servers and the
// browser see every path the sandbox id can traverse, not only their own
// directories. What keeps the owner's data out of reach is the id and the
// modes: the capsule state directory's boot dir is 0700 (state_dir_linux.go),
// the live store and credentials are expected to sit in root-only directories
// (unchecked here), and the files layer holds only o+r, non-writable files with the mailbox left out.
// The live files root and mailbox directory are reachable by this id only if
// their own modes allow it (to be checked on d@e). A CLONE_NEWNS with a
// read-only bind layout (the stage's inputs, /nix/store, the work directory)
// is cheap in this launcher, since the namespace flags are already set here
// and the stage is the only process that needs the mounts, but it is not
// built: the seccomp filter already refuses mount(2) to the workload.

// Launch runs one pass in a new sandbox: the running build re-executed as the
// stage (exe, empty for this process's own binary) in a new user and network
// namespace, so it has loopback and nothing else, and reads the Result it
// prints. A stage that dies, hangs past ctx or prints no Result is an error
// with its stderr tail; a pass that ran and failed is a Result with Error set.
func Launch(ctx context.Context, exe string, spec Spec) (Result, error) {
	if exe == "" {
		self, err := os.Executable()
		if err != nil {
			return Result{}, fmt.Errorf("shot sandbox: own binary: %w", err)
		}
		exe = self
	}
	uid, gid := capsule.ShotSandboxIDs()
	attr := &syscall.SysProcAttr{
		Cloneflags:                 unix.CLONE_NEWUSER | unix.CLONE_NEWNET,
		GidMappingsEnableSetgroups: false,
		Setpgid:                    true,
		Pdeathsig:                  syscall.SIGKILL,
	}
	if os.Geteuid() == 0 {
		// Root keeps its own id for the stage's start (it needs the
		// capabilities to bring lo up) and the stage drops to the sandbox id.
		attr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: uid, HostID: uid, Size: 1}}
		attr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: 0, Size: 1}, {ContainerID: gid, HostID: gid, Size: 1}}
		spec.RunAsUID, spec.RunAsGID = uid, gid
	} else {
		attr.UidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: uid, Size: 1}}
		attr.GidMappings = []syscall.SysProcIDMap{{ContainerID: 0, HostID: gid, Size: 1}}
		spec.RunAsUID, spec.RunAsGID = 0, 0
	}
	return launch(ctx, exe, spec, attr)
}

// launch starts the stage with attr; the split lets a test start it
// without namespaces on a host that forbids them.
func launch(ctx context.Context, exe string, spec Spec, attr *syscall.SysProcAttr) (Result, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return Result{}, err
	}
	// The stage's own timeout is the plan's plus slack; the launcher's is a
	// little more, so the stage's own report wins a race.
	limit := spec.timeouts().Plan
	if limit <= 0 {
		limit = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, limit+stageSlack+30*time.Second)
	defer cancel()
	var stdout bytes.Buffer
	stderr := &tailBuffer{max: 4096}
	cmd := exec.CommandContext(ctx, exe, StageArg)
	cmd.Env = []string{StageEnv + "=1", "PATH=" + firstNonEmpty(os.Getenv("PATH"), "/usr/bin:/bin"), "HOME=" + spec.WorkDir, "TMPDIR=" + spec.WorkDir}
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Stdout = &limitedWriter{buf: &stdout, max: 4 << 20}
	cmd.Stderr = stderr
	cmd.SysProcAttr = attr
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("shot sandbox did not start: %w", err)
	}
	waitErr := cmd.Wait()
	// Whatever the stage left behind in its group dies with it.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	var result Result
	if decodeErr := json.NewDecoder(&stdout).Decode(&result); decodeErr != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("shot sandbox timed out: %w", ctx.Err())
		}
		return Result{}, fmt.Errorf("shot sandbox left no result (%v): %s", waitErr, stderr.String())
	}
	return result, nil
}

type limitedWriter struct {
	buf *bytes.Buffer
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.max {
		return 0, errors.New("shot sandbox output over its bound")
	}
	return w.buf.Write(p)
}
