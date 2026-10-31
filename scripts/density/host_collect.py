#!/usr/bin/env python3
"""Read-only memory collector for Choir density measurements (runs ON the host).

usage: ssh node-b 'python3 - [flags]' < scripts/density/host_collect.py
       python3 scripts/density/host_collect.py [flags]            (on the host)

flags:
  --vmctl URL        vmctl base URL (default http://localhost:8083)
  --no-list          skip vmctl's /internal/vmctl/list
  --procmem IDS      comma-separated vm ids whose guest procmem to read
                     (host-sourced route GET :8085/internal/debug/pprof/procmem)
  --fincore          host fincore of the store image and of each listed vm's data.img
  --light            only meminfo, PSI, CPU window and the vmctl list (safety polls)
  --cpu-window S     host CPU window in seconds (default 2)

Everything is a read: /proc/meminfo, /proc/pressure/memory, /proc/stat,
/sys/kernel/mm/ksm/*, /proc/<pid>/{stat,status} of Firecracker processes (the
cmdline only to find the --id), vmctl's list, the guest procmem route, and
fincore (mincore: it reads page-cache residency, not file data). The internal
token header is read from the host file and sent only to a loopback internal
port (localhost vmctl), never to a guest: the guest procmem route gets the
marker only. It is never printed. Output: one JSON object.
"""
import glob, json, os, shutil, subprocess, sys, time, urllib.parse, urllib.request

HZ = os.sysconf("SC_CLK_TCK")
STATE_DIR = "/var/lib/go-choir/vm-state"
STORE_IMAGE = "/var/lib/go-choir/guest/storedisk.erofs"


def arg(name, default=None, flag=False):
    if name in sys.argv:
        return True if flag else sys.argv[sys.argv.index(name) + 1]
    return False if flag else default


def kib_fields(path):
    out = {}
    try:
        for line in open(path):
            k, _, rest = line.partition(":")
            parts = rest.split()
            if parts and parts[0].lstrip("-").isdigit():
                out[k.strip()] = int(parts[0])
    except OSError as e:
        out["error"] = str(e)
    return out


def psi():
    out = {}
    try:
        for line in open("/proc/pressure/memory"):
            kind, *kv = line.split()
            out[kind] = {k: float(v) for k, v in (x.split("=") for x in kv)}
    except OSError as e:
        out["error"] = str(e)
    return out


def cpu_total():
    f = open("/proc/stat").readline().split()[1:]
    v = [int(x) for x in f]
    return sum(v), v[3] + (v[4] if len(v) > 4 else 0)  # total, idle+iowait


def ksm():
    out = {}
    for p in sorted(glob.glob("/sys/kernel/mm/ksm/*")):
        try:
            out[os.path.basename(p)] = open(p).read().strip()
        except OSError:
            pass
    return out


# A copy of scripts/choir_internal.py (token_allowed .. internal_urlopen), so this
# file still runs piped alone to the host; scripts/selftest-guest-token
# fails if the copy differs.
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


def http_json(url, timeout=8):
    req = urllib.request.Request(url, headers=internal_headers(url))
    try:
        return json.load(internal_urlopen(req, timeout=timeout))
    except Exception as e:  # noqa: BLE001 - reported, never raised
        return {"error": f"{type(e).__name__}: {e}"[:200]}


def fc_processes():
    procs = {}
    for d in glob.glob("/proc/[0-9]*"):
        try:
            comm = open(d + "/comm").read().strip()
            if comm != "firecracker":
                continue
            toks = open(d + "/cmdline").read().split("\0")
            vm = toks[toks.index("--id") + 1] if "--id" in toks else ""
            st = kib_fields(d + "/status")
            s = open(d + "/stat").read()
            f = s[s.rfind(")") + 2:].split()
            procs[vm or d] = {
                "pid": int(d.split("/")[-1]),
                "rss_kib": st.get("VmRSS"), "rss_anon_kib": st.get("RssAnon"),
                "rss_file_kib": st.get("RssFile"), "rss_shmem_kib": st.get("RssShmem"),
                "hwm_kib": st.get("VmHWM"),
                "cpu_ticks": int(f[11]) + int(f[12]),
            }
        except (OSError, ValueError, IndexError):
            continue
    return procs


def guest_ip(vm):
    try:
        c = json.load(open(f"{STATE_DIR}/{vm}/fc-config.json"))
        for t in c["boot-source"]["boot_args"].split():
            if t.startswith("ip="):
                return t[3:].split("::")[0]
    except (OSError, KeyError, ValueError):
        pass
    return ""


def fincore(path):
    exe = shutil.which("fincore")
    if not exe:
        return {"error": "fincore not installed on the host"}
    try:
        r = subprocess.run([exe, "--bytes", "--noheadings", "--output", "RES,SIZE", path],
                           capture_output=True, text=True, timeout=60)
        parts = r.stdout.split()
        if r.returncode == 0 and len(parts) >= 2:
            return {"resident_bytes": int(parts[0]), "size_bytes": int(parts[1])}
        return {"error": (r.stderr or "no output").strip()[:200]}
    except (OSError, subprocess.SubprocessError, ValueError) as e:
        return {"error": str(e)[:200]}


def main():
    light = arg("--light", flag=True)
    win = float(arg("--cpu-window", "2"))
    t0, i0 = cpu_total()
    fc0 = fc_processes() if not light else {}
    time.sleep(win)
    t1, i1 = cpu_total()
    out = {
        "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "host_cpu_busy_fraction": round(1 - (i1 - i0) / max(t1 - t0, 1), 4),
        "meminfo_kib": kib_fields("/proc/meminfo"),
        "psi_memory": psi(),
    }
    vmctl = arg("--vmctl", "http://localhost:8083")
    listing = None
    if not arg("--no-list", flag=True):
        listing = http_json(vmctl + "/internal/vmctl/list")
        out["vmctl_list"] = listing
    if light:
        print(json.dumps(out))
        return
    out["ksm"] = ksm()
    out["firecracker"] = fc_processes()
    for vm, p in out["firecracker"].items():
        if vm in fc0:
            p["cpu_cores_over_window"] = round((p["cpu_ticks"] - fc0[vm]["cpu_ticks"]) / HZ / win, 4)
    pm = {}
    for vm in [x for x in (arg("--procmem", "") or "").split(",") if x]:
        ip = guest_ip(vm)
        pm[vm] = http_json(f"http://{ip}:8085/internal/debug/pprof/procmem") if ip else {"error": "no guest address"}
    if pm:
        out["guest_procmem"] = pm
    if arg("--fincore", flag=True):
        fc = {"store_image": {"path": STORE_IMAGE, **fincore(STORE_IMAGE)}}
        # A layered image has no storedisk.erofs: report its two layers (the
        # base is shared by every VM and release that names it).
        for layer in ("app", "base"):
            lp = f"/var/lib/go-choir/guest/storedisk-{layer}.erofs"
            if os.path.exists(lp):
                fc[f"store_{layer}_layer"] = {"path": os.path.realpath(lp), **fincore(lp)}
        vms = list(out["firecracker"].keys())
        fc["data_images"] = {vm: fincore(f"{STATE_DIR}/{vm}/data.img") for vm in vms
                             if os.path.exists(f"{STATE_DIR}/{vm}/data.img")}
        out["host_fincore"] = fc
    print(json.dumps(out))


if __name__ == "__main__":
    main()
