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
  * boot: whether the host runs what it last activated (kernel, kernel
    modules, kernel command line, declared KVM module parameters), and
    kvm_intel nested=N whatever is declared
    (docs/reports/patch-lanes-plan-2026-10-15.md, slice 1). A pending
    reboot is a FAIL unless --allow-reboot-pending.

    python3 - --boot-state-json < scripts/verify-host-hardening.py

prints only the boot-state JSON (no root needed): the deploy receipt's
host_identity.boot_state. internal/hostboot is the same contract in Go
(vmctl's GET /internal/vmctl/host/boot-state); its parity test runs this
script with --root against fixture trees.

Exit status: 0 when every check matches the expected rollout stage, 1
otherwise. Expectations:
  --expect-internal off|observe|enforce   (default observe)
  --expect-dolt users|clients|locked      (default users)
"""

import argparse
import datetime
import grp
import hashlib
import json
import os
import re
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
    ("vmctl host boot-state", 8083, "/internal/vmctl/host/boot-state", {}),
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
# The host services a guest reaches on its own tap host address: the floor's
# one list, internal/vmmanager tapReachableHostServicePorts (8089 is
# go-choir-pkgmirror, NET-02). TestHostFloorScriptsExpectEveryTapServicePort
# keeps the two in step.
TAP_SERVICE_PORTS = ("8082", "8083", "8084", "8086", "8087", "8088", "8089")
FORBIDDEN_SYSCALLS = ["bpf", "chroot", "acct", "setuid", "capset"]
# Units whose filter allows @privileged on purpose, with the reason. Every
# other go-choir unit must exclude it.
PRIVILEGED_SYSCALLS_ALLOWED = {
    "go-choir-vmctl": "the VMM jailer it runs needs them (chroot/pivot_root, setuid/setgid "
                      "uid drop, capset bounding cut): 10e9f18e1, nix/node-b.nix vmctl SystemCallFilter",
}

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
            if unit in PRIVILEGED_SYSCALLS_ALLOWED:
                record("unit", f"{unit} syscall filter allows @privileged (intended)", True,
                       f"allowed: {leaked or 'none'}; reason: {PRIVILEGED_SYSCALLS_ALLOWED[unit]}")
            else:
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


# Quick containment (blast-radius design, wave 1): PID 1's sockets, the env
# files read only through EnvironmentFile=, and vmctl's read-only view.
SYSTEMD_SOCKETS = ["/run/systemd/private", "/run/dbus/system_bus_socket"]
SYSTEMD_SOCKET_UNITS = {"go-choir-vmctl", "go-choir-dolt-liveness-probe"}  # they run systemctl
ENV_FILES = ["/var/lib/go-choir/deploy.env", "/var/lib/go-choir/corpus-dsn.env",
             "/var/lib/go-choir/sandbox-gateway-token.env"]
VMCTL_READ_ONLY = ["/var/lib/go-choir/services", "/var/lib/go-choir/guest"]


def writable_in_namespace(pid, path):
    """Whether path is writable inside pid's mount namespace (access(2), no write)."""
    proc = subprocess.run(["nsenter", "-t", str(pid), "-m", "--", "/run/current-system/sw/bin/test", "-w", path],
                          capture_output=True, text=True, check=False)
    return proc.returncode == 0


def check_containment():
    for unit in UNITS:
        pid = systemctl_show(unit, ["MainPID"]).get("MainPID", "0")
        if pid in ("", "0"):
            continue  # not running now (a oneshot); its view is checked when it runs
        hidden = [] if unit in SYSTEMD_SOCKET_UNITS else SYSTEMD_SOCKETS
        for path in hidden + ENV_FILES:
            mode = mode_in_namespace(pid, path)
            record("containment", f"{unit} cannot reach {path}", mode in ("0", "absent"),
                   f"mode inside the unit's namespace: {mode}")
    pid = systemctl_show("go-choir-vmctl", ["MainPID"]).get("MainPID", "0")
    for path in VMCTL_READ_ONLY:
        if pid in ("", "0"):
            record("containment", f"go-choir-vmctl cannot write {path}", False, "not running")
            continue
        record("containment", f"go-choir-vmctl cannot write {path}", not writable_in_namespace(pid, path),
               "writable inside vmctl's namespace" if writable_in_namespace(pid, path) else "read-only")


def check_dolt_probe():
    """The query-level Dolt liveness probe (scripts/dolt-liveness-probe.sh)
    must be scheduled and healthy: docs/problems/
    dolt-clustering-assessment-2026-10-15.md section 7. Read-only."""
    timer = systemctl_show("go-choir-dolt-liveness-probe.timer", ["ActiveState"])
    record("dolt-probe", "dolt liveness probe timer is active", timer.get("ActiveState") == "active",
           f"ActiveState={timer.get('ActiveState')}")
    svc = systemctl_show("go-choir-dolt-liveness-probe.service", ["Result"])
    # A probe-error run (credential or client fault) ends the oneshot non-zero.
    record("dolt-probe", "dolt liveness probe last run succeeded", svc.get("Result") in ("success", ""),
           f"Result={svc.get('Result')}")
    errors = journal_lines("go-choir-dolt-liveness-probe.service", "probe-error", since="-15min")
    record("dolt-probe", "dolt liveness probe reports no probe-error in the last 15 minutes", not errors,
           errors[-1][-200:] if errors else "none")
    for unit in ("go-choir-platform-dolt", "go-choir-corpus-dolt"):
        info = systemctl_show(unit + ".service", ["TimeoutStopUSec"])
        got = info.get("TimeoutStopUSec")
        record("dolt-probe", f"{unit} stop timeout is 20s", got == "20s", f"TimeoutStopUSec={got}")


def check_paper_probe():
    """The paper liveness probe (scripts/paper-liveness-probe.py) must be
    scheduled, able to see the paper, and silent: docs/problems/
    paper-silent-since-10-56-after-host-reboot-2026-10-15.md. Read-only."""
    timer = systemctl_show("go-choir-paper-liveness-probe.timer", ["ActiveState"])
    record("paper-probe", "paper liveness probe timer is active", timer.get("ActiveState") == "active",
           f"ActiveState={timer.get('ActiveState')}")
    svc = systemctl_show("go-choir-paper-liveness-probe.service", ["Result"])
    record("paper-probe", "paper liveness probe last run succeeded (it can resolve the paper vm and read the journal)",
           svc.get("Result") in ("success", ""), f"Result={svc.get('Result')}")
    alerts = journal_lines("go-choir-paper-liveness-probe.service", "ALERT", since="-15min")
    record("paper-probe", "paper made an inference call within 30 minutes (no ALERT in the last 15 minutes)", not alerts,
           alerts[-1][-200:] if alerts else "none")


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


def _run(*cmd):
    try:
        return subprocess.run(cmd, capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return subprocess.CompletedProcess(cmd, 127, "", str(exc))


def check_guest_floor(dns_port=8053):
    """FP3 group 2 (SEC-03, SEC-09, SEC-10 host side, NET-07)."""
    # SEC-09: KSM not running (2 = stopped and unmerged, 0 = stopped).
    ksm = _read_trim("/sys/kernel/mm/ksm/run")
    record("guest-floor", "ksm run", ksm in ("0", "2"), f"run={ksm or 'absent'}")
    # NET-07: /dev/kvm is exactly 0660 root:kvm (the kvm group is NixOS's
    # default, gid 302). Direct VMMs run as root; jailed VMMs mknod their own
    # node in the jail.
    try:
        st = os.stat("/dev/kvm")
        mode = stat.S_IMODE(st.st_mode)
        try:
            kvm_gid = grp.getgrnam("kvm").gr_gid
        except KeyError:
            kvm_gid = None
        record("guest-floor", "/dev/kvm mode", mode == 0o660 and st.st_uid == 0 and kvm_gid is not None and st.st_gid == kvm_gid,
               f"mode={mode:o} uid={st.st_uid} gid={st.st_gid} kvm_gid={kvm_gid} want 660 root:kvm")
    except OSError:
        record("guest-floor", "/dev/kvm mode", False, "absent")
    # SEC-03: the mangle egress policy, its jump, the IPv6 drop, the
    # reply-only FORWARD chain.
    chain = _run("iptables", "-t", "mangle", "-S", "CHOIR-GUEST-EGRESS").stdout
    rules = [l for l in chain.splitlines() if l.startswith("-A ")]
    record("guest-floor", "egress chain", len(rules) > 0 and all("80,443" not in l for l in rules)
           and any("--limit-iface-in" in l for l in rules), f"{len(rules)} rules")
    want_ports = ",".join(TAP_SERVICE_PORTS)
    services = [l for l in rules if "--limit-iface-in" in l and "-p tcp" in l and "--dports" in l and l.endswith("-j RETURN")]
    record("guest-floor", "egress service ports", any(f"--dports {want_ports} " in l for l in services),
           f"want {want_ports}; have {[l.split('--dports ')[1].split()[0] for l in services]}")
    pre = _run("iptables", "-t", "mangle", "-S", "PREROUTING").stdout.splitlines()
    record("guest-floor", "egress jump first", len(pre) > 1 and "CHOIR-GUEST-EGRESS" in pre[1],
           pre[1] if len(pre) > 1 else "no rules")
    pre6 = _run("ip6tables", "-t", "mangle", "-S", "PREROUTING").stdout
    record("guest-floor", "guest IPv6 drop", "-i vm-+" in pre6 and "-j DROP" in pre6, "ip6tables mangle PREROUTING")
    fwd = _run("iptables", "-S", "FORWARD").stdout
    record("guest-floor", "reply-only FORWARD jump", "-o vm-+" in fwd and "CHOIR-GUEST-RET" in fwd, "FORWARD -o vm-+")
    per_tap = [l for l in fwd.splitlines() if re.match(r"-A FORWARD -o vm-[a-z2-7]{12} ", l)]
    record("guest-floor", "no per-tap FORWARD ACCEPT", not per_tap, f"{len(per_tap)} left")
    # SEC-10 host side: the resolver answers on loopback.
    active = _run("systemctl", "is-active", "unbound").stdout.strip()
    record("guest-floor", "unbound active", active == "active", active)
    listening = _run("ss", "-Hlnu", f"sport = :{dns_port}").stdout.strip()
    record("guest-floor", f"resolver on 127.0.0.1:{dns_port}", "127.0.0.1:" + str(dns_port) in listening,
           listening or "not listening")


# ── boot state: installed but not booted ────────────────────────────────────
# Mirrors internal/hostboot (Go). Keep the two in step: TestPythonParity in
# internal/hostboot compares their JSON on the same fixture trees.

BOOT_SCHEMA_VERSION = 1
BOOT_OVERDUE_SECONDS = 7 * 24 * 3600
KERNEL_PARAM_RE = re.compile(r"(kvm[A-Za-z0-9_-]*)\.([A-Za-z0-9_]+)=(.*)")


def _read_trim(path):
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as fh:
            return fh.read().strip()
    except OSError:
        return ""


def _resolve(path):
    try:
        return os.path.realpath(path, strict=True)
    except OSError:
        return ""


def _iso(ts):
    return datetime.datetime.fromtimestamp(int(ts), datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _read_modprobe(system):
    d = os.path.join(system, "etc", "modprobe.d")
    try:
        names = sorted(n for n in os.listdir(d) if n.endswith(".conf"))
    except OSError:
        return ""
    return "".join(_read_trim(os.path.join(d, n)) + "\n" for n in names)


def _boot_inputs(system):
    return {
        "kernel": _resolve(os.path.join(system, "kernel")),
        "modules": _resolve(os.path.join(system, "kernel-modules")),
        "params": _read_trim(os.path.join(system, "kernel-params")),
        "modprobe": _read_modprobe(system),
    }


def _same_boot(a, b):
    def norm(s):
        return " ".join(s.split())
    return (a["kernel"] == b["kernel"] and a["modules"] == b["modules"]
            and norm(a["params"]) == norm(b["params"]) and norm(a["modprobe"]) == norm(b["modprobe"]))


def _declared_module_params(params, system):
    """KVM module parameters the system declares. A later declaration wins,
    kernel parameters last (the command line overrides modprobe options)."""
    out, index = [], {}

    def add(mod, param, value, source):
        mod = mod.replace("-", "_")
        mp = {"module": mod, "param": param, "declared": value, "source": source}
        key = mod + "." + param
        if key in index:
            out[index[key]] = mp
        else:
            index[key] = len(out)
            out.append(mp)

    for line in _read_modprobe(system).split("\n"):
        f = line.split()
        if len(f) < 3 or f[0] != "options" or not f[1].startswith("kvm"):
            continue
        for kv in f[2:]:
            if "=" in kv:
                k, v = kv.split("=", 1)
                add(f[1], k, v, "modprobe.d")
    for tok in params.split():
        m = KERNEL_PARAM_RE.fullmatch(tok)
        if m:
            add(m.group(1), m.group(2), m.group(3), "kernel-params")
    return out


def _same_param(declared, live):
    # Boolean module parameters read back as Y or N whatever was written.
    if live in ("Y", "N"):
        d = declared.lower()
        if d in ("1", "y", "yes", "true", "on"):
            return live == "Y"
        if d in ("0", "n", "no", "false", "off"):
            return live == "N"
    return declared == live


def _pending_since(profiles, booted, booted_in):
    """Activation time of the first generation after the booted one whose boot
    inputs differ (or the first differing one if the booted one is gone)."""
    try:
        names = os.listdir(profiles)
    except OSError:
        return None
    gens = []
    for name in names:
        m = re.fullmatch(r"system-(\d+)-link", name)
        if m:
            gens.append((int(m.group(1)), os.path.join(profiles, name)))
    gens.sort()
    start = 0
    for i, (_, link) in enumerate(gens):
        if _resolve(link) == booted:
            start = i + 1
    for _, link in gens[start:]:
        target = _resolve(link)
        if not target or _same_boot(_boot_inputs(target), booted_in):
            continue
        try:
            return os.lstat(link).st_mtime
        except OSError:
            return None
    return None


def boot_state(root="/", now=None):
    """The installed-but-not-booted fact. Same JSON as internal/hostboot."""
    if now is None:
        now = datetime.datetime.now(datetime.timezone.utc).timestamp()

    def at(p):
        return os.path.join(root, p.lstrip("/"))

    st = {
        "schema_version": BOOT_SCHEMA_VERSION,
        "checked_at": _iso(now),
        "available": False,
        "error": "",
        "running_kernel_release": _read_trim(at("/proc/sys/kernel/osrelease")),
        "booted_at": "",
        "booted_system": "",
        "current_system": "",
        "booted_kernel": "",
        "current_kernel": "",
        "booted_kernel_modules": "",
        "current_kernel_modules": "",
        "kernel_params_not_on_cmdline": [],
        "module_params": [],
        "reboot_pending": False,
        "reasons": [],
        "pending_since": "",
        "pending_overdue": False,
    }
    for line in _read_trim(at("/proc/stat")).split("\n"):
        if line.startswith("btime "):
            try:
                st["booted_at"] = _iso(int(line.split()[1]))
            except (IndexError, ValueError):
                pass

    def finish():
        # Empty strings are omitted, as Go's omitempty does.
        for key in ("error", "running_kernel_release", "booted_at", "booted_system", "current_system",
                    "booted_kernel", "current_kernel", "booted_kernel_modules", "current_kernel_modules",
                    "pending_since"):
            if st[key] == "":
                del st[key]
        for mp in st["module_params"]:
            if mp.get("live") == "":
                del mp["live"]
        return st

    booted = _resolve(at("/run/booted-system"))
    if not booted:
        st["error"] = "booted system unreadable"
        return finish()
    current = _resolve(at("/run/current-system"))
    if not current:
        st["error"] = "current system unreadable"
        return finish()
    st["available"] = True
    st["booted_system"], st["current_system"] = booted, current
    b_in, c_in = _boot_inputs(booted), _boot_inputs(current)
    st["booted_kernel"], st["current_kernel"] = b_in["kernel"], c_in["kernel"]
    st["booted_kernel_modules"], st["current_kernel_modules"] = b_in["modules"], c_in["modules"]
    if b_in["kernel"] != c_in["kernel"]:
        st["reasons"].append("kernel")
    if b_in["modules"] != c_in["modules"]:
        st["reasons"].append("kernel_modules")
    cmdline = set(_read_trim(at("/proc/cmdline")).split())
    st["kernel_params_not_on_cmdline"] = [p for p in c_in["params"].split() if p not in cmdline]
    if st["kernel_params_not_on_cmdline"]:
        st["reasons"].append("kernel_params")
    mismatch = False
    for mp in _declared_module_params(c_in["params"], current):
        d = at("/sys/module/" + mp["module"])
        if os.path.exists(d):
            mp["loaded"] = True
            mp["live"] = _read_trim(os.path.join(d, "parameters", mp["param"]))
            mp["match"] = _same_param(mp["declared"], mp["live"])
            mismatch = mismatch or not mp["match"]
        else:
            # Not loaded: the parameter applies when the module loads.
            mp["loaded"], mp["match"] = False, True
        st["module_params"].append(mp)
    if mismatch:
        st["reasons"].append("module_param")
    st["reboot_pending"] = bool(st["reasons"])
    if st["reboot_pending"]:
        since = _pending_since(at("/nix/var/nix/profiles"), booted, b_in)
        if since is not None:
            st["pending_since"] = _iso(since)
            st["pending_overdue"] = now - since > BOOT_OVERDUE_SECONDS
    return finish()


def check_boot(allow_pending, root="/"):
    st = boot_state(root)
    if not st["available"]:
        record("boot", "boot state readable", False, st.get("error", "unavailable"))
        return
    reasons = set(st["reasons"])
    allowed = " (allowed: --allow-reboot-pending)" if allow_pending else ""
    since = st.get("pending_since", "unknown")
    record("boot", "booted kernel is the activated kernel", "kernel" not in reasons or allow_pending,
           "running %s; booted %s; activated %s%s" % (
               st.get("running_kernel_release"), st.get("booted_kernel"), st.get("current_kernel"),
               allowed if "kernel" in reasons else ""))
    record("boot", "booted kernel modules are the activated ones", "kernel_modules" not in reasons or allow_pending,
           "pending since %s%s" % (since, allowed) if "kernel_modules" in reasons else "ok")
    record("boot", "activated kernel parameters are on /proc/cmdline", "kernel_params" not in reasons or allow_pending,
           "missing: %s%s" % (" ".join(st["kernel_params_not_on_cmdline"]), allowed)
           if "kernel_params" in reasons else "ok")
    for mp in st["module_params"]:
        live = mp.get("live", "") if mp["loaded"] else "module not loaded"
        record("boot", "%s.%s live value is the declared one" % (mp["module"], mp["param"]),
               mp["match"] or allow_pending,
               "declared %s (%s), live %s%s" % (mp["declared"], mp["source"], live, "" if mp["match"] else allowed))
    record("boot", "reboot not pending for more than 7 days", not st["pending_overdue"] or allow_pending,
           "pending since %s" % since if st["reboot_pending"] else "no reboot pending")
    # The security floor whatever is declared: no Choir guest runs a
    # hypervisor (docs/problems/host-kernel-216-days-unpatched-2026-10-15.md).
    nested = _read_trim(os.path.join(root, "sys/module/kvm_intel/parameters/nested"))
    nested_on = nested not in ("N", "0", "")
    record("boot", "kvm_intel nested is off", not nested_on or allow_pending,
           ("nested=%s" % nested if nested else "kvm_intel not loaded") + (allowed if nested_on else ""))


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--expect-internal", choices=["off", "observe", "enforce"], default="observe")
    ap.add_argument("--expect-dolt", choices=["users", "clients", "locked"], default="users")
    ap.add_argument("--with-credential", action="store_true",
                    help="also run positive controls using the root-only credential files (never printed)")
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--expect-guest-floor", action="store_true",
                    help="also check FP3 group 2: guest egress floor, KSM off, /dev/kvm 0660, the guest resolver")
    ap.add_argument("--allow-reboot-pending", action="store_true",
                    help="report a pending host reboot without failing (the baseline before a planned reboot)")
    ap.add_argument("--boot-state-json", action="store_true",
                    help="print only the boot-state JSON and exit 0 (no root needed)")
    ap.add_argument("--root", default="/", help=argparse.SUPPRESS)  # fixture trees in tests
    ap.add_argument("--now", type=float, default=None, help=argparse.SUPPRESS)  # fixed clock in tests
    args = ap.parse_args()
    if args.boot_state_json:
        print(json.dumps(boot_state(args.root, args.now), indent=2, sort_keys=True))
        return 0
    if os.geteuid() != 0:
        print("run as root on Node B (systemctl show, /proc and nsenter need it)", file=sys.stderr)
        return 2
    check_internal(args.expect_internal, args.with_credential)
    check_credential_delivery(args.expect_internal)
    check_units()
    check_containment()
    check_dolt(args.expect_dolt, args.with_credential)
    check_dolt_probe()
    check_paper_probe()
    check_modes()
    check_boot(args.allow_reboot_pending)
    if args.expect_guest_floor:
        check_guest_floor()
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
