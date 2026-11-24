#!/usr/bin/env python3
"""Corpus word error from hypothesis files, with the harness's own normalisation
(FluidSpeechMiniTests.words: lower-case, 'harbour' -> 'harbor', split on every non-alphanumeric; word edit distance over reference length).

wer_table.py <sets root> <hyp dir> <set> [<set> ...]   (hyp dir holds <set>.jsonl with {id, hyp}); prints one line per set:
set  clips  ref_words  WER  sub  ins  del  and, for sets with an empty reference, the invented words.
"""
import json, re, sys

def words(s): return [w for w in re.split(r'[\W_]+', s.lower().replace('harbour', 'harbor')) if w]

def align(r, h):  # counts of (sub, ins, del) on a minimal-edit path
    n, m = len(r), len(h)
    d = [[0] * (m + 1) for _ in range(n + 1)]
    for i in range(n + 1): d[i][0] = i
    for j in range(m + 1): d[0][j] = j
    for i in range(1, n + 1):
        for j in range(1, m + 1):
            d[i][j] = min(d[i-1][j] + 1, d[i][j-1] + 1, d[i-1][j-1] + (r[i-1] != h[j-1]))
    i, j, s, ins, de = n, m, 0, 0, 0
    while i > 0 or j > 0:
        if i > 0 and j > 0 and d[i][j] == d[i-1][j-1] + (r[i-1] != h[j-1]):
            s += r[i-1] != h[j-1]; i -= 1; j -= 1
        elif i > 0 and d[i][j] == d[i-1][j] + 1: de += 1; i -= 1
        else: ins += 1; j -= 1
    return s, ins, de

def score(root, hyps, name):
    refs = {r['id']: r['text'] for r in json.load(open(f'{root}/{name}/references.json'))}
    nw = S = I = D = 0; invented = 0; nclip = 0; clips_with_words = 0
    for line in open(f'{hyps}/{name}.jsonl'):
        x = json.loads(line); r = words(refs[x['id']]); h = words(x['hyp'])
        s, i, d = align(r, h); nw += len(r); S += s; I += i; D += d; nclip += 1
        if not r: invented += len(h); clips_with_words += bool(h)
    return dict(clips=nclip, words=nw, wer=(S + I + D) / max(nw, 1), sub=S, ins=I, dele=D, invented=invented, clips_with_invented=clips_with_words)

if __name__ == '__main__':
    root, hyps = sys.argv[1], sys.argv[2]
    for name in sys.argv[3:]:
        x = score(root, hyps, name)
        if x['words'] == 0: print(f"{name}\t{x['clips']} clips\tinvented words {x['invented']} in {x['clips_with_invented']} clips")
        else: print(f"{name}\t{x['clips']}\t{x['words']}\t{100*x['wer']:.2f}%\tsub {x['sub']} ins {x['ins']} del {x['dele']}")
