# Sourced by scripts/land and scripts/land-queue; not run on its own. Times the
# steps of a landing and appends one line per landing to a log outside the repo.
#
# Log: ${LAND_TIMING_LOG:-$HOME/.cache/go-choir/land-timings.tsv}, tab separated,
# one line per record, no header, columns:
#   kind  start_utc  outcome  failed_step  total_s  head  lanes  steps
# kind     "land" (one per scripts/land run), "wait" (scripts/land-queue waiting
#          for a push's CI, and for staging when asked) or "queue" (one per
#          scripts/land-queue run, written by lt_queue_record)
# outcome  land: landed | failed | no-push;  wait: ok | failed;
#          queue: landed | partial | failed | ci-red
# lanes    the lanes of the train, comma separated
# steps    name:seconds pairs joined by "|" (a name has no tab, newline or "|";
#          the seconds follow the LAST colon; a step run twice is summed).
#          A queue line carries counts, not seconds, in the same shape:
#          given:N|landed:N|aside:N|attempts:N|pushes:N|ci_wait:SECONDS
#          (total_s is the whole run; head is the last pushed commit)
#
# Timing never changes a landing: every function returns 0, writes nothing but
# the table and the log line, and a log that cannot be written is ignored.
# Bash 3.2 safe (the Mac's): indexed arrays only, empty-array safe under set -u.

LT_ON=0; LT_KIND=""; LT_OK=""; LT_T0=0; LT_START=""; LT_CUR=""; LT_CUR_T=0
LT_LANES=""; LT_HEAD=""; LT_NAMES=(); LT_SECS=()

lt_now() { date +%s; }
lt_log_path() { printf '%s' "${LAND_TIMING_LOG:-${HOME:-/tmp}/.cache/go-choir/land-timings.tsv}"; }
lt_clean() { printf '%s' "$1" | tr '\t\r\n|' '    '; }

# lt_init <kind> [lanes]: start timing; kind land or wait.
lt_init() {
  LT_ON=1; LT_KIND=${1:-land}; LT_LANES=$(lt_clean "${2:-}"); LT_HEAD=""
  case "$LT_KIND" in wait) LT_OK=ok ;; *) LT_OK=landed ;; esac
  LT_T0=$(lt_now); LT_START=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  LT_CUR=""; LT_CUR_T=$LT_T0; LT_NAMES=(); LT_SECS=()
  return 0
}
lt_off() { LT_ON=0; return 0; }
lt_set_head() { LT_HEAD=$(lt_clean "${1:-}"); return 0; }
lt_set_lanes() { LT_LANES=$(lt_clean "${1:-}"); return 0; }

# add seconds to a step by name (linear search: a landing has a dozen steps)
lt_add() { # name secs
  local i=0 n=${#LT_NAMES[@]}
  while [ "$i" -lt "$n" ]; do
    if [ "${LT_NAMES[$i]}" = "$1" ]; then LT_SECS[$i]=$(( ${LT_SECS[$i]} + $2 )); return 0; fi
    i=$((i+1))
  done
  LT_NAMES[$n]=$1; LT_SECS[$n]=$2
  return 0
}
lt_close() { # end the current step
  [ -n "$LT_CUR" ] || return 0
  local t; t=$(lt_now); lt_add "$LT_CUR" $(( t - LT_CUR_T ))
  LT_CUR=""; return 0
}
# lt_step <name>: end the current step and begin another.
lt_step() {
  [ "$LT_ON" = 1 ] || return 0
  lt_close
  LT_CUR=$(lt_clean "$1"); LT_CUR_T=$(lt_now)
  return 0
}

# lt_suite_name <"## cmd ..."> : a stable name for a runner step: the command with
# its ./package arguments counted, so two landings' "go test -race" runs compare.
lt_suite_name() {
  local s=${1#\#\# } w out="" n=0
  for w in $s; do
    case "$w" in ./*) n=$((n+1)) ;; *) out="$out $w" ;; esac
  done
  out=${out# }; [ "$n" -gt 0 ] && out="$out [$n pkgs]"
  printf 'suite: %s' "$(printf '%s' "$out" | cut -c1-70)"
}

# lt_finish <rc>: print the table and append the log line. Safe to call twice.
lt_finish() {
  [ "$LT_ON" = 1 ] || return 0
  LT_ON=0
  local rc=${1:-0} failed="" outcome total i=0 n steps="" line log dir
  if [ "$rc" != 0 ]; then failed=$LT_CUR; [ -n "$failed" ] || failed="(between steps)"; outcome=failed
  else outcome=$LT_OK; fi
  lt_close
  total=$(( $(lt_now) - LT_T0 ))
  n=${#LT_NAMES[@]}
  echo "$LT_KIND timings ($outcome${failed:+ at $failed}), seconds:"
  while [ "$i" -lt "$n" ]; do
    printf '  %-56s %6s\n' "${LT_NAMES[$i]}" "${LT_SECS[$i]}"
    steps="$steps${steps:+|}${LT_NAMES[$i]}:${LT_SECS[$i]}"
    i=$((i+1))
  done
  printf '  %-56s %6s\n' "total" "$total"
  line=$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "$LT_KIND" "$LT_START" "$outcome" "$(lt_clean "$failed")" "$total" "$LT_HEAD" "$LT_LANES" "$steps")
  log=$(lt_log_path); dir=$(dirname "$log")
  { mkdir -p "$dir" && printf '%s\n' "$line" >>"$log"; } 2>/dev/null || true
  return 0
}

# lt_queue_record <outcome> <start_utc> <total_s> <head> <lanes> <given> <landed> <aside> <attempts> <pushes> <ci_wait_s>
# One "queue" line per scripts/land-queue run. Never fails.
lt_queue_record() {
  local line log dir
  line=$(printf 'queue\t%s\t%s\t\t%s\t%s\t%s\tgiven:%s|landed:%s|aside:%s|attempts:%s|pushes:%s|ci_wait:%s' \
    "$(lt_clean "$2")" "$(lt_clean "$1")" "$3" "$(lt_clean "$4")" "$(lt_clean "$5")" "$6" "$7" "$8" "$9" "${10}" "${11}")
  log=$(lt_log_path); dir=$(dirname "$log")
  { mkdir -p "$dir" && printf '%s\n' "$line" >>"$log"; } 2>/dev/null || true
  return 0
}
