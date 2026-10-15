#!/usr/bin/env python3
"""OVHcloud (US) API helper for Node B: status, reboot, rescue boot.

Read-only by default. Every state change needs --execute.

  python3 scripts/ovh_node_b.py status
  python3 scripts/ovh_node_b.py reboot --execute          # hard reboot from disk
  python3 scripts/ovh_node_b.py boot-rescue --execute     # next boot: OVH rescue, then reboot
  python3 scripts/ovh_node_b.py boot-disk --execute       # next boot: hard disk (undo rescue)
  python3 scripts/ovh_node_b.py task <task-id>            # poll a reboot task

Credentials come from the operator .env (OVH_APPLICATION_KEY,
OVH_APPLICATION_SECRET, OVH_CONSUMER_KEY) and are never printed. The
server name and the boot ids were read from the API on 2026-10-15
(docs/runbooks/node-b-host-reboot.md): harddisk = 1, rescue12-customer =
218949. The script re-reads the boot list and refuses ids it does not see.
Rescue-mode credentials go to the OVH account email (rescueMail is unset).
"""
import argparse
import hashlib
import json
import os
import re
import sys
import urllib.request

BASE = "https://api.us.ovhcloud.com/1.0"
SERVER = "ns106285.ip-147-135-70.us"
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def creds():
    env = {}
    path = os.environ.get("CHOIR_PROVIDER_ENV_FILE", os.path.join(REPO, ".env"))
    with open(path) as f:
        for line in f:
            m = re.match(r"^\s*(?:export\s+)?(OVH_[A-Z_]+)\s*=\s*(.*)$", line.rstrip("\n"))
            if m:
                env[m.group(1)] = m.group(2).strip().strip('"').strip("'")
    for k in ("OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET", "OVH_CONSUMER_KEY"):
        if not env.get(k):
            sys.exit(f"{k} missing from {path}")
    return env["OVH_APPLICATION_KEY"], env["OVH_APPLICATION_SECRET"], env["OVH_CONSUMER_KEY"]


def call(method, path, body=None):
    ak, sec, ck = creds()
    url = BASE + path
    data = "" if body is None else json.dumps(body)
    t = json.load(urllib.request.urlopen(BASE + "/auth/time", timeout=15))
    sig = "$1$" + hashlib.sha1("+".join([sec, ck, method, url, data, str(t)]).encode()).hexdigest()
    req = urllib.request.Request(url, method=method, data=data.encode() if body is not None else None, headers={
        "X-Ovh-Application": ak, "X-Ovh-Consumer": ck, "X-Ovh-Timestamp": str(t),
        "X-Ovh-Signature": sig, "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as r:
        raw = r.read()
    return json.loads(raw) if raw else None


def boots():
    out = {}
    for b in call("GET", f"/dedicated/server/{SERVER}/boot"):
        d = call("GET", f"/dedicated/server/{SERVER}/boot/{b}")
        out[d.get("bootType")] = (b, d.get("kernel"))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cmd", choices=["status", "reboot", "boot-rescue", "boot-disk", "task"])
    ap.add_argument("arg", nargs="?")
    ap.add_argument("--execute", action="store_true")
    a = ap.parse_args()

    if a.cmd == "status":
        d = call("GET", f"/dedicated/server/{SERVER}")
        print(json.dumps({k: d.get(k) for k in ("name", "state", "bootId", "datacenter", "commercialRange", "monitoring")}, indent=2))
        print("boots:", boots())
        return
    if a.cmd == "task":
        print(json.dumps(call("GET", f"/dedicated/server/{SERVER}/task/{a.arg}"), indent=2))
        return
    b = boots()
    if a.cmd in ("boot-rescue", "boot-disk"):
        kind = "rescue" if a.cmd == "boot-rescue" else "harddisk"
        if kind not in b:
            sys.exit(f"no {kind} boot option listed: {b}")
        boot_id = b[kind][0]
        print(f"set bootId={boot_id} ({kind}/{b[kind][1]})" + ("" if a.execute else "  [dry run]"))
        if a.execute:
            call("PUT", f"/dedicated/server/{SERVER}", {"bootId": boot_id})
        if a.cmd == "boot-disk":
            return
    print("hard reboot " + SERVER + ("" if a.execute else "  [dry run]"))
    if a.execute:
        task = call("POST", f"/dedicated/server/{SERVER}/reboot")
        print(json.dumps({k: task.get(k) for k in ("taskId", "function", "status")}))


if __name__ == "__main__":
    main()
