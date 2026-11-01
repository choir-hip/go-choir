#!/usr/bin/env bash
# Behavioural test for scripts/node-b-growth-meter. It drives the real script
# with fake df, du, runuser/psql, systemctl and journalctl and a temp state
# root, cgroup tree and /proc, so it needs no Linux, no root and no
# PostgreSQL.
#
#   env -u BASH_ENV bash scripts/test-node-b-growth-meter.sh
#
# Failure modes, written before the script:
#   G1  a reading never lands in the ledger, or lands without items, units,
#       receipts or errors fields
#   G2  the receipt count uses a created_at scan (no index) instead of the
#       sequence mark, or counts on the first reading
#   G3  a heavy store is measured every hour instead of once per window
#   G4  an item is classified wrongly (receipts must be machine, the
#       Autopaper graph and the tape user)
#   G5  the report misses machine growth, a peak over 90% of MemoryMax or an
#       oom_kill increment, or flags user-state growth as machine growth
#   G6  the meter appends past its size cap
#   G7  the unit wrapper PATH lacks a tool the script runs (an activation
#       wrapper once lacked sed)
#   G8  a failing tool aborts the whole reading instead of recording an error
#   G9  the unit lacks MemoryMax, a timeout, idle nice/ionice, or is not wired
#       to an hourly timer
#   M1  a memory reading misses a Firecracker process (a jailed one has no
#       vm-state pid file), names it by anything but its --id, maps it to the
#       wrong computer, or counts a process that is not Firecracker
#   M2  the memory report shows no stable state or spike for a computer,
#       puts the spike at the wrong minute, or counts a restart's falling
#       read_bytes as negative reads
#   M3  the memory unit is not wired to a per-minute timer, or lacks
#       MemoryMax or idle priority
#   M4  per-VM stall is read from the wrong cgroup, is a 10 s snapshot
#       instead of the interval's share, is computed across a cgroup change,
#       or flags a VM in the shared direct cgroup as if the stall were its own
set -u
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/node-b-growth-meter"
nixfile="$here/../nix/node-b.nix"
fail=0
pass=0

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

check() {
  if eval "$2"; then pass=$((pass + 1)); else fail=$((fail + 1)); echo "FAIL: $1"; echo "  condition: $2"; fi
}

mkdir -p "$root/bin"
cat >"$root/bin/df" <<'EOF'
#!/usr/bin/env bash
echo "1B-blocks Used Avail"
echo "${FAKE_DF:-1000000 400000 600000}"
EOF
cat >"$root/bin/du" <<'EOF'
#!/usr/bin/env bash
# fake du: every child directory is 1000*MULT bytes; -sx prints one total.
mult="${FAKE_DU_MULT:-1}"
echo "du $*" >>"$FAKE_DIR/du.log"
[ -n "${FAKE_DU_FAIL:-}" ] && { echo "du: boom" >&2; exit 2; }
path="${!#}"
if [[ " $* " == *" -d1 "* ]]; then
  n=0
  for c in "$path"/*/; do
    [ -d "$c" ] || continue
    printf '%s\t%s\n' $((1000 * mult)) "${c%/}"
    n=$((n + 1))
  done
  printf '%s\t%s\n' $((1000 * mult * n + 4096)) "$path"
else
  printf '%s\t%s\n' $((5000 * mult)) "$path"
fi
EOF
cat >"$root/bin/btrfs" <<'EOF'
#!/usr/bin/env bash
# fake btrfs filesystem du -s --raw PATH
echo "btrfs $*" >>"$FAKE_DIR/btrfs.log"
echo "     Total   Exclusive  Set shared  Filename"
echo "  ${FAKE_BTRFS_TOTAL:-30000}  ${FAKE_BTRFS_EXCL:-10000}  ${FAKE_BTRFS_SHARED:-5000}  ${!#}"
EOF
cat >"$root/bin/runuser" <<'EOF'
#!/usr/bin/env bash
# fake runuser -u postgres -- psql ... -d DB -c SQL
sql="${!#}"
case "$sql" in
  *"from pg_database"*) printf 'control\ngraph\n' ;;
  *"pg_database_size"*) echo 123456 ;;
  *"pg_stat_user_tables"*) printf 'computer_event_append_receipts\t%s\t%s\n' $((1000 * ${FAKE_DU_MULT:-1})) $((50 * ${FAKE_DU_MULT:-1})) ;;
  *"with recursive"*) printf 'computer-a\t%s\n' "${FAKE_HEAD:-100}" ;;
  *"event_kind, count"*)
    echo "$sql" >>"$FAKE_DIR/kind-sql.log"
    printf 'projection_batch_recorded\t7\n' ;;
  *) echo "unexpected sql: $sql" >&2; exit 3 ;;
esac
EOF
cat >"$root/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
case "$1" in
  list-units) printf 'go-choir-vmctl.service loaded active running x\ngo-choir-nix-gc.service loaded inactive dead y\npostgresql.service loaded active running z\n' ;;
  show) echo infinity ;;
esac
EOF
cat >"$root/bin/journalctl" <<'EOF'
#!/usr/bin/env bash
case " $* " in
  *" -k "*) [ -n "${FAKE_OOM:-}" ] && echo "Out of memory: Killed process 1 (restic)"; exit 0 ;;
  *) echo "go-choir-nix-gc.service: Consumed 6.357s CPU time over 11.207s wall clock time, ${FAKE_GC_PEAK:-120.1M} memory peak, 700.1M read from disk." ;;
esac
EOF
chmod +x "$root/bin/"*

# A temp world: state root, cgroups, proc.
w="$root/world"
mkdir -p "$w/state/vm-state/vm-aaa" "$w/state/vm-state/candidate-x" \
  "$w/state/platform-artifacts/tape-seg/computer-a" \
  "$w/state/platform-artifacts/sha256/pin-receipts/computer-a" \
  "$w/state/platform-artifacts/sha256/file-cas-chunks/computer-a" \
  "$w/state/guest-releases/r1" "$w/state/files" "$w/state/mail" \
  "$w/cg/system.slice/go-choir-vmctl.service" "$w/cg/system.slice/postgresql.service" \
  "$w/proc/4242" "$root/fake"
: >"$w/state/vm-state/vm-aaa/data.img"
printf 4242 >"$w/state/vm-state/vm-aaa/firecracker.pid"
printf 'VmRSS:\t   2048 kB\n' >"$w/proc/4242/status"
printf 'MemAvailable:   1000000 kB\n' >"$w/meminfo"
printf '66 1 0:32 / / rw - btrfs /dev/x rw\n69 66 0:26 / /run rw - tmpfs tmpfs rw\n' >"$w/mountinfo"
cg() { # unit peak max oom_kill
  d="$w/cg/system.slice/$1"
  echo "$2" >"$d/memory.peak"; echo "$2" >"$d/memory.current"; echo "$3" >"$d/memory.max"
  printf 'low 0\nhigh 0\nmax 0\noom 0\noom_kill %s\noom_group_kill 0\n' "$4" >"$d/memory.events"
  printf 'anon %s\nfile 5\n' "${5:-1000}" >"$d/memory.stat"
  printf 'some avg10=0.00 avg60=%s avg300=0.00 total=1\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=1\n' "${6:-0.00}" >"$d/memory.pressure"
}
cg go-choir-vmctl.service 1000000000 8000000000 0
cg postgresql.service 1000000000 4000000000 0

run_meter() { # args...; env set by caller via env
  env -u BASH_ENV PATH="$root/bin:$PATH" FAKE_DIR="$root/fake" GROWTH_DIR="$root/ledger" \
    GROWTH_STATE_ROOT="$w/state" GROWTH_MOUNTINFO="$w/mountinfo" GROWTH_CGROUP="$w/cg" GROWTH_PROC="$w/proc" GROWTH_MEMINFO="$w/meminfo" \
    "$@"
}
t0=1793000000   # 2026-10-26T06:13:20Z
meter() { # now args...
  local now="$1"; shift
  run_meter env GROWTH_NOW="$now" "$@" python3 "$script" ${METER_ARGS:-}
}

# ---- first reading ---------------------------------------------------------
meter $t0 >"$root/out1" 2>&1
check "G1 first reading exits 0 and appends" "[ -s '$root/ledger/'*.jsonl ] && grep -q 'items=' '$root/out1'"
ledger="$(ls "$root"/ledger/*.jsonl | head -1)"
check "G1 one line, valid JSON with required fields" "python3 -c \"
import json,sys
ls=open('$ledger').read().splitlines()
assert len(ls)==1
r=json.loads(ls[0])
for k in ('ts','items','classes','units','receipts_since_last','errors','vm_rss','mem_available','kernel_oom_since_last'): assert k in r,k
assert r['items']['fs.root.used']==400000
assert r['items']['fs.nix-store']==5000
assert r['items']['fs.vm-state.vm-aaa.data.img']==0
assert 'fs.vm-state.candidate-x.data.img' not in r['items']
assert r['vm_rss']=={'vm-aaa':2097152}, r['vm_rss']
assert r['mem_available']==1024000000
assert r['units']['go-choir-vmctl.service']['peak']==1000000000
assert r['units']['go-choir-nix-gc.service']['logged_peak']==int(120.1*1048576)
assert r['items']['pg.control.size']==123456
assert r['items']['pg.control.computer_event_append_receipts.rows']==50
assert r['errors']==[], r['errors']
assert r['items']['fs.guest-releases']==5000
assert r['items']['fs.guest-releases.excl']==10000 and r['items']['fs.guest-releases.shared']==5000 and r['items']['fs.guest-releases.real']==15000
assert r['units']['go-choir-vmctl.service']['anon']==1000 and r['units']['go-choir-vmctl.service']['pressure_some_avg60']==0.0
assert 'events_delta' not in r['units']['go-choir-vmctl.service']
\""
check "G2 first reading counts no receipts, only sets the mark" "! grep -q receipts_since_last.computer-a '$ledger' && [ ! -e '$root/fake/kind-sql.log' ]"
check "G3 heavy stores measured on the first reading" "grep -q 'fs.artifacts.pin-receipts' '$ledger'"
check "G4 classes" "python3 -c \"
import json
r=json.loads(open('$ledger').readline())
c=r['classes']
assert c['pg.control.computer_event_append_receipts.rows']=='machine'
assert c['pg.graph.size']=='user'
assert c['fs.artifacts.tape-seg.computer-a']=='user'
assert c['fs.artifacts.pin-receipts']=='machine'
assert c['fs.nix-store']=='machine'
assert c['fs.vm-state.vm-aaa.data.img']=='mixed'
\""

# ---- second reading one hour later: receipts via the sequence mark ---------
: >"$root/fake/du.log"
FAKE_HEAD=130 meter $((t0 + 3600)) >"$root/out2" 2>&1
check "G2 second reading counts rows above the mark through sequence" "grep -q 'sequence > 100' '$root/fake/kind-sql.log' && ! grep -q created_at '$root/fake/kind-sql.log'"
check "G2 receipts per computer and kind recorded" "python3 -c \"
import json
r=json.loads(open('$ledger').read().splitlines()[1])
assert r['receipts_since_last']=={'computer-a':{'projection_batch_recorded':7}}, r['receipts_since_last']
\""
check "G3 heavy stores skipped within the window" "! grep -q 'pin-receipts' '$root/fake/du.log' && ! grep -q file-cas-chunks '$root/fake/du.log' && grep -q tape-seg '$root/fake/du.log'"

# ---- G8 a failing du is recorded, not fatal -------------------------------
FAKE_DU_FAIL=1 meter $((t0 + 7200)) >"$root/out3" 2>&1
check "G8 failing du still appends a line with errors" "[ \$(wc -l <'$ledger') -eq 3 ] && python3 -c \"
import json
r=json.loads(open('$ledger').read().splitlines()[2])
assert any('du' in e or 'nix-store' in e for e in r['errors']), r['errors']
assert 'fs.root.used' in r['items']
\""

# ---- report: baseline = reading 24h earlier --------------------------------
rm -rf "$root/ledger"
meter $t0 >/dev/null 2>&1
cg go-choir-vmctl.service 7500000000 8000000000 0 1000 0.00   # peak 94% of MemoryMax, all page cache
cg postgresql.service 1000000000 4000000000 2         # oom_kill 0 -> 2 would need a prior reading
FAKE_DU_MULT=100000 FAKE_HEAD=500 meter $((t0 + 43200)) >/dev/null 2>&1
cg postgresql.service 1000000000 4000000000 3
FAKE_DU_MULT=100000 FAKE_HEAD=900 FAKE_OOM=1 meter $((t0 + 86400)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 86400)) >"$root/report" 2>&1
check "G5 report flags machine growth" "grep -q 'machine state growing: fs.nix-store' '$root/report'"
check "G5 report flags machine row growth" "grep -q 'machine state growing: pg.control.computer_event_append_receipts.rows' '$root/report'"
check "G5 a peak near MemoryMax without anon or pressure is not flagged" "! grep -q '! .*go-choir-vmctl.service' '$root/report'"
check "G5 report flags oom_kill delta, not the cumulative total" "grep -q 'oom_kill +1 since previous reading: postgresql.service' '$root/report' && ! grep -q 'oom_kill +3' '$root/report'"
check "G5 report flags kernel OOM lines" "grep -q 'kernel OOM log lines' '$root/report'"
check "G5 user-state growth is not a machine flag" "! grep -q 'machine state growing: fs.artifacts.tape-seg' '$root/report' && grep -q 'fs.artifacts.tape-seg' '$root/report'"
check "G5 report prints the 24 h span" "grep -q '(24.0 h)' '$root/report'"

# anon near the line and memory pressure flag
cg go-choir-vmctl.service 7500000000 8000000000 0 7000000000 3.50
meter $((t0 + 90000)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 90000)) >"$root/report3" 2>&1
check "G5 anon near MemoryMax flags" "grep -q 'memory: go-choir-vmctl.service anon' '$root/report3'"
check "G5 memory pressure flags" "grep -q 'memory pressure: go-choir-vmctl.service some avg60 3.50' '$root/report3'"

# reflink-aware growth: du grows, .real flat -> no flag; .real grows -> flag
cg go-choir-vmctl.service 1000000000 8000000000 0
rm -rf "$root/ledger"
meter $t0 >/dev/null 2>&1
FAKE_DU_MULT=100000 meter $((t0 + 86400)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 86400)) >"$root/report4" 2>&1
check "G5 du-only growth of a reflink store is not flagged" "! grep -q 'machine state growing: fs.guest-releases ' '$root/report4' && grep -q 'fs.guest-releases ' '$root/report4'"
rm -rf "$root/ledger"
meter $t0 >/dev/null 2>&1
FAKE_BTRFS_EXCL=900000000 meter $((t0 + 86400)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 86400)) >"$root/report5" 2>&1
check "G5 real (exclusive+shared) growth is flagged" "grep -q 'machine state growing: fs.guest-releases.real' '$root/report5'"

# flat day: no growth flags
rm -rf "$root/ledger"
cg go-choir-vmctl.service 1000000000 8000000000 0
cg postgresql.service 1000000000 4000000000 0
meter $t0 >/dev/null 2>&1
meter $((t0 + 86400)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 86400)) >"$root/report2" 2>&1
check "G5 flat day reports no flags" "grep -q '^FLAGS: none' '$root/report2'"

# ---- G6 size cap -----------------------------------------------------------
before="$(wc -c <"$(ls "$root"/ledger/*.jsonl | head -1)")"
run_meter env GROWTH_NOW=$((t0 + 90000)) GROWTH_MAX_FILE_BYTES=$before python3 "$script" >"$root/out4" 2>&1
rc=$?
check "G6 refuses past the cap, exit 1, file unchanged" "[ $rc -eq 1 ] && [ \$(wc -c <\"\$(ls '$root'/ledger/*.jsonl | head -1)\") -eq $before ] && grep -q 'byte cap' '$root/out4'"

# ---- G7 wrapper PATH carries every tool -----------------------------------
unit="$(awk '/systemd.services.go-choir-growth-meter = \{/,/^  \};/' "$nixfile")"
binpath="$(grep -o 'makeBinPath \[[^]]*\]' <<<"$unit")"
tools="$(python3 -c "
import re,sys
src=open('$script').read()
t=re.search(r'^TOOLS = \(([^)]*)\)',src,re.M).group(1)
declared=set(re.findall(r'\"([a-z0-9-]+)\"',t))
called=set(re.findall(r'run\(\[\"([a-z0-9-]+)\"',src))
print('MISMATCH' if not called<=declared else ' '.join(sorted(declared)))
")"
check "G7 TOOLS covers every run([...]) call" "[ '$tools' != MISMATCH ]"
pkg_for() {
  case "$1" in
    df|du) echo coreutils ;;
    btrfs) echo btrfs-progs ;;
    runuser) echo util-linux ;;
    psql) echo postgresql_17 ;;
    journalctl|systemctl) echo systemd ;;
    *) echo UNMAPPED:"$1" ;;
  esac
}
for t in $tools; do
  p="$(pkg_for "$t")"
  check "G7 PATH carries $t (pkgs.$p)" "grep -q 'pkgs.$p ' <<<'$binpath'"
done
check "G7 python3 is the interpreter in ExecStart" "grep -q 'pkgs.python3}/bin/python3' <<<'$unit'"

# ---- G9 unit shape ---------------------------------------------------------
check "G9 MemoryMax set" "grep -q 'MemoryMax = \"256M\"' <<<'$unit'"
check "G9 timeout set" "grep -q 'TimeoutStartSec' <<<'$unit'"
check "G9 nice 19 and idle IO" "grep -q 'Nice = 19' <<<'$unit' && grep -q 'IOSchedulingClass = \"idle\"' <<<'$unit'"
check "G9 hourly timer" "grep -A8 'systemd.timers.go-choir-growth-meter' '$nixfile' | grep -q 'OnCalendar = \"hourly\"'"

# ---- M1-M3 per-minute memory readings ---------------------------------------
fc() { # pid id anon_kb file_kb read_bytes [comm] [cgroup]
  local d="$w/proc/$1"; mkdir -p "$d"
  printf '0::%s\n' "${7:-/system.slice/system-choirvms.slice/vms/$2}" >"$d/cgroup"
  printf '%s\n' "${6:-firecracker}" >"$d/comm"
  printf '/firecracker\0--id\0%s\0--api-sock\0/run/fc.sock\0' "$2" >"$d/cmdline"
  printf 'Name:\tfirecracker\nRssAnon:\t %s kB\nRssFile:\t %s kB\n' "$3" "$4" >"$d/status"
  printf 'rchar: 1\nread_bytes: %s\n' "$5" >"$d/io"
}
mkdir -p "$w/state/vm-state"
printf '{"ownerships":[{"vm_id":"vm-paper","computer_id":"computer-paper","state":"active","user_id":"u"},{"vm_id":"candidate-fleet-x","computer_id":"computer-fleet","state":"active"}]}' >"$w/state/vm-state/ownerships.json"
printf 'MemTotal: 33554432 kB\nMemAvailable: 26214400 kB\nAnonPages: 4194304 kB\nCached: 20971520 kB\nShmem: 1048576 kB\nSReclaimable: 1048576 kB\nSUnreclaim: 524288 kB\nSwapTotal: 16777216 kB\nSwapFree: 16515072 kB\n' >"$w/meminfo2"
printf 'some avg10=0.50 avg60=0.10 avg300=0.00 total=100\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=50\n' >"$w/pressure"
rm -rf "$root/ledger"
mem_meter() { # now
  run_meter env GROWTH_NOW="$1" GROWTH_MEMINFO="$w/meminfo2" GROWTH_PRESSURE="$w/pressure" python3 "$script" --memory
}
fc 501 vm-paper 3145728 5120 0
fc 502 candidate-fleet-x 1048576 1024 0
fc 503 vm-other 1 1 0 bash
vmcg() { # cgroup io_total_us memory_total_us
  mkdir -p "$w/cg$1"
  printf 'some avg10=0.00 avg60=0.00 avg300=0.00 total=%s\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n' "$2" >"$w/cg$1/io.pressure"
  printf 'some avg10=0.00 avg60=0.00 avg300=0.00 total=%s\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n' "${3:-0}" >"$w/cg$1/memory.pressure"
}
vmcg /system.slice/system-choirvms.slice/vms/vm-paper 0
vmcg /system.slice/system-choirvms.slice/vms/candidate-fleet-x 0
mem_meter $t0 >"$root/mout1" 2>&1
mledger="$root/ledger/mem-2026-10.jsonl"
check "M1 memory reading lands with host, pressure and named VMs" "python3 -c \"
import json
ls=open('$mledger').read().splitlines()
assert len(ls)==1
r=json.loads(ls[0])
assert r['host']['MemAvailable']==26214400*1024 and r['host']['AnonPages']==4194304*1024, r['host']
assert r['pressure']['some_avg10']==0.5, r['pressure']
v=r['vms']
assert set(v)=={'vm-paper','candidate-fleet-x'}, v
assert {k: v['vm-paper'][k] for k in ('anon','file','read_bytes','computer','state')}=={'anon':3145728*1024,'file':5120*1024,'read_bytes':0,'computer':'computer-paper','state':'active'}, v['vm-paper']
assert v['candidate-fleet-x']['computer']=='computer-fleet'
assert 'user_id' not in ls[0]
assert v['vm-paper']['io_some_us']==0 and v['vm-paper']['cgroup']=='/system.slice/system-choirvms.slice/vms/vm-paper'
assert 'cgroup_shared' not in v['vm-paper']
\""
# A spike at +60 min (8 GiB) and a restart at +120 min (read_bytes falls).
# The paper's io stall: 30% of the first hour (1080 s of 3600), then none.
vmcg /system.slice/system-choirvms.slice/vms/vm-paper 1080000000
fc 501 vm-paper 8388608 5120 $((10 << 30))
mem_meter $((t0 + 3600)) >/dev/null 2>&1
fc 501 vm-paper 3145728 5120 $((1 << 30))
mem_meter $((t0 + 7200)) >/dev/null 2>&1
fc 501 vm-paper 3145728 5120 $((2 << 30))
mem_meter $((t0 + 10800)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 10800)) >"$root/mreport" 2>&1
check "M2 report prints host stable state and spikes" "grep -q 'p50 = stable state' '$root/mreport' && grep -Eq 'anon +4.00 +4.00 +4.00' '$root/mreport' && grep -Eq 'swap used +0.25' '$root/mreport'"
spike_min="$(python3 -c "import time;print(time.strftime('%H:%M',time.gmtime($t0+3600)))")"
check "M2 report shows the computer's p50, max and the spike's minute" "grep -Eq 'computer-paper +3.00 +8.00 +8.00  ${spike_min}Z' '$root/mreport'"
check "M2 a restart's falling read_bytes is not negative (10 + 1 GiB over 3 h is 88 GiB a day)" "grep -Eq 'computer-paper .* 88.0 GiB' '$root/mreport'"
check "M2 host pressure over the flag is not flagged at 0.5%" "! grep -q 'host memory pressure' '$root/mreport'"
check "M4 per-VM io stall is the interval's share (30% in the first hour)" "grep -Eq 'computer-paper +0.0 / +0.0 +30.0 / 30.0' '$root/mreport'"
check "M4 a VM stalled past the flag is flagged" "grep -q 'computer-paper io stall p95 30.0%' '$root/mreport'"
check "M4 a quiet VM is not flagged" "! grep -q 'computer-fleet io stall' '$root/mreport'"
# A direct VM in the vmctl unit's cgroup is marked shared and never flagged.
rm -rf "$root/ledger"
vmcg /system.slice/go-choir-vmctl.service/supervisor 0
fc 501 vm-paper 3145728 5120 0 firecracker /system.slice/go-choir-vmctl.service/supervisor
mem_meter $t0 >/dev/null 2>&1
vmcg /system.slice/go-choir-vmctl.service/supervisor 1800000000
mem_meter $((t0 + 3600)) >/dev/null 2>&1
METER_ARGS=--report meter $((t0 + 3600)) >"$root/mreport2" 2>&1
check "M4 a direct VM reads as shared and is not flagged" "grep -q 'computer-paper (shared cgroup)' '$root/mreport2' && ! grep -q 'computer-paper io stall' '$root/mreport2'"

munit="$(awk '/systemd.services.go-choir-memory-meter = \{/,/^  \};/' "$nixfile")"
check "M3 memory unit runs --memory under MemoryMax at idle priority" "grep -q -- '--memory' <<<'$munit' && grep -q 'MemoryMax' <<<'$munit' && grep -q 'Nice = 19' <<<'$munit'"
check "M3 per-minute timer" "grep -A8 'systemd.timers.go-choir-memory-meter' '$nixfile' | grep -q 'OnCalendar = \"minutely\"'"

echo "growth meter: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
