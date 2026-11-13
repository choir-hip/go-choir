#!/usr/bin/env bash
# Behavioural test for scripts/status-paper-usage, scripts/status-storage and
# scripts/status-hourly (storage mission station S9). Each script splits
# fetching (ssh to Node B, read only) from computing; this test drives the
# computing through --fixture files of synthetic log lines, no ssh.
#
#   bash scripts/test-status-scripts.sh
#
# Failure list, written before the code:
#  paper usage
#   P1  a line outside the last hour is counted in "last hour", or a line from
#       before 00:00 UTC is counted in "today", or a future line is counted
#   P2  other computers' calls are counted in the paper's call counts, or are
#       left out of the Bedrock dollars (the budget is platform-wide)
#   P3  a "streaming inference succeeded" line is not counted
#   P4  shares: devin/swe-2-flash, the free fallbacks (iceman, beam, flint,
#       nemotron by model name) and Bedrock Haiku are not fractions of the
#       paper's calls in the window
#   P5  one Bedrock call is not priced from the repo price table with cache
#       read, 5m write, 1h write, uncached input and output split as the log
#       gives them: 1M prompt, 600k read, 400k write (100k of it 1h), 10k out
#       is $0.0685
#   P6  the projection is not today + last hour x hours left in the UTC day
#   P7  River calls and dollars, Synthetic calls are not reported
#   P8  provider=devin lines matching error|quota|429|hold are not counted by
#       kind, or a success line whose token count holds "429" counts as one
#   P9  the paper's lines in the last 20 minutes are not reported, or zero is
#       not a flag
#   P10 a flag is missing or spurious: Bedrock projected over $20; River over
#       $1; devin share under half; paper not publishing; any Synthetic or
#       River call from the paper; exit code is not non-zero with a flag
#   P11 empty input divides by zero or crashes
#  storage
#   S1  free MiB is not the Available column of df -Pm /
#   S2  the latest forecast, fast drain and growers lines are not shown, or the
#       unit's "Finished ... disk forecast (days to ...)" line is shown as the forecast
#   S3  drain over 1 h and 6 h is not from the ledger rows (GiB per hour,
#       positive = falling), or is computed from a baseline far from the window
#   S4  the paper's chunks and data image footprint is not the newest value
#   S5  the snapshot root time, head sequence and age are not shown, or a
#       missing root is not a flag
#   S6  flags: free under 65 GiB; 6 h drain over 1 GiB/h; no root in 24 h; the
#       forecast unit's own WARNING or ERROR line
#  hourly
#   H1  it does not print both reports then ONE last line under 200 chars
#   H2  the last line does not lead with the flags, or ends OK with a flag
#   H3  exit code is 0 with a flag, or 0 when a fetch fails
set -u
here="$(cd "$(dirname "$0")" && pwd)"
paper="$here/status-paper-usage"
storage="$here/status-storage"
hourly="$here/status-hourly"
fail=0; pass=0
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
check() { if eval "$2"; then pass=$((pass+1)); else fail=$((fail+1)); echo "FAIL: $1"; sed 's/^/    /' "$root/out"; fi; }
PAPERVM=vm-1669020adcece51c2f5ff9b6df399ef5
OTHERVM=vm-00000000000000000000000000000001
NOW=1794535200   # 2026-11-13T02:00:00Z

# mkpaper <python body>: writes $root/paper.json from python helpers
#   ok(ts, vm, provider, model, inp, out, cached, w, w1h, stream=False)
#   other(ts, text): a raw line
mkpaper() {
  python3 - "$root/paper.json" "$1" <<'PY'
import json, sys, time
out, body = sys.argv[1], sys.argv[2]
NOW = 1794535200
PAPER = "vm-1669020adcece51c2f5ff9b6df399ef5"
OTHER = "vm-00000000000000000000000000000001"
succ, devin = [], []
def stamp(t): return time.strftime("%Y/%m/%d %H:%M:%S", time.gmtime(t))
def ok(ago, vm, prov, model, inp=1000, outp=100, cached=0, w=0, w1h=0, stream=False):
    what = "streaming inference succeeded" if stream else "inference succeeded"
    succ.append(f"{stamp(NOW-ago)} gateway: {what} for autoputer {vm} (provider={prov} tokens={inp}+{outp} cached={cached} text_len=0 cache_write={w} cache_write_1h={w1h} model={model})")
def many(n, ago, vm, prov, model, **kw):
    for i in range(n): ok(ago + i % 3, vm, prov, model, **kw)
def derr(ago, text): devin.append(f"{stamp(NOW-ago)} gateway: {text}")
exec(body)
json.dump({"now": NOW, "success_lines": "\n".join(succ) + "\n", "devin_lines": "\n".join(devin) + "\n",
           "paper_lines_20m": globals().get("paper20", 500)}, open(out, "w"))
PY
}
runpaper() { env -u BASH_ENV python3 "$paper" --fixture "$root/paper.json" "$@" >"$root/out" 2>&1; echo $? >"$root/rc"; }
jq_() { python3 -c "import json,sys;d=json.load(open('$root/out'));print($1)"; }

# P1/P2/P3/P4: windows, vm filter, streaming, shares
mkpaper '
many(82, 600, PAPER, "devin", "swe-2-flash")
many(10, 700, PAPER, "opencode-go", "iceman-contributor-free")
many(3, 800, PAPER, "opencode-go", "beam-free")
many(2, 900, PAPER, "opencode-go", "nemotron-3-free")
many(3, 1000, PAPER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=1000, outp=100)
many(5, 5000, PAPER, "devin", "swe-2-flash")          # older than an hour, in today
many(4, 30000, PAPER, "devin", "swe-2-flash")         # 2026-11-12: yesterday
many(7, 1200, OTHER, "devin", "swe-2-flash")          # another computer
ok(300, PAPER, "devin", "swe-2-flash", stream=True)
ok(-60, PAPER, "devin", "swe-2-flash")                # future
'
runpaper --json
check "P1 last hour counts only (now-60m, now]: 82+10+3+2+3+1 streaming = 101" '[ "$(jq_ "d[\"calls\"][\"hour\"][\"total\"]")" = 101 ]'
check "P1 today counts from 00:00 UTC: 101 + 5 = 106, no yesterday, no future" '[ "$(jq_ "d[\"calls\"][\"today\"][\"total\"]")" = 106 ]'
check "P2 other computers are not in the paper counts" '[ "$(jq_ "d[\"calls\"][\"hour\"][\"by\"][\"devin/swe-2-flash\"]")" = 83 ]'
check "P3 streaming line counted (83 = 82 + 1)" '[ "$(jq_ "d[\"calls\"][\"hour\"][\"by\"][\"devin/swe-2-flash\"]")" = 83 ]'
check "P4 devin share 83/101" '[ "$(jq_ "round(d[\"shares\"][\"devin\"],4)")" = 0.8218 ]'
check "P4 free share (iceman, beam, nemotron) 15/101" '[ "$(jq_ "round(d[\"shares\"][\"free\"],4)")" = 0.1485 ]'
check "P4 haiku share 3/101" '[ "$(jq_ "round(d[\"shares\"][\"haiku\"],4)")" = 0.0297 ]'
check "P1 the human report names the window counts" 'env -u BASH_ENV python3 "$paper" --fixture "$root/paper.json" >"$root/out" 2>&1; grep -q "devin/swe-2-flash" "$root/out"'

# P5: one priced Bedrock call; P2: another computer's Bedrock counts in dollars
mkpaper 'ok(100, PAPER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=1000000, outp=10000, cached=600000, w=400000, w1h=100000)'
runpaper --json
check "P5 one Bedrock Haiku call is \$0.0685" '[ "$(jq_ "round(d[\"bedrock\"][\"hour_usd\"],4)")" = 0.0685 ]'
mkpaper '
ok(100, PAPER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=1000000, outp=10000, cached=600000, w=400000, w1h=100000)
ok(100, OTHER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=1000000, outp=10000, cached=600000, w=400000, w1h=100000)'
runpaper --json
check "P2 another computer's Bedrock call is in the dollars (0.137) not the calls" '[ "$(jq_ "round(d[\"bedrock\"][\"hour_usd\"],4)")" = 0.137 ] && [ "$(jq_ "d[\"calls\"][\"hour\"][\"total\"]")" = 1 ]'

# P6: projection. One \$1.00 call 30 min ago and one \$2.00 call at 00:30Z:
# today 3.00, last hour 1.00, 22 h left in the day at 02:00Z -> 3 + 22 = 25
mkpaper '
ok(1800, OTHER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=10000000, outp=0)
ok(5400, OTHER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=20000000, outp=0)'
runpaper --json
check "P6 today = 3.00, hour = 1.00" '[ "$(jq_ "round(d[\"bedrock\"][\"today_usd\"],2)")" = 3.0 ] && [ "$(jq_ "round(d[\"bedrock\"][\"hour_usd\"],2)")" = 1.0 ]'
check "P6 projected = 3 + 22 x 1 = 25 and flagged over \$20" '[ "$(jq_ "round(d[\"bedrock\"][\"projected_usd\"],2)")" = 25.0 ] && jq_ "d[\"flags\"]" | grep -q "bedrock"'
check "P10 a flag exits non-zero" '[ "$(cat "$root/rc")" = 1 ]'

# P7/P10: River and Synthetic from the paper
mkpaper '
many(60, 100, PAPER, "devin", "swe-2-flash")
many(2, 100, PAPER, "synthetic", "deepseek-v4.1-flash-1017")
many(3, 100, PAPER, "river", "deepseek-v4.1-flash-1017", inp=1000000, outp=0)'
runpaper --json
check "P7 river 3 calls, synthetic 2 calls" '[ "$(jq_ "d[\"river\"][\"today_calls\"]")" = 3 ] && [ "$(jq_ "d[\"synthetic\"][\"today_calls\"]")" = 2 ]'
check "P7 river dollars 3 x 1M x 0.12/M = 0.36" '[ "$(jq_ "round(d[\"river\"][\"today_usd\"],2)")" = 0.36 ]'
check "P10 synthetic/river from the paper is a flag" 'jq_ "d[\"flags\"]" | grep -q "synthetic" && jq_ "d[\"flags\"]" | grep -q "river"'
mkpaper 'many(3, 100, PAPER, "river", "deepseek-v4.1-flash-1017", inp=4000000, outp=0)
many(60, 100, PAPER, "devin", "swe-2-flash")'
runpaper --json
check "P10 river over \$1 (3 x 4M x 0.12 = 1.44) is a flag" 'jq_ "d[\"flags\"]" | grep -q "river over"'

# P8: devin error kinds
mkpaper '
many(60, 100, PAPER, "devin", "swe-2-flash", inp=14292)
derr(100, "devin: upstream status 429 too many requests (provider=devin)")
derr(110, "devin: upstream status 429 too many requests (provider=devin)")
derr(120, "provider=devin quota exhausted for key")
derr(130, "provider=devin hold: cooling down 30s")
derr(140, "provider=devin error: connection reset")
derr(5000, "provider=devin error: too old")
derr(100, "inference succeeded for autoputer vm-x (provider=devin tokens=14292+9 cached=0 text_len=0)")
derr(100, "inference request from autoputer vm-x (provider=devin model=swe-2-flash messages=429)")'
runpaper --json
check "P8 five error lines by kind: 429 x2, quota, hold, error" '[ "$(jq_ "d[\"devin_errors\"][\"total\"]")" = 5 ] && [ "$(jq_ "d[\"devin_errors\"][\"by_kind\"][\"429\"]")" = 2 ] && [ "$(jq_ "d[\"devin_errors\"][\"by_kind\"][\"quota\"]")" = 1 ] && [ "$(jq_ "d[\"devin_errors\"][\"by_kind\"][\"hold\"]")" = 1 ] && [ "$(jq_ "d[\"devin_errors\"][\"by_kind\"][\"error\"]")" = 1 ]'

# P9/P10: publishing, devin share
mkpaper 'many(60, 100, PAPER, "devin", "swe-2-flash")
paper20 = 0'
runpaper --json
check "P9 zero paper lines in 20 min: flag, count reported" '[ "$(jq_ "d[\"paper_lines_20m\"]")" = 0 ] && jq_ "d[\"flags\"]" | grep -q "publishing"'
mkpaper 'many(20, 100, PAPER, "devin", "swe-2-flash")
many(40, 100, PAPER, "opencode-go", "flint-free")'
runpaper --json
check "P10 devin share under half is a flag" 'jq_ "d[\"flags\"]" | grep -q "devin share"'
mkpaper 'many(90, 100, PAPER, "devin", "swe-2-flash")
many(10, 100, PAPER, "opencode-go", "iceman-contributor-free")'
runpaper --json
check "P10 clean: no flags, exit 0" '[ "$(jq_ "len(d[\"flags\"])")" = 0 ] && [ "$(cat "$root/rc")" = 0 ]'
mkpaper ''
runpaper --json
check "P11 empty input: no crash, JSON, only the publishing/devin flags allowed" '[ "$(jq_ "d[\"calls\"][\"hour\"][\"total\"]")" = 0 ] && [ "$(cat "$root/rc")" -le 1 ]'

# ---- storage ----------------------------------------------------------------
# mkstore <free_mib> <ledger python expr: avail GiB as function of ago_h> <snapshot age h or None>
mkstore() {
  python3 - "$root/store.json" "$1" "$2" "$3" <<'PY'
import json, sys, time
out, free_mib, shape, snap_age = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
NOW = 1794535200
G = 1 << 30
led = []   # one line per ledger row: the matched tokens, key order as the file has it
for h in range(8, -1, -1):
    t = NOW - 600 - h * 3600
    row = []
    if h == 0:
        row.append('"fs.artifacts.file-cas-chunks.computer-1355b83bced1613eb048ee28d66688a8":%d' % (150 * G))
    row.append('"fs.root.avail":%d' % int(eval(shape, {"h": h}) * G))
    if h == 0:
        row.append('"fs.vm-state.vm-1669020adcece51c2f5ff9b6df399ef5.data.img":%d' % (52 * G))
    row.append('"ts":"%s"' % time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t)))
    led.append(" ".join(row))
fc = ("<6>disk forecast ok: free 77 GiB, slope -0.5 GiB/day over 8 daily minima, 34.0 days to the 60 GiB floor\n"
      "<6>disk fast drain ok: free fell 0.4 GiB in 6.0 h (78 GiB -> 77 GiB)\n"
      "<6>disk growers (24.0 h): fs.artifacts.tape-seg +2.6 GiB, fs.a +1.0 GiB\n"
      "Finished go-choir disk forecast (days to the deploy floor, fast drain, top growers).\n")
snap = "" if snap_age == "None" else "a12c1997b1f7|10978774|3999|%d\n" % (NOW - int(float(snap_age) * 3600))
df = "Filesystem     1M-blocks   Used Available Capacity Mounted on\n/dev/md127        486720 396687   %d      84%% /\n" % free_mib
json.dump({"now": NOW, "df": df, "forecast": fc, "ledger": "\n".join(led) + "\n", "snapshot": snap}, open(out, "w"))
PY
}
runstore() { env -u BASH_ENV python3 "$storage" --fixture "$root/store.json" "$@" >"$root/out" 2>&1; echo $? >"$root/rc"; }

mkstore 77926 '78 + 0.5 * h / 6 * 0 + 0.25 * h' 3
runstore --json
check "S1 free MiB from df -Pm /" '[ "$(jq_ "d[\"free_mib\"]")" = 77926 ]'
check "S2 forecast lines shown" 'jq_ "d[\"forecast\"][\"forecast\"]" | grep -q "^disk forecast ok.*34.0 days" && jq_ "d[\"forecast\"][\"fast_drain\"]" | grep -q "fell 0.4" && jq_ "d[\"forecast\"][\"growers\"]" | grep -q "tape-seg"'
# avail = 78 + 0.25 h: 0.25 GiB/h fall; newest row at h=0 (10 min before now)
check "S3 drain 1 h = 0.25 GiB/h" '[ "$(jq_ "round(d[\"drain_gib_per_h\"][\"1h\"],2)")" = 0.25 ]'
check "S3 drain 6 h = 0.25 GiB/h" '[ "$(jq_ "round(d[\"drain_gib_per_h\"][\"6h\"],2)")" = 0.25 ]'
check "S4 footprint chunks 150 GiB, data image 52 GiB" '[ "$(jq_ "round(d[\"paper\"][\"chunks_gib\"])")" = 150 ] && [ "$(jq_ "round(d[\"paper\"][\"data_img_gib\"])")" = 52 ]'
check "S5 snapshot head sequence and age 3 h" '[ "$(jq_ "d[\"snapshot\"][\"head_sequence\"]")" = 10978774 ] && [ "$(jq_ "round(d[\"snapshot\"][\"age_h\"],1)")" = 3.0 ]'
check "S6 healthy: no flags, exit 0" '[ "$(jq_ "len(d[\"flags\"])")" = 0 ] && [ "$(cat "$root/rc")" = 0 ]'
runstore
check "S1 human report names free space and the snapshot" 'grep -q "77926 MiB" "$root/out" && grep -q "snapshot" "$root/out"'

mkstore 66000 '78 + 0.25 * h' 3;  runstore --json
check "S6 66000 MiB (64.5 GiB) is under 65 GiB: flag" 'jq_ "d[\"flags\"]" | grep -q "free"'
mkstore 77926 '78 + 2 * h' 3;  runstore --json
check "S6 drain 2 GiB/h over 6 h: flag, exit 1" 'jq_ "d[\"flags\"]" | grep -q "drain" && [ "$(cat "$root/rc")" = 1 ]'
mkstore 77926 '78 + 0.25 * h' 3
python3 - "$root/store.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1])); d["forecast"] = d["forecast"].replace("disk forecast ok", "disk forecast ERROR"); json.dump(d, open(sys.argv[1], "w"))
PY
runstore --json
check "S6 the forecast unit's ERROR line is a flag" 'jq_ "d[\"flags\"]" | grep -q "forecast unit: forecast ERROR"'
mkstore 77926 '78 + 0.25 * h' 30;  runstore --json
check "S6 snapshot older than 24 h: flag" 'jq_ "d[\"flags\"]" | grep -q "snapshot"'
mkstore 77926 '78 + 0.25 * h' None;  runstore --json
check "S5 no snapshot root: flag" 'jq_ "d[\"flags\"]" | grep -q "snapshot" && [ "$(jq_ "d[\"snapshot\"]")" = None ]'
mkstore 77926 '78 + 0.25 * h' 3
python3 - "$root/store.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1])); d["ledger"] = ""; json.dump(d, open(sys.argv[1], "w"))
PY
runstore --json
check "S3 no ledger rows: drain is null, no crash, no drain flag" '[ "$(jq_ "d[\"drain_gib_per_h\"][\"6h\"]")" = None ] && ! jq_ "d[\"flags\"]" | grep -q drain'

# ---- hourly -----------------------------------------------------------------
mkpaper 'many(82, 600, PAPER, "devin", "swe-2-flash")
many(15, 700, PAPER, "opencode-go", "iceman-contributor-free")
many(3, 800, PAPER, "bedrock", "us.anthropic.claude-haiku-5-5", inp=1000, outp=100)'
mkstore 77926 '78 + 0.5 * h' 3
runhourly() { env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/paper.json" --storage-fixture "$root/store.json" "$@" >"$root/out" 2>&1; echo $? >"$root/rc"; }
runhourly
check "H1 exit 0 when clean" '[ "$(cat "$root/rc")" = 0 ]'
check "H1 both reports precede the last line" 'grep -q "paper usage" "$root/out" && grep -q "storage" "$root/out"'
last="$(tail -n 1 "$root/out")"
check "H1 last line under 200 chars" '[ ${#last} -lt 200 ]'
check "H1 last line is the example shape: 82% swe-2-flash, 15% free, 3% Haiku ... OK" 'echo "$last" | grep -q "^Paper: 82% swe-2-flash, 15% free, 3% Haiku\. Bedrock \$0\.0[0-9]*/20\. Disk 76 GiB, -0\.5/h\. Publishing\. OK$"'
mkpaper 'many(20, 600, PAPER, "devin", "swe-2-flash")
many(40, 700, PAPER, "opencode-go", "iceman-contributor-free")
paper20 = 0'
mkstore 60000 '60 + 0.5 * h' None
runhourly
last="$(tail -n 1 "$root/out")"
check "H2 flags lead the last line" 'echo "$last" | grep -q "^ACTION: " && ! echo "$last" | grep -q "OK$"'
check "H2 last line under 200 chars even with every flag" '[ ${#last} -lt 200 ]'
check "H3 exit 1 with flags" '[ "$(cat "$root/rc")" = 1 ]'
env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/missing.json" --storage-fixture "$root/store.json" >"$root/out" 2>&1; rc=$?
echo "$rc" >"$root/rc"
check "H3 a failed fetch is non-zero and says so on the last line" '[ "$rc" -ne 0 ] && tail -n 1 "$root/out" | grep -qi "unavailable"'
mkpaper 'many(90, 100, PAPER, "devin", "swe-2-flash")'; mkstore 77926 '78 + 0.5 * h' 3
runhourly --json
check "H1 --json emits one JSON object with both parts" 'python3 -c "import json;d=json.load(open(\"$root/out\"));assert \"paper\" in d and \"storage\" in d and \"line\" in d"'

echo "status-scripts: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
