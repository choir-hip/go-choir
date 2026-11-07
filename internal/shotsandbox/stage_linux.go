//go:build linux

package shotsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/shot"
)

// stageRun shoots the plan; tests replace it.
var stageRun runFunc = shot.Run

// MaybeRunStage turns this process into the sandbox stage when it was
// started as one (argv[1] is StageArg and StageEnv is set); otherwise it
// returns at once. cmd/autoputer calls it first thing in main, so the running
// build is the one that confines every candidate, whatever the candidate is.
func MaybeRunStage() {
	if len(os.Args) < 2 || os.Args[1] != StageArg || os.Getenv(StageEnv) == "" {
		return
	}
	os.Exit(stageMain(os.Stdin, os.Stdout, os.Stderr))
}

// stageMain is the stage process: read the spec, confine this process (the
// loopback up, the unprivileged id, the capsule's workload seccomp filter
// with inet), run the pass and print the Result as one JSON document.
func stageMain(in io.Reader, out, errw io.Writer) int {
	var spec Spec
	if err := json.NewDecoder(io.LimitReader(in, 1<<20)).Decode(&spec); err != nil {
		fmt.Fprintf(errw, "shot stage: spec: %v\n", err)
		return 2
	}
	_ = os.Unsetenv(StageEnv)
	runtime.LockOSThread() // the interface ioctl and the id drop run on one thread
	if err := confine(spec); err != nil {
		// A stage that could not confine itself runs nothing: the Result
		// says so and the launcher reports it (FM3 fails closed).
		return writeResult(out, Result{Error: "the sandbox could not confine itself: " + err.Error()})
	}
	runtime.UnlockOSThread()
	return writeResult(out, RunStage(context.Background(), spec, stageRun))
}

func writeResult(out io.Writer, r Result) int {
	if err := json.NewEncoder(out).Encode(r); err != nil {
		return 3
	}
	return 0
}

// confine brings loopback up (the only interface the namespace has), drops
// to the unprivileged id and loads the capsule's inet workload filter.
func confine(spec Spec) error {
	if err := loopbackUp(); err != nil {
		return fmt.Errorf("bring loopback up: %w", err)
	}
	if spec.RunAsUID != 0 || spec.RunAsGID != 0 {
		// The group first, while the process still may (setgroups is denied in
		// this namespace and root has none); setuid drops every capability with
		// it, which is the point.
		if err := syscall.Setgid(spec.RunAsGID); err != nil {
			return fmt.Errorf("setgid: %w", err)
		}
		if err := syscall.Setuid(spec.RunAsUID); err != nil {
			return fmt.Errorf("setuid: %w", err)
		}
	}
	if err := capsule.LoadWorkloadInetFilter(); err != nil {
		return err
	}
	return nil
}

// loopbackUp sets IFF_UP on lo of the stage's own network namespace.
func loopbackUp() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	req, err := unix.NewIfreq("lo")
	if err != nil {
		return err
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, req); err != nil {
		return err
	}
	req.SetUint16(req.Uint16() | unix.IFF_UP | unix.IFF_RUNNING)
	return unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, req)
}
