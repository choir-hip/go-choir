# Choir Agent Operating Contract

This file is the repo-level contract for coding agents working on Choir, loaded
every session. It inherits [Choir Doctrine](docs/choir-doctrine.md) and must not
become a competing doctrine source. When they conflict, follow Choir Doctrine
unless this file is carrying a newer explicitly promoted operating update.

Product architecture rules live in [docs/agent-product-doctrine.md](docs/agent-product-doctrine.md)
(authority boundaries, harness minimalism, Texture control plane, runtime
configuration, product-path verification, run acceptance). Long-running mission
rules live in [docs/agent-parallax-rules.md](docs/agent-parallax-rules.md)
(Parallax, Texture narrative, independent review threads). Load those on demand.

## Default Environment

Staging is the acceptance environment: `https://choir.news`. Use local
development only for fast frontend visual iteration, focused unit shaping, or
reproducing a staging failure after staging evidence identifies the failed
transition. Do not claim local proof for vmctl, live worker/candidate computers,
gateway credentials, model/search calls, auth/session renewal, platform
promotion, rollback, or Choir-in-Choir behavior.

Use the repo dev shell for Go/Dolt work: `nix develop -c go test ...`. The
runtime package is broad and CI shards it; for local coverage prefer
`nix develop -c scripts/go-test-runtime-shards` or focused
`go test ./internal/runtime -run TestName` while shaping one transition. Do not
hand-enter `CGO_*FLAGS` for the Dolt ICU dependency except as a short diagnostic
— the durable fix is entering the dev shell.

Browser proof is specialized; do not assume every worker VM should carry
Playwright/Chromium. For source-opening doctrine, default durable web-derived
reading to Source Viewer/reader artifacts and use Web Lens only for explicit
live/original inspection.

Read [docs/computer-ontology.md](docs/computer-ontology.md) before changing VM,
sandbox, candidate-world, promotion, package, or persistent-state behavior. The
product object is a persistent user computer; `sandbox` is an implementation
service name, not the product ontology.

## Mutation Classes

Classify every mission/change by mutation class before editing:

- `green`: docs, comments, labels, and prompt/default text with no runtime
  behavior change;
- `yellow`: tests, detector manifests, or prompt framing that change future
  optimization pressure but not product behavior directly;
- `orange`: runtime behavior, product APIs, app state, database queries, or
  provider/model routing;
- `red`: protected surfaces such as Texture canonical writes, Trace/evidence,
  promotion/rollback, candidate computers, auth/session renewal, vmctl,
  gateway/provider calls, run acceptance, and deployment routing;
- `black`: irreversible or production-destructive work.

**Ceremony by class:**

- **Green/yellow:** name the class, proceed.
- **Orange:** name the class and the rollback path. Full ceremony optional
  unless touching provider routing or VM lifecycle.
- **Red/black:** full ceremony required — conjecture delta, protected surfaces,
  admissible evidence class, rollback path, and heresy delta (`discovered`,
  `introduced`, `repaired`).

Do not count newly discovered heresies as regressions, and do not count
discovery alone as repair.

## Check for Existing Fixes

Before debugging a bug in subsystem X, search for replacement or alternative
implementations of X in the codebase. If one exists:

- Is it wired in?
- If not, is the bug you're debugging a symptom of the old implementation that
  the new one would fix?
- Is connecting the existing fix cheaper than patching the old code?

If a replacement exists and is not wired in, document the connection opportunity
before patching the old code. Connecting an existing fix is preferred over
patching code that is already superseded.

## Root Cause Clustering

When you document 3+ bugs in the same subsystem within one week, stop patching.
Write a root cause clustering assessment before the next fix:

- Do these bugs share a common cause?
- Is there existing code that addresses the root cause but isn't wired in?
- Is the substrate itself broken, and are you patching symptoms on top of it?

Apply the substrate-vs-symptom classification (below) to each bug in the
cluster. If 3+ symptoms trace to the same substrate, the next action is
substrate-level, not symptom-level.

## Substrate vs Symptom Classification

When documenting a problem, classify it:

- **Substrate:** the bug is in a foundational layer (concurrency model,
  message delivery, data persistence, runtime engine, provider interface,
  VM lifecycle, event bus).
- **Symptom:** the bug is in code that runs on top of a substrate.

If you document 3+ symptom bugs traced to the same substrate, apply Root Cause
Clustering before patching the next symptom. The substrate fix may already exist
and just need wiring.

## Dead-End Escalation

If you've been working on the same problem for 3+ iterations or 2+ days without
convergence, stop patching. Write a structural assessment:

- What's the dependency graph around the problem?
- Is there a substrate-level fix that would eliminate the problem class?
- Are you debugging symptoms because the substrate is broken?
- Does a replacement implementation exist that isn't wired in?

Escalate to the human with the assessment. Do not attempt another incremental
patch without explicit direction. Continuing to patch after non-convergence is a
known failure mode, not persistence.

## Deletion-First Heuristic

Before adding code to fix a bug, ask:

- Is the code being patched already superseded by a replacement?
- Would deleting the code being patched and connecting the replacement be safer
  than patching?
- What can be removed instead of added?

Prefer connecting an existing replacement over patching superseded code. Prefer
deletion over addition when both resolve the bug. Patching superseded code
extends the life of code that should be removed.

## Problem Documentation First

Every platform behavior-changing mission must observe the following invariant:

> **Documenting a problem is the first priority. Fixing it is second.**

When staging evidence (or any reliable evidence) reveals a new problem, the
first commit that follows must be a checkpoint or mission doc update that names
the problem, records the evidence, and updates the belief state and remaining
error field — without any code fix. The fix commit(s) come second, referencing
the prior documentation.

This ensures that problems can be reviewed independently of any particular
solution, alternative solutions can be considered before committing to an
approach, and other agents and humans can examine the problem record to form
their own judgment.

Exceptions require explicit justification in the commit message, naming why the
problem could not be documented before being fixed.

See [docs/memo-problem-documentation-first.md](docs/memo-problem-documentation-first.md)
for the finding that motivated this invariant.

## Landing Loop

Every platform behavior-changing mission includes:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Do not stop at local tests or an unpushed commit when platform behavior changed.

Docs-only commits are different. The full CI/deploy workflow intentionally
ignores `docs/**` and top-level `*.md`; do not weaken those filters to force a
full CI or staging deploy path for docs-only changes. Docs-only pushes and pull
requests should run the report-only docs truth checker workflow instead.

## Worktree Hygiene

Before handing off or stopping a mission, run `git status --short` and classify
every dirty path as intentional source, durable documentation/evidence,
temporary proof output, generated artifact, or unrelated WIP.

Do not leave untracked scratch files in the repo. Temporary Playwright probes
may use `*.tmp.spec.js` while investigating, but before stopping they must be
deleted, moved outside the repo, or promoted to a normal tracked `*.spec.js`
with a clear regression purpose.

If unrelated WIP is already present, preserve it explicitly instead of mixing it
into the current mission commit. Use a named stash or a separate branch/commit,
and report the recovery handle.

## Git And Staging

GitHub `origin/main` is the source of truth for tracked deployed files. Do not
edit tracked files directly on Node B as a source/config shortcut.

If a behavior-changing commit is pushed:

1. Monitor the GitHub Actions run for that SHA.
2. Confirm Node B deploy/health reports that SHA or deployed commit.
3. Run the relevant deployed Playwright/API acceptance proof against `choir.news`.
4. Record evidence in the final report.

## Safety

Assume the worktree may contain user or other-agent changes. Do not revert
unrelated changes. Avoid destructive commands unless the user explicitly asked
for them.

Use background/candidate computers or candidate worlds for risky mutation. Failed
candidates should leave diagnostics, rollback refs, and next safe probes.

## Final Evidence

A final report for behavior-changing missions should name:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- deployed acceptance command and result;
- accepted trajectory/run/acceptance ids;
- acceptance level reached;
- verifier contracts and evidence refs;
- rollback refs;
- mutation class and protected surfaces touched;
- heresy delta: `discovered`, `introduced`, `repaired`;
- conjecture delta and human-learning digest;
- residual risks and the next realism axis.
