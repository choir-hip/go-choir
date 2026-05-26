# Public Identity And Custom Domains

**Status:** roadmap direction; not the current implementation mission
**Last updated:** 2026-05-14

Choir's public identity model should not privilege any particular person or
account. A user's public surface is addressed by a handle they choose, and later
by any custom domains they prove they control.

## Route Model

Default platform routes:

```text
choir.news              -> public platform computer surface
choir.news/:handle      -> public surface for a user-selected handle
```

Custom domain routes:

```text
mosiah.org                -> public surface selected by the verified domain owner
www.mosiah.org            -> optional alias to the same selected surface
```

The default route is the platform-level public surface. User routes are ordinary
account/computer bindings. There is no special `/yusef` rule or identity-specific
exception; `yusef` would only be a handle if some account claims it.

## Handles

Handles are product identity, not auth identity.

- A user may have multiple accounts during testing.
- A handle belongs to one account or organization at a time.
- Handle choice, transfer, release, and rename should be explicit product
  actions.
- Route lookup should resolve a handle to a published computer/publication
  target, not directly to a private active computer.
- Mutation of the public surface still requires auth and should happen through a
  candidate or active personal computer, then publish/promote.

## Custom Domain Binding

Custom domains are aliases to a selected public target.

Minimum binding record:

```text
domain
owner account/org
verification method and token
verified_at
target kind and target id
primary/alias policy
tls status
created_at / updated_at
revoked_at
```

Verification should support at least:

- DNS TXT challenge, preferred for apex domains;
- HTTP challenge under `/.well-known/choir-domain-verification`, useful when the
  user already controls a web host;
- later registrar/provider integrations where useful.

Routing should use the request host header to resolve verified domains before
falling back to `choir.news` path routing.

## TLS And Operations

The serving layer should eventually provision TLS automatically for verified
domains, likely through ACME at the edge/reverse proxy layer. Domain activation
should not point public traffic at an unverified or failing target.

Operational states should be explicit:

```text
pending_verification -> verified -> activating_tls -> active
active -> suspended/revoked/expired
```

Failed verification, expired DNS, certificate failure, or target deletion should
fall back to a safe public error page, not to another user's surface.

## Publication And Mutation Boundary

Custom domains serve public projections:

- published personal desktop/newspaper surface;
- selected published VText/document surface;
- later radio/feed projections;
- later app/package public pages.

They do not grant anonymous mutation. Anonymous users may read public surfaces.
When they attempt to mutate, Choir should ask them to register or log in, then
create or resume a user-owned active/candidate computer for the mutation path.

## State Placement

The current target is a platform Dolt microservice for platform-visible identity
and routing facts:

- handles;
- domain bindings and verification records;
- public route targets;
- publication records;
- citation graph and transclusion metadata;
- compute/accounting facts needed for later economics.

SQLite may cache hot lookups, but canonical public identity and domain state
should not be trapped in per-runtime SQLite.

## Non-Goals For The Current Slice

Do not implement custom domains during the public-desktop/auth-on-mutation
mission. Keep this as route-design pressure so the current access model does not
hard-code assumptions that block custom domains later.
