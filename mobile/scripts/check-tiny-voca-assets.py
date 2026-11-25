#!/usr/bin/python3 -I
"""Check the actual TinyVoca tree (staged, cached, or unpacked from an IPA).

The phone needs no Python and no JavaScript: three compiled Core ML models,
the Dizione phonemizer data and its static XCFramework, and the notices.
"""
import hashlib
import json
import plistlib
from pathlib import Path
import sys

PORT = "426d644dba5953a2971c8fa27eeac5df77c0c2bb"
WEIGHTS = "5fe06641d0afecb3c04bc9faca2cf64d9b2b1600"
CHECKPOINT = "ebb22871524c5732520fb1652e6aefcec708a7452e83bdd00adcf6181db5ef70"
MODELS = {name + ".mlmodelc" for name in ("Encoder", "Decoder", "Vocoder")}
PHONEMIZER = {"lexicon.tsv", "align.json", "g2p.json", "g2p.npz", "homographs.json"}
TOP = {"tiny-voca-coreml.json"}
XDK = None
LICENSES = {"LICENSE.apache-2.0", "LICENSE.cmudict", "LICENSE.homograph-data",
            "NOTICE", "phonemizer-sources.json"}
MANIFEST = "asset-manifest.json"


def _sha(path):
    sha = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            sha.update(block)
    return sha.hexdigest()


def manifest(root, platform):
    if root.is_symlink() or not root.is_dir():
        raise ValueError(f"missing or linked TinyVoca directory: {root}")
    files, seen = {}, set()
    for path in sorted(root.rglob("*")):
        name = path.relative_to(root).as_posix()
        if path.is_symlink():
            raise ValueError(f"linked resource: {name}")
        top = name.split("/")[0]
        allowed = (top in MODELS | {MANIFEST} | TOP
                   or top == "phonemizer" or top == "Licenses")
        if not allowed:
            raise ValueError(f"unexpected resource: {name}")
        if path.is_dir():
            continue
        if not path.is_file() or path.stat().st_size == 0:
            raise ValueError(f"empty or invalid resource: {name}")
        seen.add(name)
        if name != MANIFEST:
            files[name] = _sha(path)

    expected = set(TOP) | {f"Licenses/{n}" for n in LICENSES}
    expected |= {f"phonemizer/{n}" for n in PHONEMIZER}
    expected |= {f"{model}/{f}" for model in MODELS
                 for f in ("model.mil", "coremldata.bin", "weights/weight.bin")}
    missing = sorted(expected - seen)
    if missing:
        raise ValueError("missing compiled model, phonemizer data or notice: " + ", ".join(missing))

    return {"format": 2, "platform": platform, "port": PORT, "weights_revision": WEIGHTS,
            "checkpoint_sha256": CHECKPOINT, "files": files}


def main():
    if len(sys.argv) not in (3, 4) or sys.argv[2] not in ("ios", "macos") or sys.argv[3:] not in ([], ["--record"]):
        raise ValueError("usage: check-tiny-voca-assets.py <TinyVoca directory> ios|macos [--record]")
    root, platform = Path(sys.argv[1]), sys.argv[2]
    expected = manifest(root, platform)
    path = root / MANIFEST
    if sys.argv[3:] == ["--record"]:
        path.write_text(json.dumps(expected, indent=2) + "\n", encoding="utf8")
    elif json.loads(path.read_text(encoding="utf8")) != expected:
        raise ValueError("asset manifest does not match the bundled bytes, source pins or platform")


try:
    main()
except (OSError, ValueError) as error:
    sys.exit(f"Tiny Voca: {error}")
