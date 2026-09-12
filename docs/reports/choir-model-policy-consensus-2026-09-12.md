# Model-policy design — agentic consensus synthesis

Panel run: `.agentic-consensus/model-policy-design/run1` (prompt, manifest,
per-agent outputs are session diagnostics; this file is the durable record).
Mode: **divergent** with 13 lenses, so the option space was generated before any
verdict. Requested by the owner 2026-09-12: design the move of model policy off
gateway/guest config files and into runtime (yaegi) selection, under the hard
requirement that **changing or adding a model never requires a deploy**.

Final panel health (the run finished after the first synthesis): 13 agents
selected, **10 `ok`**, `omp-hy3` and `omp-gemini38` failed in 20s/30s (the same
recurring quota pair as this mission's earlier panels), and `opencode` timed out
at its 1200s deadline having emitted only its banner. `opencode` is configured
with the same model as `omp-muse-spark`, so its loss removed no coverage —
but note it means the "13-agent" default panel reliably yields about ten usable
opinions, and the two quota failures are predictable and should be replaced or
pre-excluded at the next boundary. Panel health is metadata, not evidence. Everything below marked *verified*
was checked in the tree during synthesis; anything else is a panel claim.

## 1. Option families

Eight distinguishable ownership designs recur across panelists; the same
families appear under different names (muse-spark 8, sol 8, nemotron 6,
grok46 11, claude 11). Clustered, with the sharpest variant in each:

1. **Computer-TOML sovereign** (harden status quo; muse-spark O1, sol O1,
   nemotron A). Sharpest: keep the file, add live reload and a version stamp.
2. **In-cell (yaegi) sovereign** (muse-spark O2, sol O2/O6, nemotron E, claude
   O3/O9) — the owner's stated direction. Sharpest (sol O6): **select
   capability aliases, never concrete model IDs, inside Go.**
3. **Host-side policy service sovereign** (muse-spark O3, sol O3, nemotron B):
   versioned, atomically reloadable host data; the computer holds only
   constraints.
4. **Gateway-side dynamic routing sovereign** (muse-spark O4, sol O4,
   nemotron C): the gateway becomes the policy authority, not just credential
   holder and forwarder.
5. **Three-way split with one resolver** (muse-spark O5, sol O5, nemotron D,
   devin, claude): entitlement/credentials, policy, and run intent as separate
   fact owners; **entitlement may only admit or refuse, never choose a
   replacement**, and one deterministic resolver evaluates the ordered
   candidates. Claude's formulation: *policy orders, entitlement subtracts, run
   intent narrows.*
6. **Capability negotiation** (muse-spark O6): the desk declares needs, the host
   bids; contradicted by sol's current-turn paradox (below).
7. **Content-addressed versioned policy artifact + registry** (muse-spark O7,
   sol O7): policy is an immutable artifact with a reloadable pointer.
8. **Bring mission 6 forward** (muse-spark O8): make management the sovereign
   because it owns host-side activation anyway.

Claude's cross-cutting variants worth keeping: **entitlement as a capability
minted into the session handle** (mirroring `co_super_slot`, which "the model
can never set", `internal/yaegikernel/choir.go:27-30`), **delete the overlays in
favour of admission-checked run intent**, and **lift the search plane's provider
health substrate** instead of building a second one.

### Dimensions the families genuinely differ on (grok46)

Single writer of the serving `(provider, model)`; when selection binds
(assign/open, first cell, every inference, actor epoch); what a "model change"
is (string in a file, string in a call, class remap, entitlement remap); where
entitlement sits relative to policy (filter, second authority, absent); what
stays compiled (adapter only vs catalog vs role defaults vs heuristics); guest
vs host knowledge; silence vs refusal on an empty join; and the I5 deletion
clock for overlays, fallbacks, dual `SupportedModels` and gateway heuristics.

## 2. Consensus (2+ agents, and all locally verified where marked)

**Q3 — what stays compiled.** Keep compiled: protocol adapters and wire
transformation, secret injection and credential isolation, authz/computer
scoping, the resolver algorithm and its precedence, receipt schemas, refusal
taxonomy, and conservative behaviour when capability data is absent. Move to
live versioned data: provider instance name **and adapter kind**, endpoint
reference, credential *handle* reference, model id and display name,
provider→model availability, modalities, context/output limits, capability flags
(tools, reasoning, streaming, session affinity), pricing dimensions, entitlement
and quota, deprecation state, and (non-authoritative) role hints. Placement
(consensus of sol/glm53/nemotron): platform registry (versioned, atomically
reloadable) + host secret/entitlement store + computer policy + **run ledger
holding the effective binding.** A genuinely new *wire protocol* still needs
code; a new *model* on an existing adapter family must not.

**Q4 — the residue.** Unanimous: **do not touch the V1 decode seam** in mission
3. It is a historical decoder preserving immutable unversioned computer-owned
bytes (`decodeRoleSection`, `model_policy.go:193-206`; introduced by mission 2
in `cb571960`), it decoded correctly — `[roles.co-super]` → `engineering` — and
the *value* was the defect. Removing it before stored policies are version-marked
risks restore/replay compatibility. Near-unanimous: **the unfunded values are
active harm and get fixed now**, not in the post-11 pass; the V1 seam gets a
named residue with a stronger revisit trigger (before provisioning another fresh
computer, or when the model-policy redesign lands).

**Q5 — the phantom caller.** Unanimous: **in scope for mission 3**, because it
blocks the mission's own roster evidence; the *surface* redesign still belongs to
mission 6. Smallest correct repair, in every panelist's words: apply the rule the
repo already tests — **a lifecycle activation written `state: running` without
execution admission must not satisfy `Active()` residency** (the rule encoded by
`TestCoagentRewarmUsesResidentActivationNotActiveRunProxy`), rather than
special-casing the one caller id, forcing the row passivated, or adding a second
Texture-only wake path. Recover the three stranded instructions through the
repaired deterministic path instead of minting replacements with new identities.
Patch with an explicit deletion slate for mission 6, so it is not load-bearing.

## 3. Dissent / disagreements

- **`defaultPolicyText` now or later.** glm53 and nemotron: fix the compiled
  defaults now — they poison every fresh computer and are orange-class config.
  sol and grok46: *do not* use a code/defaults change as the roster unblock — it
  requires a deploy, leaves the retained computer's bytes untouched, and
  reinforces the build-artifact mechanism the redesign retires. Resolution I
  recommend: the **live** fix now (no deploy), the **generated default** fixed
  with the redesign, unless a fresh computer may be provisioned before then.
- **How to change the live computer's policy.** sol requires CAS discipline:
  record prior bytes and digest, use the accepted per-run base-policy swap with a
  lease, record the effective provider/model/source before opening the run,
  restore through a crash-durable obligation (or keep the funded policy if the
  owner declares it the new computer policy), and refuse concurrent arms during
  the swap. Others (grok46, glm53, claude) accept a direct live-file write
  because the loader re-reads per resolve. sol's discipline is strictly safer and
  costs little.
- **Whether passivating the phantom alone unsticks the queue.** grok46 flags a
  second stall: the persistent Super drainer completed, so a repaired Texture
  caller may still have no drainer. Unresolved.
- **Duplication risk in the panel itself**: `opencode`'s configured model is the
  same `muse-spark-1.3-contributor-free` as `omp-muse-spark`.

## 4. Unique high-value findings

- **A funded cross-provider fallback ladder already exists, is wired, and
  should have rescued the dead arms** (claude). *Verified:* 
  `modelpolicy.ProviderPreconditionFallbackSelections`
  (`model_policy.go:234-250`) builds `flashPreconditionFallbackSelections`
  (`:630-651`) and appends the terminal chatgpt fallback; wired at
  `internal/agentcore/runtime.go:3372,3391` into
  `toolregistry.WithProviderPreconditionFallbacks` (`toolloop.go:159`), consumed
  at `toolloop.go:559-577`. *Verified:* the deepseek branch keys on
  `defaultConductorModel = "deepseek-v4-flash"` — exactly the model the dead arms
  resolved — yielding `xiaomi/mimo-v2.5`, then the chatgpt terminal.
- **The trigger is a string match on sanitized error text.** *Verified:*
  `isProviderAvailabilityError` (`toolloop.go:1264-1271`) matches
  `strings.Contains(text, "402") || contains("payment required")`, while the
  gateway rewrites provider failures to **502** (`handlers.go:420-423`,
  `writeGatewayJSON(w, http.StatusBadGateway, …)`) and the guest client
  **retries 502/503/504** (`internal/gateway/client.go:126`). So the fallback's
  preconditions depend on whether a sanitized body happens to retain a status
  string, with a retry layer in front of it. The gateway log for the dead arm
  shows deepseek at 19:06:22 followed by `xiaomi/mimo-v2.5` at 19:06:59 and
  chatgpt at 19:07:01 — the ladder's exact sequence — so "the ladder never fired"
  is **not** established; the arm's stall has to be explained by what happened
  after recovery.
- **Model strings are compiled in more than one place that disagrees**
  (claude). *Verified:* `internal/modelcatalog/catalog.go` carries
  `deepseek-v4.1-flash`→`opencode-go`, `glm-5.3-flash`→`opencode-go`,
  `muse-spark-1.3-contributor(-free)`→`opencode-go`/`opencode-zen`,
  `gpt-5.6-luna`; while `nix/deploy-provider-creds.sh:45-50` seeds
  `DEFAULT_GATEWAY_ZAI_MODELS="glm-5.2,glm-5.1,glm-5-turbo"` — which does **not**
  contain the roster's `glm-5.3-flash` — plus `cmd/gateway/main.go:128-160`
  ("Model selection is a runtime concern resolved here at the gateway entry
  point") and a third list in `openai_compat.go`.
- **Gateway routing falls back to model-id prefix heuristics** (`handlers.go:540-548`:
  `strings.Contains(model, "fireworks")`, `strings.HasPrefix(model, "deepseek-")`
  → the deepseek provider). *Verified by reading the code.* A config-only model
  addition would inherit this implicit authority.
- **The current-turn paradox** (sol): Go code is authored by a model that was
  already selected, so in-cell selection can govern only a *subsequent* turn,
  continuation, or child activation. Any API implying it selects the current
  model is false. This directly constrains the owner's yaegi direction.
- **Reload theatre / operator-only theatre** (sol): a data catalog is not runtime
  policy if changing it needs a Nix rebuild, a gateway restart, or a privileged
  host edit with no product path.
- **Unknown models must refuse for capability-sensitive calls** (sol, glm53):
  `MaxOutputTokensForModel`/`ContextWindowTokensForModel` currently return generic
  defaults for unknown ids (`catalog.go:251-274`), so a missing fact is silently
  treated as a known one.
- **`llmcost` pricing covers none of the 26 catalog models** (claude) — *not
  locally verified* — which would make spend limits unimplementable as specified.

## 5. Low-confidence / unverified

- `internal/searchplane/policy.go` and an `OutcomeQuotaLimited` enum do **not
  exist**; only `internal/gateway/search.go:160` `AvailableProviders()` and
  `IsAvailable()` (`:245,336,426`) exist. The "lift the search-plane substrate"
  option rests on a weaker version of that claim than stated.
- The claim that the ladder failed to fire is contradicted by the gateway
  sequence above; treat the 402-loop narrative as needing re-derivation from run
  events.
- Whether an owner/product no-SSH write path to `System/model-policy.toml` exists
  (claude's Q4 caveat): *verified* that `choir files get|put <path>` exists as
  the product path, and that `modelpolicy.Load` re-reads the file per resolve
  (`:90-116`), so a write takes effect without a restart.
- `llmcost` coverage, and the exact admission primitive to reuse for Q5.

## 6. Recommendation

1. **Adopt the three-way split with one resolver** (family 5), which is the
   plurality position and the only one that satisfies `I5` while giving the
   desk runtime say: entitlement/credentials (host; admit/refuse only), policy
   (ordered candidates per role/desk, versioned reloadable data), run intent
   (what this run asked for, recorded in the run ledger as the effective
   binding). One deterministic resolver; entitlement never silently substitutes.
2. **Honour the current-turn paradox** in the yaegi design: in-cell selection
   stages a *durable intent for the next activation*, not a retroactive choice,
   and receipts record requested-versus-effective because aliases and vendor
   routing can differ.
3. **Move model facts to versioned reloadable host data** (family 7 shape:
   content-addressed artifact + reloadable pointer), keeping adapters, secret
   injection, authz, the resolver and the refusal taxonomy compiled. New provider
   on an existing adapter family = config; new wire protocol = code.
4. **Kill the implicit authorities** on a named clock: the gateway's prefix
   heuristics, the duplicated nix/modelcatalog model lists, the overlays as a
   per-arm mechanism, and the string-matched fallback trigger (replace with a
   typed entitlement/quota class so a fallback is driven by a class, not a
   substring).
5. **Do now, in mission 3**: (a) repair the caller-admission rule (Q5) with the
   existing `Active()`-requires-admission semantics and a deletion slate for
   mission 6; (b) change the retained computer's `[roles.engineering]`/
   `[roles.verifier]` to funded models through the product file path with prior
   digest recorded, a lease, and a durable restore obligation; (c) register the
   V1 seam, the generated unfunded defaults, and the duplicated compiled lists as
   residues with explicit revisit triggers. **Defer**: the V1 seam itself, the
   `defaultPolicyText` change, the catalog-as-data inversion, `llmcost` coverage.

## 7. Raw outputs

`.agentic-consensus/model-policy-design/run1/` — `prompt.md`, `manifest.tsv`,
`<agent>.out`, `<agent>.cmd`. Non-durable session diagnostics; this synthesis and
the option families above are the durable record.
