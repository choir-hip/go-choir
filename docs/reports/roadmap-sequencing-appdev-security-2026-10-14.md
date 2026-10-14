# Sequencing the App-Development Metamission, Auth and Security

Date: 2026-10-14. Owner direction (in session): "the rest of the supervised
app dev meta mission really should be committed and prioritized. And we
should even think about when we're going to sequence in going deeper into
security. I think we need to make a better auth system. I don't think that
passkeys work for everyone. I don't even think we even have passkey
recovery. And then [...] once we get self-development working, we have a
whole agentic red teaming mission to do. Although [...] before we're ready
for the agentic red teaming, we're going to need to scale the
infrastructure so that we're not just running on one machine."

Status: proposal; the order below is for the owner to ratify.

## What exists today (checked in code)

- **Auth is passkeys only.** Registration and login are WebAuthn
  (`internal/auth/handlers.go`). API keys exist for programmatic access.
- **Passkey recovery is half built and does not work.** The backend has an
  email magic-link recovery (`internal/auth/recovery.go`, routes in
  `cmd/auth/main.go:77-78`): rate-limited, hashed single-use 15-minute
  tokens, enumeration-safe, and verification starts a new passkey
  registration. But `HandleRecoveryRequest` creates the token and
  **discards it** (`_, err = h.store.CreateRecoveryToken(...)`, about line
  152): no email is ever sent. The frontend has **no recovery screen**. A
  user who loses their passkey device cannot get back in.
- **S1 security floor is half done.** S1a (host boundary: network
  isolation, authority binding) is complete on deployed evidence; the
  runtime identity, token scrubbing and capsule/Yaegi floor remain
  (`docs/definitions/choir-appdev-s1-security-floor-2026-10-01.md`).
- **One host.** Everything runs on Node B; SP (second host, off-host
  copies, failover) is designed and deferred
  (`docs/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md`).

## Proposed order

The owner's priorities stay first: Autopaper scaling, and the data model
(object graph on Dolt) throughout.

**Phase 1, now: Autopaper scales.** L2a (synthesis intake, enabled on the
paper), management delegation (S2a building), LR cost levers, LU guest
updates (station being designed), the capsule Go module proxy (deploying).

**Phase 2: auth and account (new station LA), small and early.** It is
user-facing and blocks onboarding anyone beyond the owner, so it should not
wait for self-development.
1. Make recovery real: send the magic link through maild, add the recovery
   screen, and prove a lost-passkey recovery end to end.
2. Sign-in for people passkeys fail: an emailed one-time code as a login
   (not only recovery), so a phone, a shared computer or a browser without
   passkey support still works.
3. Several passkeys per account (add a device while signed in), visible
   and revocable sessions and keys.
4. Optional later: OAuth (Google, Apple, GitHub) as another way to prove an
   email, and TOTP or backup codes for owners who want them.
5. This is also where "your Choir account as the login for apps" (the app
   vision) starts: one account service other windows can trust.

**Phase 3: self-development (the app-development metamission).** S1
remainder, then S4 capsule open world (the module proxy is part of its
egress story), S5 live preview, S6 commit gate, Gate 2 (supervised
self-development), then LP (S7 app packages, existing apps first in S7-0).

**Phase 4: infrastructure (SP), before red teaming.** Second host with
off-host copies of every durable home and a log/metric sink (SP0),
zero-downtime deploys (SP1, partly done in L1), failover (SP2). Red teaming
on a single host risks the only copy of everything, and a red team should
be able to take a host down without taking Choir down.

**Phase 5: agentic red teaming (new station LT).** After Gate 2 and SP2.
Agents attack disposable targets: ephemeral forks (S9: fork re-keying and
isolation first) of a computer, the capsule boundary, egress, auth, the
gateway, publication and adoption. Findings become problem docs and
fixes through the normal gates. It needs S9's isolated forks so the red
team never touches a real computer's data.

**Phase 6: the rest of the app-development metamission.** S8 source
publication and adoption, S9 forks and fleets (pulled forward partly for
LT), S10 org templates, S11 mainline and security pushes across a
divergent fleet.

## Why this order

- Auth is cheap relative to its effect and blocks any second user; the
  broken recovery is a real lockout risk for the owner today.
- Self-development comes before red teaming because the red team's
  findings are fixed through the same gated path, and agentic red teaming
  of self-development needs self-development to exist.
- Infrastructure comes before red teaming for blast radius and
  resilience, as the owner said.
- App packages come after the commit gate (S6), which is their
  activation path.

## Open decisions for the owner

1. The order above (especially auth before self-development).
2. Which extra sign-in methods: emailed one-time code (recommended first),
   OAuth providers, TOTP/backup codes.
3. Whether LT's first target set includes the live paper's host services
   or only forks.
