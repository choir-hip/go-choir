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
# Still low after the last resort too: the deploy is refused, and says the journal was trimmed.
preflight "50 50 50"
if [ "$(cat "$root/rc")" != 0 ] && grep -q "LAST RESORT" "$root/out"; then ok; else bad "L7 a disk still low after the last resort did not refuse the deploy (rc $(cat "$root/rc"))"; fi
# Enough room from the start: journalctl is not called at all.
preflight "200"
if [ ! -s "$root/journalctl.calls" ]; then ok; else bad "L1 the journal is trimmed on a deploy with enough room"; fi

# A missing tool is a failure, never a pass.
command -v nix >/dev/null 2>&1 || { echo "FAIL: nix is not installed (L2 to L6 cannot be checked)"; echo "pass=$pass fail=$((fail + 1))"; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 is not installed"; echo "pass=$pass fail=$((fail + 1))"; exit 1; }

# RENDERED artifacts, not option lists: the unit and timer files, the
# logrotate configuration and journald.conf as NixOS writes them. A setting
# that reads right as an option and renders wrong (a boolean that becomes an
# unknown word, a list systemd cuts at the first space) must fail here.
json="$(nix eval --json '.#nixosConfigurations.go-choir-b.config' --apply 'c: {
  journald = c.environment.etc."systemd/journald.conf".text;
  logrotate = c.environment.etc."logrotate.conf".source.text;
  logrotateEnabled = c.services.logrotate.enable;
  ageUnit = c.systemd.units."go-choir-log-age.service".text;
  ageTimer = c.systemd.units."go-choir-log-age.timer".text;
  ageTimerWanted = c.systemd.timers.go-choir-log-age.wantedBy or [];
  rotateTimer = c.systemd.units."logrotate.timer".text;
  caddyUnit = c.systemd.units."caddy.service".text;
  printLastLog = c.services.openssh.settings.PrintLastLog or null;
  logs = builtins.mapAttrs (n: v: v.logFormat + "\n" + v.extraConfig) c.services.caddy.virtualHosts;
  caddyGlobal = c.services.caddy.globalConfig + "\n" + c.services.caddy.extraConfig;
  tmpfiles = c.systemd.tmpfiles.rules;
}' 2>"$root/eval.err")" || { echo "FAIL: nix eval: $(tail -3 "$root/eval.err")"; exit 1; }
# The sweep script is a store path only a build realises; its text is read
# from the evaluated derivation.
sweep_text="$(nix eval --raw '.#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-disk-gc.serviceConfig.ExecStart.text' 2>/dev/null || true)"

if printf '%s' "$json" | SWEEP_TEXT="$sweep_text" python3 -c '
import json, os, re, shlex, sys
c = json.load(sys.stdin); bad = []
def secs(v):
    m = re.fullmatch(r"(\d+)\s*(s|sec|m|min|h|hour|d|day|days|w|week)?", str(v).strip())
    if not m: return None
    return int(m.group(1)) * {None:1,"s":1,"sec":1,"m":60,"min":60,"h":3600,"hour":3600,"d":86400,"day":86400,"days":86400,"w":604800,"week":604800}[m.group(2)]
def size(v):
    m = re.fullmatch(r"(\d+)([KMGT])?", str(v).strip())
    return int(m.group(1)) * {None:1,"K":1<<10,"M":1<<20,"G":1<<30,"T":1<<40}[m.group(2)] if m else None
def ini(text, section):
    out, cur = {}, None
    for ln in text.splitlines():
        ln = ln.strip()
        if ln.startswith("["): cur = ln.strip("[]"); continue
        if cur == section and "=" in ln and not ln.startswith("#"):
            k, v = ln.split("=", 1); out.setdefault(k.strip(), []).append(v.strip())
    return out
def unit_env(text):
    # As systemd reads Environment=: words split on spaces unless quoted;
    # a word with no "=" is dropped (so a raw list keeps its first path only).
    env, dropped = {}, []
    for v in ini(text, "Service").get("Environment", []):
        for w in shlex.split(v):
            if "=" in w: k, val = w.split("=", 1); env[k] = val
            else: dropped.append(w)
    return env, dropped
DAY = 86400; BOUND = 30; MARGIN = 5; FLOOR = 7
def bound(name, worst_days, limit_days):
    if worst_days > BOUND: bad.append("%s: an entry may live %.1f days, over %d" % (name, worst_days, BOUND))
    elif BOUND - worst_days < MARGIN: bad.append("%s: worst case %.1f days leaves under %d days of margin" % (name, worst_days, MARGIN))
    if limit_days < FLOOR: bad.append("%s: the limit of %.1f days is under our %d-day operating floor" % (name, limit_days, FLOOR))
def timer(name, text, cal):
    t = ini(text, "Timer")
    if t.get("OnCalendar") != [cal]: bad.append("%s: the timer is %r, not %s" % (name, t.get("OnCalendar"), cal))
    if [v.lower() for v in t.get("Persistent", [])] != ["true"]: bad.append("%s: the timer is not Persistent (a host down at the mark would skip a run)" % name)
    boot = secs(t.get("OnBootSec", [""])[0]) if t.get("OnBootSec") else None
    if boot is None or boot > 600: bad.append("%s: no run within ten minutes of a boot (OnBootSec %r): a host that was off keeps old entries until the next mark" % (name, t.get("OnBootSec")))
    if secs(t.get("RandomizedDelaySec", ["0"])[0]) not in (0, None) and secs(t.get("RandomizedDelaySec", ["0"])[0]) > 600: bad.append("%s: a random delay over ten minutes" % name)
# L5: the age job, as rendered.
env, dropped = unit_env(c["ageUnit"])
if dropped: bad.append("L5 systemd would drop these words of the age job Environment (a list not quoted): %r" % dropped[:4])
timer("L5 age job", c["ageTimer"], "daily")
if "timers.target" not in c["ageTimerWanted"]: bad.append("L5 nothing starts the age timer")
svc = ini(c["ageUnit"], "Service")
if svc.get("Type") != ["oneshot"]: bad.append("L5 the age job is not a oneshot")
if svc.get("User", ["root"]) != ["root"]: bad.append("L5 the age job runs as %r and cannot rotate the journal" % svc.get("User"))
try: limit = int(env.get("LOG_MAX_DAYS", ""))
except ValueError: limit = None
if limit is None: bad.append("L5 the age job has no LOG_MAX_DAYS")
if env.get("JOURNAL_DIR") != "/var/log/journal": bad.append("L5 the age job is not told where the journal is: %r" % env.get("JOURNAL_DIR"))
if env.get("CADDY_LOG_DIR") != "/var/log/caddy": bad.append("L5 the age job does not name /var/log/caddy (the files left from before): %r" % env.get("CADDY_LOG_DIR"))
if env.get("LOGIN_RECORD_FILES", "").split() != ["/var/log/btmp", "/var/log/wtmp"]: bad.append("L5 the age job, as systemd reads it, does not check /var/log/btmp and /var/log/wtmp: %r" % env.get("LOGIN_RECORD_FILES"))
if env.get("LOGROTATE_STATUS") != "/var/lib/logrotate.status": bad.append("L5 the age job is not told where logrotate records its rotations: %r" % env.get("LOGROTATE_STATUS"))
u = ini(c["ageUnit"], "Unit")
for k in ("After", "Wants"):
    if "logrotate.service" not in " ".join(u.get(k, [])).split(): bad.append("L5 the age job does not run logrotate first (%s=logrotate.service): it would judge a stale rotation record" % k)
# The defaults in the job are what the dry run lists for the owner: they must be the values of the unit.
job = open("scripts/node-b-log-age").read()
defaults = dict(re.findall(r"^([A-Z_]+)=\"\$\{\1-([^}]*)\}\"$", job, re.M))
for k in ("LOG_MAX_DAYS", "JOURNAL_DIR", "CADDY_LOG_DIR", "LOGIN_RECORD_FILES", "LOGROTATE_STATUS", "LOGIN_LAST_FILES", "DEAD_LOG_FILES"):
    if defaults.get(k) is None or defaults.get(k) != env.get(k): bad.append("L5 %s: the default in the job %r is not the value in the unit %r, so a dry run would not list what the deploy deletes" % (k, defaults.get(k), env.get(k)))
if "LOG_AGE_DRY_RUN" in env: bad.append("L5 the unit sets LOG_AGE_DRY_RUN: the daily job would delete nothing")
last = env.get("LOGIN_LAST_FILES", "").split()
for f in ("/var/log/lastlog", "/var/log/lastlog.migrated", "/var/lib/lastlog/lastlog2.db", "/var/lib/lastlog/lastlog2.db-wal", "/var/lib/lastlog/lastlog2.db-shm"):
    if f not in last: bad.append("L5 the age job, as systemd reads it, does not remove %s" % f)
dead = env.get("DEAD_LOG_FILES", "").split()
if len(dead) != 3 or any(not d.startswith("/var/lib/go-choir/") or not d.endswith(".log") or d.count("/") != 4 for d in dead): bad.append("L5 the dead-file list, as systemd reads it, is not the three named files directly under /var/lib/go-choir: %r" % dead)
# (sshd_config is a built file whose text cannot be read by evaluation: this one is the option.)
if c["printLastLog"] is not False: bad.append("L5 sshd prints the last login (PrintLastLog %r) although the record is removed every run" % c["printLastLog"])
cadence, missed = 1.0, 1.0
# L2: journald.conf as written. One file spans MaxFileSec at most, a closed
# file is deleted by its first entry, and the job closes the open file every
# run; the span is added anyway.
j = {k: v[-1] for k, v in ini(c["journald"], "Journal").items()}
keep, span = secs(j.get("MaxRetentionSec", "")), secs(j.get("MaxFileSec", ""))
if keep is None or span is None: bad.append("L2 journald.conf has no MaxRetentionSec or no MaxFileSec: %r" % j)
else:
    bound("L2 journal", span / DAY + keep / DAY + cadence + missed, keep / DAY)
    if limit is not None and limit * DAY != keep: bad.append("L2 the age job trims to %d days but the journal limit is %.1f" % (limit, keep / DAY))
    if span > DAY: bad.append("L2 a journal file spans %.1f days" % (span / DAY))
cap = size(j.get("SystemMaxUse", ""))
if cap is None or (keep and cap < (keep / DAY) * 380 * (1 << 20)): bad.append("L3 the journal ceiling %r is under the limit at 380 MB a day (280 of services, 100 of access log)" % j.get("SystemMaxUse"))
# L4
for host, text in sorted(c["logs"].items()):
    for out in re.findall(r"^\s*output\s+(\S+)", text, re.M):
        if out != "stderr": bad.append("L4 %s: a log goes to %r (a file, a network address): only the journal has a bound" % (host, out))
    if not re.search(r"^\s*output\s+stderr\s*$", text, re.M): bad.append("L4 %s: the access log does not go to the journal (output stderr)" % host)
for out in re.findall(r"^\s*output\s+(\S+)", c["caddyGlobal"], re.M):
    if out not in ("stderr", "discard"): bad.append("L4 Caddy global or extra configuration sends a log to %r: only the journal has a bound" % out)
if ini(c["caddyUnit"], "Service").get("LogRateLimitIntervalSec", [""])[-1] not in ("0", "0s"): bad.append("L4 journald may drop access lines under load: caddy.service has no LogRateLimitIntervalSec=0")
# L6: logrotate.conf as written.
if not c["logrotateEnabled"]: bad.append("L6 logrotate is disabled")
timer("L6 logrotate", c["rotateTimer"], "hourly")
KNOWN = set("compress nocompress copy nocopy copytruncate nocopytruncate create nocreate createolddir nocreateolddir daily weekly monthly yearly hourly dateext nodateext delaycompress nodelaycompress ifempty notifempty mail nomail missingok nomissingok olddir noolddir sharedscripts nosharedscripts shred noshred rotate maxage minage minsize maxsize size start su tabooext taboopat include dateformat dateyesterday datehourago extension addextension compresscmd uncompresscmd compressext compressoptions mailfirst maillast firstaction lastaction prerotate postrotate preremove endscript renamecopy norenamecopy shredcycles allowhardlink noallowhardlink ignoreduplicates".split())
blocks, cur = {}, None
for ln in c["logrotate"].splitlines():
    w = ln.split()
    if not w: continue
    if ln.rstrip().endswith("{"): cur = ln.rsplit("{", 1)[0].strip().strip("\""); blocks[cur] = {}; continue
    if w[0] == "}": cur = None; continue
    if w[0] not in KNOWN: bad.append("L6 logrotate.conf has a word logrotate does not know: %r (it stops every rotation)" % ln.strip())
    if cur is not None:
        blocks[cur][w[0]] = " ".join(w[1:])
        if w[0] in ("olddir", "dateformat", "extension", "addextension", "compressext") and ("btmp" in cur or "wtmp" in cur): bad.append("L6 %s has %r: its rotated files would be named or placed where the age job does not look" % (cur, w[0]))
    elif w[0] in ("dateext", "olddir", "compress"): bad.append("L6 logrotate.conf has a global %r: rotated login files would be named or placed where the age job was not tested" % w[0])
for f in ("/var/log/btmp", "/var/log/wtmp"):
    r = blocks.get(f)
    if r is None: bad.append("L6 logrotate.conf has no block for %s" % f); continue
    if "daily" not in r: bad.append("L6 %s is not in daily files" % f); continue
    try: lim = min(int(r["maxage"]), int(r["rotate"]))
    except (KeyError, ValueError): bad.append("L6 %s has no maxage or rotate: %r" % (f, r)); continue
    bound("L6 %s" % f, 1 + lim + 1 / 24.0 + 1, lim)
    if "minsize" in r or "size" in r: bad.append("L6 %s is not rotated while small, so it has no time bound" % f)
    if "ifempty" not in r or "notifempty" in r: bad.append("L6 %s is not rotated while empty, so old files wait for a new record" % f)
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
