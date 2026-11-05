package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// MA3.7b: a runtime boot plus an idle tick reads no desk or service kind
// without a declared reader. The reads run on goroutines of their own so the
// oracle (which skips a stack with a test frame in it) sees the product's
// call sites, as it does on d@e at boot. Before the slice the settings and
// source-schedule reads of Start and the sweep were undeclared.
func TestBootAndIdleTickDeclareEveryReader(t *testing.T) {
	rt, st := testRuntime(t)
	owner, computer := "owner-boot-reads", rt.cfg.ComputerID
	ctx := context.Background()
	if err := st.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeEventWakes, Value: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSourceSchedule(withKindCustodian(ctx, "choir.source_schedule"), store.SourceSchedule{
		OwnerID: owner, ComputerID: computer, SourceID: "feed-boot-reads", IntervalSeconds: 600, State: store.SourceScheduleActive, NextDue: time.Now().Add(-time.Hour), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	objectgraph.ResetUndeclaredReads()
	objectgraph.ResetReaderGateCounts()
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.Start(ctx)
		_, _ = rt.sweepOverdueSourceSchedules(ctx, time.Now().UTC())
		_ = rt.intakeSurfacesEnabled(ctx, owner, computer)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("boot did not finish")
	}
	time.Sleep(500 * time.Millisecond) // the boot's background goroutines
	for key, n := range objectgraph.UndeclaredReads() {
		t.Errorf("undeclared read at boot: %s x%d", key, n)
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		t.Errorf("reader gate counted at boot: %s x%d", key, n)
	}
}
