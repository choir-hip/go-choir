//go:build unix

package receiptsigner

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStartupLeaseTracksProcessLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "startup.lease")
	command := exec.Command(os.Args[0], "-test.run=^TestStartupLeaseHelperProcess$")
	command.Env = append(os.Environ(), "CHOIR_STARTUP_LEASE_HELPER=1", "CHOIR_STARTUP_LEASE_PATH="+path)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("wait for helper lease: %v", err)
	}
	if line != "ready\n" {
		t.Fatalf("helper readiness = %q", line)
	}
	held, err := StartupLeaseHeld(path)
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("startup lease reported unheld while helper process is alive")
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	stopped = true
	held, err = StartupLeaseHeld(path)
	if err != nil {
		t.Fatal(err)
	}
	if held {
		t.Fatal("startup lease remained held after helper process exit")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("startup lease mode = %o, want 600", got)
	}
}

func TestStartupLeaseHelperProcess(t *testing.T) {
	if os.Getenv("CHOIR_STARTUP_LEASE_HELPER") != "1" {
		return
	}
	lease, err := AcquireStartupLease(os.Getenv("CHOIR_STARTUP_LEASE_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	fmt.Println("ready")
	time.Sleep(30 * time.Second)
	runtime.KeepAlive(lease)
}
