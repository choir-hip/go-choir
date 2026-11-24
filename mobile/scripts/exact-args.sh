# Sourced by exact-guard.sh (and so by exact.sh and build-lab.sh): the one place an Exact flag is recognised.
# A flag is recognised in both spellings, `--flag value` and `--flag=value`: ea_has <flag> <arguments...>.
# ea_only <allowed words...> -- <arguments...>: every argument is one of the allowed words.
ea_has() { _f="$1"; shift; for _a in "$@"; do case "$_a" in "$_f"|"$_f"=*) return 0 ;; esac; done; return 1; }
ea_only() { _ok=" "; while [ "$#" -gt 0 ] && [ "$1" != -- ]; do _ok="$_ok$1 "; shift; done; [ "$#" -eq 0 ] || shift
  for _a in "$@"; do case "$_ok" in *" $_a "*) ;; *) return 1 ;; esac; done; return 0; }
