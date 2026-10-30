# Self-Development Readiness for 2026-10-15

Date: 2026-10-14, evening (written for the morning of 2026-10-15). Mutation
class of this report: green (analysis only; no code, no deploy, no secrets).
Read-only probes: `GET https://choir.news/health` (staging serves
`f062bb13b`, deployed 22:02 UTC; CI for `5b17bb572`, today's `origin/main`,
was in progress) and `gh run list`. Everything else is code and evidence read
on `origin/main` 5b17bb572 and on the three unlanded branches.

The goal for tomorrow: a Claude Code session develops Choir's own code by
operating the `choir` CLI and MCP against a Choir computer on staging, instead
of editing the repo directly.

## The answer in one page

1. **The command the goal names does not work.** `choir self-dev start
   --prompt X` posts to `/api/computers/<id>/self-development/operations`,
   and the guest answers 405: C1e B1 (a06191f9e) deleted the public
   start-operation API (`internal/agentcore/api_self_development.go:164`, "a
   candidate opens only when an engineering assignment's Freeze opens it").
   The CLI, its MCP tool `self_dev_start`, `skills/choir-cli/SKILL.md` and
   the CLI/MCP parity report still advertise it. The real start is the prompt
   bar: `choir run start`, the same route the tracer uses.
2. **No self-development change has reached Freeze since the records path
   (C1e B1) landed.** Four engineering drills on 2026-10-14 stopped before
   Freeze; the last applied candidate on staging was M11 on 2026-10-10, on
   the path B1 deleted, with a binary the desk staged some other way.
3. **The next blocker is fixed on an unlanded branch.** The capsule cannot
   produce `bin/autoputer` (no C toolchain, 1 GiB, 1 CPU, 30 s exec). Branch
   `worktree-agent-a03d8d5f7282257c5` (7645f293f..657d3d3d1) reuses the
   running binary for a change with no Go input, adds the toolchain and
   gives engineering capsules 2 GiB. It needs a rebase (conflicts with
   tonight's precommitment commit), a red-class deploy, and one tracer run.
4. **Everything after Freeze is unexercised on the records path.**
   Verify, the owner's approval, the apply through one planned restart, the
   served change and the restore have not run since B1. The same tracer run
   is their probe; expect at least one new finding there.
5. **"Live on the computer" means frontend only.** A self-development apply
   mints a plain release; the guest's exec wrapper runs the base image's
   `bin/autoputer` for any release that is not layered
   (`nix/autoputer-vm.nix`, `autoputerRuntimeExec`). A Go change can be
   built, tested and frozen in a capsule, but it does not run on the
   computer after apply. Go changes go live only through GitHub, CI and a
   platform deploy (the S2-f layered-release wiring is the open residual).
6. **There is no path from a capsule's diff to GitHub.** Freeze already
   writes the diff a PR needs (`source.patch`, its sha256 and its base
   commit, in the frozen bundle), but no product route serves it. The
   minimal Choir-in-Choir path is one read-only route, one CLI command, and
   the outside agent opening a draft PR with the owner's existing GitHub
   credentials. Nothing on the computer ever holds a GitHub credential.

What tomorrow can realistically do, in order: land and deploy the capsule
build branch; run the engineering drill to a full pass (or to its next
documented blocker); then run the development loop on fresh test computers
for frontend changes, with the owner approving each candidate from their own
terminal. Choir-in-Choir to GitHub needs the export route first (a small
orange change, its own deploy).

## 1. The path today, from request to computer to GitHub

### 1.1 Start

The owner arms the computer once: a fresh computer's mode is `off`, and
Freeze opens a candidate only under `propose_only`. Arming is an owner act and,
since 5b17bb572, CLI-only:

```
choir self-dev mode set --computer <id> --mode propose_only \
  --expected-generation <n> --idempotency-key <k>
```

The request then enters through the prompt bar, not through `self-dev start`:

```
choir run start --idempotency-key <k> "<the change, with its deterministic check>"
```

That returns a `trajectory_id` and a Texture `doc_id`. Note the authority this
implies: a key with `write:runtime` can start self-development on an armed
computer without any self-development scope. The
`computer:self_development:propose` scope now guards a dead route.

### 1.2 Inside the computer

Texture Casts the request to `management:<owner>`. Management Casts
`engineering:<assignment>` on records. Engineering runs in a disposable
capsule whose source is `files/Source/platform`, seeded at computer creation as
a clean checkout of the commit vmctl was built from
(`internal/vmmanager/manager.go:2840`). The capsule edits
`/workspace/platform`, runs its checks, builds the release tree at
`/var/lib/artifact/release/` (an executable `bin/autoputer` and `frontend/`),
and stages `choir.Freeze` with three distinct execution receipts (build,
tests, dependency/toolchain).

Freeze opens the self-development operation (`requested -> executing ->
frozen`) and writes the frozen bundle to
`/mnt/persistent/choir-updater/incoming/<digest>/`: the runtime files, the
bundle JSON, and `source.patch`, a unified diff of every `workspace/platform`
change against the pinned base commit, proven to apply by the builder's own
applier before any verifier sees it (`internal/capsule/executor.go`,
`emitSourcePatch`). `choir.Complete` then mints engineering's final Report
naming `selfdev-operation:<id>` and `bundle:<digest>`. Management's Verify
records a verdict (`verified -> awaiting_approval`).

### 1.3 Owner decision and apply

The owner approves with `choir self-dev approve --computer <id> --operation
<op>`: it reads the operation and the event head, arms `accept_once` bound to
exactly that operation, bundle, heads and commitments (15-minute window), and
posts the decision, which consumes the arm (mode returns to `propose_only`).
The materializer mints a plain file release from the bundle's runtime files;
the computer restarts once, as a planned restart with reason
`self_development_apply`; the operation reaches `applied` with materialization,
checkpoint and route receipts. A restore to a checkpoint pinned before the
approval is `choir computer restore`, also an owner act.

What is live afterwards: the release's `frontend/` is what the computer
serves. The process is still the base image's `bin/autoputer`, because only a
layered release execs its own binary. So a frontend change is live on the
computer; a Go change is not.

### 1.4 GitHub

Nothing in `internal/` or `cmd/` pushes, commits to, or opens a PR on
`github.com/choir-hip/go-choir`; doctrine keeps it that way (computer-local
self-development and platform landing are separate authorities,
`docs/computer-ontology.md`, "Platform and public change"). Today a change made
on a computer reaches the repo only if someone retypes it.

**The minimal path (proposed, orange):**

1. A read-only guest route,
   `GET /api/computers/<id>/self-development/operations/<op>/source-patch`,
   scope `computer:self_development:read`, served from states `frozen`
   onward. It returns the patch bytes from `incoming/<bundle digest>/`
   with `bundle_digest`, `source_patch_sha256` and
   `source_patch_base_commit`, and refuses when the bytes do not hash to
   the bundle's recorded sha256. The proxy's route classifier
   (`selfDevelopmentGuestRoute`, `internal/proxy/self_development.go`) must
   map the new suffix to `read`, or the proxy answers 404.
2. A CLI command and MCP tool, `choir self-dev patch --computer <id>
   --operation <op> --out <file>` (read-only; the MCP tool returns the patch
   inline under a size cap).
3. The outside agent, in its own go-choir checkout, using the owner's
   existing `gh` authentication on the owner's machine:

   ```
   git fetch origin
   git switch -c selfdev/<op> <source_patch_base_commit>
   git apply --index <file>        # and check sha256 against the route's answer
   git commit -m "<summary>" \
     -m "Choir-Operation: <op>" -m "Choir-Bundle: <digest>" -m "Choir-Computer: <id>"
   git rebase origin/main
   git push origin selfdev/<op>
   gh pr create --draft --base main
   ```

   CI runs on pull requests (`.github/workflows/ci.yml`). The owner merges; the
   Landing Loop (CI, deploy, staging identity, deployed proof) applies as for
   any platform change.

Ways it could fail, written before any code: the bundle directory is gone
after apply or a cleanup (read retention before relying on it; serve before
apply); the patch carries build outputs (`frontend/node_modules`,
`frontend/dist`) because the frozen subject is the whole tree and ignores
`.gitignore` (the export must refuse paths git would ignore, and the
engineering precommit `paths_confined` catches it earlier); the base commit is
not on GitHub (a persistent computer seeded from a local build; refuse with
the commit named); the patch carries a secret (the Freeze secret scan guards
release files; run the refusal-grade scan over the patch at export); the PR is
taken for an owner approval (it is a proposal; the PR body names the
operation and whether the owner approved it on the computer).

A zero-code stopgap exists but is weakly bound: ask engineering to print
`git -C /workspace/platform diff` in a cell, then read it from `choir trace
export`. Nothing binds that text to the frozen bundle, so it is fine for a
first look and wrong as provenance.

## 2. Where the drill stops: the evidence

All four engineering drills on 2026-10-14 used
`node scripts/reeng_tracer.mjs --scenario engineering` on freshly minted
computers. Receipts are under `~/.config/choir-qa/traces/<label>/receipt.json`
on the owner's machine; every one exited 6 (the claim failed).

- **`c1e-b1-engineering-0444`** (04:44 UTC): three engineering assignments
  each ended `cancelled` within 25 s of the Cast. Cause: the capsule gate
  (fixed in bc09dcc1; the problem doc cites 11a94193, an unrelated
  memory-limit commit). Reached `engineering_final_report`.
- **`c1e-b1fix-engineering-0549`** (05:49): the operation opened and failed
  with `freeze: capsule release contains no frozen runtime artifacts`. The
  overlay named the release root inside `/workspace/platform`; the code reads
  it at the capsule root. The subject copy also refused an in-tree
  `node_modules` symlink. Both fixed and deployed in 9ace75e5 (Freeze fixes
  9874ebc5, 96b82cec).
- **`c1e-d4-engineering-0648`** (06:48): five assignments, each a real
  `failed` final Report naming the one blocker: Go module zips redirect to
  `storage.googleapis.com`, which capsule egress refuses. Engineering
  correctly did not Freeze. Owner chose the host module proxy
  (`docs/reports/capsule-go-module-proxy-design-2026-10-14.md`); deployed at
  3439462a.
- **`selfdev-goproxy-1802`** (18:02, staging 865dc152): the module proxy
  worked (513 requests, 513 hits, 0 misses, 331.8 MB). Freeze not reached:
  no `gcc` on the capsule PATH, then a `cc1` SIGKILL (likely the 1 GiB
  cgroup), then missing ICU headers, then a link still running at 13 min 43 s
  when the 30-minute drill ended. 59 cells, about 40 of them starting,
  polling or repairing the build. Recorded in
  `docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md`.

Freeze has therefore never been reached on the records path. The last
candidate that reached `applied` and was restored on staging is M11 rerun
`docs/evidence/m11-rerun-2026-10-10T14-46-02Z.json` (computer
`computer-44f4417211a0adfac159a84d4c5c7176`), on the packet-era path B that
C1e B1 deleted. Every step after Freeze (management's Verify on records, the
owner decision, apply, restore) is unproven on the code that runs today.

## 3. Blockers, in the order the loop meets them

### B0. `self-dev start` is a dead command (no deploy)

What: the CLI and MCP tool call a route that answers 405. Fix: delete the
command and tool, correct `skills/choir-cli/SKILL.md` and the parity report;
point every agent at `choir run start`. Branch: none yet. Deploy: none (the
CLI builds locally; there is no hosted MCP). Probe: none needed beyond
`registry_test.go`. Until it is deleted, an agent following the skill will
call it and get 405.

### B1. Freeze is unreachable: the capsule cannot produce `bin/autoputer`

Fix: branch `worktree-agent-a03d8d5f7282257c5`, local only (not on
`origin`):

- 7645f293f, Freeze reuses the running binary (`/proc/self/exe`) when no
  changed path is a Go input and the capsule's source pin equals the running
  binary's commit; the bundle records `release_binary` and management's
  Verify re-checks it against the running build;
- a80a3ea04, the guest's `gcc`, `binutils`, `pkg-config` and ICU cgo
  environment in engineering capsules (`CHOIR_CAPSULE_BUILD_PATH`, appended
  after PATH at every hop);
- 0660372af, engineering capsules get 2 GiB (measured relink peak 1.80 GiB,
  1.50 GiB stripped);
- 58eb04520 (problem doc) and 657d3d3d1 (do not cache a failed running-binary
  read). 5f7f923e9 is already on main as 60467bb27.

Landing work: rebase onto `origin/main` (80 commits behind). `git merge-tree`
shows conflicts in `internal/agentcore/tools_capsule.go` (both sides add a key
to the Freeze result map: `freezeSourcePathsKey` from the precommitment commit
0df64c8b8), `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`
(the branch rewrites the release-tree paragraph; main added the "Precommit
before you build" paragraph) and the problem doc. All three are mechanical.
Run `go test ./internal/capsule/... ./internal/runtimeprompts/...` and the
focused agentcore tests (`-run 'ReleaseBinary|EngineeringPrecommit|Budget'`)
after the rebase; this audit did not re-run them.

Deploy: red, a full platform deploy that rebuilds the guest image
(`nix/autoputer-vm.nix` changes) and the runtime and capsule broker. The
tracer mints a fresh computer, so it runs on the new image without waiting
for busy computers to refresh.

Probe:

```
node scripts/reeng_tracer.mjs --label selfdev-ab-<HHMM> --scenario engineering \
  --expect-sha <deployed sha> --timeout-min 30 --apply-min 10 --restore-min 12 \
  --require-served --keep-computer
```

B1 is repaired when the receipt reaches `operation_frozen` with a bundle
digest. Read `release_binary.origin` from the Freeze result in the trace
bundle: `reused_running` is the expected path for this frontend-only change.
If it says the reuse was refused because the source pin and the running
commit differ (the minted computer's seed comes from vmctl's commit, the
binary from the guest image's), the desk falls back to building; rerun with
`--timeout-min 60` and record it, because a cold Go build at one CPU is
tens of minutes.

### B2. The unexercised tail: Verify, approval, apply, served, restore

What: the same tracer run continues past Freeze. Its checks name each step:
`final_report_names_bundle`, `verified_against_named_bundle`,
`owner_approved`, `applied`, `one_planned_restart`, `route_receipt`,
`restore_to_pre_apply_head` and, with `--require-served`,
`served_change_then_restored`. Branch: none; failures here are new findings.
Known risks from the earlier path: the apply's checkpoint starved by resumed
work (`docs/archive/problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md`),
zombie operations pinning a computer busy
(`selfdev-zombie-operations-pin-owner-computer-2026-10-10.md`, open), and
`releaseBinaryStillRunning` refusing a pass if anything restarts the computer
between Freeze and Verify (a mid-run platform deploy would; the tracer's
`--expect-sha` fence marks that run contaminated, exit 5). Deploy: none
beyond B1. Probe: the B1 command; full pass is exit 0 with
`acceptance.pass: true`. Per Problem Documentation First, the first failure
here gets a problem doc before any fix.

### B3. No export from a frozen bundle to GitHub

What: no route serves `source.patch`. Fix: the export route and CLI command
in section 1.4. Branch: none; to be written (orange: a new read-only product
route on guest and proxy, a new CLI command). Deploy: platform (guest runtime
and proxy). Probe: there is no tracer flag for it yet. After a passing
engineering drill run with `--keep-computer`:

```
choir self-dev patch --computer <id> --operation <op> --out /tmp/<op>.patch
shasum -a 256 /tmp/<op>.patch          # equals source_patch_sha256
git apply --check /tmp/<op>.patch      # in a checkout at source_patch_base_commit
```

A tracer `--export-patch` step that does the same and records the result in
the receipt is the follow-up that makes it a one-command probe.

### B4. Go changes do not run after apply (not for tomorrow)

What: the plain release path described in 1.3, plus the build budget for Go
changes: one CPU, a cold `GOCACHE`, and 30 s exec and 60 s cell limits that
force cell-by-cell polling (follow-ups C and D in the capsule build problem
doc), and reuse across commits that differ only in non-binary inputs. Fix:
the S2-f residual (route patch-carrying bundles to the host builder, mint a
layered release), C and D. Branch: none. Probe: none exists; the tracer's
engineering story is frontend-only. Until then, a Go change made on a
computer is built and tested in the capsule and lands through B3's PR.

### Off the critical path, land in their own slots

**Go module proxy cache root** (25bed70cc on
`selfdev-drill-goproxy-2026-10-14`, local only; merges cleanly). The size walk
and eviction do not follow the systemd state-directory symlink, so the cache
reports 24 bytes and an eviction would delete the link and break module
fetches until a restart. Latent: it needs about 8 GiB of new modules, and a
frontend-only drill never touches the proxy. Deploy: host only (Node B
switch restarts `go-choir-goproxy` and re-warms). Probe: `/healthz` reporting
about 439 MB instead of 24 bytes is a host read (operator, not an agent: no
SSH from agents); the product-path probe is the first Go-touching drill.

**Desk-cell confinement** (790d5e2c1, 0400a37dd, fedb7656c on
`fix/desk-cell-confinement`, local only; conflicts with main in
`internal/vmmanager/manager.go`). Management, Texture and research cells run
as guest root today; the branch confines them (own users, no_new_privs,
Landlock, seccomp, empty network namespace) and adds host-side guest egress
policy. It is not on the drill's failure path, but it is the precondition for
running agent-driven self-development on any computer whose state matters:
management's cells drive Verify, and guest root can reach the updater root
where frozen bundles wait. Deploy: red, guest image plus vmctl, in the
foundations order's slot 6 (`docs/ACTIVE.md`). Probes: the staging Landlock
proof its doc says is owed, then the same engineering tracer command as B1
(regression: Verify must still pass under the confinement), then
`node scripts/reeng_tracer.mjs --label confine-paper-<HHMM> --scenario paper
--expect-sha <sha>`.

## 4. What the owner does and what an agent does

The rule is the conservative default: agents propose, read, test and report;
they never approve, arm a mode, or restore. Making `approve` and `mode set`
CLI-only (5b17bb572) keeps them off MCP, but the server's real guard is the
API key's scopes: the proxy checks `computer:self_development:approve` and
`:mode` on every key. A key that carries those scopes can approve through
`curl` whatever the CLI does. So the boundary is the key the agent holds.

Two facts make that boundary procedural today, not technical. First,
`scripts/qa_test_computer.mjs` mints one key per test computer with every
self-development scope, approve and mode included, and also saves the test
owner's browser session; both sit in `~/.config/choir-qa/test-computers/` on
the same machine the agent uses. Second, the engineering tracer arms
`propose_only`, arms `accept_once` and posts the approval itself, as the
owner of its disposable computer. That is the acceptance harness doing what
the drill claims, not an agent approving product work, but it should be ruled
on explicitly.

**The owner, tonight or first thing tomorrow:**

1. Choose the deploy slot for B1 (red). Recommendation: the next deploy after
   the one in flight. `docs/ACTIVE.md` lists the self-development rerun as
   "alongside, no deploy"; it cannot pass without B1's deploy.
2. Rule on drill self-approval: the engineering tracer may approve on the
   computers it mints (recommended), or the owner runs the approval step of
   every drill.
3. Approve the B3 design: a read-only patch route, and agents may push
   `selfdev/*` branches and open draft PRs, never merge. Merging stays the
   owner's act on GitHub.
4. Approve the command cuts in section 5 (delete `self-dev start`).

**The owner, during the loop:** approve or reject each candidate from their
own terminal, with their own key (`choir self-dev approve --computer <id>
--operation <op> --api-key-file <owner key>`); there is no approval UI in the
frontend today, despite `ownerActReason` naming one. Arm `propose_only` on any
computer the tracer did not arm. Restore when a change must be undone. Merge
PRs.

**An agent can:** rebase B1 (and later B3) and run the tests; push and deploy
them once the owner names the slot (the Landing Loop); run the tracer; mint
test computers; start requests with `choir run start`; watch operations;
pin a pre-apply checkpoint (`choir computer checkpoint`, not destructive);
reject its own bad candidate (`self-dev reject` is fail-safe and frees a
computer a zombie operation would pin); export patches and open draft PRs
after B3; record and settle its precommitments.

**Recommended key split (a yellow script change, no deploy):** have
`qa_test_computer.mjs` mint two keys: an agent key (`read:texture`,
`write:texture`, `read:runtime`, `write:runtime`, `read:base`,
`computer:lifecycle`, `computer:self_development:read`) and an owner key
(adds `:mode`, `:approve`, `:rollback`), and give the agent's MCP server only
the agent key. Note that the CLI's `--api-key-file` wants a bare 0600 key
file, while the mint script writes a JSON credential file; extract the key
into its own file. Neither key carries `precommit:*` scopes, which section 6
needs on a different computer.

**Which computer:** a fresh test computer per change, minted by the tracer
or `qa_test_computer.mjs`. Its source seed and its running binary come from
the same deployed commit, which is what B1's binary reuse requires; a
persistent computer's seed stays at its creation commit while its guest
image moves, so reuse refuses there after the first guest update. It also
keeps the paper computer and the owner's computer out of the blast radius.

## 5. Commands the loop needs (recommendation)

The CLI has 59 commands; 46 of them are MCP tools. The loop needs about ten.

**Agent, for the loop:** `run start` (the only working start), `run status`
and `trajectory` (watch the request), `self-dev list` (find the operation:
its `trajectory_id` equals the run's), `self-dev show` (poll state; on the
CLI, `self-dev wait --state awaiting_approval` does the polling),
`lifecycle capsule-evidence` (what engineering ran), `computer checkpoint`
(pin the pre-apply state), `self-dev reject`, `trace export` (evidence), and
`precommit add|settle|list|calibration`. After B3, `self-dev patch`.

**Owner, CLI only:** `self-dev approve`, `self-dev mode set`, `computer
restore`.

**Recommendation:**

- Delete `self-dev start` and its MCP tool now; it calls a route that no
  longer exists.
- Narrow `self-dev mode set` to `off`, `audit_only` and `propose_only`.
  `approve` already arms `accept_once` with the exact binding, and its eight
  binding flags on `mode set` are a second, error-prone way to do the same
  thing. `qualified_consensus` served the M11 path; keep it API-only until a
  live policy needs it.
- Take the operator and forensic commands off the default MCP tool list
  (`computer replay-completeness`, `replace-workspace`,
  `import-residue-snapshot`, `producer-reports`, `settle-producer-reports`,
  `recover`, `refresh`, `restart`, `stop`, `start`, `lifecycle snapshot`,
  `lifecycle events`) and the ones unrelated to this loop (`search`,
  `files *`, `texture create|revise|revisions|open-source`, `api-key *`).
  Keep them on the CLI. Expose them through an explicit profile (for
  example `choir mcp --profile operator`) rather than by default, so an
  agent doing self-development sees a short, purpose-built list. This is a
  recommendation for the owner's ruling, not a census; do not delete
  operator commands without one.
- The `computer:self_development:propose` scope guards only the dead route.
  Either retire it or make the prompt bar require it when a request will
  reach engineering on an armed computer; until then, `write:runtime` is the
  real proposal authority.

## 6. Precommitments: what each step predicts and how it settles

Two layers, and they should not be mixed.

**Engineering's own claims (inside the computer, settled by rule).** Since
0df64c8b8 (on `origin/main`, deploying with 5b17bb572) the engineering
overlay asks the desk, in its first cell, to stage one `choir.Precommit` per
claim with p(yes) in [0.02, 0.98] and a closed rule kind. The runtime settles
each from receipts it already holds, never from the model
(`internal/types/commitment_rules.go`); a settlement is a resolve record by
scorer `system:rule`, first settlement wins. For a change like the drill's:

- `tests_pass`: the test receipts the Freeze cites all exited 0. Settled at
  Freeze. The cell must fail when its test fails (failure mode R7), or a
  failing test reads as passing.
- `paths_confined` with the directories the change names (for the drill,
  `frontend/public/choir-drill`): settled at Freeze from the changed source
  files. This is the claim that catches `node_modules` or `dist` left in the
  workspace, which is the same defect that would poison an exported patch.
  Predict it honestly; a "no" here is useful before B3 exists.
- `freeze_within` N minutes of the assignment opening: settled at Freeze, or
  at the terminal transition as "timeout". With B1's reuse, 20 minutes is a
  fair bound for a frontend change; a Go change without the C and D
  follow-ups should predict low.
- `verify_pass`: settled by management's Verify; stays open while a
  completed candidate waits for it (R10).
- `completes`: settled at the assignment's end.

The desk reads its settlements through `choir.Pack`. The outside agent and the
owner cannot yet: `choir precommit list` reads only calibration-schema
records, and the say–do view (`types.RulePrecommitSayDo`) has no route.
Residual: a read route (for example `GET /api/precommit/say-do?agent=engineering:`)
so the supervision view the commit describes is visible.

**The outside agent's claims (Claude Code, through `choir precommit`).** One
claim per step, made before the evidence exists, with the settlement rule
written into `--criterion` so settling is a mechanical read:

1. Management Casts engineering within 5 minutes of `run start` (settles from
   the tracer phase `management_cast_engineering` or the inbox record).
2. The operation reaches `frozen` within 30 minutes, with
   `release_binary.origin` `reused_running` (operation state plus the Freeze
   result in the trace bundle).
3. It reaches `awaiting_approval` with at least one verifier ref
   (`self-dev show`).
4. After the owner's approval, it reaches `applied` through exactly one
   planned restart with reason `self_development_apply` and no crash lines
   (tracer check `one_planned_restart`, plus `route_receipt`).
5. The change is served after apply and gone after restore (tracer
   `served_change_then_restored`).
6. After B3: the exported patch hashes to the bundle's `source_patch_sha256`
   and applies to its base commit.
7. CI on the draft PR passes (the `gh run` conclusion).
8. After the owner merges: staging reports the merge commit and the change
   is served on a freshly minted computer.

The owner's act is not one of the agent's predictions. Settle with `choir
precommit settle <id> --outcome true|false --evidence <receipt path or run
id> --method rule --scorer tracer:<check>` when the outcome is a field the
tracer or a route reports, `--method self` otherwise; the governing order is
rule, owner, independent, self. Two practical constraints: claims live on
one computer (`ListCalibrationRecords` filters by computer), so recording
them on disposable test computers scatters the track record; record them on
one persistent computer (the owner's) with a key that carries only
`precommit:read` and `precommit:write`, and name the test computer and
operation in the claim. And set `CHOIR_AGENT`, `CHOIR_MODEL` and
`CHOIR_PROJECT=selfdev` so the calibration summary groups them; paste
`choir precommit calibration --text` into the session before stating new
probabilities.

A follow-up worth making: let the tracer write rule settlements for claims
that name its checks, so the harness, not the acting agent, settles them.

## 7. Tomorrow's sequence

1. Owner: name B1's deploy slot; rule on drill self-approval, B3 and the
   command cuts.
2. Agent: rebase B1 onto `origin/main`, resolve the three conflicts, run the
   focused tests, push, monitor CI and the deploy, confirm staging reports the
   SHA.
3. Agent: run the B1 tracer command. If it fails, write the problem doc
   first, then fix (B2).
4. Agent: write B3 (route, proxy mapping, CLI command, tracer
   `--export-patch`), problem-and-design note first; land in the next slot.
5. The loop, per frontend change: mint a test computer (agent key only in
   the MCP config), record precommitments, `choir run start`, watch the
   operation, pin a checkpoint, ask the owner to approve, confirm served,
   export the patch, open a draft PR, settle the claims.
6. Go changes: same loop through Freeze and the PR; do not count an apply as
   live until B4.

## Evidence and references

- Code: `cmd/choir/commands.go` (registry; `self-dev start` at line 568,
  `ownerActReason`), `internal/agentcore/api_self_development.go:164`,
  `internal/proxy/self_development.go` (scope checks, route classifier),
  `internal/agentcore/tools_capsule.go` (Freeze),
  `internal/capsule/executor.go` (`StageGrantedRelease`, `emitSourcePatch`),
  `internal/capsule/transaction/builder.go` (bundle schema),
  `nix/autoputer-vm.nix` (`autoputerRuntimeExec`),
  `internal/vmmanager/manager.go:2840` (source seed),
  `internal/types/commitment_rules.go`,
  `internal/agentcore/engineering_precommit_rules.go`,
  `scripts/reeng_tracer.mjs` (`engineeringScenario`),
  `scripts/qa_test_computer.mjs` (key scopes).
- Problem docs: `docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md`,
  `c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md`,
  `goproxy-cache-root-is-a-symlink-2026-10-14.md`,
  `desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md`,
  `s2-selfdev-runtime-changes-binary-carried-2026-10-04.md`,
  `selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md`,
  `selfdev-zombie-operations-pin-owner-computer-2026-10-10.md`.
- Receipts: `~/.config/choir-qa/traces/{c1e-b1-engineering-0444,c1e-b1fix-engineering-0549,c1e-d4-engineering-0648,selfdev-goproxy-1802}/receipt.json`;
  `docs/evidence/m11-rerun-2026-10-10T14-46-02Z.json`.
- Branches (all local to the owner's machine, none on `origin`):
  `worktree-agent-a03d8d5f7282257c5` (657d3d3d1),
  `selfdev-drill-goproxy-2026-10-14` (25bed70cc),
  `fix/desk-cell-confinement` (fedb7656c).

Heresy delta: discovered, `self-dev start` and its MCP tool and skill entry
advertise a deleted route; the `:propose` scope guards nothing while
`write:runtime` is the real proposal authority; the owner-act boundary is a
key-scope boundary that the test-computer mint script does not draw; the
"Choir UI" approval path named in `ownerActReason` does not exist; the
engineering say–do record has no read route. Introduced: none. Repaired:
none (analysis only).
