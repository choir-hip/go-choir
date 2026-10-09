//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/problems/capsule-session-worker-dies-at-start-2026-10-09.md.
// Failure mode: a worker that exits before its ready handshake (a fatal in
// the hardening floor) leaves only "read header: EOF"; its stderr, which
// names the cause, is discarded.
func TestSessionWorkerStartFailureCarriesStderr(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "dying-worker")
	script := "#!/bin/sh\necho 'session worker seccomp: load filter: invalid argument' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := spawnSessionWorker(bin, workerSessionConfig{
		computerID: "c", epoch: 1, activation: "a", allowedRoot: t.TempDir(), role: "engineering", slot: "s", hardened: true,
	})
	if err == nil {
		t.Fatal("a worker that exits before ready was accepted")
	}
	if !strings.Contains(err.Error(), "seccomp: load filter: invalid argument") {
		t.Fatalf("start failure does not carry the worker's stderr: %v", err)
	}
}
