# X4 step 0 probes, run on d@e through the product (2026-11-10)

Mutation class: green (evidence only; no product code). Companion to
[coding-agent-desk-step0-probes-2026-11-10.md](coding-agent-desk-step0-probes-2026-11-10.md),
which wrote the probes and listed them as "owed on d@e". Station:
[coding-agent-desk-station-2026-11-10.md](../design/coding-agent-desk-station-2026-11-10.md).
Run on test computer d@e only. No secret, token or credential was read, printed
or set by this session.

## Summary in plain language

The three probes ran on d@e, on a product path, for $0.07 of delegate spend.
The answers:

1. **Does `--setting-sources user` load the repo CLAUDE.md? No.** The first
   turn's prompt was 8,618 tokens (the script's "without CLAUDE.md" band is
   15-18k, "with" is 23k or more), and the init event lists only the user's
   auto-memory path. [M]
2. **Does the deployed `claude` connect a stdio MCP server under `-p`? Yes.**
   `mcp_servers` was `[{choirdesk, connected}]`, `mcp_server_errors` absent, and
   the echo tool was called. [M] But the binary under test was **2.1.300, not
   the pinned 2.1.197** the design and the script's verdict text name. So the
   question "does the pin need to move to v2.1.221 or later" is already
   answered by the deployed build: it is past that, and it connects. The pin text
   in the design is stale. [M]
3. **Do two queued stdin lines become two turns? Likely yes, not proved.** The
   CLI accepted `--input-format stream-json` with two lines and produced 3
   assistant messages (tool call, "done", second reply), but the final `result`
   line says `num_turns: 2`. The script's own verdict rule ("two turns if the
   second reply follows the first") needs the order of events, which sits in the
   run's `events.jsonl` inside a capsule that no longer exists. [M for the
   counts, not settled for the order]

Two defects in the probe script were also found: its usage line implies a
trailing positional `.` that argparse rejects, and its verdict text hard-codes
"2.1.197".

## Method: how the probes are meant to run, and how they did

The station doc and the script's docstring say: `--run` belongs only inside a
d@e engineering capsule, "started through the broker so the relay route and its
secret are in the environment the way the delegate has them". The script
"reads no credential, prints none, and never sets one."

Product path found from the code (`internal/claudedelegate/spec.go` `Env`,
`cmd/capsule-broker/delegate.go`): the delegate's `claude` process is started
by the capsule broker with `ANTHROPIC_BASE_URL` (the gateway relay) and
`ANTHROPIC_AUTH_TOKEN` (a per-capsule secret) in its environment; its `Bash`
tool is on, so a Bash child inherits that environment and can start a second
`claude -p` that is billed through the same relay. A plain capsule shell has
no such secret. That is the only way the probes can make a model call without a
credential in the script.

Preconditions, checked rather than assumed:

| Precondition | Result on d@e |
|---|---|
| Deployed tree contains the script and `cmd/choir-desk-mcp` | yes: d@e deployed `b798347b`, which contains `f4015bf6a` (the probe commit) and the `deskmcp` commits; the capsule's `/workspace/platform` listed both |
| python3 in the capsule | yes (floor package list has `python3`; the script ran) |
| Go toolchain to build the probe server | yes: `go build -o /tmp/choir-desk-mcp ./cmd/choir-desk-mcp` exit 0, 4,499,341 bytes (the binary is not in the guest image or the flake; it had to be built) |
| `claude` binary path | not on PATH; found with `readlink /proc/$PPID/exe` from the delegate's Bash: a `claude-code-2.1.300` nix store path |
| Delegate grant | present before I did anything: `GET /api/runtime/observability` showed `claude_delegate: {available: true, model: claude-sonnet-5-5, expires_at: 2026-11-17, daily_token_cap: 50000000}`; `choir-delegate status` in the capsule agreed. I granted nothing. |

I submitted one prompt-bar run on d@e (`POST /api/prompt-bar`, command id
`x4-step0-probes-2026-11-10`) asking the engineering desk to start a delegate
with a measurement-only brief naming the five steps. Texture acknowledged it
and cast to management; management cast to engineering with the constraints
("edit no repository file, freeze nothing, apply nothing", delegate 20 min and
3 USD); engineering started the delegate (`choir-delegate start`, wall clock 20,
budget 3) and parked on it. Timeline (UTC): submitted 11:26:25; engineering
started 11:31:06; delegate ran 11:31:59 to 11:32:45 (45 s); engineering
completed 11:34:59.

## Measured results

Delegate run `d20261110T113159-1b3b83a9`: 6 turns, 5 Bash calls, no Edit or
Write, $0.0699816 total (the two probe calls inside it cost $0.0095735 and
$0.0024990). The engineer confirmed no file in the tree changed
(`find /workspace/platform -newermt` returned 0 files).

| Step | Command (abbrev.) | Result |
|---|---|---|
| 1 | `go build` the probe server | exit 0, empty output |
| 2 | `readlink /proc/$PPID/exe` | `/nix/store/...-claude-code-2.1.300/bin/.claude-wrapped` |
| 3 | `x4_step0.py mcp --run` | exit 0; init `claude_code_version` 2.1.300; `mcp_servers` `[{name: choirdesk, source: dynamic, status: connected}]`; no `mcp_server_errors` key; tools `[Read, mcp__choirdesk__echo]`; tool use `mcp__choirdesk__echo`; 2 turns; first usage: 2 input, 3,330 cache-creation, 24 output |
| 4 | `x4_step0.py queue --run` | exit 0; `mcp_servers` connected; 3 assistant turns; `result.num_turns` 2; echo called; first usage: 2 input, 3,330 cache-read |
| 5 | `x4_step0.py events` on a stored run | **No earlier run existed** (`/tmp/choir-delegate` held only the current run and `home/`; the capsule is fresh). The delegate declined to point it at its own run. The engineering desk then ran probe (i) on that run's `events.jsonl` itself (a legitimate use: the file is a delegate run): turn-1 prompt tokens **8,618**, verdict "the prompt is the size without the repo CLAUDE.md"; init `mcp_servers` empty (the normal delegate config); `/workspace/platform/CLAUDE.md` is 32,690 bytes and is not in the prompt |

Run `events_sha256` for the evidence: `sha256:602a1b018e734102d1508c2b7546ea5717b323e81cde4996fab7f914681455bb`
(42,993 bytes; read through the capsule's `choir-delegate wait` status).

Note on probe (i): 8,618 is below the design's "15-18k without" estimate. That
estimate came from a different, older run; this run used the 2.1.300 build with
the six-tool set. The verdict (CLAUDE.md not loaded) does not depend on the
estimate, because the file is 32 KB (about 8k tokens) and the whole prompt is
8.6k including the system prompt, tools and brief.

## Claims to carry forward, with their grade

| Claim | Grade |
|---|---|
| The deployed delegate binary is claude-code 2.1.300 | [M] (path and init `claude_code_version`) |
| A stdio MCP server connects under `-p` on 2.1.300 and its tool is called | [M] one run |
| `--setting-sources user` does not load the repo CLAUDE.md in the delegate | [M] one run |
| Step 2 of the station does not need a pin bump | [I] from the first two claims; the pin string in the repo (2.1.197) disagrees with the deployed binary and should be reconciled |
| Queued stdin lines run as separate turns | [H], supported by 3 assistant messages, not settled (order unread) |
| The delegate's Bash children inherit the relay environment | [M] (the nested `claude` authenticated). An engineer's own `choir.Exec` shell does not: its replay failed with "Not logged in", 0 tokens, $0 |

## Defects found in the probe tooling (not fixed here)

1. `scripts/probes/x4_step0.py` verdict string for `mcp` is the fixed text
   "2.1.197 connects a stdio server under -p". It should print the measured
   `claude_code_version`. The design doc section 7 step 0 also cites 2.1.197.
2. My own brief ended its commands with a stray trailing `.`; argparse rejects it (exit 2) and the delegate dropped it silently. The script and the earlier evidence doc are correct; this is a brief error, recorded so it is not mistaken for a script defect.
3. Probe (iii) prints counts only, not the order of assistant and result
   events, so its own verdict rule cannot be applied from its output.

## Limits

- One run each; no repeat. The model, the relay and the wall clock all vary.
- The `events.jsonl` that holds probe (iii)'s order was inside a capsule that
  has ended; I could not re-read it. To settle (iii): rerun with the script
  printing event order, or have the delegate copy `events.jsonl` to the
  guest store (`files put`) before the capsule ends.
- Probes ran against whatever `claude` d@e shipped (2.1.300). b@c or any
  computer on another image could differ.
- I did not run the probes with the pinned 2.1.197, so "2.1.197 connects" is
  untested; only "the deployed build connects" is known.

## Cost and artifacts left on staging (d@e only)

- Delegate spend: $0.07. DeepSeek (engineering desk, synthetic provider) 25
  calls, about 732k input tokens, 18k output; plus the Texture and management
  calls of the same trajectory.
- Trajectory `95ec7965-6162-59da-8e20-26d0f1f14de7`, Texture document
  `c0d8de68-e7c6-5288-a696-91bb0a52ddc5` (two revisions: the request and
  Texture's acknowledgement), engineering assignment
  `run:assignment-f2a564fbda9db34a9092326b` (completed, "no change to apply").
  At 11:37 UTC the trajectory was still `live` with its single work item
  `open`; an owner may close it.
- Two precommit claims recorded by the engineer on d@e (p 0.85 "finish
  completed", p 0.80 "go build succeeds").
- `/tmp/choir-desk-mcp` (4.5 MB) in the ended capsule only; no repo file changed.
- Slowest hop: Texture took about 2.7 minutes to dispatch to management; the
  whole round from prompt to engineering complete was 8.5 minutes.

## What it suggests

- Probe (ii) closes the question that gated the pin: step 2 of the station
  can proceed on the deployed 2.1.300. Reconcile the pin string.
- The launch route "prompt bar, engineering, delegate Bash child" works as a
  product path for any paid probe and cost cents; the station's step 7 (d@e
  proof) can use the same route.
- Probe (iii) should be rerun with the order printed before the stdin wake
  (design Q8) is decided.
