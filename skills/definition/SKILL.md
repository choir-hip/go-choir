---
name: definition
description: >-
  Use when work needs an executable /goal file: a concrete product outcome,
  observed starting state, final artifact, proof, authority boundary, rollback,
  compact current state, or proportionate independent review for long-running
  agentic work. Produces or revises a goal file that can be run with
  `/goal path.md` without turning process receipts into a second project.
---

# Definition v2: Goal Files

Definition makes `/goal <file>.md` a run command for a real outcome.

A goal file says, in order: what exists now, what will exist when the work is
done, how that result will be proved, what may be changed, and what one safe
action comes next. It is executable authority, not a plan transcript, a model
vote, or a status report.

The point is reliability in service of the product. A goal should make it
easier to build and inspect Choir, not make the process more elaborate than the
artifact it promises.

## Use It For The Right Work

Use Definition for a long-running, cross-agent, product-changing, or
semantically uncertain effort—especially one with a meaningful authority,
rollback, evidence, or completion question. Do not wrap an ordinary bounded
coding task whose outcome and proof are already obvious in a new goal file.

Before authoring or running a goal, read `AGENTS.md` and
[`docs/standing-questions.md`](../../docs/standing-questions.md). Read only the
product doctrine relevant to the promised artifact. The repository mutation
classes and their ceremony still apply; this skill does not replace them.

## What `/goal` Means

When a compatible harness receives:

```text
/goal <file>.md
```

it must reconcile the observed starting state, build or investigate the
promised artifact within the stated boundaries, collect the named evidence, and
continue until the goal is complete, honestly blocked, or superseded. It must
not stop at a checkpoint, a passing local test, a candidate, a reviewer claim,
or a polished document.

The goal file is the one hand-maintained authority for its current state.
Evidence archives, CI/deploy receipts, review outputs, and HTML views are
referenced projections. They never silently overrule the goal file.

Only owner-stated authority, observed facts, settled decisions without a live
contradiction, formal checks within their stated scope, and explicit in-bound
operating preferences may authorize execution. A model, reviewer, or repeated
claim can supply evidence or a proposal; it is not authority by repetition.

## Start With Reality

Every goal opens with an **initial-state receipt**. Capture what is observed,
not what would be convenient:

- canonical branch/ref and relevant deployed identity;
- every dirty worktree or candidate in scope, with paths, owner, disposition,
  and recovery handle;
- the current product/repository artifact and known failures;
- existing settled decisions and unknowns that can change the next action.

Do not touch unclassified dirty work. A dirty `main` can be an intentional
candidate, user WIP, or a recovery surface; record which before working near
it. If a fact is unavailable, say `unknown` and make reconciliation the next
action. During that read-only reconciliation, mutation class may be `unknown`;
no implementation is authorized until it is classified. Never fabricate a
clean baseline or a candidate identity.

The start receipt is immutable except for a dated correction that preserves the
original observation and explains why it was wrong. It is not a running log.

## Define The Finish Before The Method

Write the goal around the thing a person, external agent, or product can use or
inspect when it succeeds:

1. **Deliver** — one plain-language user or product outcome.
2. **Finish** — the exact artifact or durable state that must exist.
3. **Acceptance** — the product path, command, or observation that proves a
   scoped claim about that artifact.
4. **Rollback** — the reversal/refusal path if the change fails after landing.
5. **Non-goals and constraints** — only boundaries that can change the
   delivery, safety, or authority.

For source or platform-behavior change, `finish` also names the required
landing path: pushed source identity, CI, deployment/staging identity, and
deployed product-path acceptance. A docs-only goal may explicitly mark that
path not applicable; it may not silently substitute a local check for it.

Internal restructuring is valid only when it directly supports this finish
line. Do not make “move packages,” “write documentation,” “get consensus,” or
“reduce a count” the mission's final artifact unless that is itself the
user-visible product outcome.

### Weak Measures Are Steering, Not Proof

Goals may use weak measures—such as LOC, a structural ratchet, docs-to-code
rhythm, panel agreement, latency, token use, or number of active agents—to
choose where to inspect next. Each measure must state:

- its observed baseline and desired direction or threshold;
- the decision it can inform; and
- what it explicitly **cannot** prove.

For example, a wrapper count can prompt inspection of an extraction; it cannot
prove behavior preservation. A green test can prove its predicate; it cannot
prove the product works. Weak measures must never advance `complete`, settle an
authority question, or turn a candidate into an accepted artifact.

## The Compact Goal File

Use the authoring schema in
[`references/mission-schema.md`](references/mission-schema.md) for a new goal
or a v1 migration. Keep the file short enough that a fresh agent can find its
finish line and current action without reading history.

Its load-bearing parts are:

```text
start       immutable observed baseline and protected WIP/candidates
finish      promised artifact, proof, rollback, and non-completion cases
boundaries  authority, mutation class, invariants, exclusions
measures    weak signals with their limits
now         the one mutable current-state card
receipts    compact refs for closed boundaries
```

Use an optional decision map only when multiple routes genuinely change the
artifact, authority, evidence floor, or stopping condition. Do not begin with
a phase taxonomy, abstract graph, or retrospective. Detailed logs, reviewer
transcripts, command output, and histories live in evidence artifacts and are
linked from the relevant receipt.

`now` is the only mutable current-state card. It has one status, one active
slice, a current reconciliation identity (base/deploy plus a compact WIP
inventory ref), one candidate disposition, one accepted decision (if any),
precise blocker/risk, evidence refs, and one executable next action. These
fields must agree. Do not retain a second `next probe`, dashboard summary,
checkpoint ledger, or hand-written current status elsewhere.

A human answer is not merely conversation context. Before it changes execution,
record the selected option, decision kind, source/evidence, time, and consequence
in `now.decision`. An answer absent from the canonical card has not been durably
incorporated. The lead may choose operational routes within settled boundaries;
purpose, architecture, or authority changes remain proposals until an owner
ratification receipt is named.

When migrating a v1 Definition, extract the live purpose, observed start,
finish, constraints, active candidate, and compact receipts. Keep historical
graphs and ledgers as evidence references; do not copy their event history into
the new card or create a parallel top-level mission.

When a route is unclear, proxy risk appears, a protected mutation needs a
formalization seam, or repeated probes stop teaching us anything, use
[`references/semantic-methods.md`](references/semantic-methods.md). It is an
on-demand reasoning aid, not a second control language or routine ceremony.

## Candidate First, Then Durable Boundaries

The default rhythm discovers facts before it writes process prose:

```text
reconcile start → prepare a disposable candidate → rehearse actual effects
→ freeze and challenge the candidate when warranted → Define → Implement
→ obtain external receipts → fold them into the next Define or final closure
```

A candidate is an isolated worktree, patch, or candidate commit with a known
base, path scope, owner/location, and—when frozen—content digest. It is
disposable. It may be compiled, checked, measured, and independently reviewed
in parallel. It is not proof or a new current-state authority.

Use rehearsal to discover real paths, caller/detector semantics, count effects,
and product-path constraints before predicting them in a durable Definition.
It may begin as an uncommitted isolated patch. If reliable evidence reveals a
new platform problem requiring repair, preserve the repository's
problem-documentation-first invariant: the first repair-code commit—including a
candidate commit—follows a code-free Define boundary naming the problem evidence
and authorized repair.

For an ordinary behavior-changing slice, the natural rhythm is:

1. **Define.** Record the actual problem, next mutation boundary, evidence
   floor, rollback, and frozen candidate identity when relevant.
2. **Implement.** One coherent boundary lands the code, tests, compact `now`
   update, generated artifacts that changed, and local evidence.

Normally those two parts land together in the implementation commit. A separate
code-free Define commit is required only when problem-documentation-first,
authority change, or the repository's mutation ceremony requires it.

External CI, deploy, reviewer, dispatch, lock, and dashboard receipts normally
fold into the next state update or terminal closure. They do not earn standalone
commits. A discarded candidate or unresolved review becomes one compact outcome
once its disposition is known, not a stream of process commits.

This naturally tends toward one concise Definition update per implementation
slice, plus opening and closing bookends. It is an observed rhythm, not a quota
or a scripted commit budget. If it drifts badly, simplify the goal or make
slices more concrete rather than inventing more bookkeeping.

## Assurance And Agentic Consensus

Use deterministic checks first. Add an independent review or agentic consensus
only when it can change a real decision: candidate scope, product behavior,
evidence floor, rollback, authority, or stopping condition. Do not send a
moving target to a panel and do not use a panel merely to narrate progress.

Bind review to a frozen candidate identity: base ref, scoped paths, digest, and
available evidence. The result is an evidence receipt with an adjudicated
outcome (`accept`, `repair`, `reject`, or `escalate`), not a vote and not its
own commit. A reproducible minority blocker outweighs an unsupported majority
pass.

Reliability needs independence, not agent count. Vary the relevant failure
surfaces: model family/version, context or memory lineage, tool/search source,
and reviewer obligation (for example builder, falsifier, verifier). Prefer
durable, differently warmed agents when Choir provides them; fresh agents are a
fallback, not proof of diversity. Record cost, latency, failure mode, and
unique finding yield in review evidence or generated telemetry, then use it at
real decision gates—not as per-slice documentation work.

For work that builds persistent agents, the goal references an immutable
computer-policy resolution/run receipt (computer, authority ref, revision/digest,
and observed resolution) and names only mission obligations. It must not copy
model/tool/search/memory values into a second configuration database or
hard-code a provider/role topology that the product should parameterize at
runtime.

## Concurrency, Evidence, And Views

Parallelize read-only mapping, candidate preparation, and independent review
after the base identity is fixed. Serialize mutations that share source paths,
canonical state, deployment routing, rollback surfaces, or protected external
effects. One integration authority lands the accepted result.

Scope every claim to its evidence. Local tests, reviews, and generated artifacts
prove only what they actually observe. Product, staging, promotion, lifecycle,
and protected-surface claims need the evidence required by `AGENTS.md` and the
goal's acceptance contract. Do not weaken an evidence floor to improve a time
or token measure.

For source or platform behavior changes, complete the repository landing loop:
commit and push, CI, deploy/staging identity, then deployed acceptance. Record
those identities and results in the terminal receipt; do not call a local test
or a deployment SHA the final proof.

For broad or long-running work, launch the skill-owned local dashboard from the
goal source:

```text
node skills/definition/scripts/dashboard.mjs <goal.md> --serve 127.0.0.1:8787 --watch
```

The dependency-free renderer and server remain skill-owned JavaScript. They
serve a polished, human-readable view in memory on localhost only; the view is
not a YAML dump and serving it does not create a repository artifact.

Present it as a responsive, prose-first editorial briefing with the most
important information first. It must be neither a card grid nor a narrow
single-column desktop reader. At 1280–1440px, use the available width for a
dense, legible editorial composition with aligned regions and structural
phase or gate sections where they improve scanning. At 480px and below, resolve
the same reading order into one clean column with no horizontal overflow.
Typography is the primary hierarchy: use font family, size, weight, bold,
italics, and restrained semantic color before borders or containers. Render
scalar and map content as prose or compact definition groups, not decorative
bullet lists. Use `ul` or `ol` only when the source is genuinely list-structured,
such as acceptance items, non-completion conditions, evidence references,
worktrees, and phases.

After a compact mission identity, provenance, and non-authority note, show the
promised finish; current phase and status with the next action; blocker or risk
and the open question; mission phase path; decision gates; proof readiness; and
the protected start. Follow with secondary context such as reconciliation,
durable decisions, candidate, evidence, dissent, visibly labelled weak
measures, and successor. Keep every gate obligation visible under
plain-language headings. Never hide mission authority, obligations, evidence,
or state behind disclosure, accordion, menu, tab, or other expand interactions.
The sole exception is the live repository metadata strip: its uncommitted-file
inventory may use one native `details`/`summary`. Open it by default. Keep the
triangle on the same line as the file-count label and totals so collapsing never
moves that summary. Under it, show a compact separator-free inventory of each
path, worktree state, and per-file +/- LOC.

In the footer, keep a quiet session-only log: the newest few dashboard events
stay visible; earlier events and dirty-file first-seen / last-modified times sit
behind one collapsed disclosure. The log is ephemeral process state, not mission
authority, and must not imply acceptance or completion.

Include source identity/digest, generator version, and generation time. The
Markdown/YAML goal remains authoritative; the dashboard is a non-editable
projection and must not infer completion. Under `--watch`, live updates retain
the server-sent-event lifecycle: a current render becomes explicitly
unavailable when its source is invalidated and becomes current again only
after a successful regeneration; stale content is never reported as current.
`--watch` also hot-reloads `dashboard-view.mjs` and `dashboard-git.mjs` into the
running server. Changes to `dashboard.mjs` itself still require a process
restart.
Generated snapshots are optional explicit `--output <path>` results and belong
outside the repository unless the Definition names one as an artifact. Refresh
the live view only as part of the relevant Define or Implement boundary, never
as a dashboard-only commit.

## Resume, Exit, And Escalate

On resume, read the compact goal file first, reconcile `now.reconciliation`
with the repository, authority/policy identities, and external artifacts, then
follow only evidence links needed for the active slice. A mismatch in an
immutable source, authority, or policy identity requires a semantic-diff and
reconciliation gate before more work. A contradictory status, stale
candidate/base, stale deployment identity, or unclassified dirty path is a
reconciliation problem, not an invitation to guess.

Use only these goal statuses:

```text
working
complete
checkpoint_incomplete
blocked_incomplete
superseded
```

`complete` requires the stated finish artifact, its acceptance evidence, and a
safe disposition for candidates and dirty work. For behavior-changing work it
also includes the required pushed SHA, CI/deploy receipts, environment identity,
and deployed acceptance result. `checkpoint_incomplete` is useful durable
progress, not success. `blocked_incomplete` names the exact blocker and required
authority or prerequisite. `superseded` means the promised artifact is no longer
the right object; create one successor authority and redirect registries
atomically where required. On a Definition create, settle, or supersession,
follow the repository registry-hygiene contract and record its verification
reference rather than copying registry state into the goal.

Escalate to the owner for purpose/identity changes, authority or safety
boundaries, irreversible/high-blast-radius actions, ungranted spend, or genuine
value conflicts after rehearsal makes the factual options clear. Normal
sequencing, bounded investigation, and in-bound implementation remain the lead
agent's responsibility.

## Conformance Check

A Definition run conforms when it:

- starts from an observed, protected baseline rather than assumed cleanliness;
- names one concrete finish artifact and scoped proof;
- labels weak measures and their limits;
- keeps one compact mutable `now` card, current reconciliation identity, and
  durable owner decisions;
- rehearses a real candidate before durable claims when the route is uncertain;
- uses Define/Implement boundaries rather than commits for process events;
- binds independent review to an immutable candidate and adjudicates it;
- keeps evidence, dashboards, and histories as referenced projections;
- serializes shared authority while parallelizing genuinely independent work;
- records the required landing and deployed-acceptance receipts for behavior
  changes;
- does not confuse a candidate, agreement, local signal, or checkpoint with
  completion; and
- exits only with proven completion, honest blockage, or explicit supersession.

## Suggested Invocation

```text
Use Definition to compile <goal>.md as executable authority. Reconcile its
observed start receipt, protect all dirty work and candidates, deliver the
promised finish artifact, and prove it through the named acceptance path. Keep
one compact `now` card; treat weak measures as steering signals, not proof.
Prepare and rehearse a disposable candidate before committing durable claims
when the route is uncertain. Preserve problem-documentation-first before any
repair-code commit; otherwise co-commit the concise Definition update with its
coherent implementation. Fold routine process receipts into the next update or
terminal receipt. Review only frozen candidates when review can change a real
decision. Continue until complete, blocked_incomplete, or superseded.
```
