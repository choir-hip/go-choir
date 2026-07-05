// Package cmdutil provides shared utilities for command-line binaries in the
// substrate-independent audited computer toolchain.  Functions here are pure
// helpers (file reading, flag parsing, path validation) that are duplicated
// across multiple cmd/* binaries.
package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

// StdinPath is the path value that indicates reading from stdin instead of a
// file.  Commands accept "-" as a sentinel for stdin input.
const StdinPath = "-"

// LoadObservationSets loads left and right observation sets from files or
// stdin.  If rightPath is empty, the left set is returned for both sides,
// allowing same-set comparison without duplicating input.
func LoadObservationSets(leftPath, rightPath string, stdin io.Reader, cmdName string) (computerversion.ObservationSet, computerversion.ObservationSet, error) {
	left, err := ReadObservationSet(leftPath, stdin)
	if err != nil {
		return computerversion.ObservationSet{}, computerversion.ObservationSet{}, fmt.Errorf("%s: read left observation set: %w", cmdName, err)
	}
	if strings.TrimSpace(rightPath) == "" {
		return left, left, nil
	}
	right, err := ReadObservationSet(rightPath, stdin)
	if err != nil {
		return computerversion.ObservationSet{}, computerversion.ObservationSet{}, fmt.Errorf("%s: read right observation set: %w", cmdName, err)
	}
	return left, right, nil
}

// ReadObservationSet reads a single observation set from a file path or stdin
// (when path is StdinPath).  Unknown JSON fields are rejected.
func ReadObservationSet(path string, stdin io.Reader) (computerversion.ObservationSet, error) {
	var reader io.Reader
	if strings.TrimSpace(path) == StdinPath {
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return computerversion.ObservationSet{}, err
		}
		defer file.Close()
		reader = file
	}
	var set computerversion.ObservationSet
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		return computerversion.ObservationSet{}, err
	}
	return set, nil
}

// ParseEpoch parses an epoch string to int64, returning 0 if empty.  The
// cmdName is used as a prefix in error messages.
func ParseEpoch(epochRaw, cmdName string) (int64, error) {
	if strings.TrimSpace(epochRaw) == "" {
		return 0, nil
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(epochRaw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: --epoch must be an int64: %w", cmdName, err)
	}
	return epoch, nil
}

// RequireDirIfSet validates that a path is an existing directory if non-empty.
// The flagName and cmdName are used in error messages.
func RequireDirIfSet(flagName, path, cmdName string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %s %q: %w", cmdName, flagName, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %s %q is not a directory", cmdName, flagName, path)
	}
	return nil
}

// RequireFileIfSet validates that a path is an existing file if non-empty.
// The flagName and cmdName are used in error messages.
func RequireFileIfSet(flagName, path, cmdName string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: %s %q: %w", cmdName, flagName, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s: %s %q is a directory", cmdName, flagName, path)
	}
	return nil
}
