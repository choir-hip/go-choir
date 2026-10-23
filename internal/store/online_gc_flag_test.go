package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// docs/problems/paper-remaining-store-stall-holders-2026-10-23.md section 1,
// fix shape 1: the online GC pass is named while it runs. Ways it could fail:
// the flag never set; set but not cleared after a pass, an error or a panic
// (the stall header would name a pass that ended); a data race between the
// pass and a reader; the flag blocking on the connection the pass holds.

func TestOnlineGCFlagIsSetDuringThePassAndClearedAfter(t *testing.T) {
	s := openTestStore(t)
	if st := s.OnlineGCStatus(); st.Active || st.Since != "" {
		t.Fatalf("idle store reports %+v", st)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	s.onlineGCExecHook = func(context.Context) error {
		close(entered)
		<-release
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- s.runOnlineDoltGC(context.Background()) }()
	<-entered

	// A reader never waits for the pass: the status and the pool stats answer
	// while it holds the write connection's place.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = s.OnlineGCStatus()
				_ = s.PoolStats()
			}
		}()
	}
	wg.Wait()
	st := s.OnlineGCStatus()
	if !st.Active {
		t.Fatalf("flag not set during the pass: %+v", st)
	}
	if _, err := time.Parse(time.RFC3339, st.Since); err != nil {
		t.Fatalf("since %q is not RFC 3339: %v", st.Since, err)
	}
	if got := s.PoolStats().OnlineGC; got != st {
		t.Fatalf("PoolStats().OnlineGC = %+v, want %+v", got, st)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if st := s.OnlineGCStatus(); st.Active || st.Since != "" {
		t.Fatalf("flag still set after the pass: %+v", st)
	}
	if got := s.PoolStats().OnlineGC; got.Active {
		t.Fatalf("PoolStats still reports a pass: %+v", got)
	}
}

func TestOnlineGCFlagIsClearedAfterAnError(t *testing.T) {
	s := openTestStore(t)
	boom := errors.New("pass failed")
	var during OnlineGCStatus
	s.onlineGCExecHook = func(context.Context) error {
		during = s.OnlineGCStatus()
		return boom
	}
	err := s.runOnlineDoltGC(context.Background())
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "dolt gc") {
		t.Fatalf("error = %v, want the pass's error wrapped", err)
	}
	if !during.Active {
		t.Fatalf("flag not set while the failing pass ran: %+v", during)
	}
	if st := s.OnlineGCStatus(); st.Active {
		t.Fatalf("flag still set after a failed pass: %+v", st)
	}
}

func TestOnlineGCFlagIsClearedAfterAPanic(t *testing.T) {
	s := openTestStore(t)
	s.onlineGCExecHook = func(context.Context) error { panic("engine panic") }
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("the panic did not propagate")
			}
		}()
		_ = s.runOnlineDoltGC(context.Background())
	}()
	if st := s.OnlineGCStatus(); st.Active {
		t.Fatalf("flag still set after a panicking pass: %+v", st)
	}
	// The commit mutex was released too: a second pass runs.
	s.onlineGCExecHook = func(context.Context) error { return nil }
	if err := s.runOnlineDoltGC(context.Background()); err != nil {
		t.Fatalf("pass after a panic: %v", err)
	}
}

// A real pass, not the hook: the flag is set while CALL DOLT_GC() runs on the
// write connection and clear when it returns.
func TestOnlineGCFlagAroundARealPass(t *testing.T) {
	s := openTestStore(t)
	writeProbeRows(t, s, 0, 300)
	var sawActive bool
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if s.OnlineGCStatus().Active {
				sawActive = true
			}
		}
	}()
	if err := s.runOnlineDoltGC(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(stop)
	wg.Wait()
	if !sawActive {
		t.Fatal("a poller never saw the flag during a real pass")
	}
	if s.OnlineGCStatus().Active {
		t.Fatal("flag set after a real pass")
	}
}
