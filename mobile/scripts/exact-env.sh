# Sourced, never run (no shebang, not executable): exact_run <Exact's arguments> runs `bun exact.mjs <arguments>` for AutoradioLab in the environment this app's builds need, and
# returns its status. The two callers are exact.sh (after exact-guard.sh) and archive-store.sh (the only caller that passes --archive; it checks everything itself, see there).
# Needs HERE (the mobile directory).
#
# The app is the production app unless AR_VARIANT / AR_APP_ID chose a development app (app-identity.sh); Exact reads the choice as EXACT_VARIANT, which `env -i` would drop.
# The sign-in callback scheme is the app's id (app.ts SIGNIN_SCHEME, grant auth.callback): given to Exact explicitly for every identity, so the default never rests on a literal
# fallback and an empty value in the caller's environment cannot make a dead grant (app.ts reads it with `??`; the build refuses `||`).
# What `env -i` would drop and Exact reads from its environment (build-and-ship.md, "Archive, TestFlight, CI"): the signing pair, the version and build number and the
# export-compliance answer: passed only when set, so a development build is exactly as it was. EXACT_DEV_MENU=0 is given to every build: Exact's developer menu is off in every
# build of this app, so nothing in a build reads a caller's value.
exact_run() {
  "$HERE/scripts/check-exact-pin.sh" || return 1      # the wrong Exact must never build quietly (see exact.pin)
  _variant="$("$HERE/scripts/app-identity.sh" variant)" || return 1
  _scheme="$("$HERE/scripts/app-identity.sh" id)" || return 1
  _bun="${BUN:-$HOME/.bun-1.4.2/bin/bun}"
  set -- "$_bun" exact.mjs "$@"
  _n=$#
  for _v in EXACT_IDENTITY EXACT_PROFILE EXACT_VERSION EXACT_BUILD_NUMBER EXACT_NON_EXEMPT_ENCRYPTION; do
    eval "_isset=\${$_v+set}"
    if [ -n "$_isset" ]; then eval "_val=\$$_v"; set -- "$@" "$_v=$_val"; fi       # a value may hold spaces (a profile's path): one argument each
  done
  while [ "$_n" -gt 0 ]; do _a="$1"; shift; set -- "$@" "$_a"; _n=$((_n - 1)); done   # the assignments first, then the command
  "$HERE/scripts/gen-build-info.sh" || return 1     # About names the commit: never build from the previous commit's build-info.gen.ts
  ( cd "$HERE/AutoradioLab" || exit 1
    exec env -i HOME="$HOME" USER="${USER:-$(id -un)}" TMPDIR="${TMPDIR:-/tmp}" BUN="$_bun" ${EXACT_SIM:+EXACT_SIM="$EXACT_SIM"} ${_variant:+EXACT_VARIANT="$_variant"} SIGNIN_SCHEME="$_scheme" \
      PATH="$(dirname "$_bun"):$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin" \
      DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer EXACT2="$HERE/.exact-src" EXACT_FEEDBACK=0 EXACT_DEV_MENU=0 \
      "$@" )
}
