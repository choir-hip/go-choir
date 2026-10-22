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
  N5 The activation snippet does not move the files on its first run, does
     not leave a link at each old path (a rollback past the move reads
     through it, and an old runbook's `>` writes through it), imports a
     file planted at an old path after its first run, or follows a planted
     link at the directory.
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
    "go-choir-gateway": (["-/var/lib/go-choir/codex-auth.json", "-/var/lib/go-choir/gateway-identities.json",
                          "-/var/lib/go-choir/auth-signing/ed25519-key"], "its provider auth and identity store"),
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
    };
  }) sel);
  tmpfiles = c.systemd.tmpfiles.rules;
  activation = if acts ? go-choir-unit-env then text acts.go-choir-unit-env else null;
}
'''

_CONFIG = None


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
        for path in SOCKETS + ["-" + UNIT_ENV_DIR, "-/var/lib/go-choir/secrets"]:
            self.assertIn(path, self.reference)

    def test_N1_every_unit_hides_the_reference_set_but_its_keeps(self):
        problems = []
        for name in covered_units(self.units):
            keep = set(KEEPS.get(name, ([], ""))[0])
            missing = sorted(self.reference - keep - set(self.units[name]["hidden"]))
            if missing:
                problems.append(f"{name} does not hide {missing}")
        self.assertEqual(problems, [])

    def test_N1_keeps_are_real(self):
        # A keep that the unit hides anyway, or that is not in the reference
        # set, is a stale record.
        for name, (keep, _why) in KEEPS.items():
            self.assertIn(name, self.units, name)
            for path in keep:
                self.assertIn(path, self.reference, f"{name}: {path}")

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

    def run_snippet(self):
        proc = subprocess.run(["bash", "-c", self.script], capture_output=True, text=True, check=False)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        return proc.stderr

    def write(self, path, body):
        with open(path, "w") as fh:
            fh.write(body)
        os.chmod(path, 0o644)

    def read(self, path):
        with open(path) as fh:
            return fh.read()

    def test_first_run_moves_and_links(self):
        self.write(os.path.join(self.state, "deploy.env"), "A=1\n")
        self.write(os.path.join(self.state, "corpus-dsn.env"), "B=2\n")
        self.run_snippet()
        self.assertEqual(stat.S_IMODE(os.lstat(self.d).st_mode), 0o700)
        for name, body in (("deploy.env", "A=1\n"), ("corpus-dsn.env", "B=2\n")):
            new = os.path.join(self.d, name)
            self.assertEqual(self.read(new), body)
            self.assertEqual(stat.S_IMODE(os.lstat(new).st_mode), 0o600)
            old = os.path.join(self.state, name)
            self.assertTrue(os.path.islink(old))
            self.assertEqual(os.readlink(old), f"unit-env/{name}")
            self.assertEqual(self.read(old), body)  # a rollback reads through the link
        for name in ("corpusd-retrieval.env", "sandbox-gateway-token.env"):
            # Absent files get a link too, so an operator who follows an old
            # runbook (`echo X=off > /var/lib/go-choir/corpusd-retrieval.env`)
            # writes into the hidden directory, where the unit reads it.
            self.assertEqual(os.readlink(os.path.join(self.state, name)), f"unit-env/{name}")
            self.assertFalse(os.path.lexists(os.path.join(self.d, name)), name)
        self.write(os.path.join(self.state, "corpusd-retrieval.env"), "CORPUSD_CAPTURE_SEARCH=off\n")
        self.assertEqual(self.read(os.path.join(self.d, "corpusd-retrieval.env")), "CORPUSD_CAPTURE_SEARCH=off\n")

    def test_second_run_changes_nothing(self):
        self.write(os.path.join(self.state, "deploy.env"), "A=1\n")
        self.run_snippet()
        ino = os.lstat(os.path.join(self.d, "deploy.env")).st_ino
        self.run_snippet()
        self.assertEqual(os.lstat(os.path.join(self.d, "deploy.env")).st_ino, ino)
        self.assertEqual(os.readlink(os.path.join(self.state, "deploy.env")), "unit-env/deploy.env")

    def test_a_file_planted_after_the_import_is_never_adopted(self):
        self.write(os.path.join(self.state, "deploy.env"), "A=1\n")
        self.run_snippet()
        # A unit that can write the state directory plants env files.
        os.unlink(os.path.join(self.state, "deploy.env"))
        self.write(os.path.join(self.state, "deploy.env"), "EVIL=1\n")
        os.unlink(os.path.join(self.state, "corpusd-retrieval.env"))  # the link
        self.write(os.path.join(self.state, "corpusd-retrieval.env"), "CHOIR_OWNER_ACT_MODE=off\n")
        err = self.run_snippet()
        self.assertEqual(self.read(os.path.join(self.d, "deploy.env")), "A=1\n")
        self.assertFalse(os.path.lexists(os.path.join(self.d, "corpusd-retrieval.env")))
        self.assertEqual(self.read(os.path.join(self.state, "deploy.env")), "EVIL=1\n")  # left for the verifier
        self.assertIn("deploy.env", err)

    def test_a_link_at_the_directory_is_moved_aside(self):
        elsewhere = os.path.join(self.tmp, "elsewhere")
        os.makedirs(elsewhere)
        os.symlink(elsewhere, self.d)
        self.write(os.path.join(self.state, "deploy.env"), "A=1\n")
        self.run_snippet()
        self.assertFalse(os.path.islink(self.d))
        self.assertTrue(os.path.isdir(self.d))
        self.assertEqual(os.listdir(elsewhere), [])
        self.assertEqual(self.read(os.path.join(self.d, "deploy.env")), "A=1\n")
        aside = [n for n in os.listdir(self.state) if n.startswith("unit-env.invalid.")]
        self.assertEqual(len(aside), 1)

    def test_a_wrong_link_at_an_old_path_is_repointed(self):
        self.write(os.path.join(self.state, "deploy.env"), "A=1\n")
        self.run_snippet()
        old = os.path.join(self.state, "deploy.env")
        os.unlink(old)
        os.symlink("/tmp/elsewhere.env", old)
        self.run_snippet()
        self.assertEqual(os.readlink(old), "unit-env/deploy.env")


if __name__ == "__main__":
    unittest.main()
