//go:build linux

package doctool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary is the helper and the probe tool: the spawner re-executes
// it with HelperArg (as it re-executes the daemon), and the helper execs it
// again with ProbeArg as the "document tool", which acts as a hostile tool
// parsing a hostile file would.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case HelperArg:
			os.Exit(RunHelper(os.Args[2:]))
		case ProbeArg:
			os.Exit(RunProbe(os.Args[2:]))
		case "doctool-test-echo":
			// An ordinary tool: prints its input file.
			raw, err := os.ReadFile(os.Args[len(os.Args)-1])
			if err != nil {
				os.Stderr.WriteString(err.Error())
				os.Exit(1)
			}
			os.Stdout.Write(raw)
			os.Exit(0)
		case "doctool-test-socket":
			// A tool that needs what the sandbox refuses.
			fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			if err != nil {
				os.Stderr.WriteString(err.Error())
				os.Exit(1)
			}
			syscall.Close(fd)
			os.Stdout.WriteString("ok")
			os.Exit(0)
		case "doctool-test-flood":
			chunk := make([]byte, 1<<16)
			for {
				if _, err := os.Stdout.Write(chunk); err != nil {
					os.Exit(1)
				}
			}
		}
	}
	os.Exit(m.Run())
}

func selfBin(t *testing.T) string {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

// unprivilegedSpawn runs the sandbox as a non-root test can: a user
// namespace that maps only the test's own uid (so the helper has no
// capability after its exec, as uid 61010 has none on the guest), with the
// same empty network and IPC namespaces, and no cgroup.
func unprivilegedSpawn(t *testing.T) runOptions {
	t.Helper()
	if out, err := exec.Command("unshare", "-Urn", "true").CombinedOutput(); err != nil {
		t.Skipf("unprivileged user and network namespaces unavailable: %v %s", err, out)
	}
	return runOptions{helperBin: selfBin(t), testUserNamespace: true, testNoCgroup: true}
}

func kernelLandlockABI() int {
	r, _, errno := syscall.Syscall(444 /* landlock_create_ruleset */, 0, 0, 1 /* LANDLOCK_CREATE_RULESET_VERSION */)
	if errno != 0 {
		return 0
	}
	return int(r)
}

// D1-D5, D10: the tool runs as a non-root identity with no capabilities, in
// its own network namespace, under no_new_privs, Landlock and seccomp; it
// reads its input and nothing else, writes nowhere, and cannot open a
// socket, trace, mount, or make a namespace.
func TestSandboxedToolCannotEscape(t *testing.T) {
	if kernelLandlockABI() < 5 {
		t.Skip("kernel offers no Landlock V5")
	}
	opts := unprivilegedSpawn(t)
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "secret")
	if err := os.WriteFile(secret, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.mode = ModeRequired
	out, warnings := run(context.Background(), opts, selfBin(t), []string{ProbeArg}, ".pdf", []byte("%PDF-1.7 hostile"), []string{secret, "/etc/passwd"})
	if len(warnings) != 0 {
		t.Fatalf("warnings %v (output %q)", warnings, out)
	}
	var probe ProbeReport
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		t.Fatalf("probe output %q: %v", out, err)
	}
	t.Logf("probe report: %+v", probe)
	if probe.Status["NoNewPrivs"] != "1" || probe.Status["Seccomp"] != "2" || probe.Status["CapEff"] != "0000000000000000" {
		t.Errorf("status %v", probe.Status)
	}
	if probe.UID == 0 {
		t.Error("the tool ran as uid 0")
	}
	ownNet, _ := os.Readlink("/proc/self/ns/net")
	if probe.NetNS == "" || probe.NetNS == ownNet {
		t.Errorf("tool network namespace %q, the test's %q: want a namespace of its own", probe.NetNS, ownNet)
	}
	if probe.Probes["read_input"] != "" {
		t.Errorf("the tool could not read its input: %s", probe.Probes["read_input"])
	}
	for _, name := range []string{"read:" + secret, "read:/etc/passwd", "write_input_dir", "write_tmp", "socket_inet", "socket_unix", "unshare_user", "unshare_net", "mount", "ptrace_parent"} {
		if probe.Probes[name] == "" {
			t.Errorf("probe %s succeeded inside the sandbox", name)
		}
		if strings.HasPrefix(probe.Probes[name], "ENOENT") {
			t.Errorf("probe %s failed with ENOENT, which proves nothing", name)
		}
	}
}

// D6, D9: required mode never runs the tool unsandboxed: a spawner that
// cannot switch identity (not root, no test namespace) refuses.
func TestRequiredModeRefusesWhatItCannotSandbox(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root the identity switch is possible")
	}
	marker := filepath.Join(t.TempDir(), "ran")
	opts := runOptions{helperBin: selfBin(t), mode: ModeRequired}
	out, warnings := run(context.Background(), opts, "/bin/sh", []string{"-c", "touch " + marker + "; cat \"$0\""}, ".pdf", []byte("x"), nil)
	if out != "" || len(warnings) != 1 || !strings.HasPrefix(warnings[0], "document_tool_sandbox_refused: ") {
		t.Fatalf("out %q warnings %v", out, warnings)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the tool ran although the sandbox was refused")
	}
	// The cgroup budget is part of required mode: a test namespace with no
	// desks cgroup is refused too.
	opts = unprivilegedSpawn(t)
	opts.testNoCgroup = false
	opts.mode = ModeRequired
	t.Setenv(cgroupRootEnv, "")
	if _, warnings := run(context.Background(), opts, "/bin/sh", []string{"-c", "true"}, ".pdf", []byte("x"), nil); len(warnings) != 1 || !strings.Contains(warnings[0], cgroupRootEnv) {
		t.Fatalf("required mode without a cgroup: warnings %v", warnings)
	}
}

// Observe runs what it cannot sandbox as before and says so in a warning the
// extraction keeps, so the staging probe can count fallbacks.
func TestObserveFallsBackVisibly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root the identity switch is possible")
	}
	opts := runOptions{helperBin: selfBin(t), mode: ModeObserve}
	out, warnings := run(context.Background(), opts, selfBin(t), []string{"doctool-test-echo"}, ".pdf", []byte("hello"), nil)
	if out != "hello" {
		t.Fatalf("observe fallback output %q, warnings %v", out, warnings)
	}
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "document_tool_sandbox_fallback: ") {
		t.Fatalf("warnings %v", warnings)
	}
}

// Off is the old path: the tool runs directly, no warning.
func TestOffRunsTheToolDirectly(t *testing.T) {
	out, warnings := run(context.Background(), runOptions{mode: ModeOff}, selfBin(t), []string{"doctool-test-echo"}, ".pdf", []byte("plain"), nil)
	if out != "plain" || len(warnings) != 0 {
		t.Fatalf("off: %q %v", out, warnings)
	}
}

// A sandboxed ordinary tool works and its output comes back.
func TestSandboxedToolOutput(t *testing.T) {
	if kernelLandlockABI() < 5 {
		t.Skip("kernel offers no Landlock V5")
	}
	opts := unprivilegedSpawn(t)
	opts.mode = ModeRequired
	out, warnings := run(context.Background(), opts, selfBin(t), []string{"doctool-test-echo"}, ".pdf", []byte("inside"), nil)
	if out != "inside" || len(warnings) != 0 {
		t.Fatalf("sandboxed: %q %v", out, warnings)
	}
}

// D11: a tool that floods stdout is stopped at the cap.
func TestSandboxedToolOutputIsCapped(t *testing.T) {
	if kernelLandlockABI() < 5 {
		t.Skip("kernel offers no Landlock V5")
	}
	opts := unprivilegedSpawn(t)
	opts.mode = ModeRequired
	opts.outputCap = 1 << 20
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, warnings := run(ctx, opts, selfBin(t), []string{"doctool-test-flood"}, ".pdf", []byte("x"), nil)
	if out != "" || len(warnings) != 1 || !strings.Contains(warnings[0], "output_exceeds") {
		t.Fatalf("flood: %d bytes, warnings %v", len(out), warnings)
	}
}

// D8: the helper entry does nothing without the spawner's environment: argv
// alone cannot make it exec a tool.
func TestHelperRefusesWithoutTheSpawnerEnvironment(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	cmd := exec.Command(selfBin(t), HelperArg, "/bin/sh", "-c", "touch "+marker)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != helperSetupExit {
		t.Fatalf("helper without env: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the helper ran a tool without the spawner's environment")
	}
}

// The deployed self-test judges the probe's raw facts; a spawner that cannot
// sandbox (not root here) reports a failure, never a pass.
func TestSelfTestFailsClosedAndJudges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root the self-test can pass")
	}
	r := SelfTest(context.Background(), selfBin(t), []string{"/etc/passwd", "/nonexistent-doctool-path"})
	if r.Report != nil || len(r.Failures) == 0 || !strings.Contains(r.Failures[0], "document_tool_sandbox_refused") {
		t.Fatalf("self-test as non-root: %+v", r)
	}
	own, _ := os.Readlink("/proc/self/ns/net")
	good := ProbeReport{UID: 61010, GID: 61010, NetNS: "net:[1]",
		Status: map[string]string{"NoNewPrivs": "1", "Seccomp": "2", "CapEff": "0000000000000000"},
		Probes: map[string]string{"read_input": "", "read:/etc/passwd": "EACCES: x"}}
	for _, n := range []string{"write_input_dir", "write_tmp", "socket_inet", "socket_unix", "unshare_user", "unshare_net", "mount", "ptrace_parent"} {
		good.Probes[n] = "EPERM: x"
	}
	if f := JudgeProbe(good, []string{"/etc/passwd"}); len(f) != 0 {
		t.Fatalf("good report judged %v", f)
	}
	bad := good
	bad.Probes = map[string]string{}
	for k, v := range good.Probes {
		bad.Probes[k] = v
	}
	bad.Probes["socket_inet"] = ""
	bad.Probes["read:/etc/passwd"] = "ENOENT: x"
	bad.NetNS = own
	bad.UID = 0
	if f := JudgeProbe(bad, []string{"/etc/passwd"}); len(f) != 4 {
		t.Fatalf("bad report judged %v, want 4 failures", f)
	}
}

// Observe never makes an extraction worse: a tool that fails only inside the
// sandbox is run again as before, and the divergence is a warning the
// staging probe can count before required mode is turned on.
func TestObserveRecordsASandboxDivergence(t *testing.T) {
	if kernelLandlockABI() < 5 {
		t.Skip("kernel offers no Landlock V5")
	}
	opts := unprivilegedSpawn(t)
	opts.mode = ModeObserve
	out, warnings := run(context.Background(), opts, selfBin(t), []string{"doctool-test-socket"}, ".pdf", []byte("x"), nil)
	if out != "ok" || len(warnings) != 1 || !strings.HasPrefix(warnings[0], "document_tool_sandbox_divergence: ") {
		t.Fatalf("observe divergence: %q %v", out, warnings)
	}
	// Required mode keeps the sandbox's answer.
	opts.mode = ModeRequired
	out, warnings = run(context.Background(), opts, selfBin(t), []string{"doctool-test-socket"}, ".pdf", []byte("x"), nil)
	if out != "" || len(warnings) != 1 || !strings.Contains(warnings[0], "_failed") {
		t.Fatalf("required: %q %v", out, warnings)
	}
}
