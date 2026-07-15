package types

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"time"
)

// EvidenceRecord is a durable piece of retrieved or generated material
// captured by an agent. It is intentionally generic so later citation,
// retrieval, and trace flows can interpret it without the schema trying to
// encode higher-level orchestration algorithms.
type EvidenceRecord struct {
	EvidenceID string          `json:"evidence_id"`
	OwnerID    string          `json:"owner_id"`
	AgentID    string          `json:"agent_id"`
	Kind       string          `json:"kind"`
	SourceURI  string          `json:"source_uri,omitempty"`
	Title      string          `json:"title,omitempty"`
	Content    string          `json:"content"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

const CoagentSourcePacketSchemaV1 = "coagent_source_packet.v1"

const terminalRunOutcomeDigestDomain = "choir.terminal-run-outcome.v1"

// TerminalRunOutcomeSHA256 returns the canonical witness for one authoritative
// terminal RunRecord outcome. Length-prefixing each field makes the encoding
// unambiguous without normalizing or copying any outcome text.
func TerminalRunOutcomeSHA256(sourceRunID string, state RunState, result, runError string) string {
	digest := sha256.New()
	writeTerminalRunOutcomeDigestField(digest, terminalRunOutcomeDigestDomain)
	writeTerminalRunOutcomeDigestField(digest, sourceRunID)
	writeTerminalRunOutcomeDigestField(digest, string(state))
	writeTerminalRunOutcomeDigestField(digest, result)
	writeTerminalRunOutcomeDigestField(digest, runError)
	return hex.EncodeToString(digest.Sum(nil))
}

func writeTerminalRunOutcomeDigestField(digest hash.Hash, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = io.WriteString(digest, value)
}

// CoagentSourcePacketPayload is the canonical update_coagent payload. It is a
// source packet, not a chat message: readable prose is a projection of this
// payload, while Texture may cite only Sources and Super may execute only
// execution_request Actions.
type CoagentSourcePacketPayload struct {
	SchemaVersion string                `json:"schema_version"`
	Kind          string                `json:"kind"`
	Summary       string                `json:"summary,omitempty"`
	Claims        []CoagentPacketClaim  `json:"claims,omitempty"`
	Sources       []CoagentPacketSource `json:"sources,omitempty"`
	Actions       []CoagentPacketAction `json:"actions,omitempty"`
	Questions     []string              `json:"questions,omitempty"`
	Notes         []string              `json:"notes,omitempty"`
}

type CoagentPacketClaim struct {
	ClaimID            string   `json:"claim_id,omitempty"`
	Text               string   `json:"text"`
	SourceIDs          []string `json:"source_ids,omitempty"`
	Stance             string   `json:"stance,omitempty"`
	RecommendedSurface string   `json:"recommended_surface,omitempty"`
}

type CoagentPacketSource struct {
	SourceID       string                             `json:"source_id,omitempty"`
	Kind           string                             `json:"kind"`
	Target         CoagentPacketSourceTarget          `json:"target"`
	Selectors      []CoagentPacketSourceSelector      `json:"selectors,omitempty"`
	Excerpt        string                             `json:"excerpt,omitempty"`
	ReaderSnapshot *CoagentPacketSourceReaderSnapshot `json:"reader_snapshot,omitempty"`
	Evidence       CoagentPacketSourceEvidence        `json:"evidence,omitempty"`
}

type CoagentPacketSourceTarget struct {
	URI       string `json:"uri,omitempty"`
	Title     string `json:"title,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

type CoagentPacketSourceSelector struct {
	Kind   string  `json:"kind"`
	Quote  string  `json:"quote,omitempty"`
	Start  int     `json:"start,omitempty"`
	End    int     `json:"end,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type CoagentPacketSourceEvidence struct {
	State       string `json:"state,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
	RightsScope string `json:"rights_scope,omitempty"`
}

type CoagentPacketSourceReaderSnapshot struct {
	TextContent       string `json:"text_content,omitempty"`
	SnapshotKind      string `json:"snapshot_kind,omitempty"`
	MediaType         string `json:"media_type,omitempty"`
	OriginalMediaType string `json:"original_media_type,omitempty"`
	SourceURL         string `json:"source_url,omitempty"`
	AccessScope       string `json:"access_scope,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

type CoagentPacketAction struct {
	ActionID        string                        `json:"action_id,omitempty"`
	Type            string                        `json:"type"`
	Objective       string                        `json:"objective"`
	Inputs          map[string]any                `json:"inputs,omitempty"`
	ExpectedSources []CoagentPacketExpectedSource `json:"expected_sources,omitempty"`
	Safety          CoagentPacketActionSafety     `json:"safety,omitempty"`
}

type CoagentPacketExpectedSource struct {
	Kind     string `json:"kind"`
	Required bool   `json:"required,omitempty"`
}

type CoagentPacketActionSafety struct {
	MutationClass string `json:"mutation_class,omitempty"`
	Network       string `json:"network,omitempty"`
	FileMutation  string `json:"file_mutation,omitempty"`
}

// CoagentSourcePacket is the persisted delivery envelope for one addressed
// source packet. The Packet field is the canonical update_coagent payload; the
// surrounding fields are runtime-owned delivery/idempotency metadata.
type CoagentSourcePacket struct {
	UpdateID            string                     `json:"update_id"`
	OwnerID             string                     `json:"owner_id"`
	AgentID             string                     `json:"agent_id"`
	TargetAgentID       string                     `json:"target_agent_id"`
	ChannelID           string                     `json:"channel_id"`
	MessageSeq          int64                      `json:"message_seq"`
	TrajectoryID        string                     `json:"trajectory_id,omitempty"`
	Role                string                     `json:"role,omitempty"`
	SourceRunID         string                     `json:"source_run_id,omitempty"`
	SourceOutcomeSHA256 string                     `json:"source_outcome_sha256,omitempty"`
	Packet              CoagentSourcePacketPayload `json:"packet"`
	Content             string                     `json:"content"`
	CreatedAt           time.Time                  `json:"created_at"`
	DeliveredToRunID    string                     `json:"delivered_to_loop_id,omitempty"`
	DeliveredAt         *time.Time                 `json:"delivered_at,omitempty"`
}
