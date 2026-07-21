package receiptsigner

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
)

type StartupStage string

const (
	StartupStageStarted                 StartupStage = "started"
	StartupStageConfigured              StartupStage = "configured"
	StartupStageKeyLoaded               StartupStage = "key-loaded"
	StartupStageHandlerConfigured       StartupStage = "handler-configured"
	StartupStageSocketListening         StartupStage = "socket-listening"
	StartupStageServeReturnedClosed     StartupStage = "serve-returned-closed"
	StartupStageServeReturnedPermission StartupStage = "serve-returned-permission"
	StartupStageServeReturnedInvalid    StartupStage = "serve-returned-invalid"
	StartupStageServeReturnedResource   StartupStage = "serve-returned-resource"
	StartupStageServeReturnedUnknown    StartupStage = "serve-returned-unknown"
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

func ServeExitIsFailure(err error) bool {
	return err != nil && !errors.Is(err, http.ErrServerClosed)
}

func ClassifyServeExit(err error) StartupStage {
	switch {
	case err == nil, errors.Is(err, http.ErrServerClosed), errors.Is(err, net.ErrClosed):
		return StartupStageServeReturnedClosed
	case errors.Is(err, os.ErrPermission), errors.Is(err, syscall.EPERM):
		return StartupStageServeReturnedPermission
	case errors.Is(err, syscall.EBADF), errors.Is(err, syscall.EINVAL):
		return StartupStageServeReturnedInvalid
	case errors.Is(err, syscall.EMFILE), errors.Is(err, syscall.ENFILE), errors.Is(err, syscall.ENOBUFS), errors.Is(err, syscall.ENOMEM):
		return StartupStageServeReturnedResource
	default:
		return StartupStageServeReturnedUnknown
	}
}

func validStartupStage(stage StartupStage) bool {
	switch stage {
	case StartupStageStarted, StartupStageConfigured, StartupStageKeyLoaded, StartupStageHandlerConfigured, StartupStageSocketListening,
		StartupStageServeReturnedClosed, StartupStageServeReturnedPermission, StartupStageServeReturnedInvalid,
		StartupStageServeReturnedResource, StartupStageServeReturnedUnknown:
		return true
	default:
		return false
	}
}
