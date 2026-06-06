# Source System Hard Mission Review

Date: 2026-06-06

Mission: `docs/mission-source-system-simplify-secure-smart-v0.md`

Contract: `docs/source-external-data-publication.md`

Method: `docs/missiongradient-method.md`

## Verdict

Status: checkpoint accepted for the main source-system correctness path, not full
mission completion.

The mission now has staging evidence for the hardest correctness claims:

- owner-authenticated Comet staging access for `yusefnathanson@me.com`;
- source-bearing legal proposal as true VText;
- legal proposal table survival through app-agent revise;
- post-publication bounded owner edit survival through app-agent revise;
- explicit VText public publication acknowledgement and policy forwarding;
- publication-carried source snapshots, transclusions, and Markdown export
  metadata;
- owner and guest Source Viewer source-open paths for focused fixtures;
- SSRF-safe source fetch policy for URL import and YouTube transcript paths;
- typed source evidence and reader artifact states in the source UI.

The mission is not globally complete because several broad architectural
requirements are still proven by focused slices rather than exhaustive contract
coverage across every future source producer. The remaining risks are explicit
below.

## Cognitive Review

Selected transforms:

1. Depth extraction: the load-bearing object is not the source UI. It is the
   revision-scoped evidence contract that survives acquisition, edit, publish,
   open, and export.
2. Inversion: the dangerous failure is not a missing button; it is a path that
   silently drops source identity while still looking readable.
3. Boundary audit: Source Viewer is a reader for durable artifacts. Web Lens is
   live/original inspection. Any URL-present fallback that changes this boundary
   is suspect.
4. Evidence realism: local UI proof is weak for auth, deploy identity, vmctl,
   worker, provider, and publication behavior. Staging evidence is required for
   these claims.

Changed review plan:

- treat public resolve/export artifacts as stronger proof than UI impressions;
- treat the legal proposal bounded edit as the central structure-preservation
  verifier;
- treat manually mirrored frontend/Go contract constants as residual risk even
  when behavior tests pass;
- reject stale Playwright storage and direct bookmarklet auth failures as proof
  of product failure unless reproduced through the product renewal path.

## Accepted Evidence

### Deploy Identity

Behavior commit deployed to staging:

`8efb05a25430330ada50e1a2ac6ebe2418af9700`

Staging health on 2026-06-06 reported:

- proxy `status=ok`;
- upstream `status=ok`;
- `vmctl_status=ok`;
- proxy and upstream deployed commit
  `8efb05a25430330ada50e1a2ac6ebe2418af9700`;
- deployed at `2026-06-06T17:44:31Z`.

Latest docs-only checkpoint:

`e808a7944f971a1d2e47b87592b644ecd7be22dc`

This was pushed to `origin/main` and is intentionally not expected to deploy
because docs-only commits are ignored by CI/deploy path filters.

### CI

Behavior commit `8efb05a2` had successful GitHub Actions runs:

- CI run `27069371444`;
- FlakeHub run `27069371443`;
- Node B deploy job `79895836858`.

Focused local checks during hard review:

```text
nix develop -c go test ./internal/sourcecontract ./internal/platform -run 'TestNormalize|TestBuildPublication|TestHandleVTextPublication|TestExport' -count=1
```

Result: passed.

```text
npm --prefix frontend run build
```

Result: passed.

### Owner Comet Capability

Computer Use / Comet proved staging owner auth for:

- email `yusefnathanson@me.com`;
- user id `5bd6de97-3b58-408c-bf89-c42c81b083de`.

Limitations:

- old headless Playwright storage at
  `/tmp/choir-policy-forward.storage.json` was stale and rejected;
- `screencapture` failed with `could not create image from display`;
- the accepted owner proof therefore relies on Computer Use observation plus
  stored product/API evidence, not a new PNG.

### Legal Proposal Bounded Edit

Document:

`f93cea62-f833-4dae-b414-8e44783d8cbe`

Accepted revision path:

- v95 `fc2cd0de-ba3a-458f-b1b3-04206d74df9c`: user bounded table edit;
- v96 `5a5532d8-0ff3-44d6-aeef-5ea6cbc08798`: app-agent revise.

v96 preserved:

- bounded edit phrase;
- Appendix A;
- glossary table header;
- bounded Vector database row;
- 7 source entities;
- 7 source markers;
- `vtext_context_mode=focused_user_edit_diff`;
- `vtext_edit_operation=apply_edits`.

Evidence:

- `docs/evidence/source-system-2026-06-06/legal-proposal-post-publication-bounded-edit-20260606T1758Z.json`

### Publication And Export

Published route:

`/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubf7bae84a8`

Publication:

- publication `pub-f7bae84a-80fa-4bf7-87f7-18ff07a01ca4`;
- publication version `pubver-ae91528d-d605-42dc-980c-16bfde4c20f8`;
- 7 transclusions;
- access policy `{"visibility":"public","route":"public"}`;
- export policy `{"copy_allowed":true,"download_allowed":true,"formats":["txt","md","html","docx","pdf"]}`.

Markdown export preserved:

- bounded edit phrase;
- Appendix A;
- glossary table header;
- `source:src_aba_formal_op_512`;
- source metadata keys including `source_entities`,
  `source_revision_hash`, and `transclusions`.

Evidence:

- `docs/evidence/source-system-2026-06-06/legal-proposal-bounded-edit-pubf7bae84a8-resolve-20260606T1758Z.json`
- `docs/evidence/source-system-2026-06-06/legal-proposal-bounded-edit-pubf7bae84a8-export-md-20260606T1758Z.json`

## Hard Findings

### Finding 1: Manual Contract Mirroring Remains Residual Risk

Severity: medium.

The backend source contract lives in `internal/sourcecontract`, while the
frontend source contract lives in `frontend/src/lib/source-contract.ts`.
Current behavior is aligned for evidence states, reader artifact states,
selectors, and open surfaces, and focused tests pass. However, the constants are
still manually mirrored rather than generated from one schema.

This is not a newly confirmed behavior problem, because current staging and
focused tests prove the important paths. It is a future drift risk.

Recommended next move:

- create one generated source-contract schema only after the final report is
  accepted, so the refactor does not disturb the now-proven behavior.

### Finding 2: Direct Bookmarklet Fetches Are Not Product Auth-Renewal Proof

Severity: low.

A compact proof collector submitted an extra document-scoped revise from v96 and
then received one `401 authentication required` when polling the owner revision
list, despite `/auth/session` having returned authenticated earlier in the same
script. The accepted product path was not affected, and the frontend product
code uses `fetchWithRenewal` rather than raw bookmarklet `fetch`.

This is a proof-path limitation, not yet a confirmed platform behavior problem.

Recommended next move:

- if auth-renewal concerns matter, reproduce through a product UI/API client
  that uses `fetchWithRenewal`, then document a new problem before fixing.

### Finding 3: Non-public Publication Semantics Are Intentionally Not Exposed

Severity: medium.

The UI now honestly exposes the supported policy: public route, source
snapshots, and copy/download formats. It does not offer private/unlisted
publication controls. That is correct for the current proof state.

Residual risk:

- future unlisted/private publication semantics still need route, reader,
  export, retrieval-source, and guest enforcement proof before UI exposure.

### Finding 4: Source-Service Breadth Is Focused, Not Exhaustive

Severity: medium.

The mission has owner and guest Source Viewer proof for focused URL-backed and
content-item/source-service-style publication records. It does not prove every
future source producer, media target, connector source, or stale/blocked access
branch.

Recommended next move:

- add a generated fixture matrix for source target kind, evidence state, reader
  artifact state, open surface, and publication visibility.

### Finding 5: Deploy Identity Stamp Still Needs Post-success Distinction

Severity: low to medium.

Earlier in the mission, `/health` showed the target commit while a deploy job
was still in progress. Current acceptance pairs `/health` identity with terminal
GitHub Actions deploy status, which is sufficient for this mission. The platform
should eventually expose post-success deployed identity separately from
in-progress target identity.

## Code-Pruning Review

No behavior-changing prune was safe in this pass.

Reason:

- source open-plan behavior now routes through `frontend/src/lib/source-contract.ts`;
- `vtext-source-renderer.ts` still owns source entity shape adaptation and
  rendering helpers, not the generic open-plan contract;
- Web Lens iframe and backend snapshot code still serve explicit live/original
  inspection, not the default durable source path;
- `ContentViewer.svelte` still imports URL content through the policy-checked
  `/api/content/import-url` path when explicitly requested.

Deleting any of these paths now would reduce mission coverage rather than remove
confirmed dead code. The right next prune is schema generation or fixture-matrix
consolidation, after a separate documented problem or refactor plan.

## Residual Risks

- Contract constants are manually mirrored between Go and TypeScript.
- Source Service and connector-like future records need broader fixture
  coverage.
- Non-public publication semantics remain unimplemented by design.
- Direct proof scripts can bypass product auth-renewal behavior and should not
  be used as product-path auth evidence.
- Desktop screenshot capture failed in this environment.
- Docs-only checkpoint `e808a794` is not deployed; deployed behavior remains
  `8efb05a2`.

## Rollback References

- Last deployed behavior commit before the final docs checkpoint:
  `8efb05a25430330ada50e1a2ac6ebe2418af9700`.
- Latest docs/evidence checkpoint:
  `e808a7944f971a1d2e47b87592b644ecd7be22dc`.
- If the publication-policy UI change needs rollback, revert
  `8efb05a25430330ada50e1a2ac6ebe2418af9700` after preserving the mission
  evidence.

## Next Realism Axis

Build one generated fixture/verifier matrix that crosses:

- source target kind;
- reader artifact state;
- evidence state;
- selector kind;
- open surface;
- owner versus guest;
- private versus public publication visibility.

This is the next high-information move because it attacks the remaining drift
risk without weakening the behavior already proven on staging.

