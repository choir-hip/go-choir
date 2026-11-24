#!/bin/sh
# Prints the path of the explicit iOS development profile for the production id (S47GERGJZ3.news.choir.autoradio) that is installed on this Mac, unexpired, with
# Associated Domains: the profile AR_VARIANT=dev signs with (exact-env.sh sets EXACT_PROFILE to it). The team's wildcard profile cannot carry Associated Domains, so a build
# signed with it does not install. The profile is made through the App Store Connect API ("Autoradio 1 Development", docs/reference/exact/build-and-ship.md); this only finds it.
# EXACT_PROFILE_DIR, when set, is searched first. Exits 1 naming what is missing; prints nothing else.
set -eu
APPID="${DEV_PROFILE_APP_ID:-S47GERGJZ3.news.choir.autoradio}"
NOW="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
best=""; bestexp=""
for d in "${EXACT_PROFILE_DIR:-}" "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles" "$HOME/Library/MobileDevice/Provisioning Profiles"; do
  [ -n "$d" ] && [ -d "$d" ] || continue
  for f in "$d"/*.mobileprovision; do
    [ -f "$f" ] || continue
    x="$(/usr/bin/security cms -D -i "$f" 2>/dev/null)" || continue
    printf '%s' "$x" | /usr/bin/plutil -extract Entitlements.application-identifier raw - 2>/dev/null | grep -qx "$APPID" || continue
    printf '%s' "$x" | /usr/bin/plutil -extract Entitlements.get-task-allow raw - 2>/dev/null | grep -qx true || continue
    printf '%s' "$x" | /usr/bin/plutil -extract 'Entitlements.com\.apple\.developer\.associated-domains' raw - >/dev/null 2>&1 || continue
    exp="$(printf '%s' "$x" | /usr/bin/plutil -extract ExpirationDate raw - 2>/dev/null)" || continue
    [ "$exp" \> "$NOW" ] || continue
    if [ -z "$best" ] || [ "$exp" \> "$bestexp" ]; then best="$f"; bestexp="$exp"; fi
  done
done
[ -n "$best" ] || { echo "dev-profile: no unexpired iOS development profile for $APPID with Associated Domains is installed; make \"Autoradio 1 Development\" in the developer account (docs/reference/exact/build-and-ship.md) and install it in ~/Library/Developer/Xcode/UserData/Provisioning Profiles" >&2; exit 1; }
printf '%s\n' "$best"
