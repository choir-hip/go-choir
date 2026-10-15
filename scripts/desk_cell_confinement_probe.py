#!/usr/bin/env python3
"""Staging proof of desk cell confinement (F4) and guest egress policy (F6).

Problem record: docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md
Code: internal/yaegikernel/desk_cell_selftest*.go, internal/agentcore/isolation_selftest.go,
internal/vmmanager/guest_egress.go.

No SSH. It asks one computer's guest runtime, through the product API, to run
its isolation self-test (POST /api/runtime/isolation-selftest, owner only):

1. desk_cells: the guest spawns one worker per desk (management, Texture,
   research) through the production spawner and entrypoint. Each worker
   confines itself (its desk's uid, no_new_privs, Landlock, seccomp, empty
   network namespace), then acts as an escaped cell: it opens sockets, execs,
   forks, signals, writes outside its tmp dir and reads files it must not:
   /etc/passwd (world-readable: only Landlock refuses it), the owner's files
   and the store (world-readable directories), the credential disk, and a
   world-readable canary in another desk's live tmp dir. The runtime reads
   each worker's /proc status and namespaces from outside, then runs one
   plain cell in it.
2. egress: the guest runtime (guest root) dials host :22, :13306, :2019,
   169.254.169.254:80 and another guest's address (must be dropped), and the
   gateway :8084, corpusd :8086 and 1.1.1.1:443 (must stay open).

This script judges the raw facts itself (it does not trust the server's
"ok"), prints one JSON receipt on stdout and writes it to --out.

Exit 0: proven. 1: a check failed. 2: the route is not deployed or the
computer could not be reached (nothing proven).

usage: desk_cell_confinement_probe.py --cred FILE --out FILE [--host URL]
  --cred is the 0600 credential file scripts/qa_test_computer.mjs writes
  (api_key or key, computer_id). The key is never printed.
"""
import argparse
import json
import sys
import time
import urllib.error
import urllib.request

ROLES = ["management", "texture", "research"]
ALLOWED = ["write_own_tmp", "list_allowed_root"]
SECCOMP_DENIED = [
    "socket_inet_stream", "socket_inet_dgram", "socket_inet6_stream", "socket_unix_stream",
    "socket_netlink", "socket_packet_raw", "socket_inet_raw",
    "fork_exec", "execve", "kill_pid1", "kill_parent", "setuid_root", "unshare_user", "unshare_net",
]
LANDLOCK_DENIED = ["write_allowed_root"]
EGRESS_WANT = {
    "host_gateway": "open", "host_corpusd": "open", "public_https": "open",
    "host_sshd": "blocked", "host_dolt": "blocked", "host_caddy_admin": "blocked",
    "metadata": "blocked", "other_guest": "blocked",
}


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

    def call(self, method, path, body=None, retries=12, timeout=180):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.id}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        for _ in range(retries):
            try:
                req = urllib.request.Request(self.host + path, data=data, headers=headers, method=method)
                return 200, json.load(urllib.request.urlopen(req, timeout=timeout))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(10)
                    continue
                # Status only: a body can echo request context.
                return err.code, {}
            except urllib.error.URLError:
                time.sleep(10)
        return 0, {}


def judge_desk_cells(dc):
    fails = []
    if dc.get("mode") != "required":
        fails.append("mode=%r, want required (the guest image sets CHOIR_DESK_CELL_CONFINEMENT)" % dc.get("mode"))
    if (dc.get("landlock_abi") or 0) < 5:
        fails.append("landlock_abi=%r, want >= 5" % dc.get("landlock_abi"))
    if dc.get("spawner_uid") != 0:
        fails.append("spawner_uid=%r, want 0 (the guest runtime is root)" % dc.get("spawner_uid"))
    workers = {w.get("role"): w for w in dc.get("workers") or []}
    uids = set()
    for role in ROLES:
        w = workers.get(role)
        p = role + ": "
        if not w:
            fails.append(p + "no worker")
            continue
        if w.get("error"):
            fails.append(p + w["error"])
            continue
        st = w.get("status") or {}
        uid = str(w.get("user_uid") or "")
        fields = (st.get("Uid") or "").split()
        if w.get("user") != "choir-cell-" + role or not uid or uid == "0" or fields != [uid] * 4:
            fails.append(p + "Uid=%r user=%r uid=%r, want choir-cell-%s on all four" % (st.get("Uid"), w.get("user"), uid, role))
        uids.add(uid)
        for k in ("CapInh", "CapPrm", "CapEff", "CapAmb"):
            if (st.get(k) or "x").strip("0") != "":
                fails.append(p + "%s=%r, want 0" % (k, st.get(k)))
        if (st.get("Groups") or "").strip():
            fails.append(p + "Groups=%r, want none" % st.get("Groups"))
        if st.get("NoNewPrivs") != "1":
            fails.append(p + "NoNewPrivs=%r" % st.get("NoNewPrivs"))
        if st.get("Seccomp") != "2":
            fails.append(p + "Seccomp=%r" % st.get("Seccomp"))
        if not w.get("net_ns_isolated"):
            fails.append(p + "shares the runtime's network namespace")
        if not w.get("ipc_ns_isolated"):
            fails.append(p + "shares the runtime's IPC namespace")
        probes = w.get("probes") or {}
        for name in ALLOWED:
            if probes.get(name, "missing") != "":
                fails.append(p + "%s should succeed: %r" % (name, probes.get(name)))
        for name in SECCOMP_DENIED:
            if not str(probes.get(name, "")).startswith("EPERM"):
                fails.append(p + "%s = %r, want EPERM" % (name, probes.get(name)))
        for name in LANDLOCK_DENIED + ["write_etc"]:
            if probes.get(name, "") in ("", None):
                fails.append(p + "%s succeeded (or did not run)" % name)
        targets = w.get("read_targets") or []
        if "/etc/passwd" not in targets:
            fails.append(p + "did not try /etc/passwd")
        canaries = [t for t in targets if t.endswith("/selftest-canary")]
        if role != "management" and len(canaries) != 1:
            fails.append(p + "did not try another desk's canary")
        for t in targets:
            got = probes.get("read:" + t)
            if got in ("", None):
                fails.append(p + "read %s succeeded (or did not run)" % t)
            elif str(got).startswith("ENOENT"):
                fails.append(p + "read %s: %s (a missing path proves nothing)" % (t, got))
        for t in canaries:
            if not str(probes.get("read:" + t, "")).startswith("EACCES"):
                fails.append(p + "read of another desk's canary = %r, want EACCES" % probes.get("read:" + t))
        if not w.get("cell_serves"):
            fails.append(p + "the check cell did not run after confinement: %r" % w.get("cell_output"))
    if len(uids) != len(ROLES):
        fails.append("desks do not have distinct uids: %r" % sorted(uids))
    return fails


def judge_egress(eg):
    fails = []
    got = {e.get("name"): e for e in eg or []}
    for name, want in EGRESS_WANT.items():
        e = got.get(name)
        if not e:
            fails.append("egress %s did not run" % name)
            continue
        result = e.get("result")
        ok = result == "open" if want == "open" else result in ("dropped", "unreachable")
        if not ok:
            fails.append("egress %s %s = %s, want %s" % (name, e.get("addr"), result, want))
    return fails


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--cred", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--host", default="https://choir.news")
    args = ap.parse_args()
    c = Computer(args.cred, args.host)
    receipt = {"kind": "desk_cell_confinement_staging_receipt", "host": c.host, "computer_id": c.id,
               "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}

    def finish(code, verdict):
        receipt["verdict"] = verdict
        receipt["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        text = json.dumps(receipt, indent=2, sort_keys=True)
        print(text)
        with open(args.out, "w") as f:
            f.write(text + "\n")
        sys.exit(code)

    code, obs = c.call("GET", "/api/runtime/observability")
    receipt["guest_build"] = (obs or {}).get("build") if code == 200 else {"error": code}
    log("observability", status=code, commit=((obs or {}).get("build") or {}).get("commit"))
    code, body = c.call("POST", "/api/runtime/isolation-selftest", {})
    log("isolation_selftest", status=code)
    if code == 404 or code == 405:
        finish(2, "route not deployed: the guest image predates the isolation self-test")
    if code != 200:
        finish(2, "self-test unavailable (HTTP %s)" % code)
    receipt["server"] = body
    fails = judge_desk_cells(body.get("desk_cells") or {}) + judge_egress(body.get("egress"))
    if not body.get("ok"):
        fails.append("server judged not ok: %r" % (body.get("failures") or []))
    receipt["failures"] = fails
    finish(0 if not fails else 1, "proven" if not fails else "failed")


if __name__ == "__main__":
    main()
