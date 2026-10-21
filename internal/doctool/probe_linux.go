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

// ProbeReport is what the probe tool saw from inside the sandbox.
type ProbeReport struct {
	UID    int               `json:"uid"`
	GID    int               `json:"gid"`
	Status map[string]string `json:"status"`
	NetNS  string            `json:"net_ns"`
	// Probes maps a probe name to "" when the operation succeeded, else
	// "ERRNO: message".
	Probes map[string]string `json:"probes"`
}

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

// SelfTestReceipt is the deployed proof of the document-tool sandbox: the
// probe tool's report from inside it on the guest, the warnings of the run,
// and what the judge found wrong.
type SelfTestReceipt struct {
	Mode     Mode         `json:"mode"`
	Report   *ProbeReport `json:"report,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
	Failures []string     `json:"failures,omitempty"`
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

// JudgeProbe lists what is wrong with a probe report: the identity, the
// confinement status, the namespace, an allowed probe that failed, or a
// denied probe that succeeded or failed only with ENOENT.
func JudgeProbe(r ProbeReport, reads []string) []string {
	var failures []string
	if r.UID == 0 || r.GID == 0 {
		failures = append(failures, fmt.Sprintf("tool ran as uid %d gid %d", r.UID, r.GID))
	}
	if r.Status["NoNewPrivs"] != "1" || r.Status["Seccomp"] != "2" || r.Status["CapEff"] != "0000000000000000" {
		failures = append(failures, fmt.Sprintf("tool status %v: want NoNewPrivs 1, Seccomp 2, CapEff 0", r.Status))
	}
	if own, _ := os.Readlink("/proc/self/ns/net"); r.NetNS == "" || r.NetNS == own {
		failures = append(failures, "tool shares the daemon's network namespace")
	}
	if r.Probes["read_input"] != "" {
		failures = append(failures, "tool could not read its input: "+r.Probes["read_input"])
	}
	denied := []string{"write_input_dir", "write_tmp", "socket_inet", "socket_unix", "unshare_user", "unshare_net", "mount", "ptrace_parent"}
	for _, p := range reads {
		denied = append(denied, "read:"+p)
	}
	for _, name := range denied {
		got, ok := r.Probes[name]
		switch {
		case !ok:
			failures = append(failures, "probe "+name+" missing")
		case got == "":
			failures = append(failures, "probe "+name+" succeeded inside the sandbox")
		case strings.HasPrefix(got, "ENOENT"):
			failures = append(failures, "probe "+name+" failed with ENOENT, which proves nothing")
		}
	}
	return failures
}
