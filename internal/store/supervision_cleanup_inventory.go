package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const SupervisionCleanupInventorySchemaV1 = "choir.supervision_cleanup_inventory.v1"

type SupervisionCleanupEvent struct {
	Sequence       uint64                  `json:"sequence"`
	EventDigest    string                  `json:"event_digest"`
	EventKind      computerevent.EventKind `json:"event_kind"`
	Status         string                  `json:"status"`
	IdempotencyKey string                  `json:"idempotency_key"`
	TrajectoryID   string                  `json:"trajectory_id,omitempty"`
	ArtifactRefs   []string                `json:"artifact_refs,omitempty"`
	MutationKinds  []string                `json:"mutation_kinds,omitempty"`
}

type SupervisionCleanupCommand struct {
	CommandID        string `json:"command_id"`
	CommandDigest    string `json:"command_digest"`
	Status           string `json:"status"`
	EventDigest      string `json:"event_digest,omitempty"`
	ArtifactDigest   string `json:"artifact_digest,omitempty"`
	HasDurableRecord bool   `json:"has_durable_record"`
}

type SupervisionCleanupRecordSet struct {
	Source     string `json:"source"`
	Kind       string `json:"kind"`
	Scope      string `json:"scope"`
	Count      uint64 `json:"count"`
	Tombstones uint64 `json:"tombstones,omitempty"`
	Digest     string `json:"digest"`
}

type SupervisionCleanupInventory struct {
	Schema                 string                        `json:"schema"`
	OwnerID                string                        `json:"owner_id"`
	ComputerID             string                        `json:"computer_id"`
	RealizationID          string                        `json:"realization_id,omitempty"`
	Head                   *computerevent.Head           `json:"head,omitempty"`
	Events                 []SupervisionCleanupEvent     `json:"events"`
	Commands               []SupervisionCleanupCommand   `json:"commands"`
	ProjectionImportEvents uint64                        `json:"projection_import_events"`
	RecordSets             []SupervisionCleanupRecordSet `json:"record_sets"`
}

// SupervisionCleanupInventory returns a read-only, content-free inventory for
// deciding whether the rejected supervision protocol can be removed without
// abandoning accepted events, pending commands, or owner data.
func (s *Store) SupervisionCleanupInventory(ctx context.Context, ownerID, computerID string) (SupervisionCleanupInventory, error) {
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if s == nil || s.db == nil || ownerID == "" || computerID == "" {
		return SupervisionCleanupInventory{}, fmt.Errorf("supervision cleanup inventory: owner_id and computer_id are required")
	}
	inventory := SupervisionCleanupInventory{
		Schema:     SupervisionCleanupInventorySchemaV1,
		OwnerID:    ownerID,
		ComputerID: computerID,
		Events:     []SupervisionCleanupEvent{},
		Commands:   []SupervisionCleanupCommand{},
		RecordSets: []SupervisionCleanupRecordSet{},
	}
	var err error
	if inventory.Head, err = s.Head(ctx, computerID); err != nil {
		return SupervisionCleanupInventory{}, err
	}
	if inventory.Events, inventory.ProjectionImportEvents, err = s.supervisionCleanupEvents(ctx, computerID); err != nil {
		return SupervisionCleanupInventory{}, err
	}
	if inventory.Commands, err = s.supervisionCleanupCommands(ctx, computerID); err != nil {
		return SupervisionCleanupInventory{}, err
	}
	objectSets, err := s.supervisionCleanupObjectSets(ctx, ownerID, computerID)
	if err != nil {
		return SupervisionCleanupInventory{}, err
	}
	inventory.RecordSets = append(inventory.RecordSets, objectSets...)
	for _, table := range []string{"texture_source_entities", "texture_source_refs"} {
		recordSets, queryErr := s.supervisionCleanupSourceSets(ctx, ownerID, computerID, table)
		if queryErr != nil {
			return SupervisionCleanupInventory{}, queryErr
		}
		inventory.RecordSets = append(inventory.RecordSets, recordSets...)
	}
	return inventory, nil
}

func (s *Store) supervisionCleanupEvents(ctx context.Context, computerID string) ([]SupervisionCleanupEvent, uint64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, event_digest, event_kind, status, idempotency_key, event_json, supervision_transaction_json FROM computer_event_index WHERE computer_id=? ORDER BY sequence`, computerID)
	if err != nil {
		return nil, 0, fmt.Errorf("supervision cleanup inventory: list canonical events: %w", err)
	}
	defer rows.Close()
	events := []SupervisionCleanupEvent{}
	var imports uint64
	for rows.Next() {
		var item SupervisionCleanupEvent
		var kind, eventJSON, transactionJSON string
		if err := rows.Scan(&item.Sequence, &item.EventDigest, &kind, &item.Status, &item.IdempotencyKey, &eventJSON, &transactionJSON); err != nil {
			return nil, 0, err
		}
		item.EventKind = computerevent.EventKind(kind)
		var event computerevent.Event
		if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
			return nil, 0, fmt.Errorf("supervision cleanup inventory: decode canonical event %s: %w", item.EventDigest, err)
		}
		item.TrajectoryID = event.TrajectoryID
		item.ArtifactRefs = append(item.ArtifactRefs, event.InputArtifactRefs...)
		item.ArtifactRefs = append(item.ArtifactRefs, event.OutputArtifactRefs...)
		if transactionJSON != "" {
			transaction, err := computerevent.DecodeSupervisionTransaction([]byte(transactionJSON))
			if err != nil {
				return nil, 0, fmt.Errorf("supervision cleanup inventory: decode transaction %s: %w", item.EventDigest, err)
			}
			item.MutationKinds = make([]string, 0, len(transaction.Mutations))
			for _, mutation := range transaction.Mutations {
				item.MutationKinds = append(item.MutationKinds, mutation.Kind)
				if mutation.Kind == "projection_imported" {
					imports++
				}
			}
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return events, imports, nil
}

func (s *Store) supervisionCleanupCommands(ctx context.Context, computerID string) ([]SupervisionCleanupCommand, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT command_id, command_digest, status, COALESCE(event_digest, ''), COALESCE(artifact_digest, ''), CASE WHEN COALESCE(event_head_receipt_json, '') = '' THEN FALSE ELSE TRUE END FROM computer_supervision_commands WHERE computer_id=? ORDER BY command_id`, computerID)
	if err != nil {
		return nil, fmt.Errorf("supervision cleanup inventory: list commands: %w", err)
	}
	defer rows.Close()
	commands := []SupervisionCleanupCommand{}
	for rows.Next() {
		var item SupervisionCleanupCommand
		if err := rows.Scan(&item.CommandID, &item.CommandDigest, &item.Status, &item.EventDigest, &item.ArtifactDigest, &item.HasDurableRecord); err != nil {
			return nil, err
		}
		commands = append(commands, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return commands, nil
}

func (s *Store) supervisionCleanupObjectSets(ctx context.Context, ownerID, computerID string) ([]SupervisionCleanupRecordSet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_kind, computer_id, canonical_id, version_id, content_hash, tombstone FROM og_objects WHERE owner_id=? AND (computer_id=? OR computer_id='') ORDER BY object_kind, computer_id, canonical_id`, ownerID, computerID)
	if err != nil {
		return nil, fmt.Errorf("supervision cleanup inventory: list object graph: %w", err)
	}
	defer rows.Close()
	sets := []SupervisionCleanupRecordSet{}
	var current *SupervisionCleanupRecordSet
	var digest hash.Hash
	for rows.Next() {
		var kind, scope, canonicalID, versionID, contentHash string
		var tombstone bool
		if err := rows.Scan(&kind, &scope, &canonicalID, &versionID, &contentHash, &tombstone); err != nil {
			return nil, err
		}
		if current == nil || current.Kind != kind || current.Scope != scope {
			if current != nil {
				current.Digest = hex.EncodeToString(digest.Sum(nil))
				sets = append(sets, *current)
			}
			current = &SupervisionCleanupRecordSet{Source: "og_objects", Kind: kind, Scope: scope}
			digest = sha256.New()
		}
		current.Count++
		if tombstone {
			current.Tombstones++
		}
		writeCleanupDigest(digest, canonicalID, versionID, contentHash, fmt.Sprintf("%t", tombstone))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if current != nil {
		current.Digest = hex.EncodeToString(digest.Sum(nil))
		sets = append(sets, *current)
	}
	return sets, nil
}

func (s *Store) supervisionCleanupSourceSets(ctx context.Context, ownerID, computerID, table string) ([]SupervisionCleanupRecordSet, error) {
	query := ""
	switch table {
	case "texture_source_entities":
		query = `SELECT computer_id, canonical_id, version_id, content_hash FROM texture_source_entities WHERE owner_id=? AND (computer_id=? OR computer_id='') ORDER BY computer_id, canonical_id, version_id`
	case "texture_source_refs":
		query = `SELECT computer_id, canonical_id, version_id, content_hash FROM texture_source_refs WHERE owner_id=? AND (computer_id=? OR computer_id='') ORDER BY computer_id, canonical_id, version_id`
	default:
		return nil, fmt.Errorf("supervision cleanup inventory: unsupported source table %q", table)
	}
	rows, err := s.db.QueryContext(ctx, query, ownerID, computerID)
	if err != nil {
		return nil, fmt.Errorf("supervision cleanup inventory: list %s: %w", table, err)
	}
	defer rows.Close()
	sets := []SupervisionCleanupRecordSet{}
	var current *SupervisionCleanupRecordSet
	var digest hash.Hash
	for rows.Next() {
		var scope, canonicalID, versionID, contentHash string
		if err := rows.Scan(&scope, &canonicalID, &versionID, &contentHash); err != nil {
			return nil, err
		}
		if current == nil || current.Scope != scope {
			if current != nil {
				current.Digest = hex.EncodeToString(digest.Sum(nil))
				sets = append(sets, *current)
			}
			current = &SupervisionCleanupRecordSet{Source: table, Kind: table, Scope: scope}
			digest = sha256.New()
		}
		current.Count++
		writeCleanupDigest(digest, canonicalID, versionID, contentHash)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if current != nil {
		current.Digest = hex.EncodeToString(digest.Sum(nil))
		sets = append(sets, *current)
	}
	return sets, nil
}

func writeCleanupDigest(digest hash.Hash, values ...string) {
	for _, value := range values {
		_, _ = digest.Write([]byte(value))
		_, _ = digest.Write([]byte{0})
	}
}
