package main

// The command registry: every `choir` command, in usage order. Each entry
// is the CLI command and, unless cliOnly says why not, the MCP tool. Scopes
// are the API key scopes the proxy checks for the routes the handler calls
// (internal/proxy requiredAPIKeyScope and the computer routes); "public"
// routes need no scope and "authenticated" means any valid key.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

var (
	readRuntime  = []string{"read:runtime"}
	writeRuntime = []string{"write:runtime"}
	readTexture  = []string{"read:texture"}
	writeTexture = []string{"write:texture"}
	lifecycle    = []string{"computer:lifecycle"}
	selfDevRead  = []string{"computer:self_development:read"}
)

func encodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func esc(s string) string { return url.PathEscape(strings.TrimSpace(s)) }

func getRaw(x *call, path string) (any, error) {
	var out json.RawMessage
	if err := x.c.do(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func sendRaw(x *call, method, path string, body any) (any, error) {
	var out json.RawMessage
	if err := x.c.do(method, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func requireSchema(raw json.RawMessage, want, what string) error {
	var envelope struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Schema != want {
		return fmt.Errorf("unsupported %s schema %q", what, envelope.Schema)
	}
	return nil
}

func computerPath(x *call, rest string) string {
	return "/api/computers/" + esc(x.str("computer")) + rest
}

// filesPath maps a guest-store path to its route, escaping each segment;
// "." and ".." segments are refused so a model-written path cannot leave
// /api/files/.
func filesPath(p string) (string, error) {
	p = strings.TrimPrefix(strings.TrimSpace(p), "/")
	segments := strings.Split(p, "/")
	for i, s := range segments {
		if s == "." || s == ".." {
			return "", usagef("path segments . and .. are not allowed")
		}
		segments[i] = url.PathEscape(s)
	}
	return "/api/files/" + strings.Join(segments, "/"), nil
}

func computerParam() param {
	return pStr("computer", "Stable ComputerID").req()
}

func idempotencyParam(desc string) param { return pStr("idempotency_key", desc) }

// ---- the registry ----

func buildRegistry() []*command {
	cmds := []*command{
		// trajectories and lifecycle
		{
			path: []string{"trajectories"}, tool: "trajectories_list",
			summary: "List recent trajectories (ingestion/run state)",
			scopes:  readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				var resp trajectoriesListResponse
				if err := x.c.do(http.MethodGet, "/api/trajectories", nil, &resp); err != nil {
					return nil, err
				}
				if len(resp.Trajectories) > defaultListLimit {
					resp.Trajectories = resp.Trajectories[:defaultListLimit]
				}
				return resp, nil
			},
		},
		{
			path: []string{"trajectory"}, tool: "trajectory_get",
			summary: "Show one trajectory's obligations",
			params:  []param{pStr("trajectory_id", "Trajectory id").req().one()},
			scopes:  readRuntime, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, "/api/trajectories/"+esc(x.str("trajectory_id"))) },
		},
		{
			path: []string{"trajectory", "cancel"}, summary: "Cancel an owner-scoped trajectory",
			params: []param{
				pStr("trajectory_id", "Trajectory id").req().one(),
				idempotencyParam("Stable caller-supplied command key for replay/conflict detection").req(),
				pInt("expected_lifecycle_version", "Lifecycle version observed before cancellation").req().atLeast(1),
				pStr("expected_head_revision_id", "Artifact head observed before cancellation").req(),
				pStr("reason", "Cancellation reason included in the request commitment").dflt("owner cancellation"),
			},
			usage:  "--idempotency-key, --expected-lifecycle-version, and --expected-head-revision-id are required",
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: func(x *call) (any, error) {
				return sendRaw(x, http.MethodPost, "/api/trajectories/"+esc(x.str("trajectory_id"))+"/cancel", map[string]any{
					"idempotency_key":            x.trim("idempotency_key"),
					"expected_lifecycle_version": x.num("expected_lifecycle_version"),
					"expected_head_revision_id":  x.trim("expected_head_revision_id"),
					"reason":                     x.trim("reason"),
				})
			},
		},
		{
			path: []string{"lifecycle", "snapshot"}, summary: "Reconstruct one durable lifecycle from canonical state",
			params: []param{pStr("trajectory_id", "Trajectory id").req().one()},
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				out, err := getRaw(x, "/api/trajectories/"+esc(x.str("trajectory_id")))
				if err != nil {
					return nil, err
				}
				return out, validateDurableWorkResponse(out.(json.RawMessage))
			},
		},
		{
			path: []string{"lifecycle", "events"}, summary: "Read reducer events after a durable cursor",
			params: []param{
				pStr("trajectory_id", "Trajectory id").req().one(),
				pInt("after", "Event cursor").dflt(0).atLeast(0),
				pInt("limit", "Maximum events per page").dflt(100).atLeast(1),
			},
			usage:  "trajectory id, non-negative --after, and positive --limit required",
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				out, err := getRaw(x, fmt.Sprintf("/api/trajectories/%s/events?after=%d&limit=%d", esc(x.str("trajectory_id")), x.num("after"), x.num("limit")))
				if err != nil {
					return nil, err
				}
				return out, validateDurableWorkResponse(out.(json.RawMessage))
			},
		},
		{
			path: []string{"lifecycle", "capsule-evidence"}, summary: "Read one capsule assignment attempt's evidence",
			params: []param{
				pStr("trajectory_id", "Trajectory id").req().one(),
				pStr("assignment_id", "Assignment id").req().one(),
				pInt("attempt", "Positive assignment attempt").req().atLeast(1),
			},
			usage:  "trajectory id, assignment id, and positive --attempt required",
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				tid, aid := x.str("trajectory_id"), x.str("assignment_id")
				if tid != strings.TrimSpace(tid) || aid != strings.TrimSpace(aid) {
					return nil, usagef("trajectory id, assignment id, and positive --attempt required")
				}
				out, err := getRaw(x, fmt.Sprintf("/api/trajectories/%s/capsule-evidence/%s?attempt=%d", url.PathEscape(tid), url.PathEscape(aid), x.num("attempt")))
				if err != nil {
					return nil, err
				}
				if requireSchema(out.(json.RawMessage), vocabmigrate.SchemaCapsuleEvidenceV1, "capsule evidence") != nil {
					return nil, errors.New("invalid capsule evidence response")
				}
				return out, nil
			},
		},
		{
			path: []string{"identity"}, tool: "identity_verify",
			summary: "Verify a nonce-bound signed execution identity",
			desc:    "Fetch and verify the computer's nonce-bound execution identity: the guest's signed receipt, the platform's join attestation against the repository trust manifest, and one commit across host, guest, route and deployment.",
			scopes:  []string{"acceptance:read"}, readOnly: true,
			handler: verifyExecutionIdentity,
		},

		// texture
		{
			path: []string{"texture", "create"}, summary: "Create a Texture document",
			params: []param{
				pStr("title", "Texture document title").req(),
				pStr("content", "initial owner-authored v0 content and objective").req(),
				pStr("request_id", "stable client occurrence id for exact retries").req(),
				pStr("desk", "bound desk; only texture (the default) is accepted"),
			},
			usage:  "--title, --content, and --request-id are required",
			scopes: writeTexture, idempotent: true, openWorld: true,
			handler: func(x *call) (any, error) {
				body := map[string]string{"title": x.trim("title"), "initial_content": x.trim("content"), "client_request_id": x.trim("request_id")}
				if d := x.trim("desk"); d != "" {
					body["desk"] = d
				}
				out, err := sendRaw(x, http.MethodPost, "/api/texture/lifecycle-documents", body)
				if err != nil {
					return nil, err
				}
				return out, requireSchema(out.(json.RawMessage), "choir.texture_create.v1", "create")
			},
		},
		{
			path: []string{"texture", "revise"}, summary: "Revise a Texture document with an owner directive",
			params: []param{
				pStr("doc_id", "Texture document id").req().one(),
				pStr("prompt", "owner directive carried by the revision").req(),
				pStr("expected_head", "head revision id observed before revising"),
				pStr("request_id", "stable client occurrence id for exact retries"),
			},
			usage:  "--prompt is required",
			scopes: writeTexture, openWorld: true,
			handler: func(x *call) (any, error) {
				body := map[string]string{"prompt": x.trim("prompt")}
				if v := x.trim("request_id"); v != "" {
					body["client_request_id"] = v
				}
				if v := x.trim("expected_head"); v != "" {
					body["expected_head_revision_id"] = v
				}
				return sendRaw(x, http.MethodPost, "/api/texture/documents/"+esc(x.str("doc_id"))+"/revise", body)
			},
		},
		{
			path: []string{"texture", "read"}, summary: "Read a Texture document's metadata (title, current revision id) and its embeds (evidence digests and how to fetch them)",
			params: []param{pStr("doc_id", "Texture document id").req().one()},
			scopes: readTexture, readOnly: true,
			handler: textureRead,
		},
		{
			path: []string{"texture", "history"}, summary: "List revision history for a document (metadata only)",
			params: []param{pStr("doc_id", "Texture document id").req().one()},
			scopes: readTexture, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, "/api/texture/documents/"+esc(x.str("doc_id"))+"/history")
			},
		},
		{
			path: []string{"texture", "revisions"}, summary: "List revisions with full content bodies",
			params: []param{pStr("doc_id", "Texture document id").req().one()},
			scopes: readTexture, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, "/api/texture/documents/"+esc(x.str("doc_id"))+"/revisions")
			},
		},
		{
			path: []string{"texture", "show"}, summary: "Show the current or an exact historical version as JSON",
			params: []param{
				pStr("doc_id", "Texture document id").req().one(),
				pStr("revision", "exact historical revision id; defaults to current head"),
			},
			scopes: readTexture, readOnly: true,
			handler: textureShow,
		},
		{
			path: []string{"texture", "watch"}, summary: "Watch durable version/control events as JSONL",
			params: []param{
				pStr("doc_id", "Texture document id").req().one(),
				pInt("after", "resume after this durable cursor").dflt(0),
				pInt("limit", "events per durable page").dflt(100),
				pBool("once", "fetch one durable page and exit"),
				pDuration("poll_interval", "delay between caught-up durable pages").dflt(time.Second),
				pInt("reconnect_attempts", "consecutive request failures tolerated before exit").dflt(3),
			},
			scopes: readTexture, readOnly: true,
			cliOnly: "streams JSONL until a terminal event; an MCP tools/call returns once (Claude caps a call at 240s). Planned: the hosted server's resource subscription on choir://texture/{doc}/events",
			legacy:  runTextureWatch,
		},
		{
			path: []string{"texture", "open-source"}, summary: "Open one pinned source of a Texture revision",
			params: []param{
				pStr("doc_id", "Texture document id").req().one(),
				pStr("revision", "exact Texture revision id").req(),
				pStr("source_ref", "canonical source_ref id").req(),
				pStr("source_ref_version", "exact source_ref version id").req(),
			},
			usage:  "--revision, --source-ref, and --source-ref-version are required",
			scopes: readTexture, readOnly: true,
			handler: func(x *call) (any, error) {
				query := url.Values{}
				query.Set("revision_id", x.str("revision"))
				query.Set("source_ref_id", x.str("source_ref"))
				query.Set("source_ref_version_id", x.str("source_ref_version"))
				out, err := getRaw(x, "/api/texture/documents/"+esc(x.str("doc_id"))+"/source-open?"+query.Encode())
				if err != nil {
					return nil, err
				}
				return out, requireSchema(out.(json.RawMessage), "choir.texture_source_open.v1", "source-open")
			},
		},

		// search and runs
		{
			path: []string{"search"}, summary: "Search the corpus",
			params: []param{pStr("query", "Search text").req().join()},
			scopes: []string{"public"}, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, "/api/platform/retrieval/search?q="+url.QueryEscape(x.trim("query")))
			},
		},
		{
			path: []string{"run", "start"}, summary: "Submit a prompt to the conductor (starts a run)",
			params: []param{
				pStr("text", "Prompt text").req().join().orSay("prompt text required"),
				idempotencyParam("Stable caller-supplied lifecycle command key for replay/conflict detection").req(),
			},
			usage:  "--idempotency-key is required",
			scopes: writeRuntime, idempotent: true, openWorld: true,
			handler: func(x *call) (any, error) {
				var resp promptBarSubmitResponse
				if err := x.c.do(http.MethodPost, "/api/prompt-bar", map[string]string{"text": x.trim("text"), "command_id": x.trim("idempotency_key")}, &resp); err != nil {
					return nil, err
				}
				if resp.Schema != "choir.durable_work.v1" || resp.CommandID == "" || resp.StartRequestDigest == "" ||
					resp.TrajectoryID == "" || resp.DocID == "" || resp.RevisionID == "" ||
					resp.SubjectID == "" || len(resp.ObligationIDs) == 0 || resp.ReducerSeq <= 0 || resp.SnapshotCursor <= 0 {
					return nil, errors.New("incomplete durable-work response")
				}
				return resp, nil
			},
		},
		{
			path: []string{"run", "status"}, summary: "Get the status of a prompt-bar submission",
			params: []param{pStr("run_id", "Run id").req().one()},
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, "/api/runs/"+esc(x.str("run_id"))) },
		},
		{
			path: []string{"run", "list"}, summary: "List recent runs",
			params: []param{pInt("limit", "Maximum number of recent runs").dflt(defaultListLimit).between(1, 500)},
			usage:  "--limit must be between 1 and 500",
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, fmt.Sprintf("/api/runs?limit=%d", x.num("limit"))) },
		},
		{
			path: []string{"run", "cancel"}, summary: "Cancel a run",
			params: []param{pStr("run_id", "Run id").req().one()},
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: func(x *call) (any, error) {
				return sendRaw(x, http.MethodPost, "/api/runs/"+esc(x.str("run_id"))+"/cancel", nil)
			},
		},

		// files
		{
			path: []string{"files", "mkdir"}, summary: "Create a directory in the guest store",
			params: []param{pStr("path", "Guest-store path").req().one()},
			scopes: writeRuntime, idempotent: true,
			handler: func(x *call) (any, error) {
				p, err := filesPath(x.str("path"))
				if err != nil {
					return nil, err
				}
				var resp map[string]any
				if err := x.c.do(http.MethodPost, p, nil, &resp); err != nil {
					return nil, err
				}
				return resp, nil
			},
		},
		{
			path: []string{"files", "put"}, summary: "Upload stdin, --local or content to a guest-store file (creates or replaces)",
			params: []param{
				pStr("path", "Guest-store path").req().one(),
				pStr("local", "Local file to upload (default: read stdin)").onlyCLI("reads the caller's local filesystem; MCP passes content instead (a hosted server has no local files, and a model must not pick local files to upload)"),
				pStr("content", "UTF-8 file content").req().onlyMCP("the CLI reads bytes from --local or stdin"),
			},
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: func(x *call) (any, error) {
				p, err := filesPath(x.str("path"))
				if err != nil {
					return nil, err
				}
				var raw []byte
				switch {
				case x.surface == "mcp":
					raw = []byte(x.str("content"))
				case x.trim("local") != "":
					if raw, err = os.ReadFile(x.trim("local")); err != nil {
						return nil, err
					}
				default:
					if raw, err = io.ReadAll(x.stdin); err != nil {
						return nil, err
					}
				}
				var resp map[string]any
				if err := x.c.doRaw(http.MethodPut, p, raw, &resp); err != nil {
					return nil, err
				}
				return resp, nil
			},
		},
		{
			path: []string{"files", "get"}, summary: "Download a guest-store file (raw bytes)",
			params: []param{pStr("path", "Guest-store path").req().one()},
			scopes: readRuntime, readOnly: true,
			handler: func(x *call) (any, error) {
				p, err := filesPath(x.str("path"))
				if err != nil {
					return nil, err
				}
				raw, err := x.c.doRawBytes(http.MethodGet, p, nil)
				if err != nil {
					return nil, err
				}
				return fileBytes(raw), nil
			},
		},

		// computer
		computerGet("status", "Read the computer's lifecycle status", "/lifecycle/status", lifecycle),
		computerGet("replay-completeness", "Capture live-versus-event-replay state evidence", "/self-development/replay-completeness", selfDevRead),
		computerPost("replace-workspace", "Quarantine the VM-local workspace onto current DDL", "/lifecycle/replace-workspace", nil, true),
		ownerAct(computerCheckpointed("rematerialize-from-tape", "Rebuild VM-local state from the event tape", "/lifecycle/rematerialize-from-tape", nil)),
		computerPost("checkpoint", "Bind a restore-set checkpoint or refuse live-only/underivable SPA", "/lifecycle/checkpoint", map[string]any{}, false),
		computerPost("import-residue-snapshot", "Append current desktop+OG residue as one tape snapshot", "/lifecycle/import-residue-snapshot", nil, false),
		ownerAct(computerCheckpointed("restore", "Restore VM-local state and served SPA from a checkpoint", "/lifecycle/restore", []string{"vm_local", "computer_surface_frontend"})),
		ownerAct(computerPost("bootstrap-chain", "Establish a canonical event chain on a pre-genesis computer", "/lifecycle/bootstrap-chain", nil, true)),
		{
			path: []string{"computer", "producer-reports"}, summary: "List pending producer reports",
			params: []param{
				computerParam(),
				pStr("agent_id", "Filter by producer agent ID"),
				pBool("stale_delivered", "List pending reports already delivered to non-active runs"),
			},
			usage:  "--computer is required",
			scopes: lifecycle, readOnly: true,
			handler: func(x *call) (any, error) {
				path := computerPath(x, "/lifecycle/producer-reports")
				query := url.Values{}
				if v := x.trim("agent_id"); v != "" {
					query.Set("agent_id", v)
				}
				if x.flag("stale_delivered") {
					query.Set("stale_delivered", "true")
				}
				if len(query) > 0 {
					path += "?" + query.Encode()
				}
				return getRaw(x, path)
			},
		},
		{
			path: []string{"computer", "settle-producer-reports"}, summary: "Settle pending producer reports",
			params: []param{
				computerParam().orSay("--computer is required"),
				pStr("command_id", "Idempotency command ID (generated if empty)"),
				pStr("trajectory", "Trajectory ID"),
				pList("report_ids", "Report IDs to settle (repeatable or comma-separated; positional arguments too)").flagName("report-ids").commas().rest().req().
					orSay("at least one report ID is required (via --report-ids or positional args)"),
				pStr("reason", "Settlement reason"),
			},
			scopes: lifecycle, destructive: true, idempotent: true,
			handler: func(x *call) (any, error) {
				cmdID := x.trim("command_id")
				if cmdID == "" {
					cmdID = fmt.Sprintf("cmd-settle-producer-reports-%d", time.Now().UTC().UnixNano())
				}
				return sendRaw(x, http.MethodPost, computerPath(x, "/lifecycle/settle-producer-reports"), map[string]any{
					"command_id":    cmdID,
					"trajectory_id": x.trim("trajectory"),
					"report_ids":    x.list("report_ids"),
					"reason":        x.trim("reason"),
				})
			},
		},
		computerAction("stop", "Stop the current computer through owner-scoped vmctl", true),
		computerAction("start", "Start or resume the current computer", false),
		computerAction("restart", "Stop then resolve the current computer (same boot artifacts)", true),
		computerAction("refresh", "Reboot onto current guest image, preserving persistent data", true),
		computerAction("recover", "Clear the maintenance hold on the current computer, then start it if it is not active", false),

		// api keys
		{
			path: []string{"api-key", "list"}, summary: "List your API keys",
			scopes: []string{"authenticated"}, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, "/auth/api-keys") },
		},
		{
			path: []string{"api-key", "create"}, summary: "Create a delegated API key (requires manage:keys or admin)",
			params: []param{
				pStr("label", "Label for the new API key").dflt("CLI key"),
				pList("scopes", "Comma-separated child scopes (must be within the caller's delegated scopes)").commas().dflt([]string{"read:texture", "read:base", "read:runtime"}),
				pStr("computer", "Optional stable ComputerID to attenuate the key to one computer; omit for an owner-wide key that controls every computer you own").fromEnv("CHOIR_COMPUTER_ID"),
				pStr("expires_at", "Optional RFC 3339 expiry (for example 2026-08-11T12:00:00Z); must be in the future"),
			},
			scopes:  []string{"manage:keys"},
			cliOnly: "returns a new bearer secret; a tool result would put a live credential into model context and chat transcripts, where a prompt-injected call could mint and exfiltrate it. Hosted MCP access is granted by OAuth consent, never by minting keys",
			handler: func(x *call) (any, error) {
				body := map[string]any{"label": x.trim("label"), "scopes": x.list("scopes"), "computer_id": x.trim("computer")}
				if body["scopes"] == nil {
					body["scopes"] = []string{}
				}
				if v := x.trim("expires_at"); v != "" {
					exp, err := time.Parse(time.RFC3339, v)
					if err != nil {
						return nil, usagef("--expires-at must be RFC 3339: %v", err)
					}
					body["expires_at"] = exp.UTC().Format(time.RFC3339)
				}
				return sendRaw(x, http.MethodPost, "/auth/api-keys", body)
			},
		},
		{
			path: []string{"api-key", "revoke"}, summary: "Revoke this key, or a delegated key with manage:keys/admin",
			params: []param{pStr("key_id", "API key id").req().one()},
			scopes: []string{"manage:keys"}, destructive: true, idempotent: true,
			handler: func(x *call) (any, error) {
				id := x.trim("key_id")
				if err := x.c.do(http.MethodDelete, "/auth/api-keys/"+esc(id), nil, nil); err != nil {
					return nil, err
				}
				return map[string]string{"revoked": id}, nil
			},
			render: func(w io.Writer, result any, _ *call) int {
				fmt.Fprintf(w, `{"revoked":%q}`+"\n", result.(map[string]string)["revoked"])
				return 0
			},
		},

		// self-development
		{
			path: []string{"self-dev", "list"}, summary: "List self-development operations (--state filters)",
			params: []param{computerParam(), pList("state", "Only operations in this state (repeatable or comma-separated)").commas()},
			usage:  selfDevUsage, scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				path := selfDevPath(x.str("computer"), "operations")
				if states := x.list("state"); len(states) > 0 {
					path += "?" + url.Values{"state": states}.Encode()
				}
				return getRaw(x, path)
			},
		},
		{
			path: []string{"self-dev", "show"}, summary: "Show one operation (--operation)",
			params: []param{computerParam().orSay(selfDevUsage), pStr("operation", "Operation ID").req()},
			usage:  "--operation is required", scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) {
				return getRaw(x, selfDevPath(x.str("computer"), "operations/"+esc(x.str("operation"))))
			},
		},
		{
			path: []string{"self-dev", "head"}, summary: "Read the computer's event head and state commitments",
			params: []param{computerParam()},
			usage:  selfDevUsage, scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, selfDevPath(x.str("computer"), "head")) },
		},
		{
			path:    []string{"self-dev", "patch"},
			summary: "Read a frozen candidate's source patch, sha256-verified (--out writes it to a file)",
			desc:    "Read a frozen self-development candidate's source patch (frozen or later) with its base commit and bundle digest. The sha256 is verified client-side; inline output is capped at 256 KiB.",
			params: []param{
				computerParam().orSay(selfDevUsage), pStr("operation", "Operation ID (frozen or later)").req(),
				pStr("out", "Write the verified patch to this file instead of inlining it").onlyCLI("writes the caller's local filesystem; MCP returns the patch inline under a size cap (a hosted server has no local files, and a model must not pick local paths)"),
			},
			usage: "--operation is required", scopes: selfDevRead, readOnly: true,
			handler: selfDevPatch,
		},
		{
			path:    []string{"self-dev", "visual"},
			summary: "List a frozen candidate's screenshots (--out DIR fetches and sha256-verifies each PNG)",
			desc:    "List a frozen self-development candidate's visual-evidence captures (frozen or later): route, size, digest, blank and resolvable, with the capture receipt. The CLI's --out also fetches each PNG, re-hashes it against its digest, and writes <route>-<digest12>.png into the directory; MCP returns the listing only.",
			params: []param{
				computerParam().orSay(selfDevUsage), pStr("operation", "Operation ID (frozen or later)").req(),
				pStr("out", "Fetch each capture's PNG, verify it against its digest, and write it into this directory (created mode 0700)").onlyCLI("writes the caller's local filesystem; MCP returns the listing only (a hosted server has no local files, and a model must not pick local paths)"),
			},
			usage: "--operation is required", scopes: selfDevRead, readOnly: true,
			handler: selfDevVisual, render: renderSelfDevVisual,
		},
		{
			path: []string{"self-dev", "wait"}, summary: "Poll an operation until it reaches --state",
			params: []param{
				computerParam(), pStr("operation", "Operation ID").req(),
				pList("state", "Stop when the operation reaches this state (repeatable or comma-separated)").commas().req(),
				pDuration("interval", "Poll interval").dflt(10 * time.Second),
				pDuration("deadline", "Give up after this long").dflt(2 * time.Hour),
			},
			scopes: selfDevRead, readOnly: true,
			cliOnly: "polls for up to --deadline (2h by default); MCP clients cap a call (Claude at 240s), so an agent polls self_dev_show instead",
			legacy:  runSelfDevWait,
		},
		{
			path: []string{"self-dev", "approve"}, summary: "Owner's single approval: arm accept_once for exactly the frozen candidate, then decide; the same key replays a landed approval",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("operation", "Operation ID awaiting approval").req(),
				idempotencyParam("Decision idempotency key (default derives from the operation)"),
				pDuration("window", "How long the owner's single approval stays armed").dflt(15 * time.Minute),
			},
			usage:       "--operation is required",
			scopes:      []string{"computer:self_development:read", "computer:self_development:mode", "computer:self_development:approve", ownerActsScope},
			destructive: true, idempotent: true,
			handler: func(x *call) (any, error) { return selfDevDecide(x, "approve") },
			cliOnly: ownerActReason,
		},
		{
			path: []string{"self-dev", "reject"}, summary: "Reject a candidate awaiting approval (--reason); the same key and reason replay a landed reject",
			params: []param{
				computerParam().orSay(selfDevUsage),
				pStr("operation", "Operation ID awaiting approval").req(),
				idempotencyParam("Decision idempotency key (default derives from the operation)"),
				pStr("reason", "Why the candidate is rejected").req(),
			},
			usage:       "--operation is required and so is --reason",
			scopes:      []string{"computer:self_development:read", "computer:self_development:approve"},
			destructive: true, idempotent: true,
			handler: func(x *call) (any, error) { return selfDevDecide(x, "reject") },
		},
		{
			path: []string{"self-dev", "mode", "get"}, summary: "Read the explicit computer mode",
			params: []param{computerParam()},
			usage:  selfDevUsage, scopes: selfDevRead, readOnly: true,
			handler: func(x *call) (any, error) { return getRaw(x, selfDevPath(x.str("computer"), "mode")) },
		},
		{
			path: []string{"self-dev", "mode", "set"}, summary: "Generation-CAS the explicit computer mode",
			params: []param{
				computerParam(),
				pStr("mode", "Computer mode").req().oneOf("off", "audit_only", "propose_only", "accept_once", "qualified_consensus", "standing_consensus"),
				pInt("expected_generation", "Expected mode generation").dflt(0).atLeast(0),
				idempotencyParam("Unique idempotency key").req(),
				pStr("expires_at", "Canonical UTC expiry for accept_once, qualified_consensus or standing_consensus (at most 24 h ahead)"),
				pStr("operation", "Exact operation ID for accept_once or qualified_consensus"),
				pStr("expected_desired_head", "Expected desired event head for accept_once or qualified_consensus"),
				pStr("expected_effective_head", "Expected effective event head for accept_once or qualified_consensus"),
				pStr("expected_pending_ref", "Expected pending transition reference (empty when absent)"),
				pStr("expected_desired_commitment", "Expected desired state commitment for accept_once or qualified_consensus"),
				pStr("expected_effective_commitment", "Expected effective state commitment for accept_once or qualified_consensus"),
				pStr("bundle", "Exact bundle digest for accept_once or qualified_consensus"),
				pStr("policy_digest", "Frozen policy digest for qualified_consensus or standing_consensus"),
				pStr("consensus_receipt_digest", "QualifiedConsensusReceipt digest for qualified_consensus"),
			},
			usage:  "--computer, --mode, and --idempotency-key are required; positional arguments are forbidden",
			scopes: []string{"computer:self_development:mode", ownerActsScope}, destructive: true, idempotent: true,
			handler: selfDevModeSet,
			cliOnly: ownerActReason,
		},

		// trace export
		{
			path: []string{"trace", "export"}, summary: "Write a trace bundle (product routes only; default ~/.config/choir-qa/traces/<trajectory>.json, mode 0600)",
			params: []param{
				pStr("trajectory_id", "Trajectory id").req().one(),
				pStr("out", "Bundle path (default ~/.config/choir-qa/traces/<trajectory>.json)"),
			},
			scopes: []string{"read:runtime", "read:texture"}, readOnly: true,
			cliOnly: "writes an unredacted owner trace bundle to a local 0600 file; inline it would put a multi-megabyte bundle into model context, and a hosted server has no local file. Planned: a resource choir://trajectories/{id}/trace-bundle",
			legacy:  runTraceExport,
		},

		// precommitments (calibration level 2)
		{
			path: []string{"precommit", "add"}, tool: "precommit_add",
			summary: "Record a calibrated claim (needs precommit:write)",
			desc:    "Record a calibrated claim (precommitment) on your Choir computer before the evidence that settles it exists. Returns the claim with its id.",
			params: []param{
				pStr("claim", "The claim, stated so it can be settled true or false (or pass it as arguments)").flagName("claim").join().req().orSay("--claim is required"),
				pNum("p", "Probability that the claim is true, 0..1").req().between(0, 1).orSay("--p is required (the probability that the claim is true)"),
				pStr("type", "assertion | forecast | commitment").dflt("assertion").oneOf("assertion", "forecast", "commitment"),
				pStr("criterion", "How the claim settles (the evidence that will decide it)"),
				pStr("deadline", "RFC 3339 time by which it should be settled"),
				pStr("project", "Project the claim belongs to").fromEnv(projectEnvVar),
				pList("tags", "Tag (repeatable, or comma-separated)").flagName("tag").commas(),
				pStr("agent", "Agent name").fromEnv(agentEnvVar).req().orSay(agentModelUsage),
				pStr("model", "Agent model id").fromEnv(modelEnvVar).req().orSay(agentModelUsage),
				pStr("session_id", "Session id").flagName("session").fromEnv(sessionEnvVar),
				pStr("idempotency_key", "Idempotency key: a rerun with the same key and claim writes nothing new").flagName("key"),
			},
			scopes: []string{"precommit:write"}, // idempotent only with a key, so not hinted
			handler: func(x *call) (any, error) {
				p, _ := x.float("p")
				return precommitAPI{x.c}.add(calibration.ClaimRequest{
					Agent:     calibration.AgentIdentity{Name: x.str("agent"), Model: x.str("model"), SessionID: x.str("session_id")},
					Claim:     x.trim("claim"),
					Type:      x.str("type"),
					P:         &p,
					Criterion: x.str("criterion"), Deadline: x.str("deadline"), Project: x.str("project"), Tags: x.list("tags"),
					IdempotencyKey: x.str("idempotency_key"),
				})
			},
		},
		{
			path: []string{"precommit", "settle"}, tool: "precommit_settle",
			summary: "Append one scorer's settlement of a claim (scorer defaults to $CHOIR_AGENT)",
			desc:    "Append one scorer's settlement of a claim, citing evidence. A scorer settles a claim once; settlements are never edited. The scorer defaults to CHOIR_AGENT: self when that is the claimant, independent otherwise.",
			params: []param{
				pStr("claim_id", "The claim id").req().one(),
				pStr("outcome", "true | false | partial | unresolvable").req().oneOf("true", "false", "partial", "unresolvable"),
				pList("evidence", "Evidence ref: commit, CI run, staging check, trace, problem doc (repeatable; at least one)").req(),
				pStr("method", "self | independent | rule | owner (default: self when you are the claimant, else independent)").oneOf("self", "independent", "rule", "owner"),
				pStr("scorer", "Scorer name (default $"+agentEnvVar+"; for rule, the rule's name)"),
				pStr("scorer_model", "Scorer model (default $"+modelEnvVar+" when the scorer defaults)"),
				pStr("note", "Short note on the outcome"),
			},
			usage:  "--outcome and at least one --evidence are required",
			scopes: []string{"precommit:write"},
			handler: func(x *call) (any, error) {
				return precommitAPI{x.c}.settle(x.str("claim_id"), settlementFromInputs(x.str("outcome"), x.list("evidence"), x.str("method"), x.str("scorer"), x.str("scorer_model"), x.str("note")))
			},
		},
		{
			path: []string{"precommit", "list"}, tool: "precommit_list",
			summary: "List claims with their settlements, governing outcome and Brier score",
			params: append(claimFilterParams(),
				pStr("status", "open (includes overdue) | overdue | settled | all").dflt("all").oneOf("open", "overdue", "settled", "all"),
				pInt("limit", "Maximum claims to return").dflt(defaultListLimit).atLeast(1),
			),
			scopes: []string{"precommit:read"}, readOnly: true,
			handler: func(x *call) (any, error) {
				q := claimFilterQuery(x)
				q.Set("status", x.trim("status"))
				q.Set("limit", strconv.FormatInt(x.num("limit"), 10))
				return precommitAPI{x.c}.list(q)
			},
		},
		{
			path: []string{"precommit", "calibration"}, tool: "calibration_summary",
			summary: "Calibration summary; --text prints the track record to paste into an agent's context",
			desc:    "Your calibration: Brier score, overconfidence, calibration bins, self-score gap, largest misses. The first text block is a short track record to paste into your context before stating new probabilities.",
			params: append(claimFilterParams(),
				pStr("scorer", "Whose settlements count; governing (default) = rule > owner > independent > self").oneOf("governing", "self", "independent", "rule", "owner"),
				pStr("group_by", "Comma-separated: agent, model, type, project (default agent,model,type)"),
				pBool("include_reconstructed", "Count claims recorded after the fact (level-1 imports)"),
				pBool("text", "Print only the pasteable track record").onlyCLI("output format: an MCP result's first text block is the track record"),
			),
			scopes: []string{"precommit:read"}, readOnly: true,
			handler: func(x *call) (any, error) {
				q := claimFilterQuery(x)
				q.Set("scorer", x.trim("scorer"))
				q.Set("group_by", x.trim("group_by"))
				if x.flag("include_reconstructed") {
					q.Set("include_reconstructed", "true")
				}
				return precommitAPI{x.c}.calibration(q)
			},
			lead: calibrationContext,
			render: func(w io.Writer, result any, x *call) int {
				if !x.flag("text") {
					return writeJSON(w, result)
				}
				fmt.Fprint(w, calibrationContext(result))
				return 0
			},
		},
		{
			path: []string{"precommit", "import-ledger"}, summary: "Import the level-1 Markdown ledger (dry run unless --apply)",
			params: []param{
				pStr("file", "The level-1 Markdown ledger").dflt("docs/calibration/claude-ledger.md"),
				pStr("model", "Model the ledger's claims were made on (required with --apply)"),
				pStr("agent_prefix", "Agent names become <prefix>/lead and <prefix>/subagent").dflt("claude-code"),
				pStr("project", "Project for the imported claims").dflt("go-choir"),
				pBool("apply", "Write to the computer (default: dry run that prints the plan)"),
			},
			scopes:  []string{"precommit:write"},
			cliOnly: "a one-time migration that reads a local Markdown ledger file and runs without a key in a dry run; agents record claims with precommit_add",
			legacy:  runPrecommitImportLedger,
		},

		// meta
		{
			path: []string{"mcp"}, summary: "Serve every MCP-exposed command as a tool over stdio MCP (key from $CHOIR_API_KEY or $CHOIR_API_KEY_FILE)",
			scopes: []string{"authenticated"}, cliOnly: "the MCP server itself",
			legacy: runMCP,
		},
		{
			path: []string{"version"}, summary: "Print CLI version",
			scopes: []string{"public"}, cliOnly: "an MCP client reads the version from initialize's serverInfo",
			legacy: func(_ []string, stdout, _ io.Writer) int {
				fmt.Fprintln(stdout, cliVersion)
				return 0
			},
		},
		{
			path: []string{"help"}, summary: "Print this usage",
			scopes: []string{"public"}, cliOnly: "an MCP client reads tools/list",
			legacy: func(_ []string, stdout, _ io.Writer) int {
				printUsage(stdout)
				return 0
			},
		},
	}
	return append(cmds, releaseCommands()...)
}

const cliVersion = "choir v0 (Phase 1: existing /api/ routes)"

const selfDevUsage = "--computer is required and positional arguments are forbidden"

const agentModelUsage = "--agent and --model (or $" + agentEnvVar + " and $" + modelEnvVar + ") are required"

func calibrationContext(result any) string {
	raw, _ := result.(json.RawMessage)
	var sum struct {
		Context string `json:"context"`
	}
	_ = json.Unmarshal(raw, &sum)
	return sum.Context
}

// claimFilterParams are the shared claim filters. The agent defaults to
// $CHOIR_AGENT; all drops it.
func claimFilterParams() []param {
	return []param{
		pStr("agent", "Claimant agent").fromEnv(agentEnvVar),
		pBool("all", "All agents (ignore the default agent)"),
		pStr("model", "Claimant model"),
		pStr("type", "assertion | forecast | commitment").oneOf("assertion", "forecast", "commitment"),
		pStr("project", "Project"),
	}
}

func claimFilterQuery(x *call) url.Values {
	q := url.Values{}
	if !x.flag("all") {
		q.Set("agent", x.trim("agent"))
	}
	q.Set("model", x.trim("model"))
	q.Set("type", x.trim("type"))
	q.Set("project", x.trim("project"))
	return q
}

// ---- computer command families ----

func computerGet(verb, summary, route string, scopes []string) *command {
	return &command{
		path: []string{"computer", verb}, summary: summary,
		params: []param{computerParam()}, usage: "--computer is required",
		scopes: scopes, readOnly: true,
		handler: func(x *call) (any, error) { return getRaw(x, computerPath(x, route)) },
	}
}

func computerPost(verb, summary, route string, body any, destructive bool) *command {
	return &command{
		path: []string{"computer", verb}, summary: summary,
		params: []param{computerParam()}, usage: "--computer is required",
		scopes: lifecycle, destructive: destructive,
		handler: func(x *call) (any, error) { return sendRaw(x, http.MethodPost, computerPath(x, route), body) },
	}
}

// ownerActReason keeps owner acts off MCP (owner, 2026-10-14: "should that
// even be in the CLI or MCP?" — undecided; the conservative default until
// the ruling). Approving a frozen candidate, setting the computer mode and
// rewriting a computer's state are the owner's supervision; an agent that
// reaches them through a tool can approve its own work.
const ownerActReason = "an owner act (approve, mode, restore, chain bootstrap, rematerialize): agents propose and read through MCP; the owner decides from their own terminal, with a key minted from their signed-in session that carries owner:acts (admin does not)"

// ownerAct marks a command as an owner act, CLI only.
// ownerAct marks a command as the owner's: CLI-only, and its key needs
// owner:acts, which the proxy requires for the act and auth grants only from
// the owner's session (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F4).
func ownerAct(cmd *command) *command {
	cmd.cliOnly = ownerActReason
	cmd.scopes = append(cmd.scopes, ownerActsScope)
	return cmd
}

const ownerActsScope = "owner:acts"

// computerCheckpointed is restore and rematerialize-from-tape: the CLI
// reads the checkpoint artifact from --checkpoint-file, MCP takes it inline.
func computerCheckpointed(verb, summary, route string, operandScopes []string) *command {
	return &command{
		path: []string{"computer", verb}, summary: summary,
		params: []param{
			computerParam(),
			pStr("checkpoint_file", "JSON checkpoint artifact").req().onlyCLI("reads the caller's local filesystem; MCP passes the checkpoint inline"),
			pJSON("checkpoint", "Checkpoint artifact: a Checkpoint, a CheckpointResponse, or the bind report choir computer checkpoint writes").req().
				onlyMCP("the CLI reads the artifact from --checkpoint-file"),
		},
		usage:  "--computer and --checkpoint-file are required",
		scopes: lifecycle, destructive: true,
		handler: func(x *call) (any, error) {
			raw := []byte(x.json("checkpoint"))
			if x.surface != "mcp" {
				var err error
				if raw, err = os.ReadFile(x.str("checkpoint_file")); err != nil {
					return nil, usageError{err.Error()}
				}
			}
			checkpoint, err := extractCheckpointArtifact(raw)
			if err != nil {
				return nil, usagef("invalid checkpoint: %v", err)
			}
			if operandScopes == nil {
				return sendRaw(x, http.MethodPost, computerPath(x, route), map[string]json.RawMessage{"checkpoint": checkpoint})
			}
			return sendRaw(x, http.MethodPost, computerPath(x, route), map[string]any{"checkpoint": checkpoint, "operand_scopes": operandScopes})
		},
	}
}

func computerAction(action, summary string, destructive bool) *command {
	return &command{
		path: []string{"computer", action}, summary: summary,
		params: []param{computerParam(), idempotencyParam("Unique idempotency key").req()},
		usage:  "--computer and --idempotency-key are required",
		scopes: lifecycle, destructive: destructive, idempotent: true,
		handler: func(x *call) (any, error) {
			return sendRaw(x, http.MethodPost, computerPath(x, "/lifecycle/"+action), map[string]string{"idempotency_key": x.trim("idempotency_key")})
		},
	}
}

// ---- larger handlers ----

func textureShow(x *call) (any, error) {
	docID := x.trim("doc_id")
	var document json.RawMessage
	if err := x.c.do(http.MethodGet, "/api/texture/documents/"+url.PathEscape(docID), nil, &document); err != nil {
		return nil, err
	}
	var head struct {
		CurrentRevisionID string `json:"current_revision_id"`
	}
	if err := json.Unmarshal(document, &head); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	requested := x.trim("revision")
	if requested == "" {
		requested = strings.TrimSpace(head.CurrentRevisionID)
	}
	if requested == "" {
		return nil, errors.New("document has no current revision")
	}
	var revision json.RawMessage
	if err := x.c.do(http.MethodGet, "/api/texture/revisions/"+url.PathEscape(requested), nil, &revision); err != nil {
		return nil, err
	}
	var exact struct {
		DocID string `json:"doc_id"`
	}
	if err := json.Unmarshal(revision, &exact); err != nil || strings.TrimSpace(exact.DocID) != docID {
		return nil, fmt.Errorf("revision %s is not an exact version of this document", requested)
	}
	return textureShowCLIResponse{
		Schema: "choir.texture_show.v1", Document: document, Revision: revision,
		Current: requested == strings.TrimSpace(head.CurrentRevisionID),
	}, nil
}

// textureRead returns the document's metadata, and, when its current
// revision holds embeds, an "embeds" list: one row per embed block with the
// digest, the candidate and the command that fetches and verifies the bytes
// (station TE, docs/design/texture-evidence-embeds-te-2026-10-25.md). A
// document with no embeds reads as before; a revision that cannot be read
// is named in "embeds_error" and does not fail the read.
func textureRead(x *call) (any, error) {
	docID := x.trim("doc_id")
	var document json.RawMessage
	if err := x.c.do(http.MethodGet, "/api/texture/documents/"+url.PathEscape(docID), nil, &document); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(document, &fields); err != nil {
		return document, nil
	}
	var head struct {
		CurrentRevisionID string `json:"current_revision_id"`
	}
	_ = json.Unmarshal(document, &head)
	revisionID := strings.TrimSpace(head.CurrentRevisionID)
	if revisionID == "" {
		return document, nil
	}
	var revision struct {
		DocID   string          `json:"doc_id"`
		BodyDoc json.RawMessage `json:"body_doc"`
	}
	if err := x.c.do(http.MethodGet, "/api/texture/revisions/"+url.PathEscape(revisionID), nil, &revision); err != nil {
		fields["embeds_error"], _ = json.Marshal("read revision " + revisionID + ": " + err.Error())
		return fields, nil
	}
	if strings.TrimSpace(revision.DocID) != docID {
		fields["embeds_error"], _ = json.Marshal("revision " + revisionID + " is not a version of this document")
		return fields, nil
	}
	rows := textureEmbedRows(revisionID, revision.BodyDoc)
	if len(rows) == 0 {
		return document, nil
	}
	fields["embeds"], _ = json.Marshal(rows)
	return fields, nil
}

type textureEmbedRow struct {
	BlockID     string `json:"block_id"`
	Kind        string `json:"kind"`
	Digest      string `json:"digest"`
	OperationID string `json:"operation_id,omitempty"`
	ComputerID  string `json:"computer_id,omitempty"`
	Route       string `json:"route,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Blank       bool   `json:"blank"`
	Caption     string `json:"caption,omitempty"`
	RevisionID  string `json:"revision_id"`
	AssetURL    string `json:"asset_url"`
	Fetch       string `json:"fetch,omitempty"`
}

func textureEmbedRows(revisionID string, body json.RawMessage) []textureEmbedRow {
	var doc texturedoc.StructuredTextureDoc
	if len(body) == 0 || json.Unmarshal(body, &doc) != nil {
		return nil
	}
	var rows []textureEmbedRow
	for _, e := range texturedoc.Embeds(doc) {
		row := textureEmbedRow{BlockID: e.ID, Kind: e.Kind, Digest: e.Digest, OperationID: e.OperationID, ComputerID: e.ComputerID,
			Route: e.Route, Width: e.Width, Height: e.Height, Blank: e.Blank, Caption: e.Caption, RevisionID: revisionID,
			AssetURL: "/api/assets/sha256/" + e.Digest}
		if e.Kind == texturedoc.EmbedKindCapsuleEvidence && e.ComputerID != "" && e.OperationID != "" {
			row.Fetch = "choir self-dev visual --computer " + shellQuote(e.ComputerID) + " --operation " + shellQuote(e.OperationID) + " --out <dir>"
		}
		rows = append(rows, row)
	}
	return rows
}

// shellQuote single-quotes s for a POSIX shell, so a pasted fetch line runs
// as shown whatever the ids hold.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func selfDevDecide(x *call, decision string) (any, error) {
	computerID, id := x.trim("computer"), x.trim("operation")
	operation, binding, replay, err := selfDevDecisionBinding(x.c, computerID, id, decision)
	if err != nil {
		return nil, err
	}
	if decision == "approve" && !replay {
		// The owner's single approval: arm accept_once bound to exactly this
		// candidate and head; the decision consumes it. A replay of a landed
		// approval arms nothing: its grant was consumed.
		var mode struct {
			Generation uint64 `json:"generation"`
		}
		if err := x.c.do(http.MethodGet, selfDevPath(computerID, "mode"), nil, &mode); err != nil {
			return nil, fmt.Errorf("read mode: %w", err)
		}
		arm := map[string]any{
			"mode": "accept_once", "expected_generation": mode.Generation, "idempotency_key": "cli-accept-once:" + id,
			"operation_id": id, "expires_at": time.Now().UTC().Add(x.dur("window")).Truncate(time.Second).Format(time.RFC3339Nano),
		}
		for field, value := range binding {
			arm[field] = value
		}
		if err := x.c.do(http.MethodPut, selfDevPath(computerID, "mode"), arm, nil); err != nil {
			return nil, fmt.Errorf("arm accept_once: %w", err)
		}
	}
	body := map[string]any{"decision": decision, "verifier_ref": operation.VerifierRefs[0]}
	for field, value := range binding {
		body[field] = value
	}
	body["idempotency_key"] = x.trim("idempotency_key")
	if body["idempotency_key"] == "" {
		body["idempotency_key"] = "cli-" + decision + ":" + id
	}
	if decision == "reject" {
		body["reason"] = x.trim("reason")
	}
	return sendRaw(x, http.MethodPost, selfDevPath(computerID, "operations/"+url.PathEscape(id)+"/decision"), body)
}

func selfDevModeSet(x *call) (any, error) {
	mode := x.str("mode")
	body := map[string]any{
		"mode": mode, "expected_generation": uint64(x.num("expected_generation")), "idempotency_key": x.str("idempotency_key"),
	}
	if mode == "accept_once" || mode == "qualified_consensus" {
		body["expires_at"] = x.str("expires_at")
		body["operation_id"] = x.str("operation")
		body["expected_desired_event_head"] = x.str("expected_desired_head")
		body["expected_effective_event_head"] = x.str("expected_effective_head")
		body["expected_pending_transition_ref"] = x.trim("expected_pending_ref")
		body["expected_desired_state_commitment"] = x.str("expected_desired_commitment")
		body["expected_effective_state_commitment"] = x.str("expected_effective_commitment")
		body["bundle_digest"] = x.str("bundle")
	}
	if mode == "qualified_consensus" {
		body["policy_digest"] = x.trim("policy_digest")
		body["consensus_receipt_digest"] = x.trim("consensus_receipt_digest")
	}
	if mode == "standing_consensus" {
		// P4: the owner's standing autonomous arm, test-tier computers only:
		// a policy (reversible-selfdev-v1) and an expiry at most 24 h ahead.
		body["policy_digest"] = x.trim("policy_digest")
		body["expires_at"] = x.str("expires_at")
	}
	return sendRaw(x, http.MethodPut, selfDevPath(x.str("computer"), "mode"), body)
}
