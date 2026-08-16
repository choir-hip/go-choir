# Effects owner refresh still masked by updater current — 2026-08-16

**Boundary:** execute (route map 9 red prep). Not live proof. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `c6dda13592c5e21ed17355fa5939a600c9534514` (`deployed_at` 2026-08-16T18:22:32Z, `built_at` 20260816181405)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## What happened

CI run https://github.com/choir-hip/go-choir/actions/runs/31963987149 deployed Node B. Global deploy preserved `snapshot_kind=constructed-computer-version` (G4). Constructed freeze remains `7122f2799be4458f4b925be11990321c7e70ffc4`.

Pre-refresh:

| Call | Result |
|---|---|
| `choir computer status` | active, epoch **272** |
| `choir computer replay-completeness` | 500 `guest credential: renewal refused` |
| `choir computer import-residue-snapshot` | **404** `computer route not found` |

Owner-scoped refresh (not rematerialize, not restart):

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-residue-import-refresh-2026-08-16T18:25Z --host https://choir.news --timeout 8m`

LifecycleReceipt `01a00bd8-51a0-7443-9ff8-5ea5473b25c8`: epoch **272 → 273**, state active, generation 34.

Post-refresh:

| Call | Result |
|---|---|
| `choir computer status` | active, epoch **273** |
| mode GET | 200 `propose_only` generation 1 |
| `choir computer replay-completeness` | 200; live_head == replay_head sequence **27**; `eligibility.eligible=false`; unsupported `desktop_app_instances`, `desktop_sessions`, `desktop_window_placements`, `desktop_workspaces`, `og_objects` |
| `choir computer import-residue-snapshot` | **404** `computer route not found` |

Proxy forwarded the import path (guest error string, not owner-wide 400). Guest checkpoint GET is 405 method not allowed, so the guest dispatcher is the older lifecycle allowlist without `import-residue-snapshot`. Guest init still prefers `/mnt/persistent/choir-updater/current/bin/autoputer` over the image binary. Refresh preserves `data.img`, so that pointer survived onto epoch 273.

Sequence 27 is not the residue import. Import was not executed. EmptyUntilSupported is unchanged. Genesis was not opened. Outbox `Armed` remains false. Super was not started.

## Consequence

Owner-scoped refresh must drop the stale updater-current pointer and exec the current deploy image. Ordinary restart/recover must keep the pointer so a promoted release survives stop+resolve.
