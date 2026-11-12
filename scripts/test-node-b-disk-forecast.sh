#!/usr/bin/env bash
# Behavioural test for `node-b-growth-meter --forecast` (storage assessment
# B2): replays canned hourly ledgers through the real script.
#
#   bash scripts/test-node-b-disk-forecast.sh
#
# Failure list, written before the code:
#   D1  a steady -16 GiB/day decline from 147 GiB (the 2026-11-01 to 11-06
#       shape) is not a warning (<4>) naming days to the 60 GiB floor
#   D2  a decline that reaches the floor in under 3 days is not an error (<3>)
#   D3  a flat or rising disk is a warning
#   D4  one transient dip that recovers (a deploy) is read as a trend: the
#       daily minimum slope must stay near zero
#   D5  a fall of more than 15 GiB in 6 hours is not a fast-drain warning, or
#       a fall of 10 GiB is
#   D6  the top growers are not the five largest 24 h increases by ledger
#       key, largest first, or list more than five, or list a shrinking key,
#       or count fs.root.* / .excl / .shared
#   D7  fewer than three days of ledger crashes or is silent
#   D8  a stale ledger (no reading for over 3 h) is not a warning
#   D9  the service and timer are not wired in nix/node-b.nix, or the unit
#       lacks idle priority
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/node-b-growth-meter"
nixfile="$here/../nix/node-b.nix"
fail=0; pass=0
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
check() { if eval "$2"; then pass=$((pass+1)); else fail=$((fail+1)); echo "FAIL: $1"; sed 's/^/    /' "$root/out"; fi; }

# gen <shape> <hours>: hourly rows ending 2026-11-12T00:00:00Z, written to
# $root/ledger/2026-11.jsonl (the ledger is the meter's own format: ts and
# items). Shapes are python expressions of h (hours before the end, 0 = newest).
gen() {
  rm -rf "$root/ledger"; mkdir -p "$root/ledger"
  python3 - "$1" "$2" "$root/ledger/2026-11.jsonl" <<'PY'
import json, sys, time
shape, hours, path = sys.argv[1], int(sys.argv[2]), sys.argv[3]
G = 1 << 30
end = 1794441600  # 2026-11-12T00:00:00Z
rows = []
for h in range(hours, -1, -1):
    t = end - h * 3600
    ns = {"h": h, "G": G, "t": t}
    avail = int(eval(shape, ns) * G)
    items = {"fs.root.avail": avail, "fs.root.size": 500 * G, "fs.root.used": 500 * G - avail}
    extra = ns.get("items")
    rows.append({"ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t)), "items": items, "units": {}})
with open(path, "w") as f:
    for r in rows:
        f.write(json.dumps(r) + "\n")
PY
}
forecast() { # [env...]
  env -u BASH_ENV GROWTH_DIR="$root/ledger" GROWTH_NOW=1794441600 "$@" python3 "$script" --forecast >"$root/out" 2>&1
  echo $? >"$root/rc"
}

# D1: the 2026-11-06 shape: 147 GiB free and falling 16 GiB a day
# ((147-60)/16 = 5.4 days to the floor).
gen '147 + 16 * h / 24' 168; forecast
check "D1 147 GiB, -16/day: warning naming 5.4 days to the 60 GiB floor" 'grep -q "^<4>disk forecast WARNING: .*5\.4 days to the 60 GiB floor" "$root/out" && grep -q "free 147 GiB" "$root/out"'
gen '100 + 40 * h / 24' 168; forecast
check "D2 100 GiB, -40/day (1.0 day): error" 'grep -q "^<3>disk forecast ERROR: .*1\.0 days" "$root/out"'
gen '138 + 16 * h / 24' 168; forecast
check "D1 138 GiB, -16/day (4.9 days): warning" 'grep -q "^<4>disk forecast WARNING: .*4\.9 days" "$root/out"'
gen '330 + 10 * h / 24' 168; forecast
check "D1 330 GiB, -10/day (27 days): ok" 'grep -q "^<6>disk forecast ok: .*27\.0 days" "$root/out"'
check "D2 exit 0" '[ "$(cat "$root/rc")" = 0 ]'
gen '40 + 1 * h / 24' 168; forecast
check "D2 already under the floor: error" 'grep -q "^<3>disk forecast ERROR: .*already below the 60 GiB floor" "$root/out"'

gen '90 - 5 * h / 24' 168; forecast
check "D3 rising disk: ok, not declining" 'grep -q "^<6>disk forecast ok: .*not declining" "$root/out"'

# D4: flat at 140 with a one-day dip to 70 on day 3 that recovers
gen '70 if 72 <= (168 - h) < 96 else 140' 168; forecast
check "D4 a recovered dip is not a trend" 'grep -q "^<6>disk forecast ok" "$root/out" && ! grep -q "^<[34]>disk forecast" "$root/out"'

# D5: fast drain
gen '140 - (20 if h < 6 else 0)' 168; forecast
check "D5 20 GiB in 6 h: fast-drain warning" 'grep -q "^<4>disk fast drain WARNING: free fell 20\.0 GiB in 6\.0 h" "$root/out"'
gen '140 - (10 if h < 6 else 0)' 168; forecast
check "D5 10 GiB in 6 h: no fast-drain warning" '! grep -q "fast drain WARNING" "$root/out" && grep -q "^<6>disk fast drain ok" "$root/out"'
gen '140 - (15 if h < 6 else 0)' 168; forecast
check "D5 exactly 15 GiB: no warning" '! grep -q "fast drain WARNING" "$root/out"'

# D6: growers. 49 hourly rows; every key is flat until the row 24 h before
# the newest, then grows linearly by its delta.
gen '140' 48
python3 - "$root/ledger/2026-11.jsonl" <<'PY'
import json, sys
G = 1 << 30
p = sys.argv[1]
rows = [json.loads(l) for l in open(p)]
grow = {"fs.a": 1, "fs.b": 9, "fs.c": 5, "fs.d": 3, "fs.e": 7, "fs.f": 2, "fs.shrink": -8, "fs.g.real": 25, "fs.g.excl": 25, "fs.g.shared": 25}
for i, r in enumerate(rows):
    frac = max(0, (i - 24) / 24)
    for k, v in grow.items():
        r["items"][k] = int((100 + v * frac) * G)
    r["items"]["fs.root.used"] = int((100 + 60 * frac) * G)
    r["items"]["pg.t.rows"] = int(1000 + 5e6 * frac)
open(p, "w").write("".join(json.dumps(r) + "\n" for r in rows))
PY
forecast
check "D6 five largest growers, largest first" 'grep "^<6>disk growers" "$root/out" | grep -q "fs.g.real +25\.0 GiB.*fs.b +9\.0 GiB.*fs.e +7\.0 GiB.*fs.c +5\.0 GiB.*fs.d +3\.0 GiB"'
check "D6 no sixth, no shrinker, no root, no excl/shared, no rows" '! grep "disk growers" "$root/out" | grep -q "fs\.f\|fs\.a\b\|fs.shrink\|fs.root\|fs.g.excl\|fs.g.shared\|pg.t.rows"'

# D7/D8
gen '140' 30; forecast
check "D7 under 3 days of ledger: info line, exit 0" 'grep -q "^<6>disk forecast: need at least 3 days" "$root/out" && [ "$(cat "$root/rc")" = 0 ]'
rm -rf "$root/ledger"; mkdir "$root/ledger"; forecast
check "D7 empty ledger: info line, exit 0" 'grep -q "need at least 3 days" "$root/out" && [ "$(cat "$root/rc")" = 0 ]'
gen '140' 168; forecast GROWTH_NOW=$((1794441600 + 4 * 3600))
check "D8 newest reading 4 h old: stale warning" 'grep -q "^<4>disk forecast WARNING: .*newest ledger reading is 4\.0 h old" "$root/out"'

# D10: under the floor already, flat: not "ok"
gen '40' 168; forecast
check "D10 under the floor and flat: error saying so" 'grep -q "^<3>disk forecast ERROR: .*below the 60 GiB floor" "$root/out"'
# D11: growers span is the real span after a gap (base reading 72 h old)
gen '140' 168
python3 - "$root/ledger/2026-11.jsonl" <<'PY'
import json, sys, calendar, time
p = sys.argv[1]
rows = [json.loads(l) for l in open(p)]
G = 1 << 30
end = calendar.timegm(time.strptime(rows[-1]["ts"], "%Y-%m-%dT%H:%M:%SZ"))
keep = []
for r in rows:
    t = calendar.timegm(time.strptime(r["ts"], "%Y-%m-%dT%H:%M:%SZ"))
    if not (end - 72 * 3600 < t < end - 2 * 3600):
        r["items"]["fs.x"] = (100 if t <= end - 72 * 3600 else 130) * G
        keep.append(r)
open(p, "w").write("".join(json.dumps(r) + "\n" for r in keep))
PY
forecast
check "D11 growers line names the real span (72 h), not 24 h" 'grep -q "^<6>disk growers (72\.0 h): fs.x +30\.0 GiB" "$root/out"'
# D12: a big ledger is streamed (the unit has MemoryMax 128M; three months
# whole-loaded was about 209 MiB)
rm -rf "$root/ledger"; mkdir -p "$root/ledger"
python3 - "$root/ledger" <<'PY'
import json, sys, time
d = sys.argv[1]
G = 1 << 30
end = 1794441600
pad = {"fs.pad.%d" % i: i for i in range(400)}
units = {"u%d.service" % i: {"peak": i, "max": 1 << 30, "events": {"oom": 0}} for i in range(60)}
for mon, t0 in (("2026-09", end - 70 * 86400), ("2026-10", end - 40 * 86400), ("2026-11", end - 10 * 86400)):
    with open("%s/%s.jsonl" % (d, mon), "w") as f:
        for h in range(0, 30 * 24):
            t = t0 + h * 3600
            if t > end:
                break
            items = dict(pad); items["fs.root.avail"] = int((200 - (end - t) / 86400.0) * G)
            f.write(json.dumps({"ts": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t)), "items": items, "units": units, "pad": "x" * 6000}) + "\n")
PY
ledger_mb=$(du -sm "$root/ledger" | cut -f1)
cat >"$root/rss.py" <<'PY'
import resource, runpy, sys
sys.argv = ["x", "--forecast"]
try:
    runpy.run_path(sys.argv_script, run_name="__main__")
except SystemExit:
    pass
kb = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
print("MAXRSS_MB", kb // (1024 * 1024) if sys.platform == "darwin" else kb // 1024)
PY
sed -i.bak "s#sys.argv_script#'$script'#" "$root/rss.py"
env GROWTH_DIR="$root/ledger" GROWTH_NOW=1794441600 python3 "$root/rss.py" >"$root/out" 2>&1
rss_mb=$(awk '/MAXRSS_MB/ {print $2}' "$root/out")
check "D12 large ledger (${ledger_mb} MB): forecast still answers" 'grep -q "^<[346]>disk forecast" "$root/out"'
check "D12 peak RSS ${rss_mb} MB stays under 64 MB (whole-load was ~209 MB at 3 months)" '[ "${rss_mb:-9999}" -lt 64 ]'

# D9: nix wiring
svc="$(awk '/systemd.services.go-choir-disk-forecast = \{/,/^  \};/' "$nixfile")"
check "D9 service runs the meter with --forecast, isolated python" 'grep -q "node-b-growth-meter} --forecast" <<<"$svc" && grep -q "python3 -I" <<<"$svc"'
check "D9 service idle priority and GROWTH_DIR" 'grep -q "Nice = 19" <<<"$svc" && grep -q "IOSchedulingClass = \"idle\"" <<<"$svc" && grep -q "GROWTH_DIR=/var/lib/go-choir/growth-meter" <<<"$svc"'
check "D9 timer is wired" 'grep -A8 "systemd.timers.go-choir-disk-forecast" "$nixfile" | grep -q "OnCalendar"'

echo "node-b-disk-forecast: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
