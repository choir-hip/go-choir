package vmmanager

import (
	"bufio"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phase 0 step 1 of docs/vmctl-360-review-2026-10-10.md: every Firecracker
// process vmmanager kills leaves an attributable receipt. Today's 09:25:48
// kill (docs/problems/vmctl-restart-reboots-busy-computer-2026-10-10.md) could
// not be traced to a caller because no kill recorded who asked.
//
// Ways the receipt can fail:
//  1. A kill path bypasses it (today's ten-plus sites, or one added later).
//  2. It does not name the kill path (cause) or the code that asked (caller).
//  3. A receipt write failure blocks the kill or panics.
//  4. The receipt file grows without bound on a long-lived host.

func readReceipts(t *testing.T, m *Manager) []DestructionReceipt {
	t.Helper()
	f, err := os.Open(m.destructionReceiptPath())
	if err != nil {
		t.Fatalf("open receipts: %v", err)
	}
	defer f.Close()
	var out []DestructionReceipt
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r DestructionReceipt
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatalf("receipt line %q: %v", scanner.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleeper: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd
}

func TestEveryProcessKillGoesThroughTheReceipt(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				isKill := sel.Sel.Name == "Kill" && len(call.Args) == 0
				// syscall.Signal(n) is a type conversion, not a method call.
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "syscall" {
					return true
				}
				isSignal := sel.Sel.Name == "Signal" && len(call.Args) == 1 && !isSignalZero(call.Args[0])
				if (isKill || isSignal) && fn.Name.Name != "killProcessWithReceipt" {
					t.Errorf("%s: %s calls %s outside killProcessWithReceipt; every kill must leave a receipt", fset.Position(call.Pos()), fn.Name.Name, sel.Sel.Name)
				}
				return true
			})
		}
	}
}

// isSignalZero reports a liveness probe, syscall.Signal(0), which kills nothing.
func isSignalZero(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Value == "0"
}

func askingCallerForReceiptTest(m *Manager, inst *VMInstance) {
	m.killFirecrackerProcess(inst, "refresh")
}

func TestKillReceiptNamesCauseCallerAndRealization(t *testing.T) {
	m := NewManager(ManagerConfig{StateDir: t.TempDir()})
	cmd := startSleeper(t)
	inst := &VMInstance{Config: VMConfig{VMID: "vm-receipt", Epoch: 7}, HostURL: "http://10.200.9.2:8085", PID: cmd.Process.Pid}

	askingCallerForReceiptTest(m, inst)

	if err := waitExit(cmd, 5*time.Second); err != nil {
		t.Fatalf("process not killed: %v", err)
	}
	receipts := readReceipts(t, m)
	if len(receipts) != 1 {
		t.Fatalf("receipts = %+v, want exactly one", receipts)
	}
	r := receipts[0]
	if r.VMID != "vm-receipt" || r.PID != cmd.Process.Pid || r.Cause != "refresh" || r.Epoch != 7 || r.HostURL != "http://10.200.9.2:8085" || r.At == "" {
		t.Fatalf("receipt = %+v", r)
	}
	if !strings.Contains(strings.Join(r.Stack, " "), "askingCallerForReceiptTest") {
		t.Fatalf("receipt stack does not name the asking code: %v", r.Stack)
	}
}

func TestKillReceiptWriteFailureStillKills(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "state")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(ManagerConfig{StateDir: blocker})
	cmd := startSleeper(t)
	m.killFirecrackerProcess(&VMInstance{Config: VMConfig{VMID: "vm-unwritable"}, PID: cmd.Process.Pid}, "stop")
	if err := waitExit(cmd, 5*time.Second); err != nil {
		t.Fatalf("a receipt write failure blocked the kill: %v", err)
	}
}

func TestKillReceiptFileIsBounded(t *testing.T) {
	m := NewManager(ManagerConfig{StateDir: t.TempDir()})
	m.receiptMaxBytes = 600
	for i := 0; i < 20; i++ {
		m.writeDestructionReceipt(DestructionReceipt{VMID: "vm-bounded", PID: 100 + i, Cause: "stop", Stack: []string{"vmmanager.(*Manager).StopVM"}})
	}
	info, err := os.Stat(m.destructionReceiptPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > m.receiptMaxBytes {
		t.Fatalf("receipt file %d bytes exceeds bound %d", info.Size(), m.receiptMaxBytes)
	}
	if _, err := os.Stat(m.destructionReceiptPath() + ".1"); err != nil {
		t.Fatalf("rotated receipt file missing: %v", err)
	}
}

func waitExit(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return os.ErrDeadlineExceeded
	}
}
