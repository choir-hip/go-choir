package platform

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLifecycleControlPersistsIntentBeforeIdempotentCompletion(t *testing.T) {
	store, root := openTestPlatformStore(t)
	defer store.Close()
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	request := LifecycleControlRequest{
		Phase: "prepare", ComputerID: "computer-lifecycle", IdempotencyKey: "restart-1",
		Action: "restart", PriorState: "active", PriorEpoch: 7,
	}
	request.RequestCommitment, _ = lifecycleControlCommitment(request)
	prepared, err := service.PrepareLifecycleControl(context.Background(), request)
	if err != nil || prepared.Status != "pending" || prepared.PriorEpoch != 7 {
		t.Fatalf("prepare = %+v err=%v", prepared, err)
	}
	replay, err := service.PrepareLifecycleControl(context.Background(), request)
	if err != nil || replay.Status != "pending" || replay.PriorEpoch != prepared.PriorEpoch {
		t.Fatalf("prepare replay = %+v err=%v", replay, err)
	}
	request.Phase, request.ResultingState, request.ResultingEpoch = "complete", "active", 8
	receipt, err := service.RecordLifecycleControl(context.Background(), request)
	if err != nil || receipt.ReceiptKind != "LifecycleReceipt" {
		t.Fatalf("complete = %+v err=%v", receipt, err)
	}
	request.Phase, request.ResultingState, request.ResultingEpoch = "prepare", "", 0
	completed, err := service.PrepareLifecycleControl(context.Background(), request)
	if err != nil || completed.Status != "completed" || completed.Receipt == nil || completed.Receipt.ReceiptID != receipt.ReceiptID {
		t.Fatalf("completed replay = %+v err=%v", completed, err)
	}
}

func TestLifecycleControlPersistsRefreshIntentBeforeIdempotentCompletion(t *testing.T) {
	store, root := openTestPlatformStore(t)
	defer store.Close()
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	request := LifecycleControlRequest{
		Phase: "prepare", ComputerID: "computer-lifecycle", IdempotencyKey: "refresh-1",
		Action: "refresh", PriorState: "active", PriorEpoch: 268,
	}
	request.RequestCommitment, _ = lifecycleControlCommitment(request)
	prepared, err := service.PrepareLifecycleControl(context.Background(), request)
	if err != nil || prepared.Status != "pending" || prepared.PriorEpoch != 268 {
		t.Fatalf("prepare = %+v err=%v", prepared, err)
	}
	request.Phase, request.ResultingState, request.ResultingEpoch = "complete", "active", 268
	if _, err := service.RecordLifecycleControl(context.Background(), request); err == nil {
		t.Fatal("refresh completion without epoch advance should fail")
	}
	request.ResultingEpoch = 269
	receipt, err := service.RecordLifecycleControl(context.Background(), request)
	if err != nil || receipt.ReceiptKind != "LifecycleReceipt" {
		t.Fatalf("complete = %+v err=%v", receipt, err)
	}
}
