# Live-proof staging gaps — problem receipt (2026-09-04)

Planning consensus (codex, cursor, opencode, omp-gpt56-sol,
omp-gpt56-terra, omp-gemini38, omp-cursor-grok46; convergent) on the Def 2
item 4 live sealed proof. Verdict: **7/7 block as framed; proceed with
changes.** Raw outputs:
`.agentic-consensus/agentic-consensus-20260904-112235/`.

## Confirmed gaps (traced in-tree by panelists, accepted)

**G1 — No product-path guest env channel (blocking).** `CHOIR_ACTUATOR`
reaches the broker only via the guest autoputer process env
(`internal/capsule/executor.go:333`). Guest env arrives solely through
the kernel-param allowlist (`nix/autoputer-vm.nix`, no `choir.actuator`
branch; unmatched params dropped). `VMConfig`/`VMManagerConfig` and the
owner refresh request (`internal/proxy/computer_lifecycle.go:94-103`,
idempotency key only) have no actuator field, and
`refreshConfigForCurrentDeploy` zeroes `KernelParams`. There is
currently no way to stage RLM on a live computer through any
owner-scoped product path. Piggyback-without-image is dead (grok,
gemini with nix line evidence).

**G2 — In-cell assign/message are not host-reconciled (blocking per
sol/terra/codex; scoped-acceptance per cursor/opencode/grok).**
`handleAssign`/`handleMessage` store payloads in worker-local maps
reporting `"dispatched"`/`DeliveredAt`
(`internal/yaegikernel/broker.go:307-350`); `DrainReceipts` deletes and
returns lossy `assign/<id>/<task>` strings; `Executor.GoEval` persists
only the outer execution receipt and no agentcore consumer reads
`GoEvalResult.Receipts`. Claiming the stated "assign" arc on markers
alone would green-wash the proof. The panel splits on scope: full
durable subordinate creation (a code mission: structured receipts +
trusted reducer + replay safety) versus accepting in-cell receipts plus
the JSON `record_assignment_result` terminalization already in the RLM
overlay. Owner decision required before code.

**G3 — Prompt/schema split-brain (blocking for a reliable run,
gemini).** `internal/runtimeprompts/overlays/co_super_runtime.yaml:7`
names the JSON exec/file tools as the complete authority with zero
mention of `capsule_go_eval` or `import "choir"`. Under the sealed
overlay the model will hallucinate stripped tools within a frozen loop
budget.

## Adjudication

- Assignment vehicle is settled (unanimous): owner → Texture
  (`choir texture tell` on a dedicated proof document, or `choir run
  start` + prompt-bar) → persistent Super → `assign_co_super`. Never
  HTTP Super-start, never direct store construction.
- Mutation class for the plumbing is **red** (sol/terra/codex): it
  crosses owner-scoped VM lifecycle/configuration and deployment
  routing, and Def 2 currently excludes vmctl — the Definition's
  authority boundary must be amended in the repair commit.
- Persistence scope splits: durable VMConfig field re-applied per boot
  (grok: only form surviving `refreshConfigForCurrentDeploy`) versus
  one-realization override failing safe to tools (sol). Owner decision;
  repair implements the chosen one.
- Abort contract (unanimous): pre-declared; refresh-reconcile, never
  cancel-as-shortcut; close assignment fate before rollback refresh;
  `Fallback=true` or route mismatch fails acceptance, never retries by
  raising loop limits.
- gemini's exact nix snippet (`choir.actuator=*` branch at
  autoputer-vm.nix:338) and vmmanager `runtimeArgs` site
  (`manager.go:1403-1434`) are taken as the implementation sketch,
  verified at implementation time.

## Repair plan (red; rollback: unset + refresh to tools, git revert)

1. This receipt (green, code-free first).
2. Owner decisions: G2 scope (full reconciler vs receipts +
   `record_assignment_result`), persistence scope (durable vs
   one-realization), proof artifact fate, abort authority.
3. Plumbing: `VMConfig.Actuator` + `choir.actuator=` kernel arg + nix
   ENV_FILE mapping + owner refresh transport + route readback/schema
   digest evidence + G3 prompt alignment. Tests: enum strictness,
   tools default, refresh round-trip, exact RLM schema, broker/agentcore
   route agreement.
4. Red landing loop (CI → deploy → identity → tools-refresh control).
5. Live proof per gates A/B/C (env stage, assignment, arc, fate close,
   rollback), each owner-gated.
