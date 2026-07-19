//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestResolveWithinSafe(t *testing.T) {
	base := "/mnt/merged"

	cases := []struct {
		rel  string
		want string
		ok   bool
	}{
		{"file.txt", "/mnt/merged/file.txt", true},
		{"subdir/file.txt", "/mnt/merged/subdir/file.txt", true},
		{"/", "/mnt/merged", true},
		{".", "/mnt/merged", true},
		{"subdir/../file.txt", "/mnt/merged/file.txt", true},
		// Traversal attempts — must fail.
		{"../../etc/passwd", "", false},
		{"../etc/shadow", "", false},
		{"subdir/../../etc/passwd", "", false},
		{"..", "", false},
	}

	for _, tc := range cases {
		got, err := resolveWithin(base, tc.rel)
		if tc.ok {
			if err != nil {
				t.Errorf("resolveWithin(%q, %q): unexpected error: %v", base, tc.rel, err)
				continue
			}
			want := filepath.Clean(tc.want)
			if got != want {
				t.Errorf("resolveWithin(%q, %q) = %q, want %q", base, tc.rel, got, want)
			}
		} else {
			if err == nil {
				t.Errorf("resolveWithin(%q, %q): expected error, got %q", base, tc.rel, got)
			}
		}
	}
}

func TestBrokerAuthenticatedRPCReadiness(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(t.TempDir(), "broker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	broker := &Broker{
		socketPath: socketPath, capsuleID: "capsule-ready", publicKey: publicKey,
		mergedDir: t.TempDir(), sessions: make(map[string]*Session), revokedCaps: make(map[string]bool),
		authorizedPeerUID: uint32(os.Geteuid()), listener: listener,
	}
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			broker.handleConnection(conn)
		}
	}()
	capability := &capsule.Capability{
		CapabilityID: "readiness", Handle: "readiness", CapsuleID: broker.capsuleID,
		AgentRunID: "guest-core-readiness", AgentRole: capsule.RoleResearcher,
		TargetCapsule: broker.capsuleID, Verbs: capsule.RoleVerbSets[capsule.RoleResearcher],
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := capsule.SignCapability(capability, privateKey, "test"); err != nil {
		t.Fatal(err)
	}
	client := capsule.NewBrokerClient(socketPath, publicKey)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Stat(ctx, capability, "."); err != nil {
		t.Fatalf("authenticated readiness RPC failed: %v", err)
	}
}
