package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/cmdutil"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

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
		fmt.Fprintf(stderr, "basecompare: %v\n", err)
		return 2
	}
	result, err := computerversion.CompareBaseCurrentStateToFileProjection(context.Background(), left, right)
	if err != nil {
		fmt.Fprintf(stderr, "basecompare: %v\n", err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "basecompare: encode result: %v\n", err)
		return 2
	}
	if result.Equivalent() {
		return 0
	}
	return 1
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("basecompare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{leftPath: cmdutil.StdinPath}
	fs.StringVar(&cfg.leftPath, "left", cfg.leftPath, "Base current-state ObservationSet JSON path, or '-' for stdin")
	fs.StringVar(&cfg.rightPath, "right", "", "file-projection ObservationSet JSON path; defaults to --left")
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
		return errors.New("basecompare: --left is required")
	}
	if strings.TrimSpace(cfg.rightPath) == cmdutil.StdinPath && strings.TrimSpace(cfg.leftPath) == cmdutil.StdinPath {
		return errors.New("basecompare: --left and --right cannot both read stdin")
	}
	return nil
}

func loadObservationSets(cfg config, stdin io.Reader) (computerversion.ObservationSet, computerversion.ObservationSet, error) {
	return cmdutil.LoadObservationSets(cfg.leftPath, cfg.rightPath, stdin, "basecompare")
}
