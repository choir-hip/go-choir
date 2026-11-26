#!/usr/bin/env python3
"""Run pinned native Android model jobs on the Motorola, extending model-bench.

The ASR harness in run_android.py stays the speech-recognition path. This file
covers llama.cpp text and TTS jobs with the same safety rules: one named phone,
only /data/local/tmp/mbench-* work directories, no install/settings/reboot,
preflight, detached execution, VmHWM sampling and cleanup on every exit.
"""
import argparse
import csv
import hashlib
import json
import os
import re
import shutil
import sys
import tarfile
import tempfile
import time
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import parse
from run_android import Adb, check_preflight, getprop, harness_commit, read_state, run_detached, with_workdir
CACHE = os.path.expanduser("~/.cache/model-bench/android-models")
SPEC = os.path.join(HERE, "native_models.json")
COLUMNS = [
    "phone", "soc", "android", "runtime", "provider", "threads", "model_id", "quant",
    "model_bytes", "model_sha256", "runtime_sha256", "prefill_tps", "decode_tps",
    "audio_seconds", "process_seconds", "peak_rss_mib", "wall_seconds",
    "thermal_before", "thermal_after", "battery_temp_before", "battery_temp_after",
    "plugged", "mem_available_mib_before", "mem_available_mib_after", "run", "date",
    "harness_commit", "status", "notes",
]
WRAPPER = r"""#!/system/bin/sh
D=$1; TAG=$2; shift; shift
cd "$D"
export LD_LIBRARY_PATH="$D"
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


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def verify_artifact(path, digest, size=None):
    got = sha256_file(path)
    if got != digest:
        raise ValueError("SHA-256 mismatch for %s: got %s, want %s" % (path, got, digest))
    if size is not None and os.path.getsize(path) != size:
        raise ValueError("size mismatch for %s: got %d, want %d" % (path, os.path.getsize(path), size))
    return got


def fetch_artifact(url, destination, digest, size=None):
    """Download once, verify before rename, and leave no final file on mismatch."""
    if os.path.exists(destination):
        return verify_artifact(destination, digest, size)
    os.makedirs(os.path.dirname(destination), exist_ok=True)
    part = destination + ".part"
    try:
        with urllib.request.urlopen(url, timeout=120) as src, open(part, "wb") as dst:
            shutil.copyfileobj(src, dst)
        verify_artifact(part, digest, size)
        os.replace(part, destination)
    finally:
        if os.path.exists(part):
            os.remove(part)
    return digest


def safe_extract(archive, destination):
    root = os.path.realpath(destination)
    os.makedirs(root, exist_ok=True)
    with tarfile.open(archive, "r:*") as tf:
        for member in tf.getmembers():
            target = os.path.realpath(os.path.join(root, member.name))
            if target != root and not target.startswith(root + os.sep):
                raise ValueError("archive path escapes destination: %s" % member.name)
            if member.issym() or member.islnk():
                raise ValueError("archive links are refused: %s" % member.name)
        tf.extractall(root)


def parse_llama_bench(text):
    decoder = json.JSONDecoder()
    for i, ch in enumerate(text):
        if ch != "[":
            continue
        try:
            value, end = decoder.raw_decode(text[i:])
        except ValueError:
            continue
        if isinstance(value, list) and all(isinstance(x, dict) for x in value):
            return value
    raise ValueError("no llama-bench JSON array in output")


def parse_tts(text):
    total = re.search(r"timings: .* = total ([0-9.]+)s", text)
    audio = re.search(r"output audio = ([0-9.]+)s", text)
    if not total or not audio:
        raise ValueError("no llama-tts timings in output")
    return {"process_seconds": float(total.group(1)), "audio_seconds": float(audio.group(1))}


def job_command(job, threads, repeats):
    exe = job["executable"]
    if not re.fullmatch(r"[A-Za-z0-9._-]+", exe):
        raise ValueError("executable must be a plain file name: %r" % exe)
    out = ["./" + exe]
    for arg in job.get("args", []):
        out.append(str(arg).replace("{threads}", str(threads)).replace("{repeats}", str(repeats)))
    if any("{" in arg or "}" in arg for arg in out):
        raise ValueError("unexpanded command argument: %r" % out)
    return out


def append_row(path, row):
    unknown = set(row) - set(COLUMNS)
    if unknown:
        raise KeyError("unknown column(s): %s" % sorted(unknown))
    new = not os.path.exists(path) or os.path.getsize(path) == 0
    with open(path, "a", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=COLUMNS, delimiter="\t", lineterminator="\n", extrasaction="ignore")
        if new:
            writer.writeheader()
        writer.writerow({key: row.get(key, "") for key in COLUMNS})


def load_spec(path=SPEC):
    with open(path) as f:
        return json.load(f)


def fetch_all(spec, cache=CACHE):
    runtime = spec["runtime"]
    archive = os.path.join(cache, "llama-runtime", "llama-b12497-bin-android-arm64.tar.gz")
    fetch_artifact(runtime["url"], archive, runtime["archive_sha256"], runtime["archive_bytes"])
    runtime_dir = os.path.join(cache, "llama-runtime", "generic", "llama-b12497")
    if not os.path.exists(os.path.join(runtime_dir, "llama-bench")):
        safe_extract(archive, os.path.join(cache, "llama-runtime", "generic"))
    for job in spec["jobs"].values():
        for name, artifact in job["files"].items():
            fetch_artifact(artifact["url"], os.path.join(cache, job["dir"], name), artifact["sha256"], artifact["bytes"])


def _verify_job(spec, job, cache):
    runtime = spec["runtime"]
    archive = os.path.join(cache, "llama-runtime", "llama-b12497-bin-android-arm64.tar.gz")
    runtime_hash = verify_artifact(archive, runtime["archive_sha256"], runtime["archive_bytes"])
    runtime_dir = os.path.join(cache, "llama-runtime", "generic", "llama-b12497")
    model_dir = os.path.join(cache, job["dir"])
    model_hash = hashlib.sha256()
    model_bytes = 0
    for name, artifact in job["files"].items():
        path = os.path.join(model_dir, name)
        got = verify_artifact(path, artifact["sha256"], artifact["bytes"])
        model_hash.update(name.encode() + b"\0" + bytes.fromhex(got))
        model_bytes += os.path.getsize(path)
    return runtime_dir, model_dir, runtime_hash, model_hash.hexdigest(), model_bytes


def run_job(args):
    spec = load_spec()
    job = spec["jobs"][args.job]
    runtime_dir, model_dir, runtime_hash, model_hash, model_bytes = _verify_job(spec, job, args.cache)
    adb = Adb()
    adb.shell("true")
    info = {
        "phone": getprop(adb, "ro.product.model"), "soc": getprop(adb, "ro.soc.model"),
        "android": "%s (API %s, %s)" % (getprop(adb, "ro.build.version.release"), getprop(adb, "ro.build.version.sdk"), getprop(adb, "ro.build.display.id")),
    }
    d = "/data/local/tmp/mbench-native-%s-%d" % (re.sub(r"[^A-Za-z0-9_.-]", "-", args.job), int(time.time()))
    commit = harness_commit()
    os.makedirs(args.out_dir, exist_ok=True)

    def work():
        for name in os.listdir(runtime_dir):
            src = os.path.join(runtime_dir, name)
            if os.path.isfile(src):
                adb.push(src, d + "/")
        for name in job["files"]:
            adb.push(os.path.join(model_dir, name), d + "/")
        wrapper = os.path.join(args.out_dir, "run.sh")
        with open(wrapper, "w", newline="\n") as f:
            f.write(WRAPPER)
        adb.push(wrapper, d + "/run.sh")
        adb.shell("chmod 755 %s/llama* %s/ggml-rpc-server %s/run.sh" % (d, d, d))
        command = job_command(job, args.threads, args.repeats)
        th0, b0 = read_state(adb)
        check_preflight(th0, b0["temp_c"], b0["level"])
        run_detached(adb, d, "run", command, args.timeout)
        th1, b1 = read_state(adb)
        for ext in ("out", "err", "meta"):
            adb.pull("%s/run.%s" % (d, ext), os.path.join(args.out_dir, "run." + ext))
        for name in job.get("outputs", []):
            adb.pull("%s/%s" % (d, name), os.path.join(args.out_dir, name))
        meta = parse.parse_meta(open(os.path.join(args.out_dir, "run.meta")).read())
        out = open(os.path.join(args.out_dir, "run.out"), errors="replace").read()
        err = open(os.path.join(args.out_dir, "run.err"), errors="replace").read()
        status = "ok" if meta["exit"] == 0 else ("killed" if meta["exit"] == 137 else "error")
        notes = []
        if status != "ok":
            notes.append("exit %d: %s" % (meta["exit"], (err or out).strip()[-300:]))
        row = {
            "phone": info["phone"], "soc": info["soc"], "android": info["android"],
            "runtime": spec["runtime"]["name"], "provider": "cpu", "threads": args.threads,
            "model_id": args.job, "quant": job["quant"], "model_bytes": model_bytes,
            "model_sha256": model_hash, "runtime_sha256": runtime_hash,
            "peak_rss_mib": "%.1f" % (meta["vmhwm_kb"] / 1024.0), "wall_seconds": meta["wall_seconds"],
            "thermal_before": th0, "thermal_after": th1, "battery_temp_before": b0["temp_c"],
            "battery_temp_after": b1["temp_c"], "plugged": b1["plugged"],
            "mem_available_mib_before": "%.0f" % b0["mem_available_mib"], "mem_available_mib_after": "%.0f" % b1["mem_available_mib"],
            "run": args.repeats, "date": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "harness_commit": commit,
            "status": status,
        }
        if status == "ok" and job["parser"] == "llama_bench":
            values = parse_llama_bench(out + "\n" + err)
            prompt = next((x for x in values if x.get("n_prompt", 0) > 0), None)
            decode = next((x for x in values if x.get("n_gen", 0) > 0), None)
            if prompt:
                row["prefill_tps"] = "%.3f" % prompt["avg_ts"]
            if decode:
                row["decode_tps"] = "%.3f" % decode["avg_ts"]
            if args.json_out:
                with open(args.json_out, "w") as f:
                    json.dump(values, f, indent=2)
        elif status == "ok" and job["parser"] == "tts":
            row.update(parse_tts(out + "\n" + err))
            for name in job.get("outputs", []):
                notes.append("%s sha256=%s bytes=%d" % (name, sha256_file(os.path.join(args.out_dir, name)), os.path.getsize(os.path.join(args.out_dir, name))))
        else:
            row["status"] = "error"
            notes.append("parser did not produce the job's metrics")
        row["notes"] = "; ".join(notes)
        append_row(args.tsv, row)
        print({key: row.get(key, "") for key in ("model_id", "prefill_tps", "decode_tps", "audio_seconds", "peak_rss_mib", "status")})

    with_workdir(adb, d, work)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--job", required=True, help="a key of native_models.json")
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--repeats", type=int, default=2)
    ap.add_argument("--timeout", type=int, default=600)
    ap.add_argument("--cache", default=CACHE)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--tsv", required=True)
    ap.add_argument("--json-out")
    args = ap.parse_args(argv)
    if args.threads < 1 or args.repeats < 1:
        ap.error("threads and repeats must be positive")
    run_job(args)


if __name__ == "__main__":
    main()
