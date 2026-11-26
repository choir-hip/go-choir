#!/usr/bin/env python3
"""Refuse an Android build whose compiled model runtime is missing, stale or unverified.

The runtime is built by scripts/build_android_model_runtime.py in the lead's build tree, and its
output (mobile/exact-package/android/jniLibs) is gitignored. This check is what keeps a clean
checkout or a stale build from silently producing an APK whose model code is absent or older than
the JNI source: the receipt beside the staged libraries must name the manifest's source, NDK, API
and cmake configuration, every library the manifest names must be present and match its receipt
hash, and the receipt must name the current native/native_models.cpp by SHA-256.

  python3 -I mobile/scripts/check-android-model-runtime.py [--root DIR]

Exit 0 with `android model runtime: verified`; otherwise the reason on stderr and exit 1.
The callers are mobile/scripts/exact-env.sh and mobile/AutoradioLab/exact.mjs, so every Android
build or drive passes here. ALLOW_NO_MODELS=1 is the existing model-less build escape (as on
Apple).
"""
import argparse
import hashlib
import json
from pathlib import Path
import sys

ABI = "arm64-v8a"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def problems(root: Path) -> list[str]:
    mobile = root / "mobile"
    manifest = json.loads((mobile / "android-model-runtime.json").read_text())
    llama = manifest["llamaCpp"]
    staged = mobile / "exact-package" / "android" / "jniLibs" / ABI
    receipt_path = mobile / "exact-package" / "android" / "jniLibs.receipt.json"
    source = mobile / "exact-package" / "android" / "native" / "native_models.cpp"
    found = []
    if not receipt_path.is_file():
        return [f"{receipt_path} is missing: run mobile/scripts/build_android_model_runtime.py in the build tree"]
    try:
        receipt = json.loads(receipt_path.read_text())
    except json.JSONDecodeError as error:
        return [f"{receipt_path} is not JSON: {error}"]
    if receipt.get("sourceSha256") != llama["sourceSha256"]:
        found.append("the runtime was built from another llama.cpp source than the manifest pins")
    if receipt.get("api") != manifest["api"]:
        found.append(f"the runtime targets API {receipt.get('api')}, the manifest API {manifest['api']}")
    if receipt.get("ndk") != manifest["ndk"]:
        found.append(f"the runtime was built with {receipt.get('ndk')}, the manifest pins {manifest['ndk']}")
    built = receipt.get("llamaCpp") or {}
    if built.get("release") != llama["release"] or built.get("commit") != llama["commit"]:
        found.append("the runtime receipt names another llama.cpp release or commit")
    if built.get("cmake") != llama["cmake"]:
        found.append("the runtime was built with another cmake configuration than the manifest's")
    records = {record["file"]: record for record in receipt.get("libraries", [])}
    staged_files = sorted(path.name for path in staged.glob("*.so")) if staged.is_dir() else []
    for name in dict.fromkeys(llama["libraries"] + [llama["wrapper"]] + staged_files):
        path = staged / name
        record = records.get(name)
        if not path.is_file():
            found.append(f"{path} is missing: build the runtime in the lead's build tree")
            continue
        if record is None:
            found.append(f"{name} has no receipt record")
            continue
        if record.get("sha256") != sha256_file(path):
            found.append(f"{name} changed after it was verified")
        alignment = [value if isinstance(value, int) else int(value, 16) for value in record.get("loadAlignments", [])]
        if not alignment or any(value < int(manifest["pageAlignment"]) for value in alignment):
            found.append(f"{name} is not {manifest['pageAlignment']}-byte page aligned")
    if not source.is_file():
        found.append(f"{source} is missing: the JNI wrapper has no source to verify")
    elif receipt.get("wrapperSourceSha256") != sha256_file(source):
        found.append("the staging receipt does not name the current native/native_models.cpp: rebuild the runtime")
    return found


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=str(Path(__file__).resolve().parents[2]))
    args = parser.parse_args()
    found = problems(Path(args.root))
    if found:
        for line in found:
            print(f"android model runtime: {line}", file=sys.stderr)
        raise SystemExit(1)
    print("android model runtime: verified")


if __name__ == "__main__":
    main()
