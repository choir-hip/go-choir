# No owner act requires step-up, and the host has no platform-owner principal

Status: open. This record documents the problem first; this commit has no
fix. Mutation class of a fix: red (auth/session, owner acts).

Found 2026-10-22 by reading code during the provider vault design.

## No step-up

- `authorizeOwnerAct` admits any valid cookie session (`case "cookie":
  return true`) or an API key with `owner:acts`
  (`internal/proxy/owner_acts.go:25-39`). It gates approve, arm, restore,
  rematerialize, bootstrap-chain and the lifecycle acts
  (`internal/proxy/self_development.go:179`, `:477-485`;
  `internal/proxy/computer_lifecycle.go:339`, `:342`, `:582`;
  `internal/proxy/releases.go:96`).
- No route requires a fresh passkey assertion. No `UserVerification`
  setting is made, so the go-webauthn default applies
  (`internal/auth/handlers.go:487-565`, `:603-681`). The refresh token
  lasts 720 hours (`internal/auth/config.go:87`, `:112`).
- So a stolen refresh cookie, or script running on the owner's origin
  (`docs/problems/guest-served-desktop-on-owner-origin-without-csp-2026-10-22.md`),
  can perform every owner act.
- Signed owner-act receipts exist, but only three act kinds, all for
  self-development. corpusd verifies them, and the mode is `observe`
  (`internal/owneract/owneract.go:66-78`; `nix/node-b.nix:117-129`).

## No platform owner

- "Owner" always means the owner of one computer. No host-admin principal
  exists. The `admin` API-key scope stays tied to its own user
  (`internal/auth/handlers.go:1557-1580`; `internal/proxy/handlers.go:572`).
- Host-level acts today, such as provider keys, pools and computer tiers,
  are reachable only by SSH or a reviewed commit. That fails standing
  question 9.

## Owner ruling

R1, 2026-10-23: "Yes, just for me." Vault and host-policy acts are
admitted by a committed allowlist holding the owner's user ID. The design
adds a step-up receipt bound to each write
(`docs/provider-vault-and-model-policy-design-2026-10-22.md` §2.5).
Whether existing owner acts should also take step-up is not decided here.
