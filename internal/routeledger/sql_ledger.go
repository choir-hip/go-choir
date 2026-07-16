package routeledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const routeLedgerSchema = `
CREATE TABLE IF NOT EXISTS computer_version_route_authorization_evidence (
  evidence_ref VARCHAR(96) NOT NULL PRIMARY KEY,
  evidence_kind VARCHAR(32) NOT NULL,
  route_slot_id VARCHAR(512) NOT NULL,
  code_ref VARCHAR(96) NOT NULL,
  artifact_program_ref VARCHAR(128) NOT NULL,
  evidence_json LONGTEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  CONSTRAINT route_evidence_code_ref_fk FOREIGN KEY (code_ref) REFERENCES computer_version_code_closures(code_ref),
  CONSTRAINT route_evidence_artifact_program_ref_fk FOREIGN KEY (artifact_program_ref) REFERENCES computer_version_artifact_programs(artifact_program_ref)
);
CREATE TABLE IF NOT EXISTS computer_version_route_slots (
  route_slot_id VARCHAR(512) NOT NULL PRIMARY KEY,
  current_code_ref VARCHAR(96) NOT NULL,
  current_artifact_program_ref VARCHAR(128) NOT NULL,
  generation BIGINT UNSIGNED NOT NULL,
  latest_receipt_id VARCHAR(64) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  CONSTRAINT route_slot_code_ref_fk FOREIGN KEY (current_code_ref) REFERENCES computer_version_code_closures(code_ref),
  CONSTRAINT route_slot_artifact_program_ref_fk FOREIGN KEY (current_artifact_program_ref) REFERENCES computer_version_artifact_programs(artifact_program_ref)
);
CREATE TABLE IF NOT EXISTS computer_version_route_transition_receipts (
  receipt_id VARCHAR(64) NOT NULL PRIMARY KEY,
  route_slot_id VARCHAR(512) NOT NULL,
  transition_kind VARCHAR(32) NOT NULL,
  old_code_ref TEXT NOT NULL,
  old_artifact_program_ref TEXT NOT NULL,
  new_code_ref VARCHAR(96) NOT NULL,
  new_artifact_program_ref VARCHAR(128) NOT NULL,
  expected_generation BIGINT UNSIGNED NOT NULL,
  committed_generation BIGINT UNSIGNED NOT NULL,
  approval_ref VARCHAR(96) NOT NULL,
  promotion_certificate_ref VARCHAR(96) NOT NULL,
  rollback_target_receipt_id TEXT NOT NULL,
  idempotency_key VARCHAR(512) NOT NULL,
  committed_at DATETIME(6) NOT NULL,
  UNIQUE KEY route_receipt_idempotency (idempotency_key),
  KEY route_receipt_slot_generation (route_slot_id, committed_generation),
  CONSTRAINT route_receipt_code_ref_fk FOREIGN KEY (new_code_ref) REFERENCES computer_version_code_closures(code_ref),
  CONSTRAINT route_receipt_artifact_program_ref_fk FOREIGN KEY (new_artifact_program_ref) REFERENCES computer_version_artifact_programs(artifact_program_ref),
  CONSTRAINT route_receipt_approval_ref_fk FOREIGN KEY (approval_ref) REFERENCES computer_version_route_authorization_evidence(evidence_ref),
  CONSTRAINT route_receipt_certificate_ref_fk FOREIGN KEY (promotion_certificate_ref) REFERENCES computer_version_route_authorization_evidence(evidence_ref)
);`

type SQLTransitionValidator func(context.Context, *sql.Tx, computerversion.ComputerVersion) error

type SQLLedger struct {
	db                 *sql.DB
	now                func() time.Time
	validateTransition SQLTransitionValidator
}

func NewSQLLedger(db *sql.DB, validateTransition SQLTransitionValidator) *SQLLedger {
	return &SQLLedger{db: db, now: time.Now, validateTransition: validateTransition}
}

func (l *SQLLedger) EnsureSchema(ctx context.Context) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("route ledger: SQL database is required")
	}
	if _, err := l.db.ExecContext(ctx, routeLedgerSchema); err != nil {
		return fmt.Errorf("route ledger: ensure schema: %w", err)
	}
	return nil
}

func (l *SQLLedger) Resolve(ctx context.Context, slotID string) (Slot, TransitionReceipt, error) {
	if l == nil || l.db == nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: SQL database is required")
	}
	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: route slot ID is required")
	}
	slot, err := querySlot(ctx, l.db, slotID, false)
	if err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	receipt, err := queryReceiptByID(ctx, l.db, slot.LatestReceiptID)
	if err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: latest receipt %q: %w", slot.LatestReceiptID, err)
	}
	if receipt.RouteSlotID != slot.ID || receipt.CommittedGeneration != slot.Generation || !SameVersion(receipt.New, slot.Current) {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: slot and latest receipt disagree")
	}
	return slot, receipt, nil
}

func (l *SQLLedger) Transition(ctx context.Context, command TransitionCommand) (Slot, TransitionReceipt, error) {
	if l == nil || l.db == nil || l.validateTransition == nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: SQL database and transition input validator are required")
	}
	command = command.normalized()
	if err := command.Validate(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	tx, err := l.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: begin transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := l.validateTransition(ctx, tx, command.New); err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: verify transition inputs: %w", err)
	}
	if err := verifyTransitionEvidenceSQL(ctx, tx, command, true); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}

	replayed, replayErr := queryReceiptByIdempotencyKey(ctx, tx, command.IdempotencyKey)
	if replayErr == nil {
		if !receiptMatchesCommand(replayed, command) {
			return Slot{}, TransitionReceipt{}, ErrIdempotencyReuse
		}
		current, err := querySlot(ctx, tx, replayed.RouteSlotID, true)
		if err != nil {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: resolve idempotent transition slot: %w", err)
		}
		return current, replayed, nil
	}
	if !errors.Is(replayErr, sql.ErrNoRows) {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: read idempotency key: %w", replayErr)
	}

	current, slotErr := querySlot(ctx, tx, command.RouteSlotID, true)
	exists := slotErr == nil
	if slotErr != nil && !errors.Is(slotErr, ErrSlotNotFound) {
		return Slot{}, TransitionReceipt{}, slotErr
	}
	if !exists {
		if command.Kind != TransitionBootstrap {
			return Slot{}, TransitionReceipt{}, ErrSlotNotFound
		}
	} else if command.Kind == TransitionBootstrap || current.Generation != command.ExpectedGeneration || !SameVersion(current.Current, command.Old) {
		return Slot{}, TransitionReceipt{}, ErrStaleTransition
	}
	if command.Kind == TransitionRollback {
		target, err := queryReceiptByID(ctx, tx, command.RollbackTargetReceiptID)
		if err != nil || target.RouteSlotID != command.RouteSlotID || !SameVersion(target.New, command.New) || target.CommittedGeneration >= current.Generation {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: rollback target receipt does not prove the requested prior ComputerVersion")
		}
	}

	generation := uint64(1)
	if exists {
		generation = current.Generation + 1
	}
	receipt := newReceipt(command, generation, l.now().UTC())
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_version_route_transition_receipts
		(receipt_id, route_slot_id, transition_kind, old_code_ref, old_artifact_program_ref,
		 new_code_ref, new_artifact_program_ref, expected_generation, committed_generation,
		 approval_ref, promotion_certificate_ref, rollback_target_receipt_id,
		 idempotency_key, committed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		receipt.ID, receipt.RouteSlotID, receipt.Kind, receipt.Old.CodeRef, receipt.Old.ArtifactProgramRef,
		receipt.New.CodeRef, receipt.New.ArtifactProgramRef, receipt.ExpectedGeneration, receipt.CommittedGeneration,
		receipt.ApprovalRef, receipt.PromotionCertificateRef, receipt.RollbackTargetReceiptID,
		receipt.IdempotencyKey, receipt.CommittedAt); err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: append receipt: %w", err)
	}
	if exists {
		result, err := tx.ExecContext(ctx, `UPDATE computer_version_route_slots
			SET current_code_ref = ?, current_artifact_program_ref = ?, generation = ?, latest_receipt_id = ?, updated_at = ?
			WHERE route_slot_id = ? AND generation = ? AND current_code_ref = ? AND current_artifact_program_ref = ?`,
			receipt.New.CodeRef, receipt.New.ArtifactProgramRef, generation, receipt.ID, receipt.CommittedAt,
			command.RouteSlotID, command.ExpectedGeneration, command.Old.CodeRef, command.Old.ArtifactProgramRef)
		if err != nil {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: update slot: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: inspect slot update: %w", err)
		}
		if rows != 1 {
			return Slot{}, TransitionReceipt{}, ErrStaleTransition
		}
	} else if _, err := tx.ExecContext(ctx, `INSERT INTO computer_version_route_slots
		(route_slot_id, current_code_ref, current_artifact_program_ref, generation, latest_receipt_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, command.RouteSlotID, receipt.New.CodeRef, receipt.New.ArtifactProgramRef, generation, receipt.ID, receipt.CommittedAt); err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: bootstrap slot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: commit transition: %w", err)
	}
	slot := Slot{ID: command.RouteSlotID, Current: command.New, Generation: generation, LatestReceiptID: receipt.ID}
	return slot, receipt, nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func querySlot(ctx context.Context, queryer queryRower, slotID string, forUpdate bool) (Slot, error) {
	query := `SELECT route_slot_id, current_code_ref, current_artifact_program_ref, generation, latest_receipt_id
		FROM computer_version_route_slots WHERE route_slot_id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var slot Slot
	if err := queryer.QueryRowContext(ctx, query, slotID).Scan(
		&slot.ID, &slot.Current.CodeRef, &slot.Current.ArtifactProgramRef, &slot.Generation, &slot.LatestReceiptID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Slot{}, ErrSlotNotFound
		}
		return Slot{}, fmt.Errorf("route ledger: resolve slot: %w", err)
	}
	if !slot.Current.Valid() || !validReceiptID(slot.LatestReceiptID) || slot.Generation == 0 {
		return Slot{}, fmt.Errorf("route ledger: invalid persisted slot %q", slot.ID)
	}
	return slot, nil
}

func queryReceiptByID(ctx context.Context, queryer queryRower, receiptID ReceiptID) (TransitionReceipt, error) {
	return queryReceipt(ctx, queryer, `SELECT receipt_id, route_slot_id, transition_kind,
		old_code_ref, old_artifact_program_ref, new_code_ref, new_artifact_program_ref,
		expected_generation, committed_generation, approval_ref, promotion_certificate_ref,
		rollback_target_receipt_id, idempotency_key, committed_at
		FROM computer_version_route_transition_receipts WHERE receipt_id = ?`, receiptID)
}

func queryReceiptByIdempotencyKey(ctx context.Context, queryer queryRower, key IdempotencyKey) (TransitionReceipt, error) {
	return queryReceipt(ctx, queryer, `SELECT receipt_id, route_slot_id, transition_kind,
		old_code_ref, old_artifact_program_ref, new_code_ref, new_artifact_program_ref,
		expected_generation, committed_generation, approval_ref, promotion_certificate_ref,
		rollback_target_receipt_id, idempotency_key, committed_at
		FROM computer_version_route_transition_receipts WHERE idempotency_key = ? FOR UPDATE`, key)
}

func queryReceipt(ctx context.Context, queryer queryRower, query string, arg any) (TransitionReceipt, error) {
	var receipt TransitionReceipt
	if err := queryer.QueryRowContext(ctx, query, arg).Scan(
		&receipt.ID, &receipt.RouteSlotID, &receipt.Kind,
		&receipt.Old.CodeRef, &receipt.Old.ArtifactProgramRef,
		&receipt.New.CodeRef, &receipt.New.ArtifactProgramRef,
		&receipt.ExpectedGeneration, &receipt.CommittedGeneration,
		&receipt.ApprovalRef, &receipt.PromotionCertificateRef, &receipt.RollbackTargetReceiptID,
		&receipt.IdempotencyKey, &receipt.CommittedAt,
	); err != nil {
		return TransitionReceipt{}, err
	}
	if err := receipt.Validate(); err != nil {
		return TransitionReceipt{}, err
	}
	return receipt, nil
}

func (l *SQLLedger) PinAuthorizationEvidence(ctx context.Context, evidence AuthorizationEvidence) (AuthorizationEvidence, error) {
	if l == nil || l.db == nil {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: SQL database is required")
	}
	if err := evidence.Validate(); err != nil {
		return AuthorizationEvidence{}, err
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: encode authorization evidence: %w", err)
	}
	if _, err := l.db.ExecContext(ctx, `INSERT INTO computer_version_route_authorization_evidence
		(evidence_ref, evidence_kind, route_slot_id, code_ref, artifact_program_ref, evidence_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE evidence_ref = evidence_ref`,
		evidence.Ref, evidence.Kind, evidence.RouteSlotID, evidence.ComputerVersion.CodeRef,
		evidence.ComputerVersion.ArtifactProgramRef, encoded, evidence.CreatedAt); err != nil {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: pin authorization evidence: %w", err)
	}
	resolved, err := queryAuthorizationEvidence(ctx, l.db, evidence.Ref, false)
	if err != nil {
		return AuthorizationEvidence{}, err
	}
	resolvedJSON, _ := json.Marshal(resolved)
	if string(resolvedJSON) != string(encoded) {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: authorization evidence ref collision")
	}
	return resolved, nil
}

func (l *SQLLedger) VerifyTransitionEvidence(ctx context.Context, command TransitionCommand) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("route ledger: SQL database is required")
	}
	return verifyTransitionEvidenceSQL(ctx, l.db, command.normalized(), false)
}

func verifyTransitionEvidenceSQL(ctx context.Context, queryer queryRower, command TransitionCommand, forUpdate bool) error {
	approval, err := queryAuthorizationEvidence(ctx, queryer, string(command.ApprovalRef), forUpdate)
	if err != nil {
		return fmt.Errorf("route ledger: resolve approval evidence: %w", err)
	}
	certificate, err := queryAuthorizationEvidence(ctx, queryer, string(command.PromotionCertificateRef), forUpdate)
	if err != nil {
		return fmt.Errorf("route ledger: resolve promotion certificate evidence: %w", err)
	}
	if approval.Kind != AuthorizationEvidenceApproval || certificate.Kind != AuthorizationEvidencePromotionCertificate ||
		approval.RouteSlotID != command.RouteSlotID || certificate.RouteSlotID != command.RouteSlotID ||
		!SameVersion(approval.ComputerVersion, command.New) || !SameVersion(certificate.ComputerVersion, command.New) {
		return fmt.Errorf("route ledger: authorization evidence does not bind the requested route and ComputerVersion")
	}
	return nil
}

func queryAuthorizationEvidence(ctx context.Context, queryer queryRower, ref string, forUpdate bool) (AuthorizationEvidence, error) {
	query := `SELECT evidence_json FROM computer_version_route_authorization_evidence WHERE evidence_ref = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var encoded []byte
	if err := queryer.QueryRowContext(ctx, query, ref).Scan(&encoded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthorizationEvidence{}, fmt.Errorf("authorization evidence not found")
		}
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: query authorization evidence: %w", err)
	}
	var evidence AuthorizationEvidence
	if err := json.Unmarshal(encoded, &evidence); err != nil {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: decode authorization evidence: %w", err)
	}
	if evidence.Ref != ref {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: authorization evidence ref mismatch")
	}
	if err := evidence.Validate(); err != nil {
		return AuthorizationEvidence{}, err
	}
	return evidence, nil
}
