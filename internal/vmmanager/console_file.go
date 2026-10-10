package vmmanager

import (
	"fmt"
	"io"
	"log"
	"os"
	"syscall"
	"time"
)

// Firecracker's console is a file, not a pipe
// (docs/problems/firecracker-sigpipe-deadlock-after-vmctl-restart-2026-10-10.md).
// A pipe needs a reader, and its reader was a goroutine in the vmctl that
// launched the VM. Once that vmctl exited, the guest's next console write
// raised SIGPIPE in firecracker, whose handler deadlocked the vCPU. A file
// needs no reader and outlives every vmctl.

const consoleBoundInterval = time.Minute

// openConsoleSink opens the console log for firecracker's stdout, appending.
// A log already over its bound is rotated first.
func openConsoleSink(path string, maxBytes int64, generations int) (*os.File, error) {
	if err := boundConsoleLog(path, maxBytes, generations); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open console log %s: %w", path, err)
	}
	return file, nil
}

// boundConsoleLog keeps the console log under maxBytes while firecracker
// holds it open: it copies the log to the first generation and truncates the
// log in place. Renaming would leave firecracker writing the renamed file.
// Firecracker appends, so its next write lands at the new end.
func boundConsoleLog(path string, maxBytes int64, generations int) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat console log %s: %w", path, err)
	}
	if info.Size() < maxBytes {
		return nil
	}
	if generations > 0 {
		for generation := generations; generation >= 2; generation-- {
			source := fmt.Sprintf("%s.%d", path, generation-1)
			destination := fmt.Sprintf("%s.%d", path, generation)
			if err := os.Rename(source, destination); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("rotate console log %s: %w", path, err)
			}
		}
		if err := copyConsoleFile(path, path+".1"); err != nil {
			return err
		}
	}
	if err := os.Truncate(path, 0); err != nil {
		return fmt.Errorf("truncate console log %s: %w", path, err)
	}
	return nil
}

func copyConsoleFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open console log %s: %w", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open console generation %s: %w", destination, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy console log %s: %w", source, err)
	}
	return out.Close()
}

// watchConsoleLog bounds a VM's console log until its process exits.
func watchConsoleLog(vmID string, pid int, path string, exited <-chan struct{}) {
	ticker := time.NewTicker(consoleBoundInterval)
	defer ticker.Stop()
	for {
		select {
		case <-exited:
			return
		case <-ticker.C:
			if !processExists(pid) {
				return
			}
			if err := boundConsoleLog(path, consoleLogMaxBytes, consoleLogGenerations); err != nil {
				log.Printf("vmmanager: bound console log for VM %s: %v", vmID, err)
			}
		}
	}
}

// drainLegacyConsolePipe gives a console pipe a reader again. A VM launched
// before consoles were files writes to a pipe whose reader died with the
// vmctl that launched it; opening the pipe through /proc (fdPath is
// /proc/<pid>/fd/1) attaches a new reader that copies into the console log
// until firecracker exits. It reports false, and does nothing, when the
// console is not a pipe.
func drainLegacyConsolePipe(fdPath, logPath string, maxBytes int64, generations int) (bool, error) {
	info, err := os.Stat(fdPath)
	if err != nil {
		return false, fmt.Errorf("stat console %s: %w", fdPath, err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return false, nil
	}
	pipe, err := os.OpenFile(fdPath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false, fmt.Errorf("open console pipe %s: %w", fdPath, err)
	}
	sink, err := newRotatingConsoleWriter(logPath, maxBytes, generations)
	if err != nil {
		_ = pipe.Close()
		return false, err
	}
	go func() {
		defer pipe.Close()
		defer sink.Close()
		if _, err := io.Copy(sink, pipe); err != nil {
			log.Printf("vmmanager: drain console pipe %s: %v", fdPath, err)
		}
	}()
	return true, nil
}

// adoptConsole keeps a reattached VM's console safe: a legacy pipe gets a
// reader, a file console gets its bound.
func (m *Manager) adoptConsole(vmID string, pid int) {
	path := consoleLogPath(m.cfg.StateDir, vmID)
	drained, err := drainLegacyConsolePipe(fmt.Sprintf("/proc/%d/fd/1", pid), path, consoleLogMaxBytes, consoleLogGenerations)
	if err != nil {
		log.Printf("vmmanager: adopt console for VM %s (pid=%d): %v", vmID, pid, err)
		return
	}
	if drained {
		log.Printf("vmmanager: VM %s (pid=%d) console pipe has a reader again", vmID, pid)
		return
	}
	go watchConsoleLog(vmID, pid, path, nil)
}
