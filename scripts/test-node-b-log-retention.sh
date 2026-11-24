#!/usr/bin/env bash
# Host logs on Node B are kept up to 30 days, and not thrown away sooner by a
# size trim (docs/problems/host-logs-kept-a-day-or-for-ever-2026-11-24.md).
#
# Failure modes:
#  L1 the deploy preflight or the disk sweep trims the journal by size in its
#     normal path: at about 250 MB a day a 256 MB trim keeps under a day, so
#     the log from before a deploy is gone (the owner, 2026-11-23: "We can't
#     lose logs").
#  L2 the journal has no time bound, or its worst case passes 30 days, or it
#     keeps fewer than 28.
#  L3 the journal's size ceiling is small enough to cut the 30 days short at
#     today's volume (it must hold at least 30 days at 250 MB a day).
#  L4 a web access log (it holds visitors' addresses) rolls by size only, or
#     its rolled files are kept past the bound, or for fewer than 28 days, or
#     a count limit deletes them sooner than the time limit.
#  L5 an access log nobody rolls any more (a quiet host, a retired host name)
#     stays for ever: the age job must run daily on /var/log/caddy.
#     Its behaviour is scripts/test-node-b-caddy-log-age.sh.
#  L6 the failed and successful login records (btmp, wtmp: they hold remote
#     addresses) are kept past 30 days, or for fewer than 28, or are not
#     rotated while small.
#  L7 a nearly full disk cannot shrink the journal: when the reclaim has not
#     freed enough the preflight trims it by size as a last resort (after
#     the time trim, announced), and never otherwise.
#
# THE BOUND, for every log: the worst age of an entry is the span of one file
# (a file is deleted whole, when its NEWEST entry is past the limit) plus the
# limit plus the wait until the deleter next runs. With files of one day, a
# limit of 28 days and a daily deleter: 1 + 28 + 1 = 30 days. The gate
# asserts both ends: at most 30 days worst case, at least 28 days kept.
#
# L2 to L6 evaluate the host configuration (nothing is built). Without nix or
# python3 the test FAILS: a skipped gate would pass a broken promise.
#
# Run: bash scripts/test-node-b-log-retention.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/.." || exit 1
pass=0; fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); echo "FAIL: $1"; }

# --- L1, L7: what the preflight asks journalctl to do, recorded by a stub.
# The preflight runs as a COPY beside a stub retention script (it calls its
# neighbour by path, and the real one deletes system generations), with stub
# df, curl, nix and journalctl first on PATH: nothing real is touched.
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/scripts"
cp scripts/node-b-deploy-disk-preflight "$root/scripts/"
printf '#!/usr/bin/env bash\necho "fake retention"\n' >"$root/scripts/node-b-nix-retention"
printf '#!/usr/bin/env bash\necho "$*" >>"$FAKE_DIR/journalctl.calls"\n' >"$root/bin/journalctl"
# FAKE_AVAIL_SEQ: GiB free at each --output=avail call (before the reclaim,
# after it, after the last resort); the last value repeats.
cat >"$root/bin/df" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *--output=avail*)
    n=$(cat "$FAKE_DIR/df.calls" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" >"$FAKE_DIR/df.calls"
    g=$(echo "$FAKE_AVAIL_SEQ" | cut -d' ' -f"$n"); [ -n "$g" ] || g=$(echo "$FAKE_AVAIL_SEQ" | awk '{print $NF}')
    printf 'Avail\n%s\n' $((g * 1024 * 1024)) ;;
  *) echo "fake df $*" ;;
esac
STUB
for t in curl nix systemctl; do printf '#!/usr/bin/env bash\nexit 0\n' >"$root/bin/$t"; done
chmod +x "$root/bin/"* "$root/scripts/"*
preflight() { # the GiB sequence
  : >"$root/journalctl.calls"; rm -f "$root/df.calls"
  env PATH="$root/bin:$PATH" FAKE_DIR="$root" FAKE_AVAIL_SEQ="$1" PREFLIGHT_NIX_CONF="$root/nix.conf" \
    bash "$root/scripts/node-b-deploy-disk-preflight" >"$root/out" 2>&1
  echo $? >"$root/rc"
}
# Low before the reclaim, enough after it: the journal is trimmed by time only.
preflight "50 200"
if grep -q "fake retention" "$root/out"; then ok; else bad "the test did not reach the stub retention script (it must never run the real one)"; fi
if grep -q -- '--vacuum-time=28d' "$root/journalctl.calls"; then ok; else bad "L1 the reclaim does not trim the journal to 28 days: $(tr '\n' ';' <"$root/journalctl.calls")"; fi
if grep -q -- '--vacuum-size' "$root/journalctl.calls"; then bad "L1 the reclaim trims the journal by size although the disk recovered"; else ok; fi
if [ "$(cat "$root/rc")" = 0 ]; then ok; else bad "L1 the preflight failed with enough room after the reclaim"; fi
# Still low after the reclaim: the last resort trims by size, after the time trim, and says so.
preflight "50 50 200"
if [ "$(grep -c -- '--vacuum' "$root/journalctl.calls")" = 2 ] && sed -n 2p "$root/journalctl.calls" | grep -q -- '--vacuum-size=2G'; then ok; else bad "L7 no last-resort size trim after a reclaim that did not free enough: $(tr '\n' ';' <"$root/journalctl.calls")"; fi
if grep -q "LAST RESORT" "$root/out"; then ok; else bad "L7 the last resort is not announced"; fi
if [ "$(cat "$root/rc")" = 0 ]; then ok; else bad "L7 the preflight failed although the last resort freed enough"; fi
# Enough room from the start: journalctl is not called at all.
preflight "200"
if [ ! -s "$root/journalctl.calls" ]; then ok; else bad "L1 the journal is trimmed on a deploy with enough room"; fi

# A missing tool is a failure, never a pass.
command -v nix >/dev/null 2>&1 || { echo "FAIL: nix is not installed (L2 to L6 cannot be checked)"; echo "pass=$pass fail=$((fail + 1))"; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 is not installed"; echo "pass=$pass fail=$((fail + 1))"; exit 1; }

json="$(nix eval --json '.#nixosConfigurations.go-choir-b.config' --apply 'c: {
  journal = c.services.journald.settings.Journal;
  logs = builtins.mapAttrs (n: v: v.logFormat) c.services.caddy.virtualHosts;
  rotate = c.services.logrotate.settings;
  age = {
    timer = c.systemd.timers.go-choir-caddy-log-age.timerConfig or null;
    wanted = c.systemd.timers.go-choir-caddy-log-age.wantedBy or [];
    env = c.systemd.services.go-choir-caddy-log-age.serviceConfig.Environment or [];
    type = c.systemd.services.go-choir-caddy-log-age.serviceConfig.Type or "";
  };
  tmpfiles = c.systemd.tmpfiles.rules;
}' 2>"$root/eval.err")" || { echo "FAIL: nix eval: $(tail -3 "$root/eval.err")"; exit 1; }
# The sweep script is a store path only a build realises; its text is read
# from the evaluated derivation.
sweep_text="$(nix eval --raw '.#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-disk-gc.serviceConfig.ExecStart.text' 2>/dev/null || true)"

if printf '%s' "$json" | SWEEP_TEXT="$sweep_text" python3 -c '
import json, os, re, sys
c = json.load(sys.stdin); bad = []
def secs(v):
    m = re.fullmatch(r"(\d+)\s*(s|sec|m|min|h|hour|d|day|days|w|week)?", str(v).strip())
    if not m: return None
    return int(m.group(1)) * {None:1,"s":1,"sec":1,"m":60,"min":60,"h":3600,"hour":3600,"d":86400,"day":86400,"days":86400,"w":604800,"week":604800}[m.group(2)]
def size(v):
    m = re.fullmatch(r"(\d+)([KMGT])?", str(v).strip())
    return int(m.group(1)) * {None:1,"K":1<<10,"M":1<<20,"G":1<<30,"T":1<<40}[m.group(2)] if m else None
DAY = 86400
# L2: one file spans MaxFileSec; journald deletes at a rotation, which comes
# at most MaxFileSec after the file passed the limit.
j = c["journal"]
keep, span = secs(j.get("MaxRetentionSec", "")), secs(j.get("MaxFileSec", ""))
if keep is None or span is None: bad.append("L2 the journal has no MaxRetentionSec or no MaxFileSec: %r" % j)
else:
    if span + keep + span > 30 * DAY: bad.append("L2 a journal entry may live %.1f days (file %.1f + limit %.1f + next rotation %.1f)" % ((2 * span + keep) / DAY, span / DAY, keep / DAY, span / DAY))
    if keep < 28 * DAY: bad.append("L2 the journal keeps only %.1f days" % (keep / DAY))
cap = size(j.get("SystemMaxUse", ""))
if cap is None or cap < 30 * 250 * (1 << 20): bad.append("L3 the journal ceiling %r is under 30 days at 250 MB a day" % j.get("SystemMaxUse"))
# L4: a rolled file spans roll_interval; Caddy deletes at a roll, at most
# roll_interval after the file passed roll_keep_for.
for host, text in sorted(c["logs"].items()):
    opt = dict(re.findall(r"^\s*(roll_\w+)\s+(\S+)\s*$", text, re.M))
    if "roll_disabled" in text: bad.append("L4 %s: rolling is disabled" % host)
    # A roll every day: one roll_at time (Caddy rolls then by a timer, with or
    # without a request) or a roll_interval (which rolls only when a request
    # arrives, counted from the start of the process: not enough alone).
    ivs = DAY if re.fullmatch(r"\d\d:\d\d", opt.get("roll_at", "")) else None
    if "roll_interval" in opt and ivs is None: bad.append("L4 %s: roll_interval alone never rolls a quiet file; use roll_at" % host)
    kfs = secs(opt["roll_keep_for"]) if "roll_keep_for" in opt else None
    if ivs is None: bad.append("L4 %s: no daily roll (roll_at %r)" % (host, opt.get("roll_at")))
    if kfs is None: bad.append("L4 %s: no roll_keep_for" % host)
    elif ivs:
        if kfs + 2 * ivs > 30 * DAY: bad.append("L4 %s: an entry may live %.1f days" % (host, (kfs + 2 * ivs) / DAY))
        if kfs < 28 * DAY: bad.append("L4 %s: rolled files are kept only %.1f days" % (host, kfs / DAY))
    # The count limit must not bite before the time limit: at one roll a day
    # (more under load is impossible without a size roll) it needs room for
    # every day of roll_keep_for, with slack for size rolls.
    try: rk = int(opt.get("roll_keep", ""))
    except ValueError: rk = None
    if rk is None or (kfs and ivs and rk < 2 * (kfs // ivs)): bad.append("L4 %s: the count limit (roll_keep %r) can delete before the time limit; the default is 10" % (host, opt.get("roll_keep")))
    if not re.search(r"output file /var/log/caddy/", text): bad.append("L4 %s: the access log is not under /var/log/caddy" % host)
# L5: the age job, daily, on that directory, with the same limit.
a = c["age"]
env = dict(e.split("=", 1) for e in a["env"] if "=" in e)
if not a["timer"] or "timers.target" not in a["wanted"]: bad.append("L5 no timer runs the access-log age job")
else:
    if a["timer"].get("OnCalendar") not in ("daily", "*-*-* 00:00:00") and secs(a["timer"].get("OnUnitActiveSec", "")) not in (DAY,): bad.append("L5 the age job is not daily: %r" % a["timer"])
    if str(a["timer"].get("Persistent", "")).lower() != "true": bad.append("L5 the age timer is not Persistent (a host down at the hour would skip a day)")
if env.get("CADDY_LOG_DIR") != "/var/log/caddy": bad.append("L5 the age job does not name /var/log/caddy: %r" % env)
if env.get("CADDY_LOG_MAX_DAYS") != "28": bad.append("L5 the age job limit is %r, not 28 days (1 day of file + 28 + 1 day to the next run = 30)" % env.get("CADDY_LOG_MAX_DAYS"))
if a["type"] != "oneshot": bad.append("L5 the age job is not a oneshot")
if any(re.match(r"\S+\s+/var/log/caddy\s", r) and re.search(r"\s\d+[dhw]\s*$", r) for r in c["tmpfiles"]): bad.append("L5 a tmpfiles age rule also sweeps /var/log/caddy (it unlinks a file Caddy holds open)")
# L6: logrotate keeps `rotate` old files; a record lives at most
# (rotate + 1) periods and maxage also bounds old files.
for f in ("/var/log/btmp", "/var/log/wtmp"):
    r = c["rotate"].get(f) or {}
    per = {"daily": 1, "weekly": 7, "monthly": 31}.get(r.get("frequency"))
    if per is None: bad.append("L6 %s has no known frequency" % f); continue
    worst = (int(r.get("rotate", 0)) + 1) * per
    if worst > 30: bad.append("L6 %s is kept up to %d days (%s, rotate %s)" % (f, worst, r.get("frequency"), r.get("rotate")))
    if worst < 28: bad.append("L6 %s is kept only %d days (%s, rotate %s)" % (f, worst, r.get("frequency"), r.get("rotate")))
    if r.get("minsize") not in (None, ""): bad.append("L6 %s is not rotated while under %s, so it has no time bound" % (f, r.get("minsize")))
    try: ma = int(r.get("maxage", ""))
    except (TypeError, ValueError): ma = None
    if ma is None or ma > 30 or ma < 28: bad.append("L6 %s maxage is %r, not 28 to 30" % (f, r.get("maxage")))
t = os.environ.get("SWEEP_TEXT", "")
if t:
    if "--vacuum-size" in t: bad.append("L1 the disk sweep trims the journal by size")
    if "--vacuum-time=28d" not in t: bad.append("L1 the disk sweep does not trim the journal to 28 days")
else: bad.append("L1 the disk sweep text could not be read from the evaluated configuration")
for b in bad: print("FAIL:", b)
sys.exit(1 if bad else 0)
'; then ok; else fail=$((fail + 1)); fi

echo "pass=$pass fail=$fail"
[ "$fail" = 0 ]
