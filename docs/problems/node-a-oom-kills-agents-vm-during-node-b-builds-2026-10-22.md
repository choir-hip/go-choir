# node-a's OOM killer takes the agents VM when Node B's builds and Mike's deploy overlap (2026-10-22)

Date: 2026-10-22, 17:50 UTC. Found by: FP0, when the agents VM dropped its
ssh sessions mid-pass. Status: open, problem record before any fix.
Mutation class of a fix: orange (Node B's builder settings) or an owner
decision (node-a, which serves Mike and stays as is).

## Evidence

node-a's journal, read only:

- 17:33:42. `mike-deploy` runs `mike-staging deploy-sha 7ff9ac48…` (a
  buildah container build).
- 17:35:12. CI's deploy job for SEC-14 deploy 1 (staging 273d139c9) starts
  Node B's host build. Node B sends its Nix builds to node-a
  (`nix.buildMachines`, `ssh-ng://root@node-a-builder`, `maxJobs = 8`).
- 17:35:13. Global OOM on node-a (`constraint=CONSTRAINT_NONE`,
  `global_oom`, `cpuset=buildah-…`). The killer chose the largest process:
  `cloud-hyperviso` pid 2780020 of `microvm@agents.service`, anon-rss
  10,482,556 kB. `microvm@agents.service: Failed with result 'oom-kill'`.
- 17:35:45. The agents VM booted again. Every process in it was gone: six
  FP0 worker sessions and the mirror queue. The workers were resumed from
  their saved sessions at 17:41.
- Mike: the backend restarted at 17:37:36 as part of its own deploy, and
  its containers kept reporting health. Nothing on node-a was changed.

Earlier the same day, 17:30:14, the agents VM's own OOM killer took a
`store.test` (2.9 GB anon) inside the VM. That one was inside the VM's
budget and is a different problem (mirror concurrency, already bounded by
one heavy lock).

## Cause

`nix/node-b.nix` describes node-a as "12 cores, 31 GiB, no guests" and lets
Node B run up to 8 build jobs there. That is no longer true. node-a now
holds:

- the agents VM, whose memory stays resident once touched (10.5 GB);
- Mike's staging stack and its deploys' container builds.

A Node B host build (several Go services at once, up to 8 jobs) plus a Mike
container build plus the VM can exceed 31 GiB. node-a's units have no
memory limits (`MemoryMax=infinity` for `microvm@agents` and `nix-daemon`,
`OOMScoreAdjust=0`), so the kernel kills the biggest process, which is the
agents VM. With a different mix it could pick Mike's database.

## What it costs

- Every overlap can kill all FP0 workers and mirrors at once (about 6
  minutes lost here, plus redone test runs).
- A CI deploy build that loses its builder mid-build falls back to building
  on Node B, beside the guests, or fails.
- Mike's staging shares the same risk.

## Fix options (not chosen here)

1. Node B side (in this repo, orange): lower `maxJobs` for node-a-builder
   (for example 3), and correct the comment. Node B still builds locally
   if node-a is unreachable.
2. node-a side (owner): a `MemoryMax` or `MemoryHigh` on node-a's
   `nix-daemon` (builds only), or a lower OOM score for `microvm@agents`
   and Mike's database so that a build is killed first.
3. Process: FP0 does not start Node B builds while Mike deploys. This is
   weak, because CI deploys start when they start.

Option 1 is FP0's to make. Option 2 changes node-a and needs the owner.
