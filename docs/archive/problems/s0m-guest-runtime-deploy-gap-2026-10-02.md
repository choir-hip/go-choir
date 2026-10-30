# Problem: in-guest runtime changes do not reach a constructed-computer-version guest via the host deploy

**Discovered:** 2026-10-02, while re-proving the S0m stranded-bound release fix.

**Mutation class:** `orange` (deployment/ops semantics, not a product behavior change).

## What happened

The S0m stranded-bound repair (`c9180cd3`, hoisted in `356cec8a`) edits
`internal/agentcore/research_checkpoint_fallback.go` — code that runs inside the
**guest** autoputer (`internal/autoputer` hosts `agentcore`). The probe
`scripts/s0m_stranded_bound_probe.mjs` drives the owner staging computer
`computer-03335285269bdba4f94377e56879f9e6`.

That computer is `snapshot_kind: constructed-computer-version` with
`divergence_status: canary`. Its `/health` (guest autoputer, via the VM
`computer_url:8085`) reported:

```
commit = 734ce69b...   deployed_commit = 734ce69b...
```

— while node-b's host autoputer package and the proxy `/health` already showed
`e824826a`. The guest's `identity.executable_resolved` is the **Nix-store**
`autoputer` binary; the updater's `current/` release serves only the frontend
SPA. So guest autoputer code is pinned by the *realization's* construction
version, not by the host `services/autoputer` package.

## Root cause

`HandleRefresh` / `RefreshVMForDesktop` (`internal/vmctl`) force-reboots a
computer onto the **current** guest image. But the CI deploy's global active-VM
refresh explicitly **skips** `constructed-computer-version` computers
(`.github/workflows/ci.yml:1338`), and the owner-scoped refresh
(`HandleRefresh`, ci.yml only targets *mutable* active computers) is the
separate authority that moves them. Net effect: a `platform-vmctl` / agentcore
commit can be deployed to every host service while every
`constructed-computer-version` guest still runs the older autoputer.

## Why it matters

Any station change in `internal/agentcore`, `internal/autoputer`,
`internal/textureowner`, `internal/actorruntime`, etc. that is proved against a
constructed guest is **unproven** until the guest is refreshed to a realization
whose autoputer package contains the commit. Proxy `/health` `deployed_commit`
only reports the host build — it is *not* evidence the guest is on the fix.

This invalidates the earlier `docs/evidence/s0m-*` "deployed proof" claim for
the stranded-bound release: that probe ran against guest `734ce69b`, which
predates the fix, and its "wedge → recovered" observation came from the
pre-existing unbind-on-activation reconcile, not the terminalization release.

## Evidence

- Owner computer: `computer-03335285...` → `deployed_commit = 734ce69b`,
  `code_ref = code:sha256:499bee7b…` (vmctl `ownerships`, `/health` :8085).
- Host autoputer package: `e824826a` (`/var/lib/go-choir/services/autoputer`).
- Probe trajectory `73266f2d-fc6d-5845-a990-1df9263fc7c9`: control
  `24e48cc6` stayed `pending`/`bound` to carrier `d4a8e927` through 20:45 UTC
  on a guest that does not carry the fix.
- `internal/agentcore` is compiled into `internal/autoputer` (guest), not the
  host `corpusd`/`platform`/`vmctl` services.

## Correct proof path

For every station whose change lives in the guest runtime:

1. Push → CI green → deploy builds the host `autoputer` package at the new
   commit.
2. Owner-scoped `vmctl refresh` (or a fresh computer that realizes onto the new
   image) so the guest `executable`/`deployed_commit` is ≥ the fix commit.
3. Re-run the deployed probe against the refreshed guest; assert the guest
   `/health` commit, not only the proxy's.

## Related

- `docs/problems/s0m-stranded-bound-control-deadlock-2026-10-02.md`
- `docs/problems/s0m-stranded-bound-release-root-landed-2026-10-02.md`
- `docs/evidence/s0m-…` — earlier deployed proof, now superseded by this
  guest-staleness finding.
