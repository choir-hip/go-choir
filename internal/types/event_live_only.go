package types

// ProgressPhaseProviderCall is the emit phase of the progress line the tool
// loop writes before each model call (payload phase "provider_call_started").
const ProgressPhaseProviderCall = "provider_call"

// LiveOnly reports whether a run event the runtime emits is published on the
// live bus and not appended to the tape.
//
// Each line named here has no durable reader (journal cuts, docs/problems/
// paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md, "Build:
// journal cuts R1 to R4"): the live readers (the Texture stream, the prompt
// ticker) subscribe to the bus, and every appended line costs a full tape
// event. The owner ruled run telemetry loss-tolerant (notes section 27
// item 18); a live-only line is lost with the process, as the open window of
// batched telemetry is.
//
// The phase matters only for loop.progress: its response line (phase
// tool_loop) is the cost record and stays durable.
//
// This is the emit-side policy. EventDurability above is what a kind costs
// when something appends it, and old tapes still hold these kinds, so every
// reader of the stored kinds keeps accepting them.
func LiveOnly(kind EventKind, phase string) bool {
	switch kind {
	case EventTextureAgentRevisionProgress:
		return true
	case EventToolInvoked:
		// The call's arguments, start and duration ride the tool.result
		// (journal cut R3); the prompt ticker reads this line on the bus.
		return true
	case EventRunProgress:
		return phase == ProgressPhaseProviderCall
	}
	return false
}
