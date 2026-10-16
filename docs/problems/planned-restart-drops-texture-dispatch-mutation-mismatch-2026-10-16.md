# A planned restart drops the Texture work that was in flight (2026-10-16)

Status: documented; cause is a hypothesis until a mutation row is traced.
Found during the 22:40 hourly check.

## Evidence (the paper, vm-1669020adcece51c2f5ff9b6df399ef5)

Every planned refresh today was followed by Texture dispatches that failed
validation, were retried eight times, and were dead-lettered:

```
texture:<doc>/<update>: actorruntime: validate Texture initial_dispatch:
  validate Texture activation: mutation authority mismatch
dispatcher: poison <id> … after 8 attempts … delivery-poison
actorruntime: dead letter: delivery_failed for update <update> from texture:<doc> dropped after delivery failed
```

| Boot (planned) | Mismatch lines |
|---|---|
| 16:29:59 | 24 |
| 16:45:16 | 36 |
| 18:39:17 | 6 |
| 21:37:26 | 24 |
| 22:22:27 | 12 |
| 22:33:08 | 18 |

The runs it hits were mid-turn when the marker-backed restart came:

- `e27c019a` (Texture of doc f9da224a): a provider call at 22:21:16, then
  `activation.passivated {"recovery":"passivated_on_restart"}` at 22:22:30
  and again at 22:33:50. Stored state: `passivated`, nothing since.
- `386ce9a8` (Texture of doc efda8ea1): a provider call at 22:31:59,
  `activation.passivated` at 22:33:10. Stored state `running`, updated
  22:34:51, with no event after the passivation.

The boot logs `boot phase end platform_update_resume dur=0s` both times.

## Hypothesis

`passivate_interrupted_activations` passivates a mid-turn Texture
activation, and its re-dispatch then fails `validateTextureActivation`
(`internal/textureowner/texture_controller.go`) because the run's agent
mutation is no longer `pending`, or no longer names the document's
current revision. The update is retried until poisoned and then dropped,
so the planned restart that the rule says "may resume work" ends that
Texture turn silently instead. The story waits for the next record or
wake to start a new turn.

To confirm: read the `agent_mutations` row of e27c019a (state, revision
id) against the document's current revision, from the run's own trace.

## Why it matters now

Each deploy drops whatever Texture turns were in flight: six deploys
today, 6 to 36 dropped dispatch attempts each. The overnight landing loop
deploys often, so the paper loses turns at exactly the moments it is
being changed.

## Related

- docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md
  (the same validation refusing a raced first turn; fixed for that race
  on 2026-10-15).
