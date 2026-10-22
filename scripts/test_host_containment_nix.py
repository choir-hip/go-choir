#!/usr/bin/env python3
"""Host unit containment, checked on the evaluated Node B configuration.

    python3 -I scripts/test_host_containment_nix.py

Evaluates `.#nixosConfigurations.go-choir-b` (evaluation only; nothing is
built) and checks what every host unit hides against the verifier's own
lists (scripts/verify-host-hardening.py), so the deployed check and the
configuration cannot drift apart. It also runs the env-directory activation
snippet, taken from the evaluated configuration, against a temporary tree.
Skipped when nix is not installed. The flake reads tracked files only:
commit (or `git add`) a change before running this.
docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md.

Failure modes pinned here:
  N1 A host unit (caddy, go-choir-goproxy, qdrant, searx, searx-init,
     go-choir-pkgmirror, unbound, or any go-choir unit) hides less than the
     full set, beyond the keeps recorded below with their reasons.
  N2 A long-running go-choir unit, or one of the upstream host units, is
     missing from the verifier's UNITS, so the deployed check never sees it.
  N3 An env file read only through EnvironmentFile= lives outside the env
     directory, or a unit hides it as a file: a rename or a late create
     then reaches every running unit (the deploy.env case).
  N4 The env directory is not created 0700 root before units start.
  N5 The activation snippet does not move the files on its first run (by
     rename, keeping the inode), does not leave a link at each old path (a
     rollback past the move reads through it, and an old runbook's `>`
     writes through it), imports a file planted at an old path after its
     first run, or follows a planted link at the directory.
  N6 (round 2) It returns 0 while a regular file sits at an old path after
     the import (F1, F2): such a file must be quarantined into the hidden
     directory as <name>.stray.<epoch>, the old path relinked, and every
     activation must fail while a stray is there (the deploy retries a
     failed switch once). A file identical to the one in unit-env (a
     reverted deploy's scrub) is only relinked. Before the import the old
     path wins. A link to a file elsewhere is imported on the first run and
     quarantined after it (F3); links are replaced through a temporary name
     and never entered when they point at a directory (F4).
  N7 (round 2b) The env directory is hidden with "-" (fail-open when it is
     missing: DS1); a unit that reads an env file from it starts without
     AssertPathExists on the import marker, so a partial first import (an
     IO failure after some moves) starts it on a partial view (DS2); a keep
     the unit hides anyway goes unnoticed (DS7).
"""

import importlib.util
import json
import os
import shutil
import stat
import subprocess
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
spec = importlib.util.spec_from_file_location("vhh", os.path.join(HERE, "verify-host-hardening.py"))
vhh = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vhh)

STATE = "/var/lib/go-choir"
UNIT_ENV_DIR = STATE + "/unit-env"
UNIT_ENV_NAMES = ("deploy.env", "corpus-dsn.env", "corpusd-retrieval.env", "sandbox-gateway-token.env")
SOCKETS = ["-/run/systemd/private", "-/run/dbus/system_bus_socket"]
UPSTREAM_UNITS = ["caddy", "qdrant", "searx", "searx-init", "unbound"]
# What a unit may keep visible, and why. Everything else in the reference
# set (go-choir-proxy's, which keeps nothing) must be hidden.
KEEPS = {
    "go-choir-auth": (["-/var/lib/go-choir/auth-signing/ed25519-key"], "it signs JWTs"),
    "go-choir-maild": (["-/var/lib/go-choir/mail"], "its mail store"),
    "go-choir-corpusd": (["-/var/lib/go-choir/platform-artifacts/signing-key",
                          "-/var/lib/go-choir/platform-artifacts/key-escrow-key"], "platform receipts and escrow"),
    "go-choir-checkpointd": (["-/var/lib/go-choir/platform-artifacts/signing-key",
                              "-/var/lib/go-choir/platform-artifacts/key-escrow-key"], "platform receipts and escrow"),
    "go-choir-vmctl": (SOCKETS, "it runs systemctl (store compaction)"),
    "go-choir-dolt-liveness-probe": (SOCKETS, "it runs systemctl (restarts a wedged Dolt)"),
    # Its JWT key entry is replaced by the whole auth-signing directory,
    # which covers it (round 2b, DS7).
    "go-choir-gateway": (["-/var/lib/go-choir/codex-auth.json", "-/var/lib/go-choir/gateway-identities.json"],
                         "its provider auth and identity store"),
    # FP0's ruling: searx's own secret stays visible to it. Hiding it would
    # gain nothing: the process holds SEARXNG_SECRET in its environment and
    # in /run/searx/settings.yml.
    "searx": (["-/var/lib/go-choir/searxng.env"], "its own secret (FP0)"),
    "searx-init": (["-/var/lib/go-choir/searxng.env"], "its own secret (FP0)"),
}

EXPR = r'''
c: let
  names = builtins.fromJSON ''%s'';
  ss = c.systemd.services;
  choir = builtins.filter (n: builtins.substring 0 9 n == "go-choir-") (builtins.attrNames ss);
  sel = builtins.filter (n: builtins.hasAttr n ss) (names ++ choir);
  asList = v: if v == null then [ ] else if builtins.isList v then v else [ v ];
  acts = c.system.activationScripts;
  text = a: if builtins.isString a then a else a.text;
in {
  units = builtins.listToAttrs (map (n: let u = ss.${n}; s = u.serviceConfig; in {
    name = n;
    value = {
      enable = u.enable;
      hidden = asList (s.InaccessiblePaths or null);
      envFiles = asList (s.EnvironmentFile or null);
      type = s.Type or "simple";
      asserts = asList (u.unitConfig.AssertPathExists or null);
    };
  }) sel);
  tmpfiles = c.systemd.tmpfiles.rules;
  activation = if acts ? go-choir-unit-env then text acts.go-choir-unit-env else null;
}
'''

_CONFIG = None


def covered(path, hidden):
    """Whether a unit hiding `hidden` cannot reach `path`: the entry itself,
    or a hidden parent directory (with or without the "-" prefix)."""
    bare = path.lstrip("-")
    for h in hidden:
        hb = h.lstrip("-")
        if hb == bare or bare.startswith(hb.rstrip("/") + "/"):
            return True
    return False


def config():
    global _CONFIG
    if _CONFIG is None:
        expr = EXPR % json.dumps(UPSTREAM_UNITS)
        proc = subprocess.run(
            ["nix", "eval", "--json", REPO + "#nixosConfigurations.go-choir-b.config", "--apply", expr],
            capture_output=True, text=True, cwd=REPO, check=False)
        if proc.returncode != 0:
            raise AssertionError("nix eval failed:\n" + proc.stderr[-3000:])
        _CONFIG = json.loads(proc.stdout)
    return _CONFIG


def covered_units(units):
    """The units whose hiding is checked: the verifier's, the upstream host
    units, and every go-choir unit that is long-running or hides anything."""
    names = set(vhh.UNITS) | set(UPSTREAM_UNITS)
    for name, u in units.items():
        if name.startswith("go-choir-") and (u["type"] != "oneshot" or u["hidden"]):
            names.add(name)
    return sorted(n for n in names if n in units and units[n]["enable"])


@unittest.skipUnless(shutil.which("nix"), "nix is not installed")
class HostUnits(unittest.TestCase):
    def setUp(self):
        self.units = config()["units"]
        self.reference = set(self.units["go-choir-proxy"]["hidden"])

    def test_reference_hides_sockets_and_the_env_directory(self):
        for path in SOCKETS + [UNIT_ENV_DIR, "-/var/lib/go-choir/secrets"]:
            self.assertIn(path, self.reference)

    def test_DS1_the_env_directory_is_hidden_fail_closed(self):
        # Without "-", a unit whose directory is missing refuses to start
        # instead of running with it unmasked once it appears.
        for name in covered_units(self.units):
            hidden = self.units[name]["hidden"]
            self.assertIn(UNIT_ENV_DIR, hidden, name)
            self.assertNotIn("-" + UNIT_ENV_DIR, hidden, name)

    def test_DS2_env_readers_assert_the_import_marker(self):
        # A partial first import (an IO failure after some moves) must not
        # start a unit that reads an env file from the directory.
        readers = sorted(n for n, u in self.units.items()
                         if any(ef.lstrip("-").startswith(UNIT_ENV_DIR + "/") for ef in u["envFiles"]))
        self.assertEqual(readers, ["go-choir-checkpointd", "go-choir-corpusd", "go-choir-proxy"])
        for name in readers:
            self.assertIn(UNIT_ENV_DIR + "/.imported", self.units[name]["asserts"], name)

    def test_N1_every_unit_hides_the_reference_set_but_its_keeps(self):
        problems = []
        for name in covered_units(self.units):
            keep = set(KEEPS.get(name, ([], ""))[0])
            missing = sorted(p for p in self.reference - keep if not covered(p, self.units[name]["hidden"]))
            if missing:
                problems.append(f"{name} does not hide {missing}")
        self.assertEqual(problems, [])

    def test_N1_keeps_are_real(self):
        # A keep that the unit hides anyway (by the entry or a hidden parent
        # directory), or that is not in the reference set, is a stale record.
        for name, (keep, _why) in KEEPS.items():
            self.assertIn(name, self.units, name)
            for path in keep:
                self.assertIn(path, self.reference, f"{name}: {path}")
                self.assertFalse(covered(path, self.units[name]["hidden"]),
                                 f"{name} keeps {path} but hides it anyway")

    def test_N2_every_long_running_unit_is_verified(self):
        for name, u in self.units.items():
            if not u["enable"]:
                continue
            if (name.startswith("go-choir-") and u["type"] != "oneshot") or (
                    name in UPSTREAM_UNITS and u["type"] != "oneshot"):
                self.assertIn(name, vhh.UNITS, name)

    def test_N2_verifier_socket_keeps_match(self):
        socket_keepers = {n for n, (keep, _w) in KEEPS.items() if set(SOCKETS) <= set(keep)}
        self.assertEqual(socket_keepers, set(vhh.SYSTEMD_SOCKET_UNITS))

    def test_N3_env_files_are_read_from_the_hidden_directory(self):
        legacy = {f"-{STATE}/{n}" for n in UNIT_ENV_NAMES} | {f"{STATE}/{n}" for n in UNIT_ENV_NAMES}
        for name, u in self.units.items():
            for ef in u["envFiles"]:
                if os.path.basename(ef) in UNIT_ENV_NAMES:
                    self.assertEqual(ef, f"-{UNIT_ENV_DIR}/{os.path.basename(ef)}", name)
            for path in u["hidden"]:
                self.assertNotIn(path, legacy, f"{name} hides {path} as a file")
                self.assertFalse(path.lstrip("-").startswith(UNIT_ENV_DIR + "/"),
                                 f"{name} hides {path} one by one; the directory hides it")
        self.assertIn(f"-{UNIT_ENV_DIR}/deploy.env", self.units["go-choir-proxy"]["envFiles"])
        for unit in ("go-choir-corpusd", "go-choir-checkpointd"):
            self.assertIn(f"-{UNIT_ENV_DIR}/corpus-dsn.env", self.units[unit]["envFiles"], unit)
        self.assertIn(f"-{UNIT_ENV_DIR}/corpusd-retrieval.env", self.units["go-choir-corpusd"]["envFiles"])

    def test_N4_tmpfiles_create_the_directory(self):
        rules = config()["tmpfiles"]
        self.assertIn(f"d {UNIT_ENV_DIR} 0700 root root -", rules)
        for name in UNIT_ENV_NAMES:
            self.assertFalse(any(f" {STATE}/{name} " in r for r in rules), name)


@unittest.skipUnless(shutil.which("nix") and shutil.which("bash"), "nix or bash is not installed")
class Activation(unittest.TestCase):
    """N5: the snippet from the evaluated configuration, run on a temp tree."""

    def setUp(self):
        text = config()["activation"]
        self.assertIsNotNone(text, "no go-choir-unit-env activation snippet")
        line = f"state={STATE}\n"
        self.assertEqual(text.count(line), 1, "the snippet must set state= once")
        self.tmp = tempfile.mkdtemp()
        self.state = os.path.join(self.tmp, "state")
        os.makedirs(self.state)
        self.script = text.replace(line, f"state={self.state}\n")
        self.assertNotIn(STATE, self.script, "the snippet names the real state directory elsewhere")
        self.d = os.path.join(self.state, "unit-env")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def run_snippet(self, rc=0):
        proc = subprocess.run(["bash", "-c", self.script], capture_output=True, text=True, check=False)
        if rc == 0:
            self.assertEqual(proc.returncode, 0, proc.stderr)
        else:
            self.assertNotEqual(proc.returncode, 0, "the snippet passed a state it must fail: " + proc.stderr)
        return proc.stderr

    def write(self, path, body):
        with open(path, "w") as fh:
            fh.write(body)
        os.chmod(path, 0o644)

    def read(self, path):
        with open(path) as fh:
            return fh.read()

    def old(self, name):
        return os.path.join(self.state, name)

    def new(self, name):
        return os.path.join(self.d, name)

    def strays(self, name=None):
        return sorted(n for n in os.listdir(self.d)
                      if ".stray." in n and (name is None or n.startswith(name + ".stray.")))

    def assert_linked(self, name):
        self.assertTrue(os.path.islink(self.old(name)), name)
        self.assertEqual(os.readlink(self.old(name)), f"unit-env/{name}")

    def assert_no_temp_links(self):
        left = [n for n in os.listdir(self.state) if ".link." in n]
        self.assertEqual(left, [])

    def landed(self, **files):
        """A first run over these files at their old paths."""
        for key, body in files.items():
            self.write(self.old(key), body)
        self.run_snippet()

    # -- first run --------------------------------------------------------

    def test_first_run_moves_and_links(self):
        self.write(self.old("deploy.env"), "A=1\n")
        self.write(self.old("corpus-dsn.env"), "B=2\n")
        self.run_snippet()
        self.assertEqual(stat.S_IMODE(os.lstat(self.d).st_mode), 0o700)
        for name, body in (("deploy.env", "A=1\n"), ("corpus-dsn.env", "B=2\n")):
            self.assertEqual(self.read(self.new(name)), body)
            self.assertEqual(stat.S_IMODE(os.lstat(self.new(name)).st_mode), 0o600)
            self.assert_linked(name)
            self.assertEqual(self.read(self.old(name)), body)  # a rollback reads through the link
        for name in ("corpusd-retrieval.env", "sandbox-gateway-token.env"):
            # Absent files get a link too, so an operator who follows an old
            # runbook (`echo X=off > /var/lib/go-choir/corpusd-retrieval.env`)
            # writes into the hidden directory, where the unit reads it.
            self.assert_linked(name)
            self.assertFalse(os.path.lexists(self.new(name)), name)
        self.assertEqual(self.strays(), [])
        self.assert_no_temp_links()
        self.write(self.old("corpusd-retrieval.env"), "CORPUSD_CAPTURE_SEARCH=off\n")
        self.assertEqual(self.read(self.new("corpusd-retrieval.env")), "CORPUSD_CAPTURE_SEARCH=off\n")

    def test_first_run_keeps_the_inode(self):
        # A rename moves the dentry that carries old-view masks (F4, F5).
        self.write(self.old("deploy.env"), "A=1\n")
        ino = os.lstat(self.old("deploy.env")).st_ino
        self.run_snippet()
        self.assertEqual(os.lstat(self.new("deploy.env")).st_ino, ino)

    def test_second_run_changes_nothing(self):
        self.write(self.old("deploy.env"), "A=1\n")
        self.run_snippet()
        ino = os.lstat(self.new("deploy.env")).st_ino
        self.run_snippet()
        self.assertEqual(os.lstat(self.new("deploy.env")).st_ino, ino)
        self.assert_linked("deploy.env")

    def test_F3_first_run_imports_the_target_of_a_link_elsewhere(self):
        target = os.path.join(self.tmp, "operator-corpus-dsn.env")
        self.write(target, "B=2\n")
        os.symlink(target, self.old("corpus-dsn.env"))
        err = self.run_snippet()
        self.assertEqual(self.read(self.new("corpus-dsn.env")), "B=2\n")
        self.assertEqual(stat.S_IMODE(os.lstat(self.new("corpus-dsn.env")).st_mode), 0o600)
        self.assert_linked("corpus-dsn.env")
        self.assertEqual(self.read(target), "B=2\n")  # the operator's file is left in place
        self.assertIn(target, err)  # and named, so it can be removed

    def test_first_run_prefers_the_old_path_and_quarantines_an_unexpected_file(self):
        # Before the import the units read the old path; a file already in
        # unit-env is of unknown origin (F10).
        os.makedirs(self.d)
        self.write(self.new("corpusd-retrieval.env"), "CHOIR_OWNER_ACT_MODE=off\n")
        self.write(self.old("corpusd-retrieval.env"), "CORPUSD_CAPTURE_SEARCH=off\n")
        err = self.run_snippet(rc=1)
        self.assertEqual(self.read(self.new("corpusd-retrieval.env")), "CORPUSD_CAPTURE_SEARCH=off\n")
        (stray,) = self.strays("corpusd-retrieval.env")
        self.assertEqual(self.read(self.new(stray)), "CHOIR_OWNER_ACT_MODE=off\n")
        self.assert_linked("corpusd-retrieval.env")
        self.assertIn("corpusd-retrieval.env", err)

    def test_DS2_a_failure_after_the_second_move_heals_on_the_next_run(self):
        # An IO failure (ENOSPC) on the third file's move, injected with a mv
        # shim on the snippet's PATH. The marker stays absent, so the env
        # readers' AssertPathExists keeps them down; the next activation
        # finishes the import.
        for name, body in (("deploy.env", "A=1\n"), ("corpus-dsn.env", "B=2\n"),
                           ("corpusd-retrieval.env", "C=3\n")):
            self.write(self.old(name), body)
        shim = os.path.join(self.tmp, "shim")
        os.makedirs(shim)
        with open(os.path.join(shim, "mv"), "w") as fh:
            fh.write("#!/bin/bash\n"
                     'for a in "$@"; do if [ "$a" = "$FAIL_MV_SOURCE" ]; then '
                     'echo "mv: injected failure for $a" >&2; exit 1; fi; done\n'
                     f'exec {shutil.which("mv")} "$@"\n')
        os.chmod(os.path.join(shim, "mv"), 0o755)
        env = dict(os.environ, PATH=shim + os.pathsep + os.environ["PATH"],
                   FAIL_MV_SOURCE=self.old("corpusd-retrieval.env"))
        proc = subprocess.run(["bash", "-c", self.script], capture_output=True, text=True, env=env, check=False)
        self.assertNotEqual(proc.returncode, 0, proc.stderr)
        self.assertIn("injected failure", proc.stderr)
        self.assertFalse(os.path.exists(os.path.join(self.d, ".imported")))
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        self.assertEqual(self.read(self.new("corpus-dsn.env")), "B=2\n")
        self.assertFalse(os.path.islink(self.old("corpusd-retrieval.env")))
        self.assertEqual(self.read(self.old("corpusd-retrieval.env")), "C=3\n")  # nothing lost
        self.run_snippet()
        self.assertTrue(os.path.exists(os.path.join(self.d, ".imported")))
        for name, body in (("deploy.env", "A=1\n"), ("corpus-dsn.env", "B=2\n"),
                           ("corpusd-retrieval.env", "C=3\n")):
            self.assertEqual(self.read(self.new(name)), body)
            self.assert_linked(name)
        self.assertEqual(self.strays(), [])
        self.assert_no_temp_links()

    def test_DS7_both_paths_with_the_same_content_before_the_import(self):
        os.makedirs(self.d)
        self.write(self.new("deploy.env"), "A=1\n")
        self.write(self.old("deploy.env"), "A=1\n")
        self.run_snippet()
        self.assert_linked("deploy.env")
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        self.assertEqual(self.strays(), [])

    def test_an_aborted_first_run_is_finished_by_the_next(self):
        self.write(self.old("deploy.env"), "A=1\n")
        self.write(self.old("corpus-dsn.env"), "B=2\n")
        self.run_snippet()
        os.unlink(os.path.join(self.d, ".imported"))  # as if the run stopped before the marker
        self.run_snippet()
        self.assertEqual(self.read(self.new("corpus-dsn.env")), "B=2\n")
        self.assertEqual(self.strays(), [])

    # -- after the import: F1, F2 ------------------------------------------

    def test_F1_a_file_planted_after_the_import_is_quarantined_and_fails(self):
        self.landed(**{"deploy.env": "A=1\n"})
        # A unit that can write the state directory, or an operator's sed -i,
        # replaces links with regular files.
        os.unlink(self.old("deploy.env"))
        self.write(self.old("deploy.env"), "EVIL=1\n")
        os.unlink(self.old("corpusd-retrieval.env"))
        self.write(self.old("corpusd-retrieval.env"), "CHOIR_OWNER_ACT_MODE=off\n")
        err = self.run_snippet(rc=1)
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")  # never adopted
        self.assertFalse(os.path.lexists(self.new("corpusd-retrieval.env")))
        for name, body in (("deploy.env", "EVIL=1\n"), ("corpusd-retrieval.env", "CHOIR_OWNER_ACT_MODE=off\n")):
            self.assert_linked(name)  # nothing secret is left at the old path
            (stray,) = self.strays(name)
            self.assertEqual(self.read(self.new(stray)), body)
            self.assertEqual(stat.S_IMODE(os.lstat(self.new(stray)).st_mode), 0o600)
            self.assertIn(self.old(name), err)
        self.assert_no_temp_links()

    def test_F2_a_file_moved_back_with_the_marker_left_fails(self):
        # The old rollback text: files moved back, marker left behind.
        self.landed(**{"corpus-dsn.env": "B=2\n"})
        os.replace(self.new("corpus-dsn.env"), self.old("corpus-dsn.env"))
        err = self.run_snippet(rc=1)
        self.assertIn(self.old("corpus-dsn.env"), err)
        self.assertEqual(len(self.strays("corpus-dsn.env")), 1)

    def test_F2_a_stray_keeps_every_activation_failing(self):
        # The deploy retries a failed switch once; the retry must fail too.
        self.landed(**{"deploy.env": "A=1\n"})
        os.unlink(self.old("deploy.env"))
        self.write(self.old("deploy.env"), "EVIL=1\n")
        self.run_snippet(rc=1)
        err = self.run_snippet(rc=1)
        (stray,) = self.strays("deploy.env")
        self.assertIn(stray, err)
        os.unlink(self.new(stray))  # the operator resolves it
        self.run_snippet()

    def test_F2_rollback_by_the_corrected_text_reimports(self):
        self.landed(**{"deploy.env": "A=1\n", "corpus-dsn.env": "B=2\n"})
        for name in ("deploy.env", "corpus-dsn.env"):
            os.replace(self.new(name), self.old(name))
        os.unlink(os.path.join(self.d, ".imported"))
        self.run_snippet()
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        self.assertEqual(self.read(self.new("corpus-dsn.env")), "B=2\n")
        self.assert_linked("deploy.env")
        self.assertEqual(self.strays(), [])

    def test_F2_a_reverted_deploys_scrub_is_relinked_quietly(self):
        # The old deploy program rewrote deploy.env by rename over the link:
        # the same content as unit-env/deploy.env.
        self.landed(**{"deploy.env": "A=1\n"})
        os.unlink(self.old("deploy.env"))
        self.write(self.old("deploy.env"), "A=1\n")
        self.run_snippet()
        self.assert_linked("deploy.env")
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        self.assertEqual(self.strays(), [])

    def test_F3_after_the_import_a_link_elsewhere_is_not_adopted(self):
        self.landed(**{"deploy.env": "A=1\n"})
        target = os.path.join(self.tmp, "planted.env")
        self.write(target, "EVIL=1\n")
        os.unlink(self.old("deploy.env"))
        os.symlink(target, self.old("deploy.env"))
        err = self.run_snippet(rc=1)
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        (stray,) = self.strays("deploy.env")
        self.assertEqual(self.read(self.new(stray)), "EVIL=1\n")
        self.assert_linked("deploy.env")
        self.assertIn(target, err)

    # -- links -----------------------------------------------------------

    def test_a_link_at_the_directory_is_moved_aside(self):
        elsewhere = os.path.join(self.tmp, "elsewhere")
        os.makedirs(elsewhere)
        os.symlink(elsewhere, self.d)
        self.write(self.old("deploy.env"), "A=1\n")
        self.run_snippet()
        self.assertFalse(os.path.islink(self.d))
        self.assertTrue(os.path.isdir(self.d))
        self.assertEqual(os.listdir(elsewhere), [])
        self.assertEqual(self.read(self.new("deploy.env")), "A=1\n")
        aside = [n for n in os.listdir(self.state) if n.startswith("unit-env.invalid.")]
        self.assertEqual(len(aside), 1)

    def test_a_dangling_wrong_link_is_repointed(self):
        self.landed(**{"deploy.env": "A=1\n"})
        os.unlink(self.old("deploy.env"))
        os.symlink(os.path.join(self.tmp, "nowhere.env"), self.old("deploy.env"))
        self.run_snippet()
        self.assert_linked("deploy.env")
        self.assert_no_temp_links()

    def test_F4_a_link_to_a_directory_is_replaced_not_entered(self):
        self.landed(**{"deploy.env": "A=1\n"})
        elsewhere = os.path.join(self.tmp, "a-dir")
        os.makedirs(elsewhere)
        os.unlink(self.old("deploy.env"))
        os.symlink(elsewhere, self.old("deploy.env"))
        self.run_snippet()
        self.assert_linked("deploy.env")
        self.assertEqual(os.listdir(elsewhere), [])
        self.assert_no_temp_links()


if __name__ == "__main__":
    unittest.main()
