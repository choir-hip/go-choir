#!/usr/bin/env python3
"""Fails when a user-visible string says the app is loading or opening ("Loading your mail...", "Opening…", "Opening the show").
Owner, 2026-11-17: a page shows its frame and an animated skeleton (no words) while content is on its way; an error is still said in
words with a retry. Scans the string literals of AutoradioLab/*.contract and AutoradioKit/Sources/**/*.swift (comments and machine values/ids such as "loading" and "doc-loading" are skipped; "Downloading" is a different word and stays). Run from anywhere: scripts/check-no-loading.py"""
import os, re, sys
HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.join(HERE, "..")
BAD = re.compile(r"\bloading\b|\bopening(\.\.\.|…| the | your | this )", re.I)
STR = re.compile(r'"((?:[^"\\\n]|\\.)*)"')
def scan(path, comment):
    out = []
    for n, line in enumerate(open(path, encoding="utf-8", errors="replace"), 1):
        code = line.split(comment)[0] if comment in line and not re.search(r'"[^"]*' + re.escape(comment), line) else line
        for m in STR.finditer(code):
            t = m.group(1)
            if re.fullmatch(r"[a-z0-9:_./-]+", t): continue   # a machine value or an id ("loading"), never drawn
            if BAD.search(t): out.append((path, n, t))
    return out
found = []
for d, ext, cm in (("AutoradioLab", ".contract", "//"), ("AutoradioKit/Sources", ".swift", "//")):
    for dp, dn, fn in os.walk(os.path.join(ROOT, d)):
        if "node_modules" in dp or "/.build" in dp or "/modules/" in dp: continue
        for f in fn:
            if f.endswith(ext) and not f.endswith(".generated.swift"): found += scan(os.path.join(dp, f), cm)
if found:
    for p, n, t in found: print("%s:%d: user-visible waiting word: %r" % (os.path.relpath(p, ROOT), n, t))
    print("check-no-loading: %d string(s) say Loading or Opening; show the frame and a skeleton instead (mobile/docs, app-motion)" % len(found)); sys.exit(1)
print("check-no-loading: no user-visible Loading or Opening string")
