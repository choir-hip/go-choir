# Choir Status Report — September 4, 2026 (rev 2)

## What this is about

Choir is building a computer that can improve its own software under
supervision. The piece under work is a new way for the AI agents that
do that work to use their isolated workspace (called a capsule).
Instead of calling a long list of individual tools one at a time over
a text protocol, the agent writes small programs in the Go programming
language that run inside a persistent session — like working in a
notebook where variables and imports carry over from one step to the
next. We call this the session interpreter, or RLM. Everything the
agent does inside the capsule happens through one doorway
(`capsule_go_eval`), and inside that doorway the agent programs
against a small built-in library called `choir`.

## What is finished and live

As of this writing the code sits at commit `8c410a0d` on the staging
site, running on the retained test computer at boot epoch 879:

- **The route switch.** Old mode (individual JSON tools) or new
  session mode, chosen by one setting. Old mode is the default. If
  the session worker fails to start, calls fall back to the old path
  automatically, and the fallback is recorded on the receipt.
- **The persistent session.** Each assignment gets its own long-lived
  interpreter process with a startup handshake proving it is truly
  alive (we learned "process started" does not mean "process works").
  Crashed sessions are replaced, never trusted after failure.
- **The `choir` library.** Eight functions: reading, writing,
  listing, executing, plus assignment, messaging, context, and
  outcome reporting. (Three of these are now slated for
  deletion/removal — see below.)
- **Containment.** A runaway program is killed as a whole process
  group and reaped within half a second, proven by an automated test
  on every code change.
- **Role safety.** A read-only researcher gets read-only powers
  only, enforced at two independent layers, proven by a test that
  boots the real program.
- **The sealed tool set.** Under session mode the visible tool list
  shrinks from ten tools to six: the session doorway plus reporting,
  verification, and bundle tools. An automated test asserts the exact
  list in both modes.

## What outside review found (and what we fixed)

Three review rounds, fifteen panel verdicts total, every confirmed
defect fixed, tested, and deployed:

- **The worker could never start.** It was given an empty identity
  that its own setup rejects — every session died at boot (exit
  code 2) while the system reported ready. Tests missed it because
  they never booted a real worker. Fixed with the handshake plus an
  end-to-end boot test.
- **A privilege hole.** A read-only researcher using the session
  would have received full write and execute powers. Fixed with
  role-scoped permissions *before* the startup fix went live — in
  that order deliberately, so fixing the boot bug could not arm the
  hole with it.
- **Dead controls.** Session start/stop commands were rejected for
  every role; the fallback was a hardwired flag. Both are now real,
  tested behavior.
- **No on-switch.** There is no supported way to flip the new mode
  on for the live computer: the setting must travel from the
  owner's refresh command into the guest boot configuration, and
  that channel does not exist yet. Fully mapped (machine setting +
  `choir.actuator` boot parameter + guest startup mapping) and
  first in the build order.
- **The prompt lies.** System instructions still describe the old
  tools as the whole world. A mode-aware prompt fix is specified.

## The design question — resolved direction, awaiting approval

The `choir` library grew three functions that were never properly
designed. To be precise: **Message was designed** (a typed
inter-agent message with recipient, kind, body, and receipt).
**Assign and Outcome were not** — conveniences with no host
counterpart, and Outcome is literally implemented as a Message
addressed to oneself. Worse, all three store payloads in the
worker program's short-term memory while reporting "dispatched"
and "delivered" — with no mailbox, no reader, and nothing on the
host ever reading them.

Two further review rounds (seven reviewers, then eight, several
consulting published RLM/agent-harness designs) converged on a
target architecture, now written up as
`docs/designs/rlm-target-architecture-2026-09-04.md` (rev 3). The
shape, in brief:

```
CoSuper model -- JSON: capsule_go_eval ONLY --> host
host -- framed cell --> capsule broker (THE broker, in-guest)
broker -- pipes --> session worker (yaegi: file/exec syscalls + intent tray)
worker -- file/exec values (guest-internal) --> broker --> capsule disk
worker -- intent tray WITH cell result --> broker --> host
host reducer: persist mailbox + receipts, run fate, wake recipient
```

File and process operations stay inside the session as real system
calls. Everything needing another actor or durable fate becomes an
*intent*: the cell appends it to an outbound tray, the host
persists and acts on it after the cell ends, then wakes the
recipient — always. Nothing claims success the host has not
performed. The model-visible surface becomes one JSON tool plus Go
spellings (`Message(to, body)`, typed `Complete(...)`); durable
machinery (`record`, `commit`, `update`) is called by the host
reducer, never by the model. Assign is deleted; Outcome-as-message
is removed.

## The two command runners, explained

The trickiest detail, because two programs grew up that both "run
commands" with different meanings:

```
INNER (worker-local)              OUTER (capsule broker)
  program + argument list            one command STRING via sh -c
  NO shell: no pipes/globs/$VAR     FULL shell language
  env: 3 vars + filtered extras      env: broker's WHOLE environment
  secrets stripped                   secrets visible
  own process group, group kill      broker's group, no group kill
```

Why `sh` and not bash: the minimal guest image only guarantees
`sh`; bash may not exist (bash is a fallback, run with startup
files disabled). The resolution all reviewers accept: the inner
semantics (direct execution, clean environment) are implemented
once in the capsule broker; the shell form stays frozen for the
old-mode rollback only, with its retirement tracked — never
reachable from the new session path.

## Exact decisions on the table

- **Complete is typed:** `(result, verdict, summary,
  evidence_refs)` with result in {completed, failed, blocked};
  at most one per assignment, last in the tray; failed cells
  deliver messages but never a Complete. The host binds real
  execution receipts itself — the cell never supplies them.
- **Messages get a fixed host envelope** (sender from the
  activation, one fixed kind, sequence, time) addressed to the
  bound parent supervisor only; no cell-chosen kinds, ever.
- **Wake is dedicated and coalesced:** a new wake path for mailbox
  commits (the old one deliberately skips this direction), one
  wake per batch, with exemptions for replays and cancelled work.
- **Caps:** 16 intents per cell, 16 KiB per message, aggregate
  tray bound, per-activation quota (numbers tunable from
  evidence; their existence is the contract).
- **Exact tool lists per assignment slot,** tracked deletions for
  every removed JSON tool, and a strict build order: boot channel
  first, then pipes, then the canonical runner, then the reducer,
  then deletions. Rollback at every step is "switch the flag off
  and refresh."

## Suggested next steps, in order

1. **You review and approve (or amend) the target architecture.**
   The full rev-3 document is in the repo, with the panel's
   dissent and alternatives recorded. Nothing below starts
   without this.
2. **Build in the specified order** (boot channel → pipes →
   runner → reducer → deletions), each step with tests and its
   own rollback.
3. **Run the live demonstration** on the test computer: switch
   to the new mode, supervised agent does a small
   read-compute-write task entirely through session programs,
   verify every receipt, switch back. Gated at each step.
4. **Follow-on work already identified:** in-cell verification
   design, supervisor-agent session treatment, retiring the
   frozen shell runner. None starts until the demonstration
   reports.

## Risks being carried

- The test computer is healthy and idle; every live step has a
  pre-spelled rollback (switch off, refresh).
- The new mode, once on, applies to every workspace on that
  computer until switched off — keep the window short and quiet.
- Panels advise; they do not prove. Every consequential claim
  traces to a test, a build, or a line of code — except future
  work, marked as such throughout.
