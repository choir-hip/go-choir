package yaegikernel

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// B2 at the transport: a worker started under an old desk epoch is served the
// verbs of that epoch's row after an update narrowed the desk, and a worker
// started after the update is not.
func TestAdmitFrameResolvesTheRowOfTheWorkersDeskEpoch(t *testing.T) {
	const role = "app:framepin"
	mk := func(verbs ...string) deskmanifest.Desk {
		d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: role, Verbs: verbs, Taint: deskmanifest.TaintUntrustedInput, UID: 62901, WorkerMaxBytes: 64 << 20})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if err := deskmanifest.RegisterAppAt(mk("ReadCapture", "RequestLease"), 3); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp(role) })
	release, ok := deskmanifest.RetainEpoch(role, 3)
	if !ok {
		t.Fatal("hold")
	}
	defer release()
	if err := deskmanifest.UpdateApp(mk("ReadCapture"), 7); err != nil {
		t.Fatal(err)
	}
	worker := func(deskEpoch uint64) *DeskSessionWorker {
		return &DeskSessionWorker{slotToken: []byte("tok"), cfg: DeskSessionWorkerConfig{Session: SessionWorkerConfig{Role: role, Epoch: 1, DeskEpoch: deskEpoch}}}
	}
	req := BrokerRequest{ProtocolVersion: ProtocolVersion, SlotToken: "tok", Epoch: 1, Action: ActionRequestLease}
	if reason, forged := worker(3).admitFrame(context.Background(), StreamBrokerEgress, req); reason != "" || forged {
		t.Fatalf("the old-epoch worker was refused: %q", reason)
	}
	if reason, _ := worker(7).admitFrame(context.Background(), StreamBrokerEgress, req); reason == "" {
		t.Fatal("the new-epoch worker kept the dropped verb")
	}
	if reason, _ := worker(5).admitFrame(context.Background(), StreamBrokerEgress, req); reason == "" {
		t.Fatal("an epoch no row was held at served a frame")
	}
}
