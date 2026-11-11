#!/usr/bin/env python3
"""Checks AutoradioLab/strings/en.json: no duplicate keys (json.load silently keeps the last), and every key starts with
a known lane prefix (purpose.* for the permission strings, then bar., paper., reader., cards., files., mail., settings.,
onboarding., mark., app.). Lanes add their strings under their own prefix, so no two lanes edit the same line."""
import json, os, sys
PREFIXES = ("purpose.", "app.", "bar.", "paper.", "reader.", "cards.", "launcher.", "files.", "viewers.", "mail.",
            "settings.", "onboarding.", "interview.", "mark.", "lab.")
path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "AutoradioLab", "strings", "en.json")
seen, dup = set(), []
def hook(pairs):
    for k, _ in pairs:
        if k in seen: dup.append(k)
        seen.add(k)
    return dict(pairs)
json.load(open(path), object_pairs_hook=hook)
bad = [k for k in seen if not k.startswith(PREFIXES)]
if dup or bad:
    print("strings: duplicate keys %s; keys without a lane prefix %s" % (dup, bad)); sys.exit(1)
print("strings: %d keys, no duplicates" % len(seen))
