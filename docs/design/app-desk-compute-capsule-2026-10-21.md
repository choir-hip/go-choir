# A Compute Capsule Kind for App Desks (SDC ARCH-09)

Date: 2026-10-21. Station:
[choir-launch-fp10-arch-09-2026-10-21.md](../definitions/choir-launch-fp10-arch-09-2026-10-21.md).
Problem: [app-desks-have-no-compute-chamber-2026-10-21.md](../problems/app-desks-have-no-compute-chamber-2026-10-21.md).
Mutation class: red (guest desk confinement, capsule spawn); S0 is a pure
package with no caller.

## 1. What it is

A **compute capsule** is a disposable guest-local capsule that an app desk
opens to turn content-addressed inputs into content-addressed output
artifacts by running one recipe its app package declared. It is the
artifact-producing sibling of the engineering capsule:

| | engineering capsule | compute capsule |
|---|---|---|
| opened by | the engineering assignment saga, after management admits a cast | an app desk's compute request |
| mounts | the preflighted platform source subject at `/workspace/platform` | its inputs read-only at `/in/<name>`, an empty `/out`; no source |
| runs | model-authored cells, exec, jobs (broker verbs) | one recipe job; no session, no cell, no exec verb |
| leaves through | Freeze: a frozen effect bundle, verify, accept, materialize | output artifacts in the asset store and one compute receipt |
| route or release effect | through self-development only | never |
| egress | per binding (proxy, Claude) | none (until NET-10 grants) |
| slot | the one live engineering assignment (ARCH-10) | its own pool; never the engineering slot |

It is not an executor VM (ARCH-11: a VM per tenant and taint class, on
another host later) and not the doctool sandbox (FP2 SEC-15: one parsing
tool, text out through a pipe, in-process for the extractor). Convergence:
attachment-to-text for E5 uses doctool; a compute capsule is for jobs that
write files, need a browser or a media tool, or run for minutes: slides and
PDF export, video render, a viewport capture an app desk wants as an
artifact rather than as self-development evidence.

## 2. Authority

- **Who may open one.** An app desk only (I26). System desks do not: engineering
  has its capsule, research is read-only, management mutates nothing. Texture
  is an app desk and may.
- **What runs.** A recipe from the requesting desk's own app package: a name,
  an argv template whose `argv[0]` is an absolute tool path the package
  declares, input slots, output media types and bounds, memory and wall-clock
  needs. The desk names the recipe and binds inputs to slots. It never writes
  argv, a shell string, an environment or a path (I19). Placeholders are
  `{in:<slot>}` and `{out}` only.
- **What it reads.** Artifacts the desk may already read, by `sha256:` ref,
  copied into the capsule read-only. The desk's handles, sockets, tokens and
  the tape never enter it (I19).
- **What it writes.** Files in `/out`, checked at close against the recipe
  (count, total bytes, media types, flat names), stored by content address.
  The desk receives refs and a receipt; making them objects (a capture, a
  revision, a media item) is a separate semantic act by the desk that owns
  the kind (Texture alone writes revisions).
- **What it cannot do.** Freeze, verify, accept, adopt, carry a patch,
  reach the network, reach the delegate, or name a capsule another desk owns.

## 3. Lifecycle and durability

A **compute request** is a durable object on the tape:
`requested -> running -> done | failed | refused | interrupted`. Its id
derives the capsule id, so a spawn retry finds its own capsule. Close always
destroys the capsule and reclaims its cgroup (the existing teardown reclaim).

- A crash restart closes every open compute request as **interrupted by a
  restart**, never re-dispatched (the capsule-job rule, `internal/capsule/jobs.go:424`).
- A planned restart also closes them (no resume in the first cut): a render is
  cheap to ask again, and a resumer is a boot-time risk for no gain. The desk
  sees the fate and may request again as new work.
- Compute capsules do not hold back a planned refresh of an always-busy
  computer. The CLAUDE.md rule "never restarted while an engineering capsule
  is live" stays about engineering capsules (owner question 1).

## 4. Resources

- Memory per doctrine: `memory.high` = the recipe's request, `memory.max` =
  2x, `memory.events` receipts, admitted against the capsules aggregate
  (FP2 RES-04, `CHOIR_CAPSULE_MEMORY_POLICY`). The request is bounded by
  the app memory cap of the requesting desk's floor.
- A per-computer cap on live compute capsules, one at first, FIFO with a
  retryable refusal (the ARCH-10 launch split), separate from the
  engineering slot.
- Wall clock per recipe, bounded; on expiry the request fails.
- Spending is tracked, never capped (DEC-07); a budget slice under ARCH-08
  applies later.

## 5. Containment

The capsule executor's existing chamber (overlay root from the lower image,
cgroup, namespaces, seccomp, Landlock, the capsule uid), with the kind
`compute` selecting: no source subject, no `/selfdev/*` mounts, no broker
session verbs, no egress proxy, the recipe's tool roots read and execute,
`/in` read, `/out` write. Inputs are untrusted (taint class
`untrusted_input`, the app desk floor): a malformed PDF crashes only its
capsule (E5's acceptance).

## 6. Failure modes (written before code)

S0 (pure, `internal/computecapsule`) tests C1 to C12. The rest belong to the
slices that wire them.

| # | Failure | Expected | Slice |
|---|---|---|---|
| C1 | a system desk (management, engineering, research) or an empty desk asks | refused `desk_not_app` | S0 |
| C2 | a desk names a recipe of another package | refused `recipe_not_owned` | S0 |
| C3 | the recipe's `argv[0]` is relative, not among its declared tools, or a shell or `env` | refused `recipe_invalid` | S0 |
| C4 | the template holds an unknown placeholder, or text that splices a placeholder into a larger argument | refused `recipe_invalid` | S0 |
| C5 | an input ref is not `sha256:<64 hex>`, a slot is unbound or unknown, or bound twice | refused `input_invalid` | S0 |
| C6 | too many inputs, or total input bytes over the cap | refused `input_invalid` | S0 |
| C7 | memory over the app cap or wall clock over the maximum (recipe or request) | refused `resources` | S0 |
| C8 | egress asked | refused `egress` (until NET-10) | S0 |
| C9 | any engineering input set (source ref or digest, verifier bundle or binding, adoption, carried patch, egress, delegate) on a compute spawn | refused `engineering_input` | S0 (pure), S1 (executor) |
| C10 | an output name with a slash, `..`, a leading dot, or a duplicate | refused `output_invalid` | S0 |
| C11 | too many outputs, total bytes over the recipe cap, or a media type outside the recipe's set | refused `output_invalid` | S0 |
| C12 | the same outputs in another order | the same manifest digest | S0 |
| C13 | Freeze or verify named on a compute capsule | refused before any staging | S1 |
| C14 | a crash with a request running | interrupted at boot, capsule reclaimed, no re-dispatch | S2 |
| C15 | a second request while the pool is full | retryable refusal, the first untouched | S2 |
| C16 | the job writes outside `/out` or reads outside its roots | denied by Landlock, recorded | S1 |
| C17 | the job exceeds `memory.max` | the capsule dies, request failed with the OOM count | S1 |
| C18 | the desk worker passes argv, env or a path in its request | not representable in the verb; unknown fields refused | S3 |
| C19 | `CHOIR_COMPUTE_CAPSULES` unset or unknown | no compute capsule opens (fails closed) | S2 |

## 7. Slices and lane order

| Slice | What | Touches | After |
|---|---|---|---|
| **S0** (this station) | `internal/computecapsule`: Request, Recipe, Validate, Argv, RefuseEngineeringInputs, CheckOutputs, Manifest digest | a new package only | nothing |
| S1 | `capsule.SpawnSpec.Kind`; executor spawns `compute` with no source and calls RefuseEngineeringInputs; Freeze and verify refuse `compute`; Landlock roots for `/in`, `/out` | `internal/capsule` | FP2 batch 2 lands (it edits `types.go`, `executor.go`, `roles.go`) |
| S2 | compute request store and runtime; boot closure; pool cap; `CHOIR_COMPUTE_CAPSULES` off by default | `internal/store`, `internal/agentcore` | S1 |
| S3 | the app manifest declares recipes; a desk verb `choir.Compute(recipe, inputs)` returns a request id, read through Poll | `internal/appdesk`, `internal/deskmanifest`, `internal/yaegikernel` | ARCH-03 lands |
| S4 | the first recipe and its tool in the guest image; the deployed proof | `nix/autoputer-vm.nix` | FP8 lands |

## 8. Owner questions (defaults stand unless ruled)

1. Compute capsules are interrupted by a planned refresh of an always-busy
   computer, rather than holding it back like an engineering capsule.
   Default: yes.
2. The first recipe: a PDF render of a Texture revision (email §6.5, Slides
   export) rather than video. Default: PDF render.
3. One live compute capsule per computer at first. Default: one.

## 9. Deltas

Conjecture delta: artifact work an app desk needs is a recipe over
content-addressed inputs in its own chamber, not a borrowed engineering
capsule; the engineering capsule keeps the only path to the platform.
Heresy delta: discovered: an app desk has no contained place to run a
converter, so the visible paths are an engineering grant or exec in the
desk worker (the problem record). Introduced: none. Repaired: none yet (S0
is the contract only).
