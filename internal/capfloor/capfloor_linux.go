//go:build linux

package capfloor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Apply runs at the top of the daemon's main, before anything else starts.
// Off and observe return nil; enforce either returns nil with every thread
// at the floor or an error (the daemon must not start), or never returns
// (it re-executes the binary).
func Apply(mode Mode) error {
	if mode != ModeObserve && mode != ModeEnforce {
		return nil
	}
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return fmt.Errorf("capfloor: %w", err)
	}
	bnd, err := parseCapBnd(string(raw))
	if err != nil {
		return fmt.Errorf("capfloor: %w", err)
	}
	extra, missing := Plan(bnd)
	if len(missing) != 0 {
		log.Printf("capfloor: the bounding set lacks %s, which the inventory keeps", strings.Join(Names(missing), " "))
	}
	if mode == ModeObserve {
		log.Printf("capfloor: observe: enforce would drop %d capabilities from the bounding set %s: %s", len(extra), Hex(bnd), strings.Join(Names(extra), " "))
		return nil
	}
	if os.Getenv(appliedEnv) == "1" {
		// K3, K4: the re-executed image verifies and never re-executes.
		if err := verifyThreads(); err != nil {
			return err
		}
		log.Printf("capfloor: enforced: bounding set %s", Hex(bnd))
		return nil
	}
	if len(extra) == 0 {
		log.Printf("capfloor: enforce: the bounding set %s is already within the floor", Hex(bnd))
		return nil
	}
	// K2: one locked thread drops, then execs; the new image inherits it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := dropInheritable(); err != nil {
		return err
	}
	for _, c := range extra {
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(c), 0, 0, 0); err != nil {
			return fmt.Errorf("capfloor: drop %s from the bounding set: %w", Name(c), err)
		}
	}
	if err := os.Setenv(appliedEnv, "1"); err != nil {
		return fmt.Errorf("capfloor: %w", err)
	}
	err = syscall.Exec("/proc/self/exe", os.Args, os.Environ())
	return fmt.Errorf("capfloor: re-exec after the drop: %w", err)
}

// dropInheritable clears every inheritable capability outside the floor: a
// root exec's permitted set is its inheritable set joined with the bounding
// set, so an inheritable capability would survive the bounding drop.
func dropInheritable() error {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capfloor: capget: %w", err)
	}
	keep := keptMask()
	data[0].Inheritable &= uint32(keep)
	data[1].Inheritable &= uint32(keep >> 32)
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		return fmt.Errorf("capfloor: capset: %w", err)
	}
	return nil
}

// verifyThreads refuses a start in which any thread holds a capability
// outside the floor.
func verifyThreads() error {
	tasks, err := filepath.Glob("/proc/self/task/*/status")
	if err != nil || len(tasks) == 0 {
		return fmt.Errorf("capfloor: cannot list this process's threads (%v)", err)
	}
	for _, path := range tasks {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue // a thread that ended
		}
		bnd, err := parseCapBnd(string(raw))
		if err != nil {
			return fmt.Errorf("capfloor: %s: %w", path, err)
		}
		if extra, _ := Plan(bnd); len(extra) != 0 {
			return fmt.Errorf("capfloor: after the re-exec %s still holds %s", path, strings.Join(Names(extra), " "))
		}
	}
	return nil
}
