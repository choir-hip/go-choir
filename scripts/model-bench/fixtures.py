#!/usr/bin/env python3
"""Build the fixed test set: LibriSpeech test-clean and test-other, 16 kHz mono wav.

  fixtures.py --clean DIR --other DIR --out DIR [--seed 20261122]

DIR is an extracted LibriSpeech split (contains speaker/chapter/*.flac and
*.trans.txt). Writes OUT/wav/<id>.wav and OUT/manifest.tsv (id, condition,
reference text, duration, sha256 of the wav). The 100-utterance full set is 50
clean and 50 other, interleaved; the 10-utterance smoke set is its first 10
(5 and 5). Candidates are shuffled with a seeded generator after sorting, then
taken in order, skipping any utterance shorter than 1.5 s or longer than 20 s
(Whisper's window is 30 s; a long one would measure the window, not the model).
ffmpeg does the FLAC decode and resample; it is the only external program.
"""
import argparse
import hashlib
import os
import random
import subprocess
import sys
import wave

MIN_S, MAX_S = 1.5, 20.0
FIELDS = ["id", "condition", "text", "duration_s", "sha256"]


def shuffled(ids, seed):
    ids = sorted(ids)
    random.Random(seed).shuffle(ids)
    return ids


def interleave(a, b):
    out = []
    for x, y in zip(a, b):
        out += [x, y]
    return out


def choose(clean, other, seed, n):
    half = n // 2
    return interleave(shuffled(clean, seed)[:half], shuffled(other, seed + 1)[:half])


def wav_seconds(path):
    with wave.open(path, "rb") as w:
        return w.getnframes() / float(w.getframerate())


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1 << 20), b""):
            h.update(b)
    return h.hexdigest()


def manifest_row(uid, condition, text, wav_path):
    return {"id": uid, "condition": condition, "text": text.replace("\t", " ").replace("\n", " "),
            "duration_s": "%.3f" % wav_seconds(wav_path), "sha256": sha256_file(wav_path)}


def write_manifest(path, rows):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write("\t".join(FIELDS) + "\n")
        for r in rows:
            f.write("\t".join(r[k] for k in FIELDS) + "\n")


def read_manifest(path):
    with open(path, encoding="utf-8") as f:
        lines = f.read().split("\n")
    assert lines[0].split("\t") == FIELDS, "unexpected manifest header"
    return [dict(zip(FIELDS, l.split("\t"))) for l in lines[1:] if l]


def scan(split_dir):
    """id -> (flac path, reference text) for an extracted LibriSpeech split."""
    out = {}
    for root, _, files in os.walk(split_dir):
        for fn in files:
            if fn.endswith(".trans.txt"):
                with open(os.path.join(root, fn), encoding="utf-8") as f:
                    for line in f:
                        uid, _, text = line.rstrip("\n").partition(" ")
                        flac = os.path.join(root, uid + ".flac")
                        if os.path.exists(flac):
                            out[uid] = (flac, text)
    return out


def convert(flac, wav):
    subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-y", "-i", flac, "-ac", "1", "-ar", "16000",
                    "-c:a", "pcm_s16le", wav], check=True)


def build(split_dir, condition, seed, n, wav_dir):
    items = scan(split_dir)
    rows = []
    for uid in shuffled(items, seed):
        if len(rows) == n:
            break
        flac, text = items[uid]
        wav = os.path.join(wav_dir, uid + ".wav")
        convert(flac, wav)
        d = wav_seconds(wav)
        if not (MIN_S <= d <= MAX_S):
            os.remove(wav)
            continue
        rows.append(manifest_row(uid, condition, text, wav))
    return rows


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--clean", required=True)
    ap.add_argument("--other", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--seed", type=int, default=20261122)
    ap.add_argument("--n", type=int, default=100)
    a = ap.parse_args(argv)
    wav_dir = os.path.join(a.out, "wav")
    os.makedirs(wav_dir, exist_ok=True)
    half = a.n // 2
    c = build(a.clean, "clean", a.seed, half, wav_dir)
    o = build(a.other, "other", a.seed + 1, half, wav_dir)
    write_manifest(os.path.join(a.out, "manifest.tsv"), interleave(c, o))
    print("wrote %d utterances to %s" % (len(c) + len(o), a.out))


if __name__ == "__main__":
    main(sys.argv[1:])
