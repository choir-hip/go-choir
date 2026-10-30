#!/usr/bin/env python3
"""Vision probe: does a model on staging see an image?

Generates a small PNG (a solid background colour with a white word drawn
in a block font, both picked at random per run), sends it through the
normal authenticated product path

    choir.news -> proxy (API key) -> the computer's
    POST /api/model-policy/verify -> host gateway -> upstream model

to each model named, and checks that the answer names both the colour and
the word. No provider key is used or needed: the gateway holds them.

Output is one JSON report: per model the verdict, the answer, the token
counts and the image's artifact:sha256 ref on the computer. The API key and
the image bytes are never printed. --save-png keeps the generated image as
a repeatable artifact.

Credentials come from a 0600 JSON file with api_key (or key) and
computer_id, as scripts/management_latency_probe.py.

usage: vision_probe.py --cred FILE [--host URL] [--model provider/model ...]
       [--seed N] [--save-png FILE] [--out FILE]

Default models: chatgpt/gpt-6-luna and opencode-go/deepseek-v4.1-flash
(docs/archive/problems/models-configured-text-only-2026-10-15.md). To clear a model
off the text-only opt-out list, run it here with --model and delete its
line in internal/modelcatalog TextOnlyModels once it passes.
"""
import argparse
import base64
import hashlib
import json
import os
import random
import re
import stat
import struct
import sys
import urllib.error
import urllib.request
import zlib

DEFAULT_HOST = "https://choir.news"
DEFAULT_MODELS = ["chatgpt/gpt-6-luna", "opencode-go/deepseek-v4.1-flash"]

COLOURS = {
    "red": (210, 30, 30),
    "green": (20, 150, 50),
    "blue": (30, 70, 210),
}
WORDS = ["FOX", "OWL", "SUN", "KEY", "BOX"]

# 5x7 block glyphs for the letters in WORDS.
GLYPHS = {
    "B": ["11110", "10001", "10001", "11110", "10001", "10001", "11110"],
    "E": ["11111", "10000", "10000", "11110", "10000", "10000", "11111"],
    "F": ["11111", "10000", "10000", "11110", "10000", "10000", "10000"],
    "K": ["10001", "10010", "10100", "11000", "10100", "10010", "10001"],
    "L": ["10000", "10000", "10000", "10000", "10000", "10000", "11111"],
    "N": ["10001", "11001", "10101", "10011", "10001", "10001", "10001"],
    "O": ["01110", "10001", "10001", "10001", "10001", "10001", "01110"],
    "S": ["01111", "10000", "10000", "01110", "00001", "00001", "11110"],
    "U": ["10001", "10001", "10001", "10001", "10001", "10001", "01110"],
    "W": ["10001", "10001", "10001", "10101", "10101", "10101", "01010"],
    "X": ["10001", "10001", "01010", "00100", "01010", "10001", "10001"],
    "Y": ["10001", "10001", "01010", "00100", "00100", "00100", "00100"],
}

SCALE = 12
PAD = 24


def render_png(word, rgb):
    """A PNG of word in white block letters on a solid rgb background."""
    cols = len(word) * 6 - 1
    width = cols * SCALE + 2 * PAD
    height = 7 * SCALE + 2 * PAD
    rows = []
    for y in range(height):
        row = bytearray([0])  # filter: none
        gy = (y - PAD) // SCALE if y >= PAD else -1
        for x in range(width):
            gx = (x - PAD) // SCALE if x >= PAD else -1
            on = False
            if 0 <= gy < 7 and 0 <= gx < cols:
                letter, col = divmod(gx, 6)
                if col < 5:
                    on = GLYPHS[word[letter]][gy][col] == "1"
            row += bytes((255, 255, 255)) if on else bytes(rgb)
        rows.append(bytes(row))
    raw = b"".join(rows)

    def chunk(kind, data):
        body = kind + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr)
            + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


def load_cred(path):
    st = os.stat(path)
    if st.st_mode & (stat.S_IRWXG | stat.S_IRWXO):
        print(f"warning: {path} is readable beyond its owner (want 0600)", file=sys.stderr)
    with open(path) as f:
        cred = json.load(f)
    key = cred.get("api_key") or cred.get("key")
    computer = cred.get("computer_id")
    if not key or not computer:
        raise SystemExit("credential needs api_key (or key) and computer_id")
    return key, computer


def verify(host, key, computer, provider, model, prompt, image_b64):
    body = json.dumps({
        "role": "verifier",
        "provider": provider,
        "model": model,
        "prompt": prompt,
        "image_base64": image_b64,
        "image_mime_type": "image/png",
    }).encode()
    req = urllib.request.Request(
        host.rstrip("/") + "/api/model-policy/verify",
        data=body,
        method="POST",
        headers={
            "Authorization": "Bearer " + key,
            "X-Choir-Computer": computer,
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=180) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as err:
        try:
            payload = json.load(err)
        except Exception:
            payload = {}
        # The route's error names the refusal; it never carries the image.
        return err.code, {"error": str(payload.get("error", ""))[:300]}
    except urllib.error.URLError as err:
        return 0, {"error": f"unreachable: {err.reason}"}


def mentions(text, word):
    return re.search(r"\b" + re.escape(word.lower()) + r"\b", text.lower()) is not None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--cred", required=True, help="0600 JSON with api_key and computer_id")
    ap.add_argument("--host", default=DEFAULT_HOST)
    ap.add_argument("--model", action="append", help="provider/model (repeatable)")
    ap.add_argument("--seed", type=int, help="pick colour and word deterministically")
    ap.add_argument("--save-png", help="write the generated image here")
    ap.add_argument("--out", help="write the JSON report here as well")
    args = ap.parse_args()

    key, computer = load_cred(args.cred)
    rng = random.Random(args.seed)
    colour = rng.choice(sorted(COLOURS))
    word = rng.choice(WORDS)
    png = render_png(word, COLOURS[colour])
    if args.save_png:
        with open(args.save_png, "wb") as f:
            f.write(png)
    image_b64 = base64.b64encode(png).decode()
    prompt = ("This image shows one word in white capital letters on a solid "
              "background. Reply with exactly two lowercase words separated by "
              "a space: the background colour (red, green or blue), then the word.")

    results = []
    for spec in args.model or DEFAULT_MODELS:
        provider, _, model = spec.partition("/")
        if not model:
            raise SystemExit(f"--model wants provider/model, got {spec!r}")
        status, payload = verify(args.host, key, computer, provider, model, prompt, image_b64)
        answer = str(payload.get("text_excerpt", ""))
        ok = status == 200 and mentions(answer, colour) and mentions(answer, word)
        results.append({
            "model": spec,
            "pass": ok,
            "http_status": status,
            "answer": answer[:200],
            "error": payload.get("error"),
            "response_model": payload.get("response_model"),
            "image_input": payload.get("image_input"),
            "image_ref": payload.get("image_ref"),
            "input_tokens": payload.get("input_tokens"),
            "output_tokens": payload.get("output_tokens"),
        })

    report = {
        "probe": "vision",
        "host": args.host,
        "computer_id": computer,
        "expected": {"colour": colour, "word": word.lower()},
        "image": {
            "bytes": len(png),
            "sha256": hashlib.sha256(png).hexdigest(),
            "saved_to": args.save_png,
        },
        "results": results,
        "pass": all(r["pass"] for r in results),
    }
    text = json.dumps(report, indent=2)
    print(text)
    if args.out:
        with open(args.out, "w") as f:
            f.write(text + "\n")
    sys.exit(0 if report["pass"] else 1)


if __name__ == "__main__":
    main()
