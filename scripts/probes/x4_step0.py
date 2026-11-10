#!/usr/bin/env python3
"""Step 0 probes of the coding-agent desk over MCP (lane X4).

docs/design/coding-agent-desk-over-mcp-2026-11-09.md section 7, step 0:

  events FILE   probe (i): read one stored events.jsonl of a delegate run and
                settle whether `--setting-sources user` loaded the repo
                CLAUDE.md, from turn 1's prompt-token total (about 15-18k
                without it, about 23-26k with it) and the system/init event.
                Reads a file only; free; runs anywhere.
  mcp           probe (ii): run the pinned `claude` (2.1.197) once with the
                one-tool probe server (`choir-desk-mcp probe-echo`) and read
                `mcp_servers` and `mcp_server_errors` in system/init, and
                whether the echo tool was called.
  queue         probe (iii): run `claude -p --input-format stream-json` with
                two user lines queued at once and count the turns.

`mcp` and `queue` make a paid model call. They print their plan and do
nothing else unless --run is given, and --run belongs only inside a d@e
engineering capsule, started through the broker so the relay route and its
secret are in the environment the way the delegate has them. This script
reads no credential, prints none, and never sets one.

Usage:
  python3 -I scripts/probes/x4_step0.py events /tmp/choir-delegate/<id>/events.jsonl
  python3 -I scripts/probes/x4_step0.py mcp   --claude claude --server /usr/local/bin/choir-desk-mcp [--run]
  python3 -I scripts/probes/x4_step0.py queue --claude claude --server /usr/local/bin/choir-desk-mcp [--run]
"""
import argparse, json, subprocess, sys

# The flags the delegate runs under today (internal/claudedelegate/spec.go
# Args), with the MCP config swapped for the probe server.
MODEL = "claude-sonnet-5-5"
PROMPT = ("Call the mcp__choirdesk__echo tool once with text \"probe\", then reply with the word done "
          "and nothing else.")


def argv(claude, server, stream_input):
    cfg = json.dumps({"mcpServers": {"choirdesk": {"command": server, "args": ["probe-echo"]}}})
    a = [claude, "-p", "--model", MODEL, "--output-format", "stream-json", "--verbose",
         "--max-turns", "4", "--max-budget-usd", "1.00", "--tools", "Read",
         "--allowedTools", "mcp__choirdesk__echo", "--strict-mcp-config", "--mcp-config", cfg,
         "--setting-sources", "user", "--settings", '{"disableAllHooks":true}']
    if stream_input:
        a += ["--input-format", "stream-json"]
    return a + ["--permission-mode", "bypassPermissions"]


def read_events(lines):
    for line in lines:
        line = line.strip()
        if not line:
            continue
        try:
            yield json.loads(line)
        except ValueError:
            continue


def summarize(events):
    out = {"init": None, "turns": 0, "first_usage": None, "tool_uses": [], "result": None}
    for e in events:
        t, st = e.get("type"), e.get("subtype")
        if t == "system" and st == "init" and out["init"] is None:
            out["init"] = {k: e.get(k) for k in ("mcp_servers", "mcp_server_errors", "tools", "model", "claude_code_version", "memory_paths") if k in e}
            out["init"]["keys"] = sorted(e.keys())
        elif t == "assistant":
            out["turns"] += 1
            m = e.get("message") or {}
            if out["first_usage"] is None and m.get("usage"):
                u = m["usage"]
                out["first_usage"] = {k: u.get(k, 0) for k in ("input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens")}
            for c in m.get("content") or []:
                if isinstance(c, dict) and c.get("type") == "tool_use":
                    out["tool_uses"].append(c.get("name"))
        elif t == "result":
            out["result"] = {k: e.get(k) for k in ("subtype", "is_error", "num_turns", "total_cost_usd")}
    return out


def cmd_events(path):
    with open(path, encoding="utf-8") as f:
        s = summarize(read_events(f))
    print(json.dumps(s, indent=2, sort_keys=True))
    u = s["first_usage"]
    if not u:
        print("verdict: no assistant usage in the file; cannot settle")
        return 1
    total = u["input_tokens"] + u["cache_creation_input_tokens"] + u["cache_read_input_tokens"]
    if total < 19000:
        v = "the prompt is the size without the repo CLAUDE.md (about 15-18k): --setting-sources user does not load it"
    elif total > 22000:
        v = "the prompt is the size with the repo CLAUDE.md (about 23k or more): it is loaded"
    else:
        v = "between the two sizes: not settled; read memory_paths or the first Read of CLAUDE.md in the file"
    print("turn-1 prompt tokens: %d\nverdict: %s" % (total, v))
    return 0


def run_probe(args, stream_input):
    a = argv(args.claude, args.server, stream_input)
    print("plan:", json.dumps(a))
    if not args.run:
        print("dry run: nothing executed. Add --run inside a d@e engineering capsule.")
        return 0
    if stream_input:
        stdin = "\n".join(json.dumps({"type": "user", "message": {"role": "user", "content": p}}) for p in (PROMPT, "Now reply with the word again and nothing else.")) + "\n"
    else:
        stdin = PROMPT
    p = subprocess.run(a, input=stdin, capture_output=True, text=True, timeout=600)
    s = summarize(read_events(p.stdout.splitlines()))
    print(json.dumps(s, indent=2, sort_keys=True))
    init = s["init"] or {}
    if stream_input:
        print("verdict: %d assistant turns, result num_turns=%s; two queued lines are two turns if the second reply follows the first" % (s["turns"], (s["result"] or {}).get("num_turns")))
    else:
        errs = init.get("mcp_server_errors")
        called = any(n == "mcp__choirdesk__echo" for n in s["tool_uses"])
        print("verdict: mcp_servers=%s mcp_server_errors=%s echo_called=%s -> %s" % (
            init.get("mcp_servers"), errs, called, "2.1.197 connects a stdio server under -p" if called and not errs else "does not connect (bump the pin to v2.1.221 or later)"))
    return p.returncode


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    e = sub.add_parser("events")
    e.add_argument("file")
    for name in ("mcp", "queue"):
        s = sub.add_parser(name)
        s.add_argument("--claude", required=True)
        s.add_argument("--server", required=True)
        s.add_argument("--run", action="store_true")
    a = ap.parse_args()
    if a.cmd == "events":
        return cmd_events(a.file)
    return run_probe(a, a.cmd == "queue")


if __name__ == "__main__":
    sys.exit(main())
