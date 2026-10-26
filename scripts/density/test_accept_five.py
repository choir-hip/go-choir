#!/usr/bin/env python3
"""Canned-JSON checks for accept_five.py (the D7 acceptance run): the guard and
protected set, the stops, the admission-decision reading, the victim and wake
rules, the plan output and the receipt summary. Run:
python3 -I scripts/density/test_accept_five.py (no host, no network, no CLI)."""
import io
import json
import os
import sys
import unittest
from contextlib import redirect_stdout

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import accept_five as af  # noqa: E402
import lib  # noqa: E402

PAPER = "computer-paper"
LISTING = {"ownerships": [
    {"computer_id": "computer-m1", "vm_id": "vm-m1", "state": "hibernated", "warmness_class": "test"},
    {"computer_id": "computer-m2", "vm_id": "vm-m2", "state": "active", "warmness_class": "test"},
    {"computer_id": "computer-other", "vm_id": "vm-other", "state": "hibernated", "warmness_class": "test"},
    {"computer_id": PAPER, "vm_id": "vm-paper", "state": "active", "warmness_class": "critical_protected"},
    {"computer_id": af.OWNER_COMPUTER_ID, "vm_id": "vm-owner", "state": "active", "warmness_class": "test"},
    {"computer_id": "computer-prim", "vm_id": "vm-prim", "state": "hibernated", "warmness_class": "primary"},
]}


def snap(avail_gib=10.0, psi=0.0, listing=LISTING):
    return {"at": "2026-10-26T12:00:00Z", "meminfo_kib": {"MemAvailable": int(avail_gib * lib.GIB / 1024)},
            "psi_memory": {"some": {"avg10": psi}}, "vmctl_list": listing}


HEALTH = {"status": "ok", "active_vms": 3, "by_state": {"active": 3, "hibernated": 2},
          "admission": {"mode": "measured", "enabled": True, "make_room": "active", "need_mode": "peak",
                        "default_need_bytes": 2 << 30, "headroom_bytes": 1 << 30, "reserved_bytes": 0,
                        "recent": [
                            {"at": "2026-10-26T11:00:00Z", "decision": "admit", "mode": "measured", "vm_id": "vm-old"},
                            {"at": "2026-10-26T12:00:05Z", "decision": "admit", "mode": "measured", "vm_id": "vm-m1", "class": "test"},
                            {"at": "2026-10-26T12:01:00Z", "decision": "admit_after_make_room", "mode": "measured", "vm_id": "vm-m2",
                             "class": "test", "made_room_vm_ids": ["vm-m1"]},
                            {"at": "2026-10-26T12:01:30Z", "decision": "refuse", "mode": "memory_grow", "vm_id": "vm-m2"},
                        ]}}


class Guard(unittest.TestCase):
    def test_protected_set_has_both_owner_computers_and_extras(self):
        ids = af.protected_ids(["computer-extra"])
        self.assertIn(lib.DELEGATE_COMPUTER_ID, ids)
        self.assertIn(af.OWNER_COMPUTER_ID, ids)
        self.assertIn("computer-extra", ids)
        self.assertTrue(af.OWNER_COMPUTER_ID.startswith("computer-03335285"))

    def test_owner_refused_even_when_classed_test(self):
        own = next(o for o in LISTING["ownerships"] if o["computer_id"] == af.OWNER_COMPUTER_ID)
        self.assertEqual(lib.refusal(own), "")  # lib alone would pass it
        self.assertTrue(lib.refusal(own, af.protected_ids()))

    def test_require_minted_safe(self):
        af.require_minted_safe({"computer_id": "computer-m1", "warmness_class": "test"}, af.protected_ids())
        for bad in ({"computer_id": PAPER, "warmness_class": "critical_protected"},
                    {"computer_id": af.OWNER_COMPUTER_ID, "warmness_class": "test"}, {}):
            with self.assertRaises(af.Stop):
                af.require_minted_safe(bad, af.protected_ids())


class Stops(unittest.TestCase):
    def test_psi_only_stop_at_5(self):
        self.assertEqual(af.psi_stop(snap(psi=4.9)), "")
        self.assertIn("PSI", af.psi_stop(snap(psi=5.1)))

    def test_no_memavailable_floor_stop(self):
        # Unlike density_bench, the floor does not end the run: admission is the thing under test.
        self.assertEqual(af.psi_stop(snap(avail_gib=0.5)), "")

    def test_protected_leaving_active(self):
        active = af.protected_active(LISTING, af.protected_ids())
        self.assertEqual(active, {PAPER, af.OWNER_COMPUTER_ID})
        after = json.loads(json.dumps(LISTING))
        next(o for o in after["ownerships"] if o["computer_id"] == PAPER)["state"] = "hibernated"
        self.assertEqual(af.left_active(active, after), {PAPER})
        self.assertEqual(af.left_active(active, LISTING), set())

    def test_test_class_hibernating_is_not_a_stop(self):
        active = af.protected_active(LISTING, af.protected_ids())
        self.assertNotIn("computer-m2", active)


class Decisions(unittest.TestCase):
    def test_parse_ts(self):
        self.assertEqual(af.parse_ts("1970-01-01T00:01:00Z"), 60)
        self.assertIsNone(af.parse_ts("garbage"))

    def test_fresh_only_newer_than_run_start_and_dedup(self):
        seen = set()
        since = af.parse_ts("2026-10-26T12:00:00Z")
        got = af.fresh_decisions(HEALTH, since, seen)
        self.assertEqual([d["vm_id"] for d in got], ["vm-m1", "vm-m2", "vm-m2"])
        self.assertEqual(af.fresh_decisions(HEALTH, since, seen), [])

    def test_start_decisions_exclude_memory_grow(self):
        ds = af.fresh_decisions(HEALTH, 0, set())
        starts = af.start_decisions(ds, "vm-m2")
        self.assertEqual([d["decision"] for d in starts], ["admit_after_make_room"])
        self.assertEqual(af.refusals(ds, "vm-m2"), [])  # the grow refusal is not a refused start

    def test_refusals_count_refuse_no_room(self):
        ds = [{"at": "2026-10-26T12:00:00Z", "decision": "refuse_no_room", "mode": "measured", "vm_id": "vm-x", "reason": "no idle"}]
        self.assertEqual(len(af.refusals(ds, "vm-x")), 1)
        self.assertEqual(len(af.refusals(ds, "vm-y")), 0)

    def test_made_room_ids(self):
        ds = af.fresh_decisions(HEALTH, 0, set())
        self.assertEqual(af.made_room_ids(ds), ["vm-m1"])


class Victims(unittest.TestCase):
    def test_victim_view_classifies(self):
        v = af.victim_view("vm-m1", LISTING, af.protected_ids(), minted_computers={"computer-m1"})
        self.assertEqual((v["computer_id"], v["warmness_class"], v["minted"], v["safe"]), ("computer-m1", "test", True, True))
        v = af.victim_view("vm-paper", LISTING, af.protected_ids(), minted_computers=set())
        self.assertFalse(v["safe"])
        self.assertTrue(v["refusal"])
        v = af.victim_view("vm-gone", LISTING, af.protected_ids(), minted_computers=set())
        self.assertFalse(v["safe"])

    def test_wake_candidates_only_hibernated_safe_minted(self):
        victims = [af.victim_view(i, LISTING, af.protected_ids(), {"computer-m1", "computer-m2"})
                   for i in ("vm-m1", "vm-m2", "vm-other", "vm-prim", "vm-owner")]
        cands, skipped = af.wake_candidates(victims, LISTING, af.protected_ids())
        self.assertEqual([c["computer_id"] for c in cands], ["computer-m1"])
        why = {s["vm_id"]: s["why"] for s in skipped}
        self.assertIn("not hibernated", why["vm-m2"])
        self.assertIn("not minted", why["vm-other"])
        self.assertIn("refused", why["vm-prim"])
        self.assertIn("refused", why["vm-owner"])


class Du(unittest.TestCase):
    def test_safe_vm_id(self):
        self.assertTrue(af.safe_vm_id("vm-1669020a"))
        for bad in ("", ".", "..", "../x", "a/b", "vm x", "-rf", "vm;rm"):
            self.assertFalse(af.safe_vm_id(bad), bad)

    def test_snapshot_dir_is_the_vm_dir_not_the_root(self):
        self.assertEqual(af.snapshot_dir("vm-a"), "/var/lib/go-choir/vm-state/vm-a/snapshot")
        with self.assertRaises(ValueError):
            af.snapshot_dir("../..")

    def test_parse_du(self):
        self.assertEqual(af.parse_du("123456\t/var/x\n"), 123456)
        self.assertIsNone(af.parse_du("du: cannot access\n"))
        self.assertIsNone(af.parse_du(""))


def receipt(**kw):
    r = {"kind": "density_accept_five", "run_id": "t", "n": 3, "stop": None,
         "minted": [
             {"index": 1, "computer_id": "computer-m1", "vm_id": "vm-m1", "active": True, "refusals": [], "made_room": []},
             {"index": 2, "computer_id": "computer-m2", "vm_id": "vm-m2", "active": True, "refusals": [], "made_room": [
                 {"vm_id": "vm-m1", "computer_id": "computer-m1", "warmness_class": "test", "snapshot": {"du_bytes": 5 << 30}}]},
             {"index": 3, "computer_id": "computer-m3", "vm_id": "vm-m3", "active": True, "refusals": [], "made_room": [
                 {"vm_id": "vm-m2", "computer_id": "computer-m2", "warmness_class": "test", "snapshot": {}}]},
         ],
         "wakes": [], "mem_series": [{"phase": "before", "i": 1, "mem_available_kib": 8 << 20},
                                     {"phase": "after", "i": 1, "mem_available_kib": 5 << 20}]}
    r.update(kw)
    return r


class Summary(unittest.TestCase):
    def test_pass_path(self):
        r = receipt(wakes=[{"computer_id": "computer-m1", "wake_ms": 400, "restore_kind": "resume"},
                           {"computer_id": "computer-m2", "wake_ms": 950, "restore_kind": "resume"}])
        c = af.evaluate(r)
        self.assertEqual((c["none_refused"], c["every_wake_under_1s"]), ("pass", "pass"))
        self.assertEqual(c["make_room_hibernations"], 2)
        self.assertEqual(c["all_started"], "pass")

    def test_refusal_fails(self):
        r = receipt()
        r["minted"][1]["refusals"] = [{"decision": "refuse_no_room"}]
        self.assertEqual(af.evaluate(r)["none_refused"], "fail")

    def test_unfinished_or_stopped_fails(self):
        r = receipt(stop="memory PSI some avg10 6 > 5")
        c = af.evaluate(r)
        self.assertEqual((c["all_started"], c["none_refused"]), ("fail", "fail"))
        r = receipt(n=5)
        self.assertEqual(af.evaluate(r)["all_started"], "fail")

    def test_slow_or_cold_wake_fails_and_still_reports_times(self):
        r = receipt(wakes=[{"computer_id": "a", "wake_ms": 1500, "restore_kind": "resume"}])
        self.assertEqual(af.evaluate(r)["every_wake_under_1s"], "fail")
        r = receipt(wakes=[{"computer_id": "a", "wake_ms": 300, "restore_kind": "cold"}])
        self.assertEqual(af.evaluate(r)["every_wake_under_1s"], "fail")
        self.assertEqual(af.wake_under_1s({"wake_ms": 300, "restore_kind": "fallback_cold"}), False)
        self.assertTrue(af.wake_under_1s({"wake_ms": 999, "restore_kind": "unknown"}))
        self.assertFalse(af.wake_under_1s({"wake_ms": 1000, "restore_kind": "resume"}))
        self.assertFalse(af.wake_under_1s({"wake_ms": None, "restore_kind": "resume"}))
        self.assertFalse(af.wake_under_1s({"wake_ms": 100, "restore_kind": "resume", "error": "boom"}))

    def test_no_wakes_is_not_run(self):
        self.assertEqual(af.evaluate(receipt())["every_wake_under_1s"], "not_run")

    def test_zero_make_room_is_flagged_vacuous(self):
        r = receipt()
        for m in r["minted"]:
            m["made_room"] = []
        c = af.evaluate(r)
        self.assertEqual(c["make_room_hibernations"], 0)
        self.assertTrue(c["vacuous"])

    def test_render_summary_has_the_three_answers_and_the_series(self):
        text = af.render_summary(receipt(wakes=[{"computer_id": "computer-m1", "wake_ms": 1500, "restore_kind": "resume"}]))
        for needle in ("none refused: pass", "make-room hibernations: 2", "every wake under 1 s: fail", "1500", "MemAvailable"):
            self.assertIn(needle, text)
        self.assertNotIn("api_key", text)


class PlanAndPreflight(unittest.TestCase):
    def args(self, *extra):
        return af.parse_args(list(extra))

    def test_defaults(self):
        a = self.args()
        self.assertEqual((a.n, a.run, a.wake, a.idle_s), (5, False, True, 60))
        self.assertEqual(self.args("--run", "--no-wake").wake, False)

    def test_preflight_ok(self):
        p = af.preflight(snap(), HEALTH, ["computer-x"], 5)
        self.assertEqual(p["blockers"], [])
        self.assertEqual(p["by_state"], {"active": 3, "hibernated": 2})
        self.assertEqual(p["admission"]["make_room"], "active")
        self.assertAlmostEqual(p["mem_available_gib"], 10.0, places=1)
        pids = {x["computer_id"] for x in p["protected_not_touched"]}
        self.assertTrue({PAPER, af.OWNER_COMPUTER_ID, "computer-prim"} <= pids)

    def test_preflight_blocks_when_admission_would_make_the_run_vacuous(self):
        h = json.loads(json.dumps(HEALTH))
        h["admission"]["make_room"] = "off"
        self.assertTrue(af.preflight(snap(), h, [], 5)["blockers"])
        h = json.loads(json.dumps(HEALTH))
        h["admission"]["enabled"] = False
        self.assertTrue(af.preflight(snap(), h, [], 5)["blockers"])
        # not exposed at all: a warning, not a blocker
        p = af.preflight(snap(), {"status": "ok"}, [], 5)
        self.assertEqual(p["blockers"], [])
        self.assertTrue(p["warnings"])

    def test_preflight_blocks_on_psi_and_missing_sample(self):
        self.assertTrue(af.preflight(snap(psi=9), HEALTH, [], 5)["blockers"])
        self.assertTrue(af.preflight({"error": "collector returned non-JSON"}, HEALTH, [], 5)["blockers"])

    def test_preflight_warns_when_host_can_hold_all(self):
        p = af.preflight(snap(avail_gib=60), HEALTH, [], 5)
        self.assertTrue(any("hold" in w for w in p["warnings"]))

    def test_plan_lists_steps_and_changes_nothing(self):
        a = self.args("--n", "5", "--run-id", "T1")
        plan = af.build_plan(a)
        text = json.dumps(plan)
        self.assertIn("d7-accept-t1-1", text)
        self.assertIn("d7-accept-t1-5", text)
        self.assertEqual(plan["will_change_anything"], False)
        self.assertEqual(plan["labels"], [f"d7-accept-t1-{i}" for i in range(1, 6)])
        self.assertNotIn("api_key", text)
        self.assertNotIn("stop", [s.split()[0] for s in plan["steps"]])

    def test_dry_run_prints_plan_and_preflight_from_canned_input(self):
        import tempfile
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "canned.json")
            with open(path, "w") as f:
                json.dump({"collect": snap(), "health": HEALTH}, f)
            buf = io.StringIO()
            with redirect_stdout(buf):
                rc = af.main(["--canned", path, "--run-id", "T2", "--out", d])
            self.assertEqual(rc, 0)
            out = json.loads(buf.getvalue())
            self.assertFalse(out["plan"]["will_change_anything"])
            self.assertEqual(out["preflight"]["blockers"], [])
            self.assertEqual(os.listdir(d), ["canned.json"])  # nothing written without --run

    def test_canned_refuses_run(self):
        with self.assertRaises(SystemExit):
            af.main(["--canned", "/nonexistent", "--run"])


class WakeHelpers(unittest.TestCase):
    T0 = 1_000_000.0

    def tl(self, kind="resume", started=None, outcome="snapshot"):
        ts = af.fmt_ts(started if started is not None else self.T0 + 1)
        return {"boot_kind": kind, "started_at": ts, "outcome": "healthy", "total_ms": 400,
                "snapshot": {"outcome": outcome, "load_ms": 120, "wake_to_healthy_ms": 380}}

    def test_restore_kind(self):
        self.assertEqual(af.restore_kind(self.tl(), self.T0), "resume")
        self.assertEqual(af.restore_kind(self.tl("cold"), self.T0), "cold")
        self.assertEqual(af.restore_kind(self.tl("resume", outcome="fallback_cold"), self.T0), "fallback_cold")
        self.assertEqual(af.restore_kind(None, self.T0), "unknown")
        self.assertEqual(af.restore_kind({"error": "no boot timeline"}, self.T0), "unknown")
        self.assertEqual(af.restore_kind(self.tl(started=self.T0 - 600), self.T0), "unknown")  # the timeline of an earlier boot

    def test_timeline_facts(self):
        f = af.timeline_facts(self.tl())
        self.assertEqual((f["boot_kind"], f["load_ms"], f["wake_to_healthy_ms"], f["total_ms"]), ("resume", 120, 380, 400))
        self.assertEqual(af.timeline_facts(None), {})

    def test_wake_argv_flags_before_positionals_and_no_secret(self):
        argv = af.wake_argv("/tmp/choir", "computer-m1", "d7-wake-t-1")
        self.assertEqual(argv[:3], ["/tmp/choir", "computer", "start"])
        self.assertEqual(argv[argv.index("--computer") + 1], "computer-m1")
        self.assertEqual(argv[argv.index("--idempotency-key") + 1], "d7-wake-t-1")
        self.assertFalse([a for a in argv if not a.startswith("-") and a not in ("computer", "start") and a not in
                          ("/tmp/choir", "computer-m1", "d7-wake-t-1", "120s")])
        self.assertFalse([a for a in argv if "choir_sk_" in a])

    def test_wake_env_carries_the_key_only_in_env(self):
        env = af.wake_env({"api_key": "choir_sk_secret", "base_url": "https://choir.news"}, {"PATH": "/bin"})
        self.assertEqual(env["CHOIR_API_KEY"], "choir_sk_secret")
        self.assertEqual(env["CHOIR_HOST"], "https://choir.news")
        with self.assertRaises(af.Stop):
            af.wake_env({"base_url": "https://choir.news"}, {})
        with self.assertRaises(af.Stop):
            af.wake_env({"api_key": "choir_sk_x", "base_url": "http://evil.example"}, {})

    def test_redact(self):
        self.assertEqual(af.redact("boom choir_sk_secret end", "choir_sk_secret"), "boom <redacted> end")
        self.assertEqual(af.redact("choir_sk_other1234", ""), "<redacted>")


if __name__ == "__main__":
    unittest.main()
