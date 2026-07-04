package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/types"
	"io"
	"os"
	"strings"
)

const (
	defaultPackageKind = computerversion.CandidateComputerPackageKind
	outputPackage      = "package"
	outputBridge       = "bridge"
	outputAcceptance   = "acceptance"
	outputIntake       = "intake"
)

type config struct {
	id                 string
	evidenceRootOutput string
	realizationFiles   stringList
	sourceComputerID   string
	sourceCandidateID  string
	candidateSourceRef string
	evidenceRefs       stringList
	ownerID            string
	traceID            string
	output             string
}

type evidenceRootOutput struct {
	Manifest    computerversion.CandidateEvidenceRootManifest `json:"manifest"`
	Observation computerversion.ObservationSet                `json:"observation"`
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*s = append(*s, value)
	}
	return nil
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
	pkg, err := cfg.packageManifest()
	if err != nil {
		fmt.Fprintf(stderr, "candidatepackage: %v\n", err)
		return 1
	}
	payload := any(pkg)
	switch cfg.output {
	case outputBridge, outputAcceptance, outputIntake:
		bridge, err := computerversion.BuildCandidatePackageAppChangeBridge(pkg)
		if err != nil {
			fmt.Fprintf(stderr, "candidatepackage: %v\n", err)
			return 1
		}
		payload = bridge
		if cfg.output == outputAcceptance || cfg.output == outputIntake {
			acceptance, err := computerversion.BuildCandidatePackageProductPathAcceptanceContract(pkg, bridge)
			if err != nil {
				fmt.Fprintf(stderr, "candidatepackage: %v\n", err)
				return 1
			}
			payload = acceptance
			if cfg.output == outputIntake {
				intake, err := cfg.intakeRecord(pkg, acceptance)
				if err != nil {
					fmt.Fprintf(stderr, "candidatepackage: %v\n", err)
					return 1
				}
				payload = intake
			}
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintf(stderr, "candidatepackage: encode output: %v\n", err)
		return 1
	}
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("candidatepackage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg config
	fs.StringVar(&cfg.id, "id", "", "candidate package id")
	fs.StringVar(&cfg.evidenceRootOutput, "evidence-root-output", "", "JSON output from cmd/evidenceroot")
	fs.Var(&cfg.realizationFiles, "realization", "JSON realization file from cmd/vmrealize; may be repeated")
	fs.StringVar(&cfg.sourceComputerID, "source-computer-id", "", "source computer id for review lineage")
	fs.StringVar(&cfg.sourceCandidateID, "source-candidate-id", "", "source candidate id for review lineage")
	fs.StringVar(&cfg.candidateSourceRef, "candidate-source-ref", "", "candidate source ref for review lineage")
	fs.Var(&cfg.evidenceRefs, "evidence-ref", "additional evidence artifact ref; may be repeated")
	fs.StringVar(&cfg.output, "output", outputPackage, "output format: package, bridge, acceptance, or intake")
	fs.StringVar(&cfg.ownerID, "owner-id", "", "owner id for intake output")
	fs.StringVar(&cfg.traceID, "trace-id", "", "optional trace id for intake output")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.id == "" {
		return config{}, fmt.Errorf("--id is required")
	}
	if cfg.evidenceRootOutput == "" {
		return config{}, fmt.Errorf("--evidence-root-output is required")
	}
	if len(cfg.realizationFiles) == 0 {
		return config{}, fmt.Errorf("at least one --realization is required")
	}
	cfg.output = strings.TrimSpace(cfg.output)
	switch cfg.output {
	case outputPackage, outputBridge, outputAcceptance, outputIntake:
	default:
		return config{}, fmt.Errorf("--output must be %q, %q, %q, or %q", outputPackage, outputBridge, outputAcceptance, outputIntake)
	}
	if cfg.output == outputIntake && strings.TrimSpace(cfg.ownerID) == "" {
		return config{}, fmt.Errorf("--owner-id is required when --output is %q", outputIntake)
	}
	return cfg, nil
}

func (cfg config) packageManifest() (computerversion.CandidateComputerPackageManifest, error) {
	rootOutput, err := readEvidenceRootOutput(cfg.evidenceRootOutput)
	if err != nil {
		return computerversion.CandidateComputerPackageManifest{}, err
	}
	realizations := make([]computerversion.Realization, 0, len(cfg.realizationFiles))
	for _, path := range cfg.realizationFiles {
		realization, err := readRealization(path)
		if err != nil {
			return computerversion.CandidateComputerPackageManifest{}, err
		}
		realizations = append(realizations, realization)
	}
	evidenceRefs := append([]string{cfg.evidenceRootOutput}, cfg.realizationFiles...)
	evidenceRefs = append(evidenceRefs, cfg.evidenceRefs...)
	manifest := computerversion.CandidateComputerPackageManifest{
		ID:                      cfg.id,
		Kind:                    defaultPackageKind,
		Version:                 rootOutput.Manifest.Fixture.Version,
		SourceComputerID:        cfg.sourceComputerID,
		SourceCandidateID:       cfg.sourceCandidateID,
		CandidateSourceRef:      cfg.candidateSourceRef,
		EvidenceRoot:            rootOutput.Manifest,
		EvidenceRootObservation: rootOutput.Observation,
		Realizations:            realizations,
		EvidenceRefs:            evidenceRefs,
	}
	return computerversion.BuildCandidateComputerPackage(manifest)
}

func (cfg config) intakeRecord(pkg computerversion.CandidateComputerPackageManifest, acceptance computerversion.CandidatePackageProductPathAcceptanceContract) (types.CandidatePackageIntakeRecord, error) {
	adoptionBlockersJSON, err := json.Marshal(acceptance.AdoptionBlockers)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("encode intake adoption blockers: %w", err)
	}
	contractsJSON, err := json.Marshal(acceptance.VerifierContracts)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("encode intake verifier contracts: %w", err)
	}
	evidenceRefsJSON, err := json.Marshal(acceptance.EvidenceRefs)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("encode intake evidence refs: %w", err)
	}
	requiredObservationsJSON, err := json.Marshal(acceptance.RequiredObservations)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("encode intake required observations: %w", err)
	}
	acceptanceJSON, err := json.Marshal(acceptance)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("encode intake acceptance: %w", err)
	}
	return types.CandidatePackageIntakeRecord{
		IntakeID:                       cfg.id,
		OwnerID:                        strings.TrimSpace(cfg.ownerID),
		CandidatePackageID:             pkg.ID,
		CandidatePackageManifestSHA256: pkg.PackageManifestSHA256,
		SourceComputerID:               pkg.SourceComputerID,
		SourceCandidateID:              pkg.SourceCandidateID,
		CandidateSourceRef:             pkg.CandidateSourceRef,
		IntakeBoundary:                 acceptance.IntakeBoundary,
		Status:                         types.CandidatePackageIntakeOwnerReviewPending,
		OwnerReviewState:               types.CandidatePackageOwnerReviewRequired,
		OwnerReviewRequired:            acceptance.OwnerReviewRequired,
		AdoptionReady:                  acceptance.AdoptionReady,
		AdoptionBlockersJSON:           adoptionBlockersJSON,
		VerifierContractsJSON:          contractsJSON,
		EvidenceRefsJSON:               evidenceRefsJSON,
		RequiredObservationsJSON:       requiredObservationsJSON,
		AcceptanceJSON:                 acceptanceJSON,
		TraceID:                        strings.TrimSpace(cfg.traceID),
	}, nil
}

func readEvidenceRootOutput(path string) (evidenceRootOutput, error) {
	var out evidenceRootOutput
	if err := readJSON(path, &out, false); err != nil {
		return evidenceRootOutput{}, fmt.Errorf("read evidence root output: %w", err)
	}
	return out, nil
}

func readRealization(path string) (computerversion.Realization, error) {
	var out computerversion.Realization
	if err := readJSON(path, &out, true); err != nil {
		return computerversion.Realization{}, fmt.Errorf("read realization %s: %w", path, err)
	}
	return out, nil
}

func readJSON(path string, out any, strict bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}
