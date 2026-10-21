#!/usr/bin/env bash
# Runs internal/vmmanager's real-netfilter tests (TestGuestEgressPolicyOnRealNetfilter,
# TestHostFloorOnRealNetfilter) as an unprivileged user, inside a fresh user,
# network and mount namespace: no root, and nothing on the host's own
# netfilter or interfaces is touched. Needs unprivileged user namespaces,
# iproute2 and iptables (nf_tables).
#
#   scripts/vmmanager-netfilter-test.sh [extra go test -run regexp]
set -euo pipefail
cd "$(dirname "$0")/.."
run="${1:-RealNetfilter}"
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
go test -c -o "$bin/vmmanager.test" ./internal/vmmanager
# A tmpfs over /run gives `ip netns` a writable /run/netns; that hides
# /run/current-system, so the tools are passed by their store paths.
path=""
for b in ip iptables ip6tables sysctl mkdir mount; do
  path="$path:$(dirname "$(readlink -f "$(command -v "$b")")")"
done
cd internal/vmmanager
unshare -rnm sh -c "mount -t tmpfs tmpfs /run && export PATH='${path#:}' && mkdir -p /run/netns && CHOIR_NETFILTER_TEST=1 '$bin/vmmanager.test' -test.run '$run' -test.v"
