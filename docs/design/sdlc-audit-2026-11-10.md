# Audit of the working method, with proposed changes (2026-11-10)

Status: proposal for the owner. Nothing here is adopted. Asked for by the
owner on 2026-11-10: the method was built for earlier models, and should be
checked against what the current orchestrator does well and badly. The
evidence is the Autopaper overnight mission
([ledger](../definitions/choir-autopaper-overnight-2026-11-09.ledger.md)),
about 40 landings in 30 hours.

## What the mission showed

**Where defects were caught.**

| Caught by | Count | Examples |
| --- | --- | --- |
| Whole-package suite or CI, after a focused run passed | 4 | two red mains (search result key), the op-ed fixture, a proxy race |
| Orchestrator checking the diff against an outside contract | 3 | Jev option set (vendor reference), plain-list policy files refused at boot, wrong option order |
| Review seat (Devin) | 2 blocking | log forgery in the reliance line, empty-query ordering |
| Staging | 3 | wave 3 inert (gateway refused the read), wrong writer on fork, polls never armed |

**Where time went.** Not on writing code. A builder finishes a change in 10
to 30 minutes. A landing then takes 40 to 70 minutes of serial work by the
orchestrator: rebase, a 10-minute agentcore suite, the mirror, a review
seat, the push, about 20 minutes of CI and deploy, then a hand-run proof.

**What failed.**

1. Main was red twice for about 50 minutes. Which suites to run was a
   judgment call each time, and the judgment was wrong.
2. Builders reported "tests pass" from focused runs.
3. Review seats were not there when needed. Devin shares one quota with
   the paper and was refused three times. On 2026-11-10 Gemini through omp
   has no credential and DeepSeek through omp answers 402. The trailer gate
   accepts only `devin`, `deepseek` or `owner`.
4. Reviews and proof runs on d@e spent the paper's free quota and pushed
   the paper onto Bedrock.
5. A proof brief carried a wrong document id typed from memory.
6. Proofs live as ad hoc scripts in a scratch directory. They are not
   rerunnable by the next session.
7. A CI run for a code commit was cancelled twice after a later push.
8. "Check on d@e first" is only possible by landing on main. There is no
   way to put a branch build on d@e.
9. The hourly usage tick runs inside the orchestrator's session and spends
   its context.

## Proposed changes

Ordered by expected gain.

**1. One landing command; no judgment about what to run.** A script,
`scripts/land`, takes one or more lane branches. It rebases them onto main,
builds, runs the gates, computes the packages that import any touched
package (`go list` reverse dependencies) and runs their whole suites on the
agents VM, then pushes. Today's rule, "run whole agentcore when a consumed
surface changed", becomes arithmetic. Builders' claims stop mattering,
because the script reruns everything.

**2. Landing trains.** Independent lanes land together: one rebase, one
suite run, one CI run, one deploy. At 40 landings a day this roughly halves
the serial time. A red train is bisected by lane.

**3. A branch build on d@e.** A verb that refreshes d@e alone to a lane
build, marker-backed, with the same guards as a ring refresh. Staging proof
then comes before main, not after it. This is ring 0 made explicit, and it
is not a switch: d@e runs a different release, as the ring design already
allows. It needs its own design note; vmctl is a red surface.

**4. Proofs are tracked files.** Each item on a goal file's done list names
one deployed spec or script under `scripts/proofs/` or `frontend/tests/`.
Ids come from the output of an earlier step, never typed. The ledger line
for a proof is the script's last line. A "done" item with no rerunnable
proof is not done.

**5. Review by role, not by vendor.**
- The trailer gate accepts a seat name from a short list that includes
  `sonnet` and `gemini`, so the review that happened can be named honestly.
- Review seats get their own credential or a reserved share, so a review
  cannot take the paper's quota (the pacing work reserves 10%).
- The contract check becomes a named step for orange and red changes: the
  brief lists the outside contracts the change touches (a vendor API, a
  file format read at boot, a tool result another package parses), and the
  orchestrator reads the diff against each primary source. This step found
  more blocking defects than the review seats did.

**6. A shorter operating contract.** `CLAUDE.md` is about 4,800 words and
loads every session. Much of it is dated history, rationale, and sections
that suspend other sections. Split it: a rules file of about 1,200 words
with only what is in force today, and a rationale file with the quotes and
dates. Remove the data system cutover section when that mission closes, and
state the expedite rule once.

**7. A lighter goal file for known routes.** Keep: start receipt, finish
list bound to proofs, boundaries, the `now` card, the ledger. Make the
conjecture and realism-axis blocks optional, for missions whose route is
uncertain. On this mission the finish list and the ledger did the work; the
other blocks were not consulted. Try it on Autoradio and compare.

**8. The problem record lands with the fix.** The separate doc-first commit
doubles pushes and, here, cancelled CI runs. Keep the record (problem,
evidence, hypothesis) in the same commit, as the cutover rules already
allow. Keep doc-first only when the fix is not yet known.

**9. Ticks and watches leave the session.** The hourly usage tick becomes a
script on the agents VM that sends the notification. The orchestrator reads
its output when it matters.

**10. Builders on the agents VM.** Compiles and suites move off the Mac,
which the iOS work needs.

## What stays

Staging as the only acceptance. Worktrees per builder. The mutation
classes. Problem records. The ledger. Sonnet for building and review. No
switches. These held up.

## Open questions for the owner

- Whether a branch build on d@e (item 3) is wanted now or after Autoradio's
  first pass.
- Whether to cut `CLAUDE.md` now (item 6), since every session pays for it.
- Whether the trailer gate may name `sonnet` and `gemini` (item 5).
