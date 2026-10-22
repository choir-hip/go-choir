"""Host-internal call headers for host-side scripts, after
hostauth.MarkInternal (internal/hostauth/hostauth.go) but narrower: the
marker header always, and the internal service token only when the
destination is 127.0.0.1, ::1 or localhost on an internal service port
(MarkInternal accepts all of 127.0.0.0/8 and any case of localhost). The
token is never sent to a guest (directly, or through a 127.0.0.1 DNAT port)
or the internet. Guest routes gate on the host source address and need only
the marker. internal_urlopen sends a token-bearing request with no proxy
and refuses its redirects.

docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md.
scripts/lib/choir_internal.sh is the shell twin; scripts/selftest-guest-token
proves both. Python 3 standard library only. node_b_host_reboot.py prepends
this file to the script it pipes to the host, and density/host_collect.py
carries a verbatim copy, so it defines names only.
"""
import shlex
import urllib.parse
import urllib.request

TOKEN_HEADER_FILE = "/var/lib/go-choir/secrets/internal-service-token.header"
MARKER = {"X-Internal-Caller": "true"}
LOOPBACK_HOSTS = ("127.0.0.1", "::1", "localhost")
INTERNAL_PORTS = ("8083", "8084", "8086", "8087")
TOKEN_HEADER_NAME = "X-Choir-Internal-Token"


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


class _RefuseRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # urllib raises HTTPError: the token never follows a redirect


def internal_urlopen(req, timeout=None):
    """urlopen, except that a request carrying the token goes direct (no
    proxy from the environment) and a redirect is an error, not followed."""
    if not req.has_header(TOKEN_HEADER_NAME.capitalize()):
        return urllib.request.urlopen(req, timeout=timeout)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), _RefuseRedirect())
    return opener.open(req, timeout=timeout)


def internal_curl_flags(url, header_file=TOKEN_HEADER_FILE):
    """curl flags for a command the host runs (over ssh): the marker, plus
    -H @header_file, when readable there, only when token_allowed(url);
    then also no proxy, and an error if -L would follow a redirect.
    curl reads the file on the host; its contents never reach this process."""
    flags = "-H 'X-Internal-Caller: true'"
    if token_allowed(url):
        f = shlex.quote(header_file)
        flags += f" --noproxy '*' --max-redirs 0 $(test -r {f} && echo -H @{f})"
    return flags
