package runtime

import "testing"

func TestDefaultChannelIDIgnoresLegacyWorkID(t *testing.T) {
	t.Parallel()

	// Given: metadata still carrying the retired work_id field but no channel_id.
	metadata := map[string]any{"work_id": "legacy-work-channel"}

	// When: the runtime derives a channel for a Super run.
	channelID := defaultChannelID(AgentProfileSuper, metadata, nil, "super:owner")

	// Then: the canonical agent channel wins instead of the deleted legacy fallback.
	if channelID != "super:owner" {
		t.Fatalf("channel_id = %q, want canonical agent channel", channelID)
	}
}
