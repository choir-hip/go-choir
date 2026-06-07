# Mid-Age Documentation Review - 2026-06-06

**Status:** cleanup ledger  
**Scope:** existing tracked docs last committed from 2026-05-24 through
2026-06-02. This deliberately excludes the 2026-05-23-and-older band already
reviewed in [old-docs-review-2026-06-06.md](old-docs-review-2026-06-06.md).  
**Decision bias:** keep current contracts, active problem records, and live
mission inputs; delete only clearly superseded or completed proof/checklist docs
after mining their durable signal.

## Mined Insights

1. **Deploy speed is graph selection first, cache second.** Cache cannot save a
   deploy graph that selects guest images, host closures, or vmctl restarts for
   unrelated changes. The surviving deploy mission is the v1 impact-isolation
   path.
2. **Features is the user-facing shape of app changes.** Owners should see
   demo, Import, background build/verify, email/Desk readiness, Activate, Roll
   back, and Roll forward. AppChangePackage/adoption/promotion remain internal
   evidence surfaces.
3. **Human proof gates app-change review.** A package/build receipt is not a
   feature proof. Reviewability requires owner-readable VText, media/benchmark
   or precise blocker evidence, post-implementation verifier output, rollback
   refs, and the right run-acceptance level.
4. **Supervision repair was folded forward.** The old async supervision repair
   mission's durable lesson is now in the runtime human-proof and runtime
   invariants docs: workers must deliver substantive updates to super, VText,
   and Trace without giving VText worker-control authority.
5. **Run-memory next frontier became AppChangePackage/adoption.** The old
   patchset mental model was pruned. The retained shape is stable foreground,
   isolated candidate mutation, typed evidence package, recipient adoption,
   verification, owner review, promotion, and rollback.
6. **Source-ledger podcast promotion was a useful forcing function, not the
   current source-system spec.** The durable insight is that one package
   identity should carry source refs, recipient build hashes, verifier evidence,
   Trace/run-acceptance refs, and rollback through user-to-user and platform
   adoption.
7. **Old API/VText hard-cutover checklist is now historical.** Its live lessons
   are the product API boundary and VText single-writer boundary already carried
   in current architecture, runtime invariants, the VText regression review, and
   [old-docs-review-2026-06-06.md](old-docs-review-2026-06-06.md).
8. **The May 12 Choir-in-Choir report is now history, not instruction.** It
   proved background worker export and content app bootstrap, but the current
   direction is typed AppChangePackage/adoption and product-path evidence.

## Kept Docs

These docs still carry current contracts, active problem records, or live
mission inputs:

- `docs/adr-dolt-as-canonical-state.md`
- `docs/auth-passkey-info-pretext-problem-2026-05-30.md`
- `docs/backend-browser-substrate-learnings.md`
- `docs/choir-agentic-depth-canonical.md`
- `docs/choir-email-reference-v0.md`
- `docs/choir-liquid-material-engine-design-v0.md`
- `docs/design-index.md`
- `docs/design-search-provider-plane-v1.md`
- `docs/frontend-app-building-api.md`
- `docs/implementation-scope.md`
- `docs/incident-gateway-deploy-eof-2026-05-29.md`
- `docs/legacy-promotion-experiments-learnings.md`
- `docs/mission-agentic-debugging-vtext-stability-v0.md`
- `docs/mission-apps-and-changes-store-sweep-v0.md`
- `docs/mission-campaign-compiler-selfdev-v0.md`
- `docs/mission-choir-grand-deformation-v0.md`
- `docs/mission-demo-stability-foundations-v0.md`
- `docs/mission-deploy-impact-isolation-cache-v1.md`
- `docs/mission-email-demo-ingress-v0.md`
- `docs/mission-geometry.md`
- `docs/mission-human-proof-experiment-rerun-v1.md`
- `docs/mission-maild-email-ingress-v0.md`
- `docs/mission-research-runtime-evidence-cadence-v1.md`
- `docs/mission-run-memory-v0.md`
- `docs/mission-runtime-model-context-substrate-v0.md`
- `docs/mission-search-provider-plane-v1.md`
- `docs/mission-super-console-real-zot-cutover-v0.md`
- `docs/mission-super-console-source-mount-promotion-v0.md`
- `docs/mission-vtext-live-cadence-repair-v3.md`
- `docs/mission-vtext-source-entities-multimedia-transclusion-v0.md`
- `docs/mission-web-surface-rationalization-v0.md`
- `docs/mission-youtube-review-studio-v0.md`
- `docs/north-star.md`
- `docs/platform-os-app-state.md`
- `docs/project-goals.md`
- `docs/public-identity-and-custom-domains.md`
- `docs/runtime-invariants.md`
- `docs/stable-platform-divergent-computers-architecture-2026-05-17.md`
- `docs/theme-system-contract-v0.md`
- `docs/vm-priority-policy.md`
- `docs/vtext-regression-review-2026-05-31.md`

## Deleted Docs

| File | Reason |
| --- | --- |
| `docs/api-vtext-hard-cutover-checklist-2026-05-01.md` | Completed historical checklist; live API/VText lessons are now in canonical docs and cleanup ledgers. |
| `docs/mission-deploy-impact-classes-cache-v0.md` | Superseded by `mission-deploy-impact-isolation-cache-v1.md`. |
| `docs/mission-features-hard-cutover-v0.md` | Completed product cutover mission; durable product-shape lessons retained above. |
| `docs/mission-gradient-choir-in-choir-final-report-2026-05-12.md` | Historical final report; durable lessons retained above and in current self-development missions. |
| `docs/mission-source-ledger-podcast-promotion-v0.md` | Superseded as a current source-system mission; package/adoption and podcast/radio lessons retained above and in current source/Autoradio docs. |
| `docs/mission-supervision-runtime-repair-experiment-rerun-v0.md` | Superseded by later runtime human-proof/runtime repair missions; supervision lessons retained above. |
| `docs/run-memory-next-frontier-2026-05-13.md` | Old next-frontier note; current target is Campaign Compiler/AppChangePackage adoption and the lesson is retained above. |

## Remaining Notes

This pass was followed by
[architecture-consolidation-2026-06-06.md](architecture-consolidation-2026-06-06.md),
which mined and deleted `docs/architecture.md` and
`docs/multiagent-architecture.md`.

It was also followed by
[source-publication-consolidation-2026-06-06.md](source-publication-consolidation-2026-06-06.md),
which mined and deleted
`docs/platform-dolt-publication-retrieval-citation-research-2026-05-16.md`.
