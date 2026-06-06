# Source System Mission Hard Review

Date: 2026-06-06

Mission: `docs/mission-source-system-simplify-secure-smart-v0.md`

Requirements contract: `docs/source-external-data-publication.md`

Operating method: `docs/missiongradient-method.md`

Current repo head at review: `34906ee2ca133f065d63e445bf117e915fe670e2`

Status: `checkpoint_incomplete`

## Executive Result

This mission made major real progress, but it is not complete.

The source system is now materially better than the starting point: source fetch
policy is shared by runtime URL import and source-service adapters, YouTube
transcript fetching is policy-checked, text-like imports become canonical
`.vtext` by first durable revision, publication preserves source entities,
selector metadata, reader snapshots, access/export metadata, and authorized
guest source windows, and the real legal proposal exists as a source-bearing
VText with a published guest-readable proof artifact.

The hard failures are still important:

- There is no single generated or cross-language source contract consumed by
  runtime, platform, frontend, Source Viewer, Web Lens, export, and Source
  Service.
- The real owner legal proposal has not yet passed a fresh post-publication
  bounded-edit proof.
- Publication policy defaults to public route/public visibility unless revision
  metadata overrides it.
- The early deploy identity stamp concern remains open.
- Source Service and researcher-backed stale/blocked/refuting/qualifying product
  flows are not fully proven.
- The final pruning pass has identified weak paths, but broad code deletion has
  not yet landed.

## Cognitive Review

Selected transforms:

1. Depth extraction: "source" is not a UI label. It is an evidence object with
   authority, provenance, selector addressability, policy, and publication
   consequences. This changes the review from "does the UI open something" to
   "does the object survive across owner, publication, export, and guest
   boundaries."
2. Invariant inversion: ask what would make the proof false. The main false
   proof pattern is owner-authenticated source reads masquerading as public
   publication proof. The new real legal proposal guest proof avoids that by
   using a signed-out page and publication-carried reader snapshots.
3. Audience-level translation for the owner: the product promise is "a published
   legal document carries its sources with it." The owner should not need to
   know whether a source came from a URL, content item, or source-service item.
4. Homotopy check: synthetic fixtures are useful only if they deform into the
   real legal proposal path. The mission now has both fixture proofs and one
   real legal proposal publication proof, but the real bounded edit path is
   still below the stopping condition.

Changed plan from this review:

- Treat the real legal proposal publication as the strongest acceptance
  artifact so far, not as mission completion.
- Prioritize a document-id-scoped owner bounded edit proof next, avoiding the
  global prompt bar because it already misrouted once.
- Treat source contract consolidation as a schema problem, not another local
  renderer helper problem.
- Treat policy defaults as a product/security risk, not a documentation footnote.

## Completion Audit

| Requirement | Current Evidence | Status |
| --- | --- | --- |
| Verify Comet owner auth for `yusefnathanson@me.com` | Comet `/auth/session` returned authenticated owner user id `5bd6de97-3b58-408c-bf89-c42c81b083de`; same-origin bookmarklet confirmed it again before publishing. | Proven |
| Document every newly confirmed behavior problem before fixing | Mission doc records Problems 1 through 31, including source-fetch, structure, metadata, open-surface, selector, evidence, reader artifact, YouTube policy, local harness, and deploy-impact issues. | Proven for known fixed problems |
| Audit whole source system before behavior-changing code | Mission doc records source-system inventory across runtime, platform, frontend, source APIs, ContentItem, VText, publication, export, and Source Viewer. | Proven |
| Convert imported Markdown/text to canonical `.vtext` by first durable revision | Behavior commit `6a141811...` plus deployed `vtext-markdown-lineage` proofs for Markdown and plain text. | Proven for Markdown/plain text |
| Preserve export back to Markdown | Deployed lineage tests and legal proposal publication Markdown export confirm export paths. | Proven for tested paths |
| Create/migrate legal cloud proposal to true VText with equivalent long-form content | Owner Comet shows `choir_private_legal_cloud_proposal.vtext`; diagnosis and publication export preserve long-form title and Appendix A content. | Proven enough for current artifact |
| Root-cause v70-v78 appendix table regression | Bounded diagnosis shows table present/absent transitions across v70-v78; local tests covered partial table preservation. Full operation-level root cause for every historical transition remains partial. | Partial |
| Repair general structure-preservation path | `bfd23fa0` repaired omitted-parent-table preservation; deployed synthetic proof distinguishes omitted table from explicit deletion. | Proven for first repair class |
| Legal proposal table survival | Current legal proposal v94 diagnosis and publication content preserve a 50-row appendix table. Fresh post-publication bounded edit is missing. | Partial |
| Bounded table edit | `frontend/tests/vtext-source-entities.spec.js` covers bounded table edit locally; owner legal proposal bounded edit after publication remains missing. | Partial |
| Source acquisition policy checked and SSRF-safe | `internal/sourcefetch` tests, runtime import policy, source-service adapter policy, YouTube transcript policy, and deployed loopback rejection proof. | Proven for implemented fetchers |
| Robots/TOS/rate policy | Contract requires registry-level source policy. Some source policy hooks exist, but complete robots/TOS/rate behavior is not proven. | Missing |
| Shared source entity/reader artifact/selector/evidence/open-surface handling | Backend `internal/sourcecontract` and frontend `source-contract.ts` reduce drift; platform/export use shared backend selectors and reader artifact states. No single generated schema spans Go and TypeScript. | Partial |
| Source Viewer default, Web Lens explicit original/live inspection | Staging owner and guest proofs show durable sources open Source Viewer and no browser/Web Lens windows. Frontend open-plan tests cover explicit aliases. | Proven for tested surfaces |
| Selector-rich transclusions through publication/export | Source-service fixture proves selector sets in resolve/export metadata. Real legal proposal currently uses single text quote selectors. | Proven for fixture, partial for real proposal |
| Source snapshots through publication | URL-backed/content-item/source-service fixtures and real legal proposal publication carry reader snapshots for authorized readers. | Proven |
| Allowed source records for authorized publication readers | Signed-out guest opened the real legal proposal ABA source from the publication-carried reader snapshot. | Proven for one real guest path |
| Replace missing-source placeholders with typed evidence states | VText gaps, repairs, publication transclusions, frontend labels, and export metadata have typed states. Stale/blocked/refuting/qualifying product flows and Source Service records remain incomplete. | Partial |
| Content-forward magazine/journal inline transclusions | Current UI renders inline source notes and Source Viewer reader content. It is improved, but not yet a fully reviewed design system. | Partial |
| Use Pretext only if proof improves wrapping | No proof established that Pretext improves source wrapping, and no Pretext dependency was added for this mission. | Satisfied by restraint |
| Owner and guest source opens for URL/content/source-service | Fixtures prove all three. Real legal proposal proves URL/content-item style source records and guest Source Viewer. | Proven for fixtures, partial for real proposal |
| Publication/export source metadata | Deployed fixtures and real legal proposal export include source entities, transclusions, access policy, export policy, retrieval source, and retrieval spans. | Proven |
| Screenshots/traces | Real legal proposal guest screenshots and trace are stored under `docs/evidence/source-system-2026-06-06/`. | Proven |
| CI and Node B identity | Behavior commit `9a86044a` CI and deploy succeeded. Classifier commit `eaf14f3d` CI succeeded and deploy skipped. Health reports `9a86044a`. Early identity-stamp risk remains. | Partial |
| Rollback refs | Real legal proposal publication returned rollback id `rollback-a3a4e807-3ef9-46b6-9209-c206168a9ad2`; behavior rollback refs are recorded in mission doc. | Proven |
| Hard mission review Markdown and PDF | This document is the Markdown report. PDF generated in iCloud Drive at `Choir Mission Reports/mission-source-system-hard-review-2026-06-06.pdf`. | Proven |
| Prune dead/weak/shortcut code paths | Weak paths are identified below. Broad pruning has not landed. | Missing |

## Evidence Ledger

Latest pushed checkpoint:

- `34906ee2ca133f065d63e445bf117e915fe670e2`, `docs: record legal proposal publication proof`.

Latest behavior commit deployed on staging:

- `9a86044a244e9e0f41afd2162cd0cb277cbdbe0f`, `fix: require dev shell for local services`.
- Staging health during the real legal proposal publication proof reported
  proxy/upstream `deployed_commit=9a86044a244e9e0f41afd2162cd0cb277cbdbe0f`,
  `status=ok`, and `vmctl_status=ok`.

Latest CI proof:

- GitHub Actions CI run `27068825236` for
  `eaf14f3d2b36feb175088ff7d064ef5562f7ace0` succeeded.
- `Build Frontend` skipped.
- `Deploy to Staging (Node B)` skipped.
- This proved local harness classifier changes no longer deploy staging.

Real legal proposal publication proof:

- Owner document id: `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- Publication id: `pub-878ee08d-2085-4291-b747-eda7ef704693`.
- Publication version: `pubver-e782e93b-5867-4e17-b921-c7f4d2619d11`.
- Public route:
  `/pub/vtext/legal-proposal-source-proof-1780766508614-pub878ee08d2`.
- Public URL:
  `https://choir.news/pub/vtext/legal-proposal-source-proof-1780766508614-pub878ee08d2`.
- Retrieval source: `source-f43d8a65-ccc4-4865-901a-268c1ce31e6b`.
- Retrieval span: `span-35e90c40-5267-4b27-8a04-c852685f13cf`.
- Citation: `cite-d823a476-9e00-423f-b26a-6584bb49d1c9`.
- Consent: `consent-2fadded5-cc60-41dd-93da-5da182593795`.
- Review: `review-d2b36a60-8785-4281-8bf0-b2c2fbf27185`.
- Rollback: `rollback-a3a4e807-3ef9-46b6-9209-c206168a9ad2`.

Real legal proposal source/export proof:

- Public resolve returned 7 source entities and 7 transclusions.
- Source entities included URL-backed records and content-item records:
  `src_gdpr_article_32`, `src_aba_formal_op_512`, `src_nixos_rollback`,
  `src_aba_rule_16`, `src_ovh_private_cloud`, `src_hetzner_datacenters`, and
  `src_qdrant_search`.
- Each resolved entity exposed `open_surface: source`, a
  publication-reader reader snapshot, and `reader_snapshot_ready`.
- Markdown export preserved the proposal title and Appendix A, and included
  source entities, transclusions, access policy, export policy, retrieval
  source, and retrieval spans in canonical metadata.

Guest evidence artifacts:

- `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-reader-20260606T1722Z.png`
- `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-source-viewer-20260606T1722Z.png`
- `docs/evidence/source-system-2026-06-06/legal-proposal-publication-guest-source-viewer-20260606T1722Z.trace.zip`

The guest proof opened the public route signed out, expanded
`src_aba_formal_op_512`, clicked `Open source`, observed a Source Viewer reader
window, and observed zero `data-browser-app` windows.

## Hard Findings

### Finding 1: The source contract is still split

Severity: high.

The mission reduced drift by adding `internal/sourcecontract` on the backend and
`frontend/src/lib/source-contract.ts` on the frontend, but this is still two
contract implementations. Runtime, platform, Source Viewer, Web Lens, export,
and future Source Service paths can still diverge when a new state, selector, or
open-surface value is added.

Next repair:

- Define a generated source contract artifact or an explicit schema-generation
  path.
- Generate Go and TypeScript constants/types from it.
- Add a test that fails when frontend and backend source states/selectors/open
  surfaces are not in lockstep.

### Finding 2: Publication policy defaults are too permissive for sensitive docs

Severity: high.

The real legal proposal publication proof succeeded because guest publication
proof was required. The same path also exposed a product risk: platform
publication defaults to public route/public visibility unless revision metadata
overrides it, and the proxy request schema does not currently accept an explicit
publication access policy from the owner publish action.

Next repair:

- Document this as a platform behavior problem before code if it has not already
  been recorded as its own problem.
- Add owner-visible publish policy controls.
- Make sensitive/private VText publications require explicit route visibility
  selection.
- Preserve guest proof by publishing a deliberate public or unlisted acceptance
  artifact, not by relying on defaults.

### Finding 3: The real bounded edit proof is still not closed

Severity: high.

The legal proposal has passed restore, source-open, diagnosis, publication,
export, and guest source-open proof. It has not passed a fresh owner
post-publication bounded edit/revise proof with table signature, represented
sources, export metadata, trace, screenshot, and rollback refs.

Next repair:

- Use a document-id-scoped owner product path.
- Avoid the global prompt bar.
- Avoid whole-document accessibility replacement.
- Verify before/after diagnosis, represented source count, source opens,
  publication/export metadata, and rollback refs.

### Finding 4: Early deploy identity can still overstate reality

Severity: medium-high.

The deploy-impact overclassification for `start-services.sh` is fixed, and CI
proved deploy skip. The separate observation remains: `/health` reported the
target commit while the deploy job was still in progress. Until that is fixed,
acceptance reports must pair health identity with terminal deploy job status.

Next repair:

- Split target commit from active installed commit, or stamp active commit only
  after install/restart/health probes complete.
- Add deploy-script proof that the active identity cannot advance before the
  deployed services are actually serving the candidate.

### Finding 5: Source acquisition policy is strong for SSRF, incomplete for source standing

Severity: medium.

`sourcefetch` closes concrete SSRF and private-network classes for implemented
fetchers. Robots/TOS/rate policy, source standing, and connector-specific policy
are still not complete Source Service behavior.

Next repair:

- Add source registry policy records for robots/TOS/rate/standing.
- Route Web Lens and future connectors through the same policy decision.
- Record failed fetches as source health state, not as missing source.

### Finding 6: Typed evidence states exist, but not as an end-to-end research workflow

Severity: medium.

Typed states now appear in VText gaps, repairs, Source Viewer labels,
publication selectors, and export metadata. The full researcher-backed lifecycle
for confirming, refuting, qualifying, no-source-needed, stale, blocked, and
unavailable is not yet proven from researcher action through publication/export.

Next repair:

- Add a product-path test with multiple evidence states in one VText.
- Publish and export it.
- Verify owner view, guest view, Source Viewer, and export metadata all agree.

## Weak And Shortcut Paths To Prune

1. Frontend/backend duplicate source contract helpers should be replaced by a
   generated contract or at least a shared conformance fixture.
2. Publication source windows currently depend on frontend reconstruction of
   publication records. Move more open-plan and reader snapshot interpretation
   into a single contract surface.
3. Global prompt-bar edits for already-open VText documents are unsafe for this
   acceptance path because they created a separate artifact. They need scoped
   routing or should be excluded from owner legal-proposal proof.
4. Direct address-bar bookmarklets are acceptable for owner-authenticated
   read/publish probes, but should not become the durable mutation harness.
5. Publication policy defaults should not remain the hidden path for sensitive
   documents.
6. Any temporary proof scripts should stay out of the repo unless promoted into
   named regression tests.

## Residual Risks

- The real legal proposal has been publicly published as an acceptance artifact.
  It carries a rollback id, but route visibility defaults need product attention.
- The current legal proposal table head is 50 rows, not the earlier 49-row head.
  This appears stable at the current head, but exact historical equivalence is
  not fully proven after a new bounded edit.
- Source snapshots for published legal proposal records are good enough for
  guest proof, but source retention and rights policy need broader review.
- Generated source schema work may uncover mismatches that current tests do not
  cover.
- FlakeHub had one external timeout during this mission. It did not block the
  relevant CI/deploy proof, but it should not be ignored if it recurs.

## Next Realism Axis

The next uphill move is:

```text
owner document-id-scoped bounded edit
-> diagnosis before/after
-> represented sources count
-> table signature/row count
-> source open
-> publish/export
-> signed-out guest source open
-> rollback refs
```

This should use a normal product path or a deliberately built owner-auth
Playwright/Chrome path. It should not use the global prompt bar and should not
replace the whole document body through accessibility APIs.

After that, the schema-generation/source-contract convergence should be the next
behavioral slice.

## Final Judgment

The mission is a strong checkpoint, not a finish.

The product now demonstrates the core user promise on staging: a real legal
proposal can be a VText, carry source metadata, publish canonical export
metadata, and let a guest inspect publication-carried sources in Source Viewer.

The mission should remain open because the highest-risk gaps are exactly the
ones that tend to break later: policy defaults, schema drift, and real owner
mutation after publication.
