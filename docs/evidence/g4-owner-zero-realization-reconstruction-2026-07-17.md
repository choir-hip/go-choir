# G4 Owner Zero-Realization Reconstruction — Deployed Proof

**Observed:** 2026-07-17 on staging Node B at deployed commit `42e50b6b1fa3ae7461bb789ec173521a768b548d` after CI run `29565482629` completed successfully.

**Mutation class:** **red**. Protected surfaces: vmctl constructed-candidate lifecycle, exact route-absence authority, immutable ComputerVersion construction, independent realization verification, owner recovered state, and disk reclamation. No production or real-owner route was created or changed. The only destroyed machines were disposable proof candidates; the legacy owner realization remained stopped and untouched.

**Machine-readable receipt:** [`g4-owner-zero-realization-reconstruction-2026-07-17.json`](g4-owner-zero-realization-reconstruction-2026-07-17.json).

## Frozen identity

- Owner: `5bd6de97-3b58-408c-bf89-c42c81b083de` (`yusefnathanson@me.com`).
- Proof route slot: `computer:5bd6de97-3b58-408c-bf89-c42c81b083de:owner-recovery-g4-20260717`; absent before, during, and after both realizations.
- CodeRef: `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`.
- ArtifactProgramRef: `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f`.
- ArtifactProgram payload: 2,076 files, 609,636,416 logical bytes. No legacy `data.img` was a constructor input.

## Destroy and reconstruct

The first realization produced disk receipt `disk-instantiation:sha256:5cd0eb3a2d777fa99c5fde63e7920ac2954a6813611a8deef25d7a80daa460c6`, allocated 627,773,440 bytes, and passed `independent-production-realization-verifier` as receipt `verification:sha256:88b3e33d7cf700fa349ff900226a4a64c17e85b93b668f0d610096962774902a`. A vmctl restart safely changed the disposable ownership from active to stopped; exact unrouted disposal then removed the ownership and `data.img` while preserving route absence.

Construction from the same immutable CodeRef and ArtifactProgramRef then produced a distinct disk receipt, `disk-instantiation:sha256:38d19298c2294844b38504a90222d416e5812fb2c910131c8892b5ada9a59e3f`, allocated 627,777,536 bytes, booted healthy, and passed the independent verifier as `verification:sha256:ac873a6db46c8c56a59ba5dd2afc1d09c94a8c0e28a6cf6d55d5c07cffc8a0b6`.

Both verifier receipts recomputed the same observation digest, `683600051b723e3844622b133473200f90488e3e8f35d2d4616976c921816a2d`. Product files API readback from the reconstructed guest matched the first realization and the immutable source:

- `legacy-recovery/sql/texture.sql`: `f0e9f62f4571408d997cd795c1f266ff9178a927757bf11fa36f8591a4875eba`;
- `legacy-recovery/actor/state-actor.db`: `82cdd14b36786e010cbce82e71cee17950b0ec483eabb5ad91a7fe2333733e6e`.

The deployed exact unrouted disposal endpoint then accepted the second realization while it was **active**. Receipt SHA-256 `066202bc2e970a550c3b7f18ad6c612bd850aa1cb6f28c720643b4ae9e24e147` records `prior_state: active`, the second disk receipt, and `route_absent: true`. After return, vmctl listed no ownership with the candidate VM ID and its `data.img` was absent.

## Final state and checks

The refreshed sanitized registry contains 150 ownerships: 148 hibernated, one stopped, and one failed. It contains no owner proof candidate. Its artifact SHA-256 is `0617a1a9294ba511081b7aa43f5c0f6c0467810615a97631c997edd663200ad3`; canonical registry SHA-256 is `dc5154781a73df5b12d84139c35190aa5f5540715572c3b157413ff15a8a13c4`.

The deployed repair therefore closes the fate-sharing gap: exact pre-stop identity/version/disk/publication/route validation; stop through VMManager; destroy and persistent ownership removal; safe refusal or stopped retry state on failure. Routed disposal remains terminal-state-only.

## Authority and rollback

This receipt authorizes evidence use at G4 only. It does not authorize a fleet detach or route CAS. The proof route remained absent, so no route rollback was required. Both disposable realizations are gone. The real owner legacy VM and disk remain the rollback source for its future serialized cutover.

**Heresy delta:** `discovered: 0`, `introduced: 0`, `repaired: 1` — repaired the staged active-candidate lifecycle split documented in `g4-staged-candidate-lifecycle-cluster-2026-07-17.md`.

**Conjecture delta:** “a verified unrouted candidate can be destroyed and reconstructed from immutable refs without inherited mutable disk state” moved from unproved to supported on deployed staging. Residual risk moves to per-route serialized fleet execution; G4 still requires independent acceptance of the frozen 150-plan packet before any fleet mutation.
