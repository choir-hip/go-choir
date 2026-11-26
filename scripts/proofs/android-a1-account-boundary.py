#!/usr/bin/env python3
"""Station A1: staging assetlinks plus the real Android account boundary.

Run from the lead's approved build tree after A1 is deployed and the lab-one
APK is built. The script never reads a key, token, email or password. It binds the app's own Autoradio and
passkeys implementations (not the agent substitutes) and drives
Exact by testId with EXACT_STORE=real, pauses for the owner's Android biometric
sheet, relaunches, reads authenticated Account data, signs out and checks only
file names left by the account. It never installs over a different APK, builds,
wipes, roots or reflashes the phone.

  scripts/proofs/android-a1-account-boundary.py \
    --commit <full deployed SHA> --apk <lab-one.apk> --app-tree <lead tree> \
    --drill-handle writer-abcdef --evidence-dir docs/evidence/<new-directory> \
    --confirm-owner-biometric 'I confirm the owner will complete the Android biometric prompt for d@e'

--check-evidence DIR validates a previous summary.json without network or device
access. Any missing receipt is a failure; a failed run never prints PROOF OK.
"""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

PACKAGE = "news.choir.autoradio.lab.one"
CERT_SHA256 = "dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7"
CERT_FINGERPRINT = "DF:E9:76:7C:64:2A:2F:86:4B:29:70:49:BD:E9:7C:08:36:CB:58:D5:CE:51:6F:FE:7A:7D:A1:1F:6F:EC:9E:A7"
ANDROID_ORIGIN = "android:apk-key-hash:3-l2fGQqL4ZLKXBJvel8CDbLWNXOUW_-en2hH2_snqc"
SERIAL = "ZY22H5K8QR"
BIOMETRIC_CONFIRMATION = "I confirm the owner will complete the Android biometric prompt for d@e"
ASSETLINKS_PATH = "/.well-known/assetlinks.json"
REQUIRED_CHECKS = {
    "staging_commit",
    "staging_assetlinks",
    "apk_identity",
    "installed_apk_identity",
    "initial_signed_out",
    "biometric_sign_in",
    "relaunch_signed_in",
    "sign_out",
    "cold_relaunch_signed_out",
}


class ProofError(RuntimeError):
    pass


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def scrub(text: str) -> str:
    text = re.sub(r"choir_sk_[A-Za-z0-9_-]+", "[redacted-key]", text)
    text = re.sub(r"(?i)(authorization\s*[:=]\s*bearer\s+)[^\s,;]+", r"\1[redacted]", text)
    return re.sub(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}", "[redacted-email]", text)


def assetlinks_document() -> str:
    return '''[
  {
    "relation": [
      "delegate_permission/common.get_login_creds"
    ],
    "target": {
      "namespace": "android_app",
      "package_name": "news.choir.autoradio.lab.one",
      "sha256_cert_fingerprints": [
        "DF:E9:76:7C:64:2A:2F:86:4B:29:70:49:BD:E9:7C:08:36:CB:58:D5:CE:51:6F:FE:7A:7D:A1:1F:6F:EC:9E:A7"
      ]
    }
  }
]
'''


def require_biometric_confirmation(value: str | None) -> None:
    if value != BIOMETRIC_CONFIRMATION:
        raise ProofError("--confirm-owner-biometric must exactly confirm the owner's d@e biometric action")


def tree_value(tree: str, test_id: str) -> str:
    lines = tree.splitlines()
    marker = f"[{test_id}]"
    for i, line in enumerate(lines):
        if marker not in line:
            continue
        indent = len(line) - len(line.lstrip())
        quoted = re.findall(r'"((?:[^"\\]|\\.)*)"', line)
        if quoted:
            return quoted[-1]
        for child in lines[i + 1 :]:
            child_indent = len(child) - len(child.lstrip())
            if child.strip() and child_indent <= indent:
                break
            quoted = re.findall(r'"((?:[^"\\]|\\.)*)"', child)
            if quoted:
                return quoted[-1]
    return ""


def apksigner_digest(output: str) -> str:
    found = re.search(r"Signer #1 certificate SHA-256 digest:\s*([0-9a-fA-F]{64})", output)
    if not found or found.group(1).lower() != CERT_SHA256:
        raise ProofError("apksigner did not report the canonical lab-one signing certificate")
    return found.group(1).lower()


def validate_summary(summary: dict) -> None:
    if summary.get("status") != "pass":
        raise ProofError("evidence status is not pass")
    if summary.get("package") != PACKAGE:
        raise ProofError("evidence names another package")
    if summary.get("certificate_sha256") != CERT_SHA256:
        raise ProofError("evidence names another signing certificate")
    if not re.fullmatch(r"[0-9a-f]{40}", str(summary.get("commit", ""))):
        raise ProofError("evidence has no full deployed commit")
    if not re.fullmatch(r"[0-9a-f]{64}", str(summary.get("apk_sha256", ""))):
        raise ProofError("evidence has no APK SHA-256")
    if summary.get("biometric_owner_action") is not True:
        raise ProofError("evidence has no owner biometric confirmation")
    if summary.get("serial") != SERIAL or not re.fullmatch(r"writer-[0-9a-f]{6}", str(summary.get("drill_handle", ""))):
        raise ProofError("evidence does not name the Motorola and the public d@e handle")
    if summary.get("android_origin") != ANDROID_ORIGIN:
        raise ProofError("evidence names another Android origin")
    checks = summary.get("checks")
    if not isinstance(checks, dict) or not REQUIRED_CHECKS.issubset(checks):
        raise ProofError("evidence is missing required checks")
    if any(checks.get(name) is not True for name in REQUIRED_CHECKS):
        raise ProofError("a required evidence check is not true")


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def http_request(base: str, path: str, method: str = "GET") -> tuple[int, dict[str, str], bytes]:
    req = urllib.request.Request(base.rstrip("/") + path, method=method, headers={"Accept": "application/json"})
    opener = urllib.request.build_opener(NoRedirect)
    try:
        with opener.open(req, timeout=20) as response:
            return response.status, {k.lower(): v for k, v in response.headers.items()}, response.read()
    except urllib.error.HTTPError as error:
        return error.code, {k.lower(): v for k, v in error.headers.items()}, error.read()


def check_staging(base: str, commit: str, run_dir: Path, checks: dict[str, bool]) -> dict:
    parsed = urllib.parse.urlparse(base)
    if parsed.scheme != "https" and not (parsed.scheme == "http" and parsed.hostname in {"127.0.0.1", "localhost", "::1"}):
        raise ProofError("staging must be HTTPS (loopback HTTP is allowed only for an offline harness)")
    status, _, body = http_request(base, "/health")
    (run_dir / "staging-health.json").write_bytes(body)
    if status != 200:
        raise ProofError(f"staging /health answered {status}")
    health = json.loads(body)
    build = health.get("build") if isinstance(health, dict) else None
    if not isinstance(build, dict) or build.get("service") != "proxy" or build.get("commit") != commit:
        raise ProofError("staging proxy is not the requested deployed commit")
    checks["staging_commit"] = True

    want = assetlinks_document().encode()
    for method in ("GET", "HEAD"):
        status, headers, body = http_request(base, ASSETLINKS_PATH, method)
        (run_dir / f"assetlinks-{method.lower()}.json").write_bytes(body)
        if status != 200 or headers.get("location"):
            raise ProofError(f"{method} assetlinks answered {status} or redirected")
        if headers.get("content-type") != "application/json":
            raise ProofError(f"{method} assetlinks content type is {headers.get('content-type')!r}")
        if headers.get("content-length") != str(len(want)):
            raise ProofError(f"{method} assetlinks content length is wrong")
        if method == "GET" and body != want:
            raise ProofError("GET assetlinks is not the exact canonical document")
        if method == "HEAD" and body:
            raise ProofError("HEAD assetlinks wrote a body")
    neighbours = []
    for path in ("/.well-known/other", ASSETLINKS_PATH + "/", ASSETLINKS_PATH + "/extra"):
        status, _, body = http_request(base, path)
        if b"delegate_permission/common.get_login_creds" in body:
            raise ProofError(f"neighbour {path} served assetlinks")
        neighbours.append({"path": path, "status": status})
    (run_dir / "assetlinks-neighbours.json").write_text(json.dumps(neighbours, indent=2) + "\n")
    checks["staging_assetlinks"] = True
    return health


def run_command(name: str, command: list[str], run_dir: Path, *, cwd: Path | None = None, env: dict[str, str] | None = None, timeout: int = 300) -> str:
    try:
        result = subprocess.run(command, cwd=cwd, env=env, text=True, capture_output=True, timeout=timeout, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise ProofError(f"{name} could not run: {error}") from error
    output = scrub((result.stdout or "") + (result.stderr or ""))
    (run_dir / f"{name}.log").write_text(output)
    if result.returncode != 0:
        raise ProofError(f"{name} exited {result.returncode}; see {name}.log")
    return output


def android_tool(explicit: str | None, name: str) -> str:
    if explicit:
        found = shutil.which(explicit) or (explicit if os.access(explicit, os.X_OK) else None)
    else:
        home = os.environ.get("ANDROID_HOME") or str(Path.home() / "Library/Android/sdk")
        candidates = sorted((Path(home) / "build-tools").glob(f"*/{name}")) if name in {"aapt", "apksigner"} else []
        found = str(candidates[-1]) if candidates else shutil.which(name)
        if name == "adb":
            found = shutil.which(name) or str(Path(home) / "platform-tools" / name)
    if not found or not os.access(found, os.X_OK):
        raise ProofError(f"needs {name} on PATH or in ANDROID_HOME")
    return found


def verify_apk(apk: Path, apksigner: str, aapt: str, adb: str, run_dir: Path, checks: dict[str, bool]) -> tuple[str, str, str]:
    if not apk.is_file():
        raise ProofError("--apk names no file")
    apk_hash = hashlib.sha256(apk.read_bytes()).hexdigest()
    signed = run_command("apksigner", [apksigner, "verify", "--print-certs", str(apk)], run_dir)
    apksigner_digest(signed)
    badging = run_command("aapt-badging", [aapt, "dump", "badging", str(apk)], run_dir)
    package = re.search(r"package: name='([^']+)'", badging)
    version = re.search(r"versionName='([^']+)'", badging)
    version_code = re.search(r"versionCode='([^']+)'", badging)
    if not package or package.group(1) != PACKAGE or not version or version.group(1) != "1.0" or not version_code:
        raise ProofError("APK is not news.choir.autoradio.lab.one version 1.0")
    checks["apk_identity"] = True

    paths = run_command("adb-pm-path", [adb, "-s", SERIAL, "shell", "pm", "path", PACKAGE], run_dir).splitlines()
    base = next((line.removeprefix("package:") for line in paths if line.startswith("package:")), "")
    if not base:
        raise ProofError("lab-one is not installed on the Motorola")
    installed = run_command("adb-installed-sha256", [adb, "-s", SERIAL, "shell", "sha256sum", base], run_dir)
    if not installed or installed.split()[0].lower() != apk_hash:
        raise ProofError("installed APK bytes differ from --apk")
    dump = run_command("adb-package", [adb, "-s", SERIAL, "shell", "dumpsys", "package", PACKAGE], run_dir)
    data = re.search(r"dataDir=(/[^\s]+)", dump)
    installed_version = re.search(r"versionName=([^\s]+)", dump)
    installed_code = re.search(r"versionCode=([0-9]+)", dump)
    if not data or not installed_version or installed_version.group(1) != "1.0" or not installed_code or installed_code.group(1) != version_code.group(1):
        raise ProofError("installed package readback is incomplete or differs from the APK")
    checks["installed_apk_identity"] = True
    return apk_hash, data.group(1), installed_code.group(1)


def app_environment(app_tree: Path, apk: Path, adb: str, version_code: str) -> dict[str, str]:
    mobile = app_tree / "mobile"
    lab = mobile / "AutoradioLab"
    exact = mobile / ".exact-src"
    for path in (mobile / "scripts/exact.sh", mobile / "scripts/check-exact-pin.sh", lab / "exact.mjs", exact / "scripts/app-cli.mjs"):
        if not path.exists():
            raise ProofError(f"lead build tree is missing {path}")
    run_command("build-tree-guard", [str(mobile / "scripts/build-tree.sh"), "is"], app_tree)
    run_command("exact-pin", [str(mobile / "scripts/check-exact-pin.sh")], mobile)
    bun = shutil.which("bun") or str(Path.home() / ".bun-1.4.2/bin/bun")
    if not os.access(bun, os.X_OK):
        raise ProofError("needs Bun 1.4.2")
    android_home = os.environ.get("ANDROID_HOME") or str(Path.home() / "Library/Android/sdk")
    java_home = os.environ.get("JAVA_HOME") or "/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home"
    path = ":".join(
        [
            str(Path(adb).parent),
            str(Path(bun).parent),
            str(Path.home() / ".cargo/bin"),
            "/usr/bin",
            "/bin",
            "/usr/sbin",
            "/sbin",
            "/opt/homebrew/bin",
        ]
    )
    return {
        "HOME": str(Path.home()),
        "USER": os.environ.get("USER") or str(Path.home().name),
        "TMPDIR": os.environ.get("TMPDIR") or "/tmp",
        "BUN": bun,
        "APP_ID": PACKAGE,
        "PATH": path,
        "DEVELOPER_DIR": "/Applications/Xcode.app/Contents/Developer",
        "EXACT2": str(exact),
        "EXACT_FEEDBACK": "0",
        "EXACT_DEV_MENU": "0",
        "EXACT_VARIANT": "lab-one",
        "EXACT_STORE": "real",
        "EXACT_ANDROID_BIN": str(apk.resolve()),
        "EXACT_VERSION": "1.0",
        "EXACT_BUILD_NUMBER": version_code,
        "JAVA_HOME": java_home,
        "ANDROID_HOME": android_home,
        "ANDROID_SERIAL": SERIAL,
    }


# A named agent store keeps Exact's secrets and app:/data across the drives (a relaunch is a new process):
# without it the harness keeps them in memory and the relaunch proves nothing. `--bind passkeys` uses the
# real Credential Manager implementation under the agent, not its substitute.
def drive(name: str, operations: list[str], app_tree: Path, env: dict[str, str], run_dir: Path) -> str:
    command = [env["BUN"], "exact.mjs", "agent", "android", "--variant", "lab-one", "--storage", "a1-proof", "--bind", "autoradio", "--bind", "passkeys", *operations]
    return run_command(name, command, run_dir, cwd=app_tree / "mobile/AutoradioLab", env=env, timeout=420)


ACCOUNT_NAV = [
    "clock +6000 real",
    "clock data",
    "tap bar-mark",
    "clock +600 real",
    "clock data",
    "tap place-settings",
    "clock +900 real",
    "clock data",
    "tap settings-nav-account",
    "clock +3000 real",
    "clock data",
]


def signed_in(tree: str, handle: str) -> bool:
    return tree_value(tree, "settings-account-signed") == "Yes" and tree_value(tree, "settings-account-handle") == handle and "[settings-signout]" in tree


def signed_out(tree: str) -> bool:
    return tree_value(tree, "settings-account-signed") == "No" and "[settings-signin]" in tree and "[settings-signout]" not in tree


def write_summary(run_dir: Path, summary: dict) -> None:
    (run_dir / "summary.json").write_text(json.dumps(summary, indent=2, sort_keys=True) + "\n")


def check_evidence(path: Path) -> None:
    summary_path = path / "summary.json" if path.is_dir() else path
    if not summary_path.is_file():
        raise ProofError(f"no summary at {summary_path}")
    validate_summary(json.loads(summary_path.read_text()))
    print("PROOF OK: A1 evidence is complete")


def full_proof(args: argparse.Namespace, run_dir: Path) -> dict:
    require_biometric_confirmation(args.confirm_owner_biometric)
    if args.serial != SERIAL:
        raise ProofError(f"A1 may use only Motorola {SERIAL}")
    if not re.fullmatch(r"[0-9a-f]{40}", args.commit):
        raise ProofError("--commit must be the full 40-character deployed SHA")
    if not re.fullmatch(r"writer-[0-9a-f]{6}", args.drill_handle):
        raise ProofError("--drill-handle must name the public d@e writer handle")
    checks: dict[str, bool] = {}
    summary = {
        "status": "failed",
        "started_at": utc_now(),
        "commit": args.commit,
        "package": PACKAGE,
        "certificate_sha256": CERT_SHA256,
        "android_origin": ANDROID_ORIGIN,
        "serial": SERIAL,
        "drill_handle": args.drill_handle,
        "biometric_owner_action": False,
        "checks": checks,
    }
    try:
        health = check_staging(args.base_url, args.commit, run_dir, checks)
        adb = android_tool(args.adb, "adb")
        apksigner = android_tool(args.apksigner, "apksigner")
        aapt = android_tool(args.aapt, "aapt")
        apk = Path(args.apk).resolve()
        app_tree = Path(args.app_tree).resolve()
        apk_hash, _data_dir, version = verify_apk(apk, apksigner, aapt, adb, run_dir, checks)
        summary["apk_sha256"] = apk_hash
        summary["staging"] = {"build": health.get("build"), "platform": health.get("platform")}
        env = app_environment(app_tree, apk, adb, version)

        initial = drive("01-initial-account", [*ACCOUNT_NAV, "tree", "logs"], app_tree, env, run_dir)
        if signed_out(initial):
            checks["initial_signed_out"] = True
        elif signed_in(initial, args.drill_handle):
            drive("01b-reset-d-at-e", [*ACCOUNT_NAV, "tap settings-signout", "clock +10000 real", "clock data", "tree", "logs"], app_tree, env, run_dir)
            reset = (run_dir / "01b-reset-d-at-e.log").read_text()
            if not signed_out(reset):
                raise ProofError("the existing d@e session could not be reset")
            checks["initial_signed_out"] = True
        else:
            raise ProofError("the phone is signed in to an unknown account; no account mutation was made")

        if not sys.stdin.isatty():
            raise ProofError("the owner biometric action requires an interactive terminal")
        print("OWNER ACTION: the next drive opens Android's passkey/biometric sheet. Choose d@e and complete the device credential. The script does not tap the system sheet or handle credentials.", flush=True)
        input("Press Enter to open the sign-in sheet; complete it on the Motorola when it appears... ")
        summary["biometric_owner_action"] = True
        signin = drive("02-biometric-signin", [*ACCOUNT_NAV, "tap settings-signin", "clock +1500 real", "clock data", "tap signin", "clock +120000 real", "clock data", "tree", "logs"], app_tree, env, run_dir)
        if not signed_in(signin, args.drill_handle):
            raise ProofError("biometric sign-in did not leave the d@e Account page signed in")
        checks["biometric_sign_in"] = True

        relaunch = drive("03-relaunch-account", [*ACCOUNT_NAV, "tap settings-account-refresh", "clock +5000 real", "clock data", "tree", "logs"], app_tree, env, run_dir)
        if not signed_in(relaunch, args.drill_handle):
            raise ProofError("the relaunched app did not read authenticated d@e Account data")
        checks["relaunch_signed_in"] = True

        signout = drive("04-sign-out", [*ACCOUNT_NAV, "tap settings-signout", "clock +10000 real", "clock data", "tree", "logs"], app_tree, env, run_dir)
        if not signed_out(signout) or "[settings-signout-message]" in signout:
            raise ProofError("sign-out did not leave a clean signed-out Account page")
        checks["sign_out"] = True
        # A cold relaunch is a new process reading the named store: if the sign-out left a usable key,
        # the Account page would come back signed in. (The lab-one release APK is not debuggable, so
        # `run-as` cannot list the private files; the in-app sign-out itself reads all three secrets
        # back and requires absent, and this proves the result holds across a process.)
        run_command("force-stop", [adb, "-s", SERIAL, "shell", "am", "force-stop", PACKAGE], run_dir)
        relaunched = drive("05-cold-relaunch-signed-out", [*ACCOUNT_NAV, "clock +3000 real", "clock data", "tree", "logs"], app_tree, env, run_dir)
        if not signed_out(relaunched):
            raise ProofError("a cold relaunch came back signed in after sign-out")
        checks["cold_relaunch_signed_out"] = True
        summary["status"] = "pass"
        summary["finished_at"] = utc_now()
        write_summary(run_dir, summary)
        validate_summary(summary)
        print("PROOF OK: Android A1 assetlinks and d@e account boundary")
        return summary
    except Exception as error:
        summary["status"] = "failed"
        summary["finished_at"] = utc_now()
        summary["error"] = scrub(str(error))
        write_summary(run_dir, summary)
        raise


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-evidence", type=Path)
    parser.add_argument("--base-url", default="https://choir.news")
    parser.add_argument("--commit")
    parser.add_argument("--apk")
    parser.add_argument("--app-tree")
    parser.add_argument("--drill-handle")
    parser.add_argument("--evidence-dir", type=Path)
    parser.add_argument("--confirm-owner-biometric")
    parser.add_argument("--serial", default=SERIAL)
    parser.add_argument("--adb")
    parser.add_argument("--apksigner")
    parser.add_argument("--aapt")
    args = parser.parse_args(argv)
    try:
        if args.check_evidence:
            check_evidence(args.check_evidence)
            return 0
        missing = [name for name in ("commit", "apk", "app_tree", "drill_handle", "evidence_dir", "confirm_owner_biometric") if not getattr(args, name)]
        if missing:
            raise ProofError("missing required proof inputs: " + ", ".join("--" + name.replace("_", "-") for name in missing))
        root = args.evidence_dir.resolve()
        root.mkdir(parents=True, exist_ok=True)
        run_dir = root / f"android-a1-{dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ')}-{os.getpid()}"
        run_dir.mkdir()
        full_proof(args, run_dir)
        return 0
    except (ProofError, OSError, ValueError, json.JSONDecodeError) as error:
        print(f"PROOF FAILED: {scrub(str(error))}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
