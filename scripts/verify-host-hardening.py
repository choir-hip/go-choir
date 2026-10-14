#!/usr/bin/env python3
"""Read-only post-deploy check of Node B host hardening.

docs/problems/loopback-is-a-credential-2026-10-14.md. Run on Node B as root:

    ssh node-b python3 - --expect-internal observe --expect-dolt users \
        < scripts/verify-host-hardening.py

It never changes anything and never prints a secret:
  * internal routes: sends GET requests (and nothing that mutates) from
    loopback WITHOUT the host-internal credential and reports which are
    refused (403). With --with-credential it repeats them WITH the
    credential (read from the root-only file, never printed) as a positive
    control.
  * units: `systemctl show` sandbox properties (no Environment, so no DSN
    or secret is read) and /proc/<MainPID>/status (Uid, CapEff, CapBnd,
    NoNewPrivs, Seccomp); whether the JWT private key is reachable inside
    the proxy's and vmctl's mount namespaces (stat via nsenter: mode only).
  * Dolt: a MySQL handshake as root with an EMPTY password on :13306 and
    :13307 (accepted = the old hole); with --with-credential, the app user
    login.
  * modes of the secret files and of formerly world-readable state.

Exit status: 0 when every check matches the expected rollout stage, 1
otherwise. Expectations:
  --expect-internal off|observe|enforce   (default observe)
  --expect-dolt users|clients|locked      (default users)
"""

import argparse
import hashlib
import json
import os
import socket
import stat
import struct
import subprocess
import sys
import urllib.error
import urllib.request

SECRETS = "/var/lib/go-choir/secrets"
TOKEN_FILE = SECRETS + "/internal-service-token"
DOLT_APP_PASSWORD_FILE = SECRETS + "/dolt-app-password"
JWT_KEY = "/var/lib/go-choir/auth-signing/ed25519-key"
HEADER = "X-Choir-Internal-Token"

# (name, port, path, extra headers). GET only: read-only routes.
INTERNAL_PROBES = [
    ("vmctl list", 8083, "/internal/vmctl/list", {}),
    ("vmctl pprof index", 8083, "/internal/debug/pprof/", {}),
    ("gateway breakers", 8084, "/provider/v1/breakers", {}),
    ("gateway search health", 8084, "/provider/v1/search/health", {}),
    ("corpusd control key", 8086, "/internal/platform/control-key", {}),
    ("corpusd pprof index", 8086, "/internal/debug/pprof/", {}),
    ("maild drafts (probe owner)", 8087, "/api/email/drafts", {"X-Authenticated-User": "verify-host-hardening-probe"}),
]

UNITS = [
    "go-choir-auth", "go-choir-proxy", "go-choir-gateway", "go-choir-corpusd",
    "go-choir-maild", "go-choir-vmctl", "go-choir-platform-dolt",
    "go-choir-corpus-dolt", "go-choir-checkpointd", "go-choir-front-importance",
    "go-choir-goproxy", "searx", "qdrant", "caddy",
]
# Root services expected to hold no capabilities after the hardening commit.
NO_CAP_UNITS = {
    "go-choir-auth", "go-choir-proxy", "go-choir-gateway", "go-choir-corpusd",
    "go-choir-maild", "go-choir-platform-dolt", "go-choir-corpus-dolt",
    "go-choir-checkpointd", "go-choir-front-importance", "go-choir-goproxy",
}
FORBIDDEN_SYSCALLS = ["bpf", "chroot", "acct", "setuid", "capset"]

results = []


def record(section, name, ok, detail):
    results.append({"section": section, "check": name, "ok": bool(ok), "detail": detail})


def read_secret(path):
    try:
        with open(path, "r", encoding="utf-8") as fh:
            return fh.read().strip()
    except OSError:
        return ""


def http_get(port, path, headers):
    req = urllib.request.Request(f"http://127.0.0.1:{port}{path}", method="GET")
    req.add_header("X-Internal-Caller", "true")
    for k, v in headers.items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            resp.read(1)
            return resp.status
    except urllib.error.HTTPError as err:
        return err.code
    except (urllib.error.URLError, OSError) as err:
        return f"error: {err.__class__.__name__}"


def check_internal(expect, with_credential):
    token = read_secret(TOKEN_FILE) if with_credential else ""
    for name, port, path, extra in INTERNAL_PROBES:
        status = http_get(port, path, dict(extra))
        if expect == "enforce":
            ok = status == 403
            want = "403"
        else:
            ok = status != 403 and isinstance(status, int)
            want = "not 403 (" + expect + ")"
        record("internal", f"{name} without credential", ok, f"status={status} want={want}")
        if with_credential:
            if not token:
                record("internal", f"{name} with credential", False, "credential file missing or empty")
                continue
            headers = dict(extra)
            headers[HEADER] = token
            status = http_get(port, path, headers)
            record("internal", f"{name} with credential", isinstance(status, int) and status != 403,
                   f"status={status} want=not 403")


def systemctl_show(unit, props):
    out = subprocess.run(["systemctl", "show", unit, "-p", ",".join(props)],
                         capture_output=True, text=True, check=False).stdout
    values = {}
    for line in out.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            values.setdefault(k, []).append(v)
    return {k: " ".join(v) for k, v in values.items()}


def proc_status(pid):
    fields = {}
    try:
        with open(f"/proc/{pid}/status", "r", encoding="utf-8") as fh:
            for line in fh:
                k, _, v = line.partition(":")
                if k in ("Uid", "CapEff", "CapBnd", "NoNewPrivs", "Seccomp"):
                    fields[k] = v.strip()
    except OSError:
        pass
    return fields


def mode_in_namespace(pid, path):
    """Mode of path inside pid's mount namespace (stat only, no read)."""
    proc = subprocess.run(["nsenter", "-t", str(pid), "-m", "--", "stat", "-c", "%a", path],
                          capture_output=True, text=True, check=False)
    if proc.returncode != 0:
        return "absent"
    return proc.stdout.strip()


# Units that must hold the host-internal credential (LoadCredential).
TOKEN_UNITS = ["go-choir-proxy", "go-choir-maild", "go-choir-corpusd", "go-choir-vmctl", "go-choir-gateway"]


def journal_lines(unit, pattern, since="-24h"):
    proc = subprocess.run(["journalctl", "-u", unit, "--since", since, "--no-pager", "-o", "cat", "--grep", pattern],
                          capture_output=True, text=True, check=False)
    return [line for line in proc.stdout.splitlines() if line.strip()]


def check_credential_delivery(expect):
    for unit in TOKEN_UNITS:
        path = f"/run/credentials/{unit}.service/choir-internal-token"
        present = os.path.exists(path)  # existence only; never opened
        record("credential", f"{unit} holds {os.path.basename(path)}", present or expect == "off",
               "present" if present else "absent")
        started = journal_lines(unit, "hostauth: internal routes")
        last = started[-1] if started else "no hostauth startup line in the last 24h"
        ok = expect == "off" or f"mode={expect}" in last
        record("credential", f"{unit} hostauth mode", ok, last[-160:])
        would = journal_lines(unit, "hostauth: observe: would refuse")
        refused = journal_lines(unit, "hostauth: enforce: refused")
        # Before flipping to enforce, "would refuse" must be zero for a day.
        record("credential", f"{unit} refusals in the last 24h", expect != "observe" or not would,
               f"would_refuse={len(would)} refused={len(refused)}" + (f"; latest: {would[-1][-200:]}" if would else ""))


def check_units():
    props = ["MainPID", "User", "DynamicUser", "CapabilityBoundingSet", "SystemCallFilter",
             "ProtectHome", "LoadCredential", "InaccessiblePaths", "ActiveState"]
    for unit in UNITS:
        info = systemctl_show(unit, props)
        pid = info.get("MainPID", "0")
        status = proc_status(pid) if pid not in ("", "0") else {}
        detail = {
            "active": info.get("ActiveState"),
            "user": info.get("User") or "root",
            "dynamic_user": info.get("DynamicUser"),
            "cap_bounding": info.get("CapabilityBoundingSet", "")[:120],
            "protect_home": info.get("ProtectHome"),
            "proc": status,
        }
        record("unit", f"{unit} running", info.get("ActiveState") in ("active", "inactive"), detail)
        if unit in NO_CAP_UNITS and status.get("CapBnd"):
            record("unit", f"{unit} has no capabilities", int(status["CapBnd"], 16) == 0,
                   f"CapBnd={status['CapBnd']} CapEff={status.get('CapEff')}")
        filt = info.get("SystemCallFilter", "")
        if filt and unit.startswith("go-choir-"):
            allowed = set(filt.lstrip("~").split())
            is_allow_list = not filt.startswith("~")
            leaked = [s for s in FORBIDDEN_SYSCALLS if is_allow_list and s in allowed]
            record("unit", f"{unit} syscall filter excludes @privileged", not leaked,
                   f"privileged syscalls still allowed: {leaked}" if leaked else "ok")
    for unit in ("go-choir-proxy", "go-choir-vmctl", "go-choir-gateway"):
        pid = systemctl_show(unit, ["MainPID"]).get("MainPID", "0")
        if pid in ("", "0"):
            record("unit", f"{unit} cannot reach the JWT private key", False, "not running")
            continue
        mode = mode_in_namespace(pid, JWT_KEY)
        record("unit", f"{unit} cannot reach the JWT private key", mode in ("0", "absent"),
               f"mode inside the unit's namespace: {mode}")


def mysql_read_packet(sock):
    header = b""
    while len(header) < 4:
        chunk = sock.recv(4 - len(header))
        if not chunk:
            raise ConnectionError("closed")
        header += chunk
    length = header[0] | (header[1] << 8) | (header[2] << 16)
    seq = header[3]
    body = b""
    while len(body) < length:
        chunk = sock.recv(length - len(body))
        if not chunk:
            raise ConnectionError("closed")
        body += chunk
    return seq, body


def mysql_send(sock, seq, payload):
    sock.sendall(struct.pack("<I", len(payload))[:3] + bytes([seq & 0xFF]) + payload)


def native_scramble(password, salt):
    if not password:
        return b""
    p1 = hashlib.sha1(password.encode()).digest()
    p2 = hashlib.sha1(p1).digest()
    p3 = hashlib.sha1(salt + p2).digest()
    return bytes(a ^ b for a, b in zip(p1, p3))


def mysql_login(port, user, password, database=None):
    """Returns 'accepted', 'refused' or 'error: ...'. Never prints the password."""
    try:
        sock = socket.create_connection(("127.0.0.1", port), timeout=10)
    except OSError as err:
        return f"error: {err.__class__.__name__}"
    try:
        seq, greet = mysql_read_packet(sock)
        if not greet or greet[0] == 0xFF:
            return "error: server refused the connection"
        pos = greet.index(b"\x00", 1) + 1  # server version
        pos += 4  # thread id
        salt = greet[pos:pos + 8]
        pos += 8 + 1  # salt part 1 + filler
        pos += 2 + 1 + 2 + 2  # cap low, charset, status, cap high
        auth_len = greet[pos] if pos < len(greet) else 0
        pos += 1 + 10
        salt += greet[pos:pos + max(13, auth_len - 8) - 1]
        salt = salt[:20]
        caps = 0x00000200 | 0x00008000 | 0x00080000  # PROTOCOL_41 | SECURE_CONNECTION | PLUGIN_AUTH
        if database:
            caps |= 0x00000008  # CONNECT_WITH_DB
        auth = native_scramble(password, salt)
        payload = struct.pack("<IIB", caps, 1 << 24, 33) + b"\x00" * 23
        payload += user.encode() + b"\x00" + bytes([len(auth)]) + auth
        if database:
            payload += database.encode() + b"\x00"
        payload += b"mysql_native_password\x00"
        mysql_send(sock, seq + 1, payload)
        seq, reply = mysql_read_packet(sock)
        if reply and reply[0] == 0xFE:  # auth switch
            rest = reply[1:]
            plugin, _, data = rest.partition(b"\x00")
            new_salt = data.rstrip(b"\x00")[:20]
            if plugin != b"mysql_native_password":
                return f"error: auth plugin {plugin.decode(errors='replace')} not supported by this probe"
            mysql_send(sock, seq + 1, native_scramble(password, new_salt))
            seq, reply = mysql_read_packet(sock)
        if reply and reply[0] == 0x00:
            return "accepted"
        if reply and reply[0] == 0xFF:
            return "refused"
        return "error: unexpected reply"
    except (OSError, ValueError, ConnectionError) as err:
        return f"error: {err.__class__.__name__}"
    finally:
        sock.close()


def check_dolt(expect, with_credential):
    for port, db in ((13306, "platform"), (13307, "corpus")):
        outcome = mysql_login(port, "root", "")
        if expect == "locked":
            ok, want = outcome == "refused", "refused"
        else:
            ok, want = outcome in ("accepted", "refused"), "any (root locked only at stage locked)"
        record("dolt", f":{port} root with empty password", ok, f"{outcome}; want {want}")
        if with_credential:
            app_pw = read_secret(DOLT_APP_PASSWORD_FILE)
            outcome = mysql_login(port, "choir", app_pw, db) if app_pw else "error: password file missing"
            record("dolt", f":{port} app user choir on {db}", outcome == "accepted", outcome)


def check_modes():
    expect = [
        ("/var/lib/go-choir", 0o750),
        (SECRETS, 0o700),
        (TOKEN_FILE, 0o600),
        (TOKEN_FILE + ".header", 0o600),
        (DOLT_APP_PASSWORD_FILE, 0o600),
        (SECRETS + "/dolt-root-password", 0o600),
        ("/var/lib/go-choir/auth", 0o750),
        ("/var/lib/go-choir/auth/auth.db", 0o600),
        ("/var/lib/go-choir/vm-state", 0o750),
        ("/var/lib/go-choir/platform-dolt", 0o750),
        ("/var/lib/go-choir/corpus-dolt", 0o750),
        ("/var/lib/go-choir/platform-artifacts", 0o750),
        ("/var/lib/go-choir/corpus-dsn.env", 0o600),
        ("/var/lib/go-choir/sandbox-gateway-token.env", 0o600),
    ]
    for path, want in expect:
        try:
            st = os.stat(path)
        except OSError:
            record("modes", path, False, "absent")
            continue
        mode = stat.S_IMODE(st.st_mode)
        # "at most": a tighter mode passes, a looser one fails.
        ok = (mode & ~want) == 0 and st.st_uid == 0
        record("modes", path, ok, f"mode={mode:o} uid={st.st_uid} want<={want:o} root")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--expect-internal", choices=["off", "observe", "enforce"], default="observe")
    ap.add_argument("--expect-dolt", choices=["users", "clients", "locked"], default="users")
    ap.add_argument("--with-credential", action="store_true",
                    help="also run positive controls using the root-only credential files (never printed)")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    if os.geteuid() != 0:
        print("run as root on Node B (systemctl show, /proc and nsenter need it)", file=sys.stderr)
        return 2
    check_internal(args.expect_internal, args.with_credential)
    check_credential_delivery(args.expect_internal)
    check_units()
    check_dolt(args.expect_dolt, args.with_credential)
    check_modes()
    if args.json:
        print(json.dumps(results, indent=2))
    else:
        for r in results:
            detail = r["detail"] if isinstance(r["detail"], str) else json.dumps(r["detail"])
            print(f"{'PASS' if r['ok'] else 'FAIL'}  [{r['section']}] {r['check']}: {detail}")
    failed = [r for r in results if not r["ok"]]
    print(f"\n{len(results) - len(failed)} passed, {len(failed)} failed "
          f"(expect internal={args.expect_internal}, dolt={args.expect_dolt})", file=sys.stderr)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
