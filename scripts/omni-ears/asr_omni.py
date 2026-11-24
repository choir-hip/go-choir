#!/usr/bin/env python3
"""Transcribe set folders with LFM3-Omni-2B through a running llama-server (b12259, mmproj loaded), one request per clip.

asr_omni.py <sets root> <out dir> <port> <set> [<set> ...]
Writes <out dir>/<set>.jsonl: {id, hyp, wall_s, prompt_ms, predicted_ms, prompt_n, predicted_n, audio_s}.
Prompt is the model card's ASR prompt, greedy: system "Perform ASR in English.", the clip as input_audio, no user text.
"""
import base64, json, os, sys, time, urllib.request
import soundfile as sf

root, out, port, sets = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
os.makedirs(out, exist_ok=True)
SYSTEM = os.environ.get('OMNI_SYSTEM', 'Perform ASR in English.')
for name in sets:
    refs = json.load(open(f'{root}/{name}/references.json'))
    with open(f'{out}/{name}.jsonl', 'w') as f:
        for r in refs:
            p = f"{root}/{name}/{r['id']}.wav"
            w = base64.b64encode(open(p, 'rb').read()).decode()
            body = {'messages': [{'role': 'system', 'content': SYSTEM}, {'role': 'user', 'content': [{'type': 'input_audio', 'input_audio': {'data': w, 'format': 'wav'}}]}], 'temperature': 0, 'max_tokens': 400}
            t = time.time()
            d = json.load(urllib.request.urlopen(urllib.request.Request(f'http://localhost:{port}/v1/chat/completions', json.dumps(body).encode(), {'Content-Type': 'application/json'}), timeout=300))
            wall = time.time() - t
            tm = d.get('timings', {}); info = sf.info(p)
            f.write(json.dumps({'id': r['id'], 'hyp': d['choices'][0]['message']['content'], 'wall_s': wall, 'prompt_ms': tm.get('prompt_ms'), 'predicted_ms': tm.get('predicted_ms'), 'prompt_n': tm.get('prompt_n'), 'predicted_n': tm.get('predicted_n'), 'audio_s': info.duration}) + '\n')
    print('done', name, flush=True)
