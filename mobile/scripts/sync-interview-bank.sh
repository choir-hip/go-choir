#!/bin/sh
# Copies mobile/interview/questions.md into InterviewBank.defaultText (Interview.swift). InterviewTests fails if they differ.
set -eu
HERE="$(cd "$(dirname "$0")/.." && pwd)"
python3 -I - "$HERE" <<'PY'
import sys, re
here = sys.argv[1]
b = open(here + "/interview/questions.md").read().rstrip("\n")
p = here + "/AutoradioKit/Sources/AutoradioKit/Interview.swift"
s = open(p).read()
s = re.sub(r'(public static let defaultText = #"""\n).*?(\n"""#)', lambda m: m.group(1) + b + m.group(2), s, flags=re.S)
open(p, "w").write(s)
PY
