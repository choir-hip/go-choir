# G4 First Fleet Canary Product-Path Authentication Blocker

**Observed:** 2026-07-17 on staging Node B after deployment of the TAP allocator repair.

**Mutation class:** **red**. Protected surfaces exercised: fleet ownership, immutable `ComputerVersion` construction, signed bootstrap/rollback CAS, existing route-ledger receipts, public product authentication, and exact legacy rollback. No production route remains changed.

## Problem

The collision-resistant TAP allocator is deployed and works. The accepted G4 sequence-1 `a@b.com` canary constructed, independently verified, and bootstrapped its immutable route successfully, but the mandatory post-CAS public product-path check could not obtain an authenticated session for the existing account. Its registered passkey private key is held outside mission authority. A fresh WebAuthn virtual authenticator correctly received `An account already exists for that email. Choose Sign in instead.` on registration and `Passkey prompt was cancelled` on login because it did not possess that account's credential.

This is not evidence that the constructed computer failed. It is an acceptance-authority blocker: direct candidate health and immutable readback passed, but G4 names any post-CAS product-path failure as a stop condition, and an external mission must not forge, replace, revoke, export, or bypass an owner's passkey. The canary therefore rolled back before any later fleet row moved.

## TAP repair deployed proof

- Source commit: `1c9ba54b196440ab13981468e55dbd1134793938`.
- CI: GitHub Actions run `29570035893`, success.
- Node B deploy receipt: exact target commit above, activated `2026-07-17T09:38:46Z`.
- Old shared `vm-candidat-tap` was absent before replacement.
- Accepted control realization resumed healthy on `vm-tvkv4ngjjlb7`.
- Disposable long-prefix candidate `candidate-tap-proof-20260717-a` constructed healthy on distinct `vm-ukfekyjlysv2` while control remained active, then exact active/unrouted disposal removed only its TAP and state.
- Sequence-1 realization `candidate-fleet-49ee3bd0ec6f366a164c02d2` subsequently constructed healthy on distinct `vm-nqqm7vjpwmxr` while control remained active. The prior EBUSY failure is repaired.

## Sequence-1 receipts

The restored reviewed legacy row re-froze exactly at persisted-row SHA-256 `79a742bbcd833af99cc1c1a39e3220d05fa2a539f2e3688b3339dfe919026c97`, state `hibernated`, epoch `25`, with route absent.

- signed detach receipt: `legacy-detach:sha256:dc5de7faa81992f438328dd5e4e3fcd86a15ab349ab851311e7d72ee9f85a258`;
- fresh sparse disk receipt: `disk-instantiation:sha256:69f63428e9041495efb351e26955d3bee631695644bcdada81551985c6ca350e`, 32 GiB logical / 10,600,448 allocated bytes;
- independent verification: `verification:sha256:2b46d4324982ea510c7822a7613194c8484d4e087e9b454053de3b8631b12cec`;
- frozen bootstrap candidate: `route-bootstrap:sha256:a9db3619d3eddd2680d7bbb603fef338e8858917a24342e65493a3ae911e5b74`;
- bootstrap receipt: `d2f8fb62-fdb4-5627-8427-28bac483ce24`, generation `0 -> 1` at `2026-07-17T09:51:38.401259161Z`;
- exact signed rollback-to-absence receipt: `a13734b6-e05d-4420-bd43-ec6af0dd273b`, generation `1 -> 2`.

## Containment and rollback proof

After public authentication could not be established:

1. vmctl stopped the routed candidate.
2. The pre-frozen, G3-signed rollback performed the only route CAS, generation 1 to generation 2 absence.
3. Route resolution returned HTTP 404.
4. Exact stopped/unrouted candidate disposal removed the constructed ownership and `data.img`.
5. Exact detach-receipt restore reinstated legacy VM `vm-d067e51c904a6fc6b7810ec7dee75ad1` at state `hibernated`, epoch `25`.
6. The legacy `data.img` device, inode, logical size, allocated blocks, mtime, and ctime matched the pre-detach tuple.
7. After vmctl restart, route resolution remained HTTP 404, the exact legacy row remained present, the candidate row remained absent, and the accepted control TAP remained active.

No later fleet row moved. No account credential was mutated.

## Authority boundary and next action

Resume requires an admissible authenticated public product-path session for the exact sequence-1 owner, or a newly frozen and independently accepted G4 packet that changes the canary order/account while preserving every per-route exact binding and owner gate. Replacing or bypassing `a@b.com`'s existing credential is forbidden.

**Rollback refs:** generation-2 route-absence receipt `a13734b6-e05d-4420-bd43-ec6af0dd273b`; exact restored legacy VM and detach receipt above.

**Heresy delta:** discovered `1` (the accepted fleet plan assumed the executor could perform owner-authenticated post-CAS checks without possessing owner credentials); introduced `0`; repaired `0`.

**Conjecture delta:** full-identity TAP naming moved from conjecture to deployed proof. Fully autonomous serialized fleet cutover moved from plausible to falsified under the current authentication authority boundary.
