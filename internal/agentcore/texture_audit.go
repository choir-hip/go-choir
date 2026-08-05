package agentcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const textureAuditMediaTypeV1 = "application/vnd.choir.texture-audit.v1+json"

// TextureAuditEntry is the compact, private audit record emitted after an
// owner-visible Texture mutation. Texture state remains owned by Texture's
// ordinary document/lifecycle store; this record is evidence, not a second
// writable projection.
type TextureAuditEntry struct {
	Schema           string `json:"schema"`
	Action           string `json:"action"`
	OwnerID          string `json:"owner_id"`
	ComputerID       string `json:"computer_id"`
	TrajectoryID     string `json:"trajectory_id,omitempty"`
	DocumentID       string `json:"document_id"`
	RevisionID       string `json:"revision_id,omitempty"`
	CommandID        string `json:"command_id"`
	CommandDigest    string `json:"command_digest"`
	LifecycleVersion int64  `json:"lifecycle_version,omitempty"`
}

// RecordTextureAudit appends one idempotent private audit event through the
// canonical ComputerEventAppender. Runtimes without computer credentials (for
// example focused local tests) have no audit capability and leave Texture's
// product path unchanged.
func (rt *Runtime) RecordTextureAudit(ctx context.Context, entry TextureAuditEntry) error {
	if rt == nil {
		return fmt.Errorf("texture audit: runtime unavailable")
	}
	if rt.eventAppender == nil && rt.privateArtifactCipher == nil {
		return nil
	}
	if rt.eventAppender == nil || rt.privateArtifactCipher == nil {
		return fmt.Errorf("texture audit: incomplete canonical event capability")
	}
	entry.Schema = "choir.texture_audit.v1"
	entry.Action = strings.TrimSpace(entry.Action)
	entry.OwnerID = strings.TrimSpace(entry.OwnerID)
	entry.ComputerID = strings.TrimSpace(entry.ComputerID)
	entry.TrajectoryID = strings.TrimSpace(entry.TrajectoryID)
	entry.DocumentID = strings.TrimSpace(entry.DocumentID)
	entry.RevisionID = strings.TrimSpace(entry.RevisionID)
	entry.CommandID = strings.TrimSpace(entry.CommandID)
	entry.CommandDigest = strings.TrimSpace(entry.CommandDigest)
	if entry.Action == "" || entry.OwnerID == "" || entry.ComputerID == "" || entry.DocumentID == "" || entry.CommandID == "" || entry.CommandDigest == "" {
		return fmt.Errorf("texture audit: action, owner, computer, document, command, and command digest are required")
	}
	payload, err := computerevent.CanonicalJSON(entry)
	if err != nil {
		return fmt.Errorf("texture audit: encode entry: %w", err)
	}
	payloadDigest := computerevent.DigestBytes(payload)
	idempotencyKey := textureAuditIdempotencyKey(entry)
	accepted, found, err := rt.store.EventByIdempotency(ctx, entry.ComputerID, idempotencyKey)
	if err == nil && found {
		if accepted.DecisionRef != payloadDigest {
			return fmt.Errorf("texture audit: command %q changed", entry.CommandID)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("texture audit: inspect command: %w", err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return fmt.Errorf("texture audit: create event id: %w", err)
	}
	event := computerevent.Event{
		SchemaVersion:  computerevent.SchemaVersionV1,
		EventID:        eventID,
		ComputerID:     entry.ComputerID,
		EventKind:      computerevent.EventLifecycleObserved,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey,
		TrajectoryID:   entry.TrajectoryID,
		ActorProfile:   "texture",
		AuthorityRef:   "texture:context",
		DecisionRef:    payloadDigest,
		PrivacyClass:   "private",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, _, err = rt.eventAppender.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, payload, textureAuditMediaTypeV1, rt.privateArtifactCipher); err == nil {
		return nil
	}
	accepted, found, lookupErr := rt.store.EventByIdempotency(ctx, entry.ComputerID, idempotencyKey)
	if lookupErr == nil && found && accepted.DecisionRef == payloadDigest {
		return nil
	}
	return fmt.Errorf("texture audit: append: %w", err)
}

func textureAuditIdempotencyKey(entry TextureAuditEntry) string {
	identityDigest := computerevent.DigestBytes([]byte(strings.Join([]string{
		entry.Action,
		entry.OwnerID,
		entry.ComputerID,
		entry.DocumentID,
		entry.RevisionID,
		entry.CommandID,
	}, "\x00")))
	return "texture-audit:" + identityDigest
}
