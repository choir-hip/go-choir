#!/usr/bin/env python3
"""Markdown table from one or more rows TSVs: one line per (model, set, threads),
with the number of runs and every run's value (no averaging that hides spread).
  summarize.py rows.tsv [more.tsv ...]
"""
import csv
import sys
from collections import OrderedDict


def load(paths):
    out = []
    for p in paths:
        with open(p, encoding="utf-8") as f:
            out += list(csv.DictReader(f, delimiter="\t"))
    return out


def table(rs):
    groups = OrderedDict()
    for r in rs:
        groups.setdefault((r["model_id"], r["set"], r["threads"]), []).append(r)
    lines = ["| model | set | threads | runs | WER | RTF | load s | peak RSS MiB | thermal (before/after) | status |",
             "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |"]
    for (m, s, t), g in groups.items():
        col = lambda k: " / ".join(x[k] or "-" for x in g)
        th = " / ".join("%s>%s" % (x["thermal_before"], x["thermal_after"]) for x in g)
        lines.append("| %s | %s | %s | %d | %s | %s | %s | %s | %s | %s |" % (
            m, s, t, len(g), col("wer"), col("rtf"), col("load_seconds"), col("peak_rss_mib"), th, col("status")))
    return "\n".join(lines)


if __name__ == "__main__":
    print(table(load(sys.argv[1:])))
