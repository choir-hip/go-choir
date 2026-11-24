#!/usr/bin/env python3
"""Tool-call probe: 20 listener requests against a queue of six and three tools, through a running llama-server (OpenAI tools field, the
model's own chat template renders them; LFM3 writes <tool_call><function=name><parameter=k>v</parameter></function></tool_call>, per the card).
probe_tools.py <port> <out.jsonl>   -> prints counts. Scoring is automatic: valid call, right tool, right arguments, no call when none is needed, no invented id."""
import json, re, sys, time, urllib.request
port, out = sys.argv[1], sys.argv[2]
Q = [('a1', 'Tides and the Moon (science)'), ('b2', 'Harbor council votes on the barrier (local news)'), ('c3', 'Jazz for a rainy Tuesday (music)'),
     ('d4', 'The sourdough revival (food)'), ('e5', 'Mars rover finds salt (science)'), ('f6', 'Match report: city wins again (sport)')]
queue = '\n'.join(f'{i + 1}. id={a} "{t}"' for i, (a, t) in enumerate(Q)) + '\nNow playing: item 1 (id=a1).'
SYSTEM = 'You are the radio\'s voice. The queue:\n' + queue + '\nUse a tool only when the listener asks for an action on the queue; otherwise answer in one short sentence.'
TOOLS = [
 {'type': 'function', 'function': {'name': 'reorder_queue', 'description': 'Put the queue in a new order.', 'parameters': {'type': 'object', 'properties': {'item_ids': {'type': 'array', 'items': {'type': 'string'}}}, 'required': ['item_ids']}}},
 {'type': 'function', 'function': {'name': 'skip', 'description': 'Skip an item.', 'parameters': {'type': 'object', 'properties': {'item_id': {'type': 'string'}}, 'required': ['item_id']}}},
 {'type': 'function', 'function': {'name': 'say_transition', 'description': 'Say a short line before an item.', 'parameters': {'type': 'object', 'properties': {'text': {'type': 'string'}, 'before_item_id': {'type': 'string'}}, 'required': ['text', 'before_item_id']}}},
]
ids = {a for a, _ in Q}
def ro(first): return lambda a: isinstance(a.get('item_ids'), list) and a['item_ids'][:1] == [first] and set(a['item_ids']) <= ids and len(a['item_ids']) == len(set(a['item_ids']))
P = [  # (request, expected tool or None, argument check)
 ('Skip this one.', 'skip', lambda a: a.get('item_id') == 'a1'),
 ('Skip the jazz.', 'skip', lambda a: a.get('item_id') == 'c3'),
 ('Skip the sourdough one.', 'skip', lambda a: a.get('item_id') == 'd4'),
 ('Skip the sports story.', 'skip', lambda a: a.get('item_id') == 'f6'),
 ('Play the Mars one next.', 'reorder_queue', lambda a: isinstance(a.get('item_ids'), list) and a['item_ids'][:2] == ['a1', 'e5'] and set(a['item_ids']) <= ids),
 ('Move the harbor council story to the top.', 'reorder_queue', ro('b2')),
 ('Put the music first.', 'reorder_queue', ro('c3')),
 ('Play the science one next.', 'reorder_queue', None),  # ambiguous: two science items (a1 is playing, e5 is the other): a1 is playing, so e5 is the answer
 ('Say something before the jazz.', 'say_transition', lambda a: a.get('before_item_id') == 'c3' and len(a.get('text', '').split()) >= 3),
 ('Introduce the sourdough story with a line.', 'say_transition', lambda a: a.get('before_item_id') == 'd4' and len(a.get('text', '').split()) >= 3),
 ("What's after this?", None, None),
 ('What is playing now?', None, None),
 ('How many stories are left?', None, None),
 ('Thanks, that was nice.', None, None),
 ('Play the weather report next.', None, None),   # not in the queue: no call, and no invented id
 ('Skip the story about the election.', None, None),  # not in the queue
 ('Do something with the news.', None, None),  # ambiguous: no call
 ('Skip it.', None, None),  # ambiguous only if no now-playing; here "it" is a1: allowed either way, scored as skip a1 OR none
 ('Put the football report before the jazz.', 'reorder_queue', lambda a: isinstance(a.get('item_ids'), list) and a['item_ids'].index('f6') < a['item_ids'].index('c3') if set(a.get('item_ids', [])) >= {'f6', 'c3'} else False),
 ('Skip the one about tides.', 'skip', lambda a: a.get('item_id') == 'a1'),
]
P[7] = ('Play the science one next.', 'reorder_queue', lambda a: isinstance(a.get('item_ids'), list) and a['item_ids'][:2] == ['a1', 'e5'] and set(a['item_ids']) <= ids)
CALL = re.compile(r'<tool_call>\s*<function=([^>\n]+)>(.*?)</function>\s*</tool_call>', re.S); PAR = re.compile(r'<parameter=([^>\n]+)>\n?(.*?)\n?</parameter>', re.S)
def parse_raw(text):
    calls = []
    for n, body in CALL.findall(text):
        a = {}
        for k, v in PAR.findall(body):
            try: a[k] = json.loads(v)
            except Exception: a[k] = v
        calls.append((n, a))
    return calls
tot = dict(valid=0, tool=0, args=0, nocall_ok=0, nocall_n=0, call_n=0, invented=0, tokens=0, ms=0)
with open(out, 'w') as f:
    for req, tool, chk in P:
        body = {'messages': [{'role': 'system', 'content': SYSTEM}, {'role': 'user', 'content': req}], 'tools': TOOLS, 'temperature': 0, 'max_tokens': 300}
        d = json.load(urllib.request.urlopen(urllib.request.Request(f'http://localhost:{port}/v1/chat/completions', json.dumps(body).encode(), {'Content-Type': 'application/json'}), timeout=300))
        m = d['choices'][0]['message']; raw = m.get('content') or ''
        calls = [(c['function']['name'], json.loads(c['function']['arguments']) if isinstance(c['function']['arguments'], str) else c['function']['arguments']) for c in m.get('tool_calls', [])]
        parsed_ok = True
        if not calls and '<tool_call>' in raw:
            calls = parse_raw(raw); parsed_ok = bool(calls)
        tm = d.get('timings', {}); tot['tokens'] += tm.get('predicted_n', 0); tot['ms'] += tm.get('predicted_ms', 0)
        allids = [x for _, a in calls for x in ([a['item_id']] if isinstance(a.get('item_id'), str) else []) + (a['item_ids'] if isinstance(a.get('item_ids'), list) else []) + ([a['before_item_id']] if isinstance(a.get('before_item_id'), str) else [])]
        inv = any(x not in ids for x in allids); tot['invented'] += inv
        names = {'reorder_queue', 'skip', 'say_transition'}
        if req == 'Skip it.':  # either skip a1 or no call is right
            ok_t = (not calls) or (calls[0][0] == 'skip' and calls[0][1].get('item_id') == 'a1'); res = dict(valid=parsed_ok, tool=ok_t, args=ok_t)
            tot['valid'] += parsed_ok; tot['tool'] += ok_t; tot['args'] += ok_t
        elif tool is None:
            tot['nocall_n'] += 1; ok = not calls; tot['nocall_ok'] += ok; res = dict(valid=parsed_ok, tool=ok, args=ok)
        else:
            tot['call_n'] += 1; v = bool(calls) and parsed_ok and calls[0][0] in names; t = v and calls[0][0] == tool
            a = bool(t and chk(calls[0][1])) if t else False
            tot['valid'] += v; tot['tool'] += t; tot['args'] += a; res = dict(valid=v, tool=t, args=a)
        f.write(json.dumps({'request': req, 'expected': tool, 'calls': calls, 'raw': raw[:300], 'invented_id': inv, **res}) + '\n')
print(json.dumps(tot), 'tok/s', round(tot['tokens'] / max(tot['ms'] / 1000, 1e-9), 1))
