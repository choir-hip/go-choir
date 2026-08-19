#!/usr/bin/env bash
# Discovery-mode heresy detector script.
#
# Reads the detector manifest in docs/heresy-detectors.md and reports counts for
# each pattern. By default it is report-only (exits 0). Run with
#   --fail-on-regression
# to fail if any row promoted with `enforce: zero` has non-zero hits. Other rows
# remain report-only until their discovery baselines are classified.
#
# The manifest is the source of truth. This script parses it at runtime so new
# detector rows are picked up without script edits.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
fail_on_regression=false
report_path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fail-on-regression) fail_on_regression=true ; shift ;;
    --report) report_path="$2"; shift 2 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 not found; skipping heresy detector" >&2
  exit 0
fi

export repo_root fail_on_regression
export report_path="${report_path:-}"

python3 - "$@" <<'PY'
import fnmatch
import json
import os
import re
import subprocess
import sys

repo = os.environ["repo_root"]
manifest = os.path.join(repo, "docs", "heresy-detectors.md")

# Fast file collection: prefer git ls-files if available, else walk with ignores.
tracked_files = []
try:
    cmd = ["git", "ls-files"]
    result = subprocess.run(cmd, cwd=repo, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, check=True)
    tracked_files = [f.strip() for f in result.stdout.splitlines() if f.strip() and not f.startswith("vendor/")]
except Exception:
    ignored_dirs = {".git", "node_modules", "vendor", "dist", ".agentic-consensus", ".direnv", "tmp", "states"}
    for root, dirs, files in os.walk(repo):
        dirs[:] = [d for d in dirs if d not in ignored_dirs and not d.startswith(".")]
        rel_dir = os.path.relpath(root, repo)
        for f in files:
            if rel_dir == ".":
                tracked_files.append(f)
            else:
                tracked_files.append(os.path.join(rel_dir, f))

# Read all relevant text files into memory in a single pass.
file_contents = {}
for rel_path in tracked_files:
    full_path = os.path.join(repo, rel_path)
    try:
        with open(full_path, "r", encoding="utf-8", errors="ignore") as f:
            file_contents[rel_path] = f.read()
    except Exception:
        pass

# Parse the | ID | Detector family | Grep patterns | Target | Notes | table.
rows = []
if os.path.exists(manifest):
    with open(manifest, "r", encoding="utf-8") as f:
        text = f.read()
    # Find the detector manifest table.
    match = re.search(r"\n\| ID \| Detector family \| Grep patterns \| Target \| Notes \|\n(.*?)\n\## Baseline Counts", text, re.S)
    if match:
        table = match.group(1)
        for line in table.splitlines():
            line = line.strip()
            if not line.startswith("|") or line.startswith("| ---"):
                continue
            parts = [p.strip() for p in line.strip("|").split("|")]
            if len(parts) < 5:
                continue
            heresy_id, family, patterns_col, target, notes = parts[:5]
            if heresy_id == "ID":
                continue
            patterns = re.findall(r"`([^`]+)`", patterns_col)
            if not patterns:
                continue
            exclude_match = re.search(r"exclude:\s*([^;|]+)", notes)
            excludes = []
            if exclude_match:
                excludes = [g.strip() for g in exclude_match.group(1).split(",")]
            enforced = bool(re.search(r"enforce:\s*zero(?:\b|$)", notes))

            pattern_hits = {}
            total_hits = 0
            for p in patterns:
                hits = 0
                for rel_path, content in file_contents.items():
                    if excludes and any(fnmatch.fnmatch(rel_path, pat) or fnmatch.fnmatch(os.path.basename(rel_path), pat) for pat in excludes):
                        continue
                    hits += content.count(p)
                pattern_hits[p] = hits
                total_hits += hits

            rows.append({
                "id": heresy_id,
                "family": family,
                "target": target,
                "notes": notes,
                "enforced": enforced,
                "total_hits": total_hits,
                "patterns": pattern_hits,
            })

print(json.dumps(rows, indent=2))

if os.environ.get("report_path"):
    with open(os.environ["report_path"], "w", encoding="utf-8") as f:
        json.dump(rows, f, indent=2)

if os.environ.get("fail_on_regression") == "true":
    non_zero = [r for r in rows if r["enforced"] and r["total_hits"] > 0]
    if non_zero:
        print(f"{len(non_zero)} heresy row(s) have non-zero hits", file=sys.stderr)
        sys.exit(1)
PY
