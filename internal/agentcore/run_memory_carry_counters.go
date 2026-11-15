package agentcore

import (
	"encoding/json"
	"sync/atomic"
)

// Run memory carry counters (store-once design R1, first step): how much of
// the run memory a run writes is the earlier activation's conversation carried
// forward (carryActorConversation) and how much is new, so the carried share
// is read from the observability endpoint. Counted at the call sites, after
// the append succeeded; no entry or tape op carries a marker. Always on,
// atomics only, no lock.
type runMemoryCarryCounters struct {
	carries              atomic.Int64
	carriedMessages      atomic.Int64
	carriedMessageBytes  atomic.Int64
	carriedCheckpoints   atomic.Int64
	carriedEmptyMessages atomic.Int64
	newMessages          atomic.Int64
	newMessageBytes      atomic.Int64
}

// runMemoryCarry is the process-wide count; run memory managers are per run
// and hold no runtime pointer.
var runMemoryCarry runMemoryCarryCounters

// RunMemoryCarryStats is the wire shape, served as run_memory_carry beside
// telemetry_batching on the guest observability endpoint.
type RunMemoryCarryStats struct {
	Carries              int64 `json:"carries"`
	CarriedMessages      int64 `json:"carried_messages"`
	CarriedMessageBytes  int64 `json:"carried_message_bytes"`
	CarriedCheckpoints   int64 `json:"carried_checkpoints"`
	CarriedEmptyMessages int64 `json:"carried_empty_messages"`
	NewMessages          int64 `json:"new_messages"`
	NewMessageBytes      int64 `json:"new_message_bytes"`
}

func (c *runMemoryCarryCounters) snapshot() RunMemoryCarryStats {
	return RunMemoryCarryStats{
		Carries: c.carries.Load(), CarriedMessages: c.carriedMessages.Load(),
		CarriedMessageBytes: c.carriedMessageBytes.Load(), CarriedCheckpoints: c.carriedCheckpoints.Load(),
		CarriedEmptyMessages: c.carriedEmptyMessages.Load(),
		NewMessages:          c.newMessages.Load(), NewMessageBytes: c.newMessageBytes.Load(),
	}
}

// recordCarry counts one successful carry: the checkpoint written (if any) and
// the tail messages.
func (c *runMemoryCarryCounters) recordCarry(checkpoint bool, tail [][]byte) {
	if !checkpoint && len(tail) == 0 {
		return
	}
	c.carries.Add(1)
	if checkpoint {
		c.carriedCheckpoints.Add(1)
	}
	var bytes, empty int64
	for _, m := range tail {
		bytes += int64(len(m))
		if len(m) == 0 {
			empty++
		}
	}
	c.carriedMessages.Add(int64(len(tail)))
	c.carriedMessageBytes.Add(bytes)
	c.carriedEmptyMessages.Add(empty)
}

func (c *runMemoryCarryCounters) recordNew(msgs ...json.RawMessage) {
	var bytes int64
	for _, m := range msgs {
		bytes += int64(len(m))
	}
	c.newMessages.Add(int64(len(msgs)))
	c.newMessageBytes.Add(bytes)
}
