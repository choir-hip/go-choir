#!/usr/bin/env python3
"""Staging proof of desk cell confinement (F4) and guest egress policy (F6).

Problem record: docs/problems/desk-cells-run-as-guest-root-and-egress-is-open-2026-10-14.md
Code: internal/yaegikernel/desk_cell_selftest*.go, internal/agentcore/isolation_selftest.go,
internal/vmmanager/guest_egress.go.

Legs 1 and 2 use no SSH: they ask one computer's guest runtime, through the
product API, to run its isolation self-test (POST
/api/runtime/isolation-selftest, owner only). Legs 3 and 4 load the guest on
purpose and are host-internal: they POST
/internal/runtime/isolation-selftest?leg=oom|seccomp on the guest's own
address from Node B over ssh, with no credential and no marker header: the
host-internal service token is never sent to a guest
(docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md).
No API key is used. The public edge refuses /internal/*, the proxy never
forwards it, and the guest admits only its host peer (its default gateway on
the tap). The deployed guest runs hostauth in off mode, so only that
transport check gates the route (review bunch 27 E1;
HandleInternalIsolationSelfTest: GuestHostCaller, then hostauth.Authorize).

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
2. document_tool: the guest runs its own binary as a document tool through
   the document-tool sandbox (uid 61010, empty network namespace, Landlock,
   seccomp; SEC-15), acting as a hostile tool: reads, writes, sockets,
   namespaces, mount, ptrace.
3. runtime_capabilities: the daemon's own bounding set against the floor
   (SEC-15); judged only when the image says enforce.
4. egress: the guest runtime (guest root) dials host :22, :13306, :2019,
   169.254.169.254:80 and another guest's address (must be dropped), and the
   gateway :8084, corpusd :8086 and 1.1.1.1:443 (must stay open).

3. oom (--leg oom, instead of 1 and 2): if MemAvailable covers the budget
   plus 1 GiB, the guest spawns a sibling (it must answer first) and a
   throwaway research worker under its real 768 MiB budget cgroup in the
   desks slice. It sends the runaway a fixed cell, compiled into the guest
   with nothing sent from here. The cell's first act is a start marker the
   host records; then it allocates until the kernel kills it. It
   reports the worker cgroup's memory.max, memory.high and memory.events,
   the slice's limits, the daemon's pid, start time and boot id before and
   after, and whether the sibling still answers. A runaway that took the
   daemon leaves no answer: one attempt, never retried, and no answer is a
   failure.

4. seccomp (--leg seccomp): the guest re-execs its own binary once per
   capsule seccomp filter. Each child loads the filter the capsule's
   constructor builds and tries clone3 (must be ENOSYS), a clone and an
   unshare into a new user namespace (must be EPERM), then fork+exec and
   os/exec (must work). SEC-13 on the deployed kernel; it does not show that
   a live capsule loads the filter, nor the session worker's bounds (RES-03).

This script judges the raw facts itself (it does not trust the server's
"ok"), prints one JSON receipt on stdout and writes it to --out.

Exit 0: proven. 1: a check failed. 2: the route is not deployed or the
computer could not be reached (nothing proven).

usage: desk_cell_confinement_probe.py --cred FILE --out FILE [--host URL]
       desk_cell_confinement_probe.py --leg oom|seccomp --guest-url URL --out FILE [--ssh node-b] [--computer ID]
  --cred is the 0600 credential file scripts/qa_test_computer.mjs writes
  (api_key or key, computer_id). The key is never printed.
"""
import argparse
import json
import re
import subprocess
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
# The free memory the guest must have beyond the budget before the runaway
# (internal/yaegikernel deskOOMHeadroomBytes: the daemon's MemoryMin plus
# room for the other desks and the page cache).
OOM_HEADROOM_BYTES = 1 << 30
# The seccomp leg: every capsule filter (internal/capsule/seccomp.go) and what
# each probe must answer under it, written out here.
SECCOMP_FILTERS = ["workload", "workload-inet", "broker", "broker-inet", "delegate", "broker-delegate"]
SECCOMP_WANT = {"clone3": "ENOSYS", "clone_newuser": "EPERM", "unshare_newuser": "EPERM", "fork_exec": "", "os_exec": ""}
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
                 "get_run_memory_entry", "research", "poll_source", "read_capture", "system1", "relate_stories", "relate", "source_api"},
    "research_child": {"search_start", "read", "read_capture", "search_wire_corpus", "read_evidence", "list_evidence"},
    "management_task": {"read_capture", "stories"},
}
FRAME_ACTIONS = {"exec", "read_file", "write_file", "list_dir", "message", "search_start", "read", "search_wire_corpus",
                 "save_evidence", "read_evidence", "list_evidence", "get_run_memory_entry", "product_api", "research",
                 "poll_source", "read_capture", "stories", "system1", "relate_stories", "relate", "source_api"}


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
    if oom.get("mode") != "required":
        fails.append("oom: confinement mode %r, want required (the guest's production mode)" % oom.get("mode"))
    avail, need = oom.get("mem_available_bytes") or 0, oom.get("required_available_bytes") or 0
    if need != OOM_BUDGET_BYTES + OOM_HEADROOM_BYTES:
        fails.append("oom: required available %r, want the budget plus %d headroom" % (need, OOM_HEADROOM_BYTES))
    elif not avail or avail < need:
        fails.append("oom: MemAvailable %r below the required %r" % (avail, need))
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
        if not w.get("cell_started"):
            fails.append("oom: the cell's start marker never reached the host (a kill before the cell ran proves nothing)")
        if w.get("result") == "returned" or not w.get("dead"):
            fails.append("oom: the runaway worker survived (result %r)" % w.get("result"))
        oom_kill = w.get("oom_kill") or 0
        if oom_kill < 1 or ((cg.get("memory_events") or {}).get("oom_kill") or 0) < 1:
            fails.append("oom: memory.events oom_kill=%r: the cgroup did not kill it (exit %r)" % (oom_kill, w.get("exit_cause")))
        if "killed by its memory budget" not in str(w.get("exit_cause") or ""):
            fails.append("oom: exit cause does not name the budget: %r" % w.get("exit_cause"))
    if not sib.get("before_answered") or not sib.get("after_answered"):
        fails.append("oom: sibling before=%r after=%r (%r)" % (sib.get("before_answered"), sib.get("after_answered"),
                                                                sib.get("error") or sib.get("output_after") or sib.get("output_before")))
    b, a = oom.get("daemon_before") or {}, oom.get("daemon_after") or {}
    if b.get("error") or a.get("error") or not b.get("pid") or not b.get("start_ticks") or not b.get("boot_id"):
        fails.append("oom: daemon identity unread: before %r after %r" % (b, a))
    elif any(b.get(k) != a.get(k) for k in ("pid", "start_ticks", "boot_id")):
        fails.append("oom: daemon identity changed: before %r after %r" % (b, a))
    return fails


def judge_seccomp(sc):
    """The capsule seccomp leg: each filter loaded, clone3 ENOSYS, no new user namespace, children start."""
    if not sc:
        return ["seccomp: no receipt (the guest predates the seccomp leg and ran the default self-test)"]
    fails = []
    if not str(sc.get("kernel") or "").strip():
        fails.append("seccomp: kernel release unread")
    got = {}
    for f in sc.get("filters") or []:
        if f.get("name") in got:
            fails.append("seccomp: filter %s reported twice" % f.get("name"))
        got[f.get("name")] = f
    hashes = set()
    for name in SECCOMP_FILTERS:
        f = got.get(name)
        p = "seccomp %s: " % name
        if not f:
            fails.append(p + "did not run")
            continue
        if f.get("error") or f.get("load_error") or not f.get("loaded"):
            fails.append(p + "not loaded: %r" % (f.get("error") or f.get("load_error")))
            continue
        h = f.get("bpf_sha256") or ""
        if not h or h != f.get("expected_sha256"):
            fails.append(p + "loaded program %r is not the capsule's %r" % (h, f.get("expected_sha256")))
        hashes.add(h)
        if f.get("seccomp") != "2":
            fails.append(p + "Seccomp=%r after the load, want 2" % f.get("seccomp"))
        probes = f.get("probes") or {}
        for probe, want in SECCOMP_WANT.items():
            if probe not in probes:
                fails.append(p + "%s did not run" % probe)
            elif want == "" and probes[probe] != "":
                fails.append(p + "%s = %r, want it to work under the filter" % (probe, probes[probe]))
            elif want and not str(probes[probe]).startswith(want + ":"):
                fails.append(p + "%s = %r, want %s" % (probe, probes[probe], want))
    if len(hashes) != len([n for n in SECCOMP_FILTERS if n in got]):
        fails.append("seccomp: two filters loaded the same program: %r" % sorted(hashes))
    return fails


# The document-tool sandbox (SEC-15, internal/doctool): the uid is written out
# here for the same reason as DESK_UIDS.
DOCTOOL_UID = 61010
DOCTOOL_DENIED = ["write_input_dir", "write_tmp", "socket_inet", "socket_unix",
                  "unshare_user", "unshare_net", "mount", "ptrace_parent"]


def judge_document_tool(dt):
    """The daemon's binary run as a hostile document tool inside the sandbox."""
    fails = []
    r = (dt or {}).get("report")
    if not r:
        return ["document_tool: no report (warnings %r, failures %r)" % (
            (dt or {}).get("warnings"), (dt or {}).get("failures"))]
    if r.get("uid") != DOCTOOL_UID or r.get("gid") != DOCTOOL_UID:
        fails.append("document_tool: ran as uid %r gid %r, want %d" % (r.get("uid"), r.get("gid"), DOCTOOL_UID))
    st = r.get("status") or {}
    if st.get("NoNewPrivs") != "1" or st.get("Seccomp") != "2" or st.get("CapEff") != "0000000000000000":
        fails.append("document_tool: status %r" % st)
    probes = r.get("probes") or {}
    if probes.get("read_input") != "":
        fails.append("document_tool: could not read its input: %r" % probes.get("read_input"))
    reads = [k for k in probes if k.startswith("read:")]
    if "read:/etc/passwd" not in reads:
        fails.append("document_tool: /etc/passwd (only Landlock refuses it) was not probed")
    for name in DOCTOOL_DENIED + reads:
        got = probes.get(name)
        if got is None:
            fails.append("document_tool: probe %s missing" % name)
        elif got == "" or got.startswith("ENOENT"):
            fails.append("document_tool: probe %s = %r, want a denial" % (name, got))
        elif name.startswith("read:") and not got.startswith(("EACCES", "EPERM")):
            # EISDIR or ELOOP fail outside the sandbox too (review E4).
            fails.append("document_tool: probe %s = %r, want EACCES or EPERM" % (name, got))
    return fails


# The daemon's capability floor (SEC-15, internal/capfloor), written out here.
CAPFLOOR = "00000000032c15ff"


def judge_runtime_capabilities(rc):
    """Report-only under observe; under enforce the daemon's set is the floor."""
    rc = rc or {}
    if rc.get("floor") and rc.get("floor") != CAPFLOOR:
        return ["runtime_capabilities: the daemon's floor is %r, the probe's %r" % (rc.get("floor"), CAPFLOOR)]
    if rc.get("mode") == "enforce" and (rc.get("extra") or rc.get("cap_bnd") != CAPFLOOR):
        return ["runtime_capabilities: enforce, but CapBnd %r holds %r" % (rc.get("cap_bnd"), rc.get("extra"))]
    return []


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


GUEST_URL = re.compile(r"^http://[0-9]{1,3}(\.[0-9]{1,3}){3}:[0-9]{1,5}$")


def internal_post(ssh_host, guest_url, leg):
    """POST the guest's internal self-test route from Node B. It carries no
    credential and no marker: the guest's source-address check is the gate.
    One attempt. Returns (http status or 0 for no answer, parsed body)."""
    script = ("curl -sS -m 170 -X POST -w '\\nhttp_status=%%{http_code}\\n' "
              "'%s/internal/runtime/isolation-selftest?leg=%s'") % (guest_url, leg)
    try:
        p = subprocess.run(["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", ssh_host, script],
                           capture_output=True, text=True, timeout=200)
    except subprocess.TimeoutExpired:
        return 0, {}
    if p.returncode != 0:
        log("ssh_failed", rc=p.returncode)
        return 0, {}
    text, _, status = p.stdout.rstrip("\n").rpartition("\nhttp_status=")
    try:
        code = int(status.strip())
    except ValueError:
        return 0, {}
    try:
        body = json.loads(text) if text.strip() else {}
    except ValueError:
        body = {}
    return code, body


def run_internal_leg(args, finish, receipt):
    """The oom and seccomp legs: host-internal, reached from Node B over ssh."""
    receipt.update({"leg": args.leg, "guest_url": args.guest_url, "computer_id": args.computer, "via": "ssh " + args.ssh})
    if not args.guest_url or not GUEST_URL.match(args.guest_url):
        finish(2, "--guest-url must be http://<guest-ip>:<port> (the computer_url vmctl lists)")
    code, body = internal_post(args.ssh, args.guest_url, args.leg)
    log("isolation_selftest_" + args.leg, status=code)
    if code in (400, 403, 404, 405):
        finish(2, "%s leg not deployed or not authorized (HTTP %s): nothing proven" % (args.leg, code))
    if code != 200:
        if args.leg == "oom":
            # Never retried: a retry could run the runaway again on a restarted daemon.
            if code == 409:
                # The guest's self-test lock was held: this call ran nothing.
                receipt["failures"] = ["oom: HTTP 409: an isolation self-test is already running on this computer, "
                                       "so this call ran nothing; wait for the other run to end and read the "
                                       "daemon's identity before retrying"]
            else:
                receipt["failures"] = ["oom: no answer (HTTP %s): the daemon may have died with the runaway; "
                                       "read its identity before retrying" % code]
            finish(1, "failed")
        finish(2, "self-test unavailable (HTTP %s)" % code)
    receipt["server"] = body
    leg_receipt = body.get(args.leg)
    fails = judge_oom(leg_receipt) if args.leg == "oom" else judge_seccomp(leg_receipt)
    receipt["failures"] = fails
    if not leg_receipt:
        finish(2, "%s leg not deployed: no receipt" % args.leg)
    if not body.get("ok"):
        fails.append("server judged not ok: %r" % (body.get("failures") or []))
    finish(0 if not fails else 1, "proven" if not fails else "failed")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--cred", help="the confinement leg's credential file (not used by --leg oom|seccomp)")
    ap.add_argument("--out", required=True)
    ap.add_argument("--host", default="https://choir.news")
    ap.add_argument("--leg", choices=["confinement", "oom", "seccomp"], default="confinement",
                    help="oom: the out-of-memory leg (a throwaway research worker is killed by its cgroup; one "
                         "attempt, never retried). seccomp: the capsule seccomp leg. Both are host-internal: "
                         "they run from Node B over ssh and need --guest-url")
    ap.add_argument("--ssh", default="node-b", help="the host that reaches the guest (oom, seccomp)")
    ap.add_argument("--guest-url", help="the guest's computer_url from vmctl's list (oom, seccomp)")
    ap.add_argument("--computer", default="", help="the computer id, recorded in the receipt (oom, seccomp)")
    ap.add_argument("--skip-egress", action="store_true",
                    help="judge the desk cells only. The guest egress checks need the host egress policy "
                         "(SEC-03, internal/vmmanager/guest_egress.go) to be deployed; before it lands, "
                         "host :22 and the metadata address are open and those checks fail by design.")
    args = ap.parse_args()
    receipt = {"kind": "desk_cell_confinement_staging_receipt",
               "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}

    def finish(code, verdict):
        receipt["verdict"] = verdict
        receipt["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        text = json.dumps(receipt, indent=2, sort_keys=True)
        print(text)
        with open(args.out, "w") as f:
            f.write(text + "\n")
        sys.exit(code)

    if args.leg in ("oom", "seccomp"):
        run_internal_leg(args, finish, receipt)
    if not args.cred:
        finish(2, "--cred is required for the confinement leg")
    c = Computer(args.cred, args.host)
    receipt.update({"host": c.host, "computer_id": c.id})
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
    fails = (judge_desk_cells(body.get("desk_cells") or {}) + judge_frames(body.get("frame_boundary"))
             + judge_document_tool(body.get("document_tool"))
             + judge_runtime_capabilities(body.get("runtime_capabilities")))
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
