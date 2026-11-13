#!/usr/bin/env python3
"""Fake curl for scripts/chaos/selftest.sh: answers the d@e product API and
the Node B loopback/guest routes the drills use, from state files under $ST.
Logs argv (never header-file contents) to $ST/curl.log, and whether the
Authorization header file carried the expected key, never the key itself."""
import hashlib
import json
import os
import sys
from urllib.parse import urlparse, parse_qs

ST = os.environ["ST"]
args = sys.argv[1:]


def flag(name):
    return os.path.exists(os.path.join(ST, name))


url, method, data, wout, headers, outfile = "", "GET", None, None, [], None
i = 0
while i < len(args):
    a = args[i]
    if a.startswith("http://") or a.startswith("https://"):
        url = a
    elif a == "-X":
        method = args[i + 1]; i += 1
    elif a in ("-d", "--data-binary"):
        data = args[i + 1]; i += 1
    elif a == "-w":
        wout = args[i + 1]; i += 1
    elif a in ("-H",):
        headers.append(args[i + 1]); i += 1
    elif a == "-o":
        outfile = args[i + 1]; i += 1
    elif a in ("-D", "--max-time", "--noproxy"):
        i += 1
    i += 1

auth = "none"
for h in headers:
    if h.startswith("@"):
        try:
            with open(h[1:]) as f:
                content = f.read()
        except OSError:
            content = ""
        if content.startswith("Authorization: Bearer "):
            auth = "ok" if content.strip().split(" ", 2)[2] == os.environ.get("FAKE_KEY", "") else "wrong"
        elif h[1:] == os.environ.get("CHOIR_INTERNAL_TOKEN_HEADER", ""):
            auth = "internal-token"
with open(os.path.join(ST, "curl.log"), "a") as f:
    f.write(json.dumps({"argv": args, "method": method, "url": url, "auth": auth}) + "\n")

u = urlparse(url)
path, q = u.path, parse_qs(u.query)
T = "computer-59c902c5d46caecc0a180a8557164e72"
killed = flag("killed")
code, body = 200, {}


def seq(line):
    with open(os.path.join(ST, "seq.log"), "a") as f:
        f.write(line + "\n")


if u.hostname == "choir.news":
    if path == "/api/compute/status":
        body = {"computers": [{"computer_id": T, "state": "active"}]}
    elif path == "/api/runtime/observability":
        body = {"ok": True}
    elif path == "/api/computer-settings":
        body = {"settings": [{"key": "telemetry.batch", "value": True}]}
    elif path.startswith("/api/files/"):
        store = os.path.join(ST, "files", path[len("/api/files/"):])
        if method == "PUT":
            if flag("api_down"):
                code = 503
            else:
                os.makedirs(os.path.dirname(store), exist_ok=True)
                with open(store, "w") as f:
                    f.write(data or "")
                body = {"status": "updated"}
        elif method == "POST":
            body = {"status": "created"}
        else:
            if os.path.isfile(store):
                sys.stdout.write(open(store).read())
                body = None
            else:
                code = 404
    elif path == "/api/texture/lifecycle-documents":
        rid = json.loads(data or "{}").get("client_request_id", "")
        h = hashlib.sha256(rid.encode()).hexdigest()[:12]
        body = {"schema": "choir.texture_create.v1", "doc_id": "doc-" + h, "revision_id": "rev-" + h}
    elif path.startswith("/api/texture/documents/"):
        body = {"doc_id": path.rsplit("/", 1)[-1]}
    elif path == "/api/prompt-bar":
        body = {"submission_id": "sub-" + json.loads(data or "{}").get("command_id", "")}
    elif path == "/api/runs":
        st = "cancelled" if killed else "running"
        body = {"runs": [{"run_id": "run-1", "state": st, "error": "interrupted_by_restart" if killed else ""}]}
    elif path == "/api/runs/run-1":
        body = {"run_id": "run-1", "state": "cancelled" if killed else "running", "error": "interrupted_by_restart" if killed else ""}
    elif path == "/api/runs/run-1/events":
        ev = [{"kind": "loop.started", "payload": {}}]
        if killed:
            ev.append({"kind": "loop.cancelled", "payload": {"fate": "interrupted_by_restart", "telemetry": "the last window of this run's trace and run memory may be missing"}})
        body = {"events": ev}
    elif path.endswith("/self-development/replay-completeness"):
        body = {"result": {"status": "equivalent"}}
    else:
        code = 404
elif u.port == 8086 and path == "/internal/computers/events/replay":
    after = int(q.get("after_sequence", ["0"])[0])
    body = [{"request": {"sequence": after + k}, "event_head_receipt": {"receipt_kind": "event_head", "signature_set": [{"s": 1}], "sequence": after + k}} for k in range(1, 6)]
elif u.port == 8086 and path == "/health":
    body = {"tape": {"segment_fallbacks": 0, "reader": {"segment_error_fallbacks": 0}}}
elif u.port == 8083 and path == "/internal/vmctl/deploy-refresh":
    seq("deploy-refresh POST")
    if flag("refresh_defer"):
        code, body = 409, {"outcome": "deferred", "reason": "busy"}
    else:
        body = {"outcome": "refreshed", "planned_restart": True, "vm_id": "vm-de"}
        with open(os.environ["FAKE_CONSOLE"], "a") as f:
            f.write("runtime: host requested a planned restart (platform update x)\nautoputer: restart kind=planned reason=platform update\n")
elif path == "/health":
    code = 503 if flag("paper_down") else 200
elif path == "/internal/runtime/planned-restart":
    seq("guest-marker " + method + " auth=" + auth)
    body = {"status": "cleared"}
else:
    code = 404

if body is not None and outfile is None:
    sys.stdout.write(json.dumps(body))
if wout:
    sys.stdout.write(wout.replace("\\n", "\n").replace("%{http_code}", str(code)))
sys.exit(22 if code >= 400 and any(a in ("-f", "-fsS") for a in args) else 0)
