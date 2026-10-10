# Gate 1 and Gate 2, later Friday evening

Written late Friday evening, October ninth, Eastern time, about three hours after the first checkpoint letter. It covers only those three hours of autonomous work, and it adds to that letter rather than replacing it.

The Texture acceptance suite now passes every check except the one waiting on your Cancel decision, and a self-development change reached approval and began to apply for the first time since the S2 layering work.

## Where things stand

Gate 1's suite ran green on its tenth run, apart from revise-after-cancel. A first draft took forty-six seconds, a revise twenty-one. A research request produced a revision citing a real web source in about three minutes, and the document then went quiet. A crash interrupts work and never resumes it, and nothing is owed afterwards.

Gate 2's probe now gets through building, freezing, independent verification, and approval. It stops inside apply. The apply step takes a checkpoint, and that checkpoint can't yet reproduce the computer's live state from its event history. I have written a diagnostic that will name the rows involved; it goes out as soon as the current probe finishes.

Your three asks from the first letter still stand: rotate the SerpAPI key, decide Cancel, and decide when research is finished.

## Research citations

The research turn in the previous run could not cite what it had just received. Research reports arrive after the turn has started, so their sources were never on the list the citation tool accepts. The tool already accepted a source written inline. The desk had simply never been shown how. The guidance and both error messages now show that shape. On the next run the turn cited both sources in the body.

One more thing turned up. The suite's research check counted revisions by position, but the list is newest first, so it missed a cited revision that had landed. I fixed the check, and I gave the suite more time so the crash checks always run.

## Gate 2, one layer at a time

The probe ran four more times tonight, and each run stopped one layer deeper.

First, freezing the change failed on a new folder. The code that turns a capsule's changes into a patch read every changed path as a file, including folders. It now skips them.

Second, engineering's reports never reached Texture. Texture checked them against an older kind of permission record that management's newer delegation never creates. Engineering reports are now checked against the assignment itself. Texture now takes a supervision turn on them.

Third, the independent verifier refused the frozen change. The verifier diagnosed this itself in its report: the in-cell tool that reads a frozen change predated two fields that every source change now carries, and it refuses unknown fields. Every check it could run by hand passed. I added the fields and a test that builds the real record, so the two can't drift apart again.

Fourth, a run failed when the shared model gateway cut off a provider. One computer's requests were being rejected as malformed, and the gateway's circuit breaker counted those rejections as the provider being down. It then turned that provider off for every computer for half a minute, including the suite's. The breaker now counts only the provider's own failures, and the gateway logs why a request was rejected.

The fifth run, the seventh overall, got through verification and the approval step, where the probe stands in for you, and began to apply. Apply stalls on its checkpoint for two reasons. First, the work that resumes after the update restart keeps adding events while the checkpoint is trying to read a still history. Second, once the computer went quiet, replaying the history did not reproduce some live object records. Something tonight wrote records outside the event history. My leading suspects are the code that parks a run cut off by the restart, and the code that stops a run at its budget.

## What I got wrong

I wrote the research check carelessly and recorded a pass as a failure. Reading the document's revisions directly caught it. I also noticed that tonight's Texture fix is what let a supervision run keep working through the apply restart. That fix was right, but it exposed a missing rule about when work may resume during apply.

## What I would do next

The thing to remember: the self-development path is now honest end to end up to apply, and each remaining failure has a name.

Next I would deploy the diagnostic, rerun the probe to the apply step, read which records replay cannot reproduce, and fix that writer. Then I would hold desk work from the apply restart until the checkpoint finishes, so work still resumes after an update as you asked, just a little later. After that comes the rest of the probe: applied, rejecting a second change, and restoring.

## Names and receipts

- Suite runs 8 to 10: docs/evidence/texture-acceptance-2026-10-09T23-19-02-531Z.json, texture-acceptance-2026-10-10T00-22-37-787Z.json, texture-acceptance-2026-10-10T00-50-39-615Z.json. Run table: docs/texture-acceptance-suite.md.
- M11 reruns 4 to 7: docs/evidence/m11-rerun-2026-10-09T22-01-41Z.json, m11-rerun-2026-10-09T23-15-26Z.json, m11-rerun-2026-10-10T00-19-25Z.json, and rerun 7 on computer-520c29ba (receipt lands when the probe times out).
- Fixes: 5e91f7d2 (inline citation shape), 2c68cc18 (source patch skips folders), 028446a5 (Engineering reports bound by assignment), 3e69567b (verifier bundle mirror), 589a68bc (inference breaker counts provider failures only; 4xx reasons logged), 2a16a6db (suite T6 check and time limit), e9805de5 (replay diagnostic names og_objects rows; committed, deploys after the current probe).
- Problem docs (docs/problems/): texture-research-sources-never-reach-the-citing-turn-2026-10-09, capsule-freeze-fails-on-new-source-directory-2026-10-09, selfdev-verifier-rejects-frozen-bundle-2026-10-10, inference-breaker-trips-on-client-errors-2026-10-10, selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.
- Named residuals: texture-terminal-trajectory-revise (your Cancel decision), texture-research-assignment-finish, texture-source-pool-refresh, suite-first-sign-in-after-deploy, engineering-provider-outage-fate, search-rate-limit-cooldown-scale.
