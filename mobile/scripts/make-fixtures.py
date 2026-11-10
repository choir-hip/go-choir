#!/usr/bin/env python3
"""Regenerate the tracked fixtures: kept turns and the programme clip. macOS only (say, afconvert).
No third-party text: the sentences below are written for this repo."""
import array, json, math, os, subprocess, sys, tempfile, wave

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "fixtures")
RATE = 16000

def tts(text, voice, path_wav):
    with tempfile.TemporaryDirectory() as d:
        aiff = os.path.join(d, "s.aiff")
        subprocess.check_call(["say", "-v", voice, "-o", aiff, text])
        subprocess.check_call(["afconvert", "-f", "WAVE", "-d", f"LEI16@{RATE}", "-c", "1", aiff, path_wav])

def read(path):
    with wave.open(path) as w:
        a = array.array("h"); a.frombytes(w.readframes(w.getnframes()))
        return [x / 32768 for x in a]

def to_caf(samples, path):
    with tempfile.TemporaryDirectory() as d:
        wav = os.path.join(d, "o.wav")
        a = array.array("h", [max(-32768, min(32767, int(x * 32767))) for x in samples])
        with wave.open(wav, "wb") as w:
            w.setnchannels(1); w.setsampwidth(2); w.setframerate(RATE); w.writeframes(a.tobytes())
        subprocess.check_call(["afconvert", "-f", "caff", "-d", f"LEF32@{RATE}", wav, path])

def rms_scale(x, target_db):
    r = math.sqrt(sum(v * v for v in x) / max(len(x), 1)) or 1e-6
    g = 10 ** (target_db / 20) / r
    return [v * g for v in x]

def bed(seconds):
    """A slow synthetic melody, not speech, to stand in for programme audio."""
    notes = [262, 330, 392, 330, 294, 370, 440, 370]
    out = []
    for i in range(int(seconds * RATE)):
        f = notes[int(i / RATE / 0.5) % len(notes)]
        out.append(0.5 * math.sin(2 * math.pi * f * i / RATE) + 0.25 * math.sin(2 * math.pi * 2 * f * i / RATE))
    return out

def make_turn(name, text, mixed):
    d = os.path.join(ROOT, "turns", name); os.makedirs(d, exist_ok=True)
    with tempfile.TemporaryDirectory() as t:
        w = os.path.join(t, "v.wav"); tts(text, "Samantha", w)
        speech = rms_scale(read(w), -22)
    lead = [0.0] * int(1.0 * RATE)
    s = lead + speech
    reference = None
    if mixed:
        b = rms_scale(bed(len(s) / RATE), -38)
        s = [x + b[i] for i, x in enumerate(s)]
        reference = b
    # a little noise floor so the adaptive floor has something to learn
    import random; random.seed(7)
    s = [x + random.uniform(-0.002, 0.002) for x in s]
    to_caf(s, os.path.join(d, "mic.caf"))
    if reference: to_caf(reference, os.path.join(d, "reference.caf"))
    words = [w.strip(".,?").lower() for w in text.split()]
    ctx = {"position": {"programmeID": "fixture-1", "blockIndex": 2,
                        "context": "Residents said the quiet was strange at first, and then they stopped noticing it.",
                        "questions": [
                            {"id": "asks_for_something", "text": "Does the listener ask for something to be done or looked up?", "options": ["yes", "no"]},
                            {"id": "carry_on", "text": "Should the programme carry on meanwhile?", "options": ["yes", "no"]}]},
           "preparedMaterial": "Fixture prepared material: the harbour park opened after the cranes moved.",
           "inputs": [], "expectedWords": ["tell", "more", "park"]}
    json.dump(ctx, open(os.path.join(d, "context.json"), "w"), indent=2, sort_keys=True)

make_turn("plain", "Please tell me more about the harbour and the park.", mixed=False)
make_turn("mixed", "Please tell me more about the harbour and the park.", mixed=True)

# the programme clip: a different voice, five seconds or so
os.makedirs(os.path.join(ROOT, "programme"), exist_ok=True)
with tempfile.TemporaryDirectory() as t:
    w = os.path.join(t, "c.wav"); tts("This is a short clip from another source, played inside the edition.", "Daniel", w)
    to_caf(rms_scale(read(w), -24), os.path.join(ROOT, "programme", "clip.caf"))
print("fixtures written under", ROOT)
