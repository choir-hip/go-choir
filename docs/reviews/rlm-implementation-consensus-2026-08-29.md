# Implementing the Real RLM on Yaegi — Design Consensus & Roadmap (2026-08-29)

The definitive design for turning Choir's actor loop into a real Recursive
Language Model — prompt-as-variable, model-authored Go cells, recursive model
calls — plus the substrate fixes and deletion wave it rides on. Grounded in
the in-repo doctrine (the persistent-RLM memo), four read-only scouts, and an
11-agent consensus panel including Claude (10 returned; gpt-5.6-luna timed
out). Panel outputs: agentic-consensus/agentic-consensus-20260829-164143.

---

## 1. Where things stand

Choir's actors are durable but their "brain" is a conventional harness: a
provider tool-loop that stuffs context into prompts. The yaegi Go interpreter
exists only as an ordinary per-call sandbox tool — a fresh interpreter per
invocation, no persistent REPL, no prompt bound as a variable, no way for the
model to call a sub-model from inside code. The original RLM paradigm (arXiv
2512.24601; reference `alexzhang13/rlm`; Prime Intellect's prime-agent) is
precisely the opposite: **the prompt is loaded into a persistent REPL as a
variable the model programmatically inspects and slices, and the model can
recursively call sub-LLMs from within the REPL.**

Choir's own doctrine memo (`docs/memo-persistent-rlm-actors-2026-08-09.md`)
already specifies this correctly: the loop is *model → Go cell → observations
→ model → typed outcome*; the actor is durable but the **activation is
disposable**; Go variables and artifact refs are working memory that live only
while an activation runs; a typed continuation (artifact refs, step receipts,
pending assignments, open questions, budget) is all that survives a park or a
kill. The memo even sketches the model API — `models.Call/Parallel/Map/
Pipeline/Reduce` — and the security posture: no raw network from interpreted
code, credentials never enter the capsule, every model call receipted, and the
RLM can never Accept/Materialize/Checkpoint/Route (canonical authority).

What was missing was the implementation plan. The panel produced one.

## 2. The design in one paragraph

**One worker process and one persistent yaegi interpreter per activation.**
Cells run serialized as framed messages over the existing signed capsule
broker RPC — the one-shot `go_eval` path stays byte-for-byte unchanged when no
session ID is present, so the candidate-A execution path is untouched. The
host prebinds a flat, immutable `context` DTO (prompt, identity heads, budget
view, opaque artifact refs — never credentials, handles, or live objects)
through the `extraSymbols` parameter that already exists in
`NewEvaluator` and is currently always `nil`. After every cell the host
returns a metadata table of top-level variables (name, type, length, digest,
redacted preview) appended to the model's observation — that is the
context-as-variable loop. A `models` module (host-compiled `interp.Exports`)
exposes `Call/Parallel/Map` whose implementations travel cell → broker → host
dispatcher → gateway, with the model token never leaving the host, every call
receipted, and all budget/policy/depth enforcement host-side. Per-agent-role
module manifests (host-owned, digest-bound into the signed activation
capability) decide which modules and stdlib each role sees. A versioned typed
continuation — refs and receipts only, never heap — is saved on park and
restored into a fresh worker on rewarm. Cutover to RLM-as-primary-actuator is
flag-gated (`actuator=tools|rlm`) and happens only after candidate A pays AND
a parity suite passes.

## 3. The design decisions (panel consensus)

**D1 — Interpreter lifecycle (high confidence, unanimous).** One worker + one
`interp.Interpreter` for the whole activation; cells serialized. `sidecar.go`
grows a framed session protocol (`init` → `eval`/`eval_result` ↔
`host_call`/`host_result` → `close`); absent session ID means today's one-shot
eval, unmodified. A cell panic/timeout/overflow kills the whole process group
and marks the session poisoned — **no partial-state salvage**; the next cell
of the same activation gets a fresh worker plus a host rebind of the last
successful flat snapshot (`InterpreterEpoch++`). Activation end (park, cancel,
deadline) always ends with `close` then SIGKILL of the group.

**D2 — Context as variable (high).** Host-side binding in a new
`internal/agentcore/rlm_activation.go` (DTO construction) and
`internal/yaegikernel/bindings.go` (interp.Exports conversion). The DTO is
identifiers and immutable observations only; artifact/memory references are
opaque typed values (ref, kind, digest, size, taint, preview). The model
cannot send prebindings. Variable metadata is delivered automatically with
each cell result (sol/claude: no `vars()` builtin, so introspection can't
become a second authority surface; grok accepts an in-cell `Vars()` as the
same snapshot — minor dissent).

**D3 — The `models` module (medium confidence — the one unproven seam).**
Surface: `Call`, `Parallel`, `Map` (claude drops `Pipeline`/`Reduce` from
phase 1; a Go loop over `Call` *is* the pattern). No `context.Context` is
exported into interpreted code (flat-DTO boundary — a justified memo
deviation). Fan-out combinators run as compiled host code, structurally
satisfying the memo's ban on unbounded interpreted goroutines. The callback
path revalidates — on every call — the activation capability, actor/computer/
trajectory/work identity, operation grant, state heads, expiry, and budget
before touching the gateway; every admitted call *and every refusal* gets a
receipt binding parent call, digests, effective model policy, usage/cost, and
heads. Grok's refinement: a **dedicated broker connection per session** so
callbacks never serialize behind the shared connection mutex during
coexistence. The framing change to the broker client is the piece to
stress-test (cancellation, concurrent callback completion).

**D4 — Per-role module manifests (high).** Module selection is host-owned and
assignment-scoped — never trusted from model-authored request fields. An
immutable `ModuleManifest` (ID/version/digest) is bound into the signed
activation capability; the broker resolves module IDs to `interp.Exports` and
any stdlib-allowlist delta; attempting to change the manifest mid-session
kills the session. Recommended initial profiles:

| Role | Modules | Explicitly absent |
|---|---|---|
| Super | context, models, evidence_read, messages, outcome | fs, exec, canonical mutation (and per grok: Super stays off RLM until last — its verb set is empty today) |
| CoSuper (restricted) | Super set + assignment/work-item ops | fs, exec |
| CoSuper (effects assignment) | restricted set + capsule_fs (+ exec if granted by the assignment) | — (granted, never inherited) |
| Researcher | context, models, sources, evidence_read, outcome | exec, generic fs |
| Texture | context, models, evidence_read, revision_read, revision_propose, outcome | canonical write |

This also pays a known debt: `capsule/roles.go` grants Researchers `go_eval`
but no runtime registry ever exposed it — manifest and legacy registry are
derived from one profile resolver during parity, then `roles.go` reduces to
coarse process verbs.

**D5 — Tool-loop integration and cutover (high).** Keep `RunToolLoop`; add
exactly one provider-visible tool, `go_cell`; module calls happen *inside*
cells and are not provider tool definitions. Cell output (stdout, errors,
module observations, receipts, variable metadata, typed outcome) enters the
conversation at the existing `ExecuteToolBatch` seam. A narrow hook lets a
typed `outcome.Complete/Park/Refuse/Fail` terminate the run through existing
state transitions instead of trusting free-form text. Claude's flagged
deviation, adopted: the terminal/report tool stays JSON-shaped (its closure
machinery and run acceptance depend on it) — purity is traded for an intact
rollback path. At cutover, ambient tools are removed so there is one
execution idiom; before cutover they coexist only behind the flag.

**D6 — Continuation (high).** Never serialize interpreter state. A versioned
`RLMContinuation` holds: current input occurrence, named artifact/memory refs,
completed cell/module receipt refs, pending assignments, open questions,
remaining budget, outcome intent, state heads. `memory.Freeze(name, value)`
lets the model explicitly persist flat data as an artifact; the continuation
stores the ref. `resumeState` gains a `ContinuationRef`; rewarm creates a
fresh worker and prebinds continuation plus newly validated heads. Run memory
is kept as audit/model-context material; the continuation is the sole RLM
recovery contract.

**D7 — Safety invariants (high, triple-layered).** Canonical-authority
refusals (no Accept/Materialize/Checkpoint/Route) are enforced by module
absence, a host dispatcher denylist, AND a typed-outcome validator (absence of
export alone leaves no receipt). No raw network or tokens: CLONE_NEWNET +
seccomp AF_UNIX-only + sanitized env stay; the token lives host-side only.
No fs outside capability; import ≠ authority (every callback revalidated); no
durable interpreter state; taint propagates through artifact reads; no
unreceipted admitted call; host-side limits on fan-out/depth/tokens/cost/
deadline/process group; stale callbacks rejected by session epoch; manifest
substitution poisons the session. The kernel definition's acceptance suite
covers every refusal under every profile, stale-head races, taint propagation,
callback-after-close, gateway outage, and timeout during a partial `Parallel`.

**D8 — Roadmap.** M1 delete the dead yaegi actor sub-stack + characterization
tests + session protocol skeleton → M2 persistent interpreter + prebound
context + poisoning behavior → M3 callback framing + `ModuleDispatcher` +
`models` + receipt graph → M4 manifest resolver + per-profile export/refusal
suites → M5 flag-gated `go_cell` + parity corpus → **cutover only after
candidate A's gate AND RLM parity both pass** → M6 forced-death /
fresh-interpreter / different-model recovery acceptance, then staging proof
before enabling more profiles.

## 4. The substrate fixes that ride along (not hidden inside RLM)

Prior-consensus repairs, each with its own behavioral contract and rollback,
landed **on the candidate-A path** because forced-death acceptance (M6)
depends on them:

1. **Generation-stamped occurrence identity** (`OccurrenceKey`): the current
   content-hashed actor occurrence ID has no epoch, and the boot migration
   discards the `inserted` flag — a re-emitted recovery occurrence is
   silently deduped, which is the pending-forever P0. Fix is recovery-scoped,
   not global (a global change risks double delivery against the live
   mailbox).
2. **Named predicate family** `Terminal()` / `Active()` / `Replaced()`
   (`Terminal()||passivated`) replacing `!Terminal()`-as-resumable, with the
   selfdev remint predicate explicitly named.
3. **Durable dead-letter for unknown actor kinds**: silent mark-processed
   becomes a receipted rejection plus mark-processed — not a hard error,
   which would poison-loop the FIFO backlog.

## 5. What runs in parallel with candidate A (safe now)

- M1: delete `internal/yaegikernel/{actor_state,broker,broker_protocol,
  profiles,handles}.go` (verified zero production importers; only
  `eval/sidecar/allowlist` are live, via `cmd/capsule-broker`).
- Inert type declarations (session protocol, continuation, manifest).
- The parity fixture harness; predicate-family extraction.
- Wave-1 deletions from the prior consensus (orphan `internal/base` API +
  cmds, unused adapter options, dead stubs) and registry reconciliation.
- Contract/paper design for the candidate-package export/import mechanism.

Must wait behind candidate A: everything touching broker RPC framing, worker
lifecycle, `runtime.go`/tool profiles, and the two delivery-semantics
substrate fixes.

## 6. Risks and their mitigations

1. **The callback framing is the least-proven seam** (medium confidence
   panel-wide): bidirectional frames over the broker change a request/response
   client. Mitigation: dedicated per-session connection, stress tests for
   cancellation + concurrent callback completion, protocol version byte from
   day one.
2. **Yaegi introspection stability**: enumerating top-level symbols may be
   brittle across yaegi versions. Mitigation: focused spike in M2; fallback is
   explicit `observe.Describe(name, value)` instead of interpreter scraping.
3. **Provider-cost blowups via recursion**: mitigated structurally — host-side
   budget/depth/token/cost enforcement before every dispatch, receipts on
   every call including refusals, activation deadline as the hard ceiling.
4. **Parity scope creep**: parity compares observable contracts (completion/
   park states, effects, refusals, receipts, injected updates, cancellation,
   budgets) over a fixed fixture corpus — never prose equality; shadow
   execution only against disposable read-only fixtures.
5. **Doing RLM work on the candidate-A path**: forbidden by sequencing; the
   flag defaults to `tools`, the one-shot eval path is unchanged, and rollback
   is a flag flip.

## 7. Verdict

The panel endorses building the real RLM on the machinery Choir already has —
the signed capsule broker, the process-group containment, the receipt path,
the gateway bridge, `RunToolLoop`, and actor park/rewarm — extended in three
seams (session protocol, prebinding, module callbacks) and simplified by one
deletion (the dead yaegi actor duplicate). No new sockets, no new trust
boundaries, no second activation loop. The implementation is orange/red
(model routing, capability checks, receipts, recovery identity are protected
surfaces), gated twice: candidate A first, parity second, with the memo's
forced-death/different-model recovery as the final acceptance before staging.
