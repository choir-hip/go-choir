# Choir Status Report — September 4, 2026

## What this is about

Choir is building a computer that can improve its own software under
supervision. The piece we have been working on is a new way for the AI
agents that do that work to use their isolated workspace (called a
capsule). Instead of calling a long list of individual tools one at a
time over a text protocol, the agent writes small programs in the Go
programming language that run inside a persistent session — like working
in a notebook where variables and imports carry over from one step to
the next. We call this the session interpreter, or RLM. Everything the
agent does inside the capsule — reading files, writing files, running
commands — happens through one doorway (`capsule_go_eval`), and inside
that doorway the agent programs against a small built-in library called
`choir`.

## What is finished and live

As of this writing the code sits at commit `8c410a0d` on the staging
site, running on the retained test computer at boot epoch 879. All of
the core construction work is done, tested, and live there:

- **The route switch.** The capsule system can run in either the old
  mode (individual JSON tools) or the new session mode, chosen by a
  single setting. The old mode remains the default, so nothing changes
  unless the new mode is deliberately switched on. If the new session
  worker ever fails to start, calls fall back to the old path
  automatically, and the fallback is recorded.
- **The persistent session.** Each assignment gets its own long-lived
  interpreter process. State survives across steps. A crashed or
  poisoned session is thrown away and replaced — never trusted after a
  failure. A startup handshake proves the worker is truly alive before
  it serves any work (we learned the hard way that "process started"
  does not mean "process works").
- **The `choir` library.** Eight functions the agent's programs can
  call: reading, writing, listing, executing, plus assignment,
  messaging, context, and outcome reporting.
- **Containment.** A runaway program is killed as a whole process group
  and reaped within half a second. Proven by an automated test that
  runs on every code change.
- **Role safety.** A read-only researcher agent that uses the session
  gets read-only powers only — it cannot write or execute, enforced at
  two independent layers. Proven by a test that boots the real program.
- **The sealed tool set.** When session mode is on, the agent's visible
  tool list shrinks from ten tools to six: the session doorway
  (`capsule_go_eval`) plus reporting, verification, and bundle tools.
  The old command tool (`capsule_exec`) and the three file tools
  disappear from view, so the agent cannot bypass the session. An
  automated test asserts the exact list in both modes, so any drift
  fails the build.

## What outside review found (and what we fixed)

We ran two rounds of multi-model review panels — seven independent AI
systems reading the actual code — and both rounds found real defects.
Every confirmed defect has been fixed, tested, and deployed:

- **Round one: the worker could never start.** The code gave the worker
  an empty computer identity, which the worker's own setup rejects, so
  every session died immediately at boot (exit code 2) while the system
  reported itself ready. Our tests missed it because they never booted
  a real worker — only a nonexistent binary and a dummy `sleep`
  process. Fixed with the startup handshake plus a test that compiles
  and boots the genuine program and runs cells through it.
- **Round one: a privilege hole.** A read-only researcher agent using
  the session would have received full write and execute powers,
  bypassing the role rules that govern everything else. Fixed with
  role-scoped permissions — researcher sessions get a read-only
  library, enforced both in which functions exist and in each function
  itself — before the startup fix ever went live. That ordering was
  deliberate: fixing the boot bug first would have switched on the
  privilege hole with it.
- **Round one: dead controls.** The session start/stop/status commands
  existed but were rejected for every user role, so no caller could
  ever reach them; and the old-mode fallback was a flag hardwired to
  "ready," so it could never actually engage. The commands are now
  admitted for the right roles, and fallback is real per-call
  behavior: if no session can start, the call runs the old way and
  says so on the receipt.
- **Round two (planning the live demonstration): no on-switch.** There
  is currently no supported way to flip the new mode on for the live
  test computer. The setting has to travel from the owner's refresh
  command into the guest machine's boot configuration, and that
  channel does not exist yet — no machine field for it, no boot
  parameter for it, no flag on the refresh command. Building it is
  the single biggest remaining engineering task, and its shape is
  fully mapped: a new machine setting, a `choir.actuator` boot
  parameter, a line in the guest startup script that turns it into
  the worker's environment, and a mode-aware agent prompt (the
  current prompt still describes the old tools as the whole world,
  which would confuse the agent once those tools are hidden).
## The open design question

The liveliest debate — including a third review round with eight
systems, several of which researched published RLM and agent-harness
designs on the web — concerns three small functions in the `choir`
library: Assign, Message, and Outcome. To be precise about what was
and was not designed: Message was designed (a typed inter-agent
message carrying recipient, kind, and body, returning a receipt). The
other two were not — Assign and Outcome grew as conveniences with no
specified host counterpart, and Outcome is literally implemented as a
Message addressed to oneself.

The problem is not the design of Message but what was built under it.
All three functions record their payloads in the worker program's
short-term memory and report success words like "dispatched" and
"delivered" — while nothing on the host side ever reads them. A
message can therefore be "delivered" with no recipient, no mailbox,
and no reader anywhere in the system. The receipt is the fiction, not
the function signature. The same holds for Assign (an assignment that
assigns nothing durable) and Outcome (a completion report nobody
collects).

All eight reviewers converged on the same principles, drawn from
real-world systems: a computation step should not *send* its result
like a letter; the host should *harvest* it like a return value. File
and process operations belong inside the session; promises about the
outside world (waking a supervisor, closing out an assignment,
freezing a bundle) belong outside it. And no function should claim a
success the host has not actually performed.

The concrete proposal on the table: delete Assign and Outcome, keep
Message only if it is re-grounded on a real host mailbox with a named
reader (otherwise remove it too), keep the file and process
operations, leave durable promises in the existing host tools, and let
the host infer results from what the session actually did. This
simplification is drawn up and reviewed but **not yet approved and
not yet built** — that approval is the decision this report is
waiting on.

## Suggested next steps, in order

1. **Approve (or amend) the simplification design.** This is the one
   decision that unblocks everything else. The options are laid out in
   the review synthesis already delivered: delete now, keep a minimal
   journal mechanism for later, and keep durable promises in host tools.
2. **Update the design documents, then change the code.** In that
   order, per project discipline: first the mission definition and the
   problem receipts, then the implementation.
3. **Build the on-switch.** Add the machine setting that carries the
   new mode from the owner's refresh command down into the guest
   machine's boot configuration, plus the mode-aware agent prompt.
   This is standard classified engineering work with a known shape.
4. **Run the live demonstration.** Switch the test computer to the new
   mode, have a supervised agent perform a small read-compute-write
   task entirely through session programs, verify every receipt, then
   switch back. Each step has a named go/no-go gate.
5. **Decide what comes next.** The follow-on work already identified: a
   proper durable channel for anything the session wants the host to
   promise, and possibly the same session treatment for the supervisor
   agent. Neither should start until the demonstration has reported.

## Risks being carried

- The test computer is healthy and idle, but every step of the live
  demonstration touches a real machine — each has a rollback (switch
  off, refresh) spelled out in advance.
- The new mode, once switched on, applies to every workspace on that
  computer until switched off. The demonstration window should be kept
  short and quiet.
- Review panels are advisors, not proof. Every consequential claim in
  this report traces to a test, a deployed build, or a line of code —
  except the future work, which is clearly marked as such.
