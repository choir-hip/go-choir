# Alternate Computer UX Experiment Portfolio Certificate

Date: 2026-05-20

Status: `checkpoint_incomplete`

This certificate records the first clean owner-pullable alternate-computer
experiment portfolio after the AppChangePackage hard cutover. It is not a
platform-default UX merge and it is not proof that the experiments should be
promoted unchanged. It proves that four experiments can be expressed as
reviewable source packages, pulled into recipient computers, built as
recipient-specific runtime/UI artifacts, verified, promoted in the recipient
context, and rolled back by named refs.

## Platform Proof Context

- Deployed staging commit: `575ff3014a85524da4233e60ce44345804d46807`
- GitHub Actions run for `575ff30`: `26187590374`, passed
- Node B deploy identity: proxy and sandbox both reported
  `575ff3014a85524da4233e60ce44345804d46807`
- Recovery hardening checkpoint: `664dc1b7949e852705daebd2c3f94416e61733ab`
  landed and deployed after this certificate's experiment proof. GitHub
  Actions run `26193426970` passed, including Node B staging deploy, and
  `/health` reported proxy and sandbox on `664dc1b`. This added bounded stale
  terminal worker/candidate VM-state reclaim under state-dir pressure; it does
  not change the selected experiment package refs below.
- Proof environment: `https://draft.choir-ip.com`
- Auth/disk recovery during proof:
  - failure: `/auth/register/begin` returned `502` because
    `go-choir-auth.service` crash-looped after Node B root disk reached 100%
  - root cause: accumulated old Nix generations plus VM state images
  - immediate recovery: journal vacuum plus deleting NixOS system generations
    older than 3 days and running `nix store gc`
  - result: auth registration returned `200`; root free space recovered to
    about 168 GB before the fresh Wave 2 proof and remained about 156 GB after
    it

## Review Model

Direct owner login to source experiment accounts is not the review path. The
review path is package mobility:

```text
experiment computer publishes AppChangePackage
-> owner inspects package/Trace/VText/run-acceptance evidence
-> owner pulls/adopts/promotes into an owner-controlled computer
-> owner decides iterate, abandon, or promote
```

The generated source and recipient accounts in these proofs are product-path
test accounts. The owner-facing handoff is the package/adoption packet below.
The owner does not need credentials for those source accounts. The practical QA
path is to pull selected package refs from this certificate into an account the
owner already controls, such as `ymnath@choir-ip.com`, then adopt, inspect,
iterate, promote, or reject there.

The same owner-controlled computer can be the review hub for multiple
experiments. Each package should enter that computer as its own candidate or
adoption attempt with distinct source refs, artifact digests, verifier results,
and rollback refs. Source-account login is not required for owner review unless
the package pull/adoption path itself fails and the root cause is auth.

## Wave 1: Chiron And Animation

Evidence packet:
`test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/alternate-portfolio-wave1-evidence.json`

Screenshot:
`test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/alternate-portfolio-wave1-source-desktop.png`

Trace:
`test-results/alternate-portfolio-wave1-deployed-575ff30-runacc-20260520T202754/trace.zip`

VText:

- doc: `1d74a744-23be-4c07-8357-54beea5010ab`
- revision: `08456a8d-9ca3-48b8-bd9d-7f98c4d1cdfc`

### Chiron Shelf Observability

- Status: `owner_pullable_experiment`
- App: `portfolio-chiron-shelf-alt-portfolio-wave1-1779308875528`
- Package: `28433c19-5d02-416f-9368-de56390e1927`
- Manifest: `ff72e7f90a5d32f5cbb6a1e1f181c68b5af721ebab48dda1946baaeb2df2eecb`
- Trace: `alt-portfolio-wave1-1779308875528-chiron-a50d2c9`
- Source acceptance: `runacc-a352091712fdd96aa00d`, `export-level`, accepted
- Recipient acceptance: `runacc-c3d70f753b81fd591442`, `promotion-level`, accepted
- Adoption: `adoption-owner-review-chiron-alt-portfolio-wave1-1779308875528`
- Runtime digest:
  `sha256:9a72bd1fe32ba54fd83eeeead73dd41a3302654d710ddb9e5e2d647b7dcc62ee`
- UI digest:
  `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`
- Rollback ref:
  `refs/computers/owner-review-chiron-alt-portfolio-wave1-1779308875528/active-foreground-tail-alt-portfolio-wave1-1779308875528`
- Recommendation: iterate; package crossed into a recipient computer with
  build/adoption evidence.

### Process/Window/Agent Animation Language

- Status: `owner_pullable_experiment`
- App: `portfolio-animation-language-alt-portfolio-wave1-1779308875528`
- Package: `98b98c73-eef0-4a88-a6f5-b7dfe695be09`
- Manifest: `8336ee42b4940a26a647c29d57a32b3107f0df473988675f0aa5c73a34882228`
- Trace: `19125861-841b-40a6-be0c-3bf64bb2f8ea-alt-portfolio-wave1-1779308875528`
- Source acceptance: `runacc-5784f0028b01753ad0ca`, `export-level`, accepted
- Recipient acceptance: `runacc-3b54c9ae8dac2337184a`, `promotion-level`, accepted
- Adoption: `adoption-owner-review-animation-alt-portfolio-wave1-1779308875528`
- Runtime digest:
  `sha256:4127a692054045e9a1362e941d387a52352ac4d71dc20384c892376eafbc484e`
- UI digest:
  `sha256:c1ce98da0c2f203160b63c1c66a45467234fc086834eb3546ffe07cbc5c9e271`
- Rollback ref:
  `refs/computers/owner-review-animation-alt-portfolio-wave1-1779308875528/active-foreground-tail-alt-portfolio-wave1-1779308875528`
- Recommendation: iterate; package crossed into a recipient computer with
  build/adoption evidence.

## Wave 2: Liquid And Python

Evidence packet:
`test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/alternate-portfolio-wave2-evidence.json`

Screenshot:
`test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/alternate-portfolio-wave2-source-desktop.png`

Trace:
`test-results/alternate-portfolio-wave2-deployed-575ff30-runacc-20260520T212756/trace.zip`

VText:

- doc: `12bf4059-5036-47fd-9209-053729d80055`
- revision: `c5b9ed96-83e6-4d01-acd0-763917d35e2a`

### Choir Liquid Material Engine

- Status: `owner_pullable_experiment`
- App: `portfolio-liquid-material-alt-portfolio-wave2-1779312477616`
- Package: `1dad3dfc-7f83-4b22-bfb5-7f1714159f66`
- Manifest: `707d28c0e0408dcab8ff3d7efa77935f7ae2ec1e06421f2e03d4e8693cf05c0e`
- Trace: `alt-portfolio-wave2-1779312477616-81e9506c7030`
- Source acceptance: `runacc-0194bfce2cdecffea784`, `export-level`, accepted
- Recipient acceptance: `runacc-d144087c5ffacad2e147`, `promotion-level`, accepted
- Adoption: `adoption-owner-review-liquid-alt-portfolio-wave2-1779312477616`
- Runtime digest:
  `sha256:1031aeb7c1d53c73077fa945661c6993c0aa9b14c3db82c7bb01ade33bde5ae3`
- UI digest:
  `sha256:e09ca2307c8e0aa0b38ec5509fe50a243d19b8c7fe0482c06377101f604d79c5`
- Rollback ref:
  `refs/computers/owner-review-liquid-alt-portfolio-wave2-1779312477616/active-foreground-tail-alt-portfolio-wave2-1779312477616`
- Recommendation: iterate; package crossed into a recipient computer with
  build/adoption evidence.

### Python Code Mode A/B

- Status: `owner_pullable_experiment`
- App: `portfolio-python-code-mode-alt-portfolio-wave2-1779312477616`
- Package: `f31edbc8-1b43-44f5-82a1-834dce4833ca`
- Manifest: `1ec8f96baa00f14062c024b3982b876b787c7353c96cc470d0b5274c42215cbb`
- Trace: `alt-portfolio-wave2-1779312477616-ee3503f67`
- Source acceptance: `runacc-a7e993d7c4f56d4420d9`, `export-level`, accepted
- Recipient acceptance: `runacc-45495b8caebc3e1b82c5`, `promotion-level`, accepted
- Adoption: `adoption-owner-review-python-alt-portfolio-wave2-1779312477616`
- Runtime digest:
  `sha256:d0f5ab65f52b6df2e03db25bb68d84b1535a6f108db8d1ce00c480473da2d6d4`
- UI digest:
  `sha256:b5cc68456c76598faa7d267f546ded558531cbd114e0a94cde2f3c445aa81519`
- Rollback ref:
  `refs/computers/owner-review-python-alt-portfolio-wave2-1779312477616/active-foreground-tail-alt-portfolio-wave2-1779312477616`
- Recommendation: iterate; package crossed into a recipient computer with
  build/adoption evidence.

## Residual Risks

- The four packages are reviewable and owner-pullable, but they are experiment
  artifacts, not platform promotion candidates ready for default deployment.
- Liquid evidence includes package/build/adoption proof and benchmark hooks, but
  the VText context explicitly says complete mobile Safari, frame-time, resource
  cost, and heavy-window benchmark evidence still needs direct inspection.
- Python evidence includes a candidate profile-family implementation and a
  benchmark note/table, but not a completed measured A/B run across comparable
  bash/Python task sets.
- Package-trace run-acceptance remains noisy: source and recipient acceptance
  records are the terminal evidence; package-trace acceptance fields are still
  `docs-level`/blocked or absent.
- VM-state disk pressure was resolved operationally by Nix/journal cleanup, and
  first product-safe stale candidate/worker VM-state reclaim has since landed at
  `664dc1b`. It still needs observation under the next large portfolio-style
  run, and Nix generation/journal cleanup remains a separate operational axis.
- Owner pull/adoption into `ymnath@choir-ip.com` specifically remains the manual
  QA step. The proof used generated recipient product accounts and durable
  package refs.

## Next Realism Axis

Pull the four package refs into an owner-controlled computer and run hands-on
QA there, preferably as separate candidate/adoption attempts inside
`ymnath@choir-ip.com` or another account the owner already controls. Do not
spend effort making the generated source accounts directly loginable. For
Liquid and Python specifically, add measured benchmark evidence: mobile
Safari/WebKit and desktop frame/resource numbers for Liquid; matched
bash-vs-Python task-set token/time/tool-loop metrics for Python. In parallel,
observe the deployed stale candidate/worker VM-state reclaim control during the
next package portfolio run so Node B disk recovery remains evidence based.
