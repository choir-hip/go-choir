# The coding-agent desk over MCP: failure list (2026-11-10)

Lane X4 of [the overnight mission](../definitions/choir-autopaper-overnight-2026-11-09.md).
Written from [the design](../design/coding-agent-desk-over-mcp-2026-11-09.md)
and its section 11 owner rulings, before any code. Mutation class of this
document: green. Each item names the check that will refuse it; a slice
lands with its items' tests first. Ids are cited by the tests.

## Tool surface and protocol (F)

- F1. A tool call names its own identity (assignment, desk, target, conversation).
  Refused: no tool schema has such a field, an unknown argument is an error, and the
  host takes identity from the socket the broker bound at spawn.
- F2. A tool name outside the catalogue, or a verb the performer's desk row does not hold,
  is called. Refused at the host; the refusal is a tool result, not a protocol error.
- F3. The catalogue drifts from the desk row: a tool maps to Cast, Escalate, Verify,
  ShotStart or any verb the performer lacks. A test derives each tool's verb and
  asks the registry (never a desk name).
- F4. Unbounded free text or arrays (claim, summary, evidence refs, argv). Every
  string and list has a bound; over it is refused with the bound named.
- F5. Parallel tool calls from a coding agent interleave two reductions for one
  assignment. Calls serialize per assignment; two assignments never block each other.
- F6. A malformed or hostile JSON-RPC line (bad JSON, unknown method, a request
  before initialize, a batch, a notification carrying an id, a line over the size
  bound) kills or wedges the server, or writes non-JSON to stdout. The server
  answers each with a JSON-RPC error and keeps serving; stdout is JSON-RPC only.
- F7. `desk_wait` blocks past the cap, or blocks 30 minutes with no progress
  notification (the agent's idle abort). Cap 1500 s; a progress notification
  every 60 s while blocked; the cap and the interval are fixed constants.
- F8. A tool result carries records as instructions. The records suffix is
  labelled as data, bounded, and says "N new records".
- F9. The in-capsule server holds or echoes a credential. It reads no
  credential variable, has no model, and its only output is what the host returned.
- F10. The host socket is absent or silent: the call hangs. Bounded dial and
  call timeouts; the failure is a tool result naming that the host is unreachable.
- F11. `desk_report` is final, or targets other than the admitter, or carries
  no acting-worker provenance, or the claim is not labelled untrusted at the reader.
- F12. A queue of pending calls survives a crash and is replayed. Nothing is
  persisted; a call in flight at a crash is lost and never retried.
- F13. `tools/list` changes during a run (`list_changed`). The set is constant
  and the server advertises no `listChanged`.
- F14. A refusal is an `isError` tool result with the host's reason, so the
  agent reads it as a cell reads `verbRefusal`.

## Interim-update routing (R)

- R1. Jev is unavailable, times out, or answers outside the two outcomes: the
  update passes through to Texture and the record says "no judgment ran". It is
  never dropped and never blocks the reducer.
- R2. The Jev call runs inside the reducer transaction. It must not: the
  judgment runs after the Report commits, and its outcome is a second commit.
- R3. A replayed Report is judged twice, or two outcomes exist for one Report.
  The decision id derives from the Report id; one outcome per Report.
- R4. A setting or allowlist selects the behaviour. None; the removal of
  `engineering.progress_to_texture` is part of wiring this seam.
- R5. The outcome is not data on the tape: which updates went straight through,
  which woke management, whether a judgment ran, who judged and what management
  changed must be a record (owner ruling 8).
- R6. Delegate-authored claim text enters Jev's state outside the data wrapper,
  so it reads as an instruction. It is wrapped and labelled untrusted.
- R7. A Jev answer that is both outcomes, neither, or a distribution that is
  not decisive is read as "no judgment ran", not as a coin toss.

## Completion (C)

- C1. The agent supplies the commands that count as acceptance. Checks come from
  the assignment only; the agent has no argument that carries a command to be
  treated as acceptance (its own `desk_run_checked` runs are advisory).
- C2. A completed result passes with no check run by the runtime. Refused.
- C3. A completed result passes with a failing check. Refused, whatever Jev says.
- C4. Jev says "suffice" and the checks were not all green. Needs both; refused.
- C5. Jev is unavailable, times out or answers unclearly: fail closed, escalate
  to management (opposite of R1: a pass is not a default).
- C6. The change read is truncated (over the 50,000-token read) and Jev says
  "suffice". A truncated read cannot carry a "suffice": escalate.
- C7. The assignment has no checks. A completed result cannot pass vacuously: it
  escalates; failed, blocked and partial results do not claim a pass and need no gate.
- C8. A job is still running at `desk_complete`. Refused (the reducer's own rule
  at `rlm_reduce.go:941-958` stays the authority; the admission checks run first
  and add no way around it).
- C9. The admission verdict is not on the tape: the result, the receipts, Jev's
  answer and the read bound are cited in the Complete's evidence refs.

## Admission and money (M), owed in later slices

- M1. No grant: a coding-agent performer is chosen anyway. The Go-driven engineer
  performs, and the binding records which performer ran.
- M2. A grant lapses mid-round: the round closes `blocked` with the relay's
  reason; nothing retries.
- M3. Probes about the pinned Claude Code (2.1.197) are answered from belief:
  they are run, not assumed (evidence doc).
