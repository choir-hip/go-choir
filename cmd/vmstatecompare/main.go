package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const stdinPath = "-"

type config struct {
	leftPath  string
	rightPath string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	left, right, err := loadObservationSets(cfg, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "vmstatecompare: %v\n", err)
		return 2
	}
	result := compareVMStateObservationSets(left, right)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "vmstatecompare: encode result: %v\n", err)
		return 2
	}
	if result.Equivalent() {
		return 0
	}
	return 1
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("vmstatecompare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{leftPath: stdinPath}
	fs.StringVar(&cfg.leftPath, "left", cfg.leftPath, "left vm_state_manifest ObservationSet JSON path, or '-' for stdin")
	fs.StringVar(&cfg.rightPath, "right", "", "right vm_state_manifest ObservationSet JSON path; defaults to --left")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.leftPath) == "" {
		return errors.New("vmstatecompare: --left is required")
	}
	if strings.TrimSpace(cfg.leftPath) == stdinPath && strings.TrimSpace(cfg.rightPath) == stdinPath {
		return errors.New("vmstatecompare: --left and --right cannot both read stdin")
	}
	return nil
}

func loadObservationSets(cfg config, stdin io.Reader) (computerversion.ObservationSet, computerversion.ObservationSet, error) {
	left, err := readObservationSet(cfg.leftPath, stdin)
	if err != nil {
		return computerversion.ObservationSet{}, computerversion.ObservationSet{}, fmt.Errorf("read left observation set: %w", err)
	}
	if strings.TrimSpace(cfg.rightPath) == "" {
		return left, left, nil
	}
	right, err := readObservationSet(cfg.rightPath, stdin)
	if err != nil {
		return computerversion.ObservationSet{}, computerversion.ObservationSet{}, fmt.Errorf("read right observation set: %w", err)
	}
	return left, right, nil
}

func readObservationSet(path string, stdin io.Reader) (computerversion.ObservationSet, error) {
	var reader io.Reader
	if strings.TrimSpace(path) == stdinPath {
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

func compareVMStateObservationSets(left, right computerversion.ObservationSet) computerversion.EquivalenceResult {
	leftRealization := computerversion.Realization{
		ID:           "vmstatecompare-left",
		Version:      left.Version,
		Capabilities: computerversion.VMManagerCapabilityManifest("vmstatecompare-left"),
		Observations: left,
	}
	rightRealization := computerversion.Realization{
		ID:           "vmstatecompare-right",
		Version:      right.Version,
		Capabilities: computerversion.VMManagerCapabilityManifest("vmstatecompare-right"),
		Observations: right,
	}
	return computerversion.EquivalenceChecker{}.CheckRealizations(leftRealization, rightRealization)
}
