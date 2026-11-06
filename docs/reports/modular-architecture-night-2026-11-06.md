# The modular architecture, the night of November sixth

Written on Friday, November sixth, after a night and morning of work that ran from late on the fifth to about nine in the morning, Eastern time. This letter covers the modular architecture mission only, and it is a checkpoint: the mission is not finished.

Every desk worker on your test computer now runs inside its own gVisor sandbox, desks can write real Go in their cells, and the Texture app has become a package like every other app.

## Where things stand

You asked to finish the mission tonight, and it did not finish. Most of it is on main and proven on the test computer, d@e. What remains is either waiting for time to pass or waiting for one more review round.

Phases one, two and three are closed. Phase four is landed up to its last two stations. Those two, engineering as a package and the deletion of the old desk-name switches, are deliberately last, and the plan says engineering waits for a week of staging runs first. Phase five, apps as packages, is landed through the Texture move. Phase six, isolation, is landed through station six of seven. The seventh station compares capsule round times with and without gVisor, and it needs real self-development rounds, which is phase eight. Phase seven, Bayesian trust, is landed through its ninth station, and its harness now has real rows to read.

There are three decisions that belong to you, and none of them blocks the work. The first is whether to switch the disclosure gate from watching to enforcing. Today it only ever sees research asking to pass a captured private page to Texture, and I did not want to decide that policy alone. The second is the default for the pasted-link chip. The third is the platform operator list on Node B, which one staging proof needs.

## What happened overnight

Late on the fifth, the act gate, the forced modularity work and the four Library stations landed, with two small expedited fixes for CI gates that the new code tripped. Then the trust work showed a quiet gap. The trust view on d@e was empty, even though desks had been judging each other's Casts all day. The reason was simple: the view read only one kind of Cast and dropped the desk-to-desk kind without a word. I documented it, fixed it, and after the deploy the view showed eighteen rows. The harness that scores trust against the mission's own predictions went from zero rows to eighteen. Its outcome tests stay sealed until there is enough evidence or until January twenty-ninth, which is how it was designed.

In the early hours, the Texture app moved into the package system. The shell no longer has any hard-coded apps. A Texture revision is now committed only for the desk that holds the Texture grant. And a visitor who is not signed in no longer sees an Edit button on a published page, which fixed a staging test that had been failing. Two reviewers accepted it. The full Texture acceptance suite on staging passed nine of eleven checks. The two failures are old, recorded problems. One is that a new document's first draft waits for your next action instead of arriving by itself. The other is that revising right after a cancel returns a conflict. I did not patch either, because the mission rule is not to patch the old system while the new one is being built.

Then came the isolation work, which took most of the night. The gVisor desk workers went through four review rounds. The first two found real defects: a sandbox could outlive its parent, one desk running out of memory could take down a whole class of desks, and the memory accounting was wrong because the C library reserves space Go never touches. The third round found that a small computer could run only three desk workers at once, so research's child workers would have been refused. I chose to make them wait in line instead, with management and Texture first. The fourth round closed the reviewers' remaining availability points. On d@e, the self-test now shows every desk under gVisor, in two sandboxes, using about four hundred megabytes between them.

Next, desks gained the ability to use far more of the Go standard library and to import curated Go modules, each pinned to the exact hash the public checksum database records. One reviewer found a near miss: four functions in the atomic package could hand a cell a raw memory pointer without importing the unsafe package. They are gone now, along with every other bound function that could do the same. A mechanical test checks every function a cell can reach.

The last station of the night was code mode. Each desk now gets its own import set, derived from its grants. The measurements found two problems in old code. Host calls from a cell were served strictly one at a time, and nothing limited how many a cell could make. A cell can now run eight calls at once and at most a thousand and twenty-four per cell, and a hundred reads in one cell cost one model round trip. Running calls in parallel opened two narrow ordering gaps, which the reviewers caught and I fixed before landing. The worst was that a web search's results could reach a cell a few microseconds before the run was marked as having read untrusted content.

## What is in review now

This morning I started the boot migration from the forced modularity design. It moves package source out of a computer's applied self-development changes, because until it runs, such a computer refuses new work. d@e is in exactly that state today. Both reviewers sent it back. The core path held up, but the owner rollback had two serious defects. Rolling back could quietly overwrite a newer change you had applied since. And after a redeploy, the next boot could undo your rollback. Devin also found that nothing stopped other writers from moving the served release while the migration ran. That is being repaired now.

One more fact matters before it rolls past d@e. I could not read your own computer or the paper with my key. If either holds package source in its applied changes, the migration could set an app aside and take it off the launcher. The app's code is kept, and a rollback brings it back, but you would see it.

## Lessons

The empty trust view taught the clearest lesson of the night. A view that silently drops what it cannot read looks exactly like a view with nothing to show. The fix counts every exclusion by name now, so an empty view has to explain itself.

The memory work taught the second. My first calibration of the gVisor sandboxes trusted the Go runtime's own numbers, and the C library's reservations made them wrong. The measurement on a real two-gigabyte computer, not the arithmetic, found it.

The third is about review. Four stations this night got two seats each, and the second seat earned its place every time. For the boot migration it found the two blocking defects before Devin finished.

## What I would do next

If I were holding the phone, I would let the boot migration's repair finish and land it on d@e, then read back the result and try the rollback once. After that, the next real step is phase eight: real self-development rounds on the new architecture, which also unlocks the capsule pilot. Engineering as a package waits for its week of staging runs, and you might decide the disclosure gate's policy whenever it suits you.

The one thing worth remembering is that the isolation phase, the part that makes it safe to run other people's desks, is now real on d@e and not just designed.

## Names and receipts

Goal file: docs/definitions/choir-modular-architecture-2026-11-04.md. Ledger: docs/definitions/choir-modular-architecture-2026-11-04.ledger.md. Plan: docs/design/modular-architecture-plan-2026-11-04.md.

Landed commits in this stretch: act gate 554f78e67; forced modularity and Library L1 to L4 through 686f89910; iframe sandbox d930f16a8; MA6.3 fixes 181e95c77; trust harness bc008b50b and 35e153ad2; trust feed e3162eccd; Texture T1 06ecaa159 and e23eeee24; gVisor desk workers ad180ea7d (epoch 85); yaegi Go modules 5c83a08ff (epoch 86); code mode 916114811 (epoch 87). Security epoch on main: 87.

Evidence: docs/evidence/ma7-9-harness/2026-11-06-59c902c5-trustfeed/; docs/evidence/texture-acceptance-2026-11-06T08-32-08-053Z.json; docs/evidence/ma6-isolation-selftest/ (three d@e receipts).

Design docs: docs/design/ma64-gvisor-desk-workers-2026-11-06.md, docs/design/ma65-yaegi-go-modules-2026-11-06.md, docs/design/ma66-code-mode-import-sets-2026-11-06.md.

Expedited commits, each carrying Ring-Expedite: fadd941ca, d613108ed, f58ba6707, c4d733c89, 856be9c9f, 302dcc89e, 21d1162a8, d06756b2a, 00d817dc9, 819e36492, 638ef567b, 73c3be4ea, ab5b1e663.

In review: the boot migration, branch m3rest, commit 57eadd60c, sent back by both seats; repair round one in progress.

Residuals named for you: the DeepSeek review seat is unavailable for lack of funds, and a Claude Opus seat stands in, named honestly in the trailers. There is no Devin provider inside the platform, so widened package installs are refused. The trust feed reads Casts one by one, which costs a query per settlement. The desk-names allowlist stands at nine hundred sixty-seven, not zero. The gVisor sandboxes leave the runsc client processes outside the memory ledger. The paper's key expires on November twelfth.
