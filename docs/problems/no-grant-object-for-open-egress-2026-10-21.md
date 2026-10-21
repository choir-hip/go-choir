# No grant object for open egress (NET-05)

Date: 2026-10-21. Class: design gap, red when built. Checklist item: NET-05
in the [security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: [FP10 NET-05](../definitions/choir-launch-fp10-net-05-2026-10-21.md).

## Observed (code at main d1c60ad87)

- The only network level a capsule can hold is L2, the ecosystem allowlist
  proxy (`internal/capsule/egress.go`, `EcosystemEgressPolicy`, 284f7c1a2).
  Its host list is compiled in; nothing on the tape widens or narrows it.
- The capsule capability's `ExternalAccess` must be empty for engineering
  (`internal/agentcore/engineering_assignment_runtime.go:603`). There is no
  grant id it could carry.
- No `EgressGrant` type, store kind, `request_grant` or `decide_grant` verb
  exists (`grep -rn EgressGrant internal` finds nothing). L1 (fetch and
  mount), L3 (open web, guarded) and L4 (full network) from the grants design
  §4.0a have no authority record at all.
- App-desk manifests (NET-10 §2.1) declare egress verbs that "the owner act
  at install" turns into grants, but the grant those verbs become is the same
  missing object. `internal/deskmanifest` does not exist yet (ARCH-02).
- An owner-act receipt exists (`internal/owneract`, minted by the proxy,
  verified by corpusd), with act kinds for self-development only.

## Why it matters

1. **Agency.** Work that needs an API, a documentation site or an arbitrary
   download link (L3), or a server or non-HTTP protocol (L4), cannot be
   granted at all, so it is either refused or routed around through research
   relaying text (grants design §2.6, D3).
2. **Two grant models would diverge.** If engineering grants and app-desk
   manifest grants are built separately, the owner sees two approval surfaces
   and the proxy enforces two shapes (NET-10 rule 2, S7 addendum, S4).
3. **Authority without expiry is the hazard.** A grant that widens egress
   and does not die on a clock or with its holder is a standing exfiltration
   path (threat T1 in the grants design).

## Not the problem

- General guest egress is still open at the substrate (NET-01, NET-04,
  `s0-tap-egress-unfiltered-2026-10-01.md`). A grant enforced at the proxy is
  only meaningful once the proxy is the only path; this record does not claim
  otherwise.
- The L2 proxy's tunnel identity (SEC-06,
  `egress-proxy-tunnel-identity-2026-10-21.md`) is FP6's.

## Belief state

- Believed: one record type can carry both engineering grants (bound to an
  assignment) and app-desk grants (bound to an installed desk), with the
  owner act as the only decider for L1, L3 and L4 in the first slice.
- Remaining error: the enforcement point (host proxy, host fetch service
  NET-01, mirrors NET-02) is in flux in other lanes; how a grant reaches it
  is designed, not built.
