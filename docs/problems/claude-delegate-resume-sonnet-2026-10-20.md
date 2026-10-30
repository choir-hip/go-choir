# Claude delegate resumes on Sonnet (2026-10-20)

Owner, 2026-10-20 (owner notes §28): "we should resume using Claude
(sonnet) within Eng desk (shelling out). We set that up then stopped it.
Let's resume that". This is the first step toward rewriting the
less-important apps through self-development, which waits on app packages.

## State found

- The delegate is built and in the tree, switched off by an empty
  allowlist (431dc5d3b, 2026-10-15). The reason was quota, not safety:
  "I'm at 87% of my token limit" (owner notes §21). Design:
  `docs/design/claude-delegate-slice1-2026-10-14.md`; research and terms:
  `docs/reports/claude-agent-sdk-in-choir-2026-10-14.md`.
- Shape: the engineering desk's implementation capsule runs unmodified
  `claude -p` through `choir-delegate` and `capsule-broker`, with egress only
  to `api.anthropic.com:443` and the Go module proxy. The token reaches the
  `claude` process only as `CLAUDE_CODE_OAUTH_TOKEN`; engineering re-runs the
  tests itself.
- `internal/claudedelegate/spec.go` pins `Model = "claude-opus-5-5"`.
- `nix/node-b.nix`: `CHOIR_CLAUDE_DELEGATE_COMPUTERS=` (empty) and
  `CHOIR_CLAUDE_DELEGATE_EXPIRES=2026-10-22T00:00:00Z`, two days out.
- The token file `/var/lib/go-choir/secrets/claude-oauth-token` is still
  installed (root, 0400, 108 bytes, written 2026-10-15 01:54 UTC; checked by
  `stat`, never read).
- The allowlisted test computer `computer-33594053a05f46e93d80e7e15ce8a0d2`
  is not running (absent from the vmctl list) and was not removed by the
  2026-10-20 tape sweep (no archive), so it boots on demand.

## Change (red)

1. `Model = "claude-sonnet-5-5"`.
2. Re-add the test computer to `CHOIR_CLAUDE_DELEGATE_COMPUTERS`.
3. Move the hard expiry to 2026-11-03T00:00:00Z: two weeks, renewed by
   an explicit edit, so a forgotten grant still lapses.

- **Conjecture delta.** The delegate on Sonnet does useful engineering work
  inside the capsule at a quota cost the owner accepts. Refuted if a run
  fails on the model name, the capsule kills `claude` (seccomp), or one
  run spends a visible share of the subscription.
- **Protected surfaces.** Provider calls (Anthropic, outside the gateway),
  the credential disk path in vmctl, capsule network policy. No change to
  Texture writes, Trace, checkpoints, auth or routing.
- **Admissible evidence.** One staging run on the test computer:
  `choir-delegate` status reports `available: true` and model
  `claude-sonnet-5-5`; one small delegated task finishes with
  `subtype: success`, and engineering's own test re-run passes.
- **Rollback.** Empty the allowlist and deploy (the grant lapses at the next
  boot, and in any case at the expiry); revoke the token if exposed.
- **Heresy delta.** Discovered: none. Introduced: none (the exception is the
  owner-ratified option T, §14/§19). Repaired: none.

## Open risks carried from the slice

- The seccomp fix for the Bun timer (fea6d7c5b,
  `docs/archive/problems/claude-delegate-bun-timer-seccomp-2026-10-15.md`) was
  verified on aarch64 in Docker only; this is its first staging run on
  x86_64.
- Claude's own Bash tool fails inside bwrap without `CAP_SYS_ADMIN` (design
  §6); delegated tasks rely on file edits, and engineering runs the tests.
- Landlock isolation of `/proc/<pid>/environ` (F13) is unverified on the
  staging kernel.
- Terms: the token stays in the unmodified binary's environment, for the
  owner's own computer, never relayed or shared. Re-read the terms wording
  before any second computer is allowlisted.
- Quota: the subscription that ran out on 2026-10-15 is the same one.

## Finding after deploy (2026-10-20 10:00 UTC): the allowlisted computer is gone

4c2d66c38 deployed and vmctl carries the allowlist and the new expiry. The
cold boot then failed: the test computer's key gets `403 api key computer
ownership required`, and `GET /api/computers/computer-33594053…` answers
404. The computer's ownership record is gone; its tape is still on disk
because it has not yet been quiet for the tape sweep's 7 days, which is why
the 2026-10-20 sweep did not list it. The test computer
`v3-events-0146` shows the same pattern (403, not in the vmctl list).
Cause of the removal not traced here.

Replacement: `scripts/qa_test_computer.mjs --label
claude-delegate-sonnet-20261020` minted `computer-ad1b46d4c20129e84b9e239cf4720fb9`
(guest build 4c2d66c38, key expiry 2026-10-27) at 10:01 UTC. It replaces the
dead id on the allowlist; the proof runs on it after a cold boot.

## Staging proof (2026-10-20 10:11–10:12 UTC, on 09e98e9b2): delegate runs, its edit does not reach engineering

Test computer `computer-ad1b46d4…` cold-booted through the lifecycle API
(stop 10:09:45, start 10:10:04). `scripts/claude_delegate_probe.py`:

- `--status-only`: pass. Guest status `available: true`, model
  `claude-sonnet-5-5`, expiry 2026-11-03, guest build 09e98e9b2.
- End to end: **fail**. Trajectory `366115a4-…`, engineering run
  `run:assignment-c64a301390f01c2d40e7f15e`, capsule network mode `claude`.
  - Engineering (gpt-6-luna) precommitted, called `choir-delegate start` and
    `wait`. Delegate run `d20261020T101200-73eeba02` ended `state: done`,
    `subtype: success`, 5 turns, 13 s, $0.039, last tools `Write, Bash,
    Bash, Read`. So the Bun seccomp fix holds on x86_64 staging.
  - stderr tail: "Permission mode forced to default —
    CLAUDE_CODE_SUBPROCESS_ENV_SCRUB is set (allowed_non_write_users
    hardening). Declare allowedTools explicitly, or set
    CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=0 to opt out." `Args` passes
    `--permission-mode dontAsk` (spec.go:163) and the env sets the scrub
    (spec.go:231); the CLI overrides the mode.
  - Engineering then ran `cat docs/probes/claude-delegate-20261020T101114.md`
    in its own cell: **No such file or directory**. It completed the
    assignment `blocked`, citing the delegate's report that its own `cat`
    checks failed because bwrap was unavailable (design §6, known).
- **Not yet known (hypotheses, the delegate's event log in the capsule's
  run dir would decide):**
  1. The forced `default` permission mode denied the Write in `-p` mode
     (look for `permission_denials` in the result).
  2. The delegate stage writes to a different view of `/workspace/platform`
     than the engineering cells see (broker namespace vs cell overlay), so
     a successful Write is invisible to engineering.
- Next probe: read the delegate's `events.jsonl` / result
  `permission_denials` through `choir-delegate status --id …` from an
  engineering cell, before any change. A fix for (1) is to drop the scrub
  variable or declare the tools as the message says, weighed against why
  the scrub was set (token isolation from Claude's subprocesses, F13).

### Hypothesis 1 refuted locally (2026-10-20 ~10:20 UTC)

Local Claude Code 2.1.299 with the delegate's exact flags (`-p`, Sonnet,
`--tools`/`--allowedTools`/`--disallowedTools` as in spec.go, empty MCP,
`--permission-mode dontAsk`) and `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1` prints
the same "Permission mode forced to default" warning, then writes the file:
`subtype: success`, `permission_denials: []`. Write is on the explicit allow
list, so the forced mode does not refuse it. The guest's Claude Code version
may differ; the evidence moves to hypothesis 2 (the delegate and the cells see
different views of `/workspace/platform`), under investigation.

## Root cause (2026-10-20, code reading; staging confirmation pending)

Both open hypotheses are wrong. Hypothesis 1 was refuted by the local repro.
Hypothesis 2 (a different filesystem view) is refuted by code: the delegate
and the cells share one chroot, one mount namespace and one overlay. The
difference is the **working directory**, which turns a relative path in the
engineering brief into two different files.

**Same view, by code.**

- The executor starts the capsule's broker once, with `Chroot: caps.MergedDir`
  and one `CLONE_NEWNS` (`internal/capsule/executor.go:608-610`); the merged
  dir is the overlay of the pinned source lower layer plus the capsule's
  upper (`executor.go:394-453`), and the broker is told `--merged /`
  (`executor.go:595`). Every process the broker starts is in that root.
- The delegate stage is a child of that broker: `startDelegate` launches
  `supervisedCommand(... "--isolation-stage", delegateStage ...)` with
  `cmd.Dir = /workspace/platform` (`cmd/capsule-broker/delegate.go:66-71`),
  applies `NewWorkloadLandlock("/")`, then `os.Chdir(/workspace/platform)`
  and `exec` (`delegate.go:106-121`). No `unshare`, `pivot_root`, bind or
  second overlay anywhere on that path.
- The cells run in a session worker the same broker spawns with
  `allowedRoot: b.mergedDir`, i.e. `/` (`cmd/capsule-broker/session_worker.go:342`),
  under the same `NewWorkloadLandlock("/")` (`cmd/capsule-broker/main.go:169`).
  The delegate and the cells are also one capsule: the assignment spawns a
  single `SpawnSpec` with `ClaudeDelegate: claude`
  (`internal/agentcore/engineering_assignment_runtime.go:452-459`).

**Different working directory, by code.** A cell's `choir.Exec(command, args)`
sends `ExecPayload{Command, Args}` with no `Dir` (`internal/yaegikernel/choir.go:538`),
and the broker then sets `cmd.Dir = b.cfg.AllowedRoot` (`internal/yaegikernel/broker.go:195-197`),
which is `/`. Relative `ReadFile`/`WriteFile` paths resolve against the same
root (`broker.go:421-425`). `SpawnSpec.WorkingDir = "/workspace/platform"`
(`engineering_assignment_runtime.go:458`) is set but read nowhere. This is the
already-known F6 / option E of `selfdev-freeze-receipts-refused-2026-10-15.md`
("`choir.Exec` runs at `/`", deferred), and the engineering overlay already
says so (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`,
step 3 uses absolute `/workspace/platform`).

**What happened.** The probe's brief (`scripts/claude_delegate_probe.py:221-227`)
names a relative target `docs/probes/claude-delegate-<stamp>.md` and tells
engineering to "read the file yourself with cat in your own cell". The delegate
(cwd `/workspace/platform`) wrote `/workspace/platform/docs/probes/...`;
engineering's `cat docs/probes/...` ran at `/` and looked for
`/docs/probes/...`. The delegate's isolation and the Write both worked; the
check was aimed at the wrong path. The product gap is that nothing in the
delegate's result gives the desk an absolute, broker-verified path to what the
delegate wrote, so a relative path in a brief is silently ambiguous.

**Confidence.** High that the views are shared and that the cell cwd is `/`
(read from the code that runs). Not yet confirmed on staging that the file
exists at `/workspace/platform/docs/probes/claude-delegate-20261020T101114.md`:
that capsule was reclaimed, so the proof is a re-run, below.

**Cheap staging confirmation.** The fix below makes `choir-delegate wait`
report `written_files` (absolute path, `exists`, `bytes`), stat'ed by the
broker in the capsule's own view when the run ends. After deploy, the probe
with an absolute target should show `exists: true` and engineering's `cat`
of the absolute path should print the line. A one-off before the fix: in an
engineering cell, `choir.Exec("ls", []string{"-l", "/workspace/platform/docs/probes"})`.

**Fix (red, `internal/claudedelegate` is broker-side delegate custody code).**
1. The server records the `file_path` of every `Write`/`Edit` tool call in the
   stream, resolves it against `/workspace/platform`, and when the run ends
   stats each path from the broker side (`written_files` in status.json).
2. The engineering overlay tells the desk to use those absolute paths
   (cells run at `/`).
3. The probe script's target for engineering's own `cat` is absolute.
No change to Landlock, capabilities, seccomp or egress, and no new gate.
Option E (default `choir.Exec` cwd) stays deferred: it changes every
existing cell's relative paths.

- **Heresy delta.** Discovered: none new (F6 is known). Introduced: none.
  Repaired: none.

**Substrate residual (counted, 2026-10-20).** This is the second failure
from one missing default: cells run at `/` because `choir.Exec` sends no
`Dir` and `SpawnSpec.WorkingDir` is never read. The first was F6 in the
Freeze drill (`selfdev-freeze-receipts-refused-2026-10-15.md`). Both were
answered with overlay text. A third occurrence triggers Root Cause
Clustering: the next action is option E itself (cells default to
`/workspace/platform` in engineering capsules, with an audit of overlay
recipes that rely on `/`), not more overlay text. The overlay also named
the delegate "Claude Opus 5.5"; it now says Sonnet 5.5, matching spec.go.

## Staging rerun (2026-10-20 10:42–10:45 UTC, on ec2dc7b7d): delegate runs, management's Cast drops the brief

Test computer cold-booted (lifecycle stop/start), guest build ec2dc7b7d,
status `available: true`, model `claude-sonnet-5-5`. Trajectory
`5ce07bc9-…`, engineering run `run:assignment-d3d91666c7948ab1741c3161`,
capsule network mode `claude`. Read from the run's events, in order:

1. Management's Cast to engineering carried only the objective "Run the
   specified Claude delegate probe and verify; do not freeze or land
   changes." and the spec `{"parent": <Texture cast record>}`. The probe
   brief's target path, line and test command were not in it.
2. Engineering precommitted, then briefed the delegate with a generic
   objective (`paths ["."]`, test `go test ./...`), because it had no
   concrete task.
3. Delegate run `d20261020T104314-f34b5796`: `state: done`,
   `subtype: success`, 9 turns, 19 s, $0.078, tools Bash, Glob, Grep, Read;
   no Write or Edit, so `written_files` is absent, which is correct.
4. Engineering ran `go -C /workspace/platform test ./...` twice in cells with
   `timeout_ms: 30000`; both returned `failed to read broker response … i/o
   timeout` (the whole-repo test cannot finish in 30 s; the known
   broker-read-timeout class, P5 residual).
5. Engineering completed `blocked`, saying the delegate "did not perform
   requested probe" and naming the run id, session, turns and cost.

So the delegate path works end to end twice (Bun seccomp fix holds; the
token, egress and status are sound). The fix's own proof (`written_files`
reporting an absolute existing file) is not yet observed, because no write
was asked for.

**Cause (trace-confirmed): engineering never receives the request.** The
Cast's `parent` names Texture's Cast to management, which engineering
cannot read (`choir.Records()` in engineering shows only management's Cast,
whose body is the `parent` pointer). This is P5 cause B
(`p5-go-served-0825-engineering-never-freezes-2026-10-15.md`: "spec passed
by reference", seen in 0101 A3 and 0825 #1), whose fix 2 ("inline the
parent's body into the assignment record") was proposed and never built.
The management overlay does not tell management that engineering cannot
read the parent.

**Root Cause Clustering.** Three occurrences across two weeks, all
answered or left as prose. The next action is substrate: the engineering
Cast delivers the parent Texture Cast's objective and body to engineering
(inlined in the assignment's inbox record, digest-bound), so engineering
reads the owner's request, not management's summary of it. Overlay text
alone is not the fix.

- Mutation class of the fix: red (Cast authority, engineering inbox
  record).
- Rollback: revert the fix commit.
- Proof: the same probe on the test computer; pass is engineering's
  `cat /workspace/platform/docs/probes/<stamp>.md` printing the line and
  `written_files[0].exists: true`.

## Fix design: engineering reads the Texture request (written before the code)

**Found, then chosen.** Nothing existing carries the parent's text to
engineering. `ParentID` on the Cast record and `ParentConversationID` /
`ParentRound` on the view are pointers only. The one precedent is the
read-time join in `loadCellRecords` (`delegatedCaptureViews`, R2-14): a
management task's opening Cast shows the records it delegated, joined from
the immutable tape when the view is built. This fix uses that pattern and adds
no stored field and no process-local state.

**Mechanism.**

1. `planEngineeringCast` already resolves the parent and checks it is a
   Texture's Cast in management's inbox with an open round. That check moves
   into one helper, `engineeringCastRequest`, which also returns the parent's
   request: `{from: parent.SenderInbox, objective, body}` verbatim, and
   `ParentDigest`, the sha256 over the full, untruncated text with a
   domain-separation prefix.
2. The parent digest is folded into the binding's `RequestDigest` (the same
   join that already holds the objective, parent id and Cast id). A replay of
   the same Cast with a different parent text computes a different binding, so
   the open command's digest differs and the replay is refused by the existing
   idempotence check. Parent records are immutable, so a legitimate replay
   always recomputes the same digest.
3. When `loadCellRecords` builds engineering's view of a Cast whose parent is a
   Texture Cast, it loads the assignment binding for the inbox, reloads the
   parent through the same helper, recomputes the `RequestDigest` and compares
   it with the committed binding. Equal: the view carries `request` (from,
   objective, body, `digest`, `truncated`). Unequal, or the parent is not a
   valid Texture Cast: no request, and a visible `request_error`. A pre-fix
   binding (legacy digest) shows `request_error: "assignment predates request
   binding"` rather than unbound text.
4. Size: the digest covers the full text; the view cuts objective at 4 KiB and
   body at 16 KiB on a rune boundary and says so (`truncated: true` and a
   marker line naming the parent record id, which stays on the tape).
5. Only management's Cast that opened the assignment (`record_id ==
   binding.ParentControlID`) gets a request. Engineering gets no read access to
   management's inbox, and Texture's and management's authority are unchanged.

**Ways it could fail, and the handling (each has a test).**

- Lost on replay or crash restart: nothing is process-local. The view is
  derived from the tape (parent record, binding) at every read, so a fresh
  runtime reading the same store shows the same request. Test builds the view
  from a store that never ran the plan.
- Digest mismatch refusing legitimate replays: the digest is over immutable
  parent text and the same Cast id, so replays agree. Tests: same inputs give
  the same `RequestDigest`; a different parent text gives a different one; an
  identical re-plan produces an identical binding.
- Oversized parent: truncation is visible and bounded; digest still covers the
  full text. Test with a body larger than the limit and a multibyte boundary.
- Untrusted text in engineering's prompt: the text enters only through
  `choir.Records()` (REPL data, like every record), never the system prompt,
  the run objective or the delegate brief. The overlay says to treat it as the
  request, not as instructions that widen authority. Test: the assigned run's
  objective stays management's objective only.
- Parent not a Texture Cast (a Note, a Cast from another sender, another
  desk's inbox): the existing refusal remains in the plan; the view shows
  `request_error` and no text. Tests for both.
- Crash restart: a restart closes work, never resumes it, so no resumer is
  added. A planned-restart resume re-reads the same tape.

**Red ceremony.**

- Mutation class: red (Cast authority, engineering inbox record).
- Conjecture delta: engineering failed these probes because the request never
  reached it, not because it lacks the capability (trace: the three
  occurrences in this doc and P5 cause B). Prediction: with the request
  visible, the same probe has engineering write the probe file, and
  `written_files[0].exists` is true. Refuted if engineering still lacks the
  target path with the request in its `Records()`.
- Protected surfaces: engineering's inbox record view, the assignment binding's
  `RequestDigest`, `planEngineeringCast`. Not touched: Texture's canonical
  writes, vmctl, gateway, auth, route projection, capsule code.
- Admissible evidence: focused Go tests (plan digest, view join, replay,
  truncation, non-Texture parent, legacy binding); then staging only: the same
  delegate probe on the test computer. No local proof is claimed for the live
  path.
- Rollback: revert the fix commit. Bindings already minted keep their digest
  (an opaque hash), so no data migration. A reverted build shows no `request`
  and recomputes nothing.
- Heresy delta: discovered: engineering's Records() views a Cast whose
  `parent` it cannot read (P5 cause B, now three occurrences);
  introduced: none expected; repaired: P5 cause B on the substrate (overlay
  text alone was not the fix).

## P6. The delegate's network mode cannot build a frontend change (code reading, 2026-10-21 00:20 UTC)

This was found while setting up the overnight two-arm drill (owner notes
section 32: the same frontend rewrite with Sonnet through the delegate,
and with GPT-6 Luna).

- On a computer on the delegate allowlist, an engineering assignment runs
  in network mode `claude`. That mode allows `api.anthropic.com:443` and
  the host Go module proxy, and nothing else
  (`internal/agentcore/engineering_assignment_runtime.go:380-386`;
  `rlm_engineering_runtime.yaml` line 9).
- The frontend build job is `npm ci && npm run build`, and Freeze needs
  the built release tree. npm is refused in `claude` mode.
- A Cast that asks for `ecosystem_proxy` gets npm, but `ClaudeDelegate`
  is set only in `claude` mode (`engineering_assignment_runtime.go:452`).
  So one round can have the delegate or npm, never both.
- No recorded run combines the two. This is a code reading; staging has not
  shown it yet.

Options, not decided:

- **(a) Two rounds, no egress change.** Round 1 runs in `claude` mode: the
  delegate writes the change and the round completes as failed (no
  build). Round 2 runs in `ecosystem_proxy`, receives round 1's diff as the
  failure 15 carried patch (option A, built 2026-10-21), and builds,
  re-tests and freezes it.
- **(b) Widen `claude` mode with the host ecosystem proxy.** This is red,
  because it changes the egress policy. It enlarges what a capsule holding
  the delegate token can reach.
- **(c) A host-built `node_modules` seed for the base tree.** It would be
  mounted read-only, like the build-cache seed in failure 12.

Tonight's drill uses (a): it needs no egress change, and it is also the
first real proof of the carried patch. (b) and (c) wait for the owner.
