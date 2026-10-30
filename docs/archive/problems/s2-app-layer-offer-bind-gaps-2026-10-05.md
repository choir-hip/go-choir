# S2-g: app-layer push reaches targets only through narrow bind gates — 3 consecutive skipped-land deploys

## Symptom

Three consecutive `deploy_app_layer=true` staging deploys (`a578bbc8` SBOM-only,
`398a9459`, `2f0e2cac`) built a layered release and pushed it, but landed it on
**zero** computers. Every deploy reported `0/0 healthy, N skipped`. The S2-g
mechanism (build → stage → offer → apply → healthy) is correct; the bind is
what refuses.

## Evidence — the two sequential bind gates

The deploy script (`ci.yml` ~1486–1560) mints a `PlatformUpdateOffer` per active
tracking/canary computer. Two independent gates each refused every target:

### Gate 1 — `canonical head unavailable` (target: `vm-7bbcf744…`, computer `6450a253`)

`GET /internal/computers/events/head` returned `computer event head not
initialized`. The guest had never CAS'd a `genesis_imported` event, so the
deploy's `base_event_head` had nothing to bind to and skipped the target.

A `file_root` append can't mint the head — `reduceGenesis` only accepts
`EventGenesisImported`. The organic first-commit path is the guest's own
`BootstrapChain` (guest-derived `code_ref`/`artifact_program_ref` from build +
image manifest), exposed at
`POST /api/computers/<id>/lifecycle/bootstrap-chain`. Calling it minted the
head (`canonical_event_head f291cd05…`, seq 4 after a resume's lifecycle
receipts).

### Gate 2 — `offer mint refused` / `verifier evidence required` (all targets)

Once a head existed, `POST /internal/computers/platform-updates/offer` returned
400 `platform update offer: verifier evidence required`. The CI mint payload
passed `verifier_refs: []`. `PlatformUpdateOfferFromRequest` requires ≥1
verifier evidence ref (S2-c). The layered probe uses `verify-<update_id>` as a
deterministic sha256 marker bound into the accepted event — the CI never set it.

### Gate 3 — `offer binds a different realization` (post-resume)

After minting a signed offer and pushing, the guest refused: the offer's
`realization_id` (`vm-…-epoch-12872`) was stale because a hibernate+resume had
bumped the boot epoch to `12873`. The realization must be re-read at push time.

## Fix landed

`a3da83c4` (`fix(s2-g)`): the CI mint now derives
`verifier_refs:[sha256("verify-<update_id>")]` matching the layered-probe shape,
so the offer signs and the guest accepts it.

## Deployed proof — manual end-to-end land

Before CI re-ran the path, the full bind→apply→healthy sequence was exercised
manually on `computer-6450a253b8b6ebc0866471973694f5be`:

1. Bootstrapped its canonical chain via `lifecycle/bootstrap-chain` (guest-derived
   code/image refs).
2. Minted `manual-app-layer` offer with `verifier_refs:[sha256("verify-…")]`
   and the correct current `realization_id` (epoch `12873`).
3. Pushed through `/internal/vmctl/autoputer-proxy/…/internal/runtime/platform-update`.
4. Guest applied the layered release, restarted, and came up healthy:
   `release_digest: 36743b1f6ddb…`, `build.commit: 2f0e2cac` (the pushed
   binary), `closure_digest: 2a5f6d59`, `layering_entrypoint` → the release's
   `bin/autoputer`, `current` → `releases/36743b1f…` atomically, accepted
   event head committed. `code_commit: 2f0e2cac`, `base_commit: eb9c5b19`,
   `builder_receipt_digest` bound (S2-c).

## Residual

- `canonical head unavailable` is still a hard gate: a tracking computer with no
  committed chain is skipped silently. Whether the deploy should bootstrap-chain
  idle targets (or whether head-less computers are out of scope for app-layer)
  is an open S2-g/S0 contract question — bootstrap-chain is owner-authorized,
  so auto-minting it from CI needs an authority decision. 2026-10-05
  acceptance run: vm-7bbcf744's head was bootstrapped by the manual proof, so
  the gate does not currently skip it.
- `offer binds a different realization` is a correctness gate (good), but the
  CI reads `epoch` from the ownership list once; a hibernate+resume mid-push
  makes the mint stale. FIXED 2026-10-05 (ci.yml, same commit family): the
  epoch is re-resolved per mint attempt and a push refusal retries the
  mint+push once with a fresh realization, then skips.
- NEW GATE (found 2026-10-05 by the acceptance probe): `expires_at` must be
  ≤5 minutes out (`selfdevprotocol/platform_update.go:134`), but the CI mint
  sent `+30 minutes` — every CI-minted offer would have been refused with
  `short canonical expiry is required`. Never observed because a3da83c4's
  deploy was ci.yml-only and skipped. Fixed in ci.yml (expiry now +4m).
- `candidate-fleet-*` ownerships fail `computer event capability refused` —
  the deploy can't mint an offer for them without a per-computer capability.
  Still open; fleet owners need event:read credentialing or an explicit
  exclusion for non-owned fleet computers.

## Mutation class

Orange (CI/deploy behavior). The bind gates are contract enforcement, not bugs —
the fix is supplying the evidence the contract demands, not weakening the gate.
## Criterion-2 close (2026-10-05 ~21:53Z) — CI-driven land delivered

CI run 37378327268 (`4ef44901`, autoputer-only diff → `deploy_app_layer=true`)
minted, signed, and pushed the layered release through the deploy job's own
path: `app-layer push: 2/2 healthy, 0 skipped, time-to-healthy=131s`. Targets:
`vm-7bbcf744`/`computer-6450a253` (S2 test disposable, active) and
`candidate-fleet-e15cb89f`/`computer-03335285` (owner computer). Guest
`current/` now points at `releases/211ef63a…` whose manifest marker is
`app-layer-4ef4490123ee` — CI-originated end to end, not a manual mint.
The `candidate-fleet` capability gate did NOT refuse this run — the earlier
`computer event capability refused` observation no longer reproduces; the
gate note below stands corrected, not reopened.

Residual bind-gate candidates that remain open: canonical-head bootstrap for
never-committed computers (owner-authority decision), and the wedge-retry
hardening landing in 7c0897c2 (deployed same window).

