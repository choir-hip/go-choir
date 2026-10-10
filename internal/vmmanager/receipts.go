package vmmanager

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Destruction receipts (docs/vmctl-360-review-2026-10-10.md, Phase 0 step 1).
// Every Firecracker process vmmanager kills leaves an append-only record of
// which VM and process, which kill path (cause), and the call chain that asked,
// so a destroyed computer is attributable from the host. The 10-10 kill of a
// busy self-development computer could not be traced to a caller because no
// kill recorded who asked
// (docs/problems/vmctl-restart-reboots-busy-computer-2026-10-10.md).
//
// A receipt is evidence, not authority: writing it never gates the kill, and a
// write failure is logged rather than blocking it.

// DestructionReceipt is one killed process.
type DestructionReceipt struct {
	At      string   `json:"at"`
	VMID    string   `json:"vm_id"`
	PID     int      `json:"pid"`
	Cause   string   `json:"cause"`
	Epoch   int64    `json:"epoch,omitempty"`
	HostURL string   `json:"host_url,omitempty"`
	Caller  string   `json:"caller,omitempty"`
	Stack   []string `json:"stack,omitempty"`
}

const (
	defaultDestructionReceiptMaxBytes = 16 << 20
	destructionReceiptStackDepth      = 24
	goChoirModulePrefix               = "github.com/yusefmosiah/go-choir/"
)

func (m *Manager) destructionReceiptPath() string {
	return filepath.Join(m.cfg.StateDir, "receipts", "destructive.jsonl")
}

// killProcessWithReceipt is the only place vmmanager kills a process. It
// records the receipt first, then kills.
func (m *Manager) killProcessWithReceipt(proc *os.Process, receipt DestructionReceipt) {
	if proc == nil {
		return
	}
	receipt.PID = proc.Pid
	receipt.Stack, receipt.Caller = destructionCallChain()
	m.writeDestructionReceipt(receipt)
	log.Printf("vmmanager: destructive kill vm=%s pid=%d cause=%s epoch=%d caller=%s", receipt.VMID, receipt.PID, receipt.Cause, receipt.Epoch, receipt.Caller)
	if err := proc.Kill(); err != nil && !strings.Contains(err.Error(), "process already finished") {
		log.Printf("vmmanager: kill vm=%s pid=%d cause=%s: %v", receipt.VMID, receipt.PID, receipt.Cause, err)
	}
}

func (m *Manager) writeDestructionReceipt(receipt DestructionReceipt) {
	if receipt.At == "" {
		receipt.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(receipt)
	if err != nil {
		log.Printf("vmmanager: destruction receipt for vm=%s: %v", receipt.VMID, err)
		return
	}
	line = append(line, '\n')
	path := m.destructionReceiptPath()
	limit := m.receiptMaxBytes
	if limit <= 0 {
		limit = defaultDestructionReceiptMaxBytes
	}

	m.receiptMu.Lock()
	defer m.receiptMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		log.Printf("vmmanager: destruction receipt for vm=%s not written: %v", receipt.VMID, err)
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size()+int64(len(line)) > limit {
		if err := os.Rename(path, path+".1"); err != nil {
			log.Printf("vmmanager: rotate destruction receipts: %v", err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		log.Printf("vmmanager: destruction receipt for vm=%s not written: %v", receipt.VMID, err)
		return
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		log.Printf("vmmanager: destruction receipt for vm=%s not written: %v", receipt.VMID, err)
		return
	}
	_ = f.Sync()
}

// destructionCallChain returns the go-choir call chain above the kill
// primitive and its first frame outside vmmanager, the code that asked.
func destructionCallChain() ([]string, string) {
	pcs := make([]uintptr, destructionReceiptStackDepth)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var stack []string
	caller := ""
	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, goChoirModulePrefix) {
			name := strings.TrimPrefix(strings.TrimPrefix(frame.Function, goChoirModulePrefix), "internal/")
			stack = append(stack, name)
			if caller == "" && !strings.HasPrefix(name, "vmmanager.") {
				caller = name
			}
		}
		if !more {
			break
		}
	}
	if caller == "" && len(stack) > 0 {
		caller = stack[len(stack)-1]
	}
	return stack, caller
}
