# A second factor for sensitive acts, from the Autoradio app

Status: owner direction, recorded 2026-11-09. Not designed in detail, not
built. Mutation class when built: red (auth and session).

## What the owner said

"The situation that I'm imagining for production is that I'm going to have
the Autoradio app on my iPhone, and we're going to connect it with
two-factor authentication. You know, like the Google Authenticator kind of
thing for all these different changes. And also encourage users to have
that set up in addition to passkeys for their computers. Maybe not
mandatory for them."

## What it means

1. **Operator acts need a second factor in production.** The acts built on
   2026-11-09 (the paper's model policy, and next the fleet model policy)
   are admitted today by a passkey session or an unbound key of an account
   on the operator roster. Before launch, each write of that kind also
   needs a fresh confirmation from the owner's phone.
2. **Users are encouraged, not required, to add it** beside their
   passkeys, for the sensitive acts on their own computers (publishing,
   self-development approval, sign-in from a new device, key creation).
3. **The Autoradio app is where it lives.** The mobile plan
   (docs/archive/reports/autoradio-mobile-app-plan-2026-10-14.md) already
   makes approvals "one tap plus Face ID" on the phone. The Autoradio
   mission (autoradio-mission-handoff-2026-11-09.md) builds the named,
   revocable device (lane S1) and the push relay this needs.

## Options, cheapest first

- **Time-based codes (the authenticator kind).** A shared secret enrolled
  once; the app, or any authenticator app, shows a six-digit code; the CLI
  or the page sends it with the act. Works with the CLI today
  (`--code 123456`), needs no push service, and works before the app has
  a paid developer account. A code can be phished and replayed within its
  window, so it is a second factor, never the first.
- **Signed approval from the app.** The act waits; the app shows what is
  being changed (the diff, the computer) and signs an approval with a
  device key behind Face ID. Not phishable, and the owner sees what they
  approve. Needs push delivery, so it waits for the app's M-stage with
  notifications.

Likely order: codes first, because they unblock the CLI path and need no
app release; the signed approval replaces them for whoever has the app.

## What this changes now

Nothing in the code landing today; this is prerelease with no users
(owner, 2026-11-09). Two things are kept open so the factor can be added
without reshaping:

- operator acts go through one admission function in the proxy, so a
  step-up check has one place to go;
- the receipt on the target's tape has room for "how it was confirmed".

Open: which acts count as sensitive for an ordinary user; recovery when
the phone is lost (a passkey session plus a waiting period is the usual
answer); whether a key-admitted operator act is allowed at all once the
factor exists.
