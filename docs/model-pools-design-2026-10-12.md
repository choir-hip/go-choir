# Model pools: design

Owner direction, 2026-10-12. Every desk runs on a mix of two models,
opencode-go `deepseek-v4.1-flash` (lowest cost, good quality) and chatgpt
`gpt-6-luna`, so each builds a sample size. Each model falls back to the
other and to nothing else. The pool grows past two later. Prompts are not
tuned to any one model. Old models go: gpt-5.4, gpt-5.4-mini, gpt-5.5,
and gpt-5.6-luna/sol/terra (gpt-6-luna replaces gpt-5.6-luna).

The owner's model of it: "a particular execution thread would be with a
specific model, so we're getting KV caching economics. But if that model
does fail in the middle of its trajectory, we can still fall over to
another model in the pool. We break the cache, but we still get to
continue execution and deliver the result. Inside the guest, they can
have their own routing logic at a level of specificity finer than the
pool. We can deal with that at a later time."

Mutation class: red (provider and gateway calls, model routing).
Rollback: git revert; computers re-derive their policy at boot.

## What exists today

From a read-only study of the code at bbc43808. Spot-checked lines are
marked ✓.

- **The gateway trusts the model name it is given.** The guest sends
  provider and model on every call (internal/gateway/handlers.go:38-44).
  When the provider is named, the model is not checked
  (handlers.go:527-535). ChatGPT passes any model string through
  (provider.go:836). OpenCode refuses anything outside a wire-shape
  switch (provider.go:1189-1199 ✓), the only real allowlist. The model
  lists in cmd/gateway/main.go:144-154 and nix/node-b.nix:905 only seed
  one model per credential. The models endpoint is a hardcoded list of
  three old models (openai_compat.go:191).
- **The catalog is a Go literal** (internal/modelcatalog/catalog.go:43-189).
  It still lists the gpt-5.x models, and gpt-6-luna has no modalities or
  context window.
- **One model per role per computer.** System/model-policy.toml maps each
  role to one selection, with no pools. It is written only if missing
  (model_policy.go:385-396), so existing computers keep gpt-5.6-luna
  forever. The VM fallback env is gpt-5.6-luna (nix/autoputer-vm.nix:875-877).
- **One model per run, no fallback.** EnrichMetadata resolves once per run
  (model_policy.go:119-148). toolloop's activeLLMConfig is never
  reassigned (toolloop.go:340). Retries cover only 429s
  (toolloop.go:1441-1467). ProviderPreconditionFallbackSelections returns
  nil and nothing calls it (model_policy.go:235-240).
- **Only research children choose a model.** choir.Research takes
  {objective, model}, and the model must already be one the computer's
  policy names (research_children.go:270-301 ✓). A child inherits the
  parent's reasoning effort and passes no conversation id, and OpenCode
  refuses a call without one (provider.go:1042-1044 ✓). So a child
  resolved to deepseek would fail (an inference, not yet seen in a trace).
  spawn_agent's `model` argument is overwritten by the policy model
  (runtime.go:1255-1257 ✓). There is no per-document model.
- **The choice is half recorded.** provider_call and tool_loop events
  carry the model (toolloop.go:497-499, 611-617). Texture's
  AuthoringModel and CommitmentProvenance.ModelID exist but are not
  filled, and children's events are discarded.
- **Mismatches:**
  - OpenCode sends no reasoning setting, so `reasoning = "high"` on
    deepseek does nothing (provider.go:1134-1170).
  - Circuit breakers are per provider, not per model (circuit_breaker.go:15).
  - The ChatGPT search default is gpt-5.5 (search_chatgpt.go:22).
- **A live constraint:** the ChatGPT account behind the review panel hit
  its usage limit on 2026-10-12. If the gateway uses the same kind of
  subscription, gpt-6-luna will hit limits too. Failover is not
  hypothetical.

## The design

**1. The gateway holds the catalog and the pools.** modelcatalog becomes
the one host catalog. Each entry has: provider, model, wire shape,
capabilities (tools, image), context window, whether it takes a
reasoning setting, request quirks (for example, Codex rejects an output
budget), and a status of active or retired. A pool is a named, ordered
list of catalog entries. Today there is one:

```
default = [opencode-go/deepseek-v4.1-flash, chatgpt/gpt-6-luna]
```

The gateway refuses a (provider, model) pair that is not active in the
catalog. `GET /provider/v1/models` returns the catalog and the pools, so
the guest learns what exists from the host. The parallel model lists in
cmd/gateway/main.go, the nix env and the OpenCode switch are derived from
the catalog or deleted.

**2. A model spec is a pool or a model.** It is a string: `pool:default`
or `provider/model`. Any unit of work may carry one. The most specific
wins:

1. an explicit choice: a parent naming a sub-RLM's model, or a desk
   switching its own;
2. the thread's current model (below);
3. the guest policy for the role (today every role says `pool:default`);
4. the gateway's default pool.

**3. A thread keeps one model.** A thread is the unit that benefits from
one cache:

- a desk's persistent session;
- a Texture document's turns;
- a sub-RLM run.

A thread that names a pool is assigned a member by a stable hash of its
id, `pool[hash(thread_id) mod n]`. The assignment is reproducible, is
balanced across many threads, survives restarts and needs no stored
state. Different Textures on one computer therefore land on different
models, and so do different desks. Only overrides are stored: a desk's
explicit switch, an explicit choice, or a failover. They are stored as a
field on the agent record or the document, and as a recorded event.

**4. A desk can choose its own model.** One verb, `choir.Model(spec)`,
sets the desk's model for its next turns (or, from a Texture, the
document's). choir.Research's `model` and a Cast's optional model go
through the same resolver (spawn_agent is deleted in Cut 1, decision 6b:
sub-RLMs are the one child primitive, so M0 does not plumb its dead
`model` argument), validated against the gateway catalog rather than
against the guest policy's role list. A child gets its own conversation
id and the reasoning effort of its own model, not its parent's.

**5. Failover is the next pool member, for the rest of the thread.** The
place is the provider call in the tool loop (CallToolLoopProviderWithRetries).
After the existing retries, a failover-class error moves the thread to the
next member of its pool, in pool order from the failed one, and it stays
there. Failover-class errors are:

- 429 still failing after its backoff;
- 5xx;
- a provider timeout;
- an unsupported model;
- an open circuit.

A context cancel or a request the gateway rejects as malformed is not
failover-class.

On a switch:

- `activeLLMConfig` changes;
- provider-specific replay items (ChatGPT reasoning items, OpenCode
  reasoning content) are dropped from the message history;
- a fresh conversation id is used;
- a `model_failover` event records thread, from, to and reason.

A pool of two means each model falls back to the other. A thread whose
last member fails too ends with the error, as today. Failover is never
attempted twice in one call. (Amended 2026-10-14: with three members, "at
most once" stranded a deepseek thread on cat-jam behind the same open
circuit. Failover now walks the remaining members forward, each at most
once, and never goes back; see
docs/archive/problems/free-model-429-opens-provider-circuit-stub-stories-2026-10-14.md.)

**6. Every choice is recorded.**

- Run metadata gains `llm_spec` (what was asked for) and
  `llm_assignment` (hash, explicit or failover), beside the existing
  `llm_provider` and `llm_model`.
- Texture AuthoringModel and CommitmentProvenance.ModelID are filled.
- Research children's events are kept. Cut 1 makes children runs, so
  this lands with it.

This is what makes per-model comparison possible later.

**7. The guest policy shrinks.** System/model-policy.toml maps each role
to a spec and a reasoning effort, with `pool:default` everywhere. Boot
regenerates it from the platform default. Nothing on a computer is
preserved before Autopaper is live (decisions §13.1), so an existing file
is replaced; a file that carries an owner marker is kept. Overlays stay.
Routing in the guest finer than the pool comes later.

**8. Retirements.** gpt-5.4, gpt-5.4-mini, gpt-5.5 and gpt-5.6-luna/sol/terra
leave:

- the catalog;
- the gateway lists and the nix env;
- the openai-compat defaults and models endpoint;
- the Zot terminal default;
- the tests that pin them.

ChatGPT search moves to gpt-6-luna. Jev is unchanged. The other active
OpenCode models (glm-5.3-flash, muse-spark, hy3, qwen3.7-max) stay in
the catalog but outside the default pool.

## First cut: simplifications

- The catalog and pools are a Go literal in internal/modelcatalog, which
  is compiled into both the gateway and the guest. The gateway validates
  against it and serves it at `GET /provider/v1/models`. The guest
  resolves pools from its own compiled copy. Having the guest fetch the
  pools from the gateway comes later, when pools change without a deploy.
- The thread key is the Texture agent id (`texture:<doc>`) for a Texture
  run, so a document keeps one model. For every other run it is the
  trajectory id, so management, research and engineering mix across
  stories. For runs today, the provider conversation and its cache are
  per run anyway (the tool loop's conversation id is the run id).
- Failover lasts for the rest of the run. The thread's next run hashes
  back to its assigned model, which may have recovered. Nothing is
  stored.
- `choir.Model` (a desk switching its own model) is a second slice, after
  pools, failover and recording are live on staging.

## Ways it can fail, written before the code

- **Skewed assignment.** Assignment by hash sends every thread to one
  member. Test: many distinct keys reach both members, and one key is
  always stable.
- **Pool spec leaks.** A pool spec reaches the gateway as a model name
  (`pool:default`). Resolution happens in EnrichMetadata, before any run
  starts; the gateway refuses non-catalog models, so a leak fails loudly.
- **Unversioned policy file.** An existing computer keeps an old
  policy file naming gpt-5.6-luna. The gateway now refuses it, so every
  run on that computer fails. Mitigation: at load, a file without the
  current platform-version line is rewritten; a role naming a retired
  model is re-resolved to the default pool rather than failing.
- **Failover on the wrong errors.** A cancelled context, or our own
  malformed request, triggers failover and doubles the damage. Only the
  listed classes fail over, and a test covers a cancel.
- **Failover loops.** Failover visits each member at most once per run
  and never returns to a member it left (amended 2026-10-14).
- **Replay poisoning.** Replaying one provider's reasoning items into the
  other provider breaks the request. On a switch, reasoning content is
  dropped from the history and a fresh conversation id is used.
- **Unknown-provider refusals.** Gateway validation refuses a provider
  the catalog does not know (fireworks, for example). Validation applies
  only to providers that have catalog entries.
- **Hidden gpt-5.x callers.** Zot, the openai-compat clients and
  search used gpt-5.5 by default. Each default moves to gpt-6-luna, and
  the models endpoint lists only catalog ids.
- **Unsupported model on a live provider.** gpt-6-luna is not enabled on
  the gateway's ChatGPT account. The first staging run shows it, and
  failover to deepseek keeps the run alive while it is fixed.

## Not now

- routing in the guest finer than a pool (weights, per-task rules)
- per-model circuit breakers and health-aware sampling
- pricing in llmcost (its table only knows gpt-4-era models)
- a reasoning setting for OpenCode models

## Acceptance (staging)

- A fresh test computer's desks and two new Textures run on both models:
  the run metadata shows each assignment, and repeated threads keep one
  model.
- A forced failure of one model mid-thread (a test hook that makes the
  gateway refuse that model for one thread) produces a `model_failover`
  event, and the thread finishes its turn on the other model.
- A research child named `opencode-go/deepseek-v4.1-flash` succeeds.
- `GET /provider/v1/models` lists the catalog with no gpt-5.x entries,
  and the gateway refuses a retired model.
- `git grep` finds no gpt-5.4, gpt-5.5 or gpt-5.6 in non-test Go, nix or
  scripts.

## Acceptance result (2026-10-12)

Code b0d69d4d, CI 38312172697 green, staging health and the test
computer's guest build both b0d69d4d. Probe `scripts/m0_pools_acceptance.mjs`
on a fresh test computer (computer-d57369c008d75ec0d3069dd4efe5a97b),
artifact docs/archive/evidence/m0-model-pools-acceptance-2026-10-12.json: PASS.
Four documents authored in 24-35 s; 9 model-bearing runs, every one
recording spec and hash assignment, every model a pool member; each of 9
threads kept one model; both members used (gpt-6-luna on 4 threads,
deepseek-v4.1-flash on 5); no catalog refusals. The first run of the
probe reported 0/3 documents because it read the revision list in the
wrong order (newest first); the documents had 3, 6 and 6 desk revisions,
and the probe was fixed. Gateway journal: both members carry traffic; one
chatgpt 429 at 21:53:02.

Carried, not proven on staging:

- the `model_failover` event: run events are not owner-readable until
  Cut 0's trace export; unit tests are the shape evidence until then;
- `GET /provider/v1/models` needs computer credentials; unit tested only;
- a research child named explicitly by model (children were assigned by
  hash; the explicit path is unit tested).

Residual: Node B's hand-written gateway env still lists gpt-5.5, gpt-5.4
and gpt-5.4-mini in GATEWAY_CHATGPT_MODELS. The catalog refuses them, so
the line is stale but harmless; rerunning nix/deploy-provider-creds.sh
(an owner host step) rewrites it.

## Where it sits

This lands before Cut 0, as station M0. Cut 0's baselines are then
measured on the models the cuts will run on. Cut 1 inherits the thread
assignment for its records and conversations.
