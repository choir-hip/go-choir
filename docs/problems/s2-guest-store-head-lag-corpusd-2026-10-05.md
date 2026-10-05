# Guest store head lags corpusd tape — layered apply permanently stale
Date: 2026-10-05 · Computer: computer-6450a253b8b6ebc0866471973694f5be (disposable, owner a85b8fee…e4c335) · Station: S2

## Symptom
Five consecutive `platform-update` offers bound against corpusd's `canonical_event_head` refused `platform update: base event head is stale` — no `effect_accepted` committed for the last three attempts (tape tail unchanged). The refusals are deterministic, not racy.

## Evidence
- corpusd `events/head`: `canonical_event_head=278bc1f8…`, `pending=""` (settled at seq 38).
- Same head served to `mk_offer` mint produces `base_event_head=278bc1f8…`.
- Guest boot timeline: `replay.committed_sequence=34` (guest's local store visible seq only reaches ~34). Discharge events at seq 33, 35, 38 were committed to corpusd via CAS by `s2_discharge_pending_transition.go` — the guest never replayed them.
- Guest resume log: `projection recovery resume … local=33 W=0` — recovery replays toward corpusd but had not reached seq 38 by push time.
- `console.log` for vm-7bbc… dies ~16s after each refresh (serial sink stops mid-boot); the guest stays `ready` but the newest apply attempts never appear — consistent with a projection store that finished boot mid-flight and stopped receiving events.

## Recurrence ladder
- 05:54 offer minted against the pre-deploy manifest (`base_commit=6fcb05e5`) refused — the deploy landed `34041932` mid-run and the booted base moved.
- After each refusal the pre-fix runtime (`2f0e2cac` still serving) wedged `pending_transition` → next offer resumed-gated `stale`.
- Manual discharges restored pending="" each time (3 discharges: 062505, 064455, 055444) — but each discharge adds an event the guest's local projection doesn't see, moving corpusd's head further ahead of the guest's head, so the *next* offer binds a head the guest doesn't have → stale again.

## Shape
`recordPlatformUpdateFailed` (via discharge tool) writes to corpusd; the guest's `rt.store.Head` reads the guest's own projected store, which only absorbs new events on replay (boot or resumption). `ApplyPlatformUpdate` compares the offer's `base_event_head` to that local head — which is behind corpusd whenever any event was committed directly to corpusd since the last replay.

The mint-side reads corpusd's head; the apply-side reads the guest's head. Any CAS write to corpusd outside the guest's append path (discharge tool, `file_root_committed` from blob upload, key_revoked lifecycle events) widens the gap.

## Open questions
- Is there a bound by which guest projection lag drains (does it ever catch up short of refresh/reboot)?
- Should `base_event_head` bind the guest's local head rather than corpusd's? Both ends must name the same authority or the gate compares different tapes.
- Console sink death ~16s post-boot blocks direct guest updater logs; the real daemon refusal reason at seq 31/36 (`updater refused request`) is still unobserved — probably an epoch-ID fence or manifest-digest mismatch that the stale-head error masks.

## Rollback
None needed (read-only diagnosis; the wedge-discharges are already on the tape).
