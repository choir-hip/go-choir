# G4 Fleet Execution Blocker — Firecracker TAP Name Collision

**Observed:** 2026-07-17 on staging Node B at deployed runtime `42e50b6b1fa3ae7461bb789ec173521a768b548d`, during the first mutation allowed by accepted G4.

**Mutation class:** **red**. Protected surfaces: real fleet ownership detach/restore, Firecracker construction, candidate cleanup, and route absence. The affected near-new canary was `a@b.com`, owner `0e5c45ab-44de-49cd-b07d-e58973b21ad5`; no route CAS occurred.

**Classification:** **substrate** — Firecracker network-device identity/allocation. This is not an account-specific or construction-payload symptom.

## Sequence and evidence

1. The accepted packet's retained synthetic control route, `computer:autoputer-control:control-20260716`, resolved generation 3 to the exact accepted baseline ComputerVersion. vmctl resumed its hibernated constructed realization and guest health plus authenticated files API readback succeeded.
2. The first mutable route, `computer:0e5c45ab-44de-49cd-b07d-e58973b21ad5:primary`, was absent. Its exact reviewed ownership was hibernated at epoch 25 with VM ID `vm-d067e51c904a6fc6b7810ec7dee75ad1`.
3. Signed exact detach produced receipt `legacy-detach:sha256:aa54eac35349e94d57c2fc8eb90885e08084d6c391401c26b722b37526312481`. The legacy `data.img` remained byte-location invariant: device 29, inode 50634058, logical bytes 34,359,738,368, allocated bytes 503,873,536, unchanged mtime/ctime seconds.
4. Baseline construction for reviewed candidate `candidate-fleet-49ee3bd0ec6f366a164c02d2` failed before a receipt was returned. Firecracker v1.14.2 reported:

   ```text
   Network device error: Could not create the network device:
   Open tap device failed: Error while creating ifreq structure:
   Device or resource busy (os error 16).
   Invalid TUN/TAP Backend provided by vm-candidat-tap.
   ```

5. The construction launcher's failure path stopped the failed Firecracker process, destroyed the candidate state directory, and removed candidate ownership. The route remained HTTP 404/absent.
6. Exact restore from the durable detach receipt reinstated the legacy ownership as hibernated at epoch 25. The same device/inode/logical/allocation/mtime/ctime tuple was observed after restore. Fleet execution stopped before any other route.

## Belief update

The G4 packet's route, detach, rollback, and candidate-disposal authorities behaved correctly. The first real fleet candidate exposed a lower network substrate collision: Firecracker was asked to open a TAP named `vm-candidat-tap`. Multiple long candidate IDs may collapse to that Linux interface name, or a stale device may be reused without ownership reconciliation. The exact derivation and any existing replacement allocator must be mapped before repair.

This is the first documented bug in this immediate TAP-name cluster, so root-cause clustering's three-symptom threshold is not yet crossed. The defect is already substrate-level and must be repaired there; retrying the account or changing its candidate ID would only mask the collision.

## Authority, rollback, and next safe probe

No fleet retry is authorized while TAP allocation remains ambiguous. Preserve the failed journal receipt and the restored legacy row. Inventory the deployed and source TAP naming/allocation/cleanup implementations, check for a replacement already present but unwired, reproduce the name collision deterministically, then repair the allocator with collision-resistant bounded Linux interface identities and exact cleanup ownership. Deployment, a disposable construction proof, and a fresh re-freeze of this canary's live row are required before another detach.

**Heresy delta:** `discovered: 1`, `introduced: 0`, `repaired: 0` — discovered a fleet-blocking shared TAP identity in the Firecracker substrate.

**Conjecture delta:** “accepted per-route construction can allocate a distinct network device for every reviewed candidate ID” moved from assumed to falsified. G4 remains accepted as a packet, but fleet execution is blocked until the substrate is repaired and the live canary row is re-frozen.
