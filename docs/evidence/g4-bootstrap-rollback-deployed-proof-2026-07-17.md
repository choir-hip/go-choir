# G4 Bootstrap Rollback-to-Absence — Deployed Proof

Observed 2026-07-17 on staging Node B at deployed commit `e6fa53f10db3ba9499175d7a1d7912a0cbe2f876`.

Mutation class: **red**. Protected surfaces: signed G3 authority, route-ledger CAS and receipts, vmctl ownership persistence, constructed candidate disposal, and exact legacy restore.

## Deployment

- CI run [`29561357407`](https://github.com/choir-hip/go-choir/actions/runs/29561357407): success, including all race shards, vet/build, differential SBOM, rolling-flake publication, and Node B deploy.
- Node B deploy receipt: target commit `e6fa53f10db3ba9499175d7a1d7912a0cbe2f876`, activated `2026-07-17T07:07:37Z`.
- `go-choir-vmctl.service`: active.

## Disposable fixture and invariants

The test used the previously restored synthetic legacy ownership `mission-restart-1778558340/primary`:

- legacy VM `vm-bb1b05195c9186fa06f22455522e81ff`;
- route slot `computer:mission-restart-1778558340:primary`;
- lifecycle `hibernated`, epoch `1`;
- no construction binding and no route;
- frozen inventory-row SHA-256 `683d9aaf581617b764a6f806b68f59acc4426c024438eb42fc4bc431db6789e1`.

The legacy `data.img` metadata tuple `device:inode:logical_bytes:allocated_blocks:mtime_epoch:ctime_epoch` was captured before detach and compared after detach, after exact restore, and after vmctl restart. It remained identical. No full-image scan, copy, or mutation was performed.

## Executed transition

1. Submitted a fresh owner-signed exact legacy detach authorization. The old ownership disappeared and its durable receipt remained available.
2. Constructed `candidate-g4-bootstrap-rollback-20260717` from the pinned baseline ComputerVersion. The construction returned a healthy boot, finalized disk receipt, and `equivalent` semantic observation result.
3. Independently verified that construction through `prepare-bootstrap`. The resulting frozen candidate contained:
   - a generation-0 `bootstrap` plan;
   - a generation-1 `bootstrap_rollback` plan;
   - the deterministic future bootstrap receipt as the exact rollback target.
4. Signed one G3 acceptance over both plan digests and applied `action: bootstrap`.
5. Observed route generation 1, exact ComputerVersion input readback, joined bootstrap receipt, and guest `/health` status `ready` with runtime `ready`.
6. Stopped the candidate while its route was still present.
7. Applied the same frozen candidate and acceptance with `action: rollback`.
8. Observed immutable transition kind `bootstrap_rollback`, committed generation 2, `route_absent: true`, and route resolution HTTP 404.
9. Replayed the exact rollback request. It returned the same receipt and preserved route absence.
10. Disposed the stopped candidate through the exact **unrouted** candidate-disposal boundary; its VM state directory was removed only after the route was absent.
11. Restored the exact legacy detach receipt. The original VM/user/desktop identity returned as `hibernated`, epoch `1`, without construction bindings.
12. Restarted vmctl. The route remained absent, the legacy ownership remained exact, the candidate remained disposed, and legacy image metadata remained invariant.
13. Changed the signed acceptance's rollback-plan digest without re-signing and replayed it. vmctl returned HTTP 409; the ownership-registry SHA-256 was byte-identical and the restored legacy ownership remained present.

## Durable packet

Machine-readable packet: [`g4-bootstrap-rollback-deployed-proof-2026-07-17.json`](g4-bootstrap-rollback-deployed-proof-2026-07-17.json)

Packet SHA-256: `76c6521bba9df23683164d696f8577fb37ef0f0c6071e81c0e96fad37cec2f87`.

The packet embeds the signed detach request/receipt, construction request/result, owner approval, independently verified frozen candidate, paired signed G3 apply requests, bootstrap and rollback resolutions, routed readback, guest health, stop response, idempotent replay, disposal receipt, exact restore response, final ownership, four route-absence receipts, forged refusal, image metadata observations, deployment identity, CI identity, and raw-artifact hashes. No private signing material is present.

## Adjudication impact

The first-bootstrap rollback substrate blocker is repaired for the deployed exact lifecycle. This proof authorizes refreshing and freezing the complete G4 fleet packet; it does **not** authorize a real-user detach or fleet route CAS without G4 independent acceptance.

Rollback of the implementation: before any fleet transition, revert `e6fa53f1`. After a bootstrap, always execute the signed rollback-to-absence, dispose the candidate, and restore the durable legacy receipt before reverting source.

Heresy delta:

- discovered: none beyond the documented first-bootstrap rollback gap;
- introduced: none observed;
- repaired: deployed, signed, receipt-bound, generation-1-only rollback from first bootstrap to route absence, including restart durability and exact legacy restoration.
