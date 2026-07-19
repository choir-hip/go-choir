package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type LifecycleControlRequest struct {
	Phase             string `json:"phase"`
	ComputerID        string `json:"computer_id"`
	IdempotencyKey    string `json:"idempotency_key"`
	RequestCommitment string `json:"request_commitment"`
	Action            string `json:"action"`
	PriorState        string `json:"prior_lifecycle_state"`
	ResultingState    string `json:"resulting_lifecycle_state"`
	PriorEpoch        int64  `json:"prior_realization_epoch"`
	ResultingEpoch    int64  `json:"resulting_realization_epoch"`
}

type LifecycleControlResult struct {
	Status     string                 `json:"status"`
	Action     string                 `json:"action"`
	PriorState string                 `json:"prior_lifecycle_state"`
	PriorEpoch int64                  `json:"prior_realization_epoch"`
	Receipt    *computerevent.Receipt `json:"receipt,omitempty"`
}

func lifecycleControlCommitment(request LifecycleControlRequest) (string, error) {
	canonical, err := computerevent.CanonicalJSON(map[string]string{
		"computer_id": request.ComputerID, "action": request.Action, "idempotency_key": request.IdempotencyKey,
	})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func (s *Service) PrepareLifecycleControl(ctx context.Context, request LifecycleControlRequest) (LifecycleControlResult, error) {
	request.ComputerID, request.IdempotencyKey = strings.TrimSpace(request.ComputerID), strings.TrimSpace(request.IdempotencyKey)
	request.RequestCommitment, request.Action, request.PriorState = strings.TrimSpace(request.RequestCommitment), strings.TrimSpace(request.Action), strings.TrimSpace(request.PriorState)
	expectedCommitment, err := lifecycleControlCommitment(request)
	if s == nil || s.store == nil || request.Phase != "prepare" || request.ComputerID == "" || request.IdempotencyKey == "" ||
		expectedCommitment != request.RequestCommitment || (request.Action != "start" && request.Action != "stop" && request.Action != "restart") || request.PriorState == "" {
		return LifecycleControlResult{}, fmt.Errorf("lifecycle control: complete durable intent is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if receipt, _, found, readErr := s.credentialLifecycleReceipt(ctx, request.ComputerID, request.IdempotencyKey, request.RequestCommitment, request.Action); readErr != nil {
		return LifecycleControlResult{}, readErr
	} else if found {
		return LifecycleControlResult{Status: "completed", Action: request.Action, Receipt: &receipt}, nil
	}
	var existing LifecycleControlResult
	var existingCommitment string
	err = s.store.db.QueryRowContext(ctx, `SELECT status, action, prior_lifecycle_state, prior_realization_epoch, request_commitment FROM computer_lifecycle_operations WHERE computer_id=? AND idempotency_key=?`, request.ComputerID, request.IdempotencyKey).
		Scan(&existing.Status, &existing.Action, &existing.PriorState, &existing.PriorEpoch, &existingCommitment)
	if err == nil {
		if existingCommitment != request.RequestCommitment || existing.Action != request.Action {
			return LifecycleControlResult{}, fmt.Errorf("lifecycle control: idempotency commitment changed")
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return LifecycleControlResult{}, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = s.store.db.ExecContext(ctx, `INSERT INTO computer_lifecycle_operations (computer_id,idempotency_key,request_commitment,action,prior_lifecycle_state,prior_realization_epoch,status,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		request.ComputerID, request.IdempotencyKey, request.RequestCommitment, request.Action, request.PriorState, request.PriorEpoch, "pending", now); err != nil {
		return LifecycleControlResult{}, err
	}
	if err = s.store.commitDolt(ctx, "prepare lifecycle "+request.Action+" for "+request.ComputerID); err != nil {
		return LifecycleControlResult{}, err
	}
	return LifecycleControlResult{Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch}, nil
}

func (s *Service) RecordLifecycleControl(ctx context.Context, request LifecycleControlRequest) (computerevent.Receipt, error) {
	request.ComputerID = strings.TrimSpace(request.ComputerID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.RequestCommitment = strings.TrimSpace(request.RequestCommitment)
	request.Action = strings.TrimSpace(request.Action)
	request.PriorState = strings.TrimSpace(request.PriorState)
	request.ResultingState = strings.TrimSpace(request.ResultingState)
	expectedCommitment, commitmentErr := lifecycleControlCommitment(request)
	if s == nil || s.store == nil || s.signingKey == nil || request.Phase != "complete" || request.ComputerID == "" || request.IdempotencyKey == "" ||
		commitmentErr != nil || expectedCommitment != request.RequestCommitment || (request.Action != "start" && request.Action != "stop" && request.Action != "restart") ||
		request.PriorState == "" || request.ResultingState == "" || (request.Action == "restart" && request.ResultingEpoch <= request.PriorEpoch) {
		return computerevent.Receipt{}, fmt.Errorf("lifecycle control: complete signed actuator result is required")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if receipt, _, found, err := s.credentialLifecycleReceipt(ctx, request.ComputerID, request.IdempotencyKey, request.RequestCommitment, request.Action); err != nil || found {
		return receipt, err
	}
	var pendingAction, pendingState, pendingCommitment string
	var pendingEpoch int64
	if err := s.store.db.QueryRowContext(ctx, `SELECT action,prior_lifecycle_state,prior_realization_epoch,request_commitment FROM computer_lifecycle_operations WHERE computer_id=? AND idempotency_key=? AND status='pending'`,
		request.ComputerID, request.IdempotencyKey).Scan(&pendingAction, &pendingState, &pendingEpoch, &pendingCommitment); err != nil ||
		pendingAction != request.Action || pendingState != request.PriorState || pendingEpoch != request.PriorEpoch || pendingCommitment != request.RequestCommitment {
		return computerevent.Receipt{}, fmt.Errorf("lifecycle control: durable intent is unavailable or changed")
	}
	var generation uint64
	if err := s.store.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) FROM computer_lifecycle_receipts WHERE computer_id=?`, request.ComputerID).Scan(&generation); err != nil {
		return computerevent.Receipt{}, err
	}
	generation++
	now := time.Now().UTC().Truncate(time.Microsecond)
	receipt, err := computerevent.NewSignedReceipt("LifecycleReceipt", "corpusd", map[string]any{
		"computer_id": request.ComputerID, "action": request.Action,
		"prior_lifecycle_state": request.PriorState, "resulting_lifecycle_state": request.ResultingState,
		"prior_realization_epoch": request.PriorEpoch, "resulting_realization_epoch": request.ResultingEpoch,
		"generation": generation, "idempotency_key": request.IdempotencyKey,
		"request_commitment": request.RequestCommitment, "completed_at": now.Format(time.RFC3339Nano),
	}, []computerevent.SigningKey{s.computerEventSigningKey()}, now)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	if err := s.insertCredentialLifecycleReceipt(ctx, request.ComputerID, request.IdempotencyKey, request.RequestCommitment, request.Action, request.PriorState, request.ResultingState, generation, now, receipt); err != nil {
		return computerevent.Receipt{}, err
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE computer_lifecycle_operations SET status='completed', completed_at=? WHERE computer_id=? AND idempotency_key=? AND status='pending'`, now, request.ComputerID, request.IdempotencyKey); err != nil {
		return computerevent.Receipt{}, err
	}
	if err := s.store.commitDolt(ctx, "complete lifecycle "+request.Action+" for "+request.ComputerID); err != nil {
		return computerevent.Receipt{}, err
	}
	return receipt, nil
}

func (s *Service) PendingLifecycleControls(ctx context.Context, computerID string) ([]computerevent.Receipt, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT receipt_json FROM computer_lifecycle_receipts WHERE computer_id=? AND action IN ('start','stop','restart') AND joined_event_digest IS NULL ORDER BY generation`, strings.TrimSpace(computerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var receipts []computerevent.Receipt
	for rows.Next() {
		var raw string
		var receipt computerevent.Receipt
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &receipt) != nil {
			return nil, fmt.Errorf("lifecycle control: invalid pending receipt")
		}
		receipts = append(receipts, receipt)
	}
	return receipts, rows.Err()
}
