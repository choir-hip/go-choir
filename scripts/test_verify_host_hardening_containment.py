#!/usr/bin/env python3
"""Test of the [containment] checks in scripts/verify-host-hardening.py.

    python3 -I scripts/test_verify_host_hardening_containment.py

Feeds check_containment fake systemctl and nsenter answers and runs the
legacy-path check against a temporary tree; needs no root.
docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md.

Failure modes pinned here:
  - the env files are checked one by one, so a file renamed or created
    after a unit started (the deploy.env case) is the only thing that can
    show, and the directory that must hide them is never checked;
  - a unit that sees the env directory, or a file in it, passes;
  - caddy, go-choir-goproxy or searx (problem 2), or the
    guest-reachable go-choir-pkgmirror and unbound, are exempt from the
    socket checks or left out of the units checked;
  - a regular file left at an old env path (nothing reads it; a unit can
    see it) passes, or the compatibility link into the directory fails;
  - the modes check still names the old paths, or not the directory;
  - (round 2) a file the activation quarantined into the directory
    (<name>.stray.<epoch>) does not show in the proof.
"""

import importlib.util
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("vhh", os.path.join(HERE, "verify-host-hardening.py"))
vhh = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vhh)

STATE = "/var/lib/go-choir"
UNIT_ENV_DIR = STATE + "/unit-env"
UNIT_ENV_NAMES = ("deploy.env", "corpus-dsn.env", "corpusd-retrieval.env", "sandbox-gateway-token.env")
SOCKETS = ("/run/systemd/private", "/run/dbus/system_bus_socket")
PROBLEM_2_UNITS = ("caddy", "go-choir-goproxy", "searx")
GUEST_REACHABLE_UNITS = ("go-choir-pkgmirror", "unbound")


def run_containment(seen=None):
    """seen maps (unit, path) to the mode a unit sees; everything else is absent."""
    seen = seen or {}
    vhh.results.clear()
    pids = {u: str(100 + i) for i, u in enumerate(vhh.UNITS)}
    units_by_pid = {p: u for u, p in pids.items()}
    vhh.systemctl_show = lambda unit, props: {"MainPID": pids.get(unit, "0")}
    vhh.mode_in_namespace = lambda pid, path: seen.get((units_by_pid.get(str(pid)), path), "absent")
    vhh.writable_in_namespace = lambda pid, path: False
    vhh.check_containment()
    return {r["check"]: r for r in vhh.results if r["section"] == "containment"}


class Layout(unittest.TestCase):
    def test_env_files_live_in_one_hidden_directory(self):
        self.assertEqual(vhh.UNIT_ENV_DIR, UNIT_ENV_DIR)
        self.assertEqual(sorted(vhh.ENV_FILES), sorted(f"{UNIT_ENV_DIR}/{n}" for n in UNIT_ENV_NAMES))
        self.assertEqual(sorted(vhh.LEGACY_ENV_FILES), sorted(f"{STATE}/{n}" for n in UNIT_ENV_NAMES))

    def test_every_exposed_unit_is_checked(self):
        for unit in PROBLEM_2_UNITS + GUEST_REACHABLE_UNITS:
            self.assertIn(unit, vhh.UNITS)
        self.assertIn("go-choir-pkgmirror", vhh.NO_CAP_UNITS)

    def test_only_systemctl_runners_keep_the_sockets(self):
        self.assertEqual(vhh.SYSTEMD_SOCKET_UNITS, {"go-choir-vmctl"})


class Containment(unittest.TestCase):
    def test_all_hidden_passes_and_every_path_is_checked(self):
        got = run_containment()
        failed = [k for k, r in got.items() if not r["ok"]]
        self.assertEqual(failed, [])
        for unit in vhh.UNITS:
            for path in [UNIT_ENV_DIR] + [f"{UNIT_ENV_DIR}/{n}" for n in UNIT_ENV_NAMES]:
                name = f"{unit} cannot reach {path}"
                self.assertTrue(name in got, name)
            for sock in SOCKETS:
                name = f"{unit} cannot reach {sock}"
                self.assertEqual(name in got, unit not in vhh.SYSTEMD_SOCKET_UNITS, name)

    def test_a_hidden_directory_reads_as_mode_zero(self):
        # systemd mounts an empty mode-000 directory over a hidden directory.
        got = run_containment({(u, UNIT_ENV_DIR): "0" for u in vhh.UNITS})
        self.assertTrue(all(r["ok"] for r in got.values()))

    def test_visible_env_directory_fails(self):
        for unit in PROBLEM_2_UNITS + ("go-choir-proxy",):
            got = run_containment({(unit, UNIT_ENV_DIR): "700"})
            self.assertFalse(got[f"{unit} cannot reach {UNIT_ENV_DIR}"]["ok"], unit)

    def test_visible_env_file_fails(self):
        # The deploy.env case: a file the unit can stat inside its namespace.
        got = run_containment({("go-choir-proxy", f"{UNIT_ENV_DIR}/deploy.env"): "600"})
        self.assertFalse(got[f"go-choir-proxy cannot reach {UNIT_ENV_DIR}/deploy.env"]["ok"])

    def test_problem_2_units_have_no_socket_exemption(self):
        for unit in PROBLEM_2_UNITS + GUEST_REACHABLE_UNITS:
            got = run_containment({(unit, "/run/systemd/private"): "777"})
            self.assertFalse(got[f"{unit} cannot reach /run/systemd/private"]["ok"], unit)


class LegacyPaths(unittest.TestCase):
    def run_legacy(self, make):
        with tempfile.TemporaryDirectory() as root:
            state = os.path.join(root, STATE.lstrip("/"))
            os.makedirs(os.path.join(state, "unit-env"))
            make(state)
            vhh.results.clear()
            vhh.check_legacy_env_paths(root)
            return {r["check"]: r for r in vhh.results if r["section"] == "containment"}

    def test_absent_and_compat_link_pass(self):
        def make(state):
            open(os.path.join(state, "unit-env", "deploy.env"), "w").close()
            os.symlink("unit-env/deploy.env", os.path.join(state, "deploy.env"))
        got = self.run_legacy(make)
        self.assertEqual(len(got), len(UNIT_ENV_NAMES))
        self.assertTrue(all(r["ok"] for r in got.values()), got)

    def test_regular_file_at_old_path_fails(self):
        def make(state):
            with open(os.path.join(state, "corpus-dsn.env"), "w") as fh:
                fh.write("X=1\n")
        got = self.run_legacy(make)
        bad = [k for k, r in got.items() if not r["ok"]]
        self.assertEqual(len(bad), 1, got)
        self.assertIn("corpus-dsn.env", bad[0])

    def test_link_elsewhere_fails(self):
        def make(state):
            os.symlink("/tmp/elsewhere.env", os.path.join(state, "deploy.env"))
        got = self.run_legacy(make)
        bad = [k for k, r in got.items() if not r["ok"]]
        self.assertEqual(len(bad), 1, got)
        self.assertIn("deploy.env", bad[0])


class Strays(unittest.TestCase):
    """Round 2 (F1, F2): a file the activation quarantined waits in the hidden
    directory for the operator; the proof must show it."""

    def run_strays(self, names):
        with tempfile.TemporaryDirectory() as root:
            d = os.path.join(root, UNIT_ENV_DIR.lstrip("/"))
            os.makedirs(d)
            for n in names:
                open(os.path.join(d, n), "w").close()
            vhh.results.clear()
            vhh.check_unit_env_strays(root)
            return [r for r in vhh.results if r["section"] == "containment"]

    def test_no_stray_passes(self):
        got = self.run_strays(["deploy.env", ".imported"])
        self.assertEqual(len(got), 1)
        self.assertTrue(got[0]["ok"], got)

    def test_a_stray_fails_and_is_named(self):
        got = self.run_strays(["deploy.env", "deploy.env.stray.1761140000"])
        self.assertEqual(len(got), 1)
        self.assertFalse(got[0]["ok"], got)
        self.assertIn("deploy.env.stray.1761140000", got[0]["detail"])


class Modes(unittest.TestCase):
    def test_modes_name_the_directory_and_not_the_old_paths(self):
        paths = {p: (want, required) for p, want, required in vhh.MODE_EXPECT}
        self.assertEqual(paths[UNIT_ENV_DIR], (0o700, True))
        for name in ("corpus-dsn.env", "sandbox-gateway-token.env"):
            self.assertEqual(paths[f"{UNIT_ENV_DIR}/{name}"], (0o600, True), name)
        # The proxy's EnvironmentFile is optional; so is the check (DS5).
        self.assertEqual(paths[f"{UNIT_ENV_DIR}/deploy.env"], (0o600, False))
        # The first import has completed (round 2b, DS2).
        self.assertEqual(paths[f"{UNIT_ENV_DIR}/.imported"], (0o600, True))
        # The kill switch is absent unless an operator writes it.
        self.assertEqual(paths[f"{UNIT_ENV_DIR}/corpusd-retrieval.env"], (0o600, False))
        for name in UNIT_ENV_NAMES:
            self.assertNotIn(f"{STATE}/{name}", paths)


if __name__ == "__main__":
    unittest.main()
