#!/usr/bin/env python3
"""Spoken questions to LFM3-Omni-2B through a running llama-server: the audio is the whole user turn (no transcription asked),
system prompt "Respond with text." (the model card's text-answer prompt). Also a control: the same question typed as text.
ask_omni.py <sets root> <out dir> <port> <set> [<set> ...]   -> <out dir>/<set>.jsonl {id, question, answer, mode}. A person grades by hand."""
import base64, json, os, sys, urllib.request
root, out, port, sets = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
os.makedirs(out, exist_ok=True)
def chat(user):
    body = {'messages': [{'role': 'system', 'content': 'Respond with text.'}, {'role': 'user', 'content': user}], 'temperature': 0, 'max_tokens': 200}
    return json.load(urllib.request.urlopen(urllib.request.Request(f'http://localhost:{port}/v1/chat/completions', json.dumps(body).encode(), {'Content-Type': 'application/json'}), timeout=300))['choices'][0]['message']['content']
for name in sets:
    refs = json.load(open(f'{root}/{name}/references.json'))
    with open(f'{out}/{name}.jsonl', 'w') as f:
        for r in refs:
            w = base64.b64encode(open(f"{root}/{name}/{r['id']}.wav", 'rb').read()).decode()
            f.write(json.dumps({'id': r['id'], 'question': r['text'], 'expected': r['answer'], 'mode': 'audio', 'answer': chat([{'type': 'input_audio', 'input_audio': {'data': w, 'format': 'wav'}}])}) + '\n')
    print('done', name, flush=True)
with open(f'{out}/text_control.jsonl', 'w') as f:
    for r in json.load(open(f'{root}/questions_clean/references.json')):
        f.write(json.dumps({'id': r['id'], 'question': r['text'], 'expected': r['answer'], 'mode': 'text', 'answer': chat(r['text'])}) + '\n')
