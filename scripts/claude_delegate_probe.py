#!/usr/bin/env python3
"""Staging proof for the Claude delegate, Slice 1
(docs/design/claude-delegate-slice1-2026-10-14.md).

Run it after the deploy, after the owner has put the token on Node B and the
test computer is on CHOIR_CLAUDE_DELEGATE_COMPUTERS and has cold-booted.

1. Gate: the token must exist. With --ssh, check the host file's mode, owner
   and size (never its content) with `sudo -n stat`. Always, read the guest's
   delegate status from GET /api/runtime/observability (claude_delegate);
   it is available only when vmctl wrote a valid token at the last cold
   boot. If either says no, stop with exit 2 and start nothing.
2. With --status-only, stop here (exit 0 when available).
3. End to end: start one prompt-bar run whose brief asks engineering to use
   the delegate for a trivial file, wait for the engineering run, and read
   its events: a `choir-delegate start`, a `wait` that reached state done
   with subtype success, and the engineering run completed.

The probe never holds the Claude token. The computer key is read from the
0600 credential file (as scripts/qa_test_computer.mjs writes it) and is
never printed. Output: one JSON receipt on stdout and in --out.

usage: claude_delegate_probe.py --cred FILE --out FILE [--ssh node-b]
                                [--status-only] [--timeout-min 45]
"""
import argparse
import json
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

HOST = "https://choir.news"
TOKEN_FILE = "/var/lib/go-choir/secrets/claude-oauth-token"
TERMINAL = {"completed", "failed", "cancelled", "blocked"}


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), file=sys.stderr, flush=True)


class Computer:
    def __init__(self, cred_file, host):
        cred = json.load(open(cred_file))
        self.key = cred.get("api_key") or cred.get("key")
        self.id = cred.get("computer_id")
        if not self.key or not self.id:
            raise SystemExit("credential needs api_key (or key) and computer_id")
        self.host = host.rstrip("/")

    def call(self, method, path, body=None, retries=20):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.id}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        for _ in range(retries):
            try:
                req = urllib.request.Request(self.host + path, data=data, headers=headers, method=method)
                return json.load(urllib.request.urlopen(req, timeout=300))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(10)
                    continue
                # Print the status only: a body can echo request context.
                return {"error": err.code, "path": path.split("?")[0]}
            except urllib.error.URLError:
                time.sleep(10)
        return {"error": "unavailable", "path": path.split("?")[0]}

    def events(self, run_id):
        out, after = [], 0
        q = urllib.parse.quote(run_id, safe="")
        while True:
            page = self.call("GET", f"/api/runs/{q}/events?limit=1000&after={after}")
            out += page.get("events") or []
            nxt = int(page.get("next_after") or 0)
            if nxt <= after:
                return out
            after = nxt


def host_token_check(ssh_host):
    """Mode, owner and size of the token file, never its bytes."""
    cmd = f"sudo -n stat -c '%a %U %s' {TOKEN_FILE}"
    try:
        out = subprocess.run(["ssh", ssh_host, cmd], capture_output=True, text=True, timeout=60)
    except subprocess.TimeoutExpired:
        return {"present": False, "reason": "ssh timed out"}
    fields = out.stdout.split()
    if out.returncode != 0 or len(fields) != 3:
        return {"present": False, "reason": "absent or unreadable (sudo -n stat failed)"}
    mode, owner, size = fields
    ok = mode == "400" and owner == "root" and size.isdigit() and int(size) > 20
    return {"present": ok, "mode": mode, "owner": owner, "size": int(size) if size.isdigit() else None,
            "reason": "" if ok else "token file must be root 0400 and non-empty"}


def strings_in(value):
    """Every string inside a JSON value, unescaping one level of nested JSON."""
    if isinstance(value, str):
        yield value
        stripped = value.strip()
        if stripped[:1] in "{[":
            try:
                yield from strings_in(json.loads(stripped))
            except ValueError:
                pass
    elif isinstance(value, dict):
        for v in value.values():
            yield from strings_in(v)
    elif isinstance(value, list):
        for v in value:
            yield from strings_in(v)


def delegate_evidence(events):
    texts = list(strings_in(events))
    started = any("choir-delegate" in t and '"start"' in t for t in texts)
    run_ids = sorted({m for t in texts for m in re.findall(r'"id":\s*"(d\d{8}T\d{6}-[0-9a-f]{8})"', t)})
    done = [t for t in texts if re.search(r'"state":\s*"done"', t) and re.search(r'"subtype":\s*"success"', t)]
    result = {}
    for t in done:
        try:
            st = json.loads(t[t.index("{"):t.rindex("}") + 1])
        except ValueError:
            continue
        res = st.get("result") or {}
        result = {"run_id": st.get("id"), "session_id": res.get("session_id"), "num_turns": res.get("num_turns"),
                  "total_cost_usd": res.get("total_cost_usd"), "events_sha256": st.get("events_sha256")}
        break
    return {"start_seen": started, "delegate_run_ids": run_ids, "done_with_success": bool(done), "result": result}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cred", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ssh", default="")
    ap.add_argument("--host", default=HOST)
    ap.add_argument("--status-only", action="store_true")
    ap.add_argument("--timeout-min", type=int, default=45)
    args = ap.parse_args()

    c = Computer(args.cred, args.host)
    receipt = {"probe": "claude_delegate_probe", "computer_id": c.id, "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
    health = c.call("GET", "/health", retries=3)
    receipt["staging_health"] = {k: health.get(k) for k in ("commit", "deployed_commit", "build", "status") if isinstance(health, dict) and k in health}

    def finish(code, verdict):
        receipt["verdict"] = verdict
        receipt["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        text = json.dumps(receipt, indent=2)
        open(args.out, "w").write(text + "\n")
        print(text)
        sys.exit(code)

    # 1. Gate: the token must exist.
    if args.ssh:
        receipt["host_token"] = host_token_check(args.ssh)
        log("host_token", **receipt["host_token"])
        if not receipt["host_token"]["present"]:
            finish(2, "refused: no usable token on Node B; nothing started")
    obs = c.call("GET", "/api/runtime/observability")
    status = obs.get("claude_delegate") if isinstance(obs, dict) else None
    receipt["guest_status"] = status
    receipt["guest_build"] = (obs.get("build") or {}) if isinstance(obs, dict) else {}
    log("guest_status", status=status)
    if not status or not status.get("available"):
        finish(2, "refused: the computer reports the delegate unavailable; nothing started")
    if args.status_only:
        finish(0, "pass: delegate available (status only)")

    # 3. End to end with a trivial brief.
    stamp = time.strftime("%Y%m%dT%H%M%S", time.gmtime())
    target = f"docs/probes/claude-delegate-{stamp}.md"
    brief = (
        f"Engineering probe {stamp} for the Claude delegate. Engineering: use the Claude delegate (choir-delegate) "
        f"to create the file {target} with the single line 'claude delegate probe {stamp} ok'. Brief it with "
        f'paths ["docs/probes"], test ["cat", "{target}"], max_turns 8 and wall_clock_min 10. Poll it until it is done, '
        f"read the file yourself with cat in your own cell, then Complete with result completed, naming the delegate's "
        f"run id, session id, turns and cost. Do not Freeze: this is a probe, not a change to land."
    )
    started = c.call("POST", "/api/prompt-bar", {"text": brief, "command_id": f"claude-delegate-probe-{stamp}"})
    tid = started.get("trajectory_id") if isinstance(started, dict) else None
    receipt["prompt_bar"] = {k: started.get(k) for k in ("trajectory_id", "doc_id", "submission_id", "error") if isinstance(started, dict)}
    log("started", **receipt["prompt_bar"])
    if not tid:
        finish(1, "fail: the prompt bar did not return a trajectory")

    deadline = time.time() + args.timeout_min * 60
    eng = None
    while time.time() < deadline:
        runs = c.call("GET", "/api/runs?limit=200").get("runs") or []
        cands = [r for r in runs if r.get("trajectory_id") == tid and r.get("agent_profile") == "engineering"]
        if cands:
            eng = cands[0]
            if eng.get("state") in TERMINAL:
                break
        time.sleep(20)
    if not eng:
        finish(1, "fail: no engineering run appeared for the trajectory in time")
    receipt["engineering_run"] = {"run_id": eng.get("loop_id"), "state": eng.get("state"),
                                  "assignment_id": (eng.get("metadata") or {}).get("assignment_id"),
                                  "capsule_network_mode": (eng.get("metadata") or {}).get("capsule_network_mode")}
    log("engineering_run", **receipt["engineering_run"])
    evidence = delegate_evidence(c.events(eng.get("loop_id")))
    receipt["delegate"] = evidence
    ok = (receipt["engineering_run"]["capsule_network_mode"] == "claude" and evidence["start_seen"]
          and evidence["done_with_success"] and eng.get("state") == "completed")
    finish(0 if ok else 1, "pass: engineering ran the delegate to success" if ok else "fail: see delegate and engineering_run")


if __name__ == "__main__":
    main()
