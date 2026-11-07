# The modular architecture mission, so far

I wrote this on Saturday, November seventh. It covers the modular architecture mission from the evening of Wednesday, November fourth, when you started it, through Saturday morning: about two and a half days of work.

Most of the new architecture is now running on staging, but the last day went the wrong way before it went the right way, and that is where your review should start.

Here is where things stand. Seven of the plan's nine phases are closed or mostly landed. Messages between desks now travel only as precommitment records. The kernels are small and checked, and every desk is registered the same way with powers that come only from grants. Custody holds per kind of data. Desks run in gVisor sandboxes. The trust model that scores desks is wired live. Self-development restarted in a reduced form on Thursday, and on Friday night I let it grow a platform screenshot service that you rightly called a break from the desk pattern. Screenshots and their review are now management's own acts on its desk. That change is live, and the proof on the test computer found two more defects in it, both fixed. What is not yet proven is a platform change that passes its screenshots and applies; the test computer has spent its daily build allowance, and the next slot opens tonight. What I would ask of you is a review in the order the last chapter lays out, starting with the desk shots design.

## Wednesday evening: the mission begins

You started this mission at about five in the afternoon on Wednesday, with the words "This is a whole mission here. Let's supersede the current metamission and make this a new throughline." The plan had nine phases, from evidence gathering through to restarting self-development on the new base. Devin accepted the plan and the goal file, and you signed off on it.

By midnight the first four phases were moving fast. The old emit and message channels between desks were deleted outright, more than sixteen hundred lines of Go. In their place, an address resolver decides where a record may go, and a request record carries every ask. The Go kernel and the front-end kernel were both cut down and checked. Manifest version two landed, and the desk registry became the one place that says what a desk is and what it may do.

One thing broke early. After manifest version two, the nineteen packages published under the first manifest format stopped being served, so the Library catalog is empty. That is still open and still yours to decide: republish them or accept the loss.

## Thursday: records, custody and the first desks as packages

Thursday was the longest day, about a hundred and fifty commits.

The first phase closed when the protocol lock landed. Every desk-facing verb, field and grant is now recorded in a lock file, and a change to that surface must raise the protocol version. It began at version one and stands at thirteen today. Sub-research casts and their judge landed. So did a fix for a final report that could strand an unread cast. The test computer settled a three-round conversation in about two and a half minutes after that fix.

Then custody. Each kind of data now has one custodian desk. Every write to the object graph names its writer, and every read names its reader. Leases grant one desk access to another's data for a while, and provenance labels follow data as it moves. A disclosure gate watches where data leaves a desk. Package kinds are enforced. The built-in kinds are only counted for now: flipping them from counting to enforcing has not happened, and it has prerequisites I list at the end.

The fourth phase began the same day. Grants became the only source of a desk's powers. Research, the mail desk and the prompt router became packages, and management became a package whose special powers are two grants with exactly one holder each. The mail desk broke CI on its first try, because two branches that were each green collided on a name. I reverted it and landed it again later the same night. That is the only revert in the mission.

Two things outside the code shaped the day. Provider quota ran out upstream, at ChatGPT and at Synthetic, and acceptance runs failed with "every model is out of quota." The paper was hit hardest. And Node A's disk filled to one hundred percent, which stopped every deploy. You told me to prune it. Deleting fifty-nine old workstation snapshots and collecting garbage freed about six hundred gigabytes. You also moved the paper onto the free-model pool, ruled that Keenable search alone is enough for now, and ruled that desk-to-desk casts are judged by the System 1 models too.

One lesson from Thursday is about my own instruments. My polling of the test computer every sixty seconds kept it marked active, and that held back its deploy refresh. The rule now is never to poll it more often than every three minutes.

## Friday: isolation, packages, and self-development returns

On Friday every desk worker began running inside gVisor, on the x86 Firecracker hosts as well as on the Mac. Yaegi, the Go interpreter that runs desk cells, now reads only the modules a desk's grants allow, and each desk has its own set of imports. A cell's host calls are bounded and served a few at a time. I measured the cost of a cell on the test computer at well under a millisecond at the median. Fuzzing the desk egress found four real holes, among them a path that was checked before it was cleaned and a capture read that did not taint the run, and all four are fixed.

Texture became a package, and the boot migration that moves a computer onto packages passed a five-step proof on the test computer, including a rollback that held across a reboot. Review caught three real defects in that migration before it landed: a stale rollback, a transient fault that could wedge it, and a confusion between unknown and mismatched states.

Then self-development came back, in a reduced form, ahead of the plan's eighth phase. You ruled that "the point is to revert rather than approve," for every self-development change except the paper's. So a candidate now applies without an owner approval, and the owner's act is the revert. A package change on the test computer went from frozen to applied in about seventy-five seconds with no owner act at all. You also ruled that a platform candidate's screenshots are taken after the host build, with a clean screenshot function and video if resources allow. The first slice of that, a source-only freeze plus candidate screenshots, landed that evening.

That plan held for packages but not for platform changes. Platform changes could not freeze on the test computer at all until the source-only freeze landed. A base-package edit still cannot publish, which is an open design question.

## Friday night into Saturday: the wrong turn

To prove a platform change end to end, I ran the same one-line change on the test computer four times. Each run found a new failure in the screenshot path, and I fixed each one where it appeared. The first run could not reach the staged frontend because of directory permissions. The second hit a Unix socket path that was too long for Chromium. The third failed because the one free vision model it called had run out of quota, so I gave that review call its own model failover.

That is when you asked why screenshot review needed its own failover, and whether it was running apart from the desks. It was. I had let a platform screenshot service grow inside the materializer: it staged builds, ran the browser and called a vision model directly, and its verdict decided the swap. You said: "If we've left the desk pattern behind, we need to use the desk pattern. There shouldn't be some platform level screenshot service. That's not right." You were right. Three fixes in a row on one path were symptoms, and I should have stopped after the second and asked what the path was. The repo's own rule says so, and I did not apply it.

## Saturday: screenshots become management's acts

I stopped work on that path and audited every place the platform calls a model or decides an outcome outside a desk. There are sixteen. Three sit on the self-development apply path, eleven are host judges that desks cannot call, one is the install review, and one is the trust model's anchor sampler, which I had wired live on Saturday morning. The audit is in the design document. Only the three on the apply path are fixed so far.

The new design makes management do the screenshot work itself, with verbs on its own desk: start the shots, wait for them, look at them, and record a verdict. The platform keeps only custody: the host build, the sandbox that runs the browser, and the swap. Management's verdict is an event on the tape. The swap reads that event and nothing else.

Two reviewers sent the first draft back, and then Devin sent the second back. Together they found nine blocking problems in the design. The worst three were these. Nothing would ever close a change if management never answered. The request to management fired only once. And a planned restart would have left the screenshot job running forever. The third draft fixed all of them. There is now a forty-five minute deadline, repeating reminders, a check that management's model can see images at all, and a cap of twenty images per request.

I built it in two slices. The first teaches the tool loop to route a turn that carries images only to models that can see them. The second is the verbs themselves, and it deletes the old screenshot worker and the hidden image review inside Verify. Devin and a Sonnet reviewer found two blocking defects in the build: an image count that could still exceed the cap, and a missing test for package changes that carry screenshots. Both are fixed. After landing, CI caught two more things I had not checked: hard-coded desk names that the registry gate refuses, and a data race in one test.

Then the proof on the test computer, which is where the real defects showed. The first run went all the way through: management started the shots, saw them, and judged them. It failed the change, for two reasons that were defects in its instructions. An old paragraph listed the wrong desktop sizes, and nothing told it that a change with nothing visible leaves the screenshots unchanged. I fixed the instructions. The second run found something worse. Management could not build the findings a pass verdict needs, because the verb took a Go type that a desk cell cannot construct. Only a fail could ever be recorded. The builder's tests had built findings in Go, never inside a cell. The fix takes plain maps, and a test that runs a real cell now guards it.

That fix is live, but the pass path is still unproven. My third run on the test computer was refused before the screenshots, because that computer had used its allowance of eight host builds in a day, and every attempt counts, the failed ones too. I tried the other test computer, b@c, and it refused to open the round. Its applied stack still carries package changes from November third, made before the package decision, and a platform round will not stack on them. Cleaning that up is a decision about b@c's state, and I have not made it. The first build slot on the test computer opens tonight at a quarter to seven Eastern, and I will run the proof then: a one-line change that should pass its screenshots, apply, and then revert.

So here is exactly what the test computer has shown. Management started the shots, saw the images in its own run, and recorded a verdict. The swap read that verdict, and when it was a fail, the change closed failed with management's reason. A pass that applies is the part still to show.

One more thing happened on the test computer along the way. A change sat in its materializing step for more than half an hour and held up its deploy. The cause was the verifier key problem from November second. The test computer lost its data disk in a drill on October thirtieth and made a new key, and the host still pinned the old one. Nobody had re-pinned it. I abandoned the stuck change and re-pinned the key on the test computer only. The hold that caused the wedge has no deadline, and that is still open.

## What is still open

The biggest open items, roughly in the order I would take them:

The fifteen platform sites from the audit that are not on the apply path still call models outside any desk. The next design gives desks a judge verb they can call, so those decisions become desk acts too. The trust model's anchor sampler is one of them, and I wired it live on Saturday morning before the desk pattern ruling, so it may want converting or reverting.

The built-in kinds are still counted rather than enforced by the writer, reader and disclosure gates.

The materializer can hold forever on an unpinned verifier key, and that blocks a computer's updates.

The engineering desk's conversion to a package, the last two slices of phase four, is built but not landed. Devin found a replay problem in it, the builder fixed it, and Devin is re-reviewing the fix now. The eighth phase, restarting self-development properly on the new base, has not started.

The Library catalog is empty until the old packages are republished.

The DeepSeek review seat has been out of funds since Thursday. A Sonnet seat has taken its place.

The paper's owner acts depend on a saved test session, and the paper's key expires on November twelfth.

One CI test, a browser start in the shot package, failed twice on Saturday on the runner and passed on rerun.

The host build allowance counts failed and abandoned candidates, so a few honest failures can stop a computer's self-development for a day. And b@c's applied stack needs cleaning before it can take a platform round.

## How to review this, in order

If I were holding the phone, I would review in this order.

First, the desk shots design, because it is the newest architecture and it answers your ruling. Read the problem section and the review section at the end, which records every finding and how it was resolved.

Second, the audit summary at the end of that design, and decide whether the anchor sampler should be reverted now or converted with the other fourteen sites.

Third, the mission ledger for Saturday, which records the wrong turn and the defects the proof found.

Fourth, the earlier phases, through the ledger's entries for Wednesday to Friday. Each landed slice names its commit and its review.

Then the open decisions that are yours: the empty Library catalog, the built-in gate flip, and funding for the DeepSeek seat.

The one thing worth remembering is this. The fastest path on Friday night was the wrong one, and the repo's own rule of stopping after repeated symptom fixes would have caught it a day earlier. Since your ruling, the design went through three review rounds and the build through two, and the test computer still found two defects that no reviewer did. That is the case for running the real proof early.

## Names and receipts

Mission goal file: docs/definitions/choir-modular-architecture-2026-11-04.md. Plan: docs/design/modular-architecture-plan-2026-11-04.md. Ledger: docs/definitions/choir-modular-architecture-2026-11-04.ledger.md.

Desk shots design: docs/design/desk-shots-and-review-2026-11-07.md, revision 2, with the audit in section 8 and the review record in section 7.

Desk shots commits on main: DS1 9f4bc41a5 and 7a46ded68; DS2 9ff269597, d1c20abaa and de2e66c45; CI fix e845237f6; instruction fix 1a8585726; findings fix bdbf996aa (Ring-Expedite). Staging and the test computer serve bdbf996aa.

Expedited releases on Saturday: bdbf996aa (no platform candidate could pass its shot verdict).

Proof runs on the test computer (d@e, computer-59c902c5d46caecc0a180a8557164e72): selfdev-99fbb87c (failed on the instruction defects), selfdev-59f177ab and selfdev-5333d330 (abandoned on the findings defect). selfdev-dd6b172a (refused at the host build quota). On b@c, trajectory e07d4389 (refused at the engineering cast: stack_package_path). The pass proof waits for the first d@e build slot at 23:46 UTC on November seventh.

Verifier key re-pin on d@e: old pin verifier-control-22033e8312994d59, new pin verifier-control-470ec38805795ff9, tape event 699b55ac. Abandoned operation selfdev-ec4dfc297b54b56e7655435cce14a9bf. Problem record: docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md.

Earlier Saturday landings: MA7.10 anchor sampler live 0047d0892; verify failover 7dee1c474; L1 shot path fixes fb9486ab9 and bf3bb7820 (superseded by DS2).

Numbers: 420 commits on main since November fourth. Security epoch 41 to 96. Desk protocol 1.0.0 to 13.0.0. One revert (the mail desk, 819e36492). About 39 expedited commits over the mission.

Not landed: MA4.10 on branch ma410 (worktree go-choir-ma410), MA4.11, MA6.7, Phase 8.
