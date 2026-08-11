# The Long Headless Run That Wasn't Quite Finished

*A conversational review of the crashed prime-agent session, 2026-08-09.*

---

Let me tell you what happened. Last Thursday night — the 7th, around 11:37 in
the evening your time — a prime agent started working headless on the
Continuous Texture Supervision definition, that owner-ratified `/goal` that
was supposed to carry the computer through its final acceptance gates. It
never stopped. For forty-five and a half hours, through Friday and most of
Saturday, it committed, pushed, deployed, reviewed, rolled back, and pushed
some more. By the time you force-quit it Saturday evening at 9:14, it had
moved the repo's head from `cdaa787b` through a hundred and sixty-eight
commits — a hundred and thirty-six of them on first-parent — and landed a
genuinely repaired runtime on staging that is still there today.

This review came out of a nine-route agentic consensus panel, rerun after you
got Claude logged back in. Seven routes delivered full verdicts; Codex timed
out mid-review but its tail corroborates the core contract; Grok-4.5 couldn't
join — no API key configured for it. Their independent readings of a 71.8
megabyte, thirty-thousand-event transcript, your git history, and the
worktree you left behind converge on a clear verdict, and I verified the
load-bearing claims myself before writing any of this down.

## What actually succeeded

The headline is that the run's core bets were real. The strongest artifact is
the deployed runtime: `choir.news/health` reports `deployed_commit fbc7ff5a`,
status ok, vmctl ok — an exact identity join between repo, CI, and the
running service that survives independent checking. That commit implements
the shared fail-closed API-key-to-computer ownership guard, and it sits
downstream of a boot-classification repair, which sits downstream of an
earlier mailbox-resume change that briefly crashed the retained computer.
That regression-repair arc — `fd83ce64` broke the boot, four
problem-documentation-first commits followed, then `7ba05599` fixed it — is
the best part of the whole transcript. The process discipline the repo asks
for was genuinely followed: the problem got documented before the fix.

The other wins cluster around the same theme: substrate repairs that held.
The atomic Texture control-schema and prompt-authority repair (`8ac0b27d`),
the Researcher bind-loop repair (`b5d907a3`) — which got real product proof,
thirteen clean ChatGPT Researcher runs with continuous-prose Texture v1
published. A strict rollback rehearsal that passed final-state restoration
but honestly recorded a midpoint forward-observability failure instead of
rewriting history into a clean pass. Review gates that actually blocked: the
first F1 capsule-evidence candidate was rejected in independent review on
store-projection integrity, proxy escaped-path preservation, and CLI
argument ordering — the system refusing to wave its own work through is the
system working.

## Where it got stuck

The blockers divide cleanly into two kinds, and the panel was unanimous that
the first kind is not a defect.

**External, human, and physically unreachable.** The definition's acceptance
ceremonies require the exact retained owner, with a native Touch ID or
security key, in ordinary headed persistent Chrome. There is no permissible
path for a headless agent to operate or inspect that browser — no
AppleScript, no CDP, no accessibility hooks, no virtual authenticator. The
agent correctly refused every available cheat and parked the frozen
bearer-key payload (SHA `a66562ec`) once it was accepted but never executed.
The retained computer is still failed at epoch 8253, its exact guest identity
unproved. The once-only recovery was authorized but never performed —
recovery needs the same physical presence. And then there's the last blocker:
you killed the process while its F1 repair subagents were mid-edit, which is
how we ended up where we are.

**Self-inflicted, and worth naming honestly.** Three things belong in this
bucket. The mailbox-resume boot crash, already mentioned, was a regression
the run recovered from. The F1 first implementation failed independent
review, which meant a repair loop was still running at kill time. And the
run never mounted the public raw-Trace route that action 9 needed before it
attempted action 9 — a sequencing mistake, documented rather than papered
over. Claude added the sharpest observation here: facing an unreachable
gate, the run produced *six* independently-accepted-but-never-executed
acceptance helpers, up to a hundred and fifty kilobytes of byte-pinned
ceremony machinery in `/tmp` — roughly twenty-five hours whose entire output
was unexecuted artifacts. The repo's own rule says escalate on dead ends;
that rule was violated six times over. Worth internalizing for the redesign.

## The state you left behind

Here is the thing the panel kept coming back to, and I confirmed it myself
with `go build ./...`:

**The F1 worktree does not compile.**

```
internal/store/cosuper_evidence.go:558: cannot use seq (int64) as types.LifecycleEvent
internal/store/cosuper_evidence.go:558: too few values in struct literal of type reportJoin
internal/store/cosuper_evidence.go:573,589: .seq undefined on reportJoin
```

It's a half-applied refactor. One repair subagent widened the `reportJoin`
struct while `cosuper_evidence.go` still consumes the old shape — the torn
write that the `worker_recovery` event at the end of the transcript warned
about, two agents editing overlapping files when the kill landed. The second
subagent's edits exist only as uncommitted working-tree changes: nothing is
The two `agent/cosuper-*` branches hold only the rejected first candidate —
the repair generation survives in exactly one place, the main worktree's
dirty files.

That said, the code review of the worktree came back more precise than the
blunt "doesn't compile" verdict: all four compile errors trace to **one
localized torn-write conflict** — `reportJoin` is declared in its new
event-based shape (`{r, event, ref}`) but three call sites still use the
superseded `{r, seq}` shape. Everything else verified consistent with the
frozen two-gate contract: the store's attestation stamping, digest
normalization, the agentcore builder/join flows, proxy escaped-path
preservation, the strict route parse, and the CLI reordering — the three
grounds the first candidate was rejected on look addressed, and the proxy
slice builds clean. The design intent is real and largely coherent, not
junk; it is roughly a ten-line localized repair from compiling. That does
not change the panel verdict — the memos reframe the problem, and the repair
generation remains uncommitted and unproven — but it does mean the F1 code
is a credible *design input* for the rewrite, not just a crime scene.

None of that is a secret risk, though. It's all sitting in the worktree,
marked modified, and the only way it hurts you is a careless `git add .`
that mixes broken F1 code into a commit with your three new redesign memos.
Those memos — persistent RLM actors, live retrospective evals, the
world-wire generalization — are untracked and untouched, and they are the
actual carry-forward.

One correction to the record: an early panel pass claimed the `/tmp`
acceptance helpers were gone. They're not. All nine are still there, and the
topology helper's SHA `785ae23b` matches its byte-pin exactly. If you intend
to run any ceremony *before* adopting the redesign memos, that dissent wins
and the helpers should be pulled out of `/tmp` today.

## What the panel decided

The verdict, converged across seven independent routes: **the run was a
runtime-hardening campaign that succeeded, and a mission that did not
complete.** Every route recommended the same concrete posture:

1. **Do not resume the CTS definition to `goal.complete()`.**
   `goal.complete()` was never called — correctly, since the mission was
   incomplete. Treat the run as a closed, evidence-rich predecessor, not an
   unfinished mission to be resumed.
2. **Quarantine the F1 worktree.** Stash it with a named handle; don't
   repair it. The memos reframe the capsule-evidence problem better than the
   frozen two-gate contract did, and rewriting against them beats archaeology
   on a dead session's intent. The durable asset is the *problem statement*
   — public surfaces can't express capsule execution evidence — and the
   rejection grounds, not the code.
3. **Commit the three memos.** They're the redesign. Keep them cleanly
   separate from the F1 diff.
4. **Close the standing liabilities before the next mission.** The retained
   computer at epoch 8253 is the one irreversible item — recover it or
   explicitly retire it. The historical API key `ak_45ce1796` and the two
   root-only provider-auth backups under `/var/lib/go-choir/provider-auth-backups/`
   are real security debt independent of the redesign.
5. **Carry the contracts, not the choreography.** Into the redesign go: the
   fail-closed ownership guard pattern, atomic Texture transitions, the
   problem-documentation-first trail (`54ffd2a7` → `35b1f2cd`), the rollback
   rehearsal's honest qualifier, and the long-horizon process discipline the
   run demonstrated. Out go: singleton-Super semantics, the old acceptance
   choreography, F2 as specified, and the six unexecuted helpers — unless
   you're running ceremonies before the memos land.

Worth recording the dissent: Devin alone recommended *continuing* the F1
repair track rather than abandoning it — but even that route gates the
proofs on "redesign-era capabilities," which lands in the same place as the
majority: don't land it now. And Claude's full writeup, saved under
`~/.claude/plans/mode-convergent-decide-glistening-falcon.md`, is worth
reading in full for the ceremony-versus-redesign sequencing debate.

## The commit log the session left behind

A second review pass looked at the *landing mechanics*, not the mission —
and found that the GitHub log misreads even when the work is good. The
session range `cdaa787b..35b1f2cd` is 168 commits, but 32 of them live
inside **seven merge bubbles**: agents worked in `/private/tmp/choir-*`
worktrees, then their branches were landed with plain `git merge` (two of
them thirteen commits wide). GitHub's default commits view renders the full
DAG without a first-parent filter, so those bubbles interleave with and
duplicate the linear story. Worse, merge landing hid three **patch-identical
re-landings** — `891f50b0`/`77e88605`, `e4a100dd`/`57f5cb10`,
`3f375cf2`/`4686ce11` — the same change appearing twice in the log because
parallel repair workstreams restated already-merged content and the merge
absorbed it silently. And one premise I'd stated early was wrong: the 33
rebased commits cluster mid-session (08-08 00:05–08:05), not at the tail —
the display problem is the bubbles and duplicates, not rebase dating.

The fix is mechanical and the reviewer was confident in it (0.9): keep
working on worktree branches, but land with `git rebase origin/main` in the
worktree, then `git merge --ff-only` (or `git push origin <branch>:main`) on
main — strictly linear, every problem-doc-first pair preserved as its own
commit, no bubbles. Enforce it with a pre-push hook or CI check that the
push range contains zero merge commits (`git rev-list --merges --count
origin/main..HEAD` == 0), since direct pushes to main bypass GitHub's
PR-only "require linear history." Add a `--cherry-pick` identity check before
rebase so duplicate-content re-landings are dropped instead of merged. The
seven existing merges stay as history; none of this rewrites them.

## The residual gaps

Most claims in this review were verified twice: once by panel routes that
independently queried git, the live `/health` endpoint, and the worktree, and
once by me directly before writing. The short list of things none of us could
re-verify without more access: the exact contents of CI run `31326948312`
beyond its green status, the internal mechanics of the thirteen Researcher
runs, the doccheck counts (357 docs, 120 warnings), and the precise failure
mode at epoch 8253. Two anomalies worth a glance when you're back at the
machine: `/health` has logged 32 `unauthorized` errors under `api.auth`
(stale-token traffic, probably, but it's your auth surface), and the
`go.mod` module path says `github.com/yusefmosiah/go-choir` while the repo's
public org is `choir-hip` — cosmetic, but it will bite someone eventually.

---

## Landing record (2026-08-10)

The F1 repair was landed on `main` at `26c53692` (commits `60267517` +
`26c53692`), after the design review panel returned unanimous
LAND / LAND WITH FIXES and zero HOLD. Staging serves `26c53692`; CI run
`31445846546` succeeded including staging deploy; rollback is a clean revert
of the two commits (feature previously unshipped, no live data migration).
Orange mutation class; protected surfaces touched: store/agentcore/proxy/CLI
routing. Heresy delta: `repaired` — the torn `reportJoin` write conflict
(`{r,event,ref}` vs `{r,seq}`) is the sole compile-blocker, now fixed and
covered by tests.

### Recorded decisions (documented-as-intended)

- **20k owner/computer-wide cliff.** The evidence projection snapshots the
  owner/computer trajectory scope and fails closed (`ErrCoSuperEvidenceTooLarge`)
  beyond hard caps. Intended: a bounded snapshot contract; a single owner
  with more evidence than the caps is a real outlier, and the failure is
  explicit rather than silent truncation.
- **Whole-trajectory coupling.** Evidence for one assignment reads the full
  trajectory's object set. Intended: cross-assignment candidate-source
  lineage requires the sibling implementation report to be present in the
  same snapshot; the reducer is the single writer, so the coupling is
  deterministic and tamper-evident by construction.
- **Source-event scope check.** The cross-assignment candidate source event
  join now validates run/agent scope exactly like the direct report join
  (`corruptEvidence("candidate source event run or agent scope")`), closing
  the one scope gap the tamper suite exposed.

## In one paragraph

A headless agent ran unattended for nearly two days, moved the runtime
forward in measurable, deployed, product-proven ways, hit a wall that was
made of your physical presence and not its own limits, and then it was killed
mid-repair with a worktree that doesn't compile and a ledger of real
liabilities waiting for you. The panel's advice, and mine: take the proofs,
take the problem records, take the discipline — leave the unfinished code and
the old choreography behind, and let the three memos decide what the computer
becomes next.

---

*Panel evidence: `.agentic-consensus/crashed-session-review-rerun/manifest.tsv`
and per-agent `.out` files; session transcript
`~/.prime/agent/sessions/019fdf72-84b8-77da-a64c-8fc3b2b86538.jsonl`; git
range `cdaa787b..35b1f2cd`; `curl https://choir.news/health`; `go build ./...`.*