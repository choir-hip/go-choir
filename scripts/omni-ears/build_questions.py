#!/usr/bin/env python3
"""The 20 spoken questions (macOS `say`, four voices) in three conditions: clean, over a second speaker's voice at 5 dB (the radio bed:
the same interferer streams build_sets.py makes, band-limited 300 Hz to 6 kHz), and over DEMAND cafeteria noise at 5 dB.
Run after build_sets.py:  build_questions.py <sets root>   (expects <root>/questions_clean/{q00..q19.wav, references.json} made by `say`)."""
import json, os, shutil, sys, glob
import numpy as np, soundfile as sf, pyarrow.parquet as pq, subprocess
from scipy.signal import butter, sosfilt
root = sys.argv[1]; SR = 16000
rng = np.random.default_rng(7)
def to16k(raw):
    p = subprocess.run(['ffmpeg', '-loglevel', 'error', '-i', 'pipe:0', '-f', 'f32le', '-ac', '1', '-ar', str(SR), 'pipe:1'], input=raw, capture_output=True, check=True)
    return np.frombuffer(p.stdout, dtype=np.float32).copy()
t = pq.ParquetFile(f'{root}/dl/librispeech_asr_parquet_all_validation.clean_0.parquet').read().to_pylist()
sp = [r for r in t if str(r['speaker_id']) == '1462'][:40]
bed = np.concatenate([to16k(r['audio']['bytes']) for r in sp])
bed = sosfilt(butter(4, [300, 6000], btype='band', fs=SR, output='sos'), bed).astype(np.float32)
cafe = sf.read(glob.glob(f'{root}/noise/PCAFETER/*/ch01.wav')[0], dtype='float32')[0]
rms = lambda x: float(np.sqrt(np.mean(x ** 2) + 1e-12))
refs = json.load(open(f'{root}/questions_clean/references.json'))
for name, src in (('questions_radio5', bed), ('questions_cafe5', cafe)):
    d = f'{root}/{name}'; os.makedirs(d, exist_ok=True); shutil.copy(f'{root}/questions_clean/references.json', d)
    for r in refs:
        s = sf.read(f"{root}/questions_clean/{r['id']}.wav", dtype='float32')[0]
        # the interferer runs a little before and after the question, as a bed does: pad 1 s of it on both sides
        pad = np.concatenate([np.zeros(SR, np.float32), s, np.zeros(SR, np.float32)])
        st = int(rng.integers(0, len(src) - len(pad))); seg = src[st:st + len(pad)]
        seg = seg * (rms(s) / rms(seg)) * 10 ** (-5 / 20)
        sf.write(f"{d}/{r['id']}.wav", np.clip(pad + seg, -1, 1), SR, subtype='PCM_16')
print('ok')
