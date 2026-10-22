//go:build linux

package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/elastic/go-seccomp-bpf"
	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// seccompSelfTestFilter is the capsule's own constructor for a filter name:
// the functions LoadWorkloadFilter and the others load (S1).
func seccompSelfTestFilter(name string) (seccomp.Filter, bool) {
	switch name {
	case "workload":
		return WorkloadSeccompFilter(), true
	case "workload-inet":
		return WorkloadInetSeccompFilter(), true
	case "broker":
		return BrokerSeccompFilter(), true
	case "broker-inet":
		return BrokerInetSeccompFilter(), true
	case "delegate":
		return DelegateSeccompFilter(), true
	case "broker-delegate":
		return BrokerDelegateSeccompFilter(), true
	}
	return seccomp.Filter{}, false
}

// seccompProgramHash assembles the filter exactly as seccomp.LoadFilter does
// and hashes the raw instructions.
func seccompProgramHash(f seccomp.Filter) (string, int, error) {
	insts, err := f.Policy.Assemble()
	if err != nil {
		return "", 0, err
	}
	raw, err := bpf.Assemble(insts)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	var b [8]byte
	for _, in := range raw {
		binary.LittleEndian.PutUint16(b[0:], in.Op)
		b[2], b[3] = in.Jt, in.Jf
		binary.LittleEndian.PutUint32(b[4:], in.K)
		h.Write(b[:])
	}
	return hex.EncodeToString(h.Sum(nil)), len(raw), nil
}

func seccompSelfTestFilterHash(name string) (string, error) {
	f, ok := seccompSelfTestFilter(name)
	if !ok {
		return "", fmt.Errorf("unknown capsule filter %q", name)
	}
	h, _, err := seccompProgramHash(f)
	return h, err
}

func seccompProbeErr(err error) string {
	if err == nil {
		return ""
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		name := unix.ErrnoName(errno)
		if name == "" {
			name = fmt.Sprintf("ERRNO%d", int(errno))
		}
		return name + ": " + errno.Error()
	}
	return "ERROR: " + err.Error()
}

// RunSeccompSelfTestChild is the child half: args is [filter name], or
// ["noop"] for the controls' own children. It prints one JSON report on
// stdout and returns the process exit code.
func RunSeccompSelfTestChild(args []string) int {
	if len(args) == 1 && args[0] == "noop" {
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: capsule-seccomp-selftest <filter>")
		return 2
	}
	name := args[0]
	f, ok := seccompSelfTestFilter(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown capsule filter %q\n", name)
		return 2
	}
	out := SeccompSelfTestFilter{Name: name, Probes: map[string]string{}}
	defer func() {
		raw, _ := json.Marshal(out)
		_, _ = os.Stdout.Write(raw)
	}()
	h, n, err := seccompProgramHash(f)
	if err != nil {
		out.LoadError = "assemble: " + err.Error()
		return 0
	}
	out.BPFSHA256, out.Instructions = h, n
	self, err := os.Executable()
	if err != nil {
		out.Error = "executable: " + err.Error()
		return 0
	}
	if err := seccomp.LoadFilter(f); err != nil {
		out.LoadError = err.Error()
		return 0
	}
	out.Loaded = true
	out.Seccomp = procSelfStatusField("Seccomp")

	// clone3 with no struct: the kernel answers EINVAL or EFAULT; the filter
	// ENOSYS.
	if _, _, e := unix.Syscall(unix.SYS_CLONE3, 0, 0, 0); e != 0 {
		out.Probes["clone3"] = seccompProbeErr(e)
	} else {
		out.Probes["clone3"] = ""
	}
	// CLONE_NEWUSER with CLONE_THREAD is invalid to the kernel (EINVAL) and
	// never makes a process; the filter refuses it first (EPERM).
	if _, _, e := unix.Syscall6(unix.SYS_CLONE, unix.CLONE_NEWUSER|unix.CLONE_THREAD, 0, 0, 0, 0, 0); e != 0 {
		out.Probes["clone_newuser"] = seccompProbeErr(e)
	} else {
		out.Probes["clone_newuser"] = ""
	}
	// unshare is outside the allowlist: EPERM. A multithreaded process that
	// reached the kernel gets EINVAL.
	out.Probes["unshare_newuser"] = seccompProbeErr(unix.Unshare(unix.CLONE_NEWUSER))

	// Controls: a child still starts and exits 0 under the filter.
	argv := []string{self, SeccompSelfTestArg, "noop"}
	pid, err := syscall.ForkExec(self, argv, &syscall.ProcAttr{Env: []string{}})
	if err == nil {
		var ws syscall.WaitStatus
		if _, werr := syscall.Wait4(pid, &ws, 0, nil); werr != nil {
			err = werr
		} else if !ws.Exited() || ws.ExitStatus() != 0 {
			err = fmt.Errorf("child ended %v", ws)
		}
	}
	out.Probes["fork_exec"] = seccompProbeErr(err)
	cmd := exec.Command(self, SeccompSelfTestArg, "noop")
	cmd.Env = []string{}
	out.Probes["os_exec"] = seccompProbeErr(cmd.Run())
	return 0
}

func procSelfStatusField(key string) string {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SeccompSelfTest runs one child per capsule filter from bin (the daemon's
// own binary) and judges the receipt. Each child leads its own process group
// (its control children join it). At the timeout the whole group is killed
// (cmd.Cancel), not only the child. Pdeathsig kills the child, but not its
// group, if the daemon thread that started it dies.
func SeccompSelfTest(ctx context.Context, bin string) SeccompSelfTestReceipt {
	rec := SeccompSelfTestReceipt{Kind: "capsule_seccomp_selftest", StartedAt: time.Now().UTC().Format(time.RFC3339)}
	var uts unix.Utsname
	if err := unix.Uname(&uts); err == nil {
		rec.Kernel = unix.ByteSliceToString(uts.Release[:])
	}
	for _, name := range SeccompSelfTestFilterNames {
		f := SeccompSelfTestFilter{Name: name}
		expected, err := seccompSelfTestFilterHash(name)
		if err != nil {
			f.Error = "expected hash: " + err.Error()
			rec.Filters = append(rec.Filters, f)
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		cmd := exec.CommandContext(cctx, bin, SeccompSelfTestArg, name)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "TMPDIR=/tmp"}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		runErr := cmd.Run()
		cancel()
		if err := json.Unmarshal(stdout.Bytes(), &f); err != nil {
			f = SeccompSelfTestFilter{Name: name}
			tail := strings.TrimSpace(stderr.String())
			if len(tail) > 300 {
				tail = tail[len(tail)-300:]
			}
			f.Error = fmt.Sprintf("child report unreadable (%v, run %v): %s", err, runErr, tail)
		} else if runErr != nil {
			f.Error = "child: " + runErr.Error()
		}
		f.Name = name
		f.ExpectedSHA256 = expected
		rec.Filters = append(rec.Filters, f)
	}
	rec.Failures = JudgeSeccompSelfTest(rec)
	rec.OK = len(rec.Failures) == 0
	return rec
}
