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
			b.WriteString("\n\n" + capsuleBrowserRecipeFor(kind != string(types.EngineeringAssignmentVerification)))
		}
	}
	return strings.TrimSpace(b.String())
}

// capsuleBrowserRecipe is the one authoritative way to run a browser in an
// engineering capsule (R10-6: four assignments made six statements about the
// toolchain, three of them false). The capsule's workspace has no
// node_modules; the Nix-built deps tree and the store Chromium are the only
// browser stack, and a bare npx fetches a newer Playwright that wants a
// browser build the capsule does not have.
const capsuleBrowserRecipe = "Capsule browser recipe (authoritative; node, npm and a store Chromium are present, do not probe for alternatives):\n" +
	"- The workspace has no node_modules. Link them from the frontend deps tree: ln -s $CHOIR_FRONTEND_DEPS/node_modules /workspace/platform/frontend/node_modules, and run Vite from that tree.\n" +
	"- Use the pinned Playwright from that node_modules (node_modules/.bin/playwright), not npx's fetched one: a bare npx playwright fetches a newer version that cannot find its browser.\n" +
	"- Launch chromium with executablePath set to $CHOIR_BROWSER, the store Chromium (a job has it, and FONTCONFIG_FILE, on every computer). Never run npx playwright install.\n" +
	"- Mock the backend with context.route for signed-in views.\n" +
	"- Link, run and unlink inside one job, ending as the build job does (s=$?; rm -rf node_modules; exit $s), write the screenshots you want the owner to see as .png files in /var/lib/artifact/screens/ (Freeze saves them as files Texture embeds) and other JSON and Playwright output under /tmp, never under /workspace/platform: a job that leaves the workspace changed is refused as evidence, and a leftover link freezes into the candidate.\n" +
	"- A frontend spec (import ... from '@playwright/test') fails with ERR_MODULE_NOT_FOUND in the bare workspace: run it as a job through sh /workspace/platform/scripts/choir-spec.sh tests/<spec>.js [playwright args], which links the locked node_modules, runs playwright test and removes the link. theme-contract.spec.js has a test per first-party package: --grep \"package <name> has\" runs only that package's check. Do not create or leave a frontend/node_modules directory yourself: choir-spec.sh refuses one that is not its link, and choir-shot.mjs builds in a private copy under /tmp and ignores it.\n" +
	"- Do not hand-write the Playwright script: node /workspace/platform/scripts/choir-shot.mjs --shots /tmp/shots.mjs [--only <name>] [--preset mobile] builds once, serves, waits for idle and a selector, and writes the PNGs to /var/lib/artifact/screens/; run it as a job, one shot while iterating and the whole set once before Freeze.\n" +
	"- A browser needs the capsule's loopback, which only an ecosystem_proxy or claude capsule has. In a networkless capsule (every verification capsule) Vite cannot bind: say so in the report instead of probing."

// capsuleComputerStateRecipe replaces the mock bullet for an implementation
// capsule, the only kind with /computer
// (docs/design/capsule-overlay-computer-state-2026-11-03.md).
const capsuleComputerStateRecipe = "- Signed-in views show this computer's real content: /computer/files is a point-in-time copy of its files and /computer/state.db a private copy of its store (data, not access; writes there stay in this capsule; /computer/receipt.json says what they hold and leave out). In the same job, before Vite, start an autoputer over them on Vite's /api port: RUNTIME_STORE_PATH=/computer/state AUTOPUTER_FILES_ROOT=/computer/files AUTOPUTER_PORT=8082 CHOIR_GUEST_TRUST_LOOPBACK=1 /var/lib/artifact/release/bin/autoputer & (with no Go change there is no such binary: first build one, go build -o /tmp/autoputer ./cmd/autoputer as its own job, and start /tmp/autoputer instead). It has no gateway token, signer or credential here, so model calls answer from the stub provider; it trusts any caller on the capsule's own loopback, and the capsule's network namespace is that boundary. Send the header X-Authenticated-User with any name through Playwright's extraHTTPHeaders, and mock only /auth with context.route. With no /computer (a copy the receipt says was refused), mock the backend with context.route instead.\n"

// capsuleBrowserRecipeFor is the recipe for an assignment's capsule: an
// implementation capsule serves real content from its copies.
func capsuleBrowserRecipeFor(implementation bool) string {
	if !implementation {
		return capsuleBrowserRecipe
	}
	return strings.Replace(capsuleBrowserRecipe, "- Mock the backend with context.route for signed-in views.\n", capsuleComputerStateRecipe, 1)
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
