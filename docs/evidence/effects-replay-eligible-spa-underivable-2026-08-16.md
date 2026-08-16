# Effects replay eligible; checkpoint SPA underivable — 2026-08-16

**Boundary:** execute (route map 9 red). Not live proof. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `06204346a431f52347586b7f68d39a4d2b9c282a` (`deployed_at` 2026-08-16T19:33:03Z)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Actuation

CI https://github.com/choir-hip/go-choir/actions/runs/31966792512 succeeded, including Node B deploy. G4 preserved `snapshot_kind=constructed-computer-version`. Constructed freeze remains `7122f2799be4458f4b925be11990321c7e70ffc4`.

Owner-scoped refresh (not rematerialize, not restart):

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-event-projection-refresh-2026-08-16T19:33Z --host https://choir.news --timeout 8m`

LifecycleReceipt `01a00c10-b658-7e65-a4aa-0567defdf74c`: epoch **274 → 275**.

Second residue import (leftover session replace):

`choir computer import-residue-snapshot --computer computer-03335285269bdba4f94377e56879f9e6 --host https://choir.news`

```json
{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","desktops":1,"sessions":2,"objects":24,"edges":0,"appended":true}
```

## After import

| Call | Result |
|---|---|
| `choir computer status` | active, epoch **275** |
| mode GET | 200 `propose_only` generation 1 |
| genesis POST | 409 `self-development effects are disabled` |
| `choir computer replay-completeness` | live_head == replay_head sequence **31**; `eligibility.eligible=true`; `result.status=equivalent`; no differences |
| `choir computer checkpoint` | **409** `self-development checkpoint: served SPA is underivable` |

Replay eligibility is paid. EmptyUntilSupported was not weakened for leftover tables (`desktop_state` stays empty_until_supported). Tables were not SQL-emptied.

Checkpoint bind still requires updater `current` with frontend files. Owner-scoped refresh dropped that pointer so the image binary ran. `ReadCurrentManifest` then fails. This is not a replay ineligibility. Super was not started. Outbox `Armed` remains false. No mail was sent. OwnerRecovery checkpoint was not published.
