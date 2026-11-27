#!/usr/bin/env python3
"""Host checks for the pinned Android model runtime and its public C boundary."""
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
MOBILE = ROOT / "mobile"
SPEC = importlib.util.spec_from_file_location(
    "build_android_model_runtime", MOBILE / "scripts" / "build_android_model_runtime.py"
)
runtime = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(runtime)


class RuntimeManifestTest(unittest.TestCase):
    def setUp(self):
        self.manifest = json.loads((MOBILE / "android-model-runtime.json").read_text())

    def test_exact_pin_and_android_toolchain_are_unchanged(self):
        self.assertEqual(
            (MOBILE / "exact.pin").read_text().strip(),
            "8552d20f9b50a013fc19d5fc9298ff149d8b2dc5",
        )
        self.assertEqual(self.manifest["ndk"], {"version": "r27d", "revision": "27.3.13750724"})
        self.assertEqual(self.manifest["api"], 34)
        self.assertEqual(self.manifest["pageAlignment"], 16384)
        self.assertEqual(
            self.manifest["onnxRuntime"]["maven"],
            "com.microsoft.onnxruntime:onnxruntime-android:1.32.0",
        )

    def test_llama_source_and_toolchain_are_pinned(self):
        llama = self.manifest["llamaCpp"]
        self.assertEqual(llama["release"], "b12497")
        self.assertEqual(llama["commit"], "3574df84b776611928d9b227bf614be71fbb9849")
        self.assertEqual(
            llama["sourceUrl"],
            "https://github.com/ggml-org/llama.cpp/archive/3574df84b776611928d9b227bf614be71fbb9849.tar.gz",
        )
        self.assertEqual(
            llama["sourceSha256"],
            "c8e5eaaa53640084d18a4b1c45eefad3d914f26adf523524b75a1afcdb3db092",
        )
        self.assertEqual(llama["sourceBytes"], 39578878)
        self.assertEqual(llama["sourceRoot"], "llama.cpp-3574df84b776611928d9b227bf614be71fbb9849")
        self.assertEqual(llama["licenseSha256"], "94f29bbed6a22c35b992c5c6ebf0e7c92f13b836b90f36f461c9cf2f0f1d010d")
        self.assertFalse(llama.get("archiveUrl"))
        cmake = llama["cmake"]
        self.assertEqual(cmake["armArch"], "armv8.2-a+dotprod+fp16")
        self.assertIn("max-page-size=16384", cmake["sharedLinkerFlags"])
        self.assertEqual(cmake["sharedLibs"], "ON")

    def test_only_required_libraries_are_staged(self):
        libraries = self.manifest["llamaCpp"]["libraries"]
        self.assertEqual(libraries, ["libllama.so", "libggml.so", "libggml-base.so", "libggml-cpu.so"])
        self.assertTrue(all(name.startswith("lib") and name.endswith(".so") for name in libraries))
        self.assertFalse(any("mtmd" in name or "common" in name for name in libraries))

    def test_native_source_calls_only_declared_public_llama_functions(self):
        allowed = set(self.manifest["llamaCpp"]["requiredSymbols"])
        source = (MOBILE / "exact-package" / "android" / "native" / "native_models.cpp").read_text()
        called = set(re.findall(r"\b(llama_[A-Za-z0-9_]+)\s*\(", source))
        self.assertTrue(called)
        self.assertEqual(called - allowed, set(), "a llama call is not in the pinned public-symbol manifest")

    def test_model_ids_and_pins_match_the_shared_catalog(self):
        models = {model["id"]: model for model in self.manifest["models"]}
        self.assertEqual(
            set(models),
            {
                "qwen3-0.6b-q4_k_m",
                "qwen3-0.6b-q8_0",
                "embeddinggemma-2-q8_0",
                "silero-vad-v6.2.1",
                "tiny-voca",
            },
        )
        self.assertEqual(models["embeddinggemma-2-q8_0"]["embeddingDimension"], 768)
        self.assertEqual(models["tiny-voca"]["supportedVoices"], ["F2", "M2"])
        self.assertEqual(models["silero-vad-v6.2.1"]["files"][0]["sha256"],
                         "7ed98ddbad84ccac4cd0aeb3099049280713df825c610a8ed34543318f1b2c49")
        benchmark = json.loads((ROOT / "scripts" / "model-bench" / "native_models.json").read_text())
        for model_id, job, filename in [
            ("qwen3-0.6b-q4_k_m", "qwen-q4", "Qwen3-0.6B-Q4_K_M.gguf"),
            ("qwen3-0.6b-q8_0", "qwen-q8", "Qwen3-0.6B-Q8_0.gguf"),
        ]:
            file = models[model_id]["files"][0]
            pinned = benchmark["jobs"][job]["files"][filename]
            self.assertEqual(file["name"], filename)
            self.assertEqual({key: file[key] for key in ("url", "sha256", "bytes")}, pinned)
        for model in models.values():
            self.assertGreater(model["bytes"], 0)
            for file in model["files"]:
                self.assertTrue(file["url"].startswith("https://"))
                self.assertRegex(file["sha256"], r"^[0-9a-f]{64}$")
                self.assertEqual(Path(file["name"]).name, file["name"])

    def test_generated_kotlin_catalog_matches_the_manifest(self):
        subprocess.run(
            [sys.executable, str(MOBILE / "scripts" / "gen_android_model_catalog.py"), "--check"],
            check=True,
        )


class ElfContractTest(unittest.TestCase):
    def test_archive_style_four_kib_alignment_is_refused(self):
        with self.assertRaisesRegex(ValueError, "16384"):
            runtime.verify_load_alignment([4096, 4096, 4096], "libllama.so")

    def test_sixteen_kib_alignment_passes(self):
        runtime.verify_load_alignment([16384, 16384, 16384], "libautoradio_models.so")

    def test_dependencies_must_be_staged_or_system_libraries(self):
        runtime.verify_dependencies(
            ["libggml.so", "libggml-base.so", "libm.so", "libdl.so", "libc.so"],
            {"libggml.so", "libggml-base.so", "libautoradio_models.so"},
            {"libm.so", "libdl.so", "libc.so"},
            "libllama.so",
        )
        with self.assertRaisesRegex(ValueError, "libmissing.so"):
            runtime.verify_dependencies(["libmissing.so"], set(), set(), "libllama.so")

    def test_source_members_cannot_escape_or_answer_another_root(self):
        root = json.loads((MOBILE / "android-model-runtime.json").read_text())["llamaCpp"]["sourceRoot"]
        self.assertEqual(str(runtime.source_member_path(root + "/include/llama.h", root)), root + "/include/llama.h")
        for name in ["../libllama.so", "/libllama.so", root + "/../../libllama.so", "llama-b12497/libllama.so", ""]:
            with self.assertRaises(ValueError):
                runtime.source_member_path(name, root)


class RuntimeReceiptTest(unittest.TestCase):
    """The build-gate check that keeps a clean or stale tree from shipping an APK without its model runtime."""

    def fixture(self, directory):
        import tempfile
        root = Path(directory)
        mobile = root / "mobile"
        android = mobile / "exact-package" / "android"
        (android / "native").mkdir(parents=True, exist_ok=True)
        (android / "jniLibs" / "arm64-v8a").mkdir(parents=True, exist_ok=True)
        manifest = json.loads((MOBILE / "android-model-runtime.json").read_text())
        (mobile / "android-model-runtime.json").write_text(json.dumps(manifest))
        (android / "native" / "native_models.cpp").write_text("// fixture")
        libraries = []
        for name in manifest["llamaCpp"]["libraries"] + [manifest["llamaCpp"]["wrapper"]]:
            path = android / "jniLibs" / "arm64-v8a" / name
            path.write_bytes(b"ELF" + name.encode())
            libraries.append({"file": name, "sha256": subprocess.run(
                ["shasum", "-a", "256", str(path)], capture_output=True, text=True, check=True).stdout.split()[0],
                "loadAlignments": ["0x4000"]})
        receipt = {
            "abi": "arm64-v8a", "api": manifest["api"], "ndk": manifest["ndk"],
            "sourceSha256": manifest["llamaCpp"]["sourceSha256"],
            "llamaCpp": {"release": manifest["llamaCpp"]["release"], "commit": manifest["llamaCpp"]["commit"],
                         "cmake": manifest["llamaCpp"]["cmake"]},
            "wrapperSourceSha256": subprocess.run(["shasum", "-a", "256", str(android / "native" / "native_models.cpp")],
                                                  capture_output=True, text=True, check=True).stdout.split()[0],
            "libraries": libraries,
        }
        (android / "jniLibs.receipt.json").write_text(json.dumps(receipt))
        return root, android, manifest

    def test_a_verified_runtime_has_no_problems(self):
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            root, _, _ = self.fixture(directory)
            self.assertEqual(runtime_problems(root), [])

    def test_a_missing_receipt_or_library_is_refused(self):
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            root, android, manifest = self.fixture(directory)
            (android / "jniLibs.receipt.json").unlink()
            self.assertIn("is missing", " ".join(runtime_problems(root)))
            root, android, manifest = self.fixture(directory)
            (android / "jniLibs" / "arm64-v8a" / manifest["llamaCpp"]["libraries"][0]).unlink()
            self.assertTrue(any("missing" in problem for problem in runtime_problems(root)))

    def test_a_stale_wrapper_or_foreign_source_is_refused(self):
        import os, tempfile
        with tempfile.TemporaryDirectory() as directory:
            root, android, manifest = self.fixture(directory)
            source = android / "native" / "native_models.cpp"
            source.write_text("// changed after the runtime was built")
            self.assertTrue(any("does not name the current" in problem for problem in runtime_problems(root)))
            root, android, manifest = self.fixture(directory)
            receipt = json.loads((android / "jniLibs.receipt.json").read_text())
            receipt["sourceSha256"] = "0" * 64
            (android / "jniLibs.receipt.json").write_text(json.dumps(receipt))
            self.assertTrue(any("another llama.cpp source" in problem for problem in runtime_problems(root)))


def runtime_problems(root):
    spec = importlib.util.spec_from_file_location("check_android_model_runtime", MOBILE / "scripts" / "check-android-model-runtime.py")
    checker = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(checker)
    return checker.problems(Path(root))


if __name__ == "__main__":
    unittest.main()
