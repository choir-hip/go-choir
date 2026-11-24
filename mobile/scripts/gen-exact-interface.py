#!/usr/bin/env python3
"""Writes mobile/exact-package/exact.json (the @choir/autoradio interface) and package.json."""
import json, os

HERE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "exact-package")
os.makedirs(os.path.join(HERE, "apple"), exist_ok=True)
S, N, B = "string", "number", "bool"
shapes = {
    "LabState": {"running": B, "paused": B, "phase": S, "level": S, "addressing": S, "transcript": S, "configName": S,
                 "blockIndex": N, "blockCount": N, "blockText": S, "residentMB": N, "thermal": S, "message": S, "micActive": B},
    # Listener path (Choir client)
    "ChoirAuth": {"signedIn": B, "baseURL": S, "status": S, "message": S},
    "ChoirPlayer": {"signedIn": B, "loading": B, "title": S, "passage": S, "playing": B, "talking": B,
                    "heard": S, "message": S},
}

def src(n, ans, opts=None, default=None):
    d = {"source": n, "live": True, "answers": ans}
    if opts: d["options"] = opts
    d["substitute"] = {"default": default}
    return d

def snd(n, opts=None):
    d = {"send": n}
    if opts: d["options"] = opts
    d["answers"] = "LabState"
    return d

empty_state = {"running": False, "paused": True, "phase": "playing", "level": "play", "addressing": "", "transcript": "",
               "configName": "", "blockIndex": 0, "blockCount": 0, "blockText": "", "residentMB": 0, "thermal": "nominal",
               "message": "", "micActive": False}
names = [
    src("choirAuth", "ChoirAuth", default={"signedIn": False, "baseURL": "", "status": "idle", "message": ""}),
    src("choirPlayer", "ChoirPlayer", default={"signedIn": False, "loading": False, "title": "", "passage": "",
        "playing": False, "talking": False, "heard": "", "message": ""}),
    snd("choirRefresh"),
    snd("choirTalkCancel"), snd("choirHandsFreeToggle"),
    snd("choirAsk", {"text": S, "aim": S}),
]
# Per-lane fragments: exact-package/interface/<lane>.json = {"shapes": {...}, "names": [...]}. Each lane owns its own
# file, so no two lanes edit the same one. A shape or a source/send name declared twice is an error.
IFACE = os.path.join(HERE, "interface")
views = {}
seen = {(n.get("source") or n.get("send")) for n in names}
for f in sorted(os.listdir(IFACE)) if os.path.isdir(IFACE) else []:
    if not f.endswith(".json"): continue
    frag = json.load(open(os.path.join(IFACE, f)))
    for k, v in frag.get("shapes", {}).items():
        if k in shapes: raise SystemExit("%s: shape %s is declared twice" % (f, k))
        shapes[k] = v
    for n in frag.get("names", []):
        key = n.get("source") or n.get("send")
        if key in seen: raise SystemExit("%s: name %s is declared twice" % (f, key))
        seen.add(key); names.append(n)
    for k, v in frag.get("views", {}).items():
        if k in views: raise SystemExit("%s: view %s is declared twice" % (f, k))
        views[k] = v

json.dump({"kind": "interface", "name": "@choir/autoradio", "version": "1.0.0", "substitute": "implementation",
           "shapes": shapes, **({"views": views} if views else {}), "names": names}, open(os.path.join(HERE, "exact.json"), "w"), indent=2)
json.dump({"name": "@choir/autoradio", "version": "1.0.0", "private": True,
           "exports": {".": "./exact.json", "./apple": "./apple/exact.json"}}, open(os.path.join(HERE, "package.json"), "w"), indent=2)
print("wrote", HERE)
