#!/usr/bin/env python3
"""Build the APK's llama.cpp JNI runtime from pinned source in the lead's build tree.

No executable is copied to /data/local/tmp. The pinned source archive is
verified by SHA-256, configured with NDK r27d/API 34 and linked with
`-Wl,-z,max-page-size=16384`, so every ELF LOAD segment meets Android's 16 KiB
page-size contract. The official prebuilt Android archive is not used: its
libraries are 4 KiB aligned (docs/problems/android-native-runtime-archive-2026-11-26.md).
The small JNI wrapper is compiled against the same source tree's public headers.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
MOBILE = ROOT / "mobile"
MANIFEST_PATH = MOBILE / "android-model-runtime.json"
CACHE = MOBILE / ".deps" / "android-model-runtime"
ANDROID_PACKAGE = MOBILE / "exact-package" / "android"
JNI_LIBS = ANDROID_PACKAGE / "jniLibs"
RECEIPT = ANDROID_PACKAGE / "jniLibs.receipt.json"
ABI = "arm64-v8a"


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fetch_verified(url: str, destination: Path, expected_sha: str, expected_bytes: int | None) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.is_file() and (expected_bytes is None or destination.stat().st_size == expected_bytes) and sha256_file(destination) == expected_sha:
        print(f"verified cache: {destination}")
        return
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme != "https" or parsed.username or parsed.password:
        raise ValueError(f"source URL must be HTTPS without credentials: {url}")
    temporary = destination.with_name(f".{destination.name}.{os.getpid()}.download")
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "Autoradio-native-model-build"})
        with urllib.request.urlopen(request, timeout=180) as response, temporary.open("wb") as output:
            shutil.copyfileobj(response, output, length=1024 * 1024)
        if expected_bytes is not None and temporary.stat().st_size != expected_bytes:
            raise ValueError(f"{destination}: expected {expected_bytes} bytes, got {temporary.stat().st_size}")
        actual = sha256_file(temporary)
        if actual != expected_sha:
            raise ValueError(f"{destination}: expected SHA-256 {expected_sha}, got {actual}")
        os.replace(temporary, destination)
    finally:
        temporary.unlink(missing_ok=True)


def source_member_path(name: str, source_root: str) -> PurePosixPath:
    path = PurePosixPath(name)
    if path.is_absolute() or not path.parts or any(part in ("", ".", "..") for part in name.split("/")):
        raise ValueError(f"unsafe archive member: {name}")
    if path.parts[0] != source_root:
        raise ValueError(f"archive member is outside {source_root}: {name}")
    return path


def extract_source(archive: Path, destination: Path, source_root: str) -> Path:
    root = PurePosixPath(source_root)
    if root.is_absolute() or len(root.parts) != 1:
        raise ValueError(f"source archive must hold exactly one top-level directory: {source_root}")
    with tarfile.open(archive, "r:gz") as bundle:
        for member in bundle.getmembers():
            source_member_path(member.name, source_root)
            if member.issym() or member.islnk():
                raise ValueError(f"source archive holds a link: {member.name}")
        bundle.extractall(destination, filter="data")
    source = destination / source_root
    if not (source / "CMakeLists.txt").is_file() or not (source / "include" / "llama.h").is_file():
        raise ValueError("source archive is not llama.cpp")
    return source


def verify_load_alignment(alignments: list[int], label: str, minimum: int = 16384) -> None:
    if not alignments or any(value < minimum for value in alignments):
        raise ValueError(f"{label}: every LOAD segment must be at least {minimum}-byte aligned; got {alignments}")


def verify_dependencies(needed: list[str], available: set[str], system: set[str], label: str) -> None:
    missing = sorted(set(needed) - available - system)
    if missing:
        raise ValueError(f"{label}: unstaged shared-library dependencies: {', '.join(missing)}")


def run(command: list[str], cwd: Path | None = None) -> str:
    result = subprocess.run(command, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=cwd)
    if result.returncode != 0:
        raise ValueError(f"command failed ({result.returncode}): {' '.join(command)}\n{result.stderr.strip()[-4000:]}")
    return result.stdout


def ndk_tool(ndk: Path, name: str) -> Path:
    host = "darwin-x86_64" if platform.system() == "Darwin" else "linux-x86_64"
    tool = ndk / "toolchains" / "llvm" / "prebuilt" / host / "bin" / name
    if not tool.is_file():
        raise ValueError(f"NDK tool is missing: {tool}")
    return tool


def require_ndk(manifest: dict) -> Path:
    revision = manifest["ndk"]["revision"]
    candidates = []
    if os.environ.get("ANDROID_NDK_HOME"):
        candidates.append(Path(os.environ["ANDROID_NDK_HOME"]))
    if os.environ.get("ANDROID_HOME"):
        candidates.append(Path(os.environ["ANDROID_HOME"]) / "ndk" / revision)
    candidates.append(Path.home() / "Library" / "Android" / "sdk" / "ndk" / revision)
    for candidate in candidates:
        properties = candidate / "source.properties"
        if properties.is_file() and f"Pkg.Revision = {revision}" in properties.read_text():
            return candidate
    raise ValueError(f"NDK {manifest['ndk']['version']} ({revision}) is required; set ANDROID_NDK_HOME to that exact NDK")


def verify_elf(path: Path, readelf: Path, nm: Path, available: set[str], system: set[str],
               soname: str | None, required_symbols: tuple[str, ...] = ()) -> dict:
    header = run([str(readelf), "-hW", str(path)])
    dynamic = run([str(readelf), "-dW", str(path)])
    segments = run([str(readelf), "-lW", str(path)])
    if not re.search(r"Class:\s+ELF64", header) or not re.search(r"Machine:\s+AArch64", header):
        raise ValueError(f"{path.name}: expected a 64-bit AArch64 ELF")
    if not re.search(r"Type:\s+DYN \(Shared object file\)", header):
        raise ValueError(f"{path.name}: expected a shared object")
    actual_soname = re.search(r"\(SONAME\).*?\[([^]]+)\]", dynamic)
    actual_soname = actual_soname.group(1) if actual_soname else None
    if actual_soname != soname:
        raise ValueError(f"{path.name}: expected SONAME {soname!r}, got {actual_soname!r}")
    needed = re.findall(r"\(NEEDED\).*?\[([^]]+)\]", dynamic)
    verify_dependencies(needed, available, system, path.name)
    alignments = [int(line.split()[-1], 16) for line in segments.splitlines() if line.strip().startswith("LOAD ")]
    verify_load_alignment(alignments, path.name)
    symbols = []
    if required_symbols:
        listing = run([str(nm), "-D", "--defined-only", str(path)])
        symbols = {line.split()[-1] for line in listing.splitlines() if line.split()}
        missing = sorted(set(required_symbols) - symbols)
        if missing:
            raise ValueError(f"{path.name}: public llama.cpp symbols are missing: {', '.join(missing)}")
    return {
        "file": path.name,
        "bytes": path.stat().st_size,
        "sha256": sha256_file(path),
        "soname": actual_soname,
        "needed": needed,
        "loadAlignments": alignments,
        "requiredSymbols": sorted(required_symbols),
    }


def ensure_source(manifest: dict) -> Path:
    llama = manifest["llamaCpp"]
    cache = CACHE / "source"
    source_archive = cache / f"llama-{llama['release']}.tar.gz"
    fetch_verified(llama["sourceUrl"], source_archive, llama["sourceSha256"], llama["sourceBytes"])
    source = cache / llama["sourceRoot"]
    receipt = cache / "receipt.json"
    recorded = json.loads(receipt.read_text()) if receipt.is_file() else {}
    if recorded.get("sha256") != llama["sourceSha256"]:
        shutil.rmtree(source, ignore_errors=True)
    if not (source / "CMakeLists.txt").is_file() or not (source / "include" / "llama.h").is_file():
        shutil.rmtree(source, ignore_errors=True)
        extract_source(source_archive, cache, llama["sourceRoot"])
    if sha256_file(source / "LICENSE") != llama["licenseSha256"]:
        raise ValueError(f"{llama['release']} LICENSE does not match the pin")
    receipt.write_text(json.dumps({"sha256": llama["sourceSha256"], "root": llama["sourceRoot"]}, indent=2) + "\n")
    return source


def compiled_runtime(manifest: dict, ndk: Path, source: Path) -> Path:
    llama = manifest["llamaCpp"]
    cmake = llama["cmake"]
    key = f"{llama['release']}-{re.sub(r'[^a-zA-Z0-9]+', '-', cmake['armArch'])}"
    runtime = CACHE / "runtime" / key
    receipt = runtime / "receipt.json"
    fingerprint = {
        "sourceSha256": llama["sourceSha256"],
        "api": manifest["api"],
        "ndk": manifest["ndk"]["revision"],
        "cmake": cmake,
    }
    if receipt.is_file():
        try:
            recorded = json.loads(receipt.read_text())
            if recorded.get("fingerprint") != fingerprint:
                raise ValueError("runtime receipt is for another source, toolchain or build configuration")
            for name in llama["libraries"]:
                path = runtime / "lib" / name
                if not path.is_file() or recorded["libraries"].get(name) != sha256_file(path):
                    raise ValueError(f"cached runtime is incomplete: {name}")
            print(f"verified compiled runtime: {runtime}")
            return runtime
        except (ValueError, KeyError, TypeError, json.JSONDecodeError):
            shutil.rmtree(runtime, ignore_errors=True)
    work = Path(tempfile.mkdtemp(prefix="build-", dir=CACHE))
    try:
        build = work / "build"
        run([
            "cmake", "-S", str(source), "-B", str(build), "-G", "Unix Makefiles",
            f"-DCMAKE_TOOLCHAIN_FILE={ndk / 'build' / 'cmake' / 'android.toolchain.cmake'}",
            f"-DANDROID_ABI={ABI}", f"-DANDROID_PLATFORM=android-{manifest['api']}",
            f"-DCMAKE_BUILD_TYPE={cmake['buildType']}",
            f"-DBUILD_SHARED_LIBS={cmake['sharedLibs']}",
            "-DLLAMA_BUILD_TESTS=OFF", "-DLLAMA_BUILD_EXAMPLES=OFF",
            "-DLLAMA_BUILD_TOOLS=OFF", "-DLLAMA_BUILD_SERVER=OFF", "-DLLAMA_CURL=OFF",
            "-DGGML_NATIVE=OFF", "-DGGML_OPENMP=OFF", "-DGGML_BACKEND_DL=OFF",
            f"-DGGML_CPU_ARM_ARCH={cmake['armArch']}",
            f"-DCMAKE_SHARED_LINKER_FLAGS={cmake['sharedLinkerFlags']}",
        ])
        run(["cmake", "--build", str(build), "--target", *cmake["targets"], "-j", str(max(2, os.cpu_count() or 2))])
        staging = work / "runtime"
        (staging / "lib").mkdir(parents=True)
        for name in llama["libraries"]:
            produced = build / "bin" / name
            if not produced.is_file():
                raise ValueError(f"cmake did not produce {name}")
            shutil.copyfile(produced, staging / "lib" / name)
        runtime.parent.mkdir(parents=True, exist_ok=True)
        shutil.rmtree(runtime, ignore_errors=True)      # an interrupted earlier build left no receipt
        os.replace(staging, runtime)
    finally:
        shutil.rmtree(work, ignore_errors=True)
    receipt.write_text(json.dumps({
        "fingerprint": fingerprint,
        "release": llama["release"],
        "commit": llama["commit"],
        "libraries": {name: sha256_file(runtime / "lib" / name) for name in llama["libraries"]},
    }, indent=2) + "\n")
    return runtime


def build(manifest: dict) -> None:
    if subprocess.run([str(MOBILE / "scripts" / "build-tree.sh"), "is"], cwd=ROOT).returncode != 0:
        raise ValueError("this script runs only in the lead's named build tree; builders do not stage native output in their lanes")
    if not shutil.which("cmake"):
        raise ValueError("cmake is required to build llama.cpp from source")
    subprocess.run([sys.executable, str(MOBILE / "scripts" / "gen_android_model_catalog.py"), "--check"], check=True)
    ndk = require_ndk(manifest)
    readelf = ndk_tool(ndk, "llvm-readelf")
    nm = ndk_tool(ndk, "llvm-nm")
    clangxx = ndk_tool(ndk, "aarch64-linux-android34-clang++")
    CACHE.mkdir(parents=True, exist_ok=True)
    llama = manifest["llamaCpp"]
    source = ensure_source(manifest)
    runtime = compiled_runtime(manifest, ndk, source)
    tracked_license = ANDROID_PACKAGE / "licenses" / "llama.cpp-LICENSE"
    if sha256_file(tracked_license) != llama["licenseSha256"]:
        raise ValueError("tracked llama.cpp LICENSE does not match the source")
    staged = ANDROID_PACKAGE / f".jniLibs-{os.getpid()}"
    try:
        abi = staged / ABI
        abi.mkdir(parents=True)
        for name in llama["libraries"]:
            shutil.copyfile(runtime / "lib" / name, abi / name)
        wrapper = abi / llama["wrapper"]
        run([
            str(clangxx), "-std=c++17", "-fPIC", "-shared", "-O2", "-DNDEBUG",
            "-Wl,-z,max-page-size=16384", "-Wl,--no-undefined", f"-Wl,-soname,{llama['wrapper']}",
            "-static-libstdc++", "-I", str(source / "include"), "-I", str(source / "ggml" / "include"),
            str(ANDROID_PACKAGE / "native" / "native_models.cpp"),
            "-L", str(abi), "-lllama", "-lggml", "-lggml-base", "-o", str(wrapper),
        ])
        available = set(llama["libraries"]) | {llama["wrapper"]}
        system = set(llama["systemLibraries"])
        extra = []
        if "libc++_shared.so" in run([str(readelf), "-dW", str(wrapper)]):
            host = "darwin-x86_64" if platform.system() == "Darwin" else "linux-x86_64"
            shared = ndk / "toolchains" / "llvm" / "prebuilt" / host / "sysroot" / "usr" / "lib" / "aarch64-linux-android" / "libc++_shared.so"
            shutil.copyfile(shared, abi / shared.name)
            available.add(shared.name)
            extra.append(shared.name)
        records = [verify_elf(abi / name, readelf, nm, available, system, name,
                              tuple(llama["requiredSymbols"]) if name == "libllama.so" else ())
                   for name in llama["libraries"]]
        records.append(verify_elf(wrapper, readelf, nm, available, system, llama["wrapper"]))
        records.extend(verify_elf(abi / name, readelf, nm, available, system, name) for name in extra)
        receipt = {
            "abi": ABI,
            "api": manifest["api"],
            "ndk": manifest["ndk"],
            "source": llama["sourceUrl"],
            "sourceSha256": llama["sourceSha256"],
            "llamaCpp": {"release": llama["release"], "commit": llama["commit"], "cmake": llama["cmake"]},
            "onnxRuntime": manifest["onnxRuntime"],
            "wrapperSourceSha256": sha256_file(ANDROID_PACKAGE / "native" / "native_models.cpp"),
            "libraries": records,
        }
        backup = ANDROID_PACKAGE / ".jniLibs-previous"
        if backup.exists():
            shutil.rmtree(backup)
        if JNI_LIBS.exists():
            os.replace(JNI_LIBS, backup)
        try:
            os.replace(staged, JNI_LIBS)
        except Exception:
            if backup.exists() and not JNI_LIBS.exists():
                os.replace(backup, JNI_LIBS)
            raise
        if backup.exists():
            shutil.rmtree(backup)
        RECEIPT.write_text(json.dumps(receipt, indent=2) + "\n")
        print(f"staged verified Android model runtime in {JNI_LIBS}")
    finally:
        if staged.exists():
            shutil.rmtree(staged, ignore_errors=True)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    manifest = json.loads(MANIFEST_PATH.read_text())
    try:
        build(manifest)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"android model runtime: {error}", file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
