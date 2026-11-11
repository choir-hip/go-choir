#!/usr/bin/env python3
"""Writes mobile/exact-package/exact.json (the @choir/autoradio interface) and package.json."""
import json, os

HERE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "exact-package")
os.makedirs(os.path.join(HERE, "apple"), exist_ok=True)
S, N, B = "string", "number", "bool"
shapes = {
    "LabState": {"running": B, "paused": B, "phase": S, "level": S, "addressing": S, "transcript": S, "configName": S,
                 "blockIndex": N, "blockCount": N, "blockText": S, "residentMB": N, "thermal": S, "message": S, "micActive": B},
    "StageRow": {"kind": S, "kindLabel": S, "id": S, "name": S, "summary": S, "selected": B, "network": B, "models": S},
    "StageList": {"rows": "StageRow[]"},
    "ParamRow": {"kind": S, "stage": S, "name": S, "label": S, "value": S, "hint": S},
    "ParamList": {"rows": "ParamRow[]"},
    "TimingRow": {"stage": S, "startMs": N, "durMs": N},
    "Trace": {"id": S, "transcript": S, "configName": S, "outcome": S, "addressedBy": S, "totalMs": N, "ackMs": N,
              "revisionMs": N, "audioSwitchMs": N, "timings": "TimingRow[]", "lines": "string[]"},
    "TurnRow": {"id": S, "transcript": S, "configName": S, "outcome": S, "ackMs": N, "revisionMs": N},
    "TurnList": {"rows": "TurnRow[]"},
    "AudioInfo": {"state": S, "policy": S, "engine": S, "route": S, "message": S, "voiceProcessing": B, "audible": B,
                  "volume": N, "lastSwitchMs": N, "policies": "string[]"},
    "ExportInfo": {"path": S, "documents": S, "name": S},
    "Lines": {"lines": "string[]"},
    "Names": {"names": "string[]"},
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
    src("labState", "LabState", default=empty_state),
    src("labStages", "StageList", default={"rows": []}),
    src("labParams", "ParamList", default={"rows": []}),
    src("labTrace", "Trace", {"turn": S}, {"id": "", "transcript": "", "configName": "", "outcome": "", "addressedBy": "",
        "totalMs": 0, "ackMs": 0, "revisionMs": 0, "audioSwitchMs": 0, "timings": [], "lines": []}),
    src("labAudio", "AudioInfo", default={"state": "none", "policy": "switchOnAddress", "engine": "off", "route": "", "message": "",
        "voiceProcessing": False, "audible": False, "volume": 1, "lastSwitchMs": 0, "policies": []}),
    snd("labInject"),
    snd("labSetAudioPolicy", {"policy": S}),
    {"send": "labExportTurn", "options": {"turn": S}, "answers": "ExportInfo"},
    src("labTurns", "TurnList", default={"rows": []}),
    src("labEvents", "Lines", default={"lines": []}),
    src("labConfigs", "Names", default={"names": []}),
    snd("labStart"), snd("labStop"), snd("labTalkDown"), snd("labTalkUp"), snd("labPlayPause"), snd("labNext"), snd("labPrevious"),
    snd("labSelectStage", {"kind": S, "id": S}),
    snd("labSetParam", {"kind": S, "name": S, "value": S}),
    snd("labSaveConfig", {"name": S}),
    snd("labLoadConfig", {"name": S}),
    snd("labLoadConfigURL", {"url": S}),
    snd("labReplay", {"turn": S}),
]
json.dump({"kind": "interface", "name": "@choir/autoradio", "version": "1.0.0", "substitute": "implementation",
           "shapes": shapes, "names": names}, open(os.path.join(HERE, "exact.json"), "w"), indent=2)
json.dump({"name": "@choir/autoradio", "version": "1.0.0", "private": True,
           "exports": {".": "./exact.json", "./apple": "./apple/exact.json"}}, open(os.path.join(HERE, "package.json"), "w"), indent=2)
print("wrote", HERE)
