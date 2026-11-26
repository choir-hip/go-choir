#!/usr/bin/env python3
"""Run the exact Silero v6.2.1 ONNX model on the Motorola with sherpa-onnx."""
import argparse
import json
import os
import re
import sys
import time
import wave

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import native
from run_android import Adb, check_preflight, find_libcxx, getprop, harness_commit, read_state, run_detached, with_workdir

BENCH = os.path.expanduser("~/.cache/model-bench")
MODEL = os.path.join(BENCH, "android-models", "silero", "silero_vad_16k_op15.onnx")
WRAPPER = r"""#!/system/bin/sh
D=$1; TAG=$2; shift; shift
cd "$D"
export LD_PRELOAD="$D/libhashshim.so" LD_LIBRARY_PATH="$D"
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


def parse_segments(text):
    segments = [(float(a), float(b)) for a, b in re.findall(r"(?m)^([0-9]+\.[0-9]+) -- ([0-9]+\.[0-9]+)\s*$", text)]
    if not segments:
        raise ValueError("no Silero speech segment in output")
    return segments


def command(threads):
    return ["./sherpa-onnx-vad", "--silero-vad-model=silero_vad_16k_op15.onnx",
            "--silero-vad-window-size=512", "--vad-num-threads=%d" % threads,
            "input.wav", "output.wav"]


def prepare(spec):
    model = spec["model"]
    native.verify_artifact(MODEL, model["sha256"], model["bytes"])
    runtime = spec["runtime"]
    archive = os.path.join(BENCH, "dl", runtime["archive"])
    native.fetch_artifact(runtime["url"], archive, runtime["sha256"], runtime["bytes"])
    runtime_dir = os.path.join(BENCH, "runtime", "v2.0.3", runtime["directory"])
    if not os.path.exists(os.path.join(runtime_dir, "bin", "sherpa-onnx-vad")):
        native.safe_extract(archive, os.path.join(BENCH, "runtime", "v2.0.3"))
    shim = os.path.join(BENCH, "runtime", "libhashshim.so")
    if not os.path.exists(shim):
        raise RuntimeError("missing libhashshim.so; run scripts/model-bench/shim/build.sh")
    return runtime_dir, shim, runtime["sha256"]


def wav_seconds(path):
    with wave.open(path, "rb") as f:
        if f.getframerate() != 16000 or f.getnchannels() != 1:
            raise ValueError("VAD fixture must be 16 kHz mono")
        return f.getnframes() / float(f.getframerate())


def run(args):
    spec = json.load(open(os.path.join(HERE, "vad_models.json")))
    runtime_dir, shim, runtime_hash = prepare(spec)
    model_hash = native.verify_artifact(MODEL, spec["model"]["sha256"], spec["model"]["bytes"])
    audio_s = wav_seconds(args.input)
    adb = Adb()
    adb.shell("true")
    info = {"phone": getprop(adb, "ro.product.model"), "soc": getprop(adb, "ro.soc.model"),
            "android": "%s (API %s, %s)" % (getprop(adb, "ro.build.version.release"), getprop(adb, "ro.build.version.sdk"), getprop(adb, "ro.build.display.id"))}
    d = "/data/local/tmp/mbench-vad-%d" % int(time.time())
    os.makedirs(args.out_dir, exist_ok=True)

    def work():
        for sub in ("bin/sherpa-onnx-vad", "lib/libsherpa-onnx-c-api.so", "lib/libonnxruntime.so"):
            adb.push(os.path.join(runtime_dir, sub), d + "/")
        adb.push(shim, d + "/libhashshim.so")
        adb.push(find_libcxx(), d + "/libc++_shared.so")
        adb.push(MODEL, d + "/silero_vad_16k_op15.onnx")
        adb.push(args.input, d + "/input.wav")
        wrapper = os.path.join(args.out_dir, "run.sh")
        with open(wrapper, "w", newline="\n") as f:
            f.write(WRAPPER)
        adb.push(wrapper, d + "/run.sh")
        adb.shell("chmod 755 %s/sherpa-onnx-vad %s/run.sh" % (d, d))
        th0, b0 = read_state(adb)
        check_preflight(th0, b0["temp_c"], b0["level"])
        run_detached(adb, d, "run", command(args.threads), args.timeout)
        th1, b1 = read_state(adb)
        for ext in ("out", "err", "meta"):
            adb.pull("%s/run.%s" % (d, ext), os.path.join(args.out_dir, "run." + ext))
        adb.pull("%s/output.wav" % d, os.path.join(args.out_dir, "output.wav"))
        meta = native.parse.parse_meta(open(os.path.join(args.out_dir, "run.meta")).read())
        out = open(os.path.join(args.out_dir, "run.out"), errors="replace").read()
        err = open(os.path.join(args.out_dir, "run.err"), errors="replace").read()
        status = "ok" if meta["exit"] == 0 else ("killed" if meta["exit"] == 137 else "error")
        notes = []
        segments = []
        if status == "ok":
            segments = parse_segments(out + "\n" + err)
            notes.append("segments=" + ",".join("%.3f-%.3f" % x for x in segments))
            notes.append("output_sha256=%s" % native.sha256_file(os.path.join(args.out_dir, "output.wav")))
        else:
            notes.append((err or out).strip()[-300:])
        row = {
            "phone": info["phone"], "soc": info["soc"], "android": info["android"],
            "runtime": spec["runtime"]["name"], "provider": "cpu", "threads": args.threads,
            "model_id": "silero-v6.2.1", "quant": "op15", "model_bytes": spec["model"]["bytes"],
            "model_sha256": model_hash, "runtime_sha256": runtime_hash,
            "audio_seconds": "%.3f" % audio_s, "process_seconds": meta["wall_seconds"],
            "peak_rss_mib": "%.1f" % (meta["vmhwm_kb"] / 1024.0), "wall_seconds": meta["wall_seconds"],
            "thermal_before": th0, "thermal_after": th1, "battery_temp_before": b0["temp_c"],
            "battery_temp_after": b1["temp_c"], "plugged": b1["plugged"],
            "mem_available_mib_before": "%.0f" % b0["mem_available_mib"], "mem_available_mib_after": "%.0f" % b1["mem_available_mib"],
            "run": 1, "date": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "harness_commit": harness_commit(),
            "status": status, "notes": "; ".join(notes),
        }
        native.append_row(args.tsv, row)
        print({key: row.get(key, "") for key in ("model_id", "audio_seconds", "process_seconds", "peak_rss_mib", "status")})

    with_workdir(adb, d, work)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--input", required=True, help="16 kHz mono WAV fixture")
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--timeout", type=int, default=600)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--tsv", required=True)
    args = ap.parse_args()
    if args.threads < 1:
        ap.error("threads must be positive")
    run(args)


if __name__ == "__main__":
    main()
