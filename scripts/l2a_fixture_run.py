#!/usr/bin/env python3
"""Run the L2a fixture on a freshly minted test computer (acceptance action 1).

1. Mint a test computer (scripts/qa_test_computer.mjs). Its key is written
   to a 0600 file and never printed.
2. Turn on intake.labels (POST /api/computer-settings).
3. Start one schedule on the fixture source, marked as a fixture run.
4. Wait while the computer polls, labels, opens and publishes, printing
   progress: captures, stories, published versions.
5. Score it with scripts/l2a_fixture_score.py against the labels file,
   which lives outside the repo.
6. Stop the schedule, then stop the test computer (POST
   /api/computers/<id>/lifecycle/stop) unless --keep-computer is given.

The run acts only on the computer it minted.

usage: l2a_fixture_run.py --labels FILE --out-dir DIR [--max-items 9]
       [--interval-min 10] [--wait-min 120] [--keep-computer]
"""
import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

HOST = "https://choir.news"
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SOURCE = "fixture:l2a_multilingual_2026_10_14"


class Computer:
    def __init__(self, cred_file, computer_id):
        cred = json.load(open(cred_file))
        if cred.get("computer_id") != computer_id:
            raise SystemExit("credential file names a different computer than the mint printed")
        self.key = cred.get("api_key") or cred.get("key")
        self.computer = computer_id

    def call(self, method, path, body=None):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        for _ in range(20):
            try:
                req = urllib.request.Request(HOST + path, data=data, headers=headers, method=method)
                return json.load(urllib.request.urlopen(req, timeout=120))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(15)
                    continue
                raise SystemExit(f"{method} {path}: {err.code} {err.read()[:300]!r}")
        raise SystemExit(f"{method} {path}: unavailable")


def log(event, **kv):
    print(json.dumps({"t": time.strftime("%H:%M:%S", time.gmtime()), "event": event, **kv}), flush=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--labels", required=True)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--max-items", type=int, default=9)
    ap.add_argument("--interval-min", type=int, default=10)
    ap.add_argument("--wait-min", type=int, default=120)
    ap.add_argument("--keep-computer", action="store_true")
    args = ap.parse_args()
    os.makedirs(args.out_dir, exist_ok=True)
    label = "l2a-fixture-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())

    mint = subprocess.run(["node", os.path.join(REPO, "scripts", "qa_test_computer.mjs"), "--label", label],
                          cwd=REPO, capture_output=True, text=True, timeout=1800)
    if mint.returncode != 0:
        raise SystemExit("mint failed: " + mint.stderr[-400:])
    minted = json.loads([ln for ln in mint.stdout.splitlines() if ln.strip()][-1])
    log("minted", computer_id=minted["computer_id"], build=minted.get("guest_build_commit"))
    c = Computer(minted["credential_file"], minted["computer_id"])
    json.dump({"computer_id": minted["computer_id"], "credential_file": minted["credential_file"], "label": label},
              open(os.path.join(args.out_dir, "run.json"), "w"), indent=1)

    c.call("POST", "/api/computer-settings", {"key": "intake.labels", "value": True})
    log("intake_labels_on", settings=c.call("GET", "/api/computer-settings"))
    sched = c.call("POST", "/api/autopaper/sources/schedules", {
        "source_id": SOURCE, "interval_minutes": args.interval_min, "max_items": args.max_items,
        "fixture_run": True, "reason": "L2a fixture run (acceptance action 1)",
    })
    log("schedule_started", schedule=sched)

    deadline = time.time() + args.wait_min * 60
    last = None
    while time.time() < deadline:
        time.sleep(120)
        scheds = c.call("GET", "/api/autopaper/sources/schedules").get("schedules") or []
        s = next((x for x in scheds if x.get("source_id") == SOURCE), {})
        docs = c.call("GET", "/api/texture/documents?limit=500")
        docs = docs.get("documents") or docs.get("items") or docs
        state = {"schedule": s.get("state"), "polls": len(s.get("recent_polls") or s.get("polls") or []),
                 "documents": len(docs)}
        if state != last:
            log("progress", **state)
            last = state

    stop = c.call("DELETE", "/api/autopaper/sources/schedules?source_id=" + SOURCE)
    log("schedule_stopped", result=stop)
    score_out = os.path.join(args.out_dir, "score.json")
    subprocess.run([sys.executable, os.path.join(REPO, "scripts", "l2a_fixture_score.py"),
                    "--cred", minted["credential_file"], "--labels", args.labels, "--out", score_out], check=False)
    log("scored", out=score_out)
    if not args.keep_computer:
        res = c.call("POST", f"/api/computers/{c.computer}/lifecycle/stop", {"idempotency_key": label + "-stop"})
        log("computer_stopped", state=res.get("resulting_lifecycle_state"))


if __name__ == "__main__":
    main()
