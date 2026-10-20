package yaegikernel

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Async search across a real worker process. Ways it can fail:
//  1. A running cell cannot see a provider that lands after the cell
//     started, because arrivals reach the worker only at a cell boundary.
//  2. An arrival frame between cells is lost or kills the session.
//  3. The count line is missing from a cell result, or repeats.
//  4. Arrival payloads reach the cell result although the cell never read
//     them.

// searchHost is a test host for search_start: it numbers searches per
// worker, pushes the start frame before answering (so the store has the
// search when SearchStart returns), and leaves later frames to the test.
type searchHost struct {
	mu     sync.Mutex
	n      int
	worker *DeskSessionWorker
	starts chan string
}

func (h *searchHost) egress(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
	if action != ActionSearchStart {
		return nil, fmt.Errorf("unexpected action %s", action)
	}
	var p SearchStartPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	worker := EgressWorkerFromContext(ctx)
	if worker == nil {
		return nil, fmt.Errorf("egress context names no worker")
	}
	h.mu.Lock()
	h.n++
	id := fmt.Sprintf("s%d", h.n)
	h.worker = worker
	h.mu.Unlock()
	start := SearchFrame{ID: id, Kind: SearchFrameStart, Query: p.Query, Providers: []string{"fast", "slow"}, Cooling: []string{"cold"}}
	if err := worker.PushSearch(start); err != nil {
		return nil, err
	}
	if err := worker.PushSearch(providerFrame(id, "fast", true, "https://fast.example/"+p.Query)); err != nil {
		return nil, err
	}
	h.starts <- id
	return json.Marshal(SearchStartResult{ID: id, Query: p.Query, Providers: start.Providers, Cooling: start.Cooling})
}

func searchWorker(t *testing.T) (*DeskSessionWorker, *searchHost) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("desk session worker spawn requires unix socketpair")
	}
	host := &searchHost{starts: make(chan string, 8)}
	cfg := deskWorkerConfig(t)
	cfg.Session.Role = SessionRoleResearch
	cfg.Egress = host.egress
	w, err := SpawnDeskSessionWorker(cfg)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(w.Close)
	return w, host
}

func TestRunningCellSeesALaterArrivalWithoutANewCell(t *testing.T) {
	w, host := searchWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		id := <-host.starts
		time.Sleep(300 * time.Millisecond)
		_ = w.PushSearch(providerFrame(id, "slow", true, "https://slow.example/late"))
		_ = w.PushSearch(SearchFrame{ID: id, Kind: SearchFrameDone})
	}()
	res, err := w.Eval(ctx, `import "time"
s, err := choir.SearchStart("late news")
if err != nil { panic(err) }
fmt.Println(s)
seen := 0
for i := 0; i < 300; i++ {
	r, _ := choir.Searches.Get(s.ID)
	seen = len(r.Arrivals)
	if seen == 2 && r.Done { break }
	time.Sleep(10 * time.Millisecond)
}
r, _ := choir.Searches.Get(s.ID)
fmt.Println("seen", seen, "done", r.Done, "last", r.Arrivals[len(r.Arrivals)-1].Results[0].URL)
`)
	if err != nil || res.Error != "" {
		t.Fatalf("cell: err=%v res=%#v", err, res)
	}
	if !strings.Contains(res.Stdout, "s1 started: 2 providers (1 cooling: cold)") {
		t.Fatalf("SearchStart stub = %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "seen 2 done true last https://slow.example/late") {
		t.Fatalf("the running cell did not see the later arrival: %q", res.Stdout)
	}
}

func TestArrivalsBetweenCellsLandAndCountOnce(t *testing.T) {
	w, host := searchWorker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first, err := w.Eval(ctx, `s, _ := choir.SearchStart("quiet")
_ = s`)
	if err != nil || first.Error != "" {
		t.Fatalf("first cell: err=%v res=%#v", err, first)
	}
	id := <-host.starts
	if got := strings.Join(first.Searches, "|"); got != id+": 1/2 answered, +1 new" {
		t.Fatalf("first cell count lines = %q", got)
	}
	if strings.Contains(first.Stdout, "fast.example") {
		t.Fatalf("an arrival payload reached a cell that never read it: %q", first.Stdout)
	}
	// Between cells: the desk's turn has ended, nothing is running.
	if err := w.PushSearch(providerFrame(id, "slow", false)); err != nil {
		t.Fatal(err)
	}
	if err := w.PushSearch(SearchFrame{ID: id, Kind: SearchFrameDone}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	second, err := w.Eval(ctx, `c, err := choir.SearchWait(s.ID, 2000)
fmt.Println(c, err)`)
	if err != nil || second.Error != "" {
		t.Fatalf("second cell: err=%v res=%#v", err, second)
	}
	if !strings.Contains(second.Stdout, id+": 1/2 answered, done") {
		t.Fatalf("SearchWait counts = %q", second.Stdout)
	}
	if got := strings.Join(second.Searches, "|"); got != id+": 1/2 answered, +1 new, done" {
		t.Fatalf("second cell count lines = %q", got)
	}
	third, err := w.Eval(ctx, `fmt.Println(len(choir.Searches.List()))`)
	if err != nil || third.Error != "" || strings.TrimSpace(third.Stdout) != "1" || len(third.Searches) != 0 {
		t.Fatalf("third cell: err=%v res=%#v, want the search still listed and no count line", err, third)
	}
}
