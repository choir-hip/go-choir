package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/cmdutil"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const defaultName = "vmmanager-scoped-path"

type config struct {
	name               string
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
	set, err := cfg.observationSet()
	if err != nil {
		fmt.Fprintf(stderr, "vmstateobserve: %v\n", err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(set); err != nil {
		fmt.Fprintf(stderr, "vmstateobserve: encode observation set: %v\n", err)
		return 2
	}
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("vmstateobserve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{name: defaultName, requireExisting: true}
	fs.StringVar(&cfg.name, "name", cfg.name, "ObservationSet evidence label")
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
	fs.BoolVar(&cfg.requireExisting, "require-existing", cfg.requireExisting, "require supplied persistent/data paths to exist before emitting observations")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func (cfg config) validate() error {
	if strings.TrimSpace(cfg.vmID) == "" {
		return errors.New("vmstateobserve: --vm-id is required")
	}
	if strings.TrimSpace(cfg.codeRef) == "" {
		return errors.New("vmstateobserve: --code-ref is required")
	}
	if strings.TrimSpace(cfg.artifactProgramRef) == "" {
		return errors.New("vmstateobserve: --artifact-program-ref is required")
	}
	if strings.TrimSpace(cfg.persistentDir) == "" && strings.TrimSpace(cfg.dataImagePath) == "" {
		return errors.New("vmstateobserve: --persistent-dir or --data-image is required")
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
			return fmt.Errorf("vmstateobserve: --epoch must be an int64: %w", err)
		}
	}
	return nil
}

func (cfg config) observationSet() (computerversion.ObservationSet, error) {
	epoch, err := cfg.epoch()
	if err != nil {
		return computerversion.ObservationSet{}, err
	}
	path := computerversion.VMManagerScopedPath{
		VMID:               cfg.vmID,
		PersistentDir:      cfg.persistentDir,
		DataImagePath:      cfg.dataImagePath,
		KernelImagePath:    cfg.kernelImagePath,
		RootfsPath:         cfg.rootfsPath,
		StoreDiskPath:      cfg.storeDiskPath,
		ComputerKind:       cfg.computerKind,
		OwnerID:            cfg.ownerID,
		DesktopID:          cfg.desktopID,
		WorkerID:           cfg.workerID,
		CandidateID:        cfg.candidateID,
		Epoch:              epoch,
		DataImageClass:     computerversion.StateClassDurableLegacyOpaque,
		PersistentDirClass: computerversion.StateClassDurableLegacyOpaque,
		BootArtifactClass:  computerversion.StateClassCodeArtifact,
	}
	return path.ObservationSet(cfg.name, computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(strings.TrimSpace(cfg.codeRef)),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(strings.TrimSpace(cfg.artifactProgramRef)),
	})
}

func (cfg config) epoch() (int64, error) {
	return cmdutil.ParseEpoch(cfg.epochRaw, "vmstateobserve")
}

func requireDirIfSet(flagName, path string) error {
	return cmdutil.RequireDirIfSet(flagName, path, "vmstateobserve")
}

func requireFileIfSet(flagName, path string) error {
	return cmdutil.RequireFileIfSet(flagName, path, "vmstateobserve")
}
