package computerversion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/journal"
)

const ArtifactProgramKindBaseJournal = "base_journal"

func NewJournalArtifactProgram(entries []journal.Entry, artifactLocator string, createdAt time.Time) (ArtifactProgram, error) {
	ordered, err := verifyBaseJournalEntries(entries)
	if err != nil {
		return ArtifactProgram{}, fmt.Errorf("artifact program: verify base journal: %w", err)
	}
	digest, err := journalEntriesSHA256(ordered)
	if err != nil {
		return ArtifactProgram{}, err
	}
	return NewArtifactProgram([]ArtifactProgramEntry{{
		Kind:          ArtifactProgramKindBaseJournal,
		ContentSHA256: digest,
		ArtifactURI:   contentAddressedURI("artifact+sha256", digest, artifactLocator),
	}}, createdAt)
}

func VerifyJournalArtifactProgram(program ArtifactProgram, version ComputerVersion, entries []journal.Entry) error {
	ordered, err := verifyBaseJournalEntries(entries)
	if err != nil {
		return fmt.Errorf("artifact program: verify base journal: %w", err)
	}
	return verifyJournalArtifactProgramWithOrdered(program, version, ordered)
}

func verifyJournalArtifactProgramWithOrdered(program ArtifactProgram, version ComputerVersion, ordered []journal.Entry) error {
	if err := program.Verify(); err != nil {
		return err
	}
	if program.Ref != version.ArtifactProgramRef {
		return fmt.Errorf("artifact program: resolved ref %q does not match ComputerVersion ref %q", program.Ref, version.ArtifactProgramRef)
	}
	var binding *ArtifactProgramEntry
	for i := range program.Entries {
		if program.Entries[i].Kind != ArtifactProgramKindBaseJournal {
			continue
		}
		if binding != nil {
			return fmt.Errorf("artifact program: multiple base journal bindings")
		}
		binding = &program.Entries[i]
	}
	if binding == nil {
		return fmt.Errorf("artifact program: base journal binding is required")
	}
	digest, err := journalEntriesSHA256(ordered)
	if err != nil {
		return err
	}
	if binding.ContentSHA256 != digest {
		return fmt.Errorf("artifact program: base journal hash mismatch")
	}
	return nil
}

func journalEntriesSHA256(entries []journal.Entry) (string, error) {
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("artifact program: marshal base journal: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
