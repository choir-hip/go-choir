"""Parsers for what the phone prints: sherpa-onnx output, the wrapper's meta file, dumpsys."""
import json
import re


def parse_sherpa(text):
    load = decode = None
    m = re.search(r"[Rr]ecognizer created in ([0-9.]+) s", text)
    if m:
        load = float(m.group(1))
    m = re.search(r"^Elapsed seconds: ([0-9.]+) s$", text, re.M)  # offline binary only
    if m:
        decode = float(m.group(1))
    texts = []
    for line in text.splitlines():
        if line.startswith("{"):
            try:
                d = json.loads(line)
            except ValueError:
                continue
            if isinstance(d, dict) and "text" in d:
                texts.append(d["text"])
    return {"load_seconds": load, "decode_seconds": decode, "texts": texts}


def parse_meta(text):
    kv = dict(p.split("=", 1) for p in text.split())
    return {"exit": int(kv["exit"]), "wall_seconds": round(float(kv["wall_end"]) - float(kv["wall_start"]), 3),
            "vmhwm_kb": int(kv["vmhwm_kb"])}


def thermal_status(text):
    m = re.search(r"Thermal Status:\s*(\d+)", text)
    if not m:
        raise ValueError("no Thermal Status in dumpsys output")
    return int(m.group(1))


def battery(text):
    def field(name):
        m = re.search(r"^\s*%s:\s*(\S+)" % re.escape(name), text, re.M)
        if not m:
            raise ValueError("no %r in dumpsys battery" % name)
        return m.group(1)

    if field("AC powered") == "true":
        plugged = "ac"
    elif field("USB powered") == "true":
        plugged = "usb"
    else:
        plugged = "no"
    return {"plugged": plugged, "level": int(field("level")), "temp_c": int(field("temperature")) / 10.0}
