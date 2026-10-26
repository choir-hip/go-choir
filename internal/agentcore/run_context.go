package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The run-context turn (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
//
// Providers cache a prompt by its leading bytes: the system prompt, then the
// tools, then the messages. The system prompt is therefore kept byte-stable
// per desk (systemPromptForRun), and everything that changes between runs or
// activations — the time, the run's identifiers, its requester, its
// assignment — is a user turn instead:
//
//   - a new run's first message carries the time and the run's facts;
//   - a resumed activation appends the time only, after its replayed history;
//   - a compaction appends the time and the facts again, since the summary
//     may have dropped them.
//
// Each is appended to run memory, so the next activation replays the same
// bytes this one sent and the cache holds the whole history.

const runContextHeader = "Run context:"

// corePrompt is the shared core every desk's system prompt starts with.
func (rt *Runtime) corePrompt() string {
	corePrompt := "Choir is a multiagent writing, research, and execution system with one user-facing product, one runtime, and one standard of truth."
	if rt != nil && rt.promptStore != nil {
		if loaded, err := rt.promptStore.LoadCore(); err == nil && strings.TrimSpace(loaded) != "" {
			corePrompt = loaded
		}
	}
	return corePrompt
}

// runContextForRun is what a run knows about itself that the stable system
// prompt must not carry: its identifiers, its requester, its assignment, the
// operation it supervises. Empty when the run has none.
func (rt *Runtime) runContextForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	var b strings.Builder
	if isManagementTaskRun(rec) {
		b.WriteString("You are management task " + managementTaskIDOfRun(rec) + ": management delegated ")
		if captures := metadataIntValue(rec.Metadata, "management_task_leased_count"); captures > 0 {
			b.WriteString(fmt.Sprintf("%d ", captures))
		}
		b.WriteString("captures to you.")
		// A task's identity line names the inbox it reads and the agent it
		// sends as, which the desk run-context overlay does not.
		b.WriteString("\nCurrent agent id: " + agentIDForRun(rec) + ". You read this inbox; you send as " + persistentManagementAgentID(rec.OwnerID) + ".")
		return b.String()
	}
	profile := agentProfileForRun(rec)
	requesterAgentID := metadataStringValue(rec.Metadata, "requested_by_agent_id")
	textureDeliveryAgentID := ""
	if profile == agentprofile.Research && isTextureAgentID(requesterAgentID) {
		textureDeliveryAgentID = requesterAgentID
	}
	b.WriteString(strings.TrimSpace(runtimeprompts.RunContextOverlay(runtimeprompts.RunContextOptions{
		AgentID:                agentIDForRun(rec),
		RequesterAgentID:       requesterAgentID,
		TextureDeliveryAgentID: textureDeliveryAgentID,
		ChannelID:              channelIDForRun(rec),
	})))
	if profile == agentprofile.Conductor {
		requestedApp := metadataStringValue(rec.Metadata, "requested_app")
		if requestedApp == "" {
			requestedApp = agentprofile.Texture
		}
		b.WriteString("\nRequested default app: " + requestedApp + ".")
		if seedPrompt := metadataStringValue(rec.Metadata, "seed_prompt"); seedPrompt != "" {
			b.WriteString("\nSeed prompt: " + seedPrompt + ".")
		}
	}
	if profile == agentprofile.Texture {
		if operationID, state := textureRunSelfDevelopmentOperation(rt, rec); operationID != "" {
			b.WriteString("\n\nSelf-development supervision:\nThis document supervises self-development operation " + operationID + ", now in state " + state + ". The runtime runs the operation: engineering builds and freezes the change, an independent judgment checks it, the owner approves or rejects it, and the update applies. Do not ask to verify, approve, apply or re-run it. Keep the document a true account of where the operation stands, from the reports in choir.Updates(): what was built, what the evidence shows, and what is still pending. When a report adds nothing the document needs, call choir.Wait(why) instead of writing.")
			b.WriteString(" An owner revision with new direction (request intent apply_owner_revision) is a new round to Cast once this operation settles, not a re-run, so do not Wait it away.")
		}
	}
	if profile == agentprofile.Engineering {
		if assignmentID := metadataStringValue(rec.Metadata, "assignment_id"); assignmentID != "" {
			kind := metadataStringValue(rec.Metadata, "assignment_kind")
			b.WriteString("\n\nExact authenticated assignment: assignment_id=")
			b.WriteString(assignmentID)
			b.WriteString(" kind=")
			b.WriteString(kind)
			b.WriteString(" subject_digest=")
			b.WriteString(metadataStringValue(rec.Metadata, "subject_digest"))
			if kind == string(types.EngineeringAssignmentVerification) {
				b.WriteString(" candidate_id=")
				b.WriteString(metadataStringValue(rec.Metadata, "source_candidate_id"))
				b.WriteString(". This verification capsule contains that exact immutable candidate subject.")
			} else {
				b.WriteString(". Implement only the bounded objective in /workspace/platform and return a typed result.")
				if notice := rt.carriedPatchNoticeForRun(rec); notice != "" {
					b.WriteString("\n\n" + notice)
				}
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// runContextText renders the run context: the time to the second, then the
// run's facts, if any.
func runContextText(now time.Time, facts string) string {
	var b strings.Builder
	b.WriteString(runContextHeader)
	b.WriteString("\n")
	b.WriteString(strings.TrimSpace(runtimeprompts.TemporalContext(runtimeprompts.TemporalContextOptions{
		NowUTC: now.UTC().Format(time.RFC3339),
	})))
	if facts = strings.TrimSpace(facts); facts != "" {
		b.WriteString("\n")
		b.WriteString(facts)
	}
	return b.String()
}

// isRunContextMessage reports whether a stored message is a run-context turn.
func isRunContextMessage(raw json.RawMessage) bool {
	var msg struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &msg) != nil || msg.Role != "user" || len(msg.Content) != 1 {
		return false
	}
	return msg.Content[0].Type == "text" && strings.HasPrefix(msg.Content[0].Text, runContextHeader+"\n")
}

// runContextMessage is the run-context turn as a user message.
func runContextMessage(now time.Time, facts string) json.RawMessage {
	msg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []any{map[string]string{"type": "text", "text": runContextText(now, facts)}},
	})
	return msg
}

// promptCacheScope names the conversation thread whose calls share a
// provider prompt cache: the computer and the desk's agent (a Texture
// document, a persistent desk, an assignment, a task). Successive runs and
// activations of one desk share it; two computers never do. It is hashed
// before it leaves the runtime (toolregistry.PromptCacheKey).
func promptCacheScope(rec *types.RunRecord) (label, scope string) {
	if rec == nil {
		return "", ""
	}
	label = agentProfileForRun(rec)
	if isManagementTaskRun(rec) {
		label = agentprofile.ManagementTask
	}
	thread := agentIDForRun(rec)
	if thread == "" {
		return "", ""
	}
	return label, strings.TrimSpace(rec.OwnerID) + "\x00" + strings.TrimSpace(rec.ComputerID) + "\x00" + thread
}
