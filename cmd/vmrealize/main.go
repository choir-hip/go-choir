package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const defaultMaterializer = "firecracker-vmmanager-scoped"

type config struct {
	id                 string
	materializer       string
	vmID               string
	persistentDir      string
	dataImagePath      string
	kernelImagePath    string
	rootfsPath         string
	storeDiskPath      string
	computerKind       string
	ownerID            string
	desktopID          string
	workerID           string
	candidateID        string
	epochRaw           string
	codeRef            string
	artifactProgramRef string
	requireExisting    bool
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
	realization, err := cfg.realization(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "vmrealize: %v\n", err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(realization); err != nil {
		fmt.Fprintf(stderr, "vmrealize: encode realization: %v\n", err)
		return 2
	}
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("vmrealize", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{materializer: defaultMaterializer, requireExisting: true}
	fs.StringVar(&cfg.id, "id", "", "Realization ID; defaults to --materializer")
	fs.StringVar(&cfg.materializer, "materializer", cfg.materializer, "declared VM materializer name")
	fs.StringVar(&cfg.vmID, "vm-id", "", "vmmanager VM ID to classify")
	fs.StringVar(&cfg.persistentDir, "persistent-dir", "", "existing per-VM persistent directory")
	fs.StringVar(&cfg.dataImagePath, "data-image", "", "existing per-VM data.img path")
	fs.StringVar(&cfg.kernelImagePath, "kernel-image", "", "kernel image path used by the scoped launch manifest")
	fs.StringVar(&cfg.rootfsPath, "rootfs", "", "root filesystem image path used by the scoped launch manifest")
	fs.StringVar(&cfg.storeDiskPath, "store-disk", "", "store disk image path used by the scoped launch manifest")
	fs.StringVar(&cfg.computerKind, "computer-kind", "", "product computer kind metadata")
	fs.StringVar(&cfg.ownerID, "owner-id", "", "owner metadata")
	fs.StringVar(&cfg.desktopID, "desktop-id", "", "desktop metadata")
	fs.StringVar(&cfg.workerID, "worker-id", "", "worker metadata")
	fs.StringVar(&cfg.candidateID, "candidate-id", "", "candidate metadata")
	fs.StringVar(&cfg.epochRaw, "epoch", "", "vm epoch metadata")
	fs.StringVar(&cfg.codeRef, "code-ref", "", "ComputerVersion CodeRef")
	fs.StringVar(&cfg.artifactProgramRef, "artifact-program-ref", "", "ComputerVersion ArtifactProgramRef")
	fs.BoolVar(&cfg.requireExisting, "require-existing", cfg.requireExisting, "require supplied persistent/data paths to exist before emitting a realization")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.materializer) == "" {
		return errors.New("vmrealize: --materializer is required")
	}
	if strings.TrimSpace(cfg.vmID) == "" {
		return errors.New("vmrealize: --vm-id is required")
	}
	if strings.TrimSpace(cfg.codeRef) == "" {
		return errors.New("vmrealize: --code-ref is required")
	}
	if strings.TrimSpace(cfg.artifactProgramRef) == "" {
		return errors.New("vmrealize: --artifact-program-ref is required")
	}
	if strings.TrimSpace(cfg.persistentDir) == "" && strings.TrimSpace(cfg.dataImagePath) == "" {
		return errors.New("vmrealize: --persistent-dir or --data-image is required")
	}
	if cfg.requireExisting {
		if err := requireDirIfSet("--persistent-dir", cfg.persistentDir); err != nil {
			return err
		}
		if err := requireFileIfSet("--data-image", cfg.dataImagePath); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.epochRaw) != "" {
		if _, err := strconv.ParseInt(strings.TrimSpace(cfg.epochRaw), 10, 64); err != nil {
			return fmt.Errorf("vmrealize: --epoch must be an int64: %w", err)
		}
	}
	return nil
}

func (cfg config) realization(ctx context.Context) (computerversion.Realization, error) {
	epoch, err := cfg.epoch()
	if err != nil {
		return computerversion.Realization{}, err
	}
	version := computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(strings.TrimSpace(cfg.codeRef)),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(strings.TrimSpace(cfg.artifactProgramRef)),
	}
	state := computerversion.VMManagerScopedPath{
		VMID:            cfg.vmID,
		PersistentDir:   cfg.persistentDir,
		DataImagePath:   cfg.dataImagePath,
		KernelImagePath: cfg.kernelImagePath,
		RootfsPath:      cfg.rootfsPath,
		StoreDiskPath:   cfg.storeDiskPath,
		ComputerKind:    cfg.computerKind,
		OwnerID:         cfg.ownerID,
		DesktopID:       cfg.desktopID,
		WorkerID:        cfg.workerID,
		CandidateID:     cfg.candidateID,
		Epoch:           epoch,
	}
	materializer := computerversion.VMManagerScopedMaterializer{ID: strings.TrimSpace(cfg.id), State: state}
	return materializer.Materialize(ctx, version, computerversion.VMManagerCapabilityManifest(cfg.materializer))
}

func (cfg config) epoch() (int64, error) {
	if strings.TrimSpace(cfg.epochRaw) == "" {
		return 0, nil
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(cfg.epochRaw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("vmrealize: --epoch must be an int64: %w", err)
	}
	return epoch, nil
}

func requireDirIfSet(flagName, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("vmrealize: %s %q: %w", flagName, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vmrealize: %s %q is not a directory", flagName, path)
	}
	return nil
}

func requireFileIfSet(flagName, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("vmrealize: %s %q: %w", flagName, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("vmrealize: %s %q is a directory", flagName, path)
	}
	return nil
}
