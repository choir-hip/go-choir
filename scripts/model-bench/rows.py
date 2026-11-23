"""The TSV row writer: one row per model per phone per set."""
import os

COLUMNS = [
    "phone", "soc", "android", "runtime", "provider", "threads", "model_id", "quant",
    "model_bytes", "set", "utterances", "audio_seconds", "wer", "subs", "dels", "ins",
    "ref_words", "load_seconds", "decode_seconds", "rtf", "peak_rss_mib", "wall_seconds",
    "thermal_before", "thermal_after", "battery_temp_before", "battery_temp_after",
    "plugged", "mem_available_mib_before", "mem_available_mib_after", "run", "date", "harness_commit", "status", "notes",
]


def _clean(v):
    return str(v).replace("\t", " ").replace("\r", " ").replace("\n", " ")


def append_row(path, row):
    unknown = set(row) - set(COLUMNS)
    if unknown:
        raise KeyError("unknown column(s): %s" % sorted(unknown))
    new = not os.path.exists(path) or os.path.getsize(path) == 0
    with open(path, "a", encoding="utf-8") as f:
        if new:
            f.write("\t".join(COLUMNS) + "\n")
        f.write("\t".join(_clean(row.get(c, "")) for c in COLUMNS) + "\n")
