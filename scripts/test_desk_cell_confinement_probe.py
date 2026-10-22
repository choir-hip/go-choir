#!/usr/bin/env python3
"""Behavioural test for scripts/desk_cell_confinement_probe.py --leg oom.

It runs the real script against a local fake guest that answers the
isolation self-test with a canned receipt. Run:
python3 scripts/test_desk_cell_confinement_probe.py
"""
import copy
import http.server
import importlib.util
import json
import os
import subprocess
import sys
import tempfile
import threading
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
PROBE = os.path.join(HERE, "desk_cell_confinement_probe.py")

spec = importlib.util.spec_from_file_location("probe", PROBE)
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)

DAEMON = {"pid": 812, "start_ticks": 4411, "boot_id": "2adf3e7d"}
GOOD_OOM = {
    "kind": "desk_oom_selftest", "role": "research", "budget_bytes": 768 << 20,
    "slice": {"path": "/sys/fs/cgroup/choir-desks", "memory_max": "4294967296", "memory_high": "3221225472"},
    "worker": {
        "pid": 900, "cell_sent": True, "result": "transport: EOF", "dead": True, "oom_kill": 1,
        "exit_cause": "exited on its own (signal: killed); killed by its memory budget (memory.max 768 MiB, oom_kill 1)",
        "cgroup": {"path": "/sys/fs/cgroup/choir-desks/w-1", "memory_max": str(768 << 20), "memory_high": "max",
                   "memory_events": {"oom": 1, "oom_kill": 1}},
    },
    "sibling": {"role": "research", "pid": 899, "before_answered": True, "after_answered": True},
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


class FakeGuest(http.server.BaseHTTPRequestHandler):
    status = 200
    body = {}
    paths = []

    def do_GET(self):
        self._send(200, {"build": {"commit": "abc"}})

    def do_POST(self):
        FakeGuest.paths.append(self.path)
        self._send(FakeGuest.status, FakeGuest.body)

    def _send(self, code, body):
        raw = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, *a):
        pass


class ProbeRun(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.srv = http.server.HTTPServer(("127.0.0.1", 0), FakeGuest)
        threading.Thread(target=cls.srv.serve_forever, daemon=True).start()
        cls.dir = tempfile.mkdtemp()
        cls.cred = os.path.join(cls.dir, "cred.json")
        with open(cls.cred, "w") as f:
            json.dump({"api_key": "k-never-printed", "computer_id": "computer-test"}, f)

    @classmethod
    def tearDownClass(cls):
        cls.srv.shutdown()

    def run_probe(self, status, body, leg="oom"):
        FakeGuest.status, FakeGuest.body, FakeGuest.paths = status, body, []
        out = os.path.join(self.dir, "receipt.json")
        p = subprocess.run([sys.executable, "-I", PROBE, "--cred", self.cred, "--out", out, "--leg", leg,
                            "--host", "http://127.0.0.1:%d" % self.srv.server_address[1]],
                           capture_output=True, text=True, timeout=60)
        self.assertNotIn("k-never-printed", p.stdout + p.stderr)
        return p.returncode, json.load(open(out)), list(FakeGuest.paths)

    def test_proven(self):
        code, rec, paths = self.run_probe(200, {"leg": "oom", "ok": True, "oom": GOOD_OOM})
        self.assertEqual(code, 0, rec.get("failures"))
        self.assertEqual(paths, ["/api/runtime/isolation-selftest?leg=oom"])

    def test_seccomp_proven(self):
        code, rec, paths = self.run_probe(200, {"leg": "seccomp", "ok": True, "seccomp": GOOD_SECCOMP}, leg="seccomp")
        self.assertEqual(code, 0, rec.get("failures"))
        self.assertEqual(paths, ["/api/runtime/isolation-selftest?leg=seccomp"])

    def test_seccomp_old_guest(self):
        code, rec, _ = self.run_probe(200, {"ok": True, "desk_cells": {}}, leg="seccomp")
        self.assertEqual(code, 2)

    def test_daemon_changed_fails(self):
        bad = copy.deepcopy(GOOD_OOM)
        bad["daemon_after"]["pid"] = 813
        code, rec, _ = self.run_probe(200, {"leg": "oom", "ok": True, "oom": bad})
        self.assertEqual(code, 1)

    def test_old_guest_ran_default_selftest(self):
        code, rec, _ = self.run_probe(200, {"ok": True, "desk_cells": {}})
        self.assertEqual(code, 2)

    def test_no_answer_is_a_failure_and_never_retried(self):
        code, rec, paths = self.run_probe(502, {})
        self.assertEqual(code, 1)
        self.assertEqual(len(paths), 1, "the OOM leg was retried")
        self.assertIn("HTTP 502", rec["failures"][0])


if __name__ == "__main__":
    unittest.main()
