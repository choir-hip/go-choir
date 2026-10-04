# S0b Boundary Panel — disposable-computer probe suite (S0 reality-boot station)

You are one member of an independent agentic consensus panel in convergent mode.
Do not assume other agents agree with you. Return concise, decision-useful output.
Frozen evidence follows; do not re-derive it.

## Mission context

Station S0 (`docs/definitions/choir-appdev-s0-reality-boot-timeline-2026-10-01.md`)
is the reality-boot timeline station. S0a (deployed instrumentation + seven
receipts) closed 2026-10-02; S0m (record-native messaging) landed between;
S1a (host-boundary hotfix) landed 2026-10-04 and closed the S1a->S0b transition.
S0b is the measurement slice: bounded disposable-computer probes per the S0
mechanism contract, which states "an unsupported transition is a valid S0
finding; it must be documented, not repaired in S0."

The S0b evidence is deployed on staging `e87f3294` (includes the S1a send-back
POST legs + a CI flake fix on the delivery-claim assertion).

## Frozen evidence - 5 probes, 1 disposable computer

Disposable: `computer-ac1808b4f04fc749c0781083ee747403`
(vm `vm-b59963240d2cd8da769b92cacb213802`, owner `69fcbf14-7fa7-442b-9e69-9c2666274b9e`,
guest `10.200.227.2:8085`), genesis'd via `bootstrap-chain` (201), API key
minted. All probes read-only or self-dev-arm-then-disarm; terminal state: VM
active, no runs, self-development mode off.

### Probe 1 - capsule health map -> **gap**
`docs/evidence/s0b-capsule-health-map-2026-10-04.json`. Guest reports ready +
empty desktop/runs; `/api/capsules` returns SPA-fallback HTML, not a capsule
projection; `/run/choir/capsules` is unreachable across the sealed boundary;
host VM-state dir has no capsule entries. **No observation was made** - this is
a sealed-boundary gap, not evidence of capsule health.

### Probe 2 - M9a bundle lifecycle -> **partial + confirmed defect**
`docs/evidence/s0b-m9a-bundle-lifecycle-2026-10-04.json`. Platform-control-signed
update offer accepted -> materialization_started -> materialization_applied ->
checkpoint_published; restarted guest serves `S0B_M9A_20261004_0707` marker;
product restore completed `witness_matched=true frontend_restaged=true`.
**But** route slot never promoted (`route_absent:true generation:0`); guest log:
`vmctl client: ComputerVersion route resolution failed (status 403): caller not
bound to route slot computer: no ownership for computer primary`. Root cause
(source-traced): the fresh-owner/desktop slot contract is `computer:<owner>:primary`,
but vmctl's guest binding calls `GetOwnershipByComputerID` on the slot's second
component - `GetOwnershipByComputerID("primary")` - while the stable computer ID
is `computer-ac...`. -> `docs/problems/s0-m9a-route-projection-owner-binding-2026-10-04.md`.

### Probe 3 - one Go effect -> **keydriver scope gap (no execution)**
`docs/evidence/s0b-go-effect-2026-10-04.json`. Key minted by the probe driver
lacked `computer:self_development:propose`; re-mint refused (`manage:keys`/`admin`
required); the only stored staging session belongs to a different owner and was
not used (single-disposable-account boundary preserved). No Go cell executed.
`scripts/s0b_disposable_keydriver.mjs` now includes the scope for future runs.

### Probe 4 - absent runtime dep lifecycle -> **gap**
`docs/evidence/s0b-absent-runtime-dep-2026-10-04.json`. Guest `/nix/store` is
EROFS, sealed VM exposes no dependency-build/install/activate/dispose surface;
`/internal/vmctl/runtime-package/autoputer` streams a package, not a build API.
No observation.

### Probe 5 - snapshot/resume -> **gap**
`docs/evidence/s0b-snapshot-resume-2026-10-04.json`. vmctl exposes
resolve/resume/recover/refresh/runtime-package; **no** snapshot create/load,
UFFD lazy-load, or reflink handler. Host state dir holds only live FC config +
disk images. No observation.

## Questions for the panel

1. **Evidence sufficiency.** Does the combination of 1 partial-with-defect +
   3 boundary gaps + 1 scope-miss satisfy S0b's contract (valid findings incl.
   gaps documented as evidence), or does the boundary require at least one
   *successful* lifecycle/observation beyond M9a's apply+restore legs?

2. **S2 builder-substrate signal.** The gaps are themselves the S0b finding:
   the current capsule/base substrate offers no capsule-health projection, no
   dep-build surface, no VM snapshot endpoint. Does this falsify any candidate
   S2 substrate (host service / privileged builder capsule / scoped guest
   service), or merely relocate the decision to S2 with the gaps as input?

3. **M9a defect severity.** The route-projection owner-binding defect is a
   confirmed product defect (not boundary noise). Does it block the S0->S2
   transition, or is it correctly filed as a problem doc for S2/S3 to own?

4. **Residual risk.** Are there S0b-class experiments that should have run
   and did not (e.g., the self-dev Go effect, the snapshot surface probe) that
   the next boundary should re-attempt before advancing?

## Panel instructions

Panel should issue a verdict: `accept`, `send_back` (with named gap), or
`accept_with_edge` (naming the accepted edge).

You have repo access (`/Users/wiz/go-choir`, read-only). If you cite file, diff,
command, or API behavior, inspect it locally before reporting it as confirmed;
mark unverified claims as unverified. Do not paste secrets.

Output format:
1. Verdict / recommendation (`accept` | `send_back` | `accept_with_edge`)
2. Top findings
3. Risks / edge cases
4. Evidence or assumptions (verified vs unverified)
5. Confidence: high / medium / low
