#!@bash@
# Guest tool exec log (docs/design/guest-tool-layers-2026-10-31.md 6).
# Logs that @name@ (tool layer @layer@) ran, at most once a minute per boot,
# and runs it. It never logs arguments, the environment or the working
# directory: they can carry user data. When the tool layer is not attached
# the real binary is absent: say so and exit 127 at once.
real='@real@'
present=0
[ -x "$real" ] && present=1
st=/run/choir-tool-exec/@name@
printf -v now '%(%s)T' -1
last=0
[ -r "$st" ] && read -r last <"$st" 2>/dev/null
case $last in '' | *[!0-9]*) last=0 ;; esac
# Only a call that could write the stamp logs: where the stamp directory is
# missing or read-only (a capsule's mount namespace), nothing is logged rather
# than a line per call.
if [ $((now - last)) -ge 60 ] && { printf '%s\n' "$now" >"$st"; } 2>/dev/null; then
	'@logger@' -t choir-tool-exec -- "tool=@name@ layer=@layer@ present=$present" </dev/null >/dev/null 2>&1 || true
fi
if [ "$present" != 1 ]; then
	echo "choir: @name@ is in the @layer@ tool layer, which is not attached to this computer (its guest tool profile); see docs/design/guest-tool-layers-2026-10-31.md" >&2
	exit 127
fi
exec "$real" "$@"
