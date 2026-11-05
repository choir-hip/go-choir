//go:build linux

package yaegikernel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TestDeskCellSeccompProfileEnumeration: the profile test of MA6.2. Every
// named class is in the explicit deny group, none of them is also allowed,
// clone carries the namespace-flag condition, and the filter assembles.
func TestDeskCellSeccompProfileEnumeration(t *testing.T) {
	filter := DeskCellSeccompFilter(os.Getpid())
	if _, err := filter.Policy.Assemble(); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	denied, allowed := map[string]bool{}, map[string]bool{}
	for _, g := range filter.Policy.Syscalls {
		for _, n := range g.Names {
			if g.Action == cellDeny {
				denied[n] = true
			}
			if g.Action == cellAllow {
				allowed[n] = true
			}
		}
		for _, n := range g.NamesWithCondtions {
			if g.Action == cellAllow {
				allowed[n.Name+"?"] = true
			}
		}
	}
	classes := map[string][]string{
		"io_uring":    {"io_uring_setup", "io_uring_enter", "io_uring_register"},
		"keyring":     {"keyctl", "add_key", "request_key"},
		"bpf":         {"bpf"},
		"perf":        {"perf_event_open"},
		"userfaultfd": {"userfaultfd"},
		"ptrace":      {"ptrace", "process_vm_readv", "process_vm_writev"},
		"namespaces":  {"unshare", "setns", "mount", "pivot_root", "chroot"},
	}
	var keys []string
	for k := range classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, class := range keys {
		for _, n := range classes[class] {
			if !denied[n] {
				t.Errorf("%s: %s is not in the explicit deny group", class, n)
			}
			if allowed[n] || allowed[n+"?"] {
				t.Errorf("%s: %s is also allowed", class, n)
			}
		}
	}
	for n := range denied {
		if allowed[n] || allowed[n+"?"] {
			t.Errorf("%s is both denied and allowed", n)
		}
	}
	for _, g := range filter.Policy.Syscalls {
		for _, n := range g.NamesWithCondtions {
			if n.Name != "clone" {
				continue
			}
			var thread, noNS bool
			for _, c := range n.Conditions {
				if c.Argument == 0 && c.Operation == "BitsSet" && c.Value == unix.CLONE_THREAD {
					thread = true
				}
				if c.Argument == 0 && c.Operation == "BitsNotSet" && c.Value == cloneNamespaceFlags {
					noNS = true
				}
			}
			if !thread || !noNS {
				t.Errorf("clone conditions %+v: want CLONE_THREAD set and every CLONE_NEW* bit clear", n.Conditions)
			}
		}
	}
	// The classes the self-test probes are the classes the profile names.
	for _, n := range deskCellClassProbeNames() {
		if n == "clone_newns" || n == "clone_newuser" {
			continue
		}
		if !denied[n] {
			t.Errorf("self-test probe %s is not an explicitly denied syscall", n)
		}
	}
}

// hardeningChild runs the named step inside a re-executed test binary.
func TestDeskCellHardeningChild(t *testing.T) {
	step := os.Getenv("CHOIR_HARDENING_STEP")
	if step == "" {
		return
	}
	out := map[string]any{}
	switch step {
	case "mounts":
		root, tmp := os.Getenv("CHOIR_HARDENING_ROOT"), os.Getenv("CHOIR_HARDENING_TMP")
		if err := isolateCellMounts(root, tmp); err != nil {
			fmt.Printf("RESULT %s\n", mustJSON(map[string]any{"error": err.Error()}))
			os.Exit(0)
		}
		if err := dropAllCapabilities(); err != nil {
			fmt.Printf("RESULT %s\n", mustJSON(map[string]any{"error": err.Error()}))
			os.Exit(0)
		}
		_, perr := os.Stat("/proc/meminfo")
		_, serr := os.Stat("/sys/kernel")
		_, merr := os.Stat("/dev/mem")
		var dev []string
		if es, err := os.ReadDir("/dev"); err == nil {
			for _, e := range es {
				dev = append(dev, e.Name())
			}
		}
		_, rerr := os.ReadFile(filepath.Join(root, "visible.txt"))
		werr := os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o600)
		terr := os.WriteFile(filepath.Join(tmp, "x"), []byte("x"), 0o600)
		rootErr := os.WriteFile("/x", []byte("x"), 0o600)
		_, nerr := os.ReadFile("/dev/null")
		wd, _ := os.Getwd()
		// Every thread has no capabilities: read this one's and a new one's.
		caps := make(chan string, 1)
		go func() {
			raw, _ := os.ReadFile("/proc/thread-self/status")
			caps <- string(raw)
		}()
		hdr := capHeader{version: linuxCapabilityVersion3}
		var data [2]capData
		_, _, e := syscall.RawSyscall(unix.SYS_CAPGET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0)
		out = map[string]any{
			"proc_err": errStr(perr), "sys_err": errStr(serr), "dev_mem_err": errStr(merr), "dev": dev,
			"read_root_err": errStr(rerr), "write_root_err": errStr(werr), "write_tmp_err": errStr(terr),
			"write_newroot_err": errStr(rootErr), "dev_null_err": errStr(nerr), "cwd": wd,
			"cap_eff": data[0].effective | data[1].effective, "cap_prm": data[0].permitted | data[1].permitted,
			"capget_errno": int(e),
		}
		select {
		case <-caps:
		case <-time.After(time.Second):
		}
	case "fds":
		entries, _ := os.ReadDir("/proc/self/fd")
		before := len(entries)
		CloseInheritedFDs()
		var left []string
		entries, _ = os.ReadDir("/proc/self/fd")
		for _, e := range entries {
			// Only the files the test leaked (the test binary); the listing's
			// own descriptor and the runtime's are not the point.
			if target, err := os.Readlink("/proc/self/fd/" + e.Name()); err == nil && target == os.Args[0] {
				left = append(left, e.Name())
			}
		}
		sort.Strings(left)
		out = map[string]any{"before": before, "after": left}
	case "rlimits":
		if err := applyCellRlimits(); err != nil {
			out = map[string]any{"error": err.Error()}
			break
		}
		lim := map[string][2]uint64{}
		for name, res := range map[string]int{"core": unix.RLIMIT_CORE, "nofile": unix.RLIMIT_NOFILE, "fsize": unix.RLIMIT_FSIZE, "memlock": unix.RLIMIT_MEMLOCK, "msgqueue": unix.RLIMIT_MSGQUEUE} {
			var l unix.Rlimit
			_ = unix.Getrlimit(res, &l)
			lim[name] = [2]uint64{l.Cur, l.Max}
		}
		out = map[string]any{"limits": lim}
	}
	fmt.Printf("RESULT %s\n", mustJSON(out))
	os.Exit(0)
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func runHardeningChild(t *testing.T, step string, attr *syscall.SysProcAttr, extraFiles []*os.File, env ...string) map[string]json.RawMessage {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeskCellHardeningChild$")
	cmd.Env = append(append(os.Environ(), "CHOIR_HARDENING_STEP="+step), env...)
	cmd.SysProcAttr = attr
	cmd.ExtraFiles = extraFiles
	raw, err := cmd.CombinedOutput()
	if err != nil {
		if attr != nil && attr.Cloneflags != 0 && strings.Contains(string(raw)+err.Error(), "operation not permitted") {
			t.Skipf("cannot create a user and mount namespace here: %v", err)
		}
		t.Fatalf("child %s: %v\n%s", step, err, raw)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "RESULT "); ok {
			var out map[string]json.RawMessage
			if err := json.Unmarshal([]byte(rest), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			return out
		}
	}
	t.Fatalf("child %s printed no result:\n%s", step, raw)
	return nil
}

func jsonString(m map[string]json.RawMessage, k string) string {
	var s string
	_ = json.Unmarshal(m[k], &s)
	return s
}

// TestIsolateCellMountsLeavesNoSysDevAndAReducedProc: in a user and mount namespace of
// its own (root there, as the spawner's ambient capability makes the worker),
// the worker builds its root, drops its capabilities on every thread, and
// sees no /sys or /dev tree and a /proc of its own pids only; the allowed root is read-only, its tmp
// dir writable and the root sealed.
func TestIsolateCellMountsLeavesNoSysDevAndAReducedProc(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "root")
	tmp := filepath.Join(base, "tmp")
	for _, d := range []string{root, tmp} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("cells can read this"), 0o644); err != nil {
		t.Fatal(err)
	}
	attr := &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	res := runHardeningChild(t, "mounts", attr, nil, "CHOIR_HARDENING_ROOT="+root, "CHOIR_HARDENING_TMP="+tmp)
	if e := jsonString(res, "error"); e != "" {
		if strings.Contains(e, "operation not permitted") || strings.Contains(e, "permission denied") {
			t.Skipf("this kernel does not let a user namespace build the root: %s", e)
		}
		t.Fatalf("isolate: %s", e)
	}
	for _, k := range []string{"proc_err", "sys_err", "dev_mem_err"} {
		if !strings.Contains(jsonString(res, k), "no such file") {
			t.Errorf("%s = %q, want the path to be absent", k, jsonString(res, k))
		}
	}
	var dev []string
	_ = json.Unmarshal(res["dev"], &dev)
	sort.Strings(dev)
	if got := strings.Join(dev, ","); got != "null,urandom" && got != "null" {
		t.Errorf("/dev = %v, want only null and urandom", dev)
	}
	if e := jsonString(res, "read_root_err"); e != "" {
		t.Errorf("allowed root unreadable: %s", e)
	}
	if e := jsonString(res, "write_root_err"); !strings.Contains(e, "read-only") {
		t.Errorf("write to the allowed root = %q, want a read-only file system", e)
	}
	if e := jsonString(res, "write_tmp_err"); e != "" {
		t.Errorf("tmp dir not writable: %s", e)
	}
	if e := jsonString(res, "write_newroot_err"); e == "" {
		t.Error("the new root accepted a write")
	}
	if e := jsonString(res, "dev_null_err"); e != "" {
		t.Errorf("/dev/null unreadable: %s", e)
	}
	if got := jsonString(res, "cwd"); got != tmp {
		t.Errorf("cwd = %q, want %q", got, tmp)
	}
	if string(res["cap_eff"]) != "0" || string(res["cap_prm"]) != "0" || string(res["capget_errno"]) != "0" {
		t.Errorf("capabilities after the drop: eff=%s prm=%s errno=%s", res["cap_eff"], res["cap_prm"], res["capget_errno"])
	}
}

// TestCloseInheritedFDsKeepsOnlyTheSessionSocket: a descriptor that crossed
// exec without close-on-exec is closed; fd 3 is kept.
func TestCloseInheritedFDsKeepsOnlyTheSessionSocket(t *testing.T) {
	var files []*os.File
	for i := 0; i < 4; i++ { // fd 3 (the socket stand-in) and three leaks
		f, err := os.Open(os.Args[0])
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		files = append(files, f)
	}
	res := runHardeningChild(t, "fds", nil, files)
	var after []string
	_ = json.Unmarshal(res["after"], &after)
	if strings.Join(after, ",") != "3" {
		t.Errorf("files of the test binary still open after CloseInheritedFDs: %v, want only fd 3", after)
	}
}

// TestApplyCellRlimits: the limits beyond the address space are in force.
func TestApplyCellRlimits(t *testing.T) {
	res := runHardeningChild(t, "rlimits", nil, nil)
	if e := jsonString(res, "error"); e != "" {
		t.Fatal(e)
	}
	var lim map[string][2]uint64
	_ = json.Unmarshal(res["limits"], &lim)
	want := map[string][2]uint64{
		"core": {0, 0}, "nofile": {cellMaxOpenFiles, cellMaxOpenFiles},
		"fsize": {cellMaxFileBytes, cellMaxFileBytes}, "memlock": {0, 0}, "msgqueue": {0, 0},
	}
	for k, w := range want {
		if lim[k] != w {
			t.Errorf("rlimit %s = %v, want %v", k, lim[k], w)
		}
	}
}

// TestActivationUIDsAreUniqueAcrossLiveWorkers: concurrent acquires never
// collide; a released uid is free again.
func TestActivationUIDsAreUniqueAcrossLiveWorkers(t *testing.T) {
	p := &uidPool{live: map[uint32]bool{}}
	var mu sync.Mutex
	seen := map[uint32]bool{}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid, _, err := p.acquire()
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if seen[uid] {
				t.Errorf("uid %d handed out twice", uid)
			}
			seen[uid] = true
			if uid < activationUIDBase || uid >= activationUIDBase+activationUIDCount {
				t.Errorf("uid %d outside the pool", uid)
			}
		}()
	}
	wg.Wait()
	uid, release, err := p.acquire()
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // idempotent
	p.mu.Lock()
	if p.live[uid] {
		t.Error("released uid still live")
	}
	p.mu.Unlock()
}
