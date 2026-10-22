"""Shared pieces of the density scripts: the host runner, the vmctl reads and
the guard that keeps every VM-creating or VM-stopping step off protected
computers. Python 3 standard library only."""
import json
import os
import shlex
import subprocess

HERE = os.path.dirname(os.path.abspath(__file__))
COLLECTOR = os.path.join(HERE, "host_collect.py")
TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
# The delegate computer (engineering-desk self-development only; FP0 metamission).
DELEGATE_COMPUTER_ID = "computer-ad1b46d4c20129e84b9e239cf4720fb9"
# The only warmness class a creating step may touch (vmctl warmness_policy.go).
TEST_CLASS = "test"
GIB = 1 << 30


class Host:
    """Runs the collector and vmctl calls on the host: over ssh, or locally."""

    def __init__(self, ssh=None):
        self.ssh = ssh  # None means this machine is the host

    def sh(self, cmd, stdin=None, timeout=300):
        argv = ["ssh", self.ssh, cmd] if self.ssh else ["bash", "-c", cmd]
        r = subprocess.run(argv, input=stdin, capture_output=True, text=True, timeout=timeout)
        return r.stdout

    def collect(self, *flags, timeout=300):
        with open(COLLECTOR) as f:
            src = f.read()
        out = self.sh("python3 - " + " ".join(shlex.quote(x) for x in flags), stdin=src, timeout=timeout)
        try:
            return json.loads(out)
        except json.JSONDecodeError:
            return {"error": "collector returned non-JSON", "body": out[:300]}

    def vmctl(self, method, path, body=None):
        auth = f"-H 'X-Internal-Caller: true' $(test -r {TOKEN_HEADER_FILE} && echo -H @{TOKEN_HEADER_FILE})"
        data = ""
        if body is not None:
            data = "-H 'Content-Type: application/json' -d " + shlex.quote(json.dumps(body))
        out = self.sh(f"curl -s -X{method} {auth} {data} 'localhost:8083{path}'")
        try:
            return json.loads(out)
        except json.JSONDecodeError:
            return {"error": "non-json", "body": out[:300]}


def ownerships(listing):
    return (listing or {}).get("ownerships") or []


def refusal(own, protect_ids=(), delegate_id=DELEGATE_COMPUTER_ID):
    """Why a creating or stopping step must not touch this ownership; "" if it may.

    Allowlist, not denylist: only a computer vmctl classifies as warmness
    class "test" passes, so a renamed paper or owner computer is still
    refused. The delegate id and any --protect-id are refused even if
    a misconfiguration ever classified them as test."""
    cid = (own or {}).get("computer_id", "")
    if not cid:
        return "no computer id"
    if cid == delegate_id:
        return "the delegate computer"
    if cid in set(protect_ids):
        return "named by --protect-id"
    if own.get("held"):
        return "held"
    cls = own.get("warmness_class", "")
    if cls != TEST_CLASS:
        return f"warmness class {cls or 'unknown'!r} is not 'test' (the paper, the owner computer and every paid or platform computer are refused)"
    return ""


def require_test(own, protect_ids=()):
    why = refusal(own, protect_ids)
    if why:
        raise SystemExit(f"refused: {(own or {}).get('computer_id', '?')}: {why}")


def safety_stop(snapshot, floor_gib, psi_stop=1.0, cpu_stop=0.8):
    """The report's safety stops that one sample can decide ("" if none).
    CPU above cpu_stop must hold for 60 s, so the caller counts it itself."""
    psi = ((snapshot.get("psi_memory") or {}).get("some") or {}).get("avg10", 0)
    if psi > psi_stop:
        return f"memory PSI some avg10 {psi} > {psi_stop}"
    avail = (snapshot.get("meminfo_kib") or {}).get("MemAvailable")
    if avail is not None and avail * 1024 < (floor_gib + 2) * GIB:
        return f"MemAvailable {avail * 1024 / GIB:.2f} GiB below floor {floor_gib} + 2 GiB"
    return ""


def non_test_active(listing, protect_ids=()):
    """computer ids of active non-test computers, to detect a reclaim of one."""
    return {o["computer_id"] for o in ownerships(listing)
            if o.get("computer_id") and o.get("state") == "active" and refusal(o, protect_ids)}
