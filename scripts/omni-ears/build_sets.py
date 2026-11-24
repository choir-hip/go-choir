#!/usr/bin/env python3
"""Build the speech sets the Omni-ears measurement uses (developer side; nothing here ships).

Run with the venv that has numpy, soundfile, pyarrow, scipy:  build_sets.py <root>
<root>/dl must hold: librispeech validation.clean and validation.other parquet files (openslr/librispeech_asr, `all` config)
and the first AMI sdm test parquet (edinburghcstr/ami); <root>/noise holds DEMAND PCAFETER and STRAFFIC (Zenodo 1227121, CC BY 4.0).
<root>/clean is the earlier builder's 73-clip set (~/.cache/ar-real-speech/clean) and is read, never written.

Each set is a folder of 16 kHz mono wavs plus references.json [{id, text}] (the harness's own format, lower-cased).
"""
import io, json, os, subprocess, sys, glob
import numpy as np, soundfile as sf, pyarrow.parquet as pq
from scipy.signal import butter, sosfilt

root = sys.argv[1]
SR = 16000
rng = np.random.default_rng(20261124)

def to16k(raw):
    p = subprocess.run(['ffmpeg', '-loglevel', 'error', '-i', 'pipe:0', '-f', 'f32le', '-ac', '1', '-ar', str(SR), 'pipe:1'], input=raw, capture_output=True, check=True)
    return np.frombuffer(p.stdout, dtype=np.float32).copy()

def write_set(name, items):  # items: [(id, samples, text)]
    d = f'{root}/{name}'; os.makedirs(d, exist_ok=True)
    for i, s, _ in items: sf.write(f'{d}/{i}.wav', np.clip(s, -1, 1), SR, subtype='PCM_16')
    json.dump([{'id': i, 'text': t} for i, _, t in items], open(f'{d}/references.json', 'w'))
    print(name, len(items), 'clips', round(sum(len(s) for _, s, _ in items) / SR / 60, 1), 'min')

def words(s):
    import re
    return [w for w in re.split(r'[\W_]+', s.lower().replace('harbour', 'harbor')) if w]

# clean: the earlier builder's folder
clean = json.load(open(os.path.expanduser('~/.cache/ar-real-speech/clean/references.json')))
clean_items = [(r['id'], sf.read(os.path.expanduser(f"~/.cache/ar-real-speech/clean/{r['id']}.wav"), dtype='float32')[0], r['text']) for r in clean]
write_set('clean', clean_items)
clean_speakers = {r['id'].split('-')[0] for r in clean}

# other: first 100 clips of validation.other
t = pq.ParquetFile(f'{root}/dl/librispeech_asr_parquet_all_validation.other_0.parquet').read().to_pylist()[:100]
items = [(r['id'], to16k(r['audio']['bytes']), r['text'].lower()) for r in t]
write_set('other', items); print('  other words', sum(len(words(x[2])) for x in items))

# ami: single distant microphone test, clips of 4 words or more, first 47
t = pq.ParquetFile(f'{root}/dl/ami0.parquet').read().to_pylist()
items = []
for r in t:
    if len(words(r['text'])) >= 4:
        items.append((r['audio_id'], to16k(r['audio']['bytes']), r['text'].lower()))
    if len(items) == 47: break
write_set('ami', items); print('  ami words', sum(len(words(x[2])) for x in items))

# interferer: a second LibriSpeech speaker (validation.clean, speakers not in the clean set), one long stream per speaker
t = pq.ParquetFile(f'{root}/dl/librispeech_asr_parquet_all_validation.clean_0.parquet').read().to_pylist()
by = {}
for r in t:
    sp = str(r['speaker_id'])
    if sp not in clean_speakers: by.setdefault(sp, []).append(r)
spk = sorted(by)[:6]
streams = []
for sp in spk:
    s = np.concatenate([to16k(r['audio']['bytes']) for r in by[sp][:40]])
    streams.append(s)
print('interferer speakers', spk, [round(len(s) / SR) for s in streams], 's')

# the radio's voice from a phone speaker: no energy below ~300 Hz, little above ~6 kHz
sos = butter(4, [300, 6000], btype='band', fs=SR, output='sos')
radio = [sosfilt(sos, s).astype(np.float32) for s in streams]

def noise(name):
    x, sr = sf.read(glob.glob(f'{root}/noise/{name}/*/ch01.wav')[0], dtype='float32')
    assert sr == SR; return x

def rms(x): return float(np.sqrt(np.mean(x ** 2) + 1e-12))

def mix(speech, interferer, snr):
    n = len(speech); start = int(rng.integers(0, len(interferer) - n))
    seg = interferer[start:start + n]
    seg = seg * (rms(speech) / rms(seg)) * 10 ** (-snr / 20)
    return speech + seg

cafe, traffic = noise('PCAFETER'), noise('STRAFFIC')
for snr in (10, 5, 0):
    write_set(f'clean_cafe{snr}', [(i, mix(s, cafe, snr), tx) for i, s, tx in clean_items])
    write_set(f'clean_traffic{snr}', [(i, mix(s, traffic, snr), tx) for i, s, tx in clean_items])
    write_set(f'clean_radio{snr}', [(i, mix(s, radio[k % len(radio)], snr), tx) for k, (i, s, tx) in enumerate(clean_items)])

# no speech at all: silence, room tone, cafeteria and street noise (reference empty; any word is invented)
none = []
for k, secs in enumerate((2, 3, 5, 8, 12)): none.append((f'silence{k}', np.zeros(secs * SR, np.float32), ''))
for k, secs in enumerate((3, 5, 8)):
    none.append((f'roomtone{k}', rng.normal(0, 0.003, secs * SR).astype(np.float32), ''))
for k, secs in enumerate((3, 5, 8, 10)):
    # cafeteria noise has babble in it: that is the hard case for a speech model, and it is not speech aimed at the phone
    st = int(rng.integers(0, len(cafe) - secs * SR)); none.append((f'cafe{k}', cafe[st:st + secs * SR] * 0.5, ''))
for k, secs in enumerate((3, 5, 8, 10)):
    st = int(rng.integers(0, len(traffic) - secs * SR)); none.append((f'traffic{k}', traffic[st:st + secs * SR] * 0.5, ''))
write_set('nospeech', none)
