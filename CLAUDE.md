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
- **The phone app is Exact; only voice is native** (owner, 2026-11-18: "only
  the voice stuff should be swift. the rest should be exact... the whole
  point is the codebase is shared, using EXACT"). The audio session, the
  microphone, speech and the on-device models are native (Swift on iPhone,
  Kotlin on Android). Everything else is written once in Exact and runs on
  both: pages, page logic, the server client, sign-in, sync, the run sheet,
  mail, files, the reader, podcasts, takes, the notebook. No new feature
  adds logic to `mobile/AutoradioKit` or `mobile/exact-package/apple`
  outside voice; a repair to existing Swift is allowed until its page is
  moved. A new native call needs a reason only native code can meet, written
  beside it. Where the app stands against this rule:
  [docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md](docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md).
- The roadmap is [docs/roadmap-2026-10-31.md](docs/roadmap-2026-10-31.md); the
  live station is [docs/ACTIVE.md](docs/ACTIVE.md).
- **What exists today is [docs/current-architecture.md](docs/current-architecture.md)**
  (owner, 2026-11-16). Read it before saying what the system does; a name, a
  comment or a design doc is not evidence that a thing is built. A landing
  that changes a row's status edits that file in the same change. Fewer,
  truer docs: update or delete an existing doc before adding one.

## Desks

Every desk (management, engineering, research, Texture, Mail and the rest) is
a package desk registered the same way. Each data kind has one custodian desk
with sole read and write. Desks talk only through precommitment records
(the forecast stroke of the four-stroke learning engine,
[docs/learning-engine.md](docs/learning-engine.md)); grants flow
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
- The author may not be a seat. Seat 2 is DeepSeek v4.1 flash (max) through
  OMP (`--include omp-deepseek-v41-flash`). Seat 1 was Gemini 4 Hydrogen
  (high) through OMP until 2026-11-17, when Google disabled Gemini Code
  Assist on that account for third-party OAuth use
  ([problem record](docs/problems/antigravity-login-suspended-2026-11-17.md));
  the id is removed from the panel, not re-pointed, and it comes back only
  through the official `agy` CLI, interactively, never an OAuth bridge
  another tool drives. Devin, Opus, Sol and Luna do not run code review.
- Seat 1 is a Claude Sonnet subagent (owner, 2026-11-18: "for the second
  review seat, use Sonnet subagents"), given the code and not the author's
  conclusion, and never the agent that wrote the change. Tier 2 is seat 1
  and seat 2 together. Re-review after a blocking finding and tier 1 use
  seat 2.
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
  caller. For Jev: as many questions as possible in the same call (owner,
  2026-11-15: "not multiple calls to the jev api. not concurrently or
  sequentially. thats the whole point of how it works"). Build one large
  context and put every question it can answer in that one call, up to the
  vendor's real limits. Efficiency means the job done with as few Jev
  calls as possible (owner, 2026-11-15); count calls per job, not per site.
  Running several calls at once is not a fix, and neither is queuing them:
  both are the defect. A second call is right only when the vendor's
  limits refuse the first, and the code says which limit. Do not split by
  capture, story, revision, batch or site when one context can hold them.
  What stays:
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
  never edit tracked files on Node B. The lead may restart a host service
  on Node B (owner, 2026-11-19: "Yes, you can restart a host service on
  Node B"); say so in the next report.
- Docs-only commits skip CI and deploy by design. Do not weaken those filters.
  Docs-like now means `docs/`, top-level `*.md`, `skills/` (except Go files)
  and developer-side `scripts/` (owner, 2026-11-12); they ship with the next
  deploy. Scripts that CI, nix or a deploy program reads still trigger
  (`.github/scripts/ci-docs-like-paths`).
- Platform deploy SHA and a computer's effective identity may differ on
  purpose after a product restore. Do not "fix" that by pushing main.

## Proof

- **Precommit, then score** (owner, 2026-11-10). Before a landing, an
  experiment or a diagnosis is settled by evidence, enter the claim with a
  probability, its test and a settle-by time in
  [docs/calibration/claude-ledger.md](docs/calibration/claude-ledger.md).
  Settle it when the evidence arrives, true or false, and read the
  scorecard before stating confidence in the next claim of that kind.
  The outcome must be one you cannot write: a CI run, a deployed proof, a
  count, the owner's verdict. Cite it in the settlement. A claim only its
  author can judge is not worth entering (learning engine, rule 1).

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
  primary checkout. **Every worktree is a lane** (owner, 2026-11-15):
  `cd` to the primary checkout, `git fetch`, then `lane new <name> --base
  origin/main` (or `--base origin/lane/<branch>` to build on a branch);
  remove it with `lane rm <name>`. Never `git worktree add` by hand, never
  `lane new --dirty` from the primary checkout (it copies the secrets
  there), never `lane init`. Every builder's brief says so. Read
  [docs/reference/LANE.md](docs/reference/LANE.md) first. Never use a bare
  `git stash`. Do not revert or destroy work that is not yours.
- **One Mac build at a time, run by the lead** (2026-11-15). A mobile worker
  writes code in a lane and may run Swift package tests, one at a time,
  through the lock (`/usr/bin/lockf -k -t 7200
  ~/.cache/autoradio-mac-build.lock ...`); it never runs an Exact build, a
  device build or a simulator. The lead's builds take the same lock.
- **Build output lives in one place** (owner, 2026-11-17, after the Mac fell
  to 30 GiB free). A builder's lane never holds a copy of Exact or of the
  app's compiled output: `mobile/scripts/setup-exact.sh` and `build-lab.sh`
  run only in the lead's build tree and refuse elsewhere. A lane's Swift
  package build folder is the only build output a builder makes, and
  `scripts/gc-build-output` deletes it once the lane has been idle a day.
  What a lane may run (2026-11-18, for the move to shared logic): the Exact
  loop that needs no native build, against the lead's one Exact checkout:
  `EXACT2=<build tree>/mobile/.exact-src`, then `scripts/sync-exact-package.sh`,
  `bun install`, and `bun exact.mjs contract build`, `test web`, `agent web`
  and `bun test` in `mobile/AutoradioLab`. That writes megabytes, not
  gigabytes. Native builds, simulators and emulators stay with the lead.
  The lead runs `scripts/gc-build-output` at the start of a session and
  before a device build, and removes a lane with `lane rm` when its branch
  lands or is dropped. Every builder's brief says so.
- **Every simulator is slim** (owner, 2026-11-15). Get one only through
  `AR_SIM_NAME=<your name> mobile/scripts/sim-device.sh`, which slims a new
  simulator with `mobile/scripts/sim-slim.sh` before first use; check with
  `sim-slim.sh <name> --status`, and shut the simulator down when the run
  ends. Never boot a stock simulator, and never more than one at a time.
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
