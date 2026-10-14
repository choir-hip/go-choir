# The Go Module Proxy Unit Cannot Create Its Cache; Deploys Fail

Date: 2026-10-14, 11:35 UTC. Deploy of 6872577c (CI 38553959817) failed at
"Deploy to Staging (Node B)": the NixOS switch reported a failed unit.
Staging kept serving 97d7a573. Every later push on main retried the same
deploy and failed (CI 38554935282, 38555830199), because the deployed
commit is still behind the proxy commits.

## Evidence

`journalctl -u go-choir-goproxy`: `goproxy: open cache: mkdir
/var/lib/go-choir/goproxy: permission denied`, ten starts in about 30 s,
then `start-limit-hit`.

The unit runs with `DynamicUser = true` and `StateDirectory =
"go-choir/goproxy"`, and the binary opens `-cache-dir
/var/lib/go-choir/goproxy`. `/var/lib/go-choir` is a real directory owned
by root with mode 0750. systemd placed the state directory at
`/var/lib/private/go-choir/goproxy` (owned by the dynamic user), but the
service's view of `/var/lib/go-choir/goproxy` sits under root's 0750
parent, which the dynamic user cannot traverse, so the mkdir fails.

## Fix

A top-level state directory the dynamic user owns:
`StateDirectory = "go-choir-goproxy"` and `-cache-dir
/var/lib/go-choir-goproxy`. Nothing else changes. The local tests could not
catch this (they do not run systemd); the design doc's "unit fails at the
switch, visible in the deploy" is what happened.
