# Sourced, never run (no shebang, not executable): exact_run <Exact's arguments> runs `bun exact.mjs <arguments>` for AutoradioLab in the environment this app's builds need, and
# returns its status. The two callers are exact.sh (after exact-guard.sh) and archive-store.sh (the only caller that passes --archive; it checks everything itself, see there).
# Needs HERE (the mobile directory).
#
# The app is the production app unless AR_VARIANT / AR_APP_ID chose a development app (app-identity.sh); Exact reads the choice as EXACT_VARIANT, which `env -i` would drop.
# The app's id (app.ts APP_ID, named to the server's passkey finish route): given to Exact explicitly for every identity, so the default never rests on a literal
# fallback and an empty value in the caller's environment cannot make a dead grant (app.ts reads it with `??`; the build refuses `||`).
# What `env -i` would drop and Exact reads from its environment (build-and-ship.md, "Archive, TestFlight, CI"): the signing pair, the version and build number and the
# export-compliance answer: passed only when set, so a development build is exactly as it was. EXACT_DEV_MENU=0 is given to every build: Exact's developer menu is off in every
# build of this app, so nothing in a build reads a caller's value.
exact_run() {
  EXACT2="$HERE/.exact-src" "$HERE/scripts/check-exact-pin.sh" || return 1      # the wrong Exact must never build quietly (see exact.pin)
  _variant="$("$HERE/scripts/app-identity.sh" variant)" || return 1
  _scheme="$("$HERE/scripts/app-identity.sh" id)" || return 1
  if [ "$_variant" = dev ]; then      # the production id as a development app: Exact is given no variant
    _variant=""
    . "$HERE/scripts/exact-args.sh"
    _phone=""; _p=""; for _a in "$@"; do [ "$_p" != 1 ] || _phone="$_a"; _p=""; case "$_a" in --phone) _p=1 ;; --phone=*) _phone="${_a#--phone=}" ;; esac; done
    _ios=""; case "${1:-}:${2:-}" in ios:*|agent:ios|test:ios) _ios=1 ;; esac
    if [ -n "$_ios" ] && { ea_has --device "$@" || ea_has --phone "$@"; }; then
      # an iOS device build is signed with the explicit development profile, never the team's wildcard (it cannot carry Associated Domains): found, or the caller's own, checked the same way
      if [ -n "${EXACT_PROFILE:-}" ]; then "$HERE/scripts/dev-profile.sh" ${_phone:+--phone "$_phone"} --check "$EXACT_PROFILE" >/dev/null || return 1
      else EXACT_PROFILE="$("$HERE/scripts/dev-profile.sh" ${_phone:+--phone "$_phone"})" || return 1; fi
      export EXACT_PROFILE
    else unset EXACT_PROFILE      # nothing else of a dev build is signed with a provisioning profile: a caller's stale one (a store or lab profile) is not handed on
    fi
  fi
  _android=""; case "${1:-}:${2:-}" in android:*|agent:android|test:android|verify:android) _android=1 ;; esac
  _path="$(dirname "${BUN:-$HOME/.bun-1.4.2/bin/bun}"):$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"
  if [ -n "$_android" ]; then
    # env -i below deliberately drops the caller's shell. Give Exact's Android host the SDK, JDK and selected device explicitly.
    if [ -z "${JAVA_HOME:-}" ] && [ -x /opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home/bin/java ]; then
      JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home
    fi
    if [ -z "${ANDROID_HOME:-}" ] && [ -d "$HOME/Library/Android/sdk" ]; then ANDROID_HOME="$HOME/Library/Android/sdk"; fi
    if [ -z "${EXACT_VERSION+x}" ]; then EXACT_VERSION="$("$HERE/scripts/app-identity.sh" version)" || return 1; fi
    if [ -z "${EXACT_BUILD_NUMBER+x}" ]; then
      EXACT_BUILD_NUMBER="$(/usr/bin/python3 -I -c 'import datetime; n=datetime.datetime.now(datetime.timezone.utc)-datetime.datetime(2026,1,1,tzinfo=datetime.timezone.utc); print(int(n.total_seconds()//60))')" || return 1
    fi
    [ -z "${ANDROID_HOME:-}" ] || _path="$ANDROID_HOME/platform-tools:$_path"
  fi
  _bun="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
  set -- "$_bun" exact.mjs "$@"
  _n=$#
  for _v in EXACT_IDENTITY EXACT_PROFILE EXACT_VERSION EXACT_BUILD_NUMBER EXACT_NON_EXEMPT_ENCRYPTION; do
    eval "_isset=\${$_v+set}"
    if [ -n "$_isset" ]; then eval "_val=\$$_v"; set -- "$@" "$_v=$_val"; fi       # a value may hold spaces (a profile's path): one argument each
  done
  if [ -n "$_android" ]; then
    [ -z "${JAVA_HOME:-}" ] || set -- "$@" "JAVA_HOME=$JAVA_HOME"
    [ -z "${ANDROID_HOME:-}" ] || set -- "$@" "ANDROID_HOME=$ANDROID_HOME"
    [ -z "${ANDROID_SERIAL:-}" ] || set -- "$@" "ANDROID_SERIAL=$ANDROID_SERIAL"
    [ -z "${ANDROID_NDK_HOME:-}" ] || set -- "$@" "ANDROID_NDK_HOME=$ANDROID_NDK_HOME"
  fi
  [ -z "${EXACT_SIM:-}" ] || set -- "$@" "EXACT_SIM=$EXACT_SIM"            # one argument each, whatever the value holds (a simulator name has spaces)
  [ -z "$_variant" ] || set -- "$@" "EXACT_VARIANT=$_variant"
  while [ "$_n" -gt 0 ]; do _a="$1"; shift; set -- "$@" "$_a"; _n=$((_n - 1)); done   # the assignments first, then the command
  "$HERE/scripts/gen-build-info.sh" || return 1     # About names the commit: never build from the previous commit's build-info.gen.ts
  ( cd "$HERE/AutoradioLab" || exit 1
    exec env -i HOME="$HOME" USER="${USER:-$(id -un)}" TMPDIR="${TMPDIR:-/tmp}" BUN="$_bun" APP_ID="$_scheme" \
      PATH="$_path" \
      DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer EXACT2="$HERE/.exact-src" EXACT_FEEDBACK=0 EXACT_DEV_MENU=0 \
      "$@" )
}
