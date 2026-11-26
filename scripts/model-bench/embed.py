#!/usr/bin/env python3
"""Benchmark EmbeddingGemma 2 Q8_0 on the Motorola through llama.cpp's public C API."""
import argparse
import json
import math
import os
import re
import subprocess
import sys
import tempfile
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import native
from run_android import Adb, check_preflight, getprop, harness_commit, read_state, run_detached, with_workdir

BENCH = os.path.expanduser("~/.cache/model-bench")
MODEL = os.path.join(BENCH, "android-models", "embeddinggemma", "embeddinggemma-2-Q8_0.gguf")
SPEC = os.path.join(HERE, "embed_models.json")
SOURCE = os.path.join(HERE, "embed_bench.cpp")


def parse_result(text):
    values = {}
    for key in ("dim", "tokens", "load_seconds", "prefill_seconds", "raw_norm", "first"):
        match = re.search(r"(?:^|\s)%s=([^\s]+)" % key, text)
        if not match:
            raise ValueError("missing %s in embedding output" % key)
        values[key] = float(match.group(1)) if "." in match.group(1) else int(match.group(1))
    if values["dim"] != 768 or values["tokens"] < 1:
        raise ValueError("unexpected embedding shape: %r" % values)
    if not all(math.isfinite(float(values[key])) for key in ("load_seconds", "prefill_seconds", "raw_norm", "first")):
        raise ValueError("non-finite embedding metric")
    return values


def command(threads):
    return ["./embed-bench", "embeddinggemma-2-Q8_0.gguf", "{prompt}", str(threads)]


def prepare(spec, cache):
    runtime = spec["runtime"]
    archive = os.path.join(cache, "llama-runtime", runtime["archive"])
    native.fetch_artifact(runtime["url"], archive, runtime["sha256"], runtime["bytes"])
    runtime_dir = os.path.join(cache, "llama-runtime", "generic", "llama-b12497")
    include_dir = os.path.join(runtime_dir, "include")
    if not os.path.exists(os.path.join(include_dir, "llama.h")):
        full = os.path.join(cache, "llama-runtime", "llama-b12497")
        if not os.path.exists(os.path.join(full, "include", "llama.h")):
            native.safe_extract(archive, os.path.join(cache, "llama-runtime"))
        include_dir = os.path.join(full, "include")
    if not os.path.exists(os.path.join(include_dir, "llama.h")):
        raise RuntimeError("llama.cpp archive has no include/llama.h")
    model_hash = native.verify_artifact(MODEL, spec["model"]["sha256"], spec["model"]["bytes"])
    return runtime_dir, include_dir, model_hash


def compile_bench(runtime_dir, include_dir, ndk, output):
    clang = os.path.join(ndk, "toolchains", "llvm", "prebuilt", "darwin-x86_64", "bin", "aarch64-linux-android34-clang++")
    if not os.path.exists(clang):
        raise RuntimeError("missing NDK compiler: %s" % clang)
    subprocess.run([
        clang, "-std=c++17", "-O2", "-fPIC", "-I", include_dir,
        SOURCE, "-L", runtime_dir, "-lllama", "-lggml", "-lggml-base",
        "-Wl,-rpath,$ORIGIN", "-Wl,-z,max-page-size=16384", "-static-libstdc++", "-o", output,
    ], check=True)


def run(args):
    with open(SPEC) as f:
        spec = json.load(f)
    runtime_dir, include_dir, model_hash = prepare(spec, args.cache)
    ndk = args.ndk or os.environ.get("ANDROID_NDK_HOME") or os.path.expanduser("~/Library/Android/sdk/ndk/27.3.13750724")
    adb = Adb()
    adb.shell("true")
    info = {
        "phone": getprop(adb, "ro.product.model"), "soc": getprop(adb, "ro.soc.model"),
        "android": "%s (API %s, %s)" % (getprop(adb, "ro.build.version.release"), getprop(adb, "ro.build.version.sdk"), getprop(adb, "ro.build.display.id")),
    }
    d = "/data/local/tmp/mbench-embed-%d" % int(time.time())
    os.makedirs(args.out_dir, exist_ok=True)
    results = []

    with tempfile.TemporaryDirectory(prefix="embed-bench-build-") as build:
        binary = os.path.join(build, "embed-bench")
        compile_bench(runtime_dir, include_dir, ndk, binary)
        binary_hash = native.sha256_file(binary)

        def work():
            adb.push(binary, d + "/embed-bench")
            for name in ("libllama.so", "libggml.so", "libggml-base.so", "libggml-cpu-android_armv8.2_2.so"):
                adb.push(os.path.join(runtime_dir, name), d + "/" + name)
            adb.push(MODEL, d + "/embeddinggemma-2-Q8_0.gguf")
            wrapper = os.path.join(args.out_dir, "run.sh")
            with open(wrapper, "w", newline="\n") as f:
                f.write(native.WRAPPER)
            adb.push(wrapper, d + "/run.sh")
            adb.shell("chmod 755 %s/embed-bench %s/run.sh" % (d, d))
            th0, b0 = read_state(adb)
            check_preflight(th0, b0["temp_c"], b0["level"])
            for index, prompt in enumerate(spec["prompts"], 1):
                tag = "run%d" % index
                cmd = command(args.threads)
                cmd[2] = prompt
                run_detached(adb, d, tag, cmd, args.timeout)
                for ext in ("out", "err", "meta"):
                    adb.pull("%s/%s.%s" % (d, tag, ext), os.path.join(args.out_dir, tag + "." + ext))
                th1, b1 = read_state(adb)
                meta = native.parse.parse_meta(open(os.path.join(args.out_dir, tag + ".meta")).read())
                out = open(os.path.join(args.out_dir, tag + ".out"), errors="replace").read()
                err = open(os.path.join(args.out_dir, tag + ".err"), errors="replace").read()
                status = "ok" if meta["exit"] == 0 else ("killed" if meta["exit"] == 137 else "error")
                row = {
                    "phone": info["phone"], "soc": info["soc"], "android": info["android"],
                    "runtime": spec["runtime"]["name"], "provider": "cpu", "threads": args.threads,
                    "model_id": "embeddinggemma-2-q8_0", "quant": "Q8_0", "model_bytes": spec["model"]["bytes"],
                    "model_sha256": model_hash, "runtime_sha256": spec["runtime"]["sha256"],
                    "process_seconds": meta["wall_seconds"], "peak_rss_mib": "%.1f" % (meta["vmhwm_kb"] / 1024.0),
                    "wall_seconds": meta["wall_seconds"], "thermal_before": th0, "thermal_after": th1,
                    "battery_temp_before": b0["temp_c"], "battery_temp_after": b1["temp_c"],
                    "plugged": b1["plugged"], "mem_available_mib_before": "%.0f" % b0["mem_available_mib"],
                    "mem_available_mib_after": "%.0f" % b1["mem_available_mib"],
                    "run": index, "date": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "harness_commit": harness_commit(),
                    "status": status,
                }
                if status == "ok":
                    try:
                        values = parse_result(out)
                        row["prefill_tps"] = "%.3f" % (values["tokens"] / values["prefill_seconds"])
                        row["notes"] = "prompt=%d load_seconds=%.6f raw_norm=%.6f binary_sha256=%s" % (
                            index, values["load_seconds"], values["raw_norm"], binary_hash)
                        results.append({"prompt_index": index, "prompt": prompt, **values})
                    except ValueError as exc:
                        row["status"] = "error"
                        row["notes"] = str(exc)
                else:
                    row["notes"] = "exit %d: %s" % (meta["exit"], (err or out).strip()[-300:])
                native.append_row(args.tsv, row)
                print({key: row.get(key, "") for key in ("model_id", "run", "prefill_tps", "process_seconds", "peak_rss_mib", "status")})
            th1, b1 = read_state(adb)
            for row in results:
                row["thermal_before"] = th0
                row["thermal_after"] = th1
            with open(os.path.join(args.out_dir, "results.json"), "w") as f:
                json.dump({"binary_sha256": binary_hash, "results": results}, f, indent=2)

        with_workdir(adb, d, work)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--timeout", type=int, default=600)
    ap.add_argument("--cache", default=native.CACHE)
    ap.add_argument("--ndk")
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--tsv", required=True)
    args = ap.parse_args()
    if args.threads < 1:
        ap.error("threads must be positive")
    run(args)


if __name__ == "__main__":
    main()
