# P1 Provider Freeze — OpenCode Go and Zen

Frozen 2026-09-11 for Definition
`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`, acceptance item
P1-provider (red surface: gateway/provider routing, Node B credentials, model
routing). Frozen before implementation. Facts below re-pin the 2026-09-10
research note (`docs/reports/choir-opencode-provider-research-2026-09-10.md`);
live probes in this phase re-confirm them before any roster conclusion.

## Model-to-wire-shape map

| Model id | Provider | Wire shape | Path |
| --- | --- | --- | --- |
| `deepseek-v4.1-flash` | `opencode-go` | OpenAI chat completions | `/zen/go/v1/chat/completions` |
| `glm-5.3-flash` | `opencode-go` | OpenAI chat completions | `/zen/go/v1/chat/completions` |
| `muse-spark-1.3-contributor` | `opencode-go` | OpenAI Responses | `/zen/go/v1/responses` |
| `muse-spark-1.3-contributor-free` | `opencode-zen` | OpenAI Responses | `/zen/v1/responses` |
| `gpt-5.6-luna` | `chatgpt` (existing) | OpenAI Responses | existing ChatGPT-authenticated path |
| `hy3` | `opencode-go` | OpenAI chat completions | `/zen/go/v1/chat/completions` (experiment only; excluded from image-bearing steps) |
| `qwen3.7-max` | `opencode-go` | Anthropic Messages | `/zen/go/v1/messages` (experiment only) |

Base URLs: `https://opencode.ai/zen/v1` (Zen), `https://opencode.ai/zen/go/v1`
(Go). Auth: `Authorization: Bearer <key>` for chat completions and Responses;
`x-api-key: <key>` for Messages. The Anthropic Messages shape is implemented
for the qwen experimental row; the expected-pass roster needs only chat
completions and Responses.

## Session identity

- `x-opencode-session` is mandatory on every Go and Zen request (missing →
  HTTP 400 `MissingSessionID`, observed live 2026-09-10).
- The value is the durable `RunID` of the calling run, threaded as
  `conversation_id`: `provideriface.ToolLoopRequest.ConversationID` →
  `gatewayruntime.llmRequest.conversation_id` →
  `gateway.ProviderRequest.conversation_id` → `provider.LLMRequest.ConversationID`
  → `x-opencode-session`. The tool loop populates it from the run's durable
  `RunID`; the gateway decoder tolerates the additive field on version skew.
- **Fail-closed rule**: the OpenCode adapter refuses to send any request whose
  `ConversationID` is empty. The failure is ours and visible, never a 400 from
  upstream. A per-request random value is rejected outright; an install-wide
  constant is the only acceptable non-run fallback and is not used here.
- `conversation_id` is request metadata for routing/cache affinity, never a
  trust input.

## Product User-Agent

`User-Agent: choir-gateway/0.1` on every OpenCode request (no provider sets a
UA today; this adapter is the first egress that must identify itself).

## Credential variable names

- `OPENCODE_API_KEY` — single key covering both Zen and Go (same key per the
  research note). `OPENCODE_GO_API_KEY` accepted as a fallback source.
- `nix/deploy-provider-creds.sh` gains `OPENCODE_API_KEY` in its pass-through
  list; delivery is that script (the authorized operator path, owner-approved
  with a standing-question-9 exception note), never ad hoc SSH edits.

## Prior host-config digest and backup

- Pre-change `/var/lib/go-choir/gateway-provider.env` on Node B:
  sha256 `7c5cc6e848471bc0e7afccbcfd3704c61dc185dffce0c535379f7c817bd5b8ef`
  (22 lines), captured 2026-09-11.
- Backup: `~/.config/go-choir/backups/gateway-provider.env.2026-09-11-pre-p1`
  on the operator workstation (digest matches the remote file).

## Spend cap

Existing subscription caps only; auto-reload stays OFF (owner decision). No
new spend accounting lands in this phase; the per-autoputer rate limiter and
circuit breaker remain the only runtime guards. Zen free ids are used while
the free quota lasts, then the paid `muse-spark-1.3-contributor` id.

## Live proof obligations

One live call per wire shape in use (chat completions, Responses; Messages if
the qwen experiment runs) plus the empty-identity negative probe. Evidence
artifact records per call: request shape, model id, status, latency, identity
present. A provider outage must never read as a prompt or carrier failure.

## Rollback

Revert the adapter commits, remove `OPENCODE_API_KEY` from the Node B env file
(restore from the backup above), restart `go-choir-gateway`, and prove staging
health returns to the pre-phase-1 identity. The adapter path is greenfield —
no OpenCode adapter exists today — so the revert target is the added code.
