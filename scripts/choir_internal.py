"""Host-internal call headers for host-side scripts, mirroring
hostauth.MarkInternal (internal/hostauth/hostauth.go): the marker header
always, and the internal service token only when the destination is a
loopback host on an internal service port. The token is never sent to a
guest (directly, or through a 127.0.0.1 DNAT port) or the internet. Guest
routes gate on the host source address and need only the marker.

docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md.
scripts/lib/choir_internal.sh is the shell twin; scripts/selftest-guest-token
proves both. Python 3 standard library only. density/lib.py prepends this
file to the collector it pipes to the host, so it defines names only.
"""
import shlex
import urllib.parse

TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
MARKER = {"X-Internal-Caller": "true"}
LOOPBACK_HOSTS = ("127.0.0.1", "::1", "localhost")
INTERNAL_PORTS = ("8083", "8084", "8086", "8087")


def token_allowed(url):
    """True only for 127.0.0.1, ::1 or localhost on 8083, 8084, 8086, 8087."""
    if "://" not in url:
        url = "http://" + url
    try:
        u = urllib.parse.urlsplit(url)
        port = u.port
    except ValueError:
        return False
    return u.hostname in LOOPBACK_HOSTS and str(port) in INTERNAL_PORTS


def internal_headers(url, header_file=TOKEN_HEADER_FILE):
    """Headers for a request this process sends to url: the marker, plus
    the token read from header_file when token_allowed(url) and the file
    is readable. The token is never printed."""
    h = dict(MARKER)
    if not token_allowed(url):
        return h
    try:
        with open(header_file) as f:
            name, _, value = f.read().strip().partition(":")
    except OSError:
        return h
    if name.strip() and value.strip():
        h[name.strip()] = value.strip()
    return h


def internal_curl_flags(url, header_file=TOKEN_HEADER_FILE):
    """curl flags for a command the host runs (over ssh): the marker, plus
    -H @header_file, when readable there, only when token_allowed(url).
    curl reads the file on the host; its contents never reach this process."""
    flags = "-H 'X-Internal-Caller: true'"
    if token_allowed(url):
        f = shlex.quote(header_file)
        flags += f" $(test -r {f} && echo -H @{f})"
    return flags
