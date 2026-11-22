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
mkpaper 'many(2, 100, PAPER, "river", "deepseek-ai/DeepSeek-V4.1-Flash-1017", inp=1000000, outp=0)'
runpaper --json
check "P7b river vendor-qualified model id is priced, not unpriced (2 x 0.12 = 0.24)" '[ "$(jq_ "round(d[\"river\"][\"today_usd\"],2)")" = 0.24 ]'
mkpaper '
many(60, 100, PAPER, "devin", "swe-2-flash")
many(2, 100, PAPER, "synthetic", "deepseek-v4.1-flash-1017")
many(3, 100, PAPER, "river", "deepseek-v4.1-flash-1017", inp=1000000, outp=0)'
runpaper --json
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

# P8b: the gateway's refusal form, "provider devin model", is counted as quota; over 20 flags
mkpaper '
many(60, 100, PAPER, "devin", "swe-2-flash", inp=14292)
for i in range(21): derr(100 + i, "provider call failed for autoputer vm-1669020adcece51c2f5ff9b6df399ef5: provider devin model swe-2-flash: provider quota exhausted (devin); resets at 2026-11-16T22:35:55Z (estimated) (paced: this thread uses the next pool member this hour; circuit open))")
derr(200, "provider=devin error: connection reset")'
runpaper --json
check "P8b refusal form counted: quota 21, other 1, flagged" '[ "$(jq_ "d[\"devin_errors\"][\"quota\"]")" = 21 ] && [ "$(jq_ "d[\"devin_errors\"][\"other\"]")" = 1 ] && grep -q "devin: 21 quota refusals in the last hour" "$root/out"'

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

# Gap signals (status-gaps lane)
#  G1 luna share and an other share, summing to 1 with the rest
#  G2 pacer: latest hour shed share, calls, trips; trips>0 flags
#  G3 search: requests, zero-provider share, per-provider counts; zero flags
#  G4 a stale forecast ERROR (free figure >10 GiB from live df) is not a flag
#  G5 wedge counts; began>cleared flags; receipts_since_last 0 flags
mkpaper '
many(60, 100, PAPER, "devin", "swe-2-flash")
many(30, 100, PAPER, "chatgpt", "gpt-6-luna")
many(10, 100, PAPER, "openrouter", "some-other-model")'
python3 - "$root/paper.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["pacer_lines"] = ("2026/11/13 16:00:00 gateway: devin pacing hour=2026-11-13T14:00Z tokens=1 ceiling=2 budget=3 shed_share=0% shed_calls=1 of 9 trips=0 complete=true unsheddable_tokens=0\n"
                    "2026/11/13 17:00:01 gateway: devin pacing hour=2026-11-13T16:00Z tokens=1 ceiling=2 budget=3 shed_share=12% shed_calls=58 of 1789 trips=2 complete=true unsheddable_tokens=0\n")
sk = "skipped=[tavily=quota_limited brave=quota_limited parallel=quota_limited serper=error serpapi=quota_limited chatgpt=capped]"
d["search_lines"] = "\n".join(["answered 1 of 1 providers (7 cooling, 0 pending) " + sk] * 9 + ["answered 0 of 1 providers (7 cooling, 0 pending) " + sk]) + "\n"
json.dump(d, open(sys.argv[1], "w"))
PY
runpaper --json
check "G1 luna 30%, other 10%, devin 60%" '[ "$(jq_ "round(d[\"shares\"][\"luna\"],2)")" = 0.3 ] && [ "$(jq_ "round(d[\"shares\"][\"other\"],2)")" = 0.1 ] && [ "$(jq_ "round(d[\"shares\"][\"devin\"],2)")" = 0.6 ]'
check "G2 pacer is the newest hour: shed 58 of 1789, trips 2, flagged" '[ "$(jq_ "d[\"pacer\"][\"shed_calls\"]")" = 58 ] && [ "$(jq_ "d[\"pacer\"][\"trips\"]")" = 2 ] && jq_ "d[\"flags\"]" | grep -q "pacer tripped"'
check "G3 search: 10 requests, 1 zero, tavily quota_limited 10, serper error 10, chatgpt capped 10; zero flagged" '[ "$(jq_ "d[\"search\"][\"requests\"]")" = 10 ] && [ "$(jq_ "d[\"search\"][\"zero\"]")" = 1 ] && [ "$(jq_ "d[\"search\"][\"by\"][\"tavily\"][\"quota_limited\"]")" = 10 ] && [ "$(jq_ "d[\"search\"][\"by\"][\"serper\"][\"error\"]")" = 10 ] && jq_ "d[\"flags\"]" | grep -q "got no provider answer"'
# G3b real line shape: answered by the one provider not skipped (named in today=), results counted
python3 - "$root/paper.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
real = "answered 1 of 1 providers (5 cooling, 0 pending) skipped=[searxng=success_empty tavily=quota_limited brave=quota_limited] served=miss results=%d unique=40 seen=6 routing=budget escalated=no paid_calls=0 today=[brave=7 keenable=%d]"
d["search_lines"] = "\n".join([real % (40, 9)] * 3 + [real % (0, 10)] + ["answered 0 of 1 providers (2 cooling, 0 pending) skipped=[brave=quota_limited] served=miss results=0 unique=0 seen=0 routing=budget escalated=no paid_calls=0 today=[brave=7]"]) + "\n"
json.dump(d, open(sys.argv[1], "w"))
PY
runpaper --json
check "G3b search: 5 requests, 3 with results, 1 zero, answered by keenable 4 and not brave" '[ "$(jq_ "d[\"search\"][\"requests\"]")" = 5 ] && [ "$(jq_ "d[\"search\"][\"with_results\"]")" = 3 ] && [ "$(jq_ "d[\"search\"][\"zero\"]")" = 1 ] && [ "$(jq_ "d[\"search\"][\"answered_by\"][\"keenable\"]")" = 4 ] && [ "$(jq_ "d[\"search\"][\"answered_by\"].get(\"brave\",0)")" = 0 ]'
runpaper
check "G3b text: results and provider lines" 'grep -q "5 requests, 3 returned results (60%), 1 got no provider answer" "$root/out" && grep -q "minus skipped=\[...\]): keenable 4" "$root/out"'
runpaper --json   # the check below read the text report before 2026-11-18 and so passed without looking
check "G3 serper erroring means not every paid provider is quota_limited" '! jq_ "d[\"flags\"]" | grep -q "every paid"'
# K: Keenable credits against the plan (problem record search-volume-outruns-keenable-credits-2026-11-18).
check "K1 keenable calls today are the largest today= count in the window, and a small day raises no flag" '[ "$(jq_ "d[\"search\"][\"keenable\"][\"today_calls\"]")" = 10 ] && ! jq_ "d[\"flags\"]" | grep -q "keenable"'
python3 - "$root/paper.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
real = "answered 1 of 1 providers (5 cooling, 0 pending) skipped=[brave=quota_limited] served=miss results=40 unique=40 seen=6 routing=budget escalated=no paid_calls=0 today=[keenable=%d]"
d["search_lines"] = "\n".join(real % n for n in range(801, 901)) + "\n"   # 100 answered this hour, 900 today at 02:00Z
d["keenable_plan"] = {"credits": 17000, "read_at": "2026-11-12T12:00:00Z", "renews": "2026-11-29T12:00:00Z"}   # 1000 a day reaches renewal
json.dump(d, open(sys.argv[1], "w"))
PY
runpaper --json
check "K2 900 today and 100 an hour with 22 h left projects 3100, over 1000 a day: flagged" '[ "$(jq_ "d[\"search\"][\"keenable\"][\"today_calls\"]")" = 900 ] && [ "$(jq_ "round(d[\"search\"][\"keenable\"][\"projected_today\"])")" = 3100 ] && [ "$(jq_ "round(d[\"search\"][\"keenable\"][\"budget_per_day\"])")" = 1000 ] && jq_ "d[\"flags\"]" | grep -q "keenable projected 3100 today, over the 1000 a day"'
runpaper
check "K2 text: the calls, the projection, the reading and how long it lasts" 'grep -q "keenable today UTC: 900 calls, projected 3100" "$root/out" && grep -q "17000 credits read 2026-11-12T12:00:00Z" "$root/out" && grep -q "lasts 5.5 days" "$root/out"'
python3 - "$root/paper.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["keenable_plan"] = {"credits": 17000, "read_at": "2026-10-20T00:00:00Z", "renews": "2026-11-05T00:00:00Z"}   # the plan renewed before now
json.dump(d, open(sys.argv[1], "w"))
PY
runpaper --json
check "K3 a reading older than the renewal is shown as stale and raises no flag" '! jq_ "d[\"flags\"]" | grep -q "keenable" && [ "$(jq_ "d[\"search\"][\"keenable\"][\"stale\"]")" = True ]'
runpaper
check "K3 text says the reading is stale" 'grep -q "reading is stale" "$root/out"'
mkstore 77926 '78 + 0.25 * h' 3
python3 - "$root/store.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["forecast"] = d["forecast"].replace("disk forecast ok: free 77 GiB", "disk forecast ERROR: free 40 GiB")
d["wedge"] = "2 computer event appender: wedge began\n5 computer event appender: wedge continues\n1 computer event appender: wedge cleared\nrc=0\n"
json.dump(d, open(sys.argv[1], "w"))
PY
runstore --json
check "G4 a stale forecast ERROR (40 vs 76 GiB live) is not a flag, and is shown stale" '! jq_ "d[\"flags\"]" | grep -q "forecast unit" && [ "$(jq_ "\"forecast\" in d[\"forecast_stale\"]")" = True ]'
check "G5 wedge began 2 cleared 1 is a flag" '[ "$(jq_ "d[\"wedge\"][\"began\"]")" = 2 ] && jq_ "d[\"flags\"]" | grep -q "wedge began 2, cleared 1"'
python3 - "$root/store.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["ledger"] += ' ts: "2026-11-13T01:59:00Z" "pg.control.receipts_since_last.computer-1355b83bced1613eb048ee28d66688a8": 0\n'
d["ledger"] = d["ledger"].replace(' ts:', ' "ts":')
json.dump(d, open(sys.argv[1], "w"))
PY
runstore --json
check "G5 receipts_since_last 0 on the newest row is a flag" '[ "$(jq_ "d[\"receipts_since_last\"]")" = 0 ] && jq_ "d[\"flags\"]" | grep -q "receipts_since_last"'

# status-paper-search-quality (search-reuse). Failure list, written before the code:
#  Q1  a story updated outside the UTC day, or an opinion piece, is counted
#  Q2  the mean counts a story with no source_count as zero
#  Q3  a story page that ends before the day began is not said to be incomplete
quality="$here/status-paper-search-quality"
python3 - "$root/front.json" <<'PY'
import json, sys
def row(t, n, kind=""):
    r = {"publication_id": "p", "title": "t", "updated_at": t}
    if n is not None: r["source_count"] = n
    if kind: r["kind"] = kind
    return r
pages = [{"stories": [row("2026-11-18T01:00:00Z", 9), row("2026-11-17T23:00:00Z", 4), row("2026-11-17T12:00:00Z", 8), row("2026-11-17T11:00:00Z", None), row("2026-11-17T10:00:00Z", 99, "opinion")], "next_cursor": "c"},
         {"stories": [row("2026-11-17T00:30:00Z", 6), row("2026-11-16T23:59:00Z", 50)]}]
json.dump(pages, open(sys.argv[1], "w"))
PY
env -u BASH_ENV python3 "$quality" --day 2026-11-17 --fixture "$root/front.json" >"$root/out" 2>&1
check "Q1/Q2 three counted stories and one without a count: 4 stories, mean of 4, 8 and 6 is 6.00" 'grep -q "stories published 4, mean distinct cited source URLs per story 6.00" "$root/out" && grep -q "1 stories have no source count" "$root/out"'
python3 - "$root/front.json" "$root/front1.json" <<'PY'
import json, sys
json.dump(json.load(open(sys.argv[1]))[:1], open(sys.argv[2], "w"))
PY
check "Q3 a list that reaches before the day is complete" '! grep -q "may be incomplete" "$root/out"'
env -u BASH_ENV python3 "$quality" --day 2026-11-17 --fixture "$root/front1.json" >"$root/out" 2>&1
check "Q3 a list that ends inside the day is said to be incomplete" 'grep -q "may be incomplete" "$root/out"'

# status-corpus-reuse (search-reuse). Failure list, written before the code:
#  R1  a query or note text reaches the output, in the text or the JSON
#  R2  a line of the older corpus shape (no text_index .. by_labels_only) is
#      dropped, or counted as a page-text miss instead of left out of that share
#  R3  the new summary fields are printed when no line carries them, or are not
#      summed when some do; wordings_sent / wordings_offered is wrong
#  R4  a window that starts before the oldest console line is not called partial
#  R5  an hour the console does not fully cover (no line before it, or none at
#      or after its end) is appended, or a covered hour is missing, or running
#      twice appends it twice, or the header is missing, or the file lands in the repo
#  R6  --first N --after T takes lines before T, takes more than N, or does not say
#      so when fewer than N exist
#  R7  held=true/own/false shares of the research desk's reads are wrong; another
#      desk's reads are counted in them
#  R8  runs with a web search and no corpus search are miscounted; the join is
#      not on run=
#  R9  the gateway count includes another computer, a line outside the window,
#      or prints the query
#  R11 gateway lines from before the console's first line (or after its last) are
#      counted when no --since is given, or an hour row counts another hour's lines
#  R12 a corpus search with caller=brief counts as the desk's own search: in the
#      per-run median, or in the runs-with-no-corpus-search count; a line without
#      the field is not a cell search; the brief count is missing from the hour row
#  R13 the research brief line's free-text reason is printed, or outcomes, median
#      ms, captures and tokens_est are wrong, or the old console without the line
#      prints a brief section
#  R14 a summary line with the wording fields BETWEEN not_sent (reasons) and
#      pages_returned (the deployed shape) finds no run; field order matters; an
#      unknown numeric field in the middle drops the line; the reasons inside
#      parentheses are read as fields or lost; the one-line label names the
#      console's span when --since/--until or --first narrowed the window
#  R16 not_sent reasons carry holds_unread from every verb: the report names the need test's refusals, the web
#      searches sent and the refused share of attempts (holds_unread / (holds_unread + started)); fresh_over_unread is
#      read (an older line's unread_notes still is) and summed, and printed
#  R15 the event layer's op=event_flush line is not read, or its sums, minted share or vector states are wrong, or a
#      console without the line prints the section
#  R10 the hourly script fails, or changes its last line, when the corpus
#      section is unavailable
corpus="$here/status-corpus-reuse"
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 3433.240153] 267hzahw99fgwgizzlinyajh0hnpc6ya-go-choir-run-autoputer-runtime[723]: "
def sr(ts, run, started, ret, new, read, extra="", reasons="", notsent=0, desk="research"):
    r = " (%s)" % reasons if reasons else ""
    return f"{PRE}{ts} runtime: search run={run} desk={desk} started={started} from_held=0 not_sent={notsent}{r} pages_returned={ret} pages_new={new} pages_read={read}{extra}\n"
def cs(ts, run, caps, new=True, body=0, both=0, desk="research"):
    f = f" text_index=true text_indexed=100 computer_pages=900 by_body={body} by_title=0 by_both={both} by_labels_only=1" if new else ""
    return (f'{PRE}{ts} corpus search computer=computer-aaaa captures_ms=5 captures={caps}{f} note="SECRET-NOTE \\"x\\"" '
            f'query="SECRET-QUERY \\"quoted\\" example.com" capture_ids= run={run} desk={desk} conv=conv-1 published=1 events=1 capped=false\n')
def rd(ts, run, held, via="read", desk="research"):
    return f"{PRE}{ts} corpus read run={run} desk={desk} conv=conv-1 capture=cap-1 held={held} via={via}\n"
NEWF = " search_all_calls=5 wordings_offered=10 wordings_sent=4 unread_notes=2 untagged=1"
# console A: 20:30 .. 23:10, whole hours 21:00 and 22:00 covered; 20:00 and 23:00 not.
a = [PRE + "2026/11/18 20:30:00 runtime: something else\n",
     sr("2026/11/18 21:10:00", "run-1", 4, 100, 80, 5, NEWF, "stopped_yielding=2", 2),
     sr("2026/11/18 21:20:00", "run-2", 6, 50, 40, 5),
     sr("2026/11/18 22:00:00", "run-3", 0, 0, 0, 1),            # exactly on the boundary: belongs to 22:00
     sr("2026/11/18 22:30:00", "run-4", 2, 10, 10, 0, desk="other"),
     cs("2026/11/18 21:09:00", "run-1", 2, True, body=1),
     cs("2026/11/18 21:19:00", "run-2", 0, False),               # old shape
     cs("2026/11/18 21:59:59", "run-3", 1, True, both=1),
     cs("2026/11/18 22:29:00", "run-4", 0, True, desk="other"),
     rd("2026/11/18 21:11:00", "run-1", "true"), rd("2026/11/18 21:11:01", "run-1", "own", "read_capture"),
     rd("2026/11/18 21:11:02", "run-1", "false"), rd("2026/11/18 21:12:00", "run-2", "false"),
     rd("2026/11/18 22:31:00", "run-4", "true", desk="other"),
     PRE + "2026/11/18 23:10:00 runtime: something else\n"]
open(root + "/cr_a.log", "w").write("".join(a))
# console B: only the new-field-free shape, 30 research runs from 21:00 on
b = [sr("2026/11/18 21:%02d:00" % i, "b-%d" % i, 1, 10, 5, 1) for i in range(30)] + [cs("2026/11/18 21:01:00", "b-0", 1)]
open(root + "/cr_b.log", "w").write("".join(b))
g = ["2026/11/18 21:10:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (query=\"SECRET-QUERY gw\" max_results=10 run=run-1 desk=research conv=c)\n",
     "2026/11/18 21:50:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (query=\"SECRET-QUERY gw\" max_results=10 run=run-2 desk=research conv=c)\n",
     "2026/11/18 21:50:00 gateway: search request from autoputer vm-00000000000000000000000000000001 (query=\"SECRET-QUERY gw\" max_results=10 run=x desk=research conv=c)\n",
     "2026/11/18 22:05:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (query=\"SECRET-QUERY gw\" max_results=10 run=run-3 desk=research conv=c)\n",
     "2026/11/18 19:00:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (query=\"SECRET-QUERY gw\" max_results=10 run=old desk=research conv=c)\n"]
open(root + "/cr_g.log", "w").write("".join(g))
PY
runcorpus() { env -u BASH_ENV python3 "$corpus" "$@" >"$root/out" 2>&1; echo $? >"$root/rc"; }
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "R1 no query or note text in the report" '! grep -q SECRET "$root/out"'
check "R3 4 runs searched; started 12; the new fields are summed over the one line that has them" 'grep -q "summary lines): 4" "$root/out" && grep -q "started (web searches sent) sum 12" "$root/out" && grep -q "wordings_offered 10, wordings_sent 4 (40.0%)" "$root/out"'
check "R3 not_sent reasons summed" 'grep -q "not_sent 2 (stopped_yielding=2)" "$root/out"'
check "R3 pages_read / pages_returned = 11/160 = 6.9%" 'grep -q "pages_returned 160, pages_new 130, pages_read 11; pages_read / pages_returned 6.9%" "$root/out"'
check "R2 research corpus searches: 3 lines, 2 with a capture; text hit 2 of the 2 lines with the fields" 'grep -q "research *3 (cell 3, brief 0); 66.7% returned one capture or more; page-text hit 100.0% (of 2 lines" "$root/out"'
check "R7 research reads: held 1, own 1, new 2; the other desk is apart" 'grep -q "held before the run 1 (25.0%), own (captured by this run) 1 (25.0%), new 2 (50.0%)" "$root/out" && grep -q "^  other  *1$" "$root/out"'
check "R8 every run has a corpus line under its own run=, so none made web searches without one (0 of 4)" 'grep -q "no corpus search at all: 0 of 4" "$root/out"'
check "R9 gateway: 3 paper lines in the window (the other computer and 19:00 are out)" 'grep -q "journal): 3 " "$root/out"'
check "R4 a window inside the console is not partial" '! grep -q PARTIAL "$root/out"'
runcorpus --fixture "$root/cr_a.log" --since 2026-11-18T19:00Z
check "R4 a window starting before the oldest line says it is partial" 'grep -q "PARTIAL window from 2026-11-18T20:30:00Z" "$root/out"'
runcorpus --fixture "$root/cr_b.log"
check "R3 no line carries the new fields: they are not printed" '! grep -q "wordings" "$root/out"'
check "R8 console B: 29 of 30 runs searched the web with no corpus search under their run=" 'grep -q "no corpus search at all: 29 of 30" "$root/out"'
runcorpus --fixture "$root/cr_a.log" --json
check "R1 JSON has no query text and carries a one-line summary" '! grep -q SECRET "$root/out" && [ "$(jq_ "len(d[\"line\"].splitlines())")" = 1 ]'
# R6
runcorpus --fixture "$root/cr_b.log" --first 24 --after 2026-11-18T21:05Z
check "R6 first 24 after 21:05: 24 found (the lines at 21:05..21:28), none before" 'grep -q "found 24" "$root/out" && grep -q "summary lines): 24" "$root/out"'
runcorpus --fixture "$root/cr_b.log" --first 24 --after 2026-11-18T21:20Z
check "R6 only 10 exist at or after 21:20: found 10, said plainly" 'grep -q "found 10 (fewer than 24 exist yet" "$root/out"'
# R5
tsv="$root/cr.tsv"
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log" --append "$tsv"
check "R5 whole hours 21:00 and 22:00 only, with a header" '[ "$(cut -f1 "$tsv")" = "$(printf "hour\n2026-11-18T21:00Z\n2026-11-18T22:00Z")" ]'
check "R5 the 21:00 row: 2 runs, 10 started; the boundary run (22:00:00) is in the 22:00 row" '[ "$(sed -n 2p "$tsv" | cut -f2,3)" = "$(printf "2\t10")" ] && [ "$(sed -n 3p "$tsv" | cut -f2,3)" = "$(printf "2\t2")" ]'
runcorpus --fixture "$root/cr_a.log" --append "$tsv"
check "R5 a second run appends nothing new" '[ "$(wc -l <"$tsv" | tr -d " ")" -eq 3 ] && grep -q "appended 0" "$root/out"'
runcorpus --fixture "$root/cr_a.log" --append "$here/../corpus-reuse-test.tsv"
check "R5 a file inside the repo is refused" '[ "$(cat "$root/rc")" = 2 ] && [ ! -e "$here/../corpus-reuse-test.tsv" ]'
# S1..S6 the gateway meter (station search-gauge; docs/evidence/who-sends-the-web-searches-2026-11-20.md).
# The console's `runtime: search run=` line is written only at a run's terminal record, so it loses runs still
# running, runs with no terminal record and counts lost to a restart (265 searches in the journal, 183 in the
# lines). The gateway request line carries run= and, as the journal is read, a query hash qh=.
#  S1  searches per research run are not computed from the gateway request lines grouped by run=: runs that searched,
#      sum, median, p90, max, the share of requests from runs with 8 or more; another computer's lines are in them;
#      served miss/hit/joined (from the answer lines) are not split into sent and from-cache
#  S2  the console-summary figure is not labelled as terminal records only, or the gap to the gateway is not a number
#  S3  same-run repeats (same run, same qh) are not counted, or a repeat across two runs is counted, or lines with no
#      qh print a number instead of saying the hash is absent; any query text, URL or computer id is printed
#  S4  research wakes per hour are not read from `actor wake latency ... target=research:`, or not split by kind, or
#      another desk's wake is counted
#  S6  the hour TSV has no runs-that-searched / searches-per-run columns (gateway basis), or an old file's header is
#      not widened with its rows kept (new cells empty), or a second run appends the hour twice
python3 - "$root" <<'PYGS'
import sys
root = sys.argv[1]
PRE = "[ 5.0] x-go-choir-run-autoputer-runtime[1]: "
PV = "vm-1669020adcece51c2f5ff9b6df399ef5"
def sr(ts, run, started):
    return f"{PRE}{ts} runtime: search run={run} desk=research started={started} from_held=0 not_sent=0 pages_returned=10 pages_new=5 pages_read=1\n"
def wk(ts, kind, tgt="research:aaaa"):
    return f"{PRE}{ts} runtime: actor wake latency kind={kind} target={tgt} waited=1.0s dispatch=0.5s\n"
L = [PRE + "2026/11/18 20:30:00 runtime: other\n",
     sr("2026/11/18 21:40:00", "run-A", 5), sr("2026/11/18 21:50:00", "run-B", 3),
     wk("2026/11/18 21:05:00", "initial_dispatch"), wk("2026/11/18 21:15:00", "initial_dispatch"),
     wk("2026/11/18 21:25:00", "inbox_record"), wk("2026/11/18 21:26:00", "initial_dispatch", "texture:bbbb"),
     wk("2026/11/18 22:05:00", "inbox_record"),
     PRE + "2026/11/18 23:10:00 runtime: other\n"]
open(root + "/gs_c.log", "w").write("".join(L))
def rq(ts, run, qh=None, vm=PV, desk="research"):
    h = "" if qh is None else f"qh={qh} "
    return f"{ts} gateway: search request from autoputer {vm} ({h}max_results=10 run={run} desk={desk} conv=c)\n"
def an(ts, served, vm=PV):
    return f"{ts} gateway: search for autoputer {vm} answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served={served} results=10 unique=10 seen=0 routing=budget escalated=no paid_calls=0 today=[keenable=1]\n"
G = []
t = 0
def tick():
    global t
    t += 60
    return "2026/11/18 21:%02d:%02d" % (t // 60, t % 60)
for qh in ("h1", "h1", "h2", "h3", "h4", "h5", "h6", "h7"):     # run-A: 8 requests, one same-run repeat (h1)
    G.append(rq(tick(), "run-A", qh))
for qh in ("h1", "h8", "h8"):                                    # run-B: 3, one repeat (h8); h1 also in run-A is not a repeat
    G.append(rq(tick(), "run-B", qh))
G.append(rq(tick(), "run-C", "h9"))
G.append(rq(tick(), "run-X", "h1", vm="vm-00000000000000000000000000000001"))   # another computer
for s_ in ["miss"] * 9 + ["hit"] * 2 + ["joined"]:
    G.append(an(tick(), s_))
G.append(an(tick(), "miss", vm="vm-00000000000000000000000000000001"))
open(root + "/gs_g.log", "w").write("".join(G))
# the same, with no hash on the request lines (the journal as it was before the host adds qh=)
open(root + "/gs_gn.log", "w").write("".join(__import__("re").sub(r"qh=\w+ ", "", l) for l in G))
PYGS
runcorpus --fixture "$root/gs_c.log" --gateway-fixture "$root/gs_g.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "S1 gateway meter: 3 runs searched, 12 requests, median 3, p90 8, max 8; runs with 8 or more sent 66.7%" 'grep -q "gateway meter (run= on the request lines): 3 runs searched, 12 requests, median 3, p90 8, max 8; runs with 8 or more sent 66.7% of the requests" "$root/out"'
check "S1 the sent / from-cache split is the answer lines: sent to providers 9, served from the cache 2, joined 1 (the other computer is out)" 'grep -q "served by the gateway: sent to providers 9, from the cache 2, joined an identical one in flight 1" "$root/out"'
check "S2 the console figure is labelled terminal records only and the gap is a number: 2 runs, 8 started; gateway 12; gap 4" 'grep -q "console summary lines (terminal records only): 2 runs, 8 started; gap to the gateway meter: 4 (12 - 8)" "$root/out"'
check "S3 same-run repeats: run-A 1 + run-B 1 = 2; h1 across two runs is not one" 'grep -q "same-run repeats (same query hash twice in one run): 2" "$root/out"'
check "S3 no query text, URL or computer id in the report" '! grep -qE "SECRET|https?://|vm-[0-9a-f]{8}|h1|h8" "$root/out"'
check "S4 research wakes per hour: 21:00 initial_dispatch 2, inbox_record 1 (the texture wake is out); 22:00 inbox_record 1" 'grep -q "2026-11-18T21:00Z  initial_dispatch 2, inbox_record 1" "$root/out" && grep -q "2026-11-18T22:00Z  initial_dispatch 0, inbox_record 1" "$root/out"'
runcorpus --fixture "$root/gs_c.log" --gateway-fixture "$root/gs_gn.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "S3 request lines with no hash: the repeats say so, as no number" 'grep -q "same-run repeats: not available (the gateway request lines carry no query hash)" "$root/out"'
runcorpus --fixture "$root/gs_c.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "S1 gateway not read: the meter says so, and no gap is invented" 'grep -q "gateway meter: not read" "$root/out" && ! grep -q "gap to the gateway meter" "$root/out"'
runcorpus --fixture "$root/gs_c.log" --gateway-fixture "$root/gs_g.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z --json
check "S1 JSON carries the meter" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); g=d[\"m\"][\"gw_meter\"]; assert g[\"runs\"]==3 and g[\"requests\"]==12 and g[\"p90\"]==8 and g[\"repeats\"]==2 and g[\"sent\"]==9 and d[\"m\"][\"gap\"]==4" "$root/out"'
# S6 the hour rows: columns 22 and 23, gateway basis (cr_g: run-1 and run-2 in the 21:00 hour, run-3 in the 22:00 hour)
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log" --append "$root/s6.tsv"
check "S6 the header ends gw_runs_searched, gw_searches_per_run; the 21:00 row has 2 runs and 1.0 a run, 22:00 has 1 and 1.0" '[ "$(head -1 "$root/s6.tsv" | cut -f22,23)" = "$(printf "gw_runs_searched\tgw_searches_per_run")" ] && [ "$(sed -n 2p "$root/s6.tsv" | cut -f22,23)" = "$(printf "2\t1.0")" ] && [ "$(sed -n 3p "$root/s6.tsv" | cut -f22,23)" = "$(printf "1\t1.0")" ]'
printf 'hour\truns\tstarted\n2026-11-18T20:00Z\t3\t7\n' >"$root/s6o.tsv"
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log" --append "$root/s6o.tsv"
check "S6 an old-format file keeps its row (new cells empty) under a widened header, then the new rows" '[ "$(head -1 "$root/s6o.tsv" | cut -f1-3)" = "$(printf "hour\truns\tstarted")" ] && [ "$(head -1 "$root/s6o.tsv" | awk -F"\t" "{print NF}")" = 23 ] && [ "$(sed -n 2p "$root/s6o.tsv" | cut -f1-3)" = "$(printf "2026-11-18T20:00Z\t3\t7")" ] && [ "$(sed -n 2p "$root/s6o.tsv" | awk -F"\t" "{print NF}")" = 23 ] && [ "$(wc -l <"$root/s6o.tsv" | tr -d " ")" = 4 ]'
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log" --append "$root/s6o.tsv"
check "S6 a second run appends nothing" '[ "$(wc -l <"$root/s6o.tsv" | tr -d " ")" = 4 ]'

# R15 the event layer's flush line (station V5): flushes, questions, minted share, vector states
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 3433.240153] 267hzahw99fgwgizzlinyajh0hnpc6ya-go-choir-run-autoputer-runtime[723]: "
def ef(ts, q, res, vec, state, un, ex, um, j, m, t, calls=1):
    return (f"{PRE}{ts} event layer: resolved {q} captures in {calls} calls: op=event_flush queued={q} research_reads={res} calls={calls} "
            f"shortlist_recent=30 shortlist_vector=4 shortlist_vector_new=2 vector={vec} vector_state={state} missing_unembedded={un} "
            f"missing_excluded={ex} vector_unmapped={um} joined={j} minted={m} tentative={t} unresolved=0 corroborated=0 cost_usd=0.00002\n")
lines = [PRE + "2026/11/18 20:30:00 runtime: something else\n",
         ef("2026/11/18 21:10:00", 10, 4, 25, "ok", 1, 0, 3, 4, 6, 0),
         ef("2026/11/18 21:20:00", 10, 0, 0, "partial", 0, 2, 0, 2, 7, 1, 2),
         ef("2026/11/18 21:30:00", 5, 5, "unavailable", "unavailable", 0, 0, 0, 0, 5, 0),
         ef("2026/11/18 23:30:00", 99, 0, 1, "ok", 0, 0, 0, 99, 0, 0),
         PRE + "2026/11/18 23:40:00 runtime: something else\n"]
open(root + "/cr_ef.log", "w").write("".join(lines))
PY
runcorpus --fixture "$root/cr_ef.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "R15 3 flushes in the window, 25 questions asked (9 from research's reads), 4 Jev calls" 'grep -q "event layer: 3 flushes, 25 questions asked (9 from research.s reads), 4 Jev calls" "$root/out"'
check "R15 joined 6, minted 18 (72.0% of the questions), tentative 1" 'grep -q "joined 6, minted 18 (72.0% of the questions), tentative 1" "$root/out"'
check "R15 vector states by count, unembedded, excluded and unmapped sums" 'grep -q "vector states of the flushes: ok 1, partial 1, unavailable 1; captures with no vector yet 1, excluded 2; neighbours mapped to no event 3" "$root/out"'
runcorpus --fixture "$root/cr_a.log" --since 2026-11-18T21:00Z --until 2026-11-18T23:00Z
check "R15 a console with no flush line prints no event layer section" '! grep -q "event layer:" "$root/out"'
# R11: no --since: the gateway's 19:00 line is before the console's first line (20:30)
runcorpus --fixture "$root/cr_a.log" --gateway-fixture "$root/cr_g.log"
check "R11 gateway lines before the console's first line are not counted (3, not 4)" 'grep -q "journal): 3 " "$root/out"'
check "R11 the hour rows count only their own hour's gateway lines (21:00: 2, 22:00: 1)" '[ "$(sed -n 2p "$tsv" | cut -f21)" = 2 ] && [ "$(sed -n 3p "$tsv" | cut -f21)" = 1 ]'
# R12/R13
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 1.0] x-go-choir-run-autoputer-runtime[1]: "
def sr(ts, run, started):
    return f"{PRE}{ts} runtime: search run={run} desk=research started={started} from_held=0 not_sent=0 pages_returned=10 pages_new=5 pages_read=1\n"
def cs(ts, run, caller):
    c = f" caller={caller}" if caller else ""
    return (f'{PRE}{ts} corpus search computer=c captures_ms=5 captures=1 note="n" query="SECRET-Q" capture_ids= run={run} '
            f'desk=research conv=v published=1 events=1 capped=false{c}\n')
def rb(ts, run, ms, caps, tok, outcome, reason, frm='unread'):
    if frm is None:  # a line from before objective_from and unread existed
        return (f'{PRE}{ts} research brief run={run} desk=research ms={ms} threads=1 queries=2 stories=3 captures={caps} passages=4 '
                f'newest_held=2026-11-18 bytes=10 tokens_est={tok} dropped=0 outcome={outcome} reason={reason}\n')
    return (f'{PRE}{ts} research brief run={run} desk=research ms={ms} threads=1 queries=2 stories=3 captures={caps} passages=4 '
            f'newest_held=2026-11-18 bytes=10 tokens_est={tok} dropped=0 objective_from={frm} unread=1 outcome={outcome} reason={reason}\n')
L = [PRE + "2026/11/18 20:30:00 other\n", PRE + "2026/11/18 22:10:00 other\n", sr("2026/11/18 21:30:00", "c-1", 3), sr("2026/11/18 21:31:00", "c-2", 2), sr("2026/11/18 21:32:00", "c-3", 0),
     cs("2026/11/18 21:01:00", "c-1", "brief"), cs("2026/11/18 21:02:00", "c-1", "cell"), cs("2026/11/18 21:03:00", "c-1", ""),
     cs("2026/11/18 21:04:00", "c-2", "brief"),   # brief only: c-2 searched the web with no cell corpus search
     rb("2026/11/18 21:01:00", "c-1", 100, 4, 1000, "delivered", '"SECRET-REASON with spaces"'),
     rb("2026/11/18 21:04:00", "c-2", 300, 0, 0, "empty", "SECRET-WORD"),
     rb("2026/11/18 21:05:00", "c-3", 200, 2, 2000, "failed", '"x"', "none"),
     rb("2026/11/18 21:06:00", "c-4", 200, 2, 1000, "delivered", "SECRET-OLD", None)]
open(root + "/cr_c.log", "w").write("".join(L))
PY
runcorpus --fixture "$root/cr_c.log"
check "R12 research corpus searches 4: cell 2 (one without the field), brief 2" 'grep -q "research *4 (cell 2, brief 2)" "$root/out"'
check "R12 per run uses cell searches only: c-1 has 2, c-2 0, c-3 0 (median 0); c-2 is a web search without a cell search: 1 of 3" 'grep -q "corpus searches median 0," "$root/out" && grep -q "no corpus search at all: 1 of 3" "$root/out"'
check "R13 briefs: 4 (one old-format line with no objective_from; delivered 2, empty 1, failed 1), median ms 200, captures 2, tokens_est 1000; no free text" 'grep -q "research briefs: 4 (delivered 2, empty 1, failed 1); median ms 200, captures 2, tokens_est 1000" "$root/out" && ! grep -q SECRET "$root/out"'
check "R13 the brief section counts where the objective came from (unread 2, none 1); the old-format line counts nowhere" 'grep -q "objective from: none 1, unread 2" "$root/out"'
runcorpus --fixture "$root/cr_a.log"
check "R13 no brief lines: no brief section" '! grep -q "research briefs" "$root/out"'
runcorpus --fixture "$root/cr_c.log" --append "$root/c.tsv"
check "R12 the hour row has the brief count (21:00 row, column 15 = 2)" '[ "$(head -1 "$root/c.tsv" | cut -f15)" = corpus_searches_brief ] && [ "$(sed -n 2p "$root/c.tsv" | cut -f15)" = 2 ]'
# R14: the guest line's vector= and bm25_kept= fields (station V4; ledger C144 is settled from vector=)
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 1.0] x-go-choir-run-autoputer-runtime[1]: "
def cs(ts, run, extra):
    return (f'{PRE}{ts} corpus search computer=c captures_ms=5 captures=1 note="n" query="SECRET-Q" capture_ids= run={run} '
            f'desk=research conv=v published=1 events=1 capped=false{extra} caller=cell\n')
L = [PRE + "2026/11/18 20:30:00 other\n", PRE + "2026/11/18 22:10:00 other\n",
     cs("2026/11/18 21:01:00", "d-1", " vector=12 vector_state=exact bm25_kept=1 by_meaning=5"), cs("2026/11/18 21:02:00", "d-1", " vector=0 vector_state=approximate bm25_kept=2 by_meaning=2"),
     cs("2026/11/18 21:03:00", "d-1", " vector=unavailable vector_state=unavailable bm25_kept=0"), cs("2026/11/18 21:04:00", "d-1", "")]
open(root + "/cr_v.log", "w").write("".join(L))
PY
runcorpus --fixture "$root/cr_v.log"
check "R14 vector half answered on 2 of the 3 lines with the field (0 rows is an answer), 3 BM25 pages inserted" 'grep -q "vector half answered 66.7% (of 3 lines with vector=); BM25 pages inserted 3" "$root/out" && ! grep -q SECRET "$root/out"'
check "R14 pages found by meaning are counted apart from by_labels_only (5 + 2 = 7)" 'grep -q "pages found by meaning 7" "$root/out"'
check "R14 the states of the answered lines are counted (exact 1, approximate 1)" 'grep -q "vector_state of those answered: approximate 1, exact 1" "$root/out"'
runcorpus --fixture "$root/cr_c.log"
check "R14 lines without the field print no vector line" '! grep -q "vector half answered" "$root/out"'
# R14: the deployed shape (new fields before pages_returned), the old shape, and an unknown field
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 9.0] x-go-choir-run-autoputer-runtime[1]: "
L = [PRE + "2026/11/19 01:00:00 runtime: other\n",
 PRE + "2026/11/19 01:50:57 runtime: search run=d-1 desk=research started=4 from_held=0 not_sent=0 search_all_calls=0 wordings_offered=0 wordings_sent=0 unread_notes=1 untagged=0 pages_returned=120 pages_new=88 pages_read=11\n",
 PRE + "2026/11/19 01:51:12 runtime: search run=d-2 desk=research started=2 from_held=0 not_sent=2 (holds_unread=2) search_all_calls=2 wordings_offered=4 wordings_sent=2 unread_notes=0 untagged=0 pages_returned=80 pages_new=75 pages_read=6\n",
 PRE + "2026/11/19 01:52:00 runtime: search run=d-3 desk=research started=1 from_held=0 not_sent=0 pages_returned=10 pages_new=5 pages_read=1\n",
 PRE + "2026/11/19 01:53:00 runtime: search run=d-4 desk=research started=3 from_held=0 not_sent=1 (stopped_yielding=1) widgets_seen=7 search_all_calls=1 wordings_offered=2 wordings_sent=1 unread_notes=0 untagged=0 pages_returned=30 pages_new=20 pages_read=2\n",
 PRE + "2026/11/19 02:30:00 runtime: other\n"]
open(root + "/cr_d.log", "w").write("".join(L))
PY
runcorpus --fixture "$root/cr_d.log"
check "R14 all four shapes count: 4 runs, started 10, from the deployed shape pages_returned 240 and read 20" 'grep -q "summary lines): 4" "$root/out" && grep -q "started (web searches sent) sum 10" "$root/out" && grep -q "pages_returned 240, pages_new 188, pages_read 20" "$root/out"'
check "R14 reasons from the parentheses are summed and not read as fields; wordings 3 sent of 6 offered (50.0%), unread_notes 1, calls 3" 'grep -q "not_sent 3 (holds_unread=2, stopped_yielding=1)" "$root/out" && grep -q "(3 lines carry the wording fields) search_all_calls 3, wordings_offered 6, wordings_sent 3 (50.0%), unread_notes 1" "$root/out"'
check "R14 an unknown numeric field in the middle is kept, shown, and does not drop the line" 'grep -q "other numeric fields on the summary lines (sums): widgets_seen=7" "$root/out"'
runcorpus --fixture "$root/cr_d.log" --first 2 --after 2026-11-19T01:51Z
check "R14 --first on the deployed shape finds 2 (d-2, d-3)" 'grep -q "found 2" "$root/out" && grep -q "summary lines): 2" "$root/out"'
check "R14 the one-line label names --first, not the console span" 'tail -n 1 "$root/out" | grep -q "^Corpus reuse (first 2 runs after 11-19T01:51, found 2)"'
runcorpus --fixture "$root/cr_d.log" --since 2026-11-19T01:50Z --until 2026-11-19T01:55Z
check "R14 the one-line label names the --since/--until window" 'tail -n 1 "$root/out" | grep -q "^Corpus reuse (11-19T01:50 to 11-19T01:55)"'
# R16: the plain verbs' refusals (holds_unread) and the fresh override
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 9.0] x-go-choir-run-autoputer-runtime[1]: "
L = [PRE + "2026/11/19 03:00:00 runtime: other\n",
 PRE + "2026/11/19 03:10:00 runtime: search run=e-1 desk=research started=1 from_held=0 not_sent=4 (holds_unread=4) search_all_calls=0 wordings_offered=0 wordings_sent=0 fresh_over_unread=1 untagged=0 pages_returned=40 pages_new=40 pages_read=3\n",
 PRE + "2026/11/19 03:11:00 runtime: search run=e-2 desk=research started=3 from_held=0 not_sent=2 (holds_unread=2) search_all_calls=1 wordings_offered=3 wordings_sent=1 fresh_over_unread=0 untagged=0 pages_returned=60 pages_new=50 pages_read=5\n",
 PRE + "2026/11/19 03:40:00 runtime: other\n"]
open(root + "/cr_e.log", "w").write("".join(L))
PY
# R17: the measurement fields (Landing 0 of the one-verb note); the old lines above still parse
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
PRE = "[ 9.0] x-go-choir-run-autoputer-runtime[1]: "
L = [PRE + "2026/11/20 03:00:00 runtime: other\n",
 PRE + "2026/11/20 03:10:00 runtime: search run=f-1 desk=research started=9 from_held=1 not_sent=2 (search_running=2) search_all_calls=1 wordings_offered=4 wordings_sent=3 fresh_over_unread=0 untagged=0 wordings_running=2 pages_returned=200 pages_new=80 pages_read=10 searches_judged=9 searches_mostly_seen=4 outlets_held_max=7 outlets_held_last=6 reuse_declined_out_of_window=2 reuse_declined_earlier_run=1 resumed_segment=1\n",
 PRE + "2026/11/20 03:11:00 runtime: search run=f-2 desk=research started=2 from_held=0 not_sent=0 pages_returned=20 pages_new=20 pages_read=1 searches_judged=2 searches_mostly_seen=0 outlets_held_max=2 outlets_held_last=2\n",
 PRE + "2026/11/20 03:12:00 runtime: search run=f-3 desk=research started=1 from_held=0 not_sent=0 pages_returned=10 pages_new=5 pages_read=1\n",
 PRE + "2026/11/20 03:40:00 runtime: other\n"]
open(root + "/cr_f.log", "w").write("".join(L))
PY
runcorpus --fixture "$root/cr_f.log"
check "R17 the measurement fields are summed over the lines that carry them (2 of 3): judged 11, mostly seen 4, running 2, resumed 1" 'grep -q "(2 lines carry the measurement fields) searches judged 11, mostly seen (75% or more already held) 4 (36.4%)" "$root/out" && grep -q "wordings refused as running 2; runs resumed after a restart (counts are one segment) 1" "$root/out"'
check "R17 the reuse decline reasons are summed by token and the outlets are shown" 'grep -q "reuse declined a prior identical search, by reason: earlier_run=1, out_of_window=2" "$root/out" && grep -q "largest 7" "$root/out"'
check "R17 the new fields are not listed as unknown numeric fields, and the older lines still count" '! grep -q "other numeric fields" "$root/out" && grep -q "summary lines): 3" "$root/out"'
runcorpus --fixture "$root/cr_e.log"
check "R16 6 searches refused by the need test, 4 sent: 60.0% of attempts" 'grep -q "refused by the need test (holds_unread, read_first, held_first, site_held; every verb) 6; web searches sent 4; refused share of attempts 60.0%" "$root/out"'
check "R16 fresh_over_unread is read, summed and printed" 'grep -q "fresh_over_unread+fresh_sent 1, untagged 0" "$root/out"'
# R10
mkpaper 'many(90, 100, PAPER, "devin", "swe-2-flash")'; mkstore 77926 '78 + 0.5 * h' 3
env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/paper.json" --storage-fixture "$root/store.json" --corpus-fixture "$root/missing.log" --corpus-append "$root/h.tsv" >"$root/out" 2>&1; rc=$?
check "R10 a corpus failure is shown in its own section and changes neither exit code nor last line" '[ "$rc" = 0 ] && grep -q "corpus reuse: unavailable" "$root/out" && tail -n 1 "$root/out" | grep -q "OK$"'
env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/paper.json" --storage-fixture "$root/store.json" --corpus-fixture "$root/cr_a.log" --corpus-append "$root/h.tsv" >"$root/out" 2>&1; rc=$?
check "R10 the corpus section prints between the reports and the last line" '[ "$rc" = 0 ] && grep -q "== corpus reuse" "$root/out" && tail -n 1 "$root/out" | grep -q "OK$"'

# status-vectors (station V2). Failure list, written before the code:
#  V1  the newest embed_gap line of each shape is not the one shown, or one
#      shape's line is shown for the other
#  V2  remaining is not the line's own remaining; have and want are not read
#  V3  searches are not counted per shape, or the mean ms is wrong; none is "-"
#  V4  a journal with no embed_gap line crashes or invents numbers
#  V5  the --json output lacks the same numbers; an unreadable fixture exits 2
vectors="$here/status-vectors"
cat >"$root/vec.log" <<'EOF'
2026/11/19 05:00:00 corpusd retrieval op=embed_gap shape=stories ms=0.00 rows=0 have=100 want=120 remaining=20 model=2188ac1deca4
2026/11/19 05:00:01 corpusd retrieval op=embed_gap shape=captures ms=0.00 rows=0 have=10 want=500 remaining=490 model=2188ac1deca4
2026/11/19 05:00:30 corpusd retrieval op=embed_gap shape=captures ms=12.50 rows=4 have=14 want=500 remaining=486 model=2188ac1deca4 refused=3
2026/11/19 05:01:00 corpusd retrieval op=embed_gap shape=stories ms=900.00 rows=60 have=120 want=118 remaining=0 model=2188ac1deca4
2026/11/19 05:02:00 corpusd retrieval op=vector_search shape=vector_stories ms=10.00 rows=5 err=
2026/11/19 05:02:00 corpusd retrieval op=vector_search shape=vector_captures ms=30.00 rows=3 err=
2026/11/19 05:03:00 corpusd retrieval op=vector_search shape=vector_stories ms=20.00 rows=5 err=
EOF
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec.log" >"$root/out" 2>&1; rc=$?
check "V1 the newest line of each shape: stories have 120, captures have 14" '[ "$rc" = 0 ] && grep -q "^stories .*have 120  want 118" "$root/out" && grep -q "^captures .*have 14  want 500" "$root/out"'
check "V2 remaining is the line's own: stories 0 (refused 0, an older line without the field), captures 486 with refused 3, model shown" 'grep -q "^stories .*remaining 0  model 2188ac1deca4  refused 0" "$root/out" && grep -q "^captures .*remaining 486  model 2188ac1deca4  refused 3" "$root/out"'
check "V3 two story searches mean 15.00 ms, one capture search 30.00 ms" 'grep -q "vector_stories .*2 searches, mean 15.00 ms" "$root/out" && grep -q "vector_captures .*1 searches, mean 30.00 ms" "$root/out"'
cat >"$root/vec1.log" <<'EOF'
2026/11/19 05:00:00 corpusd retrieval op=embed_gap shape=stories ms=0.00 rows=0 have=100 want=100 remaining=0 model=2188ac1deca4 refused=0
2026/11/19 06:00:00 corpusd vectors pass err=vectors: the pgvector extension is not installed in this database and this role cannot create it (go-choir-postgresql-roles makes it as postgres): ERROR (retry in 5s)
EOF
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec1.log" >"$root/out" 2>&1
check "V6 the extension error is newest: the first line says pgvector MISSING" 'head -n1 "$root/out" | grep -q "^pgvector  MISSING or too old: pgvector extension is not installed"'
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec.log" >"$root/out" 2>&1
check "V6 an embed_gap line after (or without) an error: pgvector ok" 'head -n1 "$root/out" | grep -q "^pgvector  ok"'
cat >"$root/vec2.log" <<'EOF'
2026/11/19 05:00:00 corpusd retrieval op=embed_gap shape=stories ms=0.00 rows=0 have=100 want=100 remaining=0 model=2188ac1deca4 refused=0
2026/11/19 05:10:00 corpusd retrieval op=vector_import shape=route ms=5000 rows=100 written=100 refused=0 sample_min_cosine=0.99990 err= reasons= job=vi-1 state=done file=
2026/11/19 06:10:00 corpusd retrieval op=vector_import shape=inbox ms=900000 rows=215189 written=214000 refused=1189 sample_min_cosine=0.99961 err= reasons=not_public:1000,text_mismatch:189 job=vi-2 state=done file=captures.jsonl
EOF
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec2.log" >"$root/out" 2>&1
check "V7 the last import job is shown with its source, counts and reasons" 'grep -q "^last import  inbox vi-2 file captures.jsonl: done, rows 215189, written 214000, dropped 1189 (not_public:1000,text_mismatch:189), min cosine 0.99961" "$root/out"'
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec.log" >"$root/out" 2>&1
check "V7 with no import line it says none" 'grep -q "^last import  none" "$root/out"'
printf 'nothing here\n' >"$root/vec0.log"
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec0.log" >"$root/out" 2>&1; rc=$?
check "V4 no embed_gap line: said so, no numbers invented, searches show -" '[ "$rc" = 0 ] && grep -q "^stories .*no embed_gap line" "$root/out" && grep -q "vector_stories .*0 searches, mean -" "$root/out"'
env -u BASH_ENV python3 "$vectors" --fixture "$root/vec.log" --json >"$root/out" 2>&1
check "V5 json carries the numbers" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"gap\"][\"captures\"][\"remaining\"]==486 and d[\"searches\"][\"vector_stories\"][\"count\"]==2" "$root/out"'
env -u BASH_ENV python3 "$vectors" --fixture "$root/nonexistent.log" >"$root/out" 2>&1; rc=$?
check "V5 an unreadable fixture exits 2" '[ "$rc" = 2 ]'

# search usage (the hourly's "search usage" block and its history file). Failure list, written before the code:
#  U1  a gateway line outside the last hour is counted in it, one from before 00:00 UTC or after now is counted
#      anywhere; a cache hit counts as sent; a failed search is not a request
#  U2  a provider's calls are not the growth of the log's today= count (a first line counts from 0), or a provider
#      that appears only in today= or only in skipped= is missing (providers are read, not hardcoded)
#  U3  refusals are not counted per provider by the log's own reason (quota_limited, capped, error, ...)
#  U4  corpus searches are not the ranked lines; shares with the vector half, median and p90 are wrong; query embeds
#      that failed or hit the deadline are not counted apart; the paper's cell and brief searches are not split from the rest
#  U5  the projection is not today's sent / hours elapsed x 24, or the 1,000 target and the owner's sentence are not printed
#  U6  a journal that could not be read prints 0 instead of "unknown: <reason>", in the block or in the history row
#  U7  a second run in the same UTC hour adds a row instead of replacing it; --history N does not print the last N rows
#      aligned; the row holds anything but counts (a vm id, a URL)
mkdir -p "$root/su"
cat >"$root/su/full.txt" <<'EOF'
##GW
2026/11/19 23:50:00 gateway: search for autoputer vm-1669020adcece51c2f5ff9b6df399ef5 answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served=miss results=99 unique=99 seen=0 routing=budget escalated=no paid_calls=0 today=[keenable=999]
2026/11/20 00:10:00 gateway: search for autoputer vm-1669020adcece51c2f5ff9b6df399ef5 answered 1 of 1 providers (2 cooling, 0 pending) skipped=[searxng=success_empty brave=quota_limited] served=miss results=40 unique=40 seen=0 routing=budget escalated=no paid_calls=0 today=[keenable=10 parallel=149/150]
2026/11/20 00:40:00 gateway: search for autoputer vm-1669020adcece51c2f5ff9b6df399ef5 answered 2 of 2 providers (1 cooling, 0 pending) skipped=[brave=quota_limited] served=miss results=20 unique=18 seen=2 routing=budget escalated=short(0):serper paid_calls=1 today=[keenable=11 serper=1/150]
2026/11/20 01:00:00 gateway: search for autoputer vm-1669020adcece51c2f5ff9b6df399ef5 answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served=hit results=40 unique=40 seen=40 routing=cache escalated=no paid_calls=0 today=[]
2026/11/20 01:10:00 gateway: search for autoputer vm-00000000000000000000000000000002 answered 1 of 1 providers (3 cooling, 0 pending) skipped=[brave=quota_limited parallel=capped serper=error] served=miss results=10 unique=10 seen=0 routing=budget escalated=no paid_calls=0 today=[keenable=12 parallel=150/150]
2026/11/20 01:20:00 gateway: search failed for autoputer vm-00000000000000000000000000000002
2026/11/20 01:45:00 gateway: search for autoputer vm-1669020adcece51c2f5ff9b6df399ef5 answered 1 of 1 providers (0 cooling, 0 pending) skipped=[] served=miss results=77 unique=77 seen=0 routing=budget escalated=no paid_calls=0 today=[keenable=13]
2026/11/19 23:50:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (max_results=10 run=r9 desk=research conv=c)
2026/11/20 00:10:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (max_results=10 run=r1 desk=research conv=c)
2026/11/20 00:40:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (max_results=10 run=r1 desk=research conv=c)
2026/11/20 00:41:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (max_results=10 run=r1 desk=research conv=c)
2026/11/20 01:00:00 gateway: search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5 (max_results=10 run=r2 desk=research conv=c)
2026/11/20 01:10:00 gateway: search request from autoputer vm-00000000000000000000000000000002 (max_results=10 run=r7 desk=research conv=c)
##CORPUS
2026/11/20 00:20:00 corpusd retrieval op=capture_search shape=ranked ms=100.00 rows=6 vector_state=exact err=
2026/11/20 00:50:00 corpusd retrieval op=capture_search shape=ranked ms=200.00 rows=6 vector_state=approximate err=
2026/11/20 01:05:00 corpusd retrieval op=capture_search shape=ranked ms=300.00 rows=6 vector_state=unavailable err=
2026/11/20 01:05:00 corpusd retrieval op=capture_search shape=terms ms=20.00 rows=6 err=
2026/11/20 01:05:00 corpusd retrieval op=capture_fulltext_search shape=passages ms=30.00 rows=6 err=
2026/11/20 01:06:00 corpusd retrieval op=capture_search shape=ranked ms=900.00 rows=6 vector_state=exact err=
2026/11/20 01:15:00 corpusd retrieval op=capture_search shape=ranked ms=400.00 rows=6 vector_state=approximate err=
2026/11/20 01:05:00 corpusd retrieval op=vector_embed shape=query ms=100.00 rows=1 err=
2026/11/20 01:06:00 corpusd retrieval op=vector_embed shape=query ms=5000.00 rows=0 err=context_deadline_exceeded
2026/11/20 01:07:00 corpusd retrieval op=vector_embed shape=query ms=50.00 rows=0 err=embedder_refused
##CONSOLE
2026/11/20 00:20:00 caller=cell
2026/11/20 01:05:00 caller=cell
2026/11/20 01:06:00 caller=brief
##END
EOF
awk '/^##GW$/{skip=1; print "##GWFAIL"; next} /^##CORPUS$/{skip=0} /^##CONSOLE$/{skip=1; next} /^##END$/{skip=0} !skip' "$root/su/full.txt" >"$root/su/nogw.txt"
runsu() { # runsu FIXTURE NOW HISTORY
  mkpaper 'many(90, 100, PAPER, "devin", "swe-2-flash")'; mkstore 77926 '78 + 0.5 * h' 3
  env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/paper.json" --storage-fixture "$root/store.json" \
    --search-fixture "$1" --search-now "$2" --search-history "$3" >"$root/out" 2>&1; }
H="$root/su/hist.tsv"; rm -f "$H"
runsu "$root/su/full.txt" 2026-11-20T01:30Z "$H"
check "U1 the block prints between the reports and the last line, which stays one line" 'grep -q "^== search usage" "$root/out" && ! tail -n 1 "$root/out" | grep -q "search usage"'
check "U1 last hour: 4 requests (paper 2, other 1, failed 1), 2 sent, 1 from the cache; the 23:50 and 01:45 lines are in no window" 'grep -q "last hour requests 4 (paper 2, other computers 1, failed 1); sent to providers 2, served from the cache 1" "$root/out"'
check "U1 today: 5 requests, 3 sent; results 40+20+40+10 = 110" 'grep -q "today     requests 5 (paper 3, other computers 1, failed 1); sent to providers 3, served from the cache 1, joined an identical one in flight 0; results returned 110" "$root/out"'
check "U2 calls are growth of today=: keenable 2 in the hour and 12 today (10 at the first line); serper 1; parallel 1 then 150 (appears in today= and skipped=)" 'grep -q "keenable  free; calls 2 last hour, 12 today" "$root/out" && grep -q "serper    paid, cap 150/day; calls 1 last hour, 1 today" "$root/out" && grep -q "parallel  paid, cap 150/day; calls 1 last hour, 150 today" "$root/out"'
check "U2 a provider only in skipped= (searxng, brave) is listed with 0 calls" 'grep -q "searxng   free; calls 0 last hour, 0 today; refused last hour: none; today: success_empty 1" "$root/out" && grep -q "brave     free; calls 0 last hour, 0 today" "$root/out"'
check "U3 refusals by the log reason: brave quota_limited 2 in the hour and 3 today; parallel capped 1; serper error 1" 'grep -q "brave .*refused last hour: quota_limited 2; today: quota_limited 3" "$root/out" && grep -q "parallel .*refused last hour: capped 1" "$root/out" && grep -q "serper .*refused last hour: error 1" "$root/out"'
check "U4 corpus: 4 ranked in the hour, 3 with the vector half (75%), exact 1 approximate 1 unavailable 1 in... states" 'grep -q "last hour total 4; with the vector half 3 (75%) \[approximate 2, exact 1, unavailable 1\]; median 350 ms, p90 900 ms" "$root/out"'
check "U4 today: 5 ranked, 4 with the vector half" 'grep -q "today     total 5; with the vector half 4 (80%)" "$root/out"'
check "U4 shapes are counted by op:shape" 'grep -q "by shape: capture_fulltext_search:passages 1, capture_search:ranked 4, capture_search:terms 1" "$root/out"'
check "U4 embeds: 3 asked, 2 failed, 1 of them the deadline" 'grep -q "query embeds: 3; failed 2 (of which deadline/timeout 1)" "$root/out"'
check "U4 callers: the paper's cell 1 and brief 1 in the hour, the other 2 by difference" 'grep -q "by caller: paper.s cell 1, paper.s brief 1, other callers 2" "$root/out"'
check "U5 the target constant and the owner's sentence are printed" 'grep -q "owner target: 1000 external web searches a day in total; \"we should be able to get our searches underneath 1,000 per day\" (owner, 2026-11-19)" "$root/out"'
check "U5 projection: 3 sent in 1.5 h is 48 a day; serper 1 call projects 16, parallel 150 projects 2400 and will hit the cap" 'grep -q "sent today 3 in 1.5 h: a day at this rate is 48 against the target 1000 (under)" "$root/out" && grep -q "serper .*paid 1 calls today, 16 projected; cap 150/day, under the cap" "$root/out" && grep -q "parallel .*150 calls today, 2400 projected; cap 150/day, will hit the cap" "$root/out"'
check "U5 corpus per web search: 4 / 2 = 2.0 in the hour; with no corpus-reuse section the per-run ratio says unknown" 'grep -q "per external web search, last hour: 2.0 (4 / 2)" "$root/out" && grep -q "per research run (console summary lines, terminal records only): unknown" "$root/out"'
check "S5 the gateway meter per run, from the request lines' run=: hour 3 / 2 runs = 1.5, today 4 / 2 = 2.0; the paper's lines only, the 23:50 line out" 'grep -q "external web searches per research run (gateway journal, run=): last hour 1.5 (3 / 2 runs); today 2.0 (4 / 2 runs)" "$root/out"'
check "S5 the request lines are not counted as requests or as unparsed" '! grep -q "did not match the known shape" "$root/out"'
runsu "$root/su/full.txt" 2026-11-20T01:30Z "$root/su/h4.tsv"
env -u BASH_ENV python3 "$hourly" --paper-fixture "$root/paper.json" --storage-fixture "$root/store.json" --corpus-fixture "$root/gs_c.log" \
  --corpus-gateway-fixture "$root/gs_g.log" --corpus-append "$root/h5.tsv" --search-fixture "$root/su/full.txt" --search-now 2026-11-20T01:30Z --search-history "$root/su/h4.tsv" >"$root/out" 2>&1
check "S5 with the corpus section read, the console-summary figure is labelled and gives the gap: 8 / 2 runs = 4.0, gap 4" 'grep -q "per research run (console summary lines, terminal records only; the corpus-reuse span): 4.0 (8 / 2 runs); the gap to the gateway.s count over that span is 4" "$root/out"'
check "U7 the search block holds no computer id, URL or query" '! sed -n "/^== search usage/,/^$/p" "$root/out" | grep -qE "vm-[0-9a-f]{8}|https?://|SECRET"'
check "U7 the first run wrote the header and one row of counts only: 4 corpus, 3 with vector, median 350, 4 requests, 1 cached, then keenable 2, parallel 1, serper 1, day 3 sent, 163 calls" '[ "$(wc -l <"$H" | tr -d " ")" = 2 ] && [ "$(sed -n 1p "$H" | tr "\t" "|")" = "utc_hour|corpus_total|corpus_vector|corpus_median_ms|ext_requests|cache_served|calls_brave|calls_keenable|calls_parallel|calls_serper|day_searches_sent|day_provider_calls|runs_searched|searches_per_run" ] && [ "$(sed -n 2p "$H" | tr "\t" "|")" = "2026-11-20T01Z|4|3|350|4|1|0|2|1|1|3|163|2|1.5" ] && ! grep -qE "vm-|http" "$H"'
runsu "$root/su/full.txt" 2026-11-20T01:50Z "$H"
check "U7 a second run in the same UTC hour replaces that hour's row (still 2 lines)" '[ "$(wc -l <"$H" | tr -d " ")" = 2 ] && [ "$(grep -c "^2026-11-20T01Z" "$H")" = 1 ]'
runsu "$root/su/full.txt" 2026-11-20T02:10Z "$H"
check "U7 the next hour adds a row" '[ "$(wc -l <"$H" | tr -d " ")" = 3 ] && sed -n 3p "$H" | grep -q "^2026-11-20T02Z"'
env -u BASH_ENV python3 "$hourly" --history 1 --search-history "$H" >"$root/out" 2>&1
check "U7 --history 1 prints the header and the last row only, aligned" '[ "$(wc -l <"$root/out" | tr -d " ")" = 2 ] && head -n1 "$root/out" | grep -q "^hour  *corpus  *w/vec" && tail -n 1 "$root/out" | grep -q "^2026-11-20T02Z"'
env -u BASH_ENV python3 "$hourly" --history --search-history "$H" >"$root/out" 2>&1
check "U7 --history alone prints up to 24 rows (here both)" '[ "$(wc -l <"$root/out" | tr -d " ")" = 3 ]'
H2="$root/su/hist2.tsv"; rm -f "$H2"
runsu "$root/su/nogw.txt" 2026-11-20T01:30Z "$H2"
check "U6 an unreadable gateway journal says unknown with the reason, never 0, and the corpus half still prints" 'grep -q "unknown: gateway journal unreadable on the host" "$root/out" && ! grep -q "requests 0" "$root/out" && grep -q "last hour total 4" "$root/out"'
check "U6 the history row says unknown for what could not be read, not 0" '[ "$(sed -n 2p "$H2" | tr "\t" "|")" = "2026-11-20T01Z|4|3|350|unknown|unknown|unknown|unknown|unknown|unknown" ]'
check "S5 an unreadable gateway says unknown for the per-run meter too" 'grep -q "per research run (gateway journal, run=): unknown: gateway journal unreadable" "$root/out"'
check "U6 without the console the caller split says unknown" 'grep -q "by caller: unknown: paper guest console unreadable" "$root/out"'

# status-entities (station E0 of docs/design/entity-layer-2026-11-20.md). Failure list, written before the code:
#  N1  entities total is not the growth read's lexicon count, or kind is invented (the public reads carry no kind: it must say so)
#  N2  stories with and without entities are not counted over the NEWEST N only (a story past N leaks in)
#  N3  attachments per story: median and p90 (nearest rank) over the stories that have entities are wrong
#  N4  entities attached to one story only: wrong count (an entity on two stories counts as none)
#  N5  duplicate candidates: containment (Trump / Donald Trump) or initials (UAE / United Arab Emirates) missed, an
#      unrelated pair (Italy / France) reported, or the list holds more than 20 pairs while the count stays total
#  N6  non-entity candidates: a demonym (Chinese) or a common word (Health) missed, a real name (Italy) flagged
#  N7  Wikidata share: not the wd: ids over distinct entities
#  N8  the last line is not one line under 200 chars; --json lacks the numbers; an unreadable fixture does not exit 2
entities="$here/status-entities"
python3 - "$root" <<'PY'
import json, sys
root = sys.argv[1]
def e(i, l): return {"id": i, "label": l}
A, B, C = e("ent:a", "Trump"), e("ent:b", "Donald Trump"), e("ent:c", "Chinese")
D, E, F = e("ent:d", "UAE"), e("ent:e", "United Arab Emirates"), e("ent:f", "Health")
G, W = e("ent:g", "Italy"), e("wd:Q142", "France")
stories = [
  {"publication_id": "p1", "title": "one", "entities": [A, B, C]},
  {"publication_id": "p2", "title": "two", "entities": [D, E, F, A]},
  {"publication_id": "p3", "title": "three", "entities": [G]},
  {"publication_id": "p4", "title": "four"},
  {"publication_id": "p5", "title": "five"},
  {"publication_id": "p6", "title": "six", "entities": [W, G]},
]
json.dump({"stories": stories, "next_cursor": ""}, open(root + "/ent_front.json", "w"))
json.dump({"lexicon": {"entities": 6690, "keys": 9120, "stopwords": 34}, "stoplist": ["AI"], "days": []}, open(root + "/ent_growth.json", "w"))
many = [{"publication_id": "q%d" % i, "title": "t", "entities": [e("ent:x%d" % i, "Zq" + chr(97 + i // 26) + chr(97 + i % 26) + " Beta"), e("ent:y%d" % i, "Zq" + chr(97 + i // 26) + chr(97 + i % 26))]} for i in range(30)]
json.dump({"stories": many, "next_cursor": ""}, open(root + "/ent_front_many.json", "w"))
PY
runent() { env -u BASH_ENV python3 "$entities" --front-fixture "$root/ent_front.json" --growth-fixture "$root/ent_growth.json" "$@" >"$root/out" 2>&1; echo $? >"$root/rc"; }
runent --newest 6
check "N1 entities total is the lexicon count and kind is said to be unavailable" 'grep -q "^entities total  6690" "$root/out" && grep -q "^by kind  unavailable" "$root/out"'
check "N2 stories 6, with entities 4, without 2" 'grep -q "^stories  6 newest: 4 with entities (66.7%), 2 without" "$root/out"'
runent --newest 5
check "N2 only the newest N: 5 stories, 3 with, 2 without" 'grep -q "^stories  5 newest: 3 with entities (60.0%), 2 without" "$root/out"'
runent --newest 6
check "N3 per story over stories with entities (3,4,1,2): median 2.5, p90 4" 'grep -q "^attachments per story  median 2.5  p90 4  max 4" "$root/out"'
check "N4 six entities sit on one story, of 8 distinct in the window" 'grep -q "^entities on one story only  6 of 8 distinct" "$root/out"'
check "N5 two duplicate candidates: Trump / Donald Trump and UAE / United Arab Emirates, nothing else" 'grep -q "^duplicate candidates  2 " "$root/out" && grep -q "Trump  /  Donald Trump" "$root/out" && grep -q "UAE  /  United Arab Emirates" "$root/out" && ! grep -q "Italy  /  France" "$root/out"'
check "N6 two non-entity candidates: Chinese, Health; not Italy" 'grep -q "^non-entity candidates  2 " "$root/out" && grep -q "Chinese" "$root/out" && grep -q "Health" "$root/out" && ! grep -q "Italy .*(stories" "$root/out"'
check "N7 one of 8 distinct entities carries a Wikidata id" 'grep -q "^wikidata  1 of 8 distinct (12.5%)" "$root/out"'
check "N8 the last line is one line, under 200 chars, with the counts" '[ "$(cat "$root/rc")" = 0 ] && [ "$(tail -n 1 "$root/out" | wc -c)" -lt 200 ] && tail -n 1 "$root/out" | grep -q "^entities: .*dup 2 .*non-entity 2"'
env -u BASH_ENV python3 "$entities" --front-fixture "$root/ent_front_many.json" --growth-fixture "$root/ent_growth.json" --newest 30 >"$root/out" 2>&1
check "N5 the count is the total (30) and the list holds 20" 'grep -q "^duplicate candidates  30 " "$root/out" && [ "$(grep -c "  /  " "$root/out")" = 20 ]'
runent --newest 6 --json
check "N8 json carries the numbers" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"stories\"]==6 and d[\"with_entities\"]==4 and d[\"dup_count\"]==2 and d[\"nonentity_count\"]==2 and d[\"entities_total\"]==6690" "$root/out"'
env -u BASH_ENV python3 "$entities" --front-fixture "$root/nonexistent.json" --growth-fixture "$root/ent_growth.json" >"$root/out" 2>&1; rc=$?
check "N8 an unreadable fixture exits 2" '[ "$rc" = 2 ]'
# S5: a history file written before the per-run columns still prints, and the next write keeps its rows
H3="$root/su/hist3.tsv"
printf 'utc_hour\tcorpus_total\tcorpus_vector\tcorpus_median_ms\text_requests\tcache_served\tcalls_keenable\tday_searches_sent\tday_provider_calls\n2026-11-20T00Z\t4\t3\t350\t4\t1\t2\t3\t5\n' >"$H3"
env -u BASH_ENV python3 "$hourly" --history --search-history "$H3" >"$root/out" 2>&1
check "S5 --history on an old-format file prints its row" '[ "$(wc -l <"$root/out" | tr -d " ")" = 2 ] && tail -n 1 "$root/out" | grep -q "^2026-11-20T00Z"'
runsu "$root/su/full.txt" 2026-11-20T01:30Z "$H3"
check "S5 the next write widens the header and keeps the old row, its new cells -" '[ "$(wc -l <"$H3" | tr -d " ")" = 3 ] && [ "$(sed -n 2p "$H3" | cut -f1,2)" = "$(printf "2026-11-20T00Z\t4")" ] && [ "$(sed -n 2p "$H3" | awk -F"\t" "{print \$(NF-1) \"|\" \$NF}")" = "-|-" ] && [ "$(sed -n 3p "$H3" | awk -F"\t" "{print \$(NF-1) \"|\" \$NF}")" = "2|1.5" ]'
env -u BASH_ENV python3 "$hourly" --history --search-history "$H3" >"$root/out" 2>&1
check "S5 --history prints the widened file with the new columns" 'head -n1 "$root/out" | grep -q "runs_searched"'

# status-story-choice. Failure list, written before the code:
#  C1  a decision outside [since, until) is counted, or an hour is not split on the UTC hour
#  C2  opens, feeds and declines are not counted apart, or a line that is none of them is counted
#  C3  answers by kind are wrong: a feed is same (per capture), an open's first capture is next
#      with a succeeds and new without, its other captures are same_as_batch, a decline is decline
#  C4  candidates by source (vector, lead), stories opened and fed (distinct), successor links
#      and the share of captures sent to an existing trajectory are not read from the lines
#  C5  research runs are not counted by cause (a successor's first round, an open's first
#      round, a later round), or captures per trajectory are not bucketed
#  C6  a line from before the judgment (no captures=) does not count one capture
#  C7  the judgment is not printed: the share of opens with a main point, and the share of
#      opens with an offer that answered every candidate offered (answers= next or new for
#      each doc in offered=; none or missing is unanswered); lines without main_point= are
#      from before the judgment and are in neither denominator
storychoice="$here/status-story-choice"
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
P = "[ 1.0] runtime[1]: "
def ln(ts, choice, story, caps, sources, offered, succeeds):
    return (f"{P}{ts} management story choice run=r-1 choice={choice} story={story} opening_shown=50 verb_shown=0 "
            f"candidates_shown=50 window=8h page_in_window=300 page_left_out=0 offered={offered} answers=- captures={caps} "
            f"sources={sources} main_point=40 succeeds={succeeds}\n")
rs = lambda ts, story, rnd, cause: f"{P}{ts} story research cast run=t-1 story={story} round={rnd} opened=true cause={cause}\n"
lines = [
  ln("2026/11/20 21:10:00", "open", "s1", 3, "near:2,lead:1", "dA,dB", "-"),
  ln("2026/11/20 21:20:00", "open", "s2", 1, "near:0,lead:0", "-", "dA"),
  ln("2026/11/20 21:30:00", "feed", "s1", 2, "near:1,lead:0", "s1", "-"),
  ln("2026/11/20 21:40:00", "feed", "s9", 1, "near:1,lead:1", "s9", "-"),
  ln("2026/11/20 22:00:00", "decline", "-", 4, "near:0,lead:0", "-", "-"),
  ln("2026/11/20 20:59:59", "open", "sx", 5, "near:0,lead:0", "-", "-"),
  P + "2026/11/20 21:15:00 management story choice run=r-1 choice=weird\n",
  P + "2026/11/20 22:10:00 management story choice run=r-1 choice=feed story=s8 opening_shown=50 verb_shown=0 candidates_shown=50\n",
  rs("2026/11/20 21:11:00", "s1", 1, "open"), rs("2026/11/20 21:21:00", "s2", 1, "successor"), rs("2026/11/20 21:31:00", "s1", 2, "feed"),
  rs("2026/11/20 21:41:00", "s9", 3, "feed"), rs("2026/11/20 20:00:00", "sx", 1, "open"),
  P + "2026/11/20 21:35:00 story research refused run=t-1 story=s1 unread=2\n",
  P + "2026/11/20 21:36:00 story research refused run=t-1 story=s1 unread=1\n",
  P + "2026/11/20 20:30:00 story research refused run=t-1 story=sx unread=9\n",
]
open(root + "/sc.log", "w").write("".join(lines))
PY
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc.log" --since 2026-11-20T21:00Z --until 2026-11-20T23:00Z --json >"$root/out" 2>&1
check "C1 two hours in the window, the 20:59 line out" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1]))[\"hours\"]; assert [r[\"hour\"] for r in d]==[\"2026-11-20T21Z\",\"2026-11-20T22Z\"], d" "$root/out"'
check "C2 window: 6 decisions are 2 opens, 3 feeds, 1 decline (the weird line out); 21h has 4" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); t=d[\"summary\"]; h=d[\"hours\"][0]; assert (t[\"decisions\"],t[\"opens\"],t[\"feeds\"],t[\"declines\"],h[\"decisions\"])==(6,2,3,1,4), (t,h)" "$root/out"'
check "C3 answers: same 4 (2+1+1 captures, the old-format feed counts 1), next 1, new 1, same_as_batch 2, decline 4" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert (t[\"same\"],t[\"next\"],t[\"new\"],t[\"same_as_batch\"],t[\"decline\"],t[\"captures\"])==(4,1,1,2,4,12), t" "$root/out"'
check "C4 sources near 4 lead 2; opened 2, fed 3, successor links 1; share 4/12" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert (t[\"cand_near\"],t[\"cand_lead\"],t[\"stories_opened\"],t[\"stories_fed\"],t[\"successor_links\"],t[\"share_to_existing_trajectory\"])==(4,2,2,3,1,0.333), t" "$root/out"'
check "C5 research: 4 runs in the window: open 1 (s1), successor 1 (s2), feed 2; trajectories: s1 5 captures, s2 1, s9 1, s8 1" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert t[\"research_runs\"]==4 and t[\"research_by_cause\"]=={\"open\":1,\"successor\":1,\"feed\":2,\"continue\":0} and t[\"captures_per_trajectory\"]=={\"1\":3,\"2\":0,\"3\":0,\"4-5\":1,\"6+\":0}, t" "$root/out"'
check "C5b research refusals: 2 in the window owing 3 captures (the 20:30 one is out)" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert (t[\"research_refused_unread\"],t[\"research_refused_captures\"])==(2,3), t" "$root/out"'
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc.log" --since 2026-11-20T21:00Z --until 2026-11-20T23:00Z >"$root/out" 2>&1
check "C6 the text table prints the answers and the research causes" 'grep -q "same 4, next 1, same_as_batch 2, new 1, decline 4" "$root/out" && grep -q "research Casts 4 (4 opened a round), by cause as logged at the source: open 1, successor 1, feed or later round 2, continue (inside an open round) 0" "$root/out" && grep -q "refused because fed captures were unread: 2 (3 captures" "$root/out"'

python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
P = "[ 1.0] runtime[1]: "
def op(ts, point, offered, answers, extra=" main_point=%d"):
    mp = (" main_point=%d" % point) if point is not None else ""
    return (f"{P}{ts} management story choice run=r-1 choice=open story=s-{ts[-2:]} opening_shown=5 verb_shown=0 candidates_shown=5 "
            f"window=8h page_in_window=30 page_left_out=0 offered={offered}" + (f" answers={answers}" if answers is not None else "") + f" captures=1 sources=near:1,lead:0{mp} succeeds=-\n")
lines = [
  op("2026/11/20 21:10:10", 40, "dA,dB", "dA:new,dB:next"),
  op("2026/11/20 21:10:20", 40, "-", "-"),
  op("2026/11/20 21:10:30", 0, "dA", "dA:none"),
  op("2026/11/20 21:10:40", 40, "dA,dB", "dA:new"),
  op("2026/11/20 21:10:50", None, "dA", None),
]
open(root + "/sc2.log", "w").write("".join(lines))
PY
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc2.log" --json >"$root/out" 2>&1
check "C7 opens judged 4 (the old-format open is out), with a main point 3, with an offer 3, answering every candidate 1" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert (t[\"opens\"],t[\"opens_judged\"],t[\"opens_with_main_point\"],t[\"opens_with_offer\"],t[\"opens_answering_every_candidate\"])==(5,4,3,3,1), t" "$root/out"'
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc2.log" >"$root/out" 2>&1
check "C7 the text table prints both shares" 'grep -q "opens with a main point: 3 of 4 (75.0%)" "$root/out" && grep -q "opens that answered every candidate offered: 1 of 3 (33.3%)" "$root/out"'

printf '%s\n' "[ 1.0] runtime[1]: 2026/11/20 21:10:00 story research gate skipped run=t-1 story=s1 err=\"context canceled\"" "[ 1.0] runtime[1]: 2026/11/20 21:11:00 story research gate skipped run=t-1 story=s2 err=\"x\"" "[ 1.0] runtime[1]: 2026/11/20 19:11:00 story research gate skipped run=t-1 story=s3 err=\"out of window\"" "[ 1.0] runtime[1]: 2026/11/20 21:12:00 story research gate released run=t-1 story=s1 unread=1 refused=3" >"$root/sc3.log"
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc3.log" --since 2026-11-20T21:00Z --until 2026-11-20T23:00Z --json >"$root/out" 2>&1
check "C8 the gate's failing open and its release are counted: 2 skipped in the window, 1 released" 'python3 -c "import json,sys; t=json.load(open(sys.argv[1]))[\"summary\"]; assert (t[\"research_gate_skipped\"],t[\"research_gate_released\"])==(2,1), t" "$root/out"'
env -u BASH_ENV python3 "$storychoice" --fixture "$root/sc3.log" --since 2026-11-20T21:00Z --until 2026-11-20T23:00Z >"$root/out" 2>&1
check "C8 the text table prints them" 'grep -q "research gate: 2 skipped (a store read failed, research went unchecked), 1 released" "$root/out"'


# status-search-reads (the deployed proof of the read-first rule). Failure list,
# written before the code:
#  W1  runs, searches or the mean per run are wrong, or a desk other than research is counted
#  W2  the 1/2/3/4+ distribution or the 7-or-more tail is wrong
#  W3  a later search sent with a read since the previous one is counted as zero-read, or one with none is not
#  W4  refusals by reason (read_first, held_first, site_held, holds_unread) are not summed from the run lines
#  W5  fresh_sent is not summed, or pages returned and read are not
#  W6  a search before --since is counted
searchreads="$here/status-search-reads"
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
VM = "vm-1669020adcece51c2f5ff9b6df399ef5"
G = lambda ts, run, desk="research", vm=VM: f"{ts} gateway: search request from autoputer {vm} ( max_results=40 run={run} desk={desk} conv=c-{run})\n"
R = lambda ts, run: f"[ 1.0] runtime[1]: {ts} corpus read run={run} desk=research conv=c capture=obj:x held=false via=read\n"
S = lambda ts, run, extra: f"[ 1.0] runtime[1]: {ts} runtime: search run={run} desk=research {extra}\n"
out = ["##READS\n", R("2026/11/21 04:59:00", "other"), R("2026/11/21 05:00:30", "r1"),
       "##SUM\n",
       S("2026/11/21 05:05:00", "r1", "started=3 from_held=0 not_sent=3 (read_first=2 held_first=1) children=2 started_research=1 started_research_web=2 search_all_calls=1 wordings_offered=1 wordings_sent=1 fresh_sent=1 untagged=0 wordings_running=0 pages_returned=100 pages_new=90 pages_read=12"),
       S("2026/11/21 05:06:30", "r4", "started=1 from_held=0 not_sent=0 fresh_sent=0 pages_returned=10 pages_new=10 pages_read=2").replace("desk=research ", "desk=research_web "),
       S("2026/11/21 05:06:00", "r2", "started=1 from_held=0 not_sent=1 (holds_unread=1 site_held=2) fresh_sent=0 pages_returned=40 pages_new=40 pages_read=8"),
       "##FRESH\n",
       f"[ 1.0] runtime[1]: 2026/11/21 05:07:00 runtime: search fresh run=r1 why_len=39 why_hash=aaaa1111\n",
       f"[ 1.0] runtime[1]: 2026/11/21 05:08:00 runtime: search fresh run=r2 why_len=39 why_hash=aaaa1111\n",
       f"[ 1.0] runtime[1]: 2026/11/21 05:09:00 runtime: search fresh run=r2 why_len=31 why_hash=bbbb2222\n",
       "##LOST\n",
       f"[ 1.0] runtime[1]: 2026/11/21 05:06:00 runtime: search gate state lost at resume run=r2\n",
       f"[ 1.0] runtime[1]: 2026/11/21 05:06:10 runtime: search gate state lost at resume run=r2\n",
       "##GW\n",
       G("2026/11/21 04:00:00", "early"),
       G("2026/11/21 05:05:00", "r4", "research_web"),
       G("2026/11/21 05:05:10", "r1", vm="vm-another-computer"), G("2026/11/21 05:05:20", "-"),
       G("2026/11/21 05:00:00", "r1"), G("2026/11/21 05:01:00", "r1"), G("2026/11/21 05:02:00", "r1"),
       G("2026/11/21 05:03:00", "r2"),
       G("2026/11/21 05:04:00", "r2", "management")]
for i in range(7):
    out.append(G("2026/11/21 05:%02d:00" % (10 + i), "r3"))
open(root + "/sr.txt", "w").write("".join(out))
PY
env -u BASH_ENV python3 "$searchreads" --fixture "$root/sr.txt" --since 2026-11-21T05:00Z --json >"$root/out" 2>&1
check "W1 4 runs, 12 searches (research_web counted; the management line, the 04:00 search, another computer and run=- are out), mean 3.0" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert (d[\"runs\"],d[\"searches\"],d[\"mean_per_run\"])==(4,12,3.0), d" "$root/out"'
check "W2 runs by searches 1:1 2:0 3:1 4+:1; one run of 7 with 7 searches" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"runs_by_searches\"]=={\"1\":2,\"2\":0,\"3\":1,\"4+\":1} and (d[\"runs_7plus\"],d[\"searches_from_7plus\"])==(1,7), d" "$root/out"'
check "W3 8 later searches in the console span, 7 with no successful read since the previous one (r1 read once, between its first two)" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert (d[\"later_searches\"],d[\"later_no_successful_read\"])==(8,7), d" "$root/out"'
check "W4 refusals by reason are summed from the run lines" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"refusals\"]=={\"read_first\":2,\"held_first\":1,\"holds_unread\":1,\"site_held\":2} and d[\"refused\"]==6, d" "$root/out"'
check "W5 fresh_sent 1; pages returned 150, read 22 (the research_web run line counts)" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert (d[\"search_fresh\"],d[\"pages_returned\"],d[\"pages_read\"])==(1,150,22), d" "$root/out"'
env -u BASH_ENV python3 "$searchreads" --fixture "$root/sr.txt" --since 2026-11-21T05:00Z --json >"$root/out" 2>&1
check "W10 searches by desk come from the gateway lines (research 11, research_web 1), children and started by desk from the run lines" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"searches_by_desk\"]=={\"research\":11,\"research_web\":1} and d[\"children\"]==2 and d[\"started_by_desk\"]=={\"research\":1,\"research_web\":2}, d" "$root/out"'
check "W7 SearchFresh reasons are counted by distinct hash (never text): 3 sends, 2 distinct, the repeated one first, with its length; a resumed run with lost gate state counts once" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d[\"fresh_reasons\"][0]==[\"aaaa1111\",39,2] and len(d[\"fresh_reasons\"])==2 and d[\"gate_state_lost_runs\"]==1, d" "$root/out"'
printf '##READS\n##SUM\n##FRESH\n##GW\n2026/11/21 05:00:00 gateway: search request from autoputer vm-x ( max_results=40 run=r1 desk=research conv=c)\n2026/11/21 05:01:00 gateway: search request from autoputer vm-x ( max_results=40 run=r1 desk=research conv=c)\n' >"$root/sr_empty.txt"
env -u BASH_ENV python3 "$searchreads" --fixture "$root/sr_empty.txt" --since 2026-11-21T05:00Z >"$root/out" 2>&1
check "W8 an empty console window says so instead of printing zeros" 'grep -q "console window empty" "$root/out" && ! grep -q "no SUCCESSFUL read since the run" "$root/out"'
env -u BASH_ENV python3 "$searchreads" --fixture "$root/sr.txt" --since 2026-11-21T04:00Z >"$root/out" 2>&1
check "W6 the text report prints the runs line and the SearchFresh count" 'grep -q "^runs 5  searches 13  mean per run 2.6" "$root/out" && grep -q "SearchFresh sends (ended runs): 1" "$root/out"'

# W9 the console span starts at the console's first line, not at its first read, and a
# read in the same second as a search counts (seat 2 of the read-first change, 2026-11-21).
python3 - "$root" <<'PY'
import sys
root = sys.argv[1]
VM = "vm-1669020adcece51c2f5ff9b6df399ef5"
G = lambda ts: f"{ts} gateway: search request from autoputer {VM} ( max_results=40 run=rx desk=research conv=c-rx)\n"
out = ["##READS\n", f"[ 1.0] runtime[1]: 2026/11/21 05:10:00 corpus read run=rx desk=research conv=c capture=obj:x held=false via=read\n",
       "##SUM\n##FRESH\n##GW\n", G("2026/11/21 05:09:00"), G("2026/11/21 05:10:00"), G("2026/11/21 05:12:00"), G("2026/11/21 05:14:00")]
open(root + "/sr9.txt", "w").write("".join(out))
PY
env -u BASH_ENV python3 "$searchreads" --fixture "$root/sr9.txt" --since 2026-11-21T05:00Z --json >"$root/out" 2>&1
check "W9 the run's first search precedes the first read: only the gaps that start in the console count (2), the same-second read counts, one gap has none (1)" 'python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert (d[\"later_searches\"],d[\"later_no_successful_read\"],d[\"console_span_runs\"])==(2,1,1), d" "$root/out"'

echo "status-scripts: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
