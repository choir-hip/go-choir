#!/usr/bin/env bash
# The boot-time prune unit (go-choir-updater-prune in nix/autoputer-vm.nix) under
# the real NixOS module system: the guest configuration is evaluated (nothing is
# built) and the unit's ordering and failure properties are asserted. Skipped
# when nix is not installed.
#
# Failure modes (docs/problems/updater-prune-three-rounds-structural-assessment-2026-11-14.md):
#  P1 the prune is not ordered before the updater daemon, or before the runtime:
#     it would then run beside a live restore or swap, the defect that sank the
#     earlier candidates.
#  P2 the updater or the runtime requires, binds to or is part of the prune, or
#     the prune is required by anything: a slow, failed or killed prune would
#     then fail or delay-forever a boot. Only ordering (before/after) and a want
#     from multi-user.target are allowed.
#  P3 the unit is not a oneshot that ends, or has no start timeout, or its
#     timeout is not longer than the budget it passes the binary, or it restarts
#     itself.
#  P4 it does not run the binary's prune mode on the updater root, or it can
#     write outside the updater root.
#  P5 it is not started at boot (nothing wants it).
#  P6 it can run again while the daemon and the runtime are live (a later
#     `systemctl start multi-user.target` or a manual start), beside a live
#     restore with store/ mounted as an overlay upperdir: it must stay active
#     after its boot run (RemainAfterExit), refuse a manual start, and carry a
#     condition on the daemon's own started marker.
#  P7 its mount ordering or its privileges drift: it needs /mnt/persistent
#     (RequiresMountsFor; the real chain is the fstab-generator mount unit that
#     microvm.nix declares for the data volume, mnt-persistent.mount, then
#     local-fs.target, then basic.target, then this unit; the mount is not in
#     config.systemd.units, so only the RequiresMountsFor line is assertable
#     here), runs as root with exactly CAP_DAC_OVERRIDE and CAP_FOWNER.
#
# Run: bash scripts/updater-prune-unit-nix-test.sh
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
cd "$HERE/.." || exit 1

if ! command -v nix >/dev/null 2>&1; then echo "skip: nix is not installed"; exit 0; fi
if ! command -v python3 >/dev/null 2>&1; then echo "skip: python3 is not installed"; exit 0; fi

json="$(nix eval --json '.#nixosConfigurations.go-choir-autoputer-vm.config.systemd.services' --apply '
  svc:
  let
    names = builtins.attrNames svc;
    get = n: a: let v = svc.${n}.${a} or []; in if builtins.isList v then v else [ v ];
    rel = a: builtins.listToAttrs (map (n: { name = n; value = get n a; }) names);
    prune = svc.go-choir-updater-prune;
  in {
    before = get "go-choir-updater-prune" "before";
    wantedBy = get "go-choir-updater-prune" "wantedBy";
    requiredBy = get "go-choir-updater-prune" "requiredBy";
    requires = rel "requires";
    bindsTo = rel "bindsTo";
    partOf = rel "partOf";
    upholds = rel "upholds";
    wants = rel "wants";
    serviceRequiredBy = rel "requiredBy";
    updaterAfter = get "go-choir-updater" "after";
    sc = prune.serviceConfig;
    unit = prune.unitConfig;
    autoputerPre = svc.go-choir-autoputer.serviceConfig.ExecStartPre or "";
  }' 2>"${TMPDIR:-/tmp}/updater-prune-nix.err")" || { echo "FAIL: nix eval:"; cat "${TMPDIR:-/tmp}/updater-prune-nix.err"; exit 1; }

printf '%s' "$json" | python3 -c '
import json, sys
d = json.load(sys.stdin)
P = "go-choir-updater-prune"
bad = []
def has(lst): return any(x in (P, P + ".service") for x in lst)
# P1
for u in ("go-choir-updater.service", "go-choir-autoputer.service"):
    if u not in d["before"]:
        bad.append("P1 not ordered before " + u)
# P2
for rel in ("requires", "bindsTo", "partOf", "upholds"):
    for name, lst in d[rel].items():
        if has(lst):
            bad.append("P2 %s lists the prune in %s" % (name, rel))
if d["requiredBy"]:
    bad.append("P2 the prune is requiredBy %s" % d["requiredBy"])
for name, lst in d["serviceRequiredBy"].items():
    if name != P and has(lst):
        bad.append("P2 %s is requiredBy the prune" % name)
for name in ("go-choir-updater", "go-choir-autoputer"):
    if has(d["wants"].get(name, [])):
        bad.append("P2 %s wants the prune (a want is harmless, but the prune is not theirs to pull in)" % name)
# P3
sc = d["sc"]
if sc.get("Type") != "oneshot": bad.append("P3 Type is %r" % sc.get("Type"))
if sc.get("RemainAfterExit") is not True: bad.append("P6 RemainAfterExit is %r: a re-activation would run the prune again" % sc.get("RemainAfterExit"))
unit = d["unit"]
if unit.get("RefuseManualStart") not in (True, "yes", "true"): bad.append("P6 RefuseManualStart is %r" % unit.get("RefuseManualStart"))
conds = unit.get("ConditionPathExists", [])
conds = conds if isinstance(conds, list) else [conds]
for want in ("!/run/choir/updater-unit-entered", "!/run/choir/updater.sock"):
    if want not in conds: bad.append("P6 no ConditionPathExists=%s (have %r)" % (want, conds))
if unit.get("RequiresMountsFor") != "/mnt/persistent": bad.append("P7 RequiresMountsFor is %r" % unit.get("RequiresMountsFor"))
if sc.get("NoNewPrivileges") is not True: bad.append("P7 NoNewPrivileges is %r" % sc.get("NoNewPrivileges"))
pre = d["autoputerPre"]
pre = " ".join(pre) if isinstance(pre, list) else pre
if "/run/choir/updater-unit-entered" not in pre: bad.append("P6 the runtime does not mark itself entered (ExecStartPre %r): a failed updater unit would leave the runtime live with no marker" % pre)
if sc.get("User") != "root": bad.append("P7 User is %r" % sc.get("User"))
if sorted(sc.get("CapabilityBoundingSet", [])) != ["CAP_DAC_OVERRIDE", "CAP_FOWNER"]: bad.append("P7 CapabilityBoundingSet is %r" % sc.get("CapabilityBoundingSet"))
if sc.get("Restart", "no") != "no": bad.append("P3 Restart is %r" % sc.get("Restart"))
timeout = sc.get("TimeoutStartSec", "")
def secs(v):
    v = str(v).strip()
    if v.endswith("min"): return float(v[:-3]) * 60
    if v.endswith("s"): return float(v[:-1])
    return float(v)
exe = sc.get("ExecStart", "")
if isinstance(exe, list): exe = " ".join(exe)
try:
    t = secs(timeout)
except Exception:
    t = None; bad.append("P3 TimeoutStartSec is %r" % timeout)
# P4
if " prune --root /mnt/persistent/choir-updater " not in exe + " ": bad.append("P4 ExecStart is %r" % exe)
rw = sc.get("ReadWritePaths", [])
if rw != ["/mnt/persistent/choir-updater"]: bad.append("P4 ReadWritePaths is %r" % rw)
if sc.get("ProtectSystem") != "strict": bad.append("P4 ProtectSystem is %r" % sc.get("ProtectSystem"))
if sc.get("PrivateNetwork") is not True: bad.append("P4 the prune has a network")
if "--budget" not in exe: bad.append("P3 no budget passed")
else:
    b = secs(exe.split("--budget")[1].split()[0])
    if t is not None and not t > b: bad.append("P3 TimeoutStartSec %s is not longer than the budget %s" % (t, b))
# P5
if "multi-user.target" not in d["wantedBy"]: bad.append("P5 nothing wants the prune at boot: wantedBy %s" % d["wantedBy"])
if bad:
    print("FAIL"); [print("  " + b) for b in bad]; sys.exit(1)
print("ok: go-choir-updater-prune is ordered before the updater and the runtime, required by neither, a bounded oneshot")
'
