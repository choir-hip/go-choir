"""Word error rate with one fixed normalisation, standard library only.

Normalisation (the same on reference and hypothesis, for every model):
  1. lowercase;
  2. every character that is not a letter, a digit or an apostrophe becomes a
     space (so "well-known" is two words and "1,000" is "1 000");
  3. apostrophes not inside a word are dropped ("'quoted'" -> "quoted");
     apostrophes inside a word stay ("don't");
  4. whitespace is collapsed.
Numbers are NOT expanded to words and no spelling variants are merged, so a
model that writes "20" where the reference says "twenty" is charged for it.
"""
import re
from collections import namedtuple

Errors = namedtuple("Errors", "subs dels ins ref_words")
Errors.total = property(lambda self: self.subs + self.dels + self.ins)


def normalize(text):
    text = text.lower()
    text = re.sub(r"[^\w']", " ", text.replace("_", " "))
    text = re.sub(r"(?<![^\W_])'|'(?![^\W_])", " ", text)
    return " ".join(text.split())


def errors(ref, hyp):
    r = normalize(ref).split()
    h = normalize(hyp).split()
    # d[i][j] = (total, subs, dels, ins) for r[:i] against h[:j].
    prev = [(j, 0, 0, j) for j in range(len(h) + 1)]
    for i in range(1, len(r) + 1):
        cur = [(i, 0, i, 0)]
        for j in range(1, len(h) + 1):
            t, s, d, n = prev[j - 1]
            cands = []
            if r[i - 1] == h[j - 1]:
                cands.append((t, s, d, n))
            else:
                cands.append((t + 1, s + 1, d, n))
            t, s, d, n = prev[j]
            cands.append((t + 1, s, d + 1, n))
            t, s, d, n = cur[j - 1]
            cands.append((t + 1, s, d, n + 1))
            cur.append(min(cands))
        prev = cur
    _, s, d, n = prev[len(h)]
    return Errors(s, d, n, len(r))


def corpus_wer(pairs):
    """Total errors over total reference words (not a mean of per-utterance rates)."""
    tot = Errors(0, 0, 0, 0)
    for ref, hyp in pairs:
        e = errors(ref, hyp)
        tot = Errors(tot.subs + e.subs, tot.dels + e.dels, tot.ins + e.ins, tot.ref_words + e.ref_words)
    if tot.ref_words == 0:
        raise ValueError("no reference words")
    return tot.total / tot.ref_words
