package yaegikernel

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestActorRegistrationAndState(t *testing.T) {
	mgr := NewActorStateManager()
	actor, err := mgr.RegisterActor("actor-cosuper-1", "cosuper", "gpt-5.6-sol")
	if err != nil {
		t.Fatalf("RegisterActor failed: %v", err)
	}
	if actor.ActorID != "actor-cosuper-1" || actor.CurrentEpoch != 1 || actor.ModelID != "gpt-5.6-sol" {
		t.Fatalf("unexpected actor state: %+v", actor)
	}

	// Record an assignment
	err = mgr.RecordAssignment(&DurableAssignment{
		AssignmentID: "asgn-001",
		ActorID:      "actor-cosuper-1",
		Instruction:  "Implement feature X",
		Status:       "in_progress",
	})
	if err != nil {
		t.Fatalf("RecordAssignment failed: %v", err)
	}

	// Record a message
	err = mgr.RecordMessage(&DurableMessage{
		MessageID:   "msg-001",
		SenderID:    "actor-super",
		RecipientID: "actor-cosuper-1",
		Kind:        "directive",
		Body:        "Proceed with build",
	})
	if err != nil {
		t.Fatalf("RecordMessage failed: %v", err)
	}

	// Record an obligation
	err = mgr.RecordObligation(&DurableObligation{
		ObligationID: "ob-001",
		ActorID:      "actor-cosuper-1",
		Description:  "Verify tests pass",
		Satisfied:    false,
	})
	if err != nil {
		t.Fatalf("RecordObligation failed: %v", err)
	}
}

func TestRewarmActorMonotonicEpochAndContinuity(t *testing.T) {
	mgr := NewActorStateManager()
	_, err := mgr.RegisterActor("actor-cosuper-2", "cosuper", "gpt-5.6-sol")
	if err != nil {
		t.Fatal(err)
	}

	// Add 1 open assignment, 1 completed assignment
	_ = mgr.RecordAssignment(&DurableAssignment{
		AssignmentID: "asgn-open",
		ActorID:      "actor-cosuper-2",
		Instruction:  "Open task",
		Status:       "in_progress",
	})
	_ = mgr.RecordAssignment(&DurableAssignment{
		AssignmentID: "asgn-done",
		ActorID:      "actor-cosuper-2",
		Instruction:  "Done task",
		Status:       "completed",
	})

	// Add 1 unacknowledged message
	_ = mgr.RecordMessage(&DurableMessage{
		MessageID:   "msg-pending",
		SenderID:    "actor-super",
		RecipientID: "actor-cosuper-2",
		Kind:        "update",
		Body:        "Pending update",
	})

	// Add 1 unsatisfied obligation
	_ = mgr.RecordObligation(&DurableObligation{
		ObligationID: "ob-open",
		ActorID:      "actor-cosuper-2",
		Description:  "Must freeze bundle",
		Satisfied:    false,
	})

	// Rewarm actor under model switch (e.g. gpt-5.6-sol -> gemini-3.7-flash)
	actor, openAsgns, pendingMsgs, openObs, err := mgr.RewarmActor(context.Background(), "actor-cosuper-2", "gemini-3.7-flash")
	if err != nil {
		t.Fatalf("RewarmActor failed: %v", err)
	}

	if actor.CurrentEpoch != 2 {
		t.Fatalf("expected epoch 2 after rewarm, got %d", actor.CurrentEpoch)
	}
	if actor.ModelID != "gemini-3.7-flash" {
		t.Fatalf("expected model switched to gemini-3.7-flash, got %s", actor.ModelID)
	}
	if len(openAsgns) != 1 || openAsgns[0].AssignmentID != "asgn-open" {
		t.Fatalf("expected 1 open assignment, got %+v", openAsgns)
	}
	if len(pendingMsgs) != 1 || pendingMsgs[0].MessageID != "msg-pending" {
		t.Fatalf("expected 1 pending message, got %+v", pendingMsgs)
	}
	if len(openObs) != 1 || openObs[0].ObligationID != "ob-open" {
		t.Fatalf("expected 1 unsatisfied obligation, got %+v", openObs)
	}
}

func TestEpochFencingEndToEnd(t *testing.T) {
	secret := make([]byte, 32)
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}

	mgr := NewActorStateManager()
	actor, err := mgr.RegisterActor("actor-fenced", "cosuper", "model-v1")
	if err != nil {
		t.Fatal(err)
	}

	broker, err := NewBroker(BrokerConfig{
		ComputerID:   "computer-1",
		CurrentEpoch: actor.CurrentEpoch,
		AllowedRoot:  t.TempDir(),
	}, issuer)
	if err != nil {
		t.Fatal(err)
	}

	// Issue handle for Epoch 1
	h1, err := issuer.Issue("computer-1", "cosuper", actor.CurrentEpoch, []BrokerAction{ActionExec}, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	reqPayload, _ := json.Marshal(ExecPayload{Command: "echo", Args: []string{"epoch1"}})
	req1 := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-e1",
		HandleRef:       h1,
		Epoch:           1,
		Action:          ActionExec,
		Payload:         reqPayload,
	}

	// Execution on epoch 1 succeeds
	resp1 := broker.HandleRequest(context.Background(), req1)
	if !resp1.Success {
		t.Fatalf("epoch 1 request failed: %s", resp1.Error)
	}

	// Simulate activation death & rewarm
	actor, _, _, _, err = mgr.RewarmActor(context.Background(), "actor-fenced", "model-v2")
	if err != nil {
		t.Fatal(err)
	}
	broker.SetEpoch(actor.CurrentEpoch) // Advance broker to epoch 2

	// Old handle request is rejected by broker
	respOld := broker.HandleRequest(context.Background(), req1)
	if respOld.Success {
		t.Fatal("expected request with stale handle epoch to fail")
	}
	if !strings.Contains(respOld.Error, "stale activation epoch") {
		t.Fatalf("unexpected error message: %s", respOld.Error)
	}

	// Issue fresh handle for Epoch 2
	h2, err := issuer.Issue("computer-1", "cosuper", actor.CurrentEpoch, []BrokerAction{ActionExec}, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req2 := &BrokerRequest{
		ProtocolVersion: ProtocolVersion,
		RequestID:       "req-e2",
		HandleRef:       h2,
		Epoch:           actor.CurrentEpoch,
		Action:          ActionExec,
		Payload:         reqPayload,
	}

	// Execution on epoch 2 succeeds
	resp2 := broker.HandleRequest(context.Background(), req2)
	if !resp2.Success {
		t.Fatalf("epoch 2 request failed: %s", resp2.Error)
	}
}
