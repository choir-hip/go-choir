# Host-locality inventory, first pass (2026-10-28)

Slice 1 of docs/definitions/choir-data-architecture-selection-2026-10-28.md.
Read-only grep of main 3e9057149 by FP0; counts are files or occurrences,
not yet classified call by call. [V] means the grep hit; whether each hit is
a real cross-host blocker is [H] until each one is read.

## Loopback addresses in nix/node-b.nix [V]

Services find each other at 127.0.0.1 or localhost: 8083 (5 refs), 8086
(5), 8087 (4), 8082 (4), 13306 platform Dolt (3), 8084 (2), and one each
for 8081, 8085, 8088, 8089, 8888 and 6333 (qdrant). The host-internal token
is valid only on loopback ports 8083/8084/8086/8087 and the vmctl socket by
rule. Moving a service off-host needs an address, authentication (mTLS or
a scoped token) and TLS in place of loopback trust.

## Same-filesystem copies: FICLONE and reflinks [V]

internal/vmmanager (data_image_capture, data_image_template, manager,
launch_linux, confinement, jail_shared), internal/vmctl
(data_image_capture, store_compaction_pressure), internal/projectionbase
(capture, compact). The S2 capture, data image templates, compaction and
the squash backups assume one copy-on-write filesystem. These belong to
the compute host and stay host-local; the durable outputs (bases, tape
segments) are what must leave the host.

## Artifacts read by path [V]

internal/platform (config, handlers, platform_update, objectgraph_store,
artifact_gc), internal/projectionbase (source, types),
internal/ledgerwitness/walk.go, cmd/corpusd, cmd/ledger-witness. These read
/var/lib/go-choir/platform-artifacts directly; an object-store interface
(local CAS now, S3-compatible later) replaces them.

## Renames that may assume one filesystem [V count, H meaning]

os.Rename call sites by package: vmmanager 12, platform 10, agentcore 10,
vmctl 6, projectionbase 6, updater 4, storeschema 3, store 2. A rename
is atomic only within one filesystem; each site needs reading to see
whether it crosses a boundary that would move off-host.

## Next

Classify each hit as compute-local (stays), shared-store access (moves
behind an address or object API), or durable output (ships off-host).
Then the size census by table and identity kind, with rates.
