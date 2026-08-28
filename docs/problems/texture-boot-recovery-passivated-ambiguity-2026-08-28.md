# Passivated Texture Revision Runs Make Boot Texture Recovery Ambiguously Refuse Startup

<problem_id: texture-boot-recovery-passivated-ambiguity-2026-08-28>
<first_observed: 2026-08-28T19:20Z>
<mutation_class: red>
<deployed_commit: ca01210f40d1bdbd0264e203c9488af5d5cfb055>
<affected_surfaces: [internal/textureowner/texture_controller.go, internal/types/task.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

After the worker-update substrate repair landed and the guest was refreshed
(epoch 834, `ca01210f`), boot now completes every phase - including
`sweep_open_work_item_actors` and `runtime: started` - and then refuses
startup deterministically, crash-looping every ~33 seconds:

```text
autoputer: runtime startup refused: actorruntime: reconcile Texture owner:
ambiguous boot Texture recovery run authority
```

Mechanism: `internal/textureowner/texture_controller.go` boot recovery picks a
non-terminal Texture agent-revision run per document to resume. The 08-28 boot
crash loop passivated interrupted runs repeatedly, minting multiple
`passivated` Texture revision runs for the same document/trajectory
(`passivated` is `Terminal()==false` per `types.RunState.Terminal()`). The
candidate enumeration (`ListLifecycleRunsByChannel`) therefore finds two
matching candidates and returns `ambiguous boot Texture recovery run authority`,
refusing `Runtime` startup. The passivation damage from the outage now bricks
Texture owner reconcile even though every scan substrate is repaired.

This is the same family as
`docs/problems/selfdev-wake-passivated-super-silent-noop-2026-08-28.md`:
`passivated` means "interrupted and permanently replaced by boot restart"
(`RunState.Active()` doc: passivated runs "no longer own live actor slots"),
but recovery-site predicates keep treating passivated as live-eligible.

## 2. Evidence (live, 2026-08-28, Node B journal)

Guest `x0dx6l624xa5hrmdwxpna0hfqvg9svlg` (build `ca01210f`,
`deployed_at=2026-08-28T18:51:06Z`), generations:

| listen | refused |
|---|---|
| 19:20:50 | 19:21:17 |
| 19:21:21 | 19:21:49 |
| 19:21:54 | 19:22:22 |
| 19:22:27 | 19:22:54 |
| 19:22:49 (`runtime: started` in ring) | 19:23:26 |

Full boot ring for the 19:22:27 generation shows every phase completing:
passivation (candidates=1, run `fe92ea2b` pending→passivated), Super rewarm
(`delivered-pending-runs=1280`, validate, `pending=1`, dispatched), terminal
outcome (candidates=0), work-item sweep (`open_items=20`, 5.6s),
`runtime: started` at 19:22:49 - then the refusal at 19:23:26 in the
post-start Texture owner reconcile. Code:
`internal/textureowner/texture_controller.go:130` returns the ambiguity error
when two channel runs pass the `!State.Terminal()` + revision-type + mutation
match; the crash loop minted >= 2 passivated such runs for texture document
`c273a57b-a253-5234-888d-6139024a6cf1`.

## 3. Non-fixes

- Do not change `RunState.Terminal()` globally (passivated is deliberately
  non-terminal across the platform; a global flip is an unbounded semantic
  change).
- Do not delete or SQL-edit the passivated runs.
- Do not disable or bypass Texture owner reconcile.
- Do not re-hold or SSH-mutate the guest.

## 4. Fix (general product path)

In the boot Texture recovery candidate selection, require the recovery run to
be live-eligible: `State.Active()` (pending/running/blocked) instead of
`!State.Terminal()` for all three candidate sources (active-run pointer, actor
memory, channel enumeration). Per the platform's own doctrine, passivated runs
no longer own live actor slots and must never be resumed as boot recovery
authority; skipping them leaves zero candidates when only passivated runs
remain, and reconcile mints a fresh Texture run instead of refusing.

Rollback: revert restores the ambiguity refusal (crash loop) — no schema
change.

## 5. Acceptance

Guest boot on the repair commit reaches `runtime: started` AND holds past the
Texture owner reconcile (no `startup refused`), >= 3 minutes stable, owner API
200s. Unit test: two passivated revision runs for one document must not produce
the ambiguity error and must not activate either run.
