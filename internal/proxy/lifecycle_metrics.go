package proxy

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

type lifecycleRecorder struct {
	mu     sync.Mutex
	stages map[string]*lifecycleStageAggregate
}

type lifecycleStageAggregate struct {
	Count           int64            `json:"count"`
	Errors          int64            `json:"errors,omitempty"`
	TotalDurationMs int64            `json:"total_duration_ms,omitempty"`
	MaxDurationMs   int64            `json:"max_duration_ms,omitempty"`
	ByStatus        map[string]int64 `json:"by_status,omitempty"`
}

type lifecycleHealthSummary struct {
	Stages []lifecycleStageSummary `json:"stages,omitempty"`
}

type lifecycleStageSummary struct {
	Stage         string           `json:"stage"`
	Count         int64            `json:"count"`
	Errors        int64            `json:"errors,omitempty"`
	AvgDurationMs int64            `json:"avg_duration_ms,omitempty"`
	MaxDurationMs int64            `json:"max_duration_ms,omitempty"`
	ByStatus      map[string]int64 `json:"by_status,omitempty"`
}

func newLifecycleRecorder() *lifecycleRecorder {
	return &lifecycleRecorder{stages: map[string]*lifecycleStageAggregate{}}
}

func (r *lifecycleRecorder) record(stage, status string, duration time.Duration) {
	if r == nil {
		return
	}
	if stage == "" {
		stage = "unknown"
	}
	if status == "" {
		status = "ok"
	}
	durationMs := duration.Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	agg := r.stages[stage]
	if agg == nil {
		agg = &lifecycleStageAggregate{ByStatus: map[string]int64{}}
		r.stages[stage] = agg
	}
	agg.Count++
	agg.TotalDurationMs += durationMs
	if durationMs > agg.MaxDurationMs {
		agg.MaxDurationMs = durationMs
	}
	agg.ByStatus[status]++
	if status != "ok" && status != "http_200" && status != "connected" {
		agg.Errors++
	}
}

func (r *lifecycleRecorder) summary() lifecycleHealthSummary {
	if r == nil {
		return lifecycleHealthSummary{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	names := make([]string, 0, len(r.stages))
	for name := range r.stages {
		names = append(names, name)
	}
	sort.Strings(names)

	out := lifecycleHealthSummary{Stages: make([]lifecycleStageSummary, 0, len(names))}
	for _, name := range names {
		agg := r.stages[name]
		if agg == nil || agg.Count == 0 {
			continue
		}
		statuses := make(map[string]int64, len(agg.ByStatus))
		for status, count := range agg.ByStatus {
			statuses[status] = count
		}
		out.Stages = append(out.Stages, lifecycleStageSummary{
			Stage:         name,
			Count:         agg.Count,
			Errors:        agg.Errors,
			AvgDurationMs: agg.TotalDurationMs / agg.Count,
			MaxDurationMs: agg.MaxDurationMs,
			ByStatus:      statuses,
		})
	}
	return out
}

type lifecycleStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *lifecycleStatusRecorder) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *lifecycleStatusRecorder) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(data)
}

func (w *lifecycleStatusRecorder) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *lifecycleStatusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func lifecycleHTTPStatus(status int) string {
	if status == 0 {
		status = 200
	}
	return "http_" + statusText(status)
}

func statusText(status int) string {
	if status < 100 || status > 999 {
		return "unknown"
	}
	return string([]byte{
		byte('0' + status/100),
		byte('0' + (status/10)%10),
		byte('0' + status%10),
	})
}
