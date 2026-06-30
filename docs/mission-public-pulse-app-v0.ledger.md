# Public Pulse App v0 Ledger

## 2026-06-14 — paradoc opened

- Claim: launch usage monitoring should be a public Choir app rather than a
  private operator analytics console.
- Owner premise: aggregate usage/health data should be public; private
  surveillance data is liability, not asset.
- Move: created `docs/archive/mission-public-pulse-app-v0.md` with working name
  Pulse, no-surveillance invariants, aggregate metric scope, classification
  boundary, and settlement requirements.
- Expected ΔV: 0 against implementation obligations; this creates the source
  program for the next launch-prep app mission.
- Actual ΔV: 0.

## 2026-06-14 — Pulse implementation slice

- Claim tested: a public app can expose useful launch health without creating a
  private analytics/surveillance store.
- Move: added the `pulse` app registry entry, public `/api/pulse/summary`
  proxy route, internal vmctl aggregate source, account classifier, storage and
  real-computer health buckets, and report-only frontend surface.
- Privacy boundary: response emits aggregate class counts only. It excludes raw
  user IDs, email addresses/lists, prompts, docs, traces, messages, source
  histories, generated artifacts, IPs, user agents, referrers, device
  fingerprints, geolocation, session replay, and per-user timelines.
- Classification boundary: `example.com` and `example.test` classify as
  `codex_agentic_test`; `a@b.com` and `b@c.com` classify as
  `protected_test` internally but are not emitted as email strings.
- Evidence so far: focused vmctl/proxy tests passed locally; frontend
  production build passed locally with only pre-existing Universal Wire Svelte
  warnings.
- Expected ΔV: public app/API/test obligations mostly discharged; staging
  deploy, public route proof, and runtime measurement remain.
- Actual ΔV: implementation variant decreased from "no app/API" to "local
  tested app/API awaiting CI and staging evidence."

## 2026-06-14 — staging settlement evidence

- Behavior commit: `9b39b84d9fa7d964847c2bb8cfb4bce7ba85f166` added the
  public Pulse app/API.
- Follow-up commit: `a92498e2349bfb1965a577123e019d4e0c7da369` fixed browser
  rendering of account class counts after Playwright caught the loading/count
  mismatch.
- CI/deploy evidence:
  - Run `27509667041` passed full CI and deployed the app/API slice to staging.
    Deploy selected frontend and host services, skipped ordinary/playwright
    guest image build/install, and did refresh active computers because vmctl
    changed.
  - Run `27509797114` passed full CI and deployed the frontend-only fix.
    Deploy selected frontend only: `deploy_host=false`,
    `deploy_vmctl_restart=false`, `deploy_active_vm_refresh=false`,
    `deploy_ordinary_guest=false`, and `deploy_playwright_guest=false`.
- Staging identity: `https://choir.news/health` reported deployed commit
  `a92498e2349bfb1965a577123e019d4e0c7da369` at
  `2026-06-14T19:39:10Z`.
- Public API proof: `https://choir.news/api/pulse/summary` returned HTTP 200 in
  `0.122590s`, below the 10-second settlement bound.
- Public aggregate snapshot: `total=381`, `real=3`,
  `codex_agentic_test=376`, `protected_test=2`, `unknown=0`,
  `real_active_last_24h=1`, `real_primary_usable=2/2`,
  `manual_recovery_snapshot_count=4`, `vm_state_used_percent≈75.53`.
- Privacy proof: saved response passed an identity-field grep for raw email
  addresses, `user_id`, `trace_id`, `session_id`, `ip_address`, and
  `user_agent`; browser text proof also reported `leaked=false`.
- Browser proof: signed-out Playwright opened Pulse from `choir.news` and
  verified rendered counts `REAL 3`, `CODEX TEST 376`, `PROTECTED TEST 2`, and
  `TOTAL 381`; screenshot saved outside the repo at
  `/tmp/pulse-staging-final.png`.
- Settlement: v0 public Pulse app is deployed and report-only. Remaining
  hardening axis is cache/rate-limiting for public storage sampling if traffic
  rises; this does not block v0 launch visibility.

## 2026-06-14 — Parallax State reconciled

- Claim: the paradoc should not keep routing future agents through an
  implementation handoff after v0 staging settlement has already been recorded.
- Move: rewrote `docs/archive/mission-public-pulse-app-v0.md` Parallax State from
  `open_handoff` to `settled`, named Pulse / Choir Pulse as the v0 product
  name, preserved "metrics" as the ordinary noun for counters, and updated the
  suggested goal string to prevent accidental v0 re-entry.
- Evidence: prior ledger entry records commits `9b39b84d` and `a92498e`,
  CI/deploy runs `27509667041` and `27509797114`, public API proof, signed-out
  browser proof, and privacy grep proof.
- Expected ΔV: close the stale-state mismatch.
- Actual ΔV: v0 mission state now matches the recorded settlement evidence.
