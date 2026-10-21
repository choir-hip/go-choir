# Capsule package installs leave through a CONNECT tunnel, not a mirror

Date: 2026-10-21. Status: open; the fix is station FP10
([choir-launch-fp10-net02-mirrors-2026-10-21.md](../definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md)),
security and density checklist NET-02. Mutation class of the fix: red.

## Problem

An implementation capsule (`ecosystem_proxy`, L2) installs npm, PyPI and
crates packages and clones git repositories through the capsule egress
proxy's CONNECT tunnel to the registry or forge itself
(`internal/capsule/egress.go`, `ecosystemHosts`). Only Go modules go through
a host mirror: `GOPROXY=http://goproxy.choir.internal,off`, served by
`go-choir-goproxy` on Node B (`internal/goproxy`, tap port 8088).

A CONNECT tunnel carries opaque TLS. The proxy sees a host name (since
`a38eb7f67`, also the ClientHello's server name), never the method, the
path or the body. So:

1. **What a capsule may say to a listed host is unbounded.** Any request the
   host accepts (a POST, a push, an upload, a path that encodes data) is
   possible. The tunnel-identity record
   ([egress-proxy-tunnel-identity-2026-10-21.md](egress-proxy-tunnel-identity-2026-10-21.md))
   names a fetch service per ecosystem as the durable fix; the grants
   design argued the same in principle 2 and threats T1/T8
   ([engineering-network-grants-2026-10-10.md](../design/engineering-network-grants-2026-10-10.md)).
2. **Suffix matching admits every subdomain** of a listed name
   (`githubusercontent.com`, `pythonhosted.org`, `crates.io`), an open
   residual of the same record.
3. **Nothing is shared.** Every computer downloads the same packages from
   the internet; the Go proxy alone dedups across users.
4. **Guest egress cannot close** (NET-04) while package installs need the
   guest to reach the internet.

## Exposure

Every L2 capsule on every computer. `claude`-mode capsules reach only the
model API and the Go proxy and are not exposed through package hosts.

## Evidence

Code read on `53a691be6`: `ecosystemHosts` lists the registries and forges;
`EgressProxyEnv` points npm, pip, uv, git and cargo at the CONNECT proxy and
only Go at a host mirror; `tapReachableHostServicePorts` DNATs 8088 alone
for package traffic. The checklist entry (NET-02) states "Now: only the Go
module proxy (3c6347e81, c841fa474)".

## Fix direction

The goproxy pattern, per ecosystem: a host read-through mirror that speaks
plain HTTP to the capsule, GET/HEAD only, a path grammar per protocol,
fixed upstreams over TLS, an append-only shared cache, and verification
against the registry's published hash before an artifact enters the
cache. Capsules reach it through a fixed egress route, never an address
they name. Off by default, per ecosystem on the host and per computer in
the guest, with a separate switch that removes the mirrored registries
from the CONNECT allowlist ("mirrors only"). The station file carries the
design and the failure list.
