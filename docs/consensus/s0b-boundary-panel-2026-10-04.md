# S0b Boundary Panel — Convergent Adjudication (2026-10-04)

**Scope adjudicated:** S0b disposable-computer probe suite on
`computer-ac1808b4f04fc749c0781083ee747403` — whether it closes the
S0→S2 boundary for the supervised-app-development metamission.

**Mode:** convergent. **Panel:** 11 agents, all OK
(`/tmp/s0b-panel/manifest.tsv`, archived at
`docs/consensus/s0b-boundary-panel-2026-10-04-raw/`).
Anchor probe: all 7 OMP pins served their exact requested models
(agentic-consensus probe, 2026-10-04). Codex verified the keydriver fix;
Devin verified the M9a root cause in source; five others worked in-repo;
claude/opus, gpt6-luna, space-bunny, qwen38max worked read-only.

## Verdicts (raw, per agent)

| agent | verdict |
| --- | --- |
| codex | accept_with_edge |
| omp-gemini38 | accept_with_edge |
| omp-muse-spark | accept_with_edge |
| omp-qwen38max | accept_with_edge |
| opencode | accept_with_edge |
| omp-gpt61-sol (flagship) | send_back |
| omp-gpt6-luna | send_back |
| omp-glm53-flash | send_back |
| omp-space-bunny | send_back |
| claude (opus) | send_back |
| devin | incomplete — its tool calls were rejected by its own sandbox; the workflow evidence is partial and stands below |

**Synthesized verdict: `accept_with_edge`** — accept the S0b probe suite as
the boundary evidence, name the accepted edges, and discharge two
station-aggregation debts inside the close record. 5×accept_with_edge vs
5×send_back is a real split, not an averaging accident; the send-back camp's
sharpest objections are correct as *facts* but do not survive the
cost/benefit test at this boundary (below).

## Adjudicated findings (each verified against the repo before inclusion)

1. **M9a route-projection defect: confirmed at source, S1a-introduced,
   does not block S0→S2 — but its ownership must be S1a, not "S2/S3".**
   Verified chain: `internal/routeledger/ledger.go:106-117`
   (`ParseRouteSlotID` returns the second component as `computerID`, which
   for canonical slots is the *desktop* id), `internal/vmctl/route_authority.go:398-406`
   passes it to `bindRequestToGuestComputer`, `internal/vmctl/handlers.go:1516-1533`
   calls `GetOwnershipByComputerID("primary")` → nil → 403.
   `git log -L` attributes the binding leg to `525f282b red(s1a)`.
   Same-package correct pattern: `self_development_route.go:56-61` binds
   `ownershipKey(ownerID, desktopID)`; `cold_recover.go:451-456` round-trips
   correctly. Confined to `route_authority.go:398` — other
   `GetOwnershipByComputerID` call sites take real computer IDs.
   The S1a refusal matrix's green L2 passed only because
   `scripts/s1a_refusal_matrix_probe.mjs:141` built the slot as
   `computer:${A.user}:${A.computer}` (stable computer ID, non-canonical),
   and the only route-resolve test runs over httptest loopback, where
   `isInternalCaller` short-circuits before the tap bind (G2b, verified in
   `handlers.go:1556-1575` + `self_development_route_test.go`).
   **Heresy accounting: discovered by S0b, introduced by S1a, unrepaired.**
   The repair belongs to S1a's boundary (or a named S1a-follow-up), not S2/S3:
   S1a closed on 2026-10-04 the same day the regression shipped; leaving a
   red-surface regression unowned across a boundary contradicts the S1a close
   receipt. S0b itself must not repair it (contract honored — no repair was
   made).

2. **The Go-effect probe was blocked by the harness, not the product, and
   must be re-run before S2 consumes effect facts.** Verified:
   `scripts/s0b_disposable_keydriver.mjs:80` now mints
   `computer:self_development:propose` + `mode` + `manage:keys` (commit
   `57613ddc`), but its 401/failed-mint fallback (lines 76-99) retries
   **without any scopes** — a successful fallback mint would silently repeat
   the original defect. The re-run is one cheap disposable session and is a
   named edge, not a send-back trigger on its own.

3. **The builder-substrate decision was deferred, not made — and the
   deferral is honest but must be explicit, not silent.** Verified:
   `docs/evidence/s0b-builder-substrate-decision-2026-10-04.json` exists,
   correctly states all three substrates remain unselected/unfalsified, and
   binds requirements for S2. The gaps falsify **no candidate**: the
   sealed-boundary probes measured product surfaces only —
   *scoped guest service* is weakened by the EROFS/no-build-surface facts;
   *host service* and *privileged builder capsule* were never exercised
   (host-side vmctl was reachable and carries all M9a/restore evidence).
   S0's `main_uncertainty` and the S2 settled decision both name "defer"
   as the excluded option, so the deferral must be owner-visible: the S2
   entry decision must explicitly adopt or re-scope it, not inherit it.
   However — the deferral receipt exists and says so. What was missing was
   only the aggregation surface, not the finding.

4. **Station-aggregation debt (paperwork, must land with the close record):**
   the canonical probe index `docs/evidence/s0-probe-index-2026-10-01.json`
   still lists only the seven S0a receipts; the ten S0b receipts (five
   probed + five folded gap/decision receipts) must be indexed with their
   mapping to the definition's original named receipts (snapshot/UFFD/reflink
   collapsed into `s0b-snapshot-resume`; overlay/EROFS/nixpkgs folded into
   two gap receipts). Without this, `finish.artifact` is formally unmet and
   a strict-textualist reviewer is right to object.

5. **Fixture hygiene is clean.** Verified `s0b-disposition-2026-10-04.md`:
   disposable left active, no runs, self-dev off, no repair commits, and an
   explicit warning not to reuse the M9a route result as healthy. Terminal
   state is unverified live (accepted as frozen evidence).

## Consensus

- The M9a defect chain and its S1a provenance are source-verified by three
  independent agents (devin, qwen38max, opencode) and matched by the
  synthesizer's own read.
- No candidate S2 substrate was falsified by the probes; the decision moves
  to S2 with gaps as binding input.
- The Go-effect probe must be re-run; the snapshot/dep/capsule surface
  probes must not (re-running on the same sealed boundary reproduces the
  same gaps by construction).

## Dissent / Disagreements

- **send_back camp (5/10 deciders, including the flagship):** the
  unsupported-transition clause licenses gaps *reached by attempting a
  transition*, not three receipts with zero observation standing in for
  named acceptance items; the substrate-decision obligation is
  owner-ratified and cannot be waived by the panel. This is the strongest
  dissent and is factually correct on every point it verifies.
- **Adjudication:** the send-back conditions are real but cheaper to
  discharge at the close boundary than the alternative: re-running one
  harness-fixed probe and indexing existing receipts is minutes of work;
  re-running the full M9a/snapshot fixture round is not. The accept camp's
  core point also holds: M9a's apply+restore legs *are* a successful
  lifecycle observation (materialization + witness-matched product restore),
  so the "no successful lifecycle" objection is factually wrong.
- **Devin produced no verdict** (its sandbox rejected its own tool calls);
  treated as metadata per the runner contract, not as a vote.

## Unique high-value findings

- The S1a refusal-matrix L2 leg is green *because of a non-canonical slot
  construction* — the acceptance evidence itself used the buggy form. Any
  repair regression test must use a **tap-sourced, non-loopback,
  non-192.0.2.x** caller and the canonical `computer:<owner>:primary` slot;
  a loopback test re-greens this exact bug.
- The keydriver fallback path minting without scopes is a second latent
  instance of the same defect class the primary path just had.
- `binding` defect is confined to `route_authority.go:398` — the blast
  radius is one handler, not the registry API.

## Risks / edge cases

- Fresh-computer platform-follow promotion is broken on staging until the
  S1a repair lands; any S2 probe needing ComputerVersion route identity
  (observed: `kernel-capabilities` 503) will fail for the same reason and
  must not be misread as new substrate evidence.
- The disposable still holds the only pre-repair state of the defect; if the
  owner ratifies teardown, keep the receipt evidence (already committed) and
  the problem doc — they are sufficient to reproduce the diagnosis.
- Unverified live: staging terminal state, deploy identity `e87f3294`
  (git object verified locally; deployed serving identity taken as frozen).

## Decision

**`accept_with_edge`** — accept S0b's evidence set, with these edges named
in the transition receipt and owned explicitly:

- **E1 (S1a repair):** fix the `route_authority.go:398` desktop-component
  binding under S1a authority (not S2/S3); regression test must be
  tap-sourced + canonical slot; end-to-end fresh-computer assertion
  (marker + route generation 1) required.
- **E2 (S2 entry):** re-run the self-dev Go effect with the corrected
  keydriver (and fix its scope-less fallback first).
- **E3 (S2 entry):** S2's first decision must explicitly adopt or re-scope
  the substrate deferral recorded in
  `s0b-builder-substrate-decision-2026-10-04.json`; the three-candidate
  space remains open with the EROFS/sealed-boundary facts as constraints.
- **E4 (S3):** snapshot/UFFD/reflink facts are "vmctl exposes no endpoint";
  the Firecracker capability itself remains unmeasured and belongs to S3.
- **E5 (paperwork, this boundary):** index all ten S0b receipts in
  `s0-probe-index-2026-10-01.json` with the name mapping to the
  definition's original receipt list.

**Confidence: high** on the defect chain, S1a provenance, and the
deferral-as-finding reading (all source-verified locally); medium on the
live staging terminal state (frozen, not re-derived).
