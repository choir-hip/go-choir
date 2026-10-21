//go:build linux

package doctool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// The probe tool: the daemon's own binary run as a document tool
// (`autoputer doc-tool-probe <input> <reads...>` inside the sandbox) that acts
// as a hostile tool parsing a hostile file would, and prints what happened.
// The isolation self-test runs it on the deployed guest (SelfTest); the
// package tests run it locally. Ways it could mislead (written first): a
// probe failing with ENOENT reads as a denial (the judge rejects ENOENT); a
// probe that succeeds leaves a file behind (it is removed); the report is
// forged by a compromised tool (the probe runs before any untrusted parse,
// and the identity and namespace are also read against the spawner's own).

// RunProbe is the probe tool's entry: args are the input path and paths it
// must not be able to read.
func RunProbe(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "doc-tool-probe: no input")
		return 2
	}
	input := args[0]
	r := ProbeReport{UID: os.Getuid(), GID: os.Getgid(), Status: map[string]string{}, Probes: map[string]string{}}
	if raw, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			switch k {
			case "NoNewPrivs", "Seccomp", "CapEff", "CapPrm", "CapBnd", "Uid":
				r.Status[k] = strings.TrimSpace(v)
			}
		}
	} else {
		r.Status["error"] = probeErr(err)
	}
	r.NetNS, _ = os.Readlink("/proc/self/ns/net")
	_, err := os.ReadFile(input)
	r.Probes["read_input"] = probeErr(err)
	for _, p := range args[1:] {
		r.Probes["read:"+p] = probeRead(p)
	}
	w := filepath.Join(filepath.Dir(input), "probe-write")
	r.Probes["write_input_dir"] = probeErr(os.WriteFile(w, []byte("x"), 0o600))
	_ = os.Remove(w)
	w = filepath.Join(os.TempDir(), "choir-doctool-probe-"+strconv.Itoa(os.Getpid()))
	if os.TempDir() == filepath.Dir(input) {
		w = "/tmp/choir-doctool-probe-" + strconv.Itoa(os.Getpid())
	}
	r.Probes["write_tmp"] = probeErr(os.WriteFile(w, []byte("x"), 0o600))
	_ = os.Remove(w)
	r.Probes["socket_inet"] = probeSocket(unix.AF_INET)
	r.Probes["socket_unix"] = probeSocket(unix.AF_UNIX)
	r.Probes["unshare_user"] = probeErr(unix.Unshare(unix.CLONE_NEWUSER))
	r.Probes["unshare_net"] = probeErr(unix.Unshare(unix.CLONE_NEWNET))
	r.Probes["mount"] = probeErr(unix.Mount("none", filepath.Dir(input), "tmpfs", 0, ""))
	_, _, errno := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_ATTACH, uintptr(os.Getppid()), 0, 0, 0, 0)
	if errno != 0 {
		r.Probes["ptrace_parent"] = probeErr(errno)
	} else {
		r.Probes["ptrace_parent"] = ""
		_, _, _ = unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_DETACH, uintptr(os.Getppid()), 0, 0, 0, 0)
	}
	if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
		return 1
	}
	return 0
}

func probeRead(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return probeErr(err)
	}
	defer f.Close()
	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil && !errors.Is(err, os.ErrClosed) {
		if _, derr := f.ReadDir(1); derr == nil {
			return ""
		}
		return probeErr(err)
	}
	return ""
}

func probeSocket(family int) string {
	fd, err := unix.Socket(family, unix.SOCK_STREAM, 0)
	if err == nil {
		_ = unix.Close(fd)
	}
	return probeErr(err)
}

func probeErr(err error) string {
	if err == nil {
		return ""
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return unix.ErrnoName(errno) + ": " + err.Error()
	}
	return "ERROR: " + err.Error()
}

// SelfTest runs the probe tool through the sandbox in required mode with the
// daemon's own binary as helper and tool, whatever ModeEnv says, and judges
// the report. mustNotRead are paths the tool must not open (they are probed
// only if they exist, so a denial is never an ENOENT).
func SelfTest(ctx context.Context, bin string, mustNotRead []string) SelfTestReceipt {
	receipt := SelfTestReceipt{Mode: ModeFromEnv(nil)}
	var reads []string
	for _, p := range mustNotRead {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			reads = append(reads, p)
		}
	}
	out, warnings := run(ctx, runOptions{mode: ModeRequired, helperBin: bin}, bin, []string{ProbeArg}, ".pdf", []byte("%PDF-1.7 doc-tool self-test"), reads)
	receipt.Warnings = warnings
	if len(warnings) != 0 {
		receipt.Failures = append(receipt.Failures, "the sandboxed probe did not run cleanly: "+strings.Join(warnings, "; "))
		return receipt
	}
	var report ProbeReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		receipt.Failures = append(receipt.Failures, "probe output: "+err.Error())
		return receipt
	}
	receipt.Report = &report
	receipt.Failures = append(receipt.Failures, JudgeProbe(report, reads)...)
	return receipt
}
