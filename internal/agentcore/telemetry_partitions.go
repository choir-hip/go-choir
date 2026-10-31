package agentcore

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// store.telemetry_partitions at the runtime seam
// (docs/design/one-tape-durability-2026-10-31.md section 6c). The owner's
// stored setting, read at boot and applied at once on a change. The store
// writes the floor before the writer stamps, and stamps only once the
// stream_seq lease is ready on the tape; off is the live rollback.

// configureTelemetryPartitions applies the setting at boot, before the first
// event of the process.
func (rt *Runtime) configureTelemetryPartitions(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	rt.applyTelemetryPartitionsSetting(rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingTelemetryPartitions))
}

// applyTelemetryPartitionsSetting is the owner's change at runtime.
func (rt *Runtime) applyTelemetryPartitionsSetting(value bool) {
	if rt == nil || rt.store == nil {
		return
	}
	if !value && !rt.store.TelemetryPartitionWritesOn() {
		_ = rt.store.SetTelemetryPartitionWrites(false)
		return
	}
	if err := rt.store.SetTelemetryPartitionWrites(value); err != nil {
		log.Printf("runtime: telemetry partitions: %v", err)
		return
	}
	log.Printf("runtime: telemetry partitions: writer %s", map[bool]string{true: "armed (stamps once the stream_seq lease is ready)", false: "off"}[value])
}
