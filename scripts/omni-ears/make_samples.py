#!/usr/bin/env python3
"""Ten voice samples from LFM3-Omni-2B through /v1/realtime: five spoken questions (audio in, audio out) and five two-sentence radio
transitions read from text. make_samples.py <port> <sets root> <out dir>; writes WAVs and README.md (prompt, time to first audio, total).
Audio never goes in the repository."""
import json, os, subprocess, sys
port, root, out = sys.argv[1], sys.argv[2], sys.argv[3]
here = os.path.dirname(os.path.abspath(__file__)); os.makedirs(out, exist_ok=True); os.makedirs(f'{out}/in', exist_ok=True)
qs = json.load(open(f'{root}/questions_clean/references.json')); pick = [0, 4, 6, 10, 16]
tr = ["Mars rover finds salt. The rover has found salt in an ancient lake bed, which suggests water once stayed there for a long time.",
      "That was Jazz for a rainy Tuesday. Up next, the harbor council votes on the tide barrier, and what it could cost the east channel.",
      "Thanks for staying with us. Coming up, the sourdough revival, and why bakers in the city are queueing before dawn.",
      "You are listening to the station. After the break, a match report: the city wins again, and the crowd goes home happy.",
      "Tides and the Moon is next. It is a short one about why the water rises twice a day, and why the harbor cares."]
rows = ['# Omni voice samples, 2026-11-24', '', 'LFM3-Omni-2B Q8_0 + mmproj Q8_0, llama.cpp b12259 `llama serve`, `/v1/realtime` WebSocket, Mac (not a phone). Time to first audio is measured from `response.create` to the first `response.output_audio.delta`; total to `response.done`.', '', '| file | kind | prompt | first audio (s) | total (s) | audio (s) | the model text |', '| --- | --- | --- | --- | --- | --- | --- |']
def run(inp, outp, instr, mods='["audio"]'):
    p = subprocess.run([sys.executable, f'{here}/s2s_realtime.py', port, inp, outp, instr], capture_output=True, text=True, env={**os.environ, 'OMNI_MODS': mods})
    return json.loads(p.stdout.strip().splitlines()[-1])
for k, i in enumerate(pick):
    q = qs[i]; r = run(f"{root}/questions_clean/{q['id']}.wav", f'{out}/answer{k + 1}.wav', 'Respond with interleaved text and audio.', '["text","audio"]')
    rows.append(f"| answer{k + 1}.wav | spoken question, spoken answer | audio of \"{q['text']}\" (macOS `say`, voice {q['voice']}) | {r['first_audio_s_after_response_create']:.2f} | {r['total_s']:.2f} | {r['audio_s']:.1f} | {r['text'][:80]} |")
for k, t in enumerate(tr):
    open(f'{out}/in/t{k + 1}.txt', 'w').write(t); r = run(f'{out}/in/t{k + 1}.txt', f'{out}/transition{k + 1}.wav', 'Perform TTS in American English using a female voice.')
    rows.append(f"| transition{k + 1}.wav | text read aloud | \"{t}\" (system: Perform TTS in American English using a female voice.) | {r['first_audio_s_after_response_create']:.2f} | {r['total_s']:.2f} | {r['audio_s']:.1f} | |")
open(f'{out}/README.md', 'w').write('\n'.join(rows) + '\n'); print('\n'.join(rows))
