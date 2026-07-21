package receiptsigner

import (
	"fmt"
	"os"
	"path/filepath"
)

type StartupStage string

const (
	StartupStageStarted           StartupStage = "started"
	StartupStageConfigured        StartupStage = "configured"
	StartupStageKeyLoaded         StartupStage = "key-loaded"
	StartupStageHandlerConfigured StartupStage = "handler-configured"
	StartupStageSocketListening   StartupStage = "socket-listening"
)

func WriteStartupStage(path string, stage StartupStage) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || path == string(os.PathSeparator) || !validStartupStage(stage) {
		return fmt.Errorf("receipt signer: absolute startup status path and valid stage are required")
	}
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("receipt signer: startup status directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("receipt signer: startup status directory must be a directory")
	}
	temporary, err := os.CreateTemp(directory, ".startup-stage-*")
	if err != nil {
		return fmt.Errorf("receipt signer: create startup status: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("receipt signer: startup status permissions: %w", err)
	}
	if _, err := fmt.Fprintf(temporary, "%s\n", stage); err != nil {
		temporary.Close()
		return fmt.Errorf("receipt signer: write startup status: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("receipt signer: close startup status: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("receipt signer: publish startup status: %w", err)
	}
	return nil
}

func validStartupStage(stage StartupStage) bool {
	switch stage {
	case StartupStageStarted, StartupStageConfigured, StartupStageKeyLoaded, StartupStageHandlerConfigured, StartupStageSocketListening:
		return true
	default:
		return false
	}
}
