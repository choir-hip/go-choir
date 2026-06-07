# Old Documentation Review - 2026-06-06

**Status:** cleanup ledger  
**Scope:** tracked `docs/**/*.md` whose last Git update was on or before
2026-05-23.  
**Method:** mine durable insight first, then delete stale snapshots when a
current canonical doc or this ledger preserves the useful signal.

## Review Rule

Old docs were classified by marginal value:

- **Keep canonical:** still a live contract or vocabulary source.
- **Keep deferred:** still a useful backlog, incident, or resumable mission.
- **Mined then deleted:** useful lesson preserved here or in a newer doc; the
  source file was stale as an operating instruction.
- **Deleted obsolete:** no marginal current insight beyond pointers to newer
  docs.

This review treats old proof and mission files as evidence, not authority. When
an old file conflicts with `AGENTS.md`, `docs/source-external-data-publication.md`,
`docs/current-architecture.md`, `docs/runtime-invariants.md`, or newer dated
problem/spec docs, the newer contract wins.

## Mined Insights

1. **Public API boundary:** browser-public APIs should express product intent,
   not agent control. The browser may submit prompts, VText revisions, source
   actions, and app operations, but must not select runtime roles, mutate
   prompts, spawn raw agents, write raw events, or bypass Trace/product APIs.
2. **VText writer boundary:** conductor may route and open; VText should own
   canonical appagent document synthesis through the same edit/revision path
   used for later versions.
3. **Prompt taxonomy is weak control flow:** keyword lists for "needs research"
   or "needs worker" repeatedly accumulate exceptions. Durable co-agent
   messages and VText-owned revision loops are the better control surface.
4. **Dolt layering:** per-user embedded Dolt is a natural private computer
   ledger; platform Dolt is publication/routing/provenance ledger. SQLite can
   remain for hot runtime, auth/session, cache, and transitional compatibility.
5. **Publication is a trust-domain copy:** publishing copies selected immutable
   refs, source metadata, citations, manifests, access policy, export policy,
   hashes, and rollback refs into a platform-visible artifact.
6. **Pretext is layout, not provenance:** rendering can use Pretext-style
   disclosure/layout, but source identity belongs in typed source entities,
   ContentItems, publication edges, and immutable refs.
7. **Podcast/radio proof path:** an imported podcast/feed episode can become a
   durable source artifact and VText radio brief. That path is now an input to
   Automatic Radio, where playback, source traversal, VText narration, and user
   voice contributions converge.
8. **Run memory proof path:** compaction and overflow handling need durable
   operational sufficient statistics linked to run/session branches. A chat
   summary alone is not recovery.
9. **Context limits are product behavior:** near-limit compaction, overflow
   detection, forced retry, and continuation selection are part of the runtime
   contract and should appear in Trace/run-memory evidence.
10. **App-owned previews:** desktop overview previews should be app-owned
    descriptors with privacy/resource redaction, not fake thumbnails or shell
    guesses.
11. **Media apps should be real app surfaces:** PDF/EPUB/audio/video/podcast
    surfaces should use real artifacts and app-specific readers/players.
    `ContentViewer` should not become a universal dumping ground.
12. **Recovery/observability is a product faculty:** users need product-facing
    status and bounded recovery actions for warmness, hibernation, app restore
    pressure, and runtime health, without raw vmctl/internal exposure.
13. **Human-proof loop:** self-development proof requires narrative VText,
    post-implementation verifier evidence, screenshots/video or precise
    blocker, package/adoption/rollback refs, and run acceptance at the right
    evidence level. Receipts alone are not feature proof.
14. **Test pyramid direction:** pure routing/model/prompt policy should be
    cheap unit tests; embedded Dolt belongs to persistence/restart/schema truth;
    worker/browser/live proof belongs to explicit integration or staging paths.

## Project Evolution Lessons

Choir has moved from milestone/checklist docs toward a smaller set of stronger
contracts:

- **From routes to product APIs:** early API reviews named dangerous browser
  access; later contracts made prompt-bar, VText, Trace, and app-change APIs the
  product boundary.
- **From demo surfaces to owner artifacts:** early media and desktop missions
  tried to make apps feel real; newer source/VText docs insist on durable
  source entities, provenance, and publication contracts.
- **From local proof to staging evidence:** old local proof notes are useful
  only when they feed deployed product-path evidence and run acceptance.
- **From agent choreography to durable messages:** repeated VText regressions
  show that hidden tool-order state should give way to addressed co-agent
  messages and single-writer revision semantics.
- **From "run a VM" to "own a computer":** old sandbox/vm terminology now
  resolves into the persistent computer ontology and candidate-world promotion
  model.
- **From radio as feature to radio as interface:** old podcast proof is now a
  seed for Automatic Radio: continuous retrieval/playback, user speech as
  content, DJ/control/notification layers, and private workflow radio.

## File Decisions

| File | Decision | Marginal insight retained |
| --- | --- | --- |
| `docs/PROJECT-STATE.md` | Deleted obsolete | It only pointed to newer docs; the pointer itself added no live insight. |
| `docs/api-surface-and-vtext-workflow-review-2026-05-01.md` | Mined then deleted | Public API boundary, Trace as read-only projection, and VText writer boundary retained above. |
| `docs/choir-grand-deformation-product-slice-2026-05-13.md` | Mined then deleted | Product-pressure slice and canonical VText route proof retained as evolution lesson. |
| `docs/cognitive-transform-portfolio.md` | Keep canonical | Repo-facing entrypoint to the cognitive-transform skill. |
| `docs/computer-ontology.md` | Keep canonical | Referenced by `AGENTS.md`; governs computer/VM/candidate vocabulary. |
| `docs/context-limit-recovery-proof-2026-05-13.md` | Mined then deleted | Context-limit and compaction proof lesson retained above. |
| `docs/deferred-reliability-migrations-2026-05-14.md` | Keep deferred | Still names sandbox-to-computer rename, SQLite/Dolt cleanup, and storage retention backlog. |
| `docs/glossary.md` | Keep canonical | Current vocabulary source. |
| `docs/incident-gateway-auth-vm-orphan-2026-05-21.md` | Keep deferred | Incident evidence still useful for gateway token lifecycle and orphan VM recovery. |
| `docs/memo-problem-documentation-first.md` | Keep canonical | Referenced by `AGENTS.md`; current operating invariant. |
| `docs/mission-4-core-functionality-and-choir-in-choir.md` | Mined then deleted | Historical milestone sequencing retained as evolution lesson. |
| `docs/mission-computer-recovery-system-monitor-v0.md` | Keep deferred | Still a coherent recovery/observability mission with live product value. |
| `docs/mission-desktop-overview-app-owned-spatial-previews-v0.md` | Mined then deleted | App-owned preview descriptor insight retained above. |
| `docs/mission-real-media-apps-ux-sweep-v0.md` | Mined then deleted | Real media app and `ContentViewer` boundary retained above. |
| `docs/mission-runtime-human-proof-experiment-rerun-v1.md` | Keep deferred | Still the best detailed record of the human-proof self-development gate. |
| `docs/mission-runtime-test-pyramid-hardening-v0.md` | Keep deferred | Still the live test-pyramid hardening direction. |
| `docs/mission-ux-full-bag-sweep-v0.md` | Mined then deleted | Mobile desktop, app boundary, and product-path proof lessons retained above. |
| `docs/missiongradient-method.md` | Keep canonical | Referenced by `AGENTS.md`; current MissionGradient method. |
| `docs/podcast-radio-brief-proof-2026-05-13.md` | Mined then deleted | Feed-to-VText radio brief proof retained above and in the Automatic Radio plan. |
| `docs/publication-path-skeleton-2026-05-12.md` | Mined then deleted | Publication trust-domain copy and platform ledger boundary retained above. |
| `docs/publication-reader-retrieval-pretext-research-2026-05-16.md` | Mined then deleted | Pretext/provenance distinction and source-publication insights retained above. |
| `docs/research-dolt.md` | Mined then deleted | Dolt layering retained in the ADR and summarized above. |
| `docs/run-memory-v0-dogfood-2026-05-13.md` | Mined then deleted | Run-memory proof lesson retained above; next frontier doc remains. |
| `docs/vtext-next-planning-checklist-2026-05-09.md` | Mined then deleted | Scope separation and VText coding-benchmark path retained above. |

## Remaining Cleanup Direction

- Keep pruning only when an old file has been indexed here or folded into a
  canonical contract.
- Prefer one cleanup ledger per review wave rather than scattered inline notes
  in every old mission.
- Treat current source-system, VText, Base, and Automatic Radio docs as the
  active planning surfaces; old proof files should feed them only through mined
  lessons.
