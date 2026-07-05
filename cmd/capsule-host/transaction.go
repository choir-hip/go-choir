//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// TransactionBuilder converts classified capsule diffs into structured
// transaction records for the tape. It runs on the host (trusted zone)
// alongside the Classifier.
type TransactionBuilder struct {
	classifier *Classifier
}

// NewTransactionBuilder creates a new transaction builder with the given classifier.
func NewTransactionBuilder(classifier *Classifier) *TransactionBuilder {
	return &TransactionBuilder{classifier: classifier}
}

// TransactionRecord is the structured output of the transaction builder.
// It represents a single capsule's diff as a tape-appendable record.
type TransactionRecord struct {
	CapsuleID    string                    `json:"capsule_id"`
	Timestamp    time.Time                 `json:"timestamp"`
	ClassifierV  string                    `json:"classifier_version"`
	ClassifierDigest string                `json:"classifier_digest"`
	Groups       map[string][]ChangeRecord `json:"groups"`
	Ignored      []ChangeRecord            `json:"ignored"`
	Unknown      []ChangeRecord            `json:"unknown,omitempty"`
	Rejected     bool                      `json:"rejected"` // true if unknown paths present
	RejectReason string                    `json:"reject_reason,omitempty"`
}

// ChangeRecord is a single file change in the transaction record.
type ChangeRecord struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // "added", "modified", "deleted"
	Mode uint32 `json:"mode"`
}

// BuildTransactionFromDiff takes a capsule's file changes, classifies them,
// and builds a structured transaction record. If unknown paths are present,
// the record is marked as rejected (v7 decision: no catch-all ledger).
func (b *TransactionBuilder) BuildTransactionFromDiff(capsuleID string, changes []capsule.FileChange) (*TransactionRecord, error) {
	result := b.classifier.Classify(changes)

	record := &TransactionRecord{
		CapsuleID:        capsuleID,
		Timestamp:        time.Now().UTC(),
		ClassifierV:      result.Version,
		ClassifierDigest: result.Digest,
		Groups:           make(map[string][]ChangeRecord),
		Ignored:          toChangeRecords(result.Ignored),
		Unknown:          toChangeRecords(result.Unknown),
	}

	// Convert groups to ChangeRecord format.
	for kind, groupChanges := range result.Groups {
		record.Groups[kind.String()] = toChangeRecords(groupChanges)
	}

	// Reject if unknown paths are present.
	if result.HasUnknown() {
		record.Rejected = true
		record.RejectReason = fmt.Sprintf("unknown paths rejected at commit time: %d paths", len(result.Unknown))
	}

	return record, nil
}

// MarshalForTape serializes the transaction record for tape append.
func (r *TransactionRecord) MarshalForTape() ([]byte, error) {
	return json.Marshal(r)
}

// toChangeRecords converts a slice of capsule.FileChange to ChangeRecord.
func toChangeRecords(changes []capsule.FileChange) []ChangeRecord {
	if len(changes) == 0 {
		return nil
	}
	records := make([]ChangeRecord, len(changes))
	for i, change := range changes {
		records[i] = ChangeRecord{
			Path: change.Path,
			Kind: change.Kind.String(),
			Mode: uint32(change.Mode),
		}
	}
	return records
}
