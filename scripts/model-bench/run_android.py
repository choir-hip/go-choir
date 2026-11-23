#!/usr/bin/env python3
"""Run one model on the Motorola over adb and append rows to a TSV. See README.md.

The phone is touched only under /data/local/tmp/mbench-<id>/, which is removed
at the end of every run, on failure and on Ctrl-C too. No install, no settings,
no reboot, nothing is killed except the one process this script started.
"""
import argparse
import json
import os
import re
import shlex
import subprocess
import sys
import time

import fixtures
import parse
import rows
import wer

SERIAL = "ZY22H5K8QR"  # the only phone this script may address; never take it from the command line
ADB = os.path.expanduser("~/Library/Android/sdk/platform-tools/adb")
CACHE = os.path.expanduser("~/.cache/model-bench")
HERE = os.path.dirname(os.path.abspath(__file__))
TMP = "/data/local/tmp"
RUNTIME_NAME = "sherpa-onnx-v2.0.3-android-aarch64-termux-shared"
LIBCXX_GLOB_DIR = "toolchains/llvm/prebuilt"


class PhoneNotReady(Exception):
    pass


class Adb:
    def __init__(self, serial=SERIAL, adb=ADB):
        self.base = [adb, "-s", serial]

    def _run(self, args, timeout=None):
        # The USB link dropped once for about a minute during a run (the phone stayed up):
        # retry when adb says the device is not there, nothing else.
        for attempt in range(12):
            p = subprocess.run(self.base + args, capture_output=True, text=True, timeout=timeout)
            msg = p.stderr or p.stdout
            if p.returncode != 0 and ("not found" in msg or "offline" in msg) and attempt < 11:
                time.sleep(5)
                continue
            break
        if p.returncode != 0:
            raise RuntimeError("adb %s failed (%d): %s" % (" ".join(args[:2]), p.returncode, msg[-500:]))
        return p.stdout

    def shell(self, cmd, timeout=None):
        return self._run(["shell", cmd], timeout)

    def push(self, src, dst):
        self._run(["push", src, dst])

    def pull(self, src, dst):
        self._run(["pull", src, dst])


DIR_RE = re.compile(r"^/data/local/tmp/mbench-[A-Za-z0-9_.-]+$")


def remove_dir(adb, path):
    """Delete only a directory of ours; refuse anything else, including '..'."""
    if not DIR_RE.match(path) or ".." in path:
        raise ValueError("refusing to remove %r: not a /data/local/tmp/mbench-<id> directory" % path)
    adb.shell("rm -rf %s" % path)
    left = adb.shell("ls %s" % TMP)
    if path.rsplit("/", 1)[1] in left.split():
        raise RuntimeError("cleanup failed: %s is still on the phone" % path)


def with_workdir(adb, path, fn):
    if not DIR_RE.match(path) or ".." in path:
        raise ValueError("bad work directory %r" % path)
    adb.shell("mkdir -p %s" % path)
    try:
        return fn()
    finally:
        remove_dir(adb, path)


def check_preflight(thermal_status, battery_temp_c, battery_level):
    if thermal_status >= 2:
        raise PhoneNotReady("thermal status %d (moderate or worse): stop" % thermal_status)
    if battery_temp_c > 40.0:
        raise PhoneNotReady("battery temperature %.1f C is above 40" % battery_temp_c)
    if battery_level < 30:
        raise PhoneNotReady("battery level %d%% is under 30" % battery_level)


WRAPPER = r"""#!/system/bin/sh
# run.sh <workdir> <tag> <command...>: run the command as a child, sample its VmHWM
# every 50 ms, write <tag>.out/.err/.meta. Peak RSS is the last VmHWM read (it only grows).
D=$1; TAG=$2; shift; shift
cd "$D"
export LD_PRELOAD="$D/lib/libhashshim.so" LD_LIBRARY_PATH="$D/lib"
t0=$(date +%s.%N)
"$@" > "$TAG.out" 2> "$TAG.err" &
pid=$!
echo $pid > "$D/pid"
peak=0
while kill -0 $pid 2>/dev/null; do
  while read k v u; do
    if [ "$k" = "VmHWM:" ]; then peak=$v; fi
  done < /proc/$pid/status 2>/dev/null
  sleep 0.05
done
wait $pid
rc=$?
t1=$(date +%s.%N)
echo "exit=$rc wall_start=$t0 wall_end=$t1 vmhwm_kb=$peak" > "$TAG.meta"
"""


def sh_quote_all(parts):
    return " ".join(shlex.quote(p) for p in parts)


def getprop(adb, name):
    return adb.shell("getprop %s" % name).strip()


def read_state(adb):
    th = parse.thermal_status(adb.shell("dumpsys thermalservice | grep -m1 'Thermal Status'"))
    bat = parse.battery(adb.shell("dumpsys battery"))
    mem = int(adb.shell("grep MemAvailable /proc/meminfo").split()[1]) / 1024.0
    bat["mem_available_mib"] = mem
    return th, bat


def run_detached(adb, d, tag, cmd, timeout):
    """Start the wrapper detached on the phone and poll for its .meta file, so a USB
    drop on the Mac side cannot kill the run; adb errors while polling are retried."""
    adb.shell("cd %s && rm -f %s.meta && (nohup sh %s/run.sh %s %s %s >/dev/null 2>&1 &)"
              % (d, tag, d, d, tag, sh_quote_all(cmd)))
    deadline = time.time() + timeout
    while time.time() < deadline:
        time.sleep(3)
        try:
            if adb.shell("[ -f %s/%s.meta ] && echo yes" % (d, tag)).strip() == "yes":
                return
        except (RuntimeError, subprocess.TimeoutExpired):
            time.sleep(5)  # the device may be re-enumerating; keep polling
    kill_started(adb, d)
    raise TimeoutError("run %s did not finish in %d s" % (tag, timeout))


def harness_commit():
    try:
        sha = subprocess.run(["git", "-C", HERE, "rev-parse", "--short", "HEAD"], capture_output=True, text=True).stdout.strip()
        dirty = subprocess.run(["git", "-C", HERE, "status", "--porcelain", "--", HERE], capture_output=True, text=True).stdout.strip()
        return sha + ("+dirty" if dirty else "")
    except OSError:
        return ""


def find_libcxx():
    base = os.path.expanduser("~/Library/Android/sdk/ndk")
    for v in sorted(os.listdir(base), reverse=True):
        for root, _, files in os.walk(os.path.join(base, v, LIBCXX_GLOB_DIR)):
            if root.endswith("aarch64-linux-android") and "libc++_shared.so" in files:
                return os.path.join(root, "libc++_shared.so")
    raise RuntimeError("libc++_shared.so not found in the NDK")


def kill_started(adb, d):
    """Kill the one process this script started (its pid is in <dir>/pid)."""
    try:
        adb.shell("[ -f %s/pid ] && kill $(cat %s/pid)" % (d, d))
    except RuntimeError:
        pass


def record(args, m, info, items, audio_s, model_bytes, commit, threads, rep, parts, th0, b0, th1, b1):
    """parts: [(local dir, tag, chunk items)], one per process run (the set is run in chunks,
    because sherpa-onnx-offline decodes all given files as one batch and 100 at once
    panicked the phone's kernel: see the evidence README)."""
    notes = []
    status = "ok"
    metas, ps, texts = [], [], []
    for local, tag, chunk in parts:
        meta = parse.parse_meta(open(os.path.join(local, tag + ".meta")).read())
        out = open(os.path.join(local, tag + ".out"), errors="replace").read()
        err = open(os.path.join(local, tag + ".err"), errors="replace").read()
        p = parse.parse_sherpa(out + "\n" + err)
        if meta["exit"] != 0:
            status = "killed" if meta["exit"] == 137 else "error"
            notes.append("%s exit %d: %s" % (tag, meta["exit"], (err or out).strip()[-200:]))
        metas.append(meta)
        ps.append(p)
        texts += p["texts"]
    wall = sum(x["wall_seconds"] for x in metas)
    peak_kb = max(x["vmhwm_kb"] for x in metas)
    loads = [p["load_seconds"] for p in ps]
    load = sum(loads) / len(loads) if all(x is not None for x in loads) else None
    decs = [p["decode_seconds"] for p in ps]
    if all(x is not None for x in decs):
        decode = round(sum(decs), 3)
    elif load is not None:
        decode = round(wall - sum(loads), 3)
        notes.append("decode = wall - load (online binary prints 2 digits)")
    else:
        decode = None
    if len(parts) > 1:
        notes.append("%d chunks of up to %d utterances, one process each: load is the mean, decode and wall are sums, peak RSS is the largest" % (len(parts), args.chunk))
    meta = {"vmhwm_kb": peak_kb, "wall_seconds": round(wall, 3)}
    p = {"load_seconds": round(load, 3) if load is not None else None, "texts": texts}
    row = {
        "phone": info["phone"], "soc": info["soc"], "android": info["android"],
        "runtime": "sherpa-onnx v2.0.3 android-aarch64-termux-shared + hash_memory shim", "provider": "cpu",
        "threads": threads, "model_id": args.model, "quant": m["quant"], "model_bytes": model_bytes,
        "set": args.set, "utterances": len(items), "audio_seconds": "%.3f" % audio_s,
        "load_seconds": p["load_seconds"] if p["load_seconds"] is not None else "",
        "decode_seconds": decode if decode is not None else "",
        "rtf": "%.4f" % (decode / audio_s) if decode is not None else "",
        "peak_rss_mib": "%.1f" % (meta["vmhwm_kb"] / 1024.0), "wall_seconds": meta["wall_seconds"],
        "thermal_before": th0, "thermal_after": th1, "battery_temp_before": b0["temp_c"],
        "battery_temp_after": b1["temp_c"], "plugged": b1["plugged"], "run": rep,
        "mem_available_mib_before": "%.0f" % b0["mem_available_mib"], "mem_available_mib_after": "%.0f" % b1["mem_available_mib"],
        "date": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "harness_commit": commit, "status": status,
    }
    if status == "ok":
        if len(p["texts"]) != len(items):
            row["status"] = "error"
            notes.append("%d transcripts for %d utterances" % (len(p["texts"]), len(items)))
        else:
            pairs = [(i["text"], t) for i, t in zip(items, p["texts"])]
            tot = [wer.errors(r, h) for r, h in pairs]
            s, dl, n, rw = (sum(getattr(e, k) for e in tot) for k in ("subs", "dels", "ins", "ref_words"))
            row.update({"wer": "%.4f" % ((s + dl + n) / rw), "subs": s, "dels": dl, "ins": n, "ref_words": rw})
            if args.transcripts:
                os.makedirs(args.transcripts, exist_ok=True)
                path = os.path.join(args.transcripts, "%s-%s-t%d-r%d.tsv" % (args.model, args.set, threads, rep))
                with open(path, "w") as f:
                    for i, t in zip(items, p["texts"]):
                        f.write("%s\t%s\n" % (i["id"], t.replace("\t", " ").replace("\n", " ")))
    row["notes"] = "; ".join(notes)
    rows.append_row(args.tsv, row)
    print({k: row.get(k, "") for k in ("model_id", "set", "threads", "run", "wer", "rtf", "load_seconds", "peak_rss_mib", "status")})




def bench(args, adb=None):
    adb = adb or Adb()
    models = json.load(open(os.path.join(HERE, "models.json")))
    m = models[args.model]
    manifest = fixtures.read_manifest(os.path.join(args.fixtures, "manifest.tsv"))
    items = manifest[:10] if args.set == "smoke" else manifest
    audio_s = sum(float(i["duration_s"]) for i in items)
    for i in items:  # the fixture must still be the one the manifest describes
        if fixtures.sha256_file(os.path.join(args.fixtures, "wav", i["id"] + ".wav")) != i["sha256"]:
            raise RuntimeError("fixture %s changed since the manifest was written" % i["id"])

    adb.shell("true")
    info = {
        "phone": getprop(adb, "ro.product.model"), "soc": getprop(adb, "ro.soc.model"),
        "android": "%s (API %s, %s)" % (getprop(adb, "ro.build.version.release"), getprop(adb, "ro.build.version.sdk"),
                                         getprop(adb, "ro.build.display.id")),
    }
    mdir = os.path.join(CACHE, "models", m["archive"], m["archive"])
    model_bytes = sum(os.path.getsize(os.path.join(mdir, f)) for f in m["files"])
    rt = os.path.join(CACHE, "runtime", "v2.0.3", RUNTIME_NAME)
    shim = os.path.join(CACHE, "runtime", "libhashshim.so")
    d = "%s/mbench-%s-%d" % (TMP, args.model.split("-")[0][:12], int(time.time()))
    commit = harness_commit()

    def work():
        adb.shell("mkdir -p %s/bin %s/lib %s/m %s/wav" % (d, d, d, d))
        wrapper = os.path.join(CACHE, "run.sh")
        with open(wrapper, "w", newline="\n") as f:
            f.write(WRAPPER)
        adb.push(wrapper, d + "/run.sh")
        adb.push(os.path.join(rt, "bin", m["binary"]), d + "/bin/")
        for lib in ["libonnxruntime.so", "libsherpa-onnx-c-api.so", "libsherpa-onnx-cxx-api.so"]:
            adb.push(os.path.join(rt, "lib", lib), d + "/lib/")
        adb.push(find_libcxx(), d + "/lib/")
        adb.push(shim, d + "/lib/")
        for fn in m["files"]:
            adb.push(os.path.join(mdir, fn), d + "/m/")
        for i in items:
            adb.push(os.path.join(args.fixtures, "wav", i["id"] + ".wav"), d + "/wav/")
        adb.shell("chmod +x %s/bin/* %s/run.sh" % (d, d))
        base = ["./bin/" + m["binary"], "--num-threads=0"] + [a.replace("{m}", "m") for a in m["args"]]
        chunks = [items[k:k + args.chunk] for k in range(0, len(items), args.chunk)]
        for threads in args.threads:
            for rep in range(1, args.repeats + 1):
                parts = []
                th0 = b0 = None
                for ci, chunk in enumerate(chunks):
                    tag = "%s_t%d_r%d_c%d" % (args.set, threads, rep, ci)
                    time.sleep(args.cooldown if ci == 0 else 0)
                    st = read_state(adb)
                    check_preflight(st[0], st[1]["temp_c"], st[1]["level"])
                    if ci == 0:
                        th0, b0 = st
                    c = list(base)
                    c[1] = "--num-threads=%d" % threads
                    c += ["wav/%s.wav" % i["id"] for i in chunk]
                    run_detached(adb, d, tag, c, args.timeout)
                    local = os.path.join(args.work, "%s-%s" % (tag, args.model))
                    os.makedirs(local, exist_ok=True)
                    for ext in ("out", "err", "meta"):
                        adb.pull("%s/%s.%s" % (d, tag, ext), os.path.join(local, "%s.%s" % (tag, ext)))
                    parts.append((local, tag, chunk))
                th1, b1 = read_state(adb)
                record(args, m, info, items, audio_s, model_bytes, commit, threads, rep, parts, th0, b0, th1, b1)

    with_workdir(adb, d, work)

def main(argv):
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--model", required=True, help="a key of models.json")
    ap.add_argument("--set", choices=["smoke", "full"], default="smoke")
    ap.add_argument("--threads", default="4", help="comma list, e.g. 1,2,4,6")
    ap.add_argument("--repeats", type=int, default=2)
    ap.add_argument("--cooldown", type=float, default=20, help="seconds before each run")
    ap.add_argument("--chunk", type=int, default=10, help="utterances per process run (see README)")
    ap.add_argument("--timeout", type=int, default=1800)
    ap.add_argument("--fixtures", default=os.path.join(CACHE, "fixtures"))
    ap.add_argument("--tsv", required=True)
    ap.add_argument("--transcripts", help="directory for per-run transcripts (hypotheses)")
    ap.add_argument("--work", default=os.path.join(CACHE, "pulled"), help="where raw pulled output goes on the Mac")
    a = ap.parse_args(argv)
    a.threads = [int(x) for x in a.threads.split(",")]
    bench(a)


if __name__ == "__main__":
    main(sys.argv[1:])
