# G4 Fleet Cutover Blocker — Recovered Owner State Is Not Yet an Immutable Input

**Observed:** 2026-07-17 while refreshing the frozen G4 fleet packet against deployed staging commit `e6fa53f10db3ba9499175d7a1d7912a0cbe2f876`.

## Mutation and authority

- Mutation class: **red**.
- Substrate: immutable `ArtifactProgramRef` construction input and fleet cutover authority.
- Protected surfaces: owner `yusefnathanson@me.com`, recovered legacy state, platform blob/input stores, production materializer, and G4 fleet authorization.
- Governing gate: `G4-frozen-deployed-cutover-packet`; no owner detach or fleet CAS is authorized.
- Heresy delta: `discovered: 1`; `introduced: 0`; `repaired: 0` at this checkpoint.

## Problem

The bounded read-only Phase-A extraction recovered and independently read back useful owner state:

- Texture SQL export: 61 MiB, SHA-256 `f0e9f62f4571408d997cd795c1f266ff9178a927757bf11fa36f8591a4875eba`;
- VText SQL export: 434 MiB, SHA-256 `5a99adf4e1aff3f3600da3ce0f1e6794879e01d5de12a5cfbf515dea2e05cf25`;
- actor recovery DB: 24 KiB, SHA-256 `82cdd14b36786e010cbce82e71cee17950b0ec483eabb5ad91a7fe2333733e6e`;
- user-visible `/files` tree: 92 MiB;
- prompt defaults: 60 KiB.

`docs/evidence/audited-construction-phase-a-2026-07-16.md` explicitly records these payloads as recovery-only until a typed immutable authority pins them. The deployed baseline ArtifactProgram `artifact-program:sha256:c106eb2c6dd72097e27754ba28ae9cb32bd962adca63fe973ebb906ac3ce824d` contains only the synthetic control journal. It does not name the recovered owner payloads.

The owner-settled Definition permits incomplete recovery, but requires any independently verified recovered state selected for retention to be included in the owner's `ArtifactProgramRef`. Reusing the synthetic baseline unchanged for the owner would silently omit evidence that was successfully recovered and would contradict that explicit selection rule.

## Required repair before G4 can freeze

Create one owner-specific immutable Base-journal artifact program in the existing platform input stores. It must:

1. include the two verified SQL exports, actor recovery DB, prompt defaults, and regular user-visible files selected from the bounded extraction;
2. omit tool caches, runtime binaries, generated build artifacts, raw Dolt working directories, and other realization-local acceleration state;
3. store every selected file as a content-addressed blob and bind the exact ordered journal entry tape to one new `ArtifactProgramRef`;
4. retain the deployed immutable CodeRef unchanged;
5. construct a fresh disposable owner candidate through the production materializer from that ComputerVersion without reading the old image;
6. independently verify exact generated-file/blob observations, boot/readiness, bounded sparse allocation, and input readback;
7. prove deletion and reconstruction from only the immutable refs; and
8. freeze the resulting owner ComputerVersion and receipts into the complete G4 packet.

The extraction source remains read-only evidence and never becomes a constructor input. The constructor may read only the new immutable journal/blob/program stores. If a selected payload cannot be safely represented, record its explicit omission rather than copying an opaque realization.

## Rollback

Before any route transition, rollback is deletion of the newly constructed disposable candidate and retention of the immutable content-addressed artifacts as unreferenced evidence. The legacy owner ownership and image remain untouched. After a future accepted G4 cutover, rollback must use the signed bootstrap rollback-to-absence and exact legacy restore receipts already proven on the disposable fixture.
