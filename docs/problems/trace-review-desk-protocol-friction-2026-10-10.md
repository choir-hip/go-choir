# Trace review: what the logs say about the desks and the setup (2026-10-10)

Status: open. The owner asked for a review of every agent trace and log,
past and current. The review covered:

- the QA computer consoles from 2026-10-09 12:00Z to 2026-10-10 07:40Z
  (24 VMs, owner computer excluded);
- the Node B gateway, vmctl, proxy and auth journals;
- all 21 acceptance receipts;
- M11 rerun 10's full run events (4 runs, 1,204 events).

Five read-only haiku reviewers did the first pass on local copies. Their
reports are scratch, not committed. Claims marked **verified** I checked
myself against the trace or the code. Claims marked **reported** come
from a reviewer and are hypotheses until verified.

## The headline

- **Reported:** 19 of 20 failed acceptance attempts failed first for
  platform, infra, provider or probe reasons, not desk behavior.
- **Verified:** the desks' main inefficiency is the protocol telling
  them *that* they are wrong but not *what is right*. They then guess.
  The setup changes with the most leverage are in the reducer and
  protocol surface, not the prompts.

## Findings

### F1. Desks guess enum values against the reducer (verified)

From 07:38:17 to 07:40:04Z in rerun 10, Texture run `e76cd867` made 16
consecutive `update_coagent` attempts. Each was rejected with only
`... is not supported`:

- action `type` guesses: `repair`, `command`, `edit`, `zzznotype`,
  `shell`, `task`, `self_development` and `patch`;
- `safety.mutation_class` guesses: `reversible`, `capsule`, `repo`,
  `none`, `capsule_mutation` and `workspace`.

The accepted sets (`run_command`, `produce_diff`, …; `green`…`black`)
were never shown. Texture then gave up on opening a repair assignment
and wrote the document without one. The reducer already lists valid
fields for unknown JSON fields; enum rejections did not.
(`internal/coagentpacket/packet.go`.)

### F2. Interpreter friction in the REPL (verified, rerun 10)

The implementation run made 104 turns and the verifier 86. Their cell
failures:

- top-level statements parsed as declarations (`expected declaration,
  found …`), 7 times;
- yaegi `variable definition loop` / `constant definition loop`, 2;
- `undefined: err`, 2;
- `import "crypto/sha256" refused: not in activation allowlist`, 1;
- `session worker read: frame: read header: EOF`, a session-worker
  crash, 1 in each engineering run.

Each costs a turn and resends the full context.

### F3. The freeze emitted an unappliable patch (verified, fixed in 1ae758ef)

See `selfdev-verifier-rejects-frozen-bundle-2026-10-10.md`. The
verifier caught it in rerun 10. The verifiers in reruns 8 and 9 passed
the same defect, so verifier contracts vary between runs (residual
`verifier-apply-check`).

### F4. Peer messaging is refused inside engineering (verified)

The verifier's `choir.Message` to the implementation desk was refused
with `update_coagent engineering cannot message engineering`, so its
findings travelled only in its completion summary. The rule routes owner
intent through Texture, which is correct for owner intent. Whether a
verifier should reach its implementer directly is a protocol decision,
recorded as residual `verifier-implementer-channel`. No change.

### F5. Texture exhausts its token budget during the apply stall (reported, code-checked)

Reruns 7, 8 and 9 logged `tool loop budget "texture:…" exhausted:
total tokens ~1.24M exceeded max 1200000` at 01:19:40, 03:00:27 and
04:30:22. The budget (`textureowner/tool_loop_policy.go`: 80 calls,
1.2M total tokens) counts the input tokens resent on every call. A
Texture run that keeps taking supervision turns while apply stalls
reaches it in about 60 to 80 calls. Spend also carries across resumes
(`actor_budget_spent_*`).

**Owner decision (2026-10-10): make the system work before constraining
it.** The caps were inherited defaults the owner never set. Desk token
and call budgets are raised 100x:

- Texture: 8000 calls and 120M tokens;
- engineering: 20000 calls.

The elapsed bounds stay (Texture 45m, engineering 60m) as the backstop
for a dead-worker loop. Uncontrolled loops are watched in the traces by
recurring haiku trace reviews instead. Passivating while waiting on
evidence remains the right protocol fix.

### F6. The apply checkpoint chases a moving head (reported; matches the open apply-starvation doc)

Every refused tail probe in reruns 7 and 8 wants head minus one, while
the head advances (1368→1683, 1478→1862). This supports
`selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md`. The
reviewer also found that rerun 9 did not run resumed work. It poisoned
at cap expiry instead, so starvation by resumed work explains reruns 7
and 8, not 9. To be verified before the next apply fix.

### F7. Loops on an identical error (reported)

On 2026-10-09, two Texture runs looped on `binding authority mismatch`:
465 iterations on `vm-1034d6a9` and 200 on `vm-9bfd4048`. Rerun 4 did
the same for 115 iterations. Nothing short-circuits an identical
reducer rejection.

### F8. Recovery reboots are logged as deploy refreshes (verified; not a rule violation)

After the opt-in rule (f6001aea, 2026-10-09 16:09Z), CI refreshed no
computer: `REFRESH_ACTIVE_COMPUTERS` is false outside a manual
dispatch, and the last one was at 14:12Z. Seven QA computers were still
"refreshed" between 14:57Z on the 9th and 00:56Z on the 10th. Every
one went through the proxy's compute recovery
(`proxy/compute_status.go`), which force-reboots a computer that is
unreachable, booting, degraded or failed.

vmctl logs every refresh as `deploy-image-refresh`
(`vmctl/ownership.go`), which mislabels these. To the guest, a
recovery reboot is a crash restart, so closing its open work follows
the owner rule. Residual `refresh-log-reason`: log the caller's reason.

### F9. Gateway 400s tripped the circuit breaker (verified; already fixed)

The gateway journal logged 24 `opencode-go` 400s (15 on rerun 5's VM at
23:41–23:44Z, 9 on rerun 6's at 00:23–00:25Z) and 13 circuit-open
refusals, one of which spread to a VM with no 400s. 589a68bc
(2026-10-10 00:39Z) already made the breaker ignore client 4xx except
408 and 429 (`gateway/circuit_breaker.go`). Both windows predate it.
Repaired; it needs no new work.

### F10. Receipts lack identity and reasons (reported)

No receipt records the deployed commit, model cost, or a specific
failure reason beyond the operation's error field. Probe blocker text is
generic in 7 of 12 M11 receipts. There are 9 distinct blockers in about
10.5 hours, but clustering assessments exist only for Texture. The
capsule, bundle and replay clusters each have 3 or more fixes.

### F11. A dead report wake makes Texture's disposition fail (reported)

The console logs `actor wake outbox disposed dead wake
assignment-report:…` at 07:20:38 and 07:36:01. Those are the moments
each Texture run was created. In both runs Texture's
`update_dispositions` naming that report were refused, as "does not
name a pending target-bound producer report eligible to this
activation". Code check (`agentcore/runtime.go` sweep): "disposed dead wake" means
the sweep's re-drive found the backing occurrence already gone. The
direct delivery that created the Texture run had consumed it, so the
disposal itself is benign. That leaves the refusal unexplained.
Texture tried two id forms, `assignment-report:report:sha256:…` and
`work:assignment…`, and both were refused. Next hypothesis: the
activation binds the report under an id the desk never sees. Open,
with the trace refs above.

### F12. Message validation fails a whole cell (reported)

The verifier's `choir.Message` failed twice: an unknown field
`artifact_refs`, then the engineering-to-engineering refusal. The
reducer rejects the whole tray, so the cell's other intents are lost
with it. The implementation desk froze with no patch preview and no
apply check, though git was available in its capsule. The repl
friction it hit: `choir.Exec` takes a slice, not variadic arguments,
and inner exit codes are not surfaced.

### F13. A desk does not know its own interaction surface (verified, rerun 11)

From 08:52 to 08:54Z in rerun 11, Texture run `4bc98ed4` made about 35
turns. None reached `choir.ApplyTexture`:

- bare `ReadDoc()` failed 5 times ("constant definition loop") until
  `choir.ReadDoc()`;
- then about 19 of 20 turns guessed API names: `PendingUpdates`,
  `Ledger`, `Messages`, `ReadUpdate`, `Poll`, and `Report` with six
  argument shapes.

Cause (code reading):

- Texture's surface is described only in prose fragments across
  `textureprompts/overlays/*.yaml`. There is no complete function list,
  no signatures, and no return-type fields (`DocSnapshot`,
  `PendingUpdate`, …).
- The prompt itself says `ReadDoc()` and `Inbox()` without the `choir.`
  prefix (`run_system.yaml:14`).
- There is no in-REPL help.
- An undefined `choir.X` or a wrong argument count never names what
  exists.

The exact per-desk surface already exists at runtime
(`yaegikernel.ChoirExports` over `deskModuleSets`) but is never shown.
This is F1's disease one layer down: the surface says "wrong" and never
"what is right".

## Proposed changes, by layer

- **reducer/protocol**
  - Every enum rejection names its accepted values (F1).
  - Short-circuit an identical rejection after N repeats, with the
    rejection fed back as a hard stop (F7).
  - Passivate Texture while it waits on evidence (F5).
- **repl**
  - Accept top-level statements without a declaration wrapper error.
  - Surface the activation allowlist in the rejection.
  - Diagnose the session-worker EOF (F2).
- **tools**
  - A builder-faithful `choir.CheckBundle` (git apply plus byte compare)
    for verifiers, so verification does not depend on the desk thinking
    of it (F3).
- **platform**
  - The freeze round-trip (done, F3).
  - Breaker classification of 4xx (F9).
  - Planned-refresh marker audit (F8).
- **probe**
  - Record the deployed commit and failure reason in receipts.
  - Stop the QA computer at exit.
  - Run one acceptance at a time (F10).
- **prompts**: none first. The evidence points at the surface the
  prompts describe, not at the prompts themselves.
