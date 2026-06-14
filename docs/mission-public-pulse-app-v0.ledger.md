# Public Pulse App v0 Ledger

## 2026-06-14 — paradoc opened

- Claim: launch usage monitoring should be a public Choir app rather than a
  private operator analytics console.
- Owner premise: aggregate usage/health data should be public; private
  surveillance data is liability, not asset.
- Move: created `docs/mission-public-pulse-app-v0.md` with working name
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
