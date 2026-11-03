# Arm rounds build for 50 minutes and then fail at Freeze; capsules cannot show real content (2026-11-03)

Status: open. Mutation class of this doc: green. Found by lane C (FP0) on the
first Files (reinvented) round on the four test arms. Evidence is metadata
from the CLI and the arms' ledgers, read by the Sonnet monitors.

## Problem 1: a computer whose self-development mode is "off" still admits engineering

All four stock arms (claude a875023a, deepseek e80d229b, luna 2bf3cab1, pool
4d38420c) had `self-dev mode get` = `off`, generation 0. Management Cast the
Files request on claude and deepseek at 03:15Z; each engineer read, wrote,
built and screenshotted for about 50 minutes; then `choir.Freeze` refused:
"self-development candidate: the computer's mode does not authorize a
proposal: mode is "off"" (claude 04:01:31Z and 04:02:04Z; deepseek
04:05:14Z, `internal/agentcore/freeze_candidate.go:40`). The capsules were
revoked, so the work was lost (engineering.carry_unfinished is off on the arms).

- The mode is consulted only at Freeze. Nothing at Cast or engineering
  admission (`internal/agentcore/engineering_admission.go`) checks it, so a
  round that cannot end in a proposal spends a full build first. Round 1
  cost about $1.0 (claude, plus $2.81 of delegate) and $1.4 (deepseek).
- `scripts/test-computer-create` sets neither the mode nor `files.object`;
  a fresh test computer cannot complete a self-development round.
- The refusal did reach Texture in prose (deepseek's turn at 04:07:03Z says to
  enable the mode and retry), which is right; the gap is the order of checks.

Recovery used: `choir self-dev mode set --mode propose_only
--expected-generation 0` on all four arms (04:07Z, generation 1), then an
owner revision on each document.

Fix direction (orange): check the mode at admission and refuse the Cast with
the reason on the commitment ledger, or let test-computer-create set
`propose_only` for test computers. Either one removes the class.

## Problem 2: the capsule cannot read the computer's files, so screenshots are not of real content

The spec ("Proof is real content") and the screens-as-files design ask the
engineer to screenshot the app showing the files on the computer. Both
engineers report that the capsule has no live `/api/files` and no view of
the owner's file tree (16 seeded files on each arm were invisible). They
served stand-ins: claude generated files with the same names; deepseek used
repository samples. So every Files screenshot is of stand-in content, which
is the fixture pattern the owner rejected, now produced by the engineer
instead of the platform.

Fix direction (red: capsule isolation, owner data): a read-only, per-round
grant of named owner paths (or a snapshot copy) mounted into the capsule,
recorded on the tape; or the screenshots move after apply, taken on the live
computer by the engineering desk outside the capsule.

Interim answer (main, 2026-11-03): stand-ins are acceptable inside the
capsule this round, but they are not the acceptance evidence. After a
candidate is applied on its arm, FP0 takes the real-content screenshots on
the live arm with a Playwright spec run from the Mac, signed in as the arm
with its key, showing the arm's 16 seeded files in the bar's presets and
sizes. FP0 decides on those, and they are the morning evidence. The red
design (a read-only grant of named owner paths into the capsule) waits for a
later decision. Problems 1 and 4: a builder is on branch
selfdev-mode-at-admission; the arms keep their manual settings.

## Problem 3: the computer's source tree predates docs the request cites

The capsule's tree is the computer's own `Source/platform`, written at its
creation (00:34Z). `docs/design/apps-feature-complete-spec-2026-11-03.md`
landed after it, so both engineers searched for it and worked from the
request text. Workaround: the round-2 revisions quote the spec sections
verbatim. This is by design (a computer's source is its own), but a request
that cites repo docs has no way to know which the arm holds.

## Problem 4: the Claude delegate cannot compile inside the capsule

On the claude arm the engineer started one delegate run at 03:24:21Z
(`choir-delegate`, 58 turns, about $2.81, 156,770 output tokens by its
report) and polled it 25 times. It reports the delegate's sandbox has no
bwrap, so the delegate could not run builds; the engineer built and fixed
everything itself. A delegate that cannot run `npm run build` writes code
blind. Cause is a hypothesis until the delegate's own log is read
(`internal/claudedelegate` notes a bubblewrap sandbox).

## Remaining error field

- Problem 1 class fix not landed; arms recovered by hand.
- Problem 2 open; FP0 judges round 2 on stand-in content and names it.
- Problem 4 hypothesis; check the delegate log on the next claude round.

## Owner ruling (2026-11-03 ~04:25Z)

On problems 2 and 3: "capsules should be an overlay over the exact computer
state"; and "computer source tree should auto update." A capsule's lower
layer is the computer's exact current state (its files, its source tree,
its data), read-only; the capsule's writes live in its own upper layer and
reach the computer only through Freeze, verify and apply. This supersedes
the interim answer above (live-arm screenshots after apply) once built, and
it supersedes "a read-only grant of named owner paths". The source tree
stops being a creation-time copy: the overlay's lower is the computer's
current source, kept current by source projection after every deploy. An
investigation of the gap (today's capsule root and mounts, the isolation
boundary that keeps owner data out, consistency of a live store under the
overlay, secrets that must stay out) is running; the design follows from it.

## Problem 5 (hypothesis): management's Cast to engineering drops parts of the request

Claude arm round 3 (assignment-74d314c1d1429249750d8abb). The engineer asked
management for the twelve-entry screenshot list at 04:43:51Z and again at
05:12:03Z. Management replied at 04:51Z that "the Texture request packet
available to management contains only the actions and omits the
filename-to-preset-to-view/state list". The engineer's report says management
then read the list from the Texture document's revision 6. If the Cast body
carries only the actions, every detailed acceptance list in a request (shots,
features, measurements) reaches engineering secondhand or not at all. Status:
checked against the Cast record, 05:45Z: **partially confirmed, and the cut
is one level up**. Texture's Cast to management
(`cell:bd00c647-...:cast:tray-2`, 04:41:57Z, body 4,976 bytes:
actions, summary, owner_revision, owner_directions) named only the round-3
revision 27c25e19 in `owner_revision`. So `attachCastOwnerDirections`
(internal/agentcore/texture_owner_directions_cast.go:136-165) attached only
that revision's 2,264-character note, which points at "the twelve screenshots
the request lists" without the list. The request revision 91b253a0, which
holds the list, was never named, and `ownerDirectionFromRevision` (:238-261)
takes a document body only for the first, parentless revision. Nothing was
truncated (castOwnerTextMax 64 KiB). Engineering sees the parent Cast verbatim
(engineering_cast_request.go:101-146), so it lacked what Texture's Cast
lacked. internal/textureprompts/texture.yaml:32 asks Texture to list the
owner's request and each later direction; this Cast listed only the newest.
Fix direction: always attach the request-kind revision of the trajectory,
or have Texture name every owner revision. Management's own engineering Cast
was not in the trace records (hypothesis that it carried the same).

## Problem 6: a Library release cannot carry the app-sdk re-exports its package needs

`choir package publish` for pool selfdev-1d0527916b182fb513039514188a874d
(05:35Z) answered 409 package_change_not_confined: the operation also changed
frontend/src/lib/app-sdk.ts. All three candidates so far added re-exports
there (the request allowed it), because a package may import only app-sdk and
the file routes, fetchWithRenewal and the registry were not exported. Main's
fix: the re-exports on main (branch app-sdk-exports), reaching the arms
through the ring walk; then each arm's next round is package-only and publishes.

## Problem 7: the luna arm's capsule had no browser

Luna round 2 (assignment-43851d28d33d373927c8fb8a, 05:36-05:39Z): the
engineer's Playwright launch with `executablePath=$CHOIR_BROWSER` failed
because `CHOIR_BROWSER` was unset and no Playwright cache existed
("Executable doesn't exist at /tmp/.cache/ms-playwright/..."). The claude,
deepseek and pool capsules took screenshots in the same hour. Hypothesis: the
browser tool layer did not attach on luna's capsule (or its env did not export
it); check vmctl's "tool layers:" line for luna's last launch. The engineer
froze a 12-line stub and the judge failed it ("no browser behavior was
exercised because Chromium was unavailable").

## Problem 8: Texture can absorb an owner revision without forwarding it

Luna arm, trajectory e998f5ca. The owner revision at 05:51:15Z (artifact_head_advanced) asked for a new build
after two judge failures. Texture's turn at 05:52:46Z wrote revision 61ac46cf (appagent) restating the fixes
("The owner fixed the browser path ... required the whole feature list before Freeze ...") and the activation
passivated; no co_super_assignment followed for 14 minutes. A resent, self-contained revision at 06:06:34Z that
said "forward this to management now" produced a Cast in 79 s (turn 06:07:53Z, assignment 06:08:01Z). Whether
the first turn emitted no control or emitted one that was dropped is a hypothesis until its controls[] are read;
the lifecycle events show only the turn and no assignment. Workaround: every FP0 revision names the forward
explicitly and carries the whole request (which also answers Problem 5).

Checked against the cells, 06:31Z (Sonnet trace read of luna e998f5ca and deepseek 8048fb68): **confirmed, three
times**. In each dropped turn (luna 05:52:46Z and 06:20:29Z, deepseek 06:27:24Z) the committing cell called only
choir.Write; its output carries `receipts:["rlm:texture_turn:0"]` and no `rlm:cast:N`. Every forwarding turn
(luna 06:07:53Z, deepseek 05:28:50Z) has `choir.Cast("management", ...)` in the cell and an `rlm:cast:0` receipt,
with the assignment 8-9 s later. In two dropped turns the model had just printed `choir.Help("Cast")`; in luna's
06:20:29Z turn the commit reason says "the short cast with the owner's revision is sent", which no cell did.
Nothing in the runtime requires a Cast (or a stated reason for none) when an owner revision asks for work. Even an
explicit "forward this to management now" in the revision does not make the cell call it (06:18Z and 06:25Z
revisions both said so). Hypothesis on why: the model treats a revision as an amendment to the document and
completes the turn on the Write. Fix direction (orange, Texture turn contract): a turn woken by an owner revision
on a trajectory with no live assignment must commit a Cast, an Ask, or a stated refusal; a commit reason that
claims a send must match an rlm:cast receipt. Workaround in use: a one-line follow-up revision that names
choir.Cast and the owner_revision id.

## Problem 9: engineering reads only the first 16 KiB of Texture's request, while Texture may carry 64 KiB

Luna arm, assignment-24a775b85a245c31ad635d9f (06:08Z). FP0's self-contained resend (15,064 bytes, written to
answer Problems 5 and 8) became a Texture Cast whose body was 20,923 bytes. Engineering saw "[... truncated:
showing 16384 of 20923 bytes of the Texture request's body (record cell:36ba0624-...:cast:tray-1). Bytes
16384-20923 are not readable from any desk; do not guess them ...]", reported that to management ("Texture's
carried request Cast arrived truncated at 16,384 bytes ... Working from visible requirements pending it"),
froze what it had, and the judge failed selfdev-6038b25b1a45521496562f6f8924b46a (06:12:41Z: "explicitly
partial, lacks required feature behavior and browser validation"). The cap is
`engineeringRequestBodyMax = 16 << 10` (internal/agentcore/engineering_cast_request.go:35-36); Texture's
`castOwnerTextMax` is 64 KiB, so a request Texture accepts can be cut one hop later. The truncation notice works
as designed (engineering asked for a shorter resend and did not guess), but the round still spends a capsule
and a judge on a partial build. Texture answered by rewriting the request to fit (06:11:58Z, revision 307df8dc,
pointing at the repo spec files).
Fix direction (orange): one cap for the whole carry (Texture refuses or splits a Cast its engineering hop
would cut), or raise the engineering cap to Texture's. Workaround: FP0 revisions stay under about 8 KB and point
at the spec in the source tree (projection is on, so the spec is there).

## Problem 10: stack_base_moved after a backward boot leaves an applied arm unable to build, and no rebase notice drives it

Claude arm (a875023a). Round 3 (selfdev-40faf6ee) applied at 05:35Z on a stack based on 4036f0c37a84. At the
06:07:49Z boot the guest logged: "source projection (boot): stack_base_moved base=4036f0c37a84
head=ff9070317d9a booted=2795b8ee6881 op=selfdev-40faf6ee056fec39eb534c07226ffb95 changed=false repaired=false
diverged=true ... that commit is not in the guest source repository, so the stack cannot be shown to be
upstreamed"; 06:08:01Z "source base fetch (attempt 1): fetched commit=2795b8ee6881". The computer booted an
older platform than its stack's base (2795b8ee is the pre-walk platform; staging serves fe981c3dd), so the base
moved backward. FP0's round-4 revision (06:13Z) was refused at engineering admission twice on the base mismatch
before any capsule opened (Texture 06:19:22Z). Texture asked management to rebase (06:19:08Z); management did not
Cast: "the request packet lacks the required exact rebase_notice identifier ... I did not cast engineering because
that cast would be refused without the exact notice", and found no notice in its inbox and no read route for one
(06:23:36Z). The U4 timer (internal/agentcore/rebase_notice.go) should mint a notice to the root after a 5-minute
stay-up (about 06:12:49Z); none reached management by 06:23Z.
Hypothesis (not confirmed): U4 mints no notice when the stack diverged (the booted base is behind the stack's base,
"cannot be shown to be upstreamed"), so the arm waits until the ring walk boots a platform that contains
4036f0c3. Status: claude parked; FP0 does not invent a notice id or roll back the applied round.
Fix direction (red, source projection): a backward boot either mints a notice the root can carry, or admission
builds on the computer's effective tree (base plus applied stack) instead of refusing.

## Problem 11: once one applied round touched app-sdk.ts, no later round of that package can publish

Pool arm. Round 2 (selfdev-c4ac7712467862d5b7cd527ed6287034, applied 06:43:14Z) changed only package files
against the state round 1 left, so confinement passed. `choir package publish --package files-next --operation
selfdev-c4ac7712...` (06:43Z) answered 409: "package_parent_unpublished: frontend/src/packages/files-next before
this operation is 8ff6cbc9... (from operation selfdev-1d0527916b182fb513039514188a874d), which is neither the
base's 4b825dc6... nor the track head's". DraftPackage check 2 (internal/pkgrelease/publisher.go, "own change
only") requires the package subtree before the operation to be the base's or the published track head's. Round 1
can never publish (it changed app-sdk.ts, Problem 6), so every round stacked on it inherits an unpublished
parent. The same holds on claude (round 3 changed app-sdk.ts) and deepseek (round 3 did). Main's re-exports on
main do not unblock these stacks; they only let a fresh stack start package-only.
Ways out: (a) the publisher accepts a parent that is an unpublished operation in the same applied stack and
publishes the cumulative package tree (orange, publisher); (b) per arm, roll back to the state before the first
files-next round (`choir self-dev rollback`) once the re-exports are on the arm's base, then build one
package-only round from the base and publish it (an owner act on a test arm; loses nothing the next round does
not rebuild, but costs a round). FP0 has not rolled back any arm; the choice is main's.

## Problem 12: under standing_consensus, every candidate frozen during a deploy window is rejected, and the reason is invisible

The arms became test tier (main, about 06:55Z); FP0 armed `standing_consensus` (policy reversible-selfdev-v1,
digest c34ddf07..., expires 2026-11-04T06:00Z) on all four at 07:01-07:02Z (pool generation 6, claude 4, deepseek
4, luna 2). Deepseek round 4 froze selfdev-4ee90af97baf3ce7093445718aa3da8e at 07:02:42Z: a package-only candidate
(the engineer: "exactly three files changed, all inside frontend/src/packages/files-next/"). At 07:05:10Z it was
`rejected`, decision_actor `consensus-dissent:3813a663...`. Neither `self-dev show` nor the trajectory says why,
and Texture's next turn (07:06:32Z) reported the candidate as "verified and frozen". Node B's corpusd log (read
once by FP0, 07:06Z) has the reason: "consensus: computer=computer-e80d229b... operation=selfdev-4ee90af9...
dissent: decision policy: unresolved policy-blocking dissent: host build: base_moved: the computer booted base
516ec805...; the host's deployed guest image is 235acb42... (a deploy landed since the computer booted)".
So the host-build seat dissents for any candidate on a computer that has not yet refreshed to the newest deploy;
with ring walks taking tens of minutes, every round that freezes in that window is thrown away, with its
capsule time. Two gaps: (1) the dissent should defer (wait for the refresh, or build on the booted base) rather
than reject a candidate whose own content is fine; (2) the rejection reason belongs on the operation and in the
trajectory, so Texture and the owner can see it without SSH.
FP0 response: luna returned to propose_only at 07:07Z (generation 3) to protect its in-flight round; the other
arms keep standing_consensus and get their next round only after their refresh to 29d47c215.

Re-check after the 29d47c215 refresh (07:15Z, main's ask): publish of every applied files-next candidate is still
refused. pool selfdev-c4ac7712 package_parent_unpublished (parent selfdev-1d052791); pool selfdev-1d052791,
claude selfdev-40faf6ee and deepseek selfdev-574077b3 package_change_not_confined (app-sdk.ts; deepseek also two
specs under frontend/tests/files-next/). The re-exports on the base let new rounds be package-only, but each new
round stacks on an unpublished parent, so Problem 11 still blocks them. All three stacked arms logged
stack_base_moved again at the 07:07-07:08Z boot (booted=29d47c215b96, "not in the guest source repository"),
with the base fetched 8-10 s later.

Problem 10 widens after the 29d47c215 refresh (07:15-07:20Z): every arm with an applied files-next stack is now
refused at engineering admission. Pool round 3 (07:14:30Z): management's report, "the Cast was refused before an
assignment was opened. The current computer is on release 29d47c215 while the existing stack is based on
2795b8ee; its patch is not in the booted base and touches frontend/src/lib/app-sdk.ts, so the platform requires a
root-desk rebase that is unavailable in this run." Claude has been parked on the same gate since 06:19Z; deepseek
is expected to hit it. Luna (no applied stack) is not affected. The owner rollback does not help: it returns to
the state an applied operation left and refuses the genesis baseline, so it cannot remove the first files-next
round (each arm's first applied operation is a files-next round). So the stacked arms wait on the platform: a
rebase notice that reaches the root (U4), a graft by source auto-update, or a way to start a package-only round
on the base with the stack set aside. FP0 keeps their next requests queued.

## Problem 14: a rebase cannot drop a hunk the new base already carries (claude, 11:15Z)

After eb1fb6408 the claude arm minted its rebase notice (10:52:48Z) and management opened the rebase assignment
(10:53:00Z). The rebase operation selfdev-22655878c3170572dd841124b2538aa4 (created 11:07:32Z) ended failed at
11:15:12Z: "assignment failed before its candidate froze; last refusal: freeze: rebase_content_mismatch:
hunk_missing: paths of the applied change that conflict with the new base lack a hunk of that change, or were
deleted or edited elsewhere in the path: frontend/src/lib/app-sdk.ts". Claude's round 3 added the app-sdk
re-exports; eb1fb6408 carries the same re-exports from main (e5915e0ca), so the hunk conflicts with the base and
the right rebase drops it as already upstream. The Freeze content check requires every conflicting hunk to
survive, so the rebase can never freeze. Deepseek's rebase (selfdev-152fe036bf575df9af28820e3132bae7, applied
11:02:09Z) passed because its app-sdk hunk was only a reorder. Fix direction (red, rebase content check): accept
a missing hunk when the base already holds that hunk's resulting content (upstreamed), and say so on the record.
Also seen: deepseek's rebase operation changes app-sdk.ts and two test specs, so it is not confined itself; the
arm publishes from its next package-only round (Fix 11 publishes the stack's cumulative package then).
Both ways out are closed for claude: Texture's 11:20:55Z turn reads "the already-landed app-sdk hunk blocks it
and the merged duplicate exports fail the build" (keeping the hunk duplicates the exports and breaks npm run
build; dropping it trips hunk_missing). The arm cannot build again until the content check is fixed.

## Problem 15: luna's raised engineering reasoning reaches the provider as a malformed value (11:23Z)

Luna round 7 (doc 789f95a0, trajectory e52877df). Both engineering assignments were cancelled at tool-loop
iteration 3: assignment-35bbb08249a0557c9fc83f56 (11:22:35-11:23:37Z) and the next (11:27:38-11:28:40Z), each
"gateway call failed: gateway client: chatgpt: status 400 Bad Request (sanitized)". Node B's gateway log:
"provider: chatgpt rejected request status=400 reason="invalid_request_error Invalid value: 'hig...03)'.
Supported values are: 'none', 'minimal', 'low', 'medium', 'high', 'xhigh', and 'max'."" (11:28:22Z, 11:28:38Z,
11:28:40Z, vm-774a03f8...). The effort string sent for engineering is "high" plus trailing text ending "03)",
apparently a note such as "(owner, 2026-11-03)" carried into the value. Management calls on the same arm log
reasoning=high and succeed, so only the engineering desk's setting is malformed. Fix: set the engineering
effort to the bare value "high" (and validate effort values against the provider's list when set).

## Problem 16: pool's rebase notice reached management under a mangled id, admission refused both forms, and pool cannot build again

Pool minted rebase-notice:selfdev-c4ac7712467862d5b7cd527ed6287034:eb1fb6408ab8...:1 at 10:45:39Z (deployed proof
doc). Texture dispatched management at 10:45:41Z. Management's report (pool trajectory 5dc6a6ff, 10:50:16Z): "The
request names rebase-notice:selfdev-rebase:selfdev-c4ac7712467862d5b7cd527ed6287034:eb1fb6408ab8437ff01053b93c0681818d0c2479:1,
and engineering admission refuses that string as not a rebase notice id; the short id quoted in the same request is
refused instead for not being carried by the request. Either version is refused, so no rebase assignment opened".
The id gained a "selfdev-rebase:" segment between the notice's mint and the Cast (claude's and deepseek's notices,
minted 10:52Z, reached engineering). With the stack not rebased, pool's next request (Slides, 11:29:36Z) stops at
Texture: "cannot proceed until a live root or owner initiates the required platform rebase; no engineering work
is in flight" (11:31:03Z). Whether U4 re-mints after its 30-minute answer window is not yet seen.
Fix direction: carry the notice id verbatim from the notice record into the Cast (or have admission resolve the
notice by operation and base), and let a refused rebase re-mint promptly.

## Problem 17: deepseek's Texture livelocks on an owner revision after resuming from hibernate (11:21Z onward)

Deepseek resumed from a hibernate snapshot at 11:19:39Z ("agentcore: resumed from hibernate snapshot; clock stepped
576108ms"). FP0's round-6 revision (11:20:59Z) and a nudge (11:33:54Z) on document ddbdd1ae (trajectory 8048fb68)
produced no Texture turn: the work item stays open, the activation reads passivated, and the guest log repeats,
every 5-7 s since 11:21:40Z, "dispatcher: deferred ...texture:ddbdd1ae-.../<id> deferrals=N cause=actor: defer
unprocessed occurrence: actorruntime: Texture activation returned without disposing exact trigger" and "...discards
1 emitted update(s)", alternating between two occurrence ids, each followed by "tool loop: iteration 1, executed 1
tools, continuing" (73 such loop lines by 11:39Z). The revision's trigger is never disposed, so Texture re-runs and
throws away its own output indefinitely. Hypothesis: two occurrences for the one agent (one per revision) and the
activation disposes neither exactly. Workaround tried: a new document for round 6 (below).
Recurrence (11:44Z): main restarted deepseek (an owner restart; the 11:40:16Z document's trajectory was cancelled and
the two old updates dead-lettered at 11:40:39Z and 11:40:48Z). FP0 re-sent round 6 as a brand-new document
799ed6d2-b8aa-5c2f-b498-51206b850e72 (trajectory 6be571d8, 11:43:35Z). From 11:44:12Z the same loop: "deferred
...texture:799ed6d2-.../e979af8f-441d-5850-aa03-24a181a7a571 deferrals=N cause=... Texture activation returned
without disposing exact trigger" and "discards 1 emitted update(s) ids=[fd70fac3-...]", deferrals=14 by 11:49:40Z
(about every 35 s), no turn committed. One occurrence, a fresh document and no hibernate: the hypothesis of two
occurrences is wrong; Texture's activation on this arm never disposes its trigger. FP0 stopped sending to deepseek
and told main (11:50Z).

## Problem 18: scripts/choir-shot.mjs and the theme spec cannot resolve their dependencies in the capsule (luna, 11:42-11:48Z)

Luna round 7b, assignment-eb38b054067ce9086841d23a (11:42:40-11:48:20Z), the first luna engineer to reach the
browser step, completed blocked: "choir-shot's initial Vite build failed exactly because it could not resolve
@chenglou/pretext (the dependency exists in the read-only Nix node_modules, but the shot helper's build did not
resolve it); Playwright theme spec invocation with --grep files-next reported 'Error: No tests found' because the
spec has one whole-frontend test and no package-scoping test". The assignment before it
(assignment-7fc486680d52f4095cec7270) hit "Error [ERR_MODULE_NOT_FOUND]: Cannot find package '@playwright/test'
imported from /workspace/platform/frontend/tests/theme-contract.spec.js". So the screenshot helper main shipped for
rounds (choir-shot.mjs) does not build inside a capsule whose node_modules come from $CHOIR_FRONTEND_DEPS, and the
theme spec cannot be scoped to one package. The other arms' engineers wrote their own Playwright harnesses and did
not use choir-shot. This revises the luna verdict: luna's last round was stopped by the capsule's tooling, not only
by the engineer. Fix direction: resolve choir-shot's Vite build and the specs against $CHOIR_FRONTEND_DEPS/node_modules
(NODE_PATH or a symlinked node_modules), and give theme-contract.spec.js a package filter.

## Problem 21: after 0de7c5865, pool's U4 timer mints no rebase notice ("rebase_needs_root")

Pool refreshed to 0de7c5865 at 12:58Z ("source base fetch (attempt 1): fetched commit=0de7c5865d0c" 12:58:05Z). At
13:03:04Z, the stay-up mark, the guest logged "rebase notice: none minted: rebase_needs_root: no live root desk to Cast the
rebase". 524a94bad says a computer with no live root desk sends the notice to the stack operation's own document Texture
(pool's is b1ad3952, trajectory 5dc6a6ff: live, activation passivated, last turn 10:50:16Z). At 10:45Z the same arm minted
a notice to that Texture. So the fallback did not take on this boot, and pool's Slides round (doc 023c9d49) still cannot
build. Shift tried: an owner revision on b1ad3952 (13:08:33Z) to wake that Texture; it woke and waited (13:08:37Z).
13:16:52Z the guest minted "rebase-notice:selfdev-c4ac7712467862d5b7cd527ed6287034:3ab1d5c3342f2f5ebbf5b98f1913dc4a8ecdf6af:1
(attempt 1)", on base 3ab1d5c3 rather than 0de7c5865, so pool likely booted a newer release first and this was a fresh
stay-up mint with the stack's Texture live. Texture cast it at 13:16:56Z. Whether the live Texture or the reboot made the
difference is not separable from this evidence.
After 781fcfc46 (the fix: the notice goes to the operation's document Texture as unread pressure), pool rebooted 13:54:07Z
and at 13:58:37Z logged "rebase notice: none minted: rebase_needs_root: no live root desk to Cast the rebase, and the applied
operation's document Texture cannot take it: its trajectory 5dc6a6ff-41d6-5ee6-9025-151c57bb5eff binds no live document
(none, archived, or bound to another trajectory)". The end-to-end proof failed: the fallback looks up the operation's
original trajectory, while document b1ad3952 has since moved to newer trajectories (owner revisions at 13:08Z and 13:40Z).
Reported to main 14:01Z.

## Problem 22: a refresh lands while a candidate awaits its decision, and the candidate dies on booted_base_moved (pool, 13:33Z)

Pool's rebase (notice minted 13:16:52Z on base 3ab1d5c3; assignment 13:17:02Z) froze selfdev-ab2912d3185c40206a80bd4d33bc76c9
at 13:29:02Z; it reached awaiting_approval at 13:32:24Z although the arm is on standing_consensus (generation 6; no consensus
decision came in the 3+ minutes). 2cd70f606 deployed at 13:21:52Z; pool was rebooted for it at about 13:32:55Z (proxy:
"computer ... booting was not woken"), between the freeze and FP0's approval. FP0 approved at 13:33:25Z (receipt
01a1a19d-5691-7ddc-9f28-d93f369c77b2, accepted), and the operation failed the same second: "rebase_content_mismatch:
booted_base_moved: the computer booted a newer base after this rebase was claimed; the next base gets its own notice". The
idle-refresh rule (no capsule live, no apply in flight) does not count a frozen candidate awaiting its decision as work, so
a rebase that took 16 minutes is thrown away by the next deploy, and with expedite deploys every 30-60 minutes the stacked
arms can loop: notice, rebase, refresh, notice. Fix direction: hold the refresh while an operation is frozen, awaiting
approval or accepted (or let a rebase finish onto the base it claimed and re-notice after); and say why standing_consensus
left it at awaiting_approval.

## Problem 23: a stack that once touched files outside its package cannot be made publishable (deepseek, 14:21Z)

Deepseek's Files round 7 (selfdev-0edfc337c6eff5455e4b938622c8a178, built 13:26-14:14Z, FP0-approved 14:21:08Z, applied
14:21:10Z) was asked to be package-only. `choir package publish --package files-next` answered 409: "package_change_not_confined:
the operation changed paths outside frontend/src/packages/files-next: frontend/src/lib/app-sdk.ts,
frontend/tests/files-next/file-kinds.spec.js, frontend/tests/files-next/files-next-ui.spec.js". The stack under it (rounds 3 and
the 11:02Z rebase) already carried an app-sdk.ts edit and two specs outside the package; this round changed them again (its
cumulative app-sdk hunk adds isOwnLiveEvent and APP_REGISTRY exports and reorders rememberRecentMedia). Even a round that only
reverts those paths to the base changes paths outside the package, so the publisher's check 1 (the operation's own change is
confined) refuses every cleanup too. Ways out: let check 1 accept outside paths whose result equals the base (net zero), or
let the owner publish the package subtree of an applied stack whatever its outside paths, recording them as left behind.
Interim: the next deepseek request asks for app-sdk.ts and frontend/tests/files-next/ to be restored to the base exactly.

## Unnumbered (lane C, 16:15Z): pool refreshed twice after its rebase applied, and no rebase notice reached it

Evidence, metadata only. Pool's rebase selfdev-5dbd9ba2 onto 059763bb applied at 15:39:41Z, on live document trajectory
a6b8f87c. vmctl then refreshed pool's VM vm-04a4e430 twice, both with planned_restart=true:
- 15:47:12Z to 6e4658891 (retry-sweep);
- 16:01:42Z to 52e05f16c (deploy).
At 16:15Z, 28 and 13 minutes after those boots, there is no notice. Pool has no run created after 15:29Z, no new self-dev
operation, and no event on a6b8f87c after its 15:46:44Z turn, which is still live. The notice usually comes about 306 s
after boot. So the base moved twice under an applied stack and U4 minted no notice, or the notice never reached the
trajectory. Not yet checked: the guest-side U4 log line, which separates "no notice minted" from "minted but not
delivered". Lane C's rounds also showed a deepseek build cancelled at exactly 60:00 after it bound ("context deadline
exceeded", assignment-17a659c2, 14:42:27Z to 15:42:27Z), which bounds every rebase.
