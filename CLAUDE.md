# Choir Agent Operating Contract

The rules in force, loaded every session. The reasons, owner quotes and dates
are in [docs/operating-contract-rationale.md](docs/operating-contract-rationale.md).
Architecture is normative in [docs/choir-doctrine.md](docs/choir-doctrine.md);
this file must not become a second doctrine. Where the two conflict, doctrine
wins unless this file carries a newer, explicitly promoted update.

## Product

- The product is the automatic computer: a persistent computer for supervised
  self-development ([docs/choir-vision.md](docs/choir-vision.md)). Read
  [docs/computer-ontology.md](docs/computer-ontology.md) before changing VM,
  candidate, promotion, package or persistent-state behavior.
- Names are never camel case: Autopaper, Autoputer, Autoradio. "World Wire",
  "Universal Wire" and "sourcecycled" are retired names for Autopaper. The
  shared graph of published objects is **the Autopaper object graph**.
- The roadmap is [docs/roadmap-2026-10-31.md](docs/roadmap-2026-10-31.md); the
  live station is [docs/ACTIVE.md](docs/ACTIVE.md).

## Desks

Every desk (management, engineering, research, Texture, Mail and the rest) is
a package desk registered the same way. Each data kind has one custodian desk
with sole read and write. Desks talk only through precommitment records
([docs/precommitment-records.md](docs/precommitment-records.md)); grants flow
through custodian leases. Management owns computer-wide coherence and admits
engineering; engineering mutates only inside capsules; research has read-only
world-evidence authority. Use the in-cell module surface, not legacy tool
calls. Product rules:
[docs/agent-product-doctrine.md](docs/agent-product-doctrine.md).

## Before you change anything

- **Read the reference page before touching a new dependency** (owner,
  2026-11-10). A dependency newer than your training data (Jev, Exact, the
  on-device models) has a page in [docs/reference](docs/reference/README.md).
  Read it first, do not answer about the dependency from memory, and update
  the page in the same change that changes how we use it. A new such
  dependency gets a page.

1. **Read the traces first.** A claim about why an agent or desk did
   something is a hypothesis until the run's ledger confirms it (turn-commit
   reasons, controls, work items, event order).
2. **Substrate or symptom?** If the problem lives in a foundational layer,
   fix it there. Search for an existing, unwired replacement before patching
   the old code, and prefer deleting to adding.
3. **Three bugs in one subsystem in a week:** stop and write a clustering
   assessment before the next fix.
4. **Understand before tuning.** No prompt or parameter hill-climbing until
   the failure is understood from traces and code.
5. **Investigate eagerly, act deliberately.** An unexplained number gets a
   read-only investigation and a record when it is seen.
6. **A hard or recurring problem:** change the frame before the patch
   ([skills/cognitive-transform-portfolio](skills/cognitive-transform-portfolio/SKILL.md)):
   ask what the thing is, redefine the problem, work out the physical bounds,
   map the options, check whether one mechanism can do several jobs.
7. **Three rounds without convergence:** stop patching. Write a structural
   assessment and change the approach; escalate to the owner if that fails.
8. **Durable resource savings beat short-term uptime**, given a proof and a
   rollback.

## Mutation classes

Classify every change before editing.

| Class | What | Ceremony |
| --- | --- | --- |
| green | docs, comments, labels, prompt text with no behavior change | name it |
| yellow | tests, detectors, gates, scripts | name it |
| orange | runtime behavior, APIs, queries, routing | name it and the rollback |
| red | Texture canonical writes, tape and evidence, checkpoint and route projection, auth and session, vmctl, gateway and provider calls, run acceptance, deploy routing | conjecture delta, protected surfaces, evidence class, rollback, heresy delta |
| black | irreversible or destructive in production | the same, plus the owner |

A commit subject ends with its class and rollback, for example
`(orange; rollback: revert)`.

## Review

- Cheap checks run before every landing: `scripts/land` (when present) or, by
  hand, the build, the repo gates, the **whole** uncached suite of every
  touched package and of every package that imports it, a nix eval of both
  systems when `nix/` changed, and a rebase onto main.
- **Read the diff against every outside contract it touches** (a vendor API,
  a file read at boot, a tool result another package parses). Use the primary
  source.
- Green and yellow: no model review. Orange: one seat. Red where a defect
  would be silent or irreversible: two seats. Re-review a red change after a
  blocking finding.
- Any second agent may be the seat; the author may not. The default seat is
  Gemini 3.8 flash (high) through `agy`. Sonnet is the second seat. Opus, Sol
  and Luna do not run code review.
- Guest-security paths need the `Security-Effect` and
  `Security-Effect-Reviewed-By` trailers (`scripts/check-guest-security-epoch`).
  Never write a review trailer that no review backs.
- Models: Opus for open-ended design only; Sonnet for investigation, building
  and review. Builders work in their own worktrees and compile on the build
  VM (`ssh choir-agents`). Never `pkill`; kill only your own PIDs.
- Panels for decisions:
  [skills/agentic-consensus](skills/agentic-consensus/SKILL.md). While
  blocked on CI or a deploy, use the time for a big-picture review.

## Delivery

- **No switches.** New behavior ships on: no off-by-default setting, no
  selector, no allowlist that gates a feature. A bug is fixed, not hidden. A
  correctness refusal is not a switch.
  A grant that spends the owner's money is an owner act on the tape.
- **No limits on our own model calls** (owner, 2026-11-10: "too many model
  calls is a code problem to fix, not a limit to impose", enforced
  consistently). No rate bucket, in-flight cap, spacing or call quota on our
  own code's calls to a model or to Jev. When calls are too many, fix the
  caller. For Jev: one call asks many questions; calls are not issued
  concurrently. Separate contexts can justify concurrent calls, but that
  is never plan A. What stays:
  backing off after a provider's real refusal, an owner-set spending cap,
  bounds on what strangers can cause, and deadlines. Log volume so a spike
  is seen.
- **Expedite everything until the owner says Autoradio is solid.** Every
  landing commit that is not docs-only carries `Ring-Expedite: <the defect it
  unblocks>`. The deploy also expedites, without a trailer, any release that
  touches the paths listed in `.github/scripts/node-b-ci-deploy`
  (`internal/agentcore/` among them). An expedite keeps the planned,
  marker-backed refresh, the capsule, apply and skew refusals, the health
  guards and automatic rollback. Name every expedite in the next report.
- **Landing loop** for any behavior change: commit, push `origin main`, watch
  CI, watch the staging deploy, verify the deployed commit, run the deployed
  proof. Local tests are not proof. `origin/main` is the source of truth;
  never edit tracked files on Node B.
- Docs-only commits skip CI and deploy by design. Do not weaken those filters.
- Platform deploy SHA and a computer's effective identity may differ on
  purpose after a product restore. Do not "fix" that by pushing main.

## Proof

- Staging (`https://choir.news`) is the acceptance environment. Local proof
  does not count for vmctl, live computers, gateway credentials, model calls,
  auth, checkpoints, rollback or Choir-in-Choir.
- Prefer end-to-end tests that leave a rerunnable artifact. Never write unit
  tests after the code: write the failure list first, then tests, then code.
- A check that has not run proves nothing. Do not report a test as passing
  unless you saw it pass.
- Adversarial drills run on d@e only, never on the paper or the owner's
  computers.
- Browser proof is specialized; not every worker VM carries Playwright. For
  web-derived reading, default to Source Viewer artifacts; use Web Lens only
  for explicit live inspection.

## Problems come first

When evidence shows a new problem, record it (problem, evidence, belief,
remaining error) before or with the fix. If the fix is not yet known, the
record lands alone first. See
[docs/memo-problem-documentation-first.md](docs/memo-problem-documentation-first.md).

## Restarts

- **A crash restart never resumes work.** At boot every open obligation is
  closed as "interrupted by a restart". Standing owner intents (an Autopaper
  source schedule) survive; their in-flight occurrence does not. A schedule
  whose tick crashes the computer twice is closed.
- **A planned restart may resume once.** The host writes a durable marker
  before the restart; the boot consumes it before resuming. No marker means a
  crash. Do not add a boot-time resumer that runs without a marker.
- Deploys refresh idle computers and retry busy ones. `critical_protected`
  and `public_platform` computers take a planned refresh while busy, but never
  mid self-development apply or while an engineering capsule is live.

## Missions

- Read [docs/standing-questions.md](docs/standing-questions.md) before
  authoring or running a goal file. The format is
  [skills/throughline](skills/throughline/SKILL.md); earlier formats are
  archived.
- **No blocking asks.** When a decision has no obvious answer, run the
  transforms or a panel, take the conservative option and say so, or record a
  named residual and continue. Ask only before something irreversible with no
  safe default, or a credential or authority question.
- After a mission lands, closes or is superseded: update `docs/ACTIVE.md` and
  `docs/mission-graph.yaml`, author the next goal file, delete superseded
  drafts.
- The data system cutover runs under its own tempo rules, kept in the
  rationale file under "Operating Tempo".

## Safety and hygiene

- Assume the worktree holds other people's changes. Never commit in the
  primary checkout; use a worktree. Never use a bare `git stash`. Do not
  revert or destroy work that is not yours.
- No secrets, tokens or the owner's email addresses in the repo or in output.
  Per-developer secrets live in the gitignored `.envrc.local`.
- Risky mutation happens in disposable capsules inside the stable computer.
  A candidate VM is never a self-development candidate of its parent. A
  forked computer is a sibling with its own identity; its results return only
  through publication and adoption.
- Before stopping, run `git status --short` and account for every dirty
  path. Leave no scratch files in the repo.
- The dev shell (Nix flake via direnv) provides Go and PostgreSQL 17; there
  is no cgo. `internal/agentcore` and `internal/textureowner` are large; use
  `scripts/go-test-runtime-shards` or the build VM.

## Final report

For a behavior change, name: the pushed SHA; the CI run and deploy status;
the staging commit; the proof command and its result; ids of accepted runs;
the rollback; the mutation class and protected surfaces; every expedite; what
was discovered, introduced and repaired; residual risks and the next step.
