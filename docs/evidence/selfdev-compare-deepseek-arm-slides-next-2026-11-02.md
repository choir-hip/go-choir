# Self-development comparison, DeepSeek arm (d@e): slides-next, 2026-11-02

Read-only monitor record. Computer `computer-59c902c5d46caecc0a180a8557164e72`
(VM `vm-70070cf7337d45f2d46eafc83b9783b9`), platform build 7c73688b3.
Trajectory `5c705f55-84f4-56da-86e7-b9379f0b7d2f`, Texture doc
`ddea2c8e-e277-5200-8751-cb66229f9e93`. Observation window 19:40Z to 20:04Z
(24 minutes of a 120 minute budget).

## Outcome

No candidate was produced. Engineering never started: the Cast was
accepted and the assignment opened, but engineering admission has been
refused every 10 to 60 seconds since 19:40:32Z by a stale self-development
operation (item 2). At 20:04Z the assignment is `open / capsule unbound`, no
capsule exists, there are no engineering runs, tool calls, freeze or verdict.
Items 3 to 5 are therefore empty by construction. This is a platform
blocker, not a model result: the DeepSeek engineering model was never
exercised.

Monitor-hygiene note: the scratchpad is shared with the b@c monitor, which
overwrote my `c.sh` wrapper (pointing it at b@c's key and computer) and
`tail.log`. Readings taken 19:55 to 20:02Z through that wrapper were of
b@c's computer and are discarded. Everything below is from d@e reads
(CLI with the d@e key, verified `computer_id` in each reply, and host-side
console log of this VM).

## 1. Per desk

Source: `trace export` layers (runs, run_events, cells), lifecycle snapshot.

| Desk | Runs | Model served | Controls | Notes |
|---|---|---|---|---|
| Texture (`texture:ddea2c8e`, loop `fe250f95`) | 1 (19:40:13 to 19:40:27Z, passivated `durable_transition_success`) | `gpt-6-luna` (loop.progress) | 2 intents in the one commit: revision `209677a4` (author_kind appagent, v1) and the hand-off to management | 2 `desk_go_eval` calls, 0 empty rounds, 0 refusals |
| Management (loop `416b99af`) | 1 (19:40:27 to 19:40:34Z, completed) | `gpt-6-luna` | 1 Cast (`cast:tray-1`, call `call_a00861e11dc441518d29dbe5ae0d7be9`), assignment kind `implementation`, on the first run | 0 empty rounds, 0 refusals, no dead letters |
| Engineering (`engineering:assignment-a484736a...`) | 0 | none (never admitted) | none | pinned to DeepSeek-V4.1-Flash-1017 per the brief; not observable |

Dead letters: none seen. Storage errors: none seen in the trace or the
console grep. Management's first-run Cast contrasts with the earlier b@c run
(three runs to Cast).

## 2. Blockers before engineering

Blocker 1, the only one so far: engineering admission waits behind
self-development operation
`rollback-4dca8039-97fe-4615-a048-36f1662efb66`
(trajectory `trajectory-rollback-49f6cd84-f1d5-440e-a6b0-6107baf56994`),
state `degraded`, created 2026-10-27T00:07:47Z, last updated
2026-10-30T16:32:25Z. Its recorded error: "the settlement did not finish on
two boots (last: rollback_pending): the rollback is recorded on the tape,
its checkpoint and route promotion are not; an owner act (rollback, or a
manual re-bind) decides". It is a rollback, not an awaiting-approval op.

Console evidence (this VM), repeated:
`engineering admission: assignment assignment-a484736afd99457686541f67 waits
behind self-development operation rollback-4dca8039... still degraded, which
a Freeze on this computer waits for (operation_open); retry N in 1m0s`
- first line 19:40:32Z (after `capsule ... saga phase=cast_preflight
  took=488ms`), retry count 7 at 19:44:42Z, 13 at 19:50:42Z, 22 at 19:59:42Z,
  26 at about 20:03Z; no change in the interval.
- In parallel the materializer logs, several times a second, `selfdev
  materializer: operation rollback-4dca8039... in degraded needs retry:
  materializer: rollback release unavailable: updater: pinned release
  unavailable` (so the op cannot self-resolve: its pinned release is gone).
- Boot at 19:35:40Z logged `selfdev: crash boot: rollback-4dca8039
  (degraded) interrupted; settles from the updater journal, never
  re-applied`: the op is re-reported, never settled.
- Planned platform-update restart of the VM at 19:58:05Z (`restart
  kind=planned reason=platform update target=cc0a05be95ad...`, marker consumed
  after 0s). The queued request was kept: `planned boot: queued engineering
  request assignment-a484736a... kept and resumed`. Still refused afterwards.
  The VM had also booted at 19:38:52Z, before the request.

No `stack_base_moved`, no rebase, no stale awaiting-approval op were seen on
d@e. `self-dev list` for d@e: 1 degraded (the one above), the rest are
applied, rejected, rolled_back or failed from 2026-10-26 and 27.

Resolution: none. It needs an owner act (the error text says so) or a
platform change; I did not act.

## 3. Engineering capsule

Capsule id `capsule-396b18ef-4f2b-523d-981a-3bd08d844735`, assignment
`assignment-a484736afd99457686541f67` attempt 1, allocated on paper only
(`capsule_disposition: unbound`). From `capsule-evidence --attempt 1`:
network_mode `ecosystem_proxy`, filesystem_mode
`assignment_local_writable_overlay`, cast_authority `records`, source
artifact `capsule-source-git:7c73688b3d72...`, writable true. Evidence
verifier `f1-incomplete`; deficits listed: capsule_fate_history_missing,
grant_policy_attestation_missing, isolation_probe_missing,
run_acceptance_gate_missing, texture_source_missing. Tool calls 0,
iterations 0, no freeze, no verify, no judge.

Host log on this VM at boot (19:38:52Z): `capsule executor: visual
capture: this computer is not granted visual capture by the host (no grant
on the credential disk)`, and `package mirrors` line absent at that boot
(earlier boot at 19:35:39Z logged "not granted the host package mirror").
Hypothesis: if the admission blocker is cleared, the visual judge and
package checks may find d@e without those grants; not verified.

## 4. Timing

| Stage | Time (Z) | Delta |
|---|---|---|
| Request (trajectory start, Texture loop) | 19:40:13 | 0 |
| Texture commit and hand-off | 19:40:27 | +14s |
| Management loop done, first Cast | 19:40:31.7 (cell intent), loop done 19:40:34 | +18s |
| Assignment opened, cast preflight | 19:40:32 | +19s |
| Capsule start | never (as of 20:04Z) | blocked 23+ min |
| Freeze, verdict | none | n/a |

The brief's "about 19:50Z" request time does not match the ledger
(trajectory created 19:40:13Z).

## 5. Scope

`self-dev patch` has no candidate to read. Scope check: not applicable.
Nothing outside `frontend/src/packages/slides-next/` can have been changed
because no capsule has run.

## 6. Hypotheses for tuning (labelled as such)

- H1: A `degraded` self-development operation that can never settle
  (pinned release unavailable, six days old) blocks every engineering
  admission on the computer indefinitely. The admission log retries every
  minute with no cap, no escalation to the requester, and no Texture-visible
  fate; Texture reads "waiting" while the cause sits in the console only.
  Candidates: surface the refusal on the tape/lifecycle (the b@c prior run
  in the brief saw a similar stale op); give degraded rollbacks with an
  unavailable pinned release a terminal "abandoned" fate; or do not let a
  degraded rollback hold the freeze lock.
- H2: The brief says the engineering desk is pinned to DeepSeek-V4.1-Flash,
  so the DeepSeek-versus-Claude comparison cannot start on d@e until H1 is
  cleared; d@e needs the stale op settled by an owner act before a re-cast
  is meaningful. Restarting does not clear it (19:58Z planned restart
  re-resumed the request into the same refusal).
- H3: Management and Texture both ran on `gpt-6-luna` here, not DeepSeek;
  the "default pool per thread" claim held for this thread only.
- H4: No empty round, refusal or dead letter on the Texture and management
  desks at this build; the first-run Cast suggests the three-run delay in the
  b@c run was model- or prompt-specific, not structural (single sample).

---

# Round 2 (resumed 22:42Z): candidate awaiting approval

Fix 91fb7c574 deployed; the coordinator abandoned
`rollback-4dca8039` at 22:41:42Z (state now `failed`, nothing open). That
restart (22:41:34Z) cancelled the old assignment `assignment-a484736a`
(`interrupted_by_restart`, capsule revoked) after 3h01m blocked.

Spec provenance: the coordinator sent revision `ef10b4da` (22:42:06Z) on the
same trajectory and document. It carries the cumulative corrected spec
(stage fill, title, thumbnails, present-mode contrast, app captures). This
arm therefore received the spec that b@c learned over 3 rounds; the two arms
are not comparable on spec learning.

## Outcome (A): candidate frozen and awaiting approval

Operation `selfdev-534017c7e5002466af83df914e6b7d50`, capsule
`capsule-9834ad35-a750-55c6-b591-11eb2c0ea09b`, assignment
`assignment-a6f6419c63338e1172ce0450` (completed), state `awaiting_approval`
at 23:00:06Z. Source patch base commit `91fb7c574`.

### Timings (Z)
| Stage | Time | Delta |
|---|---|---|
| Revision ef10b4da sent | 22:42:06 | 0 |
| Texture commit | 22:42:35 | +29s |
| Management run (1f063f62, one run) Cast | 22:42:36 to 22:42:41 | +30s |
| Assignment opened / bound, engineering loop started | 22:42:40 / 22:42:41 | +34s |
| Freeze requested / frozen | 22:59:58 / 22:59:58 (`choir.Freeze` call 22:59:35) | +17m52s |
| Revoked, reported | 23:00:00 | +17m54s |
| awaiting_approval | 23:00:06 | +18m00s |
Admission was immediate (no wait behind any operation). No stall: the
longest silent gap between tool results was about 40s.

### Desk table
- Texture: 2 runs this round side (`e499830c` 22:42:06 to :36, `bae61a70` 23:00:16 to :22 reacting to the report), model `gpt-6-luna`. One eval at 22:42:27 failed to compile (`expected type, found '}'`); the retry succeeded. No refusals.
- Management: 1 run, Cast on the first run, `gpt-6-luna`.
- Engineering: model served `deepseek-ai/DeepSeek-V4.1-Flash-1017` (94 events carry it). Network mode `ecosystem_proxy`, overlay filesystem. One engineering run, 92 `capsule_go_eval` tool calls: 51 `choir.Exec`, 5 `choir.Start` jobs, 3 `choir.Poll`, rest ReadFile/ListDir/WriteFile-class and Freeze; 3 calls errored (list_dir of a missing package dir at 22:42:49, an `Output` field that does not exist at 22:42:55, `import "os" refused: direct OS manipulation must route through capsule broker` at 22:54:26). Roughly 22:42 to 22:50 reading docs and Slides source, 22:50 to 22:57 trying to obtain a browser, 22:57 to 22:59 build and checks, then Freeze.

### Scope (item 5)
`self-dev patch` (36,888 bytes) touches only
`frontend/src/packages/slides-next/App.svelte` and
`frontend/src/packages/slides-next/choir-package.json`. No file outside the
package. `sha256_verified` true.

### Verify: did it capture the app itself? No.
- `self-dev visual` returns 404 `the candidate carries no visual evidence`.
- Host log at 22:41:32Z: `visual capture: this computer is not granted visual capture by the host (no grant on the credential disk)`.
- The engineering report says: `npm run build` (generate-source-contract --check, check-packages, vite build) exited 0; check-packages ok for 3 packages; an assertion cell checked manifest fields, unique theme attributes, no colour literals, token usage, geometry (168/136/44/72px), and that existing Slides is intact. "LIMITATION: no UI captures were produced": the capsule has no Chromium; `@sparticuz/chromium` 141 and 153 exited 133 (SIGTRAP). The frame share (1100x750, frame 900x506 in a 900x634 stage gutter) was computed from CSS, not pixel-measured.
- The engineer spent about 7 minutes (22:50 to 22:57, about 40 calls) trying to assemble a browser by hand (npm installs, ELF/library inspection, patching), which failed. Management's relay to Texture at 23:00:21Z: build and structural checks passed, browser-rendered validation unavailable.
- Judge: no independent-judge outcome is exposed on the operation; it carries `verifier_refs` `e4a597e7...` only. I did not see a "source patch without delegate" line in the ledger or console (search for delegate/judge/source patch returned nothing relevant after 22:41Z), so I cannot confirm the expected line.

## Head mismatch watch
The operation shows `desired_head fcddd72b... != effective_head ce1737e7...`
(same pair the coordinator reported). It did not block admission, freeze, or
the move to `awaiting_approval`. I never decide or apply, so whether a later
decide/apply refuses on it is untested; no refusal text exists to report.

## Hypotheses (labelled)
- H5: A DeepSeek engineer without visual capture and without a browser burns 7 of 18 minutes improvising a Chromium; a host-supplied browser in the capsule, or an early explicit "no browser, declare LIMITATION" instruction, would likely save that. The honest limitation report is a good sign for DeepSeek-V4.1-Flash.
- H6: Since this candidate has no capture, a visual judge (if enabled for it) will have nothing to judge, and approval would rest on the owner reading the limitation.
- H7: The compile error in Texture's 22:42:27 eval and the three engineer errors were all self-recovered; none cost a round.

---

# Round 2 apply: stuck in `materializing` (observed 23:03Z)

Coordinator approved `selfdev-534017c7e5002466af83df914e6b7d50` at 23:02:15Z.
Operation row at 23:03:31Z: `materializing`, desired_head `8c00c822`,
effective_head `ce1737e7`, no error field, updated 23:02:15Z.

Console (VM `vm-70070cf7...`), in order:
1. 23:02:15 `autoputer: restart kind=planned reason=self_development_apply target=selfdev-534017c7... marker_age_at_process_start=0s consumed_after=0s` (planned restart: yes, marker consumed once).
2. 23:02:15 `actorruntime: boot follows a planned restart (self_development_apply selfdev-534017c7...); pre-boot work resumes` and `work waits while selfdev-534017c7... is materializing (at most 10m0s)`.
3. 23:02:16 `source projection (boot): pending base= head=91fb7c574e39 booted=91fb7c574e39 ... operation selfdev-534017c7... is materializing; it settles when the operation records applied or failed; if it stays there (apply attempts spent, or degraded), the owner clears it with a self-development rollback`.
4. 23:02:17 (twice) `selfdev materializer: operation selfdev-534017c7... in materializing needs retry: guest credential: checkpoint refused with status 400: {"error":"checkpoint authority: verifier key is not the pinned computer key"}`.
5. 23:03:17 `runtime: refuse self_development_apply selfdev-534017c7...: guest apply attempts spent: this apply has already started the limit of times; an owner act is needed (self_development_apply selfdev-534017c7..., 3 starts)` and `selfdev materializer: operation ... in materializing held: guest apply attempts spent ...`.

Reading (from log lines, not a guess): the apply started 3 times within the
first minute; every start failed at the guest-credential checkpoint step
because the checkpoint authority did not accept the verifier key as the
pinned computer key; after the third start the materializer holds the
operation and asks for an owner act. No journal phase or fence line is logged
for this operation. Host journal shows no vmctl/checkpoint line for it.

Hypotheses (labelled): H8, the refusal is a key-pinning mismatch between the
guest credential and the checkpoint authority for this computer (cf. the
older desired `fcddd72b` != effective `ce1737e7` head: effective is unchanged
since before this round, so the unreconciled head may be why no pinned key
matches); not shown by logs. H9, three starts in under a minute exhaust the
attempt budget before any backoff could outlast a transient condition.
Commit 00548f21a (visual capture grant for d@e) applies only at the next
cold boot after deploy.
