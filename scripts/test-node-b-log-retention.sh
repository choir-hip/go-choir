#!/usr/bin/env bash
# Host logs on Node B are kept at most 30 days, and not thrown away sooner by a
# size trim (docs/problems/host-logs-kept-a-day-or-for-ever-2026-11-24.md).
#
# Failure modes:
#  L1 the deploy preflight or the disk sweep trims the journal by size: at
#     about 250 MB a day a 256 MB trim keeps under a day, so the log from
#     before a deploy is gone (the owner, 2026-11-23: "We can't lose logs").
#  L2 the journal has no time bound, or one over 30 days: an entry may outlive
#     the period the privacy policy states. A journal file is deleted only
#     when every entry in it is past the bound, so the bound plus the longest
#     span of one file must be 30 days or less.
#  L3 the journal's size ceiling is small enough to cut the 30 days short at
#     today's volume (it must hold at least 30 days at 250 MB a day).
#  L4 a web access log (it holds visitors' addresses) rolls by size only: a
#     quiet host's file is never rolled and its entries never expire. Every
#     virtual host must roll daily and keep rolled files 29 days, with no
#     count limit that would delete sooner than the time limit.
#  L5 an access log file nobody writes any more (a retired host name) stays
#     for ever: the directory needs an age sweep.
#  L6 the failed and successful login records (btmp, wtmp: they hold remote
#     addresses) are kept past 30 days: monthly rotation with one kept file
#     and a minimum size reaches about 60 days, or for ever while small.
#
# L1 runs anywhere. L2 to L6 evaluate the host configuration (nothing is
# built) and are skipped when nix is not installed.
#
# Run: bash scripts/test-node-b-log-retention.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/.." || exit 1
pass=0; fail=0
ok() { pass=$((pass + 1)); }
bad() { fail=$((fail + 1)); echo "FAIL: $1"; }

# --- L1: what the preflight and the sweep ask journalctl to do, recorded by a stub.
root="$(mktemp -d)"; trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin"
cat >"$root/bin/journalctl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >>"$FAKE_DIR/journalctl.calls"
EOF
cat >"$root/bin/df" <<'EOF'
#!/usr/bin/env bash
# Low headroom, so the preflight takes its reclaim path (the only path that trims).
printf 'Avail\n%s\n' $((50 * 1024 * 1024))
EOF
for t in curl nix systemctl node-b-nix-retention; do printf '#!/usr/bin/env bash\nexit 0\n' >"$root/bin/$t"; done
chmod +x "$root/bin/"*
: >"$root/journalctl.calls"
env PATH="$root/bin:$PATH" FAKE_DIR="$root" PREFLIGHT_NIX_CONF="$root/nix.conf" \
  bash scripts/node-b-deploy-disk-preflight >"$root/out" 2>&1
if grep -q -- '--vacuum-size' "$root/journalctl.calls"; then bad "L1 the deploy preflight trims the journal by size: $(cat "$root/journalctl.calls")"; else ok; fi
if grep -q -- '--vacuum-time=29d' "$root/journalctl.calls"; then ok; else bad "L1 the deploy preflight does not trim the journal to 29 days: $(cat "$root/journalctl.calls")"; fi

if ! command -v nix >/dev/null 2>&1; then echo "skip: nix is not installed (L2 to L6 not checked)"; echo "pass=$pass fail=$fail"; [ "$fail" = 0 ]; exit; fi
if ! command -v python3 >/dev/null 2>&1; then echo "skip: python3 is not installed"; exit 0; fi

json="$(nix eval --json '.#nixosConfigurations.go-choir-b.config' --apply 'c: {
  journal = c.services.journald.settings.Journal;
  logs = builtins.mapAttrs (n: v: v.logFormat) c.services.caddy.virtualHosts;
  tmpfiles = c.systemd.tmpfiles.rules;
  rotate = c.services.logrotate.settings;
  sweep = c.systemd.services.go-choir-disk-gc.serviceConfig.ExecStart;
}' 2>"$root/eval.err")" || { echo "FAIL: nix eval: $(tail -3 "$root/eval.err")"; exit 1; }
sweep="$(printf '%s' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["sweep"])')"
# The sweep script is a store path only a build realises; its text is the nix
# string, read here from the evaluated derivation's text attribute.
sweep_text="$(nix eval --raw '.#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-disk-gc.serviceConfig.ExecStart.text' 2>/dev/null || true)"

printf '%s' "$json" | SWEEP_TEXT="$sweep_text" python3 -c '
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
j = c["journal"]
keep, span = secs(j.get("MaxRetentionSec", "")), secs(j.get("MaxFileSec", ""))
if keep is None or span is None: bad.append("L2 the journal has no MaxRetentionSec or no MaxFileSec: %r" % j)
elif keep + span > 30 * DAY: bad.append("L2 a journal entry may live %d days" % ((keep + span) // DAY))
elif keep < 28 * DAY: bad.append("L2 the journal keeps only %d days" % (keep // DAY))
cap = size(j.get("SystemMaxUse", ""))
if cap is None or cap < 30 * 250 * (1 << 20): bad.append("L3 the journal ceiling %r is under 30 days at 250 MB a day" % j.get("SystemMaxUse"))
for host, text in sorted(c["logs"].items()):
    opt = dict(re.findall(r"^\s*(roll_\w+)\s+(\S+)\s*$", text, re.M))
    if "roll_disabled" in text: bad.append("L4 %s: rolling is disabled" % host)
    iv, kf = opt.get("roll_interval"), opt.get("roll_keep_for")
    ivs = secs(iv) if iv else None
    kfs = secs(kf) if kf else None
    if ivs is None or ivs > DAY: bad.append("L4 %s: no daily roll (roll_interval %r)" % (host, iv))
    if kfs is None: bad.append("L4 %s: no roll_keep_for" % host)
    elif ivs and kfs + ivs > 30 * DAY: bad.append("L4 %s: an entry may live %d days" % (host, (kfs + ivs) // DAY))
    if opt.get("roll_keep") != "-1": bad.append("L4 %s: a count limit (roll_keep %r) can delete before the time limit, or the default of 10 applies" % (host, opt.get("roll_keep")))
    if not re.search(r"output file /var/log/caddy/", text): bad.append("L4 %s: the access log is not under /var/log/caddy" % host)
if not any(re.fullmatch(r"e\s+/var/log/caddy\s+-\s+-\s+-\s+30d", r.strip()) for r in c["tmpfiles"]):
    bad.append("L5 no age sweep of /var/log/caddy at 30 days in systemd.tmpfiles.rules")
for f in ("/var/log/btmp", "/var/log/wtmp"):
    r = c["rotate"].get(f) or {}
    per = {"daily": 1, "weekly": 7, "monthly": 31}.get(r.get("frequency"))
    if per is None or (int(r.get("rotate", 0)) + 1) * per > 30: bad.append("L6 %s is kept up to %s days (%s, rotate %s)" % (f, "?" if per is None else (int(r.get("rotate", 0)) + 1) * per, r.get("frequency"), r.get("rotate")))
    if r.get("minsize") not in (None, ""): bad.append("L6 %s is not rotated while under %s, so it has no time bound" % (f, r.get("minsize")))
    if str(r.get("maxage", "")) not in ("30", "29", "28"): bad.append("L6 %s has no maxage" % f)
t = os.environ.get("SWEEP_TEXT", "")
if t:
    if "--vacuum-size" in t: bad.append("L1 the disk sweep trims the journal by size")
    if "--vacuum-time=29d" not in t: bad.append("L1 the disk sweep does not trim the journal to 29 days")
else: bad.append("L1 the disk sweep text could not be read from the evaluated configuration")
for b in bad: print("FAIL:", b)
sys.exit(1 if bad else 0)
' && ok || fail=$((fail + 1))

echo "pass=$pass fail=$fail"
[ "$fail" = 0 ]
