#!/usr/bin/env bash
# Jev call statistics from the gateway journal's `jev call` lines: counts and
# timings only, no question text, no answers, no state (the line carries none).
# Reproduces sections 1 and 2 of docs/reference/system1/our-usage.md.
#
#   scripts/proofs/jev-call-stats.sh --fetch "2026-11-10 04:00" [--until "2026-11-10 22:00"]
#       one read-only `journalctl` over ssh to node-b, grep'd on the host for the
#       `jev call` and `jev hour summary` lines only, saved to a temp file, then
#       analysed locally (nothing heavy runs on Node B)
#   scripts/proofs/jev-call-stats.sh FILE
#       analyse a saved journal extract (`journalctl -o short-iso` format)
#
# Output: totals and status counts; upstream_ms p50/p90/p99/max overall and by
# questions per call (1, 2, 3-4, 5-8, 9-12, 13+); queue_ms by class; calls and
# questions per hour (UTC); calls per second histogram (bursts); calls over 1 s.
# Tokens are not in the line; see the page for where they come from.
set -uo pipefail

usage() { sed -n '2,15p' "$0"; exit 2; }
file=""; fetched=""; work=""
cleanup() { [ -n "$fetched" ] && rm -f "$fetched"; [ -n "$work" ] && rm -rf "$work"; }
trap cleanup EXIT
if [ "${1:-}" = "--fetch" ]; then
  since="${2:-}"; [ -n "$since" ] || usage
  shift 2
  until_arg=""
  if [ "${1:-}" = "--until" ]; then until_arg="--until '${2:-}'"; shift 2; fi
  file="$(mktemp -t jevcalls.XXXXXX)"; fetched="$file"
  ssh node-b "journalctl -u go-choir-gateway --no-pager -o short-iso --since '$since' $until_arg | grep -E 'jev (call|hour summary)'" >"$file" || exit 1
elif [ -n "${1:-}" ] && [ -r "${1:-}" ]; then
  file="$1"
else
  usage
fi

work="$(mktemp -d -t jevstats.XXXXXX)"

# One row per call: ts class questions queue_ms upstream_ms status ok computer-prefix pid
grep 'jev call' "$file" | sed -E \
  's/^([^ ]+) .*\[([0-9]+)\]: .*jev call computer=([^ ]+) class=([^ ]+) questions=([0-9]+) queue_ms=([0-9]+) upstream_ms=([0-9]+) status=([0-9]+) ([a-z]+).*/\1 \4 \5 \6 \7 \8 \9 \3 \2/' \
  | awk 'NF==9 { print $1, $2, $3, $4, $5, $6, $7, substr($8,1,11), $9 }' >"$work/rows"

n=$(wc -l <"$work/rows" | tr -d ' ')
echo "== window: $(head -1 "$work/rows" | cut -d' ' -f1) .. $(tail -1 "$work/rows" | cut -d' ' -f1)  calls=$n"
echo "== status / ok:";   awk '{c[$6" "$7]++} END{for(k in c) print "  status/ok " k ": " c[k]}' "$work/rows" | sort
echo "== class:";         awk '{c[$2]++} END{for(k in c) print "  " k ": " c[k]}' "$work/rows" | sort
echo "== computer (prefix):"; awk '{c[$8]++} END{for(k in c) print "  " k ": " c[k]}' "$work/rows" | sort

# pctl FILE-OF-NUMBERS -> n p50 p90 p99 max
pctl() {
  sort -n | awk '{a[NR]=$1} END{ if(NR==0){print "n=0"; exit}
    i=int(0.50*NR); if(i<1)i=1; j=int(0.90*NR); if(j<1)j=1; k=int(0.99*NR); if(k<1)k=1;
    printf "n=%d p50=%d p90=%d p99=%d max=%d\n", NR, a[i], a[j], a[k], a[NR] }'
}

echo "== upstream_ms, status 200 (all):"
awk '$6==200 {print $5}' "$work/rows" | pctl
echo "== upstream_ms by questions per call (status 200):"
for bin in "1 1" "2 2" "3 4" "5 8" "9 12" "13 9999"; do
  set -- $bin
  printf "  q %s-%s: " "$1" "$2"
  awk -v lo="$1" -v hi="$2" '$6==200 && $3>=lo && $3<=hi {print $5}' "$work/rows" | pctl
done
echo "== effective ms per question (sum upstream_ms / sum questions, status 200) by bin:"
for bin in "1 1" "2 2" "3 4" "5 8" "9 12" "13 9999"; do
  set -- $bin
  awk -v lo="$1" -v hi="$2" '$6==200 && $3>=lo && $3<=hi {u+=$5; q+=$3; c++} END{ if(c) printf "  q %s-%s: calls=%d questions=%d ms/question=%.1f\n", lo, hi, c, q, u/q }' "$work/rows"
done
echo "== queue_ms by class (n, queued>0, p50 p90 p99 max):"
for cls in bulk interactive oped_gate; do
  printf "  %s: " "$cls"
  awk -v c="$cls" '$2==c {print $4}' "$work/rows" | pctl
  awk -v c="$cls" '$2==c && $4>0 {n++} END{printf "    queued>0: %d\n", n+0}' "$work/rows"
done
echo "== calls over 1000 ms upstream, by hour (UTC):"
awk '$6==200 && $5>1000 {print substr($1,1,13)}' "$work/rows" | sort | uniq -c | awk '{printf "  %s %d\n", $2, $1}'
echo "== calls and questions per hour (UTC), p50/p90 upstream_ms:"
cut -c1-13 "$work/rows" | sort -u | while read -r h; do
  c=$(awk -v h="$h" 'substr($1,1,13)==h' "$work/rows" | wc -l | tr -d ' ')
  q=$(awk -v h="$h" 'substr($1,1,13)==h {s+=$3} END{print s+0}' "$work/rows")
  printf "  %s calls=%s questions=%s " "$h" "$c" "$q"
  awk -v h="$h" 'substr($1,1,13)==h && $6==200 {print $5}' "$work/rows" | pctl
done
echo "== histogram of questions per call:"
awk '{c[$3]++} END{for(k in c) print k, c[k]}' "$work/rows" | sort -n | awk '{printf "  q=%s: %s\n", $1, $2}'
echo "== calls per second (same log second), histogram; N>=5 is a burst:"
awk '{c[$1]++} END{for(k in c) h[c[k]]++; for(n in h) print n, h[n]}' "$work/rows" | sort -n | awk '{printf "  %s calls in a second: %s seconds\n", $1, $2}'
echo "== gateway restarts seen (distinct pids in the window):"
grep 'jev call' "$file" | sed -E 's/.*\[([0-9]+)\]: .*/\1/' | uniq | wc -l | awk '{print "  " $1}'
echo "== jev hour summary lines (emitted per process hour; restarts suppress them):"
grep 'jev hour summary' "$file" | sed -E 's/^([^ ]+) .*jev hour summary: /  \1 /' || true
