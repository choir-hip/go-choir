//go:build !linux

package vmmanager

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"time"
)

// Off Linux there is no Firecracker: these keep the package building and the
// tests running. The thread policy is reported unsupported.

func startWithThreadPolicy(cmd *exec.Cmd, _ threadPolicy) (LaunchThreadReport, error) {
	return LaunchThreadReport{Supported: false}, cmd.Start()
}

func watchProcessExit(pid int) (<-chan struct{}, error) {
	if !processExists(pid) {
		return nil, errors.New("process does not exist")
	}
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		for processExists(pid) {
			time.Sleep(200 * time.Millisecond)
		}
	}()
	return ch, nil
}

func cloneFile(_, _ *os.File) error { return errors.ErrUnsupported }

func dialUnixSocket(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}
