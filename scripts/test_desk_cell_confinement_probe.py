#!/usr/bin/env python3
"""Behavioural test for scripts/desk_cell_confinement_probe.py --leg oom.

It runs the real script against a local fake guest that answers the
isolation self-test with a canned receipt. Run:
python3 scripts/test_desk_cell_confinement_probe.py
"""
import copy
import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
PROBE = os.path.join(HERE, "desk_cell_confinement_probe.py")

spec = importlib.util.spec_from_file_location("probe", PROBE)
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)

DAEMON = {"pid": 812, "start_ticks": 4411, "boot_id": "2adf3e7d"}
GOOD_OOM = {
    "kind": "desk_oom_selftest", "mode": "required", "role": "research", "budget_bytes": 768 << 20,
    "mem_available_bytes": 5 << 30, "required_available_bytes": (768 << 20) + (1 << 30),
    "slice": {"path": "/sys/fs/cgroup/choir-desks", "memory_max": "4294967296", "memory_high": "3221225472"},
    "worker": {
        "pid": 900, "cell_sent": True, "cell_started": True, "result": "transport: EOF", "dead": True, "oom_kill": 1,
        "exit_cause": "exited on its own (signal: killed); killed by its memory budget (memory.max 768 MiB, oom_kill 1)",
        "cgroup": {"path": "/sys/fs/cgroup/choir-desks/w-1", "memory_max": str(768 << 20), "memory_high": "max",
                   "memory_events": {"oom": 1, "oom_kill": 1}},
    },
    "sibling": {"role": "research", "pid": 899, "before_answered": True, "after_answered": True,
                "output_before": "cell 42", "output_after": "cell 42"},
    "daemon_before": dict(DAEMON), "daemon_after": dict(DAEMON), "ok": True,
}


class JudgeOOM(unittest.TestCase):
    def test_good(self):
        self.assertEqual(probe.judge_oom(copy.deepcopy(GOOD_OOM)), [])

    def test_each_shortfall(self):
        cases = {
            "missing receipt": lambda r: r.clear(),
            "no oom_kill": lambda r: (r["worker"].update(oom_kill=0), r["worker"]["cgroup"]["memory_events"].update(oom_kill=0)),
            "events say no kill": lambda r: r["worker"]["cgroup"]["memory_events"].update(oom_kill=0),
            "host killed it": lambda r: r["worker"].update(exit_cause="killed by the host (signal: killed)"),
            "survived": lambda r: r["worker"].update(dead=False),
            "returned": lambda r: r["worker"].update(result="returned"),
            "not sent": lambda r: r["worker"].update(cell_sent=False),
            "budget loosened": lambda r: r["worker"]["cgroup"].update(memory_max=str(1 << 30)),
            "unbounded worker": lambda r: r["worker"]["cgroup"].update(memory_max="max"),
            "unbounded slice": lambda r: r["slice"].update(memory_max="max"),
            "other role": lambda r: r.update(role="management"),
            "daemon restarted": lambda r: r["daemon_after"].update(start_ticks=9999),
            "daemon pid": lambda r: r["daemon_after"].update(pid=1),
            "guest rebooted": lambda r: r["daemon_after"].update(boot_id="x"),
            "daemon unread": lambda r: r["daemon_before"].update(error="no /proc"),
            "sibling silent": lambda r: r["sibling"].update(after_answered=False),
            "worker refused": lambda r: r["worker"].update(error="refused: no desks cgroup"),
            # Review bunch 22: a kill before the cell ran, a non-production
            # mode, and a guest short of memory are not proofs.
            "cell never started": lambda r: r["worker"].update(cell_started=False),
            "mode auto": lambda r: r.update(mode="auto"),
            "memory short": lambda r: r.update(mem_available_bytes=r["required_available_bytes"] - 1),
            "memory unread": lambda r: r.update(mem_available_bytes=0),
            "headroom shrunk": lambda r: r.update(required_available_bytes=768 << 20),
        }
        for name, mutate in cases.items():
            r = copy.deepcopy(GOOD_OOM)
            mutate(r)
            self.assertTrue(probe.judge_oom(r), name)


GOOD_SECCOMP = {
    "kind": "capsule_seccomp_selftest", "kernel": "6.1.155", "ok": True,
    "filters": [{
        "name": n, "bpf_sha256": "h%d" % i, "expected_sha256": "h%d" % i, "loaded": True, "seccomp": "2",
        "probes": {"clone3": "ENOSYS: function not implemented", "clone_newuser": "EPERM: operation not permitted",
                   "unshare_newuser": "EPERM: operation not permitted", "fork_exec": "", "os_exec": ""},
    } for i, n in enumerate(probe.SECCOMP_FILTERS)],
}


class JudgeSeccomp(unittest.TestCase):
    def test_good(self):
        self.assertEqual(probe.judge_seccomp(copy.deepcopy(GOOD_SECCOMP)), [])

    def test_each_shortfall(self):
        cases = {
            "missing receipt": lambda r: r.clear(),
            "no kernel": lambda r: r.update(kernel=""),
            "filter missing": lambda r: r["filters"].pop(),
            "not loaded": lambda r: r["filters"][0].update(loaded=False),
            "other program": lambda r: r["filters"][0].update(bpf_sha256="x"),
            "same program twice": lambda r: (r["filters"][1].update(bpf_sha256="h0", expected_sha256="h0")),
            "seccomp off": lambda r: r["filters"][0].update(seccomp="0"),
            "clone3 reached the kernel": lambda r: r["filters"][2]["probes"].update(clone3="EINVAL: invalid argument"),
            "new user namespace": lambda r: r["filters"][3]["probes"].update(clone_newuser=""),
            "unshare reached": lambda r: r["filters"][3]["probes"].update(unshare_newuser="EINVAL: invalid argument"),
            "fork broken": lambda r: r["filters"][4]["probes"].update(fork_exec="EPERM: operation not permitted"),
            "os/exec broken": lambda r: r["filters"][5]["probes"].update(os_exec="ENOSYS: function not implemented"),
            "probe missing": lambda r: r["filters"][5]["probes"].pop("os_exec"),
        }
        for name, mutate in cases.items():
            r = copy.deepcopy(GOOD_SECCOMP)
            mutate(r)
            self.assertTrue(probe.judge_seccomp(r), name)


FAKE_SSH = """#!/bin/sh
# Fake ssh: logs its argv, then prints $FAKE_SSH_BODY_FILE and a status line,
# or fails with $FAKE_SSH_RC.
printf '%s\\n' "$*" >>"$FAKE_SSH_LOG"
if [ -n "$FAKE_SSH_RC" ]; then echo "ssh: connect failed" >&2; exit "$FAKE_SSH_RC"; fi
cat "$FAKE_SSH_BODY_FILE"
printf '\\nhttp_status=%s\\n' "$FAKE_SSH_STATUS"
"""

TOKEN_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
GUEST = "http://10.9.7.2:8080"


class ProbeRun(unittest.TestCase):
    """The OOM and seccomp legs are host-internal: the probe reaches the guest
    from Node B over ssh. It sends the guest no host credential: the
    internal service token never goes to a guest, and the route needs no
    marker (docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md).
    No API key is involved."""

    @classmethod
    def setUpClass(cls):
        cls.dir = tempfile.mkdtemp()
        bindir = os.path.join(cls.dir, "bin")
        os.mkdir(bindir)
        with open(os.path.join(bindir, "ssh"), "w") as f:
            f.write(FAKE_SSH)
        os.chmod(os.path.join(bindir, "ssh"), 0o755)
        cls.path = bindir + os.pathsep + os.environ.get("PATH", "")

    def run_probe(self, status, body, leg="oom", rc="", guest=GUEST):
        log = os.path.join(self.dir, "ssh.log")
        if os.path.exists(log):
            os.remove(log)
        bodyf = os.path.join(self.dir, "body.json")
        with open(bodyf, "w") as f:
            json.dump(body, f)
        out = os.path.join(self.dir, "receipt.json")
        if os.path.exists(out):
            os.remove(out)
        env = dict(os.environ, PATH=self.path, FAKE_SSH_LOG=log, FAKE_SSH_BODY_FILE=bodyf,
                   FAKE_SSH_STATUS=str(status), FAKE_SSH_RC=rc)
        p = subprocess.run([sys.executable, "-I", PROBE, "--out", out, "--leg", leg, "--ssh", "node-b",
                            "--guest-url", guest, "--computer", "computer-test"],
                           capture_output=True, text=True, timeout=60, env=env)
        calls = open(log).read().splitlines() if os.path.exists(log) else []
        rec = json.load(open(out)) if os.path.exists(out) else {}
        return p.returncode, rec, calls

    def assert_internal_call(self, call, leg):
        self.assertIn("node-b", call)
        self.assertIn(GUEST + "/internal/runtime/isolation-selftest?leg=" + leg, call)
        self.assertNotIn(TOKEN_FILE, call)
        self.assertNotIn("internal-service-token", call)
        self.assertNotIn("X-Internal-Caller", call)
        self.assertNotIn("$AUTH", call)
        self.assertNotIn("/api/", call)

    def test_proven(self):
        code, rec, calls = self.run_probe(200, {"leg": "oom", "ok": True, "oom": GOOD_OOM})
        self.assertEqual(code, 0, rec.get("failures"))
        self.assertEqual(len(calls), 1)
        self.assert_internal_call(calls[0], "oom")

    def test_seccomp_proven(self):
        code, rec, calls = self.run_probe(200, {"leg": "seccomp", "ok": True, "seccomp": GOOD_SECCOMP}, leg="seccomp")
        self.assertEqual(code, 0, rec.get("failures"))
        self.assert_internal_call(calls[0], "seccomp")

    def test_seccomp_old_guest(self):
        code, rec, _ = self.run_probe(404, {"error": "not found"}, leg="seccomp")
        self.assertEqual(code, 2)

    def test_daemon_changed_fails(self):
        bad = copy.deepcopy(GOOD_OOM)
        bad["daemon_after"]["pid"] = 813
        code, rec, _ = self.run_probe(200, {"leg": "oom", "ok": True, "oom": bad})
        self.assertEqual(code, 1)

    def test_old_guest_without_the_route(self):
        code, rec, _ = self.run_probe(404, {"error": "not found"})
        self.assertEqual(code, 2)

    def test_no_answer_is_a_failure_and_never_retried(self):
        code, rec, calls = self.run_probe(502, {})
        self.assertEqual(code, 1)
        self.assertEqual(len(calls), 1, "the OOM leg was retried")
        self.assertIn("HTTP 502", rec["failures"][0])

    def test_a_conflict_is_named_not_called_a_dead_daemon(self):
        # Review bunch 27 E2: HTTP 409 means another self-test holds the lock
        # and this call ran nothing. It still fails (exit 1) and is never
        # retried, but it must not claim the daemon may have died.
        code, rec, calls = self.run_probe(409, {"error": "an isolation self-test is already running"})
        self.assertEqual(code, 1)
        self.assertEqual(len(calls), 1, "the OOM leg was retried")
        msg = rec["failures"][0]
        self.assertIn("HTTP 409", msg)
        self.assertIn("already running", msg)
        self.assertNotIn("may have died", msg)

    def test_ssh_failure_during_the_oom_leg_is_a_failure(self):
        code, rec, calls = self.run_probe(200, {}, rc="255")
        self.assertEqual(code, 1)
        self.assertEqual(len(calls), 1)

    def test_refuses_a_guest_url_that_is_not_an_address(self):
        code, rec, calls = self.run_probe(200, {"leg": "oom", "ok": True, "oom": GOOD_OOM}, guest="http://x';rm -rf /;'")
        self.assertEqual(code, 2)
        self.assertEqual(calls, [])


if __name__ == "__main__":
    unittest.main()
