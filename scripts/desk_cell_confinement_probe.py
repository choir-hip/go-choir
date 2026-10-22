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

3. oom (--leg oom, instead of 1 and 2): the guest spawns a sibling and a
   throwaway research worker under its real 768 MiB budget cgroup in the
   desks slice, and sends the runaway a fixed cell (compiled into the guest;
   nothing is sent from here) that allocates until the kernel kills it. It
   reports the worker cgroup's memory.max, memory.high and memory.events,
   the slice's limits, the daemon's pid, start time and boot id before and
   after, and whether the sibling still answers. A runaway that took the
   daemon leaves no answer: one attempt, never retried, and no answer is a
   failure.

This script judges the raw facts itself (it does not trust the server's
"ok"), prints one JSON receipt on stdout and writes it to --out.

Exit 0: proven. 1: a check failed. 2: the route is not deployed or the
computer could not be reached (nothing proven).

usage: desk_cell_confinement_probe.py --cred FILE --out FILE [--host URL] [--leg oom]
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
# The uid each desk's cells must run as: internal/deskmanifest Sandbox.UID.
# Written out here on purpose, so a manifest edit that moves a desk onto
# another desk's uid fails the probe instead of passing it.
DESK_UIDS = {"management": 61001, "texture": 61002, "research": 61003}
ALLOWED = ["write_own_tmp", "list_allowed_root"]
SECCOMP_DENIED = [
    "socket_inet_stream", "socket_inet_dgram", "socket_inet6_stream", "socket_unix_stream",
    "socket_netlink", "socket_packet_raw", "socket_inet_raw",
    "fork_exec", "execve", "kill_pid1", "kill_parent", "setuid_root", "unshare_user", "unshare_net",
]
LANDLOCK_DENIED = ["write_allowed_root"]
# The OOM leg: the class it runs and that class's hard budget per worker
# (internal/deskmanifest, research WorkerMaxBytes), written out here so a
# manifest edit that loosens the budget fails the probe.
OOM_ROLE = "research"
OOM_BUDGET_BYTES = 768 << 20
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
        last = 0
        for attempt in range(retries):
            try:
                req = urllib.request.Request(self.host + path, data=data, headers=headers, method=method)
                return 200, json.load(urllib.request.urlopen(req, timeout=timeout))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    last = err.code
                    if attempt + 1 < retries:
                        time.sleep(10)
                    continue
                # Status only: a body can echo request context.
                return err.code, {}
            except urllib.error.URLError:
                if attempt + 1 < retries:
                    time.sleep(10)
        return last, {}


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
        want_uid = str(DESK_UIDS[role])
        if not uid or uid == "0" or uid != want_uid or fields != [want_uid] * 4:
            fails.append(p + "Uid=%r planned=%r, want %s (the desk manifest's uid for %s) on all four" % (st.get("Uid"), uid, want_uid, role))
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


# The host-served actions the daemon must serve to each desk, and nothing else.
# Written out here, not read from the guest: this is the authority the batch
# must not widen (internal/deskmanifest, internal/yaegikernel/frame_role_test.go).
FRAME_ALLOWED = {
    "management": {"product_api", "read_capture", "stories"},
    "texture": {"read_capture"},
    "research": {"search_start", "read", "search_wire_corpus", "save_evidence", "read_evidence", "list_evidence",
                 "get_run_memory_entry", "research", "poll_source", "read_capture", "system1", "source_api"},
    "research_child": {"search_start", "read", "read_capture", "search_wire_corpus", "read_evidence", "list_evidence"},
    "management_task": {"read_capture", "stories"},
}
FRAME_ACTIONS = {"exec", "read_file", "write_file", "list_dir", "message", "search_start", "read", "search_wire_corpus",
                 "save_evidence", "read_evidence", "list_evidence", "get_run_memory_entry", "product_api", "research",
                 "poll_source", "read_capture", "stories", "system1", "source_api"}


def judge_frames(fb):
    """The forged-frame probe: raw observations of the daemon's frame admission."""
    fails = []
    roles = {r.get("role"): r for r in (fb or {}).get("roles") or []}
    for role, allowed in FRAME_ALLOWED.items():
        r = roles.get(role)
        p = "frames %s: " % role
        if not r:
            fails.append(p + "no observations")
            continue
        if r.get("error"):
            fails.append(p + r["error"])
            continue
        served, refused = set(r.get("served") or []), set(r.get("refused") or [])
        if served != allowed:
            fails.append(p + "served %r, want exactly %r (extra: %r, missing: %r)" % (
                sorted(served), sorted(allowed), sorted(served - allowed), sorted(allowed - served)))
        if (served | refused) != FRAME_ACTIONS or (served & refused):
            fails.append(p + "served and refused do not partition the actions: %r %r" % (sorted(served), sorted(refused)))
        for case in ("wrong_token", "missing_token", "foreign_token", "stale_epoch", "bad_protocol", "emit_stream", "bad_version_header"):
            if not str(r.get(case, "")).startswith("refused"):
                fails.append(p + "%s = %r, want a refusal" % (case, r.get(case)))
        if role != "management" and not str(r.get("claimed_other_desk", "")).startswith("refused"):
            fails.append(p + "claiming management got %r, want a refusal" % r.get("claimed_other_desk"))
        if role == "management" and r.get("claimed_other_desk") != "served":
            fails.append(p + "management's own product_api was %r" % r.get("claimed_other_desk"))
        if not r.get("wrong_token_killed"):
            fails.append(p + "a wrong slot token did not end the worker")
        if r.get("refused_reached_handler"):
            fails.append(p + "%d refused frames reached a handler" % r["refused_reached_handler"])
        if not r.get("refusals_reported"):
            fails.append(p + "no refusal was reported to the host's trace hook")
    return fails


def judge_oom(oom):
    """The out-of-memory leg: killed by its cgroup at its budget, nothing else harmed."""
    fails = []
    if not oom:
        return ["oom: no receipt (the guest predates the OOM leg and ran the default self-test)"]
    w = oom.get("worker") or {}
    cg = w.get("cgroup") or {}
    sib = oom.get("sibling") or {}
    if oom.get("role") != OOM_ROLE:
        fails.append("oom: role=%r, want %s" % (oom.get("role"), OOM_ROLE))
    if w.get("error"):
        fails.append("oom: worker: %s" % w["error"])
    if cg.get("memory_max") != str(OOM_BUDGET_BYTES):
        fails.append("oom: worker memory.max=%r, want %d (768 MiB)" % (cg.get("memory_max"), OOM_BUDGET_BYTES))
    slice_max = str((oom.get("slice") or {}).get("memory_max") or "")
    if not slice_max.isdigit() or int(slice_max) == 0:
        fails.append("oom: desks slice memory.max=%r, want a hard limit" % slice_max)
    if not w.get("cell_sent"):
        fails.append("oom: the runaway cell was not sent")
    else:
        if w.get("result") == "returned" or not w.get("dead"):
            fails.append("oom: the runaway worker survived (result %r)" % w.get("result"))
        oom_kill = w.get("oom_kill") or 0
        if oom_kill < 1 or ((cg.get("memory_events") or {}).get("oom_kill") or 0) < 1:
            fails.append("oom: memory.events oom_kill=%r: the cgroup did not kill it (exit %r)" % (oom_kill, w.get("exit_cause")))
        if "killed by its memory budget" not in str(w.get("exit_cause") or ""):
            fails.append("oom: exit cause does not name the budget: %r" % w.get("exit_cause"))
    if not sib.get("before_answered") or not sib.get("after_answered"):
        fails.append("oom: sibling before=%r after=%r (%r)" % (sib.get("before_answered"), sib.get("after_answered"), sib.get("output") or sib.get("error")))
    b, a = oom.get("daemon_before") or {}, oom.get("daemon_after") or {}
    if b.get("error") or a.get("error") or not b.get("pid") or not b.get("start_ticks") or not b.get("boot_id"):
        fails.append("oom: daemon identity unread: before %r after %r" % (b, a))
    elif any(b.get(k) != a.get(k) for k in ("pid", "start_ticks", "boot_id")):
        fails.append("oom: daemon identity changed: before %r after %r" % (b, a))
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
    ap.add_argument("--leg", choices=["confinement", "oom"], default="confinement",
                    help="oom: run only the out-of-memory leg (a throwaway research worker is killed by its "
                         "cgroup; one attempt, never retried)")
    ap.add_argument("--skip-egress", action="store_true",
                    help="judge the desk cells only. The guest egress checks need the host egress policy "
                         "(SEC-03, internal/vmmanager/guest_egress.go) to be deployed; before it lands, "
                         "host :22 and the metadata address are open and those checks fail by design.")
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
    if args.leg == "oom":
        receipt["leg"] = "oom"
        code, body = c.call("POST", "/api/runtime/isolation-selftest?leg=oom", {}, retries=1)
        log("isolation_selftest_oom", status=code)
        if code in (400, 404, 405):
            finish(2, "OOM leg not deployed (HTTP %s)" % code)
        if code != 200:
            receipt["failures"] = ["oom: no answer (HTTP %s): the daemon may have died with the runaway; "
                                   "read its identity in observability before retrying" % code]
            finish(1, "failed")
        receipt["server"] = body
        fails = judge_oom(body.get("oom"))
        if body.get("oom") and not body.get("ok"):
            fails.append("server judged not ok: %r" % (body.get("failures") or []))
        receipt["failures"] = fails
        if not body.get("oom"):
            finish(2, "OOM leg not deployed: the guest ran the default self-test")
        finish(0 if not fails else 1, "proven" if not fails else "failed")
    code, body = c.call("POST", "/api/runtime/isolation-selftest", {})
    log("isolation_selftest", status=code)
    if code == 404 or code == 405:
        finish(2, "route not deployed: the guest image predates the isolation self-test")
    if code != 200:
        finish(2, "self-test unavailable (HTTP %s)" % code)
    receipt["server"] = body
    fails = judge_desk_cells(body.get("desk_cells") or {}) + judge_frames(body.get("frame_boundary"))
    if args.skip_egress:
        receipt["egress_judged"] = False
        server_fails = [f for f in (body.get("failures") or []) if not f.startswith("egress:")]
        if server_fails:
            fails.append("server judged not ok: %r" % server_fails)
    else:
        fails += judge_egress(body.get("egress"))
        if not body.get("ok"):
            fails.append("server judged not ok: %r" % (body.get("failures") or []))
    receipt["failures"] = fails
    finish(0 if not fails else 1, "proven" if not fails else "failed")


if __name__ == "__main__":
    main()
