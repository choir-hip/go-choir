# Coding-agent desk: step 0 probes (2026-11-10)

Lane X4. Step 0 of [the design](../design/coding-agent-desk-over-mcp-2026-11-09.md)
section 7. Mutation class: green. This session ran nothing that spends money
and touched no staging, no ssh, no Claude login or credential; every probe that
needs those is "owed on d@e" with its exact command. The probe tooling is
[`scripts/probes/x4_step0.py`](../../scripts/probes/x4_step0.py) (dry run by
default; `--run` only inside a d@e engineering capsule) and the probe server
`choir-desk-mcp probe-echo` ([`cmd/choir-desk-mcp`](../../cmd/choir-desk-mcp/main.go)).

| Probe | What it settles | Result here | Status |
|---|---|---|---|
| (i) does `--setting-sources user` load the repo CLAUDE.md | whether the SystemContext sentence guards a file the delegate never reads, and whether 8k tokens sit in its prefix | Could not run: no stored delegate `events.jsonl` exists in the tree or on this Mac (searched the repo for `events.jsonl`, `"subtype":"init"`, `memory_paths`, `mcp_servers`: only design text and `cmd/choir/login*` match). The parser was exercised on a three-line synthetic file (not evidence about Claude Code, only about the script) and reports the verdict rule correctly. | owed on d@e |
| (ii) does pinned `claude` 2.1.197 connect a stdio MCP server under `-p` | whether step 2 needs a pin bump to v2.1.221 | Server half: pass. `choir-desk-mcp probe-echo` answers `initialize`, `tools/list` and `tools/call` over stdio with JSON-RPC alone on stdout (`TestProbeEchoSpeaksMCP`, `internal/deskmcp/server_test.go`). Client half (that `claude` connects, lists `mcp_servers` as connected, calls the tool): could not run, it needs a model call. | server half proved; client half owed on d@e |
| (iii) do two queued stdin lines become two turns | whether the stdin wake (design Q8) is viable; the design's test E2 | Could not run: needs a model call. | owed on d@e |

## Owed on d@e

Run inside a d@e engineering capsule (the E2 procedure of
`docs/evidence/token-efficiency-2026-11-09/followup/claude-delegate-desk.md`
section 2.5: through the broker's launch hook, so the relay route and its secret
are in the environment as the delegate has them). Paid, but a single short
round each; allowed on d@e only.

```text
# build and place the probe server (any machine with go)
go build -o /usr/local/bin/choir-desk-mcp ./cmd/choir-desk-mcp

# (i) free; read the newest stored delegate run
python3 -I scripts/probes/x4_step0.py events "$(ls -t /tmp/choir-delegate/*/events.jsonl | head -1)"

# (ii) paid, one round
python3 -I scripts/probes/x4_step0.py mcp   --claude claude --server /usr/local/bin/choir-desk-mcp --run

# (iii) paid, one round
python3 -I scripts/probes/x4_step0.py queue --claude claude --server /usr/local/bin/choir-desk-mcp --run
```

Reading the answers: (i) prints turn-1 prompt tokens and a verdict (about
15-18k means the repo CLAUDE.md is not loaded; about 23k or more means it is).
(ii) prints `mcp_servers`, `mcp_server_errors` and whether the echo tool was
called; `connected` with the call made and no errors means 2.1.197 needs no
pin bump. (iii) prints the assistant turn count and the result's `num_turns`.
Record each answer in this table.

## Unverified items the script cannot settle

- The exact stdin line shape under `--input-format stream-json` is the SDK's
  documented form; the script uses it, and a rejection by the CLI is itself
  the answer to (iii).
- Whether `codex exec` and `opencode run` load MCP servers non-interactively
  (design step 8), untouched here.
- The `init` event's field names (`mcp_servers`, `mcp_server_errors`) are the
  design's; the script prints every init key it finds, so a different spelling
  shows in the output.
