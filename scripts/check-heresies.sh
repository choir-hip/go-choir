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

if ! command -v rg >/dev/null 2>&1; then
  echo "rg not found; skipping heresy detector" >&2
  exit 0
fi

export repo_root fail_on_regression
export report_path="${report_path:-}"

python3 - "$@" <<'PY'
import json, os, re, subprocess, sys

repo = os.environ["repo_root"]
manifest = os.path.join(repo, "docs", "heresy-detectors.md")
search_paths = [
    os.path.join(repo, "README.md"),
    os.path.join(repo, "AGENTS.md"),
    os.path.join(repo, "docs"),
    os.path.join(repo, "internal"),
    os.path.join(repo, "cmd"),
    os.path.join(repo, "frontend"),
    os.path.join(repo, "scripts"),
    os.path.join(repo, "specs"),
    os.path.join(repo, ".github"),
]

def count_pattern(pattern, excludes=None):
    # rg -F counts fixed-string occurrences. Sum per-file counts.
    # Per-row path exclusions can be listed in the Notes column as
    #   exclude: glob1, glob2, ...
    excludes = excludes or []
    cmd = ["rg", "-F", "--no-heading", "-c"]
    for glob in excludes:
        cmd.append("--glob")
        cmd.append("!" + glob.strip())
    cmd.append(pattern)
    cmd.extend(search_paths)
    try:
        result = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            check=False,
        )
    except FileNotFoundError:
        return 0
    if not result.stdout:
        return 0
    total = 0
    for line in result.stdout.splitlines():
        if ":" in line:
            try:
                total += int(line.rsplit(":", 1)[1])
            except ValueError:
                pass
    return total

# Parse the | ID | Detector family | Grep patterns | Target | Notes | table.
rows = []
if os.path.exists(manifest):
    with open(manifest) as f:
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
                n = count_pattern(p, excludes=excludes)
                pattern_hits[p] = n
                total_hits += n
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
    with open(os.environ["report_path"], "w") as f:
        json.dump(rows, f, indent=2)

if os.environ.get("fail_on_regression") == "true":
    non_zero = [r for r in rows if r["enforced"] and r["total_hits"] > 0]
    if non_zero:
        print(f"{len(non_zero)} heresy row(s) have non-zero hits", file=sys.stderr)
        sys.exit(1)
PY
