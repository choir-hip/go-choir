package agentcore

import (
	"context"
	"runtime"
	"time"

	"github.com/yusefmosiah/go-choir/internal/toolresidency"
)

// toolLayerReadBudget bounds the whole tool-layer measurement on a request.
const toolLayerReadBudget = 2 * time.Second

// toolLayerObservability is one guest tool layer (an erofs mount at
// /nix/.ro-tool-<layer>) as the L3.5 proof reads it without SSH: how much of it
// sits in the page cache, whether a process uses it, and when the idle timer
// last dropped it (docs/design/guest-tools-live-load-2026-11-09.md).
type toolLayerObservability struct {
	Layer         string `json:"layer"`
	Mounted       bool   `json:"mounted"`
	ResidentBytes int64  `json:"resident_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
	Busy          bool   `json:"busy"`
	BusyPids      []int  `json:"busy_pids,omitempty"`
	// LastEvictedAt and LastRunAt come from choir-tool's status file
	// (/run/choir-tool-evict/<layer>.json); empty before the first timer run.
	LastEvictedAt string `json:"last_evicted_at,omitempty"`
	LastRunAt     string `json:"last_run_at,omitempty"`
	// Error is a measurement failure for this layer (for example the budget
	// ran out); the other fields are then partial.
	Error string `json:"error,omitempty"`
}

// toolLayerTimerObservability is the idle-eviction schedule, so a proof knows
// how long to wait: a layer idle for the grace is dropped at the next tick.
type toolLayerTimerObservability struct {
	IntervalSeconds  int64 `json:"interval_seconds"`
	IdleGraceSeconds int64 `json:"idle_grace_seconds"`
}

func toolLayerTimer() *toolLayerTimerObservability {
	return &toolLayerTimerObservability{
		IntervalSeconds:  int64(toolresidency.TimerInterval / time.Second),
		IdleGraceSeconds: int64(toolresidency.IdleGrace / time.Second),
	}
}

// readToolLayers measures every mounted layer under mountRoot. Never nil, never
// an error: a failure lands in the layer's Error. Empty off Linux and on the
// single image (no layer mounts).
func readToolLayers(ctx context.Context, mountRoot, procRoot, stateDir string) []toolLayerObservability {
	out := []toolLayerObservability{}
	if runtime.GOOS != "linux" {
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, toolLayerReadBudget)
	defer cancel()
	for _, layer := range toolresidency.Layers(mountRoot) {
		l := toolLayerObservability{Layer: layer, Mounted: true}
		dir := toolresidency.LayerDir(mountRoot, layer)
		var err error
		l.ResidentBytes, l.TotalBytes, err = toolresidency.Resident(ctx, dir)
		if err != nil {
			l.Error = "resident walk incomplete: " + err.Error()
		}
		l.BusyPids = toolresidency.Busy(ctx, procRoot, dir)
		l.Busy = len(l.BusyPids) > 0
		if l.Error == "" && ctx.Err() != nil {
			l.Error = "busy scan incomplete: " + ctx.Err().Error()
		}
		st := toolresidency.ReadStatus(stateDir, layer)
		if !st.LastEvictedAt.IsZero() {
			l.LastEvictedAt = st.LastEvictedAt.UTC().Format(time.RFC3339)
		}
		if !st.LastRunAt.IsZero() {
			l.LastRunAt = st.LastRunAt.UTC().Format(time.RFC3339)
		}
		out = append(out, l)
	}
	return out
}
