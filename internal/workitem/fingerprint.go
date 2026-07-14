// Package workitem owns deterministic identities for durable work items.
package workitem

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// ObjectiveFingerprint returns the stable identity of a spawned objective.
func ObjectiveFingerprint(ownerID, trajectoryID, parentRunID, objective string) string {
	parts := []string{
		strings.TrimSpace(ownerID),
		strings.TrimSpace(trajectoryID),
		strings.TrimSpace(parentRunID),
		normalizeObjective(objective),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// PublicationFingerprint returns the identity of a publication obligation.
func PublicationFingerprint(trajectoryID, revisionID string) string {
	trajectoryID = strings.TrimSpace(trajectoryID)
	revisionID = strings.TrimSpace(revisionID)
	if trajectoryID == "" || revisionID == "" {
		return ""
	}
	return "wire_publication:" + trajectoryID + ":" + revisionID
}

// StoryResolutionFingerprint returns the identity of a story-resolution obligation.
func StoryResolutionFingerprint(trajectoryID, docID string) string {
	trajectoryID = strings.TrimSpace(trajectoryID)
	docID = strings.TrimSpace(docID)
	if trajectoryID == "" || docID == "" {
		return ""
	}
	return "wire_story_resolution:" + trajectoryID + ":" + docID
}

// ProcessorDecisionFingerprint returns the identity of a processor decision obligation.
func ProcessorDecisionFingerprint(trajectoryID string) string {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return ""
	}
	return "wire_processor_request_resolution:" + trajectoryID
}

// SourceItemDecisionFingerprint returns the identity of a source-item decision obligation.
func SourceItemDecisionFingerprint(trajectoryID, sourceItemID string) string {
	trajectoryID = strings.TrimSpace(trajectoryID)
	sourceItemID = strings.TrimSpace(sourceItemID)
	if trajectoryID == "" || sourceItemID == "" {
		return ""
	}
	return "wire_source_item_resolution:" + trajectoryID + ":" + sourceItemID
}

func normalizeObjective(raw string) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			pendingSpace = false
			continue
		}
		pendingSpace = b.Len() > 0
	}
	return b.String()
}
