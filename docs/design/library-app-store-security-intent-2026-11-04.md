# Library as an app store: the security model (intent, 2026-11-04)

Readiness: intent. Not to build yet. Recorded from the owner's
conversation of 2026-11-04 so the direction is not lost; a design follows
once self-development is reliable and app agents are under way (owner
order: loose ends, self-development reliability, app agents, Autopaper).

## The goal

The Library is an app store. Apps are built on one computer, published,
and installed by other users. Installed apps run as a layer (S12), with
the installer's session, so distributing an app is distributing code.
Owner: "realistically, this is the same risk as GitHub." The security
model is essential to the app store, not an add-on.

## Possibilities the owner named (not committed)

1. **Publisher attestation.** The publishing user signs that their app is
   secure, and their agents attest too. A signature is accountability,
   not proof.
2. **Review before install.** A security review runs before any other
   user installs a release. Two places were named:
   - on a platform-level computer, once per release; or
   - on the installer's own computer, before they install it. The owner
     leans this way ("maybe we want to say, no, we don't run the code
     review on a platform machine. We run it on the user's VM before
     they install it").
3. **Enterprise setting.** A deployment can keep third-party installs off
   until a code review has run locally on its own machine.
4. **Precommitment records for apps.** Reuse the precommitment record
   mechanism (docs/precommitment-records.md): attach records to Library
   releases, focused on security questions, and keep them across an
   app's version history. Statistics across installs and versions build
   a track record. A positive track record is evidence, never proof that
   the current version is secure.
5. **Remediation.** If an insecure app is caught, or worse, found after
   it spread: yank or force-update the release (the forced app update in
   the reliability bar), and ding or ban the publishing account.

## What exists today

- Releases are public (2026-11-03); withdraw exists; yank exists in the
  listing code; no signed listings (S11), no namespace signing (S13/S14).
- S12 runtime install runs modules same-origin with the installer's
  session. Since the owner's ruling of 2026-11-04 any signed-in user can
  install any listed release (`cross_owner_before_frame` is removed, no
  switch); the install dialog names the publisher. The sandboxed frame
  (S13) is a planned mitigation, not a precondition.
- The modularity check (9858baf1e) proves an app keeps working across
  platform updates; it says nothing about whether the app is safe.

## Questions for the design

- Where review runs (installer's computer, platform computer, both), who
  pays for it, and what a review record binds (release digest, module
  sha, reviewer model, questions asked, verdict).
- Whether the sandboxed frame (S13) is required for every third-party
  install, or whether review plus attestation can stand in for it.
- How account standing works (who decides, appeal, what a ban removes).
- How a forced update or yank reaches computers that installed the app,
  and what happens to the app's data.
- How the app layer's security model stays separate from the base
  computer's (security epoch, guest surfaces).
