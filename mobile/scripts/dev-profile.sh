#!/bin/sh
# dev-profile.sh [--phone <udid>] [--check <profile>]
# Prints the path of the explicit iOS development profile for the production id (S47GERGJZ3.news.choir.autoradio) that is installed on this Mac: the profile AR_VARIANT=dev signs
# with (exact-env.sh sets EXACT_PROFILE to it, for an iOS device build only). The team's wildcard profile cannot carry Associated Domains, so a build signed with it does not
# install. The profile is made through the App Store Connect API ("Autoradio 1 Development", docs/reference/exact/build-and-ship.md); this only finds it.
# A profile qualifies only if ALL hold (each is read from the decoded plist, nothing is matched as a pattern):
#   Platform includes iOS; Entitlements.application-identifier is EXACTLY the explicit id (never a wildcard); get-task-allow is true (a development profile);
#   com.apple.developer.associated-domains is present and covers webcredentials ("*" or a webcredentials:<host> entry; absent, false or empty never qualifies);
#   ExpirationDate is in the future; and, when --phone names a device, ProvisionedDevices lists it.
# The newest expiry wins (ties: the path that sorts first), so the choice is the same on every run. --check <profile> applies the same tests to that one file (a profile the caller
# named) and prints it or refuses it. Searched: $EXACT_PROFILE_DIR, then Xcode's two profile folders. Exits 1 naming what is missing; prints nothing else on stdout.
# DEV_PROFILE_DECODER (default /usr/bin/security, run as `security cms -D -i <file>`) and DEV_PROFILE_APP_ID exist so a test can use fixture plists and another id.
# --phone must be the device's UDID (a name is not a ProvisionedDevices entry and is refused as "not in the profile").
set -eu
exec /usr/bin/python3 -I - "$@" <<'PY'
import datetime, glob, os, plistlib, subprocess, sys
APPID = os.environ.get("DEV_PROFILE_APP_ID") or "S47GERGJZ3.news.choir.autoradio"
DECODER = os.environ.get("DEV_PROFILE_DECODER") or ""
def die(m): sys.stderr.write("dev-profile: " + m + "\n"); sys.exit(1)
phone, check, a = "", "", sys.argv[1:]
while a:
    x = a.pop(0)
    if x == "--phone" and a: phone = a.pop(0)
    elif x.startswith("--phone="): phone = x[8:]
    elif x == "--check" and a: check = a.pop(0)
    else: die("usage: dev-profile.sh [--phone <udid>] [--check <profile>]")
_c = {}
def decode(f):
    if f not in _c: _c[f] = _decode(f)
    return _c[f]
def _decode(f):
    try:
        cmd = [DECODER, f] if DECODER else ["/usr/bin/security", "cms", "-D", "-i", f]
        return plistlib.loads(subprocess.run(cmd, capture_output=True, check=True, timeout=30, stdin=subprocess.DEVNULL).stdout)
    except Exception: return None
def why_not(f):
    """None when the profile qualifies, else the first test it fails."""
    if not os.path.isfile(f): return "is not a file"
    p = decode(f)
    if not isinstance(p, dict): return "is not a readable provisioning profile"
    e = p.get("Entitlements"); e = e if isinstance(e, dict) else {}
    if "iOS" not in (p.get("Platform") or []): return "is not an iOS profile"
    if e.get("application-identifier") != APPID: return f"is for {e.get('application-identifier')!r}, not exactly {APPID} (a wildcard profile cannot carry Associated Domains)"
    if e.get("get-task-allow") is not True: return "is not a development profile (get-task-allow is not true)"
    d = e.get("com.apple.developer.associated-domains")
    d = [d] if isinstance(d, str) else d
    if not isinstance(d, list) or not any(isinstance(i, str) and (i == "*" or i.startswith("webcredentials:")) for i in d): return "does not carry the Associated Domains entitlement for webcredentials"
    x = p.get("ExpirationDate")
    if not isinstance(x, datetime.datetime) or x.replace(tzinfo=None) <= datetime.datetime.utcnow(): return "is expired"
    if phone and phone not in (p.get("ProvisionedDevices") or []): return f"does not include the device {phone}"
    return None
def expiry(f): return decode(f)["ExpirationDate"].replace(tzinfo=None)
if check:
    w = why_not(check)
    if w: die(f"{check} {w}")
    print(check); sys.exit(0)
home = os.path.expanduser("~")
best = None
for d in (os.environ.get("EXACT_PROFILE_DIR", ""), home + "/Library/Developer/Xcode/UserData/Provisioning Profiles", home + "/Library/MobileDevice/Provisioning Profiles"):
    if not d or not os.path.isdir(d): continue
    for f in sorted(glob.glob(d + "/*.mobileprovision")):
        if why_not(f) is None:
            k = expiry(f)
            if best is None or k > best[0]: best = (k, f)      # sorted order + strict ">" : a tie keeps the path that sorts first
if best is None:
    die(f"no unexpired iOS development profile for {APPID} with Associated Domains" + (f" and the device {phone}" if phone else "") + " is installed; make \"Autoradio 1 Development\" in the developer account (docs/reference/exact/build-and-ship.md) and install it in ~/Library/Developer/Xcode/UserData/Provisioning Profiles")
print(best[1])
PY
