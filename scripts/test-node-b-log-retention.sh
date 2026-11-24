#!/usr/bin/env bash
# No host log entry on Node B is kept longer than 30 days
# (docs/problems/host-logs-kept-a-day-or-for-ever-2026-11-24.md).
#
# THE PROMISE IS AN UPPER BOUND ONLY. Nothing here promises a minimum: a
# nearly full disk may delete sooner. The 7-day floor asserted below is OURS,
# an operating floor so a typo cannot throw the logs away; it is not policy.
#
# THE BOUND. Every deleter goes by a file's OLDEST entry, or bounds the span
# of a file. The gate computes the worst case from the evaluated settings:
#   journal        limit + the job's cadence + one missed run
#   login records  one file's span + limit + cadence + one missed run
# and fails over 30 days, or with under 5 days of margin for a host that is
# off (the timers are Persistent, so the job runs when it comes back).
#
# Failure modes:
#  L1 the deploy preflight or the disk sweep trims the journal by size in its
#     normal path (at about 280 MB a day a 256 MB trim keeps under a day), or
#     not to the limit.
#  L2 the journal has no time bound, its worst case passes 30 days or leaves
#     under 5 days of margin, or the limit is under the 7-day floor.
#  L3 the journal's ceiling is too small for the limit at today's volume.
#  L4 a web access log (visitors' addresses) is written to a file: nothing
#     rolls a file Caddy has not written to since it started, so a file has
#     no bound. It must go to the journal.
#  L5 the daily age job is missing, not daily, not Persistent, does not use
#     the journal's limit, does not name the access-log folder or the
#     last-login records, or sshd still prints the last login.
#     Its behaviour is scripts/test-node-b-log-age.sh.
#  L6 the login records (btmp, wtmp) are not in daily files, are kept past
#     the bound, skip a small or an empty file, or have no maxage.
#  L7 a nearly full disk cannot shrink the journal: the preflight trims it by
#     size as a last resort (after the time trim, announced), never otherwise.
#  L8 a tmpfiles age rule sweeps a log folder by last-write time.
#
# L2 to L8 evaluate the host configuration (nothing is built). Without nix or
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
if grep -q -- '--vacuum-time=21d' "$root/journalctl.calls"; then ok; else bad "L1 the reclaim does not trim the journal to 21 days: $(tr '\n' ';' <"$root/journalctl.calls")"; fi
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
  rotateTimer = c.systemd.timers.logrotate.timerConfig or {};
  age = {
    timer = c.systemd.timers.go-choir-log-age.timerConfig or null;
    wanted = c.systemd.timers.go-choir-log-age.wantedBy or [];
    env = c.systemd.services.go-choir-log-age.serviceConfig.Environment or [];
    type = c.systemd.services.go-choir-log-age.serviceConfig.Type or "";
    user = c.systemd.services.go-choir-log-age.serviceConfig.User or "root";
  };
  printLastLog = c.services.openssh.settings.PrintLastLog or null;
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
DAY = 86400; BOUND = 30; MARGIN = 5; FLOOR = 7
def bound(name, worst_days, limit_days):
    if worst_days > BOUND: bad.append("%s: an entry may live %.1f days, over %d" % (name, worst_days, BOUND))
    elif BOUND - worst_days < MARGIN: bad.append("%s: worst case %.1f days leaves under %d days of margin" % (name, worst_days, MARGIN))
    if limit_days < FLOOR: bad.append("%s: the limit of %.1f days is under our %d-day operating floor" % (name, limit_days, FLOOR))
# L5 first: the cadence of the job every bound leans on.
a = c["age"]; cadence = None
env = dict(e.split("=", 1) for e in a["env"] if "=" in e)
if not a["timer"] or "timers.target" not in a["wanted"]: bad.append("L5 no timer runs the log age job")
else:
    if a["timer"].get("OnCalendar") in ("daily", "*-*-* 00:00:00"): cadence = 1.0
    else: bad.append("L5 the age job is not daily: %r" % a["timer"])
    if str(a["timer"].get("Persistent", "")).lower() != "true": bad.append("L5 the age timer is not Persistent (a host down at the hour would skip a day)")
if a["type"] != "oneshot": bad.append("L5 the age job is not a oneshot")
if a["user"] != "root": bad.append("L5 the age job runs as %r and cannot rotate the journal" % a["user"])
try: limit = int(env.get("LOG_MAX_DAYS", ""))
except ValueError: limit = None
if limit is None: bad.append("L5 the age job has no LOG_MAX_DAYS: %r" % env)
if env.get("CADDY_LOG_DIR") != "/var/log/caddy": bad.append("L5 the age job does not name /var/log/caddy (the files left from before): %r" % env.get("CADDY_LOG_DIR"))
for f in ("/var/log/lastlog", "/var/lib/lastlog/lastlog2.db"):
    if f not in env.get("LOGIN_LAST_FILES", "").split(): bad.append("L5 the age job does not remove %s" % f)
if c["printLastLog"] is not False: bad.append("L5 sshd prints the last login (PrintLastLog %r) although the record is removed daily" % c["printLastLog"])
missed = cadence or 1.0   # one run that did not happen
# L2: journald deletes a closed file by its first entry (the oldest), and
# the job closes the open file every run.
j = c["journal"]
keep, span = secs(j.get("MaxRetentionSec", "")), secs(j.get("MaxFileSec", ""))
if keep is None or span is None: bad.append("L2 the journal has no MaxRetentionSec or no MaxFileSec: %r" % j)
elif cadence:
    bound("L2 journal", keep / DAY + cadence + missed, keep / DAY)
    if limit is not None and limit * DAY != keep: bad.append("L2 the age job trims to %d days but the journal limit is %.1f" % (limit, keep / DAY))
    if span > DAY: bad.append("L2 a journal file spans %.1f days" % (span / DAY))
cap = size(j.get("SystemMaxUse", ""))
if cap is None or (keep and cap < (keep / DAY) * 380 * (1 << 20)): bad.append("L3 the journal ceiling %r is under the limit at 380 MB a day (280 of services, 100 of access log)" % j.get("SystemMaxUse"))
# L4
for host, text in sorted(c["logs"].items()):
    if re.search(r"output\s+file", text): bad.append("L4 %s: the access log is written to a file" % host)
    if not re.search(r"^\s*output\s+stderr\s*$", text, re.M): bad.append("L4 %s: the access log does not go to the journal (output stderr): %r" % (host, text))
# L6: a daily file; maxage deletes old files by last write, so an entry is
# at most one file span older than that.
rt = c["rotateTimer"].get("OnCalendar")
if isinstance(rt, list): rt = rt[0] if len(rt) == 1 else None
rcad = {"hourly": 1 / 24.0, "daily": 1.0}.get(rt)
if rcad is None: bad.append("L6 logrotate runs %r, not hourly or daily" % rt); rcad = 1.0
for f in ("/var/log/btmp", "/var/log/wtmp"):
    r = c["rotate"].get(f) or {}
    if r.get("frequency") != "daily": bad.append("L6 %s is not in daily files (%r)" % (f, r.get("frequency"))); continue
    try: ma = int(r.get("maxage", "")); keepn = int(r.get("rotate", ""))
    except (TypeError, ValueError): bad.append("L6 %s has no maxage or rotate: %r" % (f, r)); continue
    lim = min(ma, keepn)
    bound("L6 %s" % f, 1 + lim + rcad + 1, lim)
    if r.get("minsize") not in (None, ""): bad.append("L6 %s is not rotated while under %s, so it has no time bound" % (f, r.get("minsize")))
    if r.get("notifempty") or not r.get("ifempty"): bad.append("L6 %s is not rotated while empty, so old files wait for a new record" % f)
# L8
for r in c["tmpfiles"]:
    if re.match(r"\S+\s+/var/log(/\S*)?\s", r) and re.search(r"\s\d+[dhw]\s*$", r): bad.append("L8 a tmpfiles age rule sweeps a log folder by last-write time: %s" % r)
t = os.environ.get("SWEEP_TEXT", "")
if t:
    if "--vacuum-size" in t: bad.append("L1 the disk sweep trims the journal by size")
    if limit is not None and ("--vacuum-time=%dd" % limit) not in t: bad.append("L1 the disk sweep does not trim the journal to %d days" % limit)
else: bad.append("L1 the disk sweep text could not be read from the evaluated configuration")
for b in bad: print("FAIL:", b)
sys.exit(1 if bad else 0)
'; then ok; else fail=$((fail + 1)); fi

echo "pass=$pass fail=$fail"
[ "$fail" = 0 ]
