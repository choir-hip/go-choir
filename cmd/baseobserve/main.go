package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const (
	journalPathEnv        = "BASE_API_JOURNAL_PATH"
	blobRootEnv           = "BASE_API_BLOB_ROOT"
	codeRefEnv            = "BASE_OBSERVE_CODE_REF"
	artifactProgramRefEnv = "BASE_OBSERVE_ARTIFACT_PROGRAM_REF"
	defaultName           = "base-current-state"
)

type config struct {
	journalPath        string
	blobRoot           string
	codeRef            string
	artifactProgramRef string
	name               string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	set, err := observe(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(stderr, "baseobserve: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(set); err != nil {
		fmt.Fprintf(stderr, "baseobserve: encode observation set: %v\n", err)
		return 1
	}
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("baseobserve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{}
	fs.StringVar(&cfg.journalPath, "journal", os.Getenv(journalPathEnv), "existing Base SQLite journal path opened read-only")
	fs.StringVar(&cfg.blobRoot, "blob-root", os.Getenv(blobRootEnv), "existing Base blob store root opened without creation")
	fs.StringVar(&cfg.codeRef, "code-ref", os.Getenv(codeRefEnv), "CodeRef for the observed ComputerVersion")
	fs.StringVar(&cfg.artifactProgramRef, "artifact-program-ref", os.Getenv(artifactProgramRefEnv), "ArtifactProgramRef for the observed ComputerVersion")
	fs.StringVar(&cfg.name, "name", defaultName, "evidence label for the emitted ObservationSet")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.journalPath) == "" {
		return fmt.Errorf("baseobserve: --journal or %s is required", journalPathEnv)
	}
	if strings.TrimSpace(cfg.blobRoot) == "" {
		return fmt.Errorf("baseobserve: --blob-root or %s is required", blobRootEnv)
	}
	if strings.TrimSpace(cfg.codeRef) == "" {
		return fmt.Errorf("baseobserve: --code-ref or %s is required", codeRefEnv)
	}
	if strings.TrimSpace(cfg.artifactProgramRef) == "" {
		return fmt.Errorf("baseobserve: --artifact-program-ref or %s is required", artifactProgramRefEnv)
	}
	if strings.TrimSpace(cfg.name) == "" {
		return fmt.Errorf("baseobserve: --name is required")
	}
	return nil
}

func observe(ctx context.Context, cfg config) (computerversion.ObservationSet, error) {
	if err := cfg.validate(); err != nil {
		return computerversion.ObservationSet{}, err
	}
	source, err := computerversion.OpenBaseCurrentStateSource(computerversion.BaseCurrentStatePaths{
		JournalPath: cfg.journalPath,
		BlobRoot:    cfg.blobRoot,
	})
	if err != nil {
		return computerversion.ObservationSet{}, err
	}
	defer source.Close()
	return source.ObservationSet(ctx, cfg.name, computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(cfg.codeRef),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(cfg.artifactProgramRef),
	})
}
