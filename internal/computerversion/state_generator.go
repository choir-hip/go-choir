package computerversion

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

const (
	// StateGeneratorMaterializer names the realization produced by the
	// state generator — the function that generates concrete filesystem
	// state from a ComputerVersion's typed artifact program.
	StateGeneratorMaterializer = "state-generator"
	// StateGeneratorSubstrate names the substrate-independent generation
	// substrate. The generator writes to any filesystem directory; the
	// substrate identity comes from where the output is placed, not from
	// the generator itself.
	StateGeneratorSubstrate = "substrate-independent/fs-generation"
)

// StateGenerator generates concrete filesystem state from a ComputerVersion.
// It is the generator function: the inverse of the extractor.
//
// The generator reads the journal's entries once, verifies the
// tamper-evident chain on that exact slice, derives a tree from the
// verified events, and writes the tree's files to a target directory
// using the blob store for file content.
//
// ArtifactProgram cryptographically binds the exact journal entry slice used
// for generation to the ComputerVersion's immutable ArtifactProgramRef.
//
// This bridges the gap between the abstract ComputerVersion and concrete
// substrate state. If the generator is correct, the extractor reads back
// what the generator wrote, and the observations match.
type StateGenerator struct {
	// Journal provides the typed event tape for the artifact program.
	Journal journal.Journal
	// Blobs provides file content by blob ref.
	Blobs *blob.Store
	// ArtifactProgram is the immutable resolver result for the version.
	ArtifactProgram ArtifactProgram
}

// Generate writes the ComputerVersion's durable state to targetDir.
//
// It reads the journal's entries once into a local slice, verifies the
// tamper-evident chain on that exact slice via verifyBaseJournalEntries
// (which is a stronger check than VerifyChain because it re-derives and
// re-checks hashes from the raw entries), and derives the tree from the
// verified ordered entries. This avoids a verify/read race where a live
// journal could change between a separate VerifyChain() call and the
// Entries() call.
func (g StateGenerator) Generate(ctx context.Context, version ComputerVersion, targetDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !version.Valid() {
		return fmt.Errorf("state generator: invalid computer version")
	}
	if g.Journal == nil {
		return fmt.Errorf("state generator: nil journal")
	}
	if g.Blobs == nil {
		return fmt.Errorf("state generator: nil blob store")
	}

	// Read entries once into a local slice. We verify and derive from
	// this exact slice to avoid a verify/read race where a live journal
	// could change between VerifyChain() and Entries().
	entries := g.Journal.Entries()
	ordered, err := verifyBaseJournalEntries(entries)
	if err != nil {
		return fmt.Errorf("state generator: verify entries: %w", err)
	}
	if err := verifyJournalArtifactProgramWithOrdered(g.ArtifactProgram, version, ordered); err != nil {
		return fmt.Errorf("state generator: bind ArtifactProgramRef: %w", err)
	}

	// Derive the tree from the verified, ordered entries.
	tree := basetree.Derive(journal.Events(ordered))

	// Write the tree to the target directory.
	if err := TreeToFS(ctx, tree, g.Blobs, targetDir); err != nil {
		return fmt.Errorf("state generator: %w", err)
	}

	return nil
}

// GenerateFromEvents writes durable state from a typed journal entry slice
// (no live journal required). This is useful for tests and for replaying
// from a captured event tape without a database connection. The entries are
// verified via the same tamper-evident chain check as a live journal.
func GenerateFromEvents(ctx context.Context, entries []journal.Entry, blobs *blob.Store, program ArtifactProgram, version ComputerVersion, targetDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !version.Valid() {
		return fmt.Errorf("state generator: invalid computer version")
	}
	if blobs == nil {
		return fmt.Errorf("state generator: nil blob store")
	}

	ordered, err := verifyBaseJournalEntries(entries)
	if err != nil {
		return fmt.Errorf("state generator: verify entries: %w", err)
	}
	if err := verifyJournalArtifactProgramWithOrdered(program, version, ordered); err != nil {
		return fmt.Errorf("state generator: bind ArtifactProgramRef: %w", err)
	}
	tree := basetree.Derive(journal.Events(ordered))

	if err := TreeToFS(ctx, tree, blobs, targetDir); err != nil {
		return fmt.Errorf("state generator: %w", err)
	}
	return nil
}

// StateGeneratorCapabilityManifest declares the observation scope for the
// state generator. The generator produces file_manifest and blob_set
// observations by writing files to a filesystem, so it supports those kinds.
func StateGeneratorCapabilityManifest(materializer, substrate string) CapabilityManifest {
	materializer = strings.TrimSpace(materializer)
	if materializer == "" {
		materializer = StateGeneratorMaterializer
	}
	substrate = strings.TrimSpace(substrate)
	if substrate == "" {
		substrate = StateGeneratorSubstrate
	}
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    substrate,
		Supported:    []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationDoltHead, Reason: "state generator does not produce Dolt ledger head"},
			{Kind: ObservationObjectGraphHead, Reason: "state generator does not produce object graph head"},
			{Kind: ObservationProvenanceAnswer, Reason: "state generator does not answer provenance queries"},
			{Kind: ObservationLiveProcessContinuity, Reason: "state generator does not launch a live process"},
			{Kind: ObservationVMStateManifest, Reason: "state generator does not classify VM launch metadata"},
			{Kind: ObservationPromotionCertificate, Reason: "state generator does not produce a promotion certificate"},
		},
	}
}
