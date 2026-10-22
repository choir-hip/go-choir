# shellcheck shell=sh
# Host-internal curl for host-side shell, mirroring hostauth.MarkInternal
# (internal/hostauth/hostauth.go): the marker header always, and the
# internal service token header file only when the URL is a loopback host
# on an internal service port. The token never goes to a guest (directly, or
# through a 127.0.0.1 DNAT port) or the internet; guest routes gate on the
# host source address and need only the marker. POSIX sh, so it runs under
# `sh -s` on Node B and bash 3.2 on a Mac. Sourced, never executed.
#
# docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md.
# scripts/choir_internal.py is the Python twin; scripts/selftest-guest-token
# proves both.

CHOIR_INTERNAL_TOKEN_HEADER=${CHOIR_INTERNAL_TOKEN_HEADER:-/var/lib/go-choir/secrets/internal-service-token.header}

# choir_internal_token_allowed URL: true only for 127.0.0.1, ::1 or localhost
# on 8083, 8084, 8086, 8087.
choir_internal_token_allowed() {
	_ci_hp=${1#*://}
	_ci_hp=${_ci_hp%%[/?#]*}
	_ci_hp=${_ci_hp##*@}
	case $_ci_hp in
	\[*\]:*) _ci_host=${_ci_hp%%\]:*} _ci_host=${_ci_host#\[} _ci_port=${_ci_hp##*\]:} ;;
	*:*) _ci_host=${_ci_hp%:*} _ci_port=${_ci_hp##*:} ;;
	*) return 1 ;;
	esac
	case $_ci_host in 127.0.0.1 | ::1 | localhost) ;; *) return 1 ;; esac
	case $_ci_port in 8083 | 8084 | 8086 | 8087) return 0 ;; esac
	return 1
}

# choir_internal_curl URL [curl args...]: curl URL with the marker, and the
# header file (read by curl, when readable) only for an allowed URL. Refuses
# -L: curl resends custom headers to a redirect's host.
choir_internal_curl() {
	_ci_url=$1
	shift
	for _ci_a in "$@"; do
		case $_ci_a in
		-[XHdowuAem]*) ;; # a flag with its value attached (-XDELETE)
		-L | --location | --location-trusted | -[!-]*L*)
			echo "choir_internal_curl: refusing $_ci_a (redirects resend headers)" >&2
			return 2
			;;
		esac
	done
	if [ -r "$CHOIR_INTERNAL_TOKEN_HEADER" ] && choir_internal_token_allowed "$_ci_url"; then
		curl -H 'X-Internal-Caller: true' -H "@$CHOIR_INTERNAL_TOKEN_HEADER" "$@" "$_ci_url"
	else
		curl -H 'X-Internal-Caller: true' "$@" "$_ci_url"
	fi
}
