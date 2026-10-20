# Account recovery protector: design for launch (2026-10-20)

Status: design only. No code changed. Mutation class of the eventual work:
**red** (auth and keys). Source ruling: owner notes
[§27 item 13](owner-notes-2026-10-14.md): "Recovery says plainly that it
restores sign-in, not passkey-wrapped data; a recovery key is designed later.
Owner: '#13 requires more work before we can safely launch.' The recovery
protector is a launch blocker." Background:
[account-recovery-token-discarded-2026-10-14.md](../problems/account-recovery-token-discarded-2026-10-14.md)
("Owner root key after recovery", slice 2 item 5) and item thirteen of the
[decisions digest](owner-decisions-digest-2026-10-20.md).

## The short version

I read the code before designing, and one finding changes the problem.
**Today no user data is protected by a passkey.** The passkey-wrapped owner
root key exists on the server, but no browser code asks for it and nothing is
encrypted under it. Every computer's data key is held by the host, in custodian
escrow. So a person who loses every passkey loses no data right now. Recovery
brings back sign-in, and the computer comes with it.

That sharpens the launch blocker rather than shrinking it. It binds the moment
we start protecting data with the passkey, because from then on a lost passkey
can mean lost data. There is also a present problem: the same email link that
restores sign-in hands the account, and the host-held key path behind it, to
whoever controls the inbox. The protector design has to cover both.

Recommendation in brief: say the true thing now, harden the email path, and
build a user-held recovery key before any user-held data mode is switched on for
an outside user.

## 1. The problem, precisely

### How data keys are wrapped today

There are two separate key systems. The ruling treats them as one.

**System one: the per-computer data key (DEK), wrapped by the host.** Each
computer has a random 32-byte key that encrypts its private event history and
file blocks (`internal/computerevent/privacy.go:84-95`, accessor
`ExportKeyForEscrow`). At boot a realization exports that key and uploads it to
the platform, sealed to the host's escrow public key
(`internal/autoputer/run.go:403-413`, `internal/autoputer/key_escrow.go:85`,
`internal/keyescrow/keyescrow.go:128`). Before the first event exists this is
required, because the realization's disk is otherwise the only copy ("O21",
`run.go:339`, `:403`). The host escrow private key is the file `key-escrow-key`
on Node B (`internal/platform/config.go:58`), and the off-host backup plan lists
it as something the backup must include
(`no-off-host-backup-2026-10-20.md`, line 44).

The host uses that escrow in three ways. A new realization of an existing
computer is handed its key automatically, sealed to a recipient key it made,
with the delivery logged first (`internal/platform/key_delivery.go:14-30`,
`:127-133`). The checkpoint worker opens it (`cmd/checkpointd/worker.go:146`).
And a person asking for the raw key goes through a two-approval gate: two
different named operators must approve
(`internal/platform/key_escrow.go:220-229`, `key_escrow_http.go:56`).

So the host can read user data today. The design archive says this on purpose:
"recoverable-by-default… not zero-knowledge by default"
(`docs/archive/choir-durable-substrate-2026-08-23.md` §3.2, §4), and marked it
an owner values decision. I found no record of the owner ratifying it. That is
Question 1.

**System two: the owner root key (ROOT), wrapped per passkey with WebAuthn
PRF.** This is what the ruling is about. One 32-byte key per account:

- At registration the server asks for the `prf` extension
  (`internal/auth/handlers.go:557`). If the authenticator reports PRF enabled,
  the server stores a random 32-byte salt on the credential
  (`handlers.go:837-844`, `store.go:45-46`).
- At login the server asks for a PRF evaluation with that salt
  (`handlers.go:651-670`). If the client sends `unwrap_root`, the server derives
  a wrapping key from the PRF output by HKDF, bound to the account id
  (`internal/keyescrow/keyescrow.go:228-236`), and opens the stored wrap
  (`handlers.go:1037-1088`).
- The first PRF login with `unwrap_root` makes ROOT on the server, seals it
  under that passkey's key, stores the wrap in `owner_root_key_wraps`, and
  returns ROOT to the browser (`handlers.go:1054-1073`, `store.go:124-139`,
  `:560-583`). So the server sees ROOT in memory once.
- One wrap per passkey. A second passkey never gets one. The code says so:
  "owner ROOT exists without this credential wrap" (`handlers.go:1061`), and it
  returns empty.

**What is switched on.** Nothing in the browser sends `unwrap_root`, passes the
`prf` extension to `navigator.credentials.create` or `get`, or includes client
extension results in the finish bodies (`frontend/src/lib/auth.js:118-165`,
`:300-322`). The server's `passkeyPRFEnabled` check (`handlers.go:837`)
therefore never fires. By code reading, **no credential is PRF-capable and no
ROOT exists in production.** I could not check the database from here. The
check is two non-secret counts on the auth database:
`SELECT COUNT(*) FROM credentials WHERE prf_capable=1` and
`SELECT COUNT(*) FROM owner_root_keys`. Both should be zero. Confirm before this
is told to the owner as fact.

Nothing consumes ROOT either. `SealRoot` and `OpenRoot` are called only in the
login handler. The archive said ROOT would wrap the DEKs (§3.2); that link was
never built.

### What recovery restores now

Slice 1 is on a branch. It emails a single-use link to the stored address,
15-minute expiry, three per hour per email
(`internal/auth/recovery.go:74-81`, `:121`). Pressing "Add a new passkey"
registers a new passkey for the same account and signs the person in
(`recovery.go:176-296`, finishing at `/auth/register/finish`): same account,
same computer, same documents.

It restores sign-in. It does not create a ROOT wrap for the new passkey. It does
not revoke other sessions (slice 2 item 4). It does not prove the address ever
belonged to the person, because sign-up never verifies it (slice 2 item 2). And
the new passkey must be a platform authenticator (`recovery.go:237`,
`handlers.go:554`), so a hardware security key cannot be added.

### What is lost if every passkey is lost

Today: none of the data. Sign-in returns by email, the host still holds the DEK,
the computer boots, everything is there. What is lost is only the guarantee that
the email owner is the account owner.

Once ROOT protects data: everything protected by ROOT alone. The wraps open only
with a credential's PRF output, so with every credential gone they are
ciphertext nobody can open, and a recovered account holds a ROOT row with no
usable wrap. An email link must not fix that: if it could, ROOT would be no
stronger than the inbox. So the launch question is **what Choir promises about
who can read a person's computer, and what happens to that promise when they
lose their devices.**

## 2. Threat model

**Choir operator or host compromise.** Root on Node B yields the escrow key
file, the escrow table and the auth database, and so every DEK. The two-approval
gate is an application rule, not cryptography; the archive says so. A user-held
key cannot make a running computer opaque to its host, because the computer must
hold its key in memory. It does help against stolen disks and backups, if the
recovery key never sits where the host can reach it.

**Email account takeover.** The likeliest real attack. Recovery by email plus an
unverified sign-up address means control of an inbox is control of the account:
a new passkey and a signed-in session on the victim's computer. A good protector
makes inbox control insufficient for data the user locked, and makes the attempt
slow and loud enough for the real owner to stop.

**Lost device.** The ordinary case: a dead phone, a stolen laptop, a lost Apple
or Google account. The person is legitimate and has memory and maybe paper. The
design must work for someone who prepared nothing and degrade honestly.

**Coerced recovery.** A user-held key lets Choir truthfully say it cannot
comply; escrow does not. A delay does not stop in-person coercion and I do not
claim it does.

**Insider at the host.** A staff member with shell on Node B can approve their
own recovery by editing the database. The defences are a user-held key that
never touches the host, and a host-side recovery path that is delayed and
visible to the user, not only to operators. The existing transparency log is
operator-facing; the user-facing half does not exist.

Not designed against: a person with no key, passkey, device or email who wants
locked data back. The honest answer is no, told in advance.

## 3. Options

### A. A user-held recovery key

A 256-bit random key made in the browser at setup, shown once as 24 words (the
BIP39 list, which has a checksum) and optionally a QR code for printing. It
never goes to the server. The browser derives a wrapping key and wraps ROOT, the
way a passkey's PRF output does today. The protector column already exists
(`owner_root_key_wraps.protector`; `keyescrow.ProtectorOfflineCapsule` is a
named constant, `keyescrow.go:37`; the archive called it the "owner offline
capsule").

Gives: recovery without Choir. An attacker with the inbox, the host or a court
order cannot open it. The only option where "Choir cannot read it" can be true.

Costs: people lose paper. Support cannot help, by construction. A wrap with no
credential id needs a home (the column is `NOT NULL` and in a unique key, so a
sentinel or a separate table). ROOT must be made in the browser, not on the
server (`handlers.go:1054-1058`), or "never touches the host" is false. The read
path filters `protector = 'passkey_prf'` (`store.go:608-612`) and needs a second
branch.

Honest line: "This is the only way back to what you lock if you lose every
device. Choir cannot reset it."

### B. Escrow with the host (the key-escrow-key)

The host holds a copy of ROOT, sealed to its escrow key, released after email
recovery plus an operator step or timer. The machinery exists for DEKs. It gives
the friendliest recovery: a person with nothing still gets back in. It gives up
the privacy claim. Choir, a rogue operator, or whoever compromises Node B can
read the data retroactively, including from node-a backups once they hold the
escrow key file, and it puts a second high-value secret on the host. Defensible
if said plainly; indefensible beside "only you can unlock your data."

Honest line: "Choir keeps a recovery copy and can open your data under a logged
two-person rule."

### C. Several passkeys, including synced ones

More than one passkey: phone plus laptop, or a platform passkey that syncs
through iCloud Keychain or Google Password Manager. The backend lists, renames
and deletes credentials with a last-credential guard (`recovery.go:316-418`).
Missing: an "add a passkey" route for a signed-in user, the settings screen, and
a wrap per added passkey. It gives the best everyday experience; a synced
passkey comes back with the new phone, no paper.

Cautions. A synced passkey is as safe as its sync account, and two passkeys in
one ecosystem are one backup. PRF on synced passkeys has been uneven across
vendors; I believe current Apple and Google sync support it, but check real
devices first (the archive warned the same, §3.2). The platform-only rule
(`handlers.go:554`) blocks hardware keys, the one truly independent device. And
a new passkey can get a ROOT wrap only while ROOT is unlocked by an existing
passkey or the recovery key, never by email recovery. That is the point, and
must be explained.

Honest line: "A second passkey on another device is the easiest backup. If both
live in one Apple or Google account, that is one backup."

### D. Threshold schemes (Shamir, user share plus host share)

Split ROOT, say two of three: a passkey wrap, a recovery key, a host share; any
two recover it. The archive names Shamir as the cryptographic version of the
two-approval gate. The host alone learns nothing. But it is heavy machinery on a
red surface with no production history, and a person who lost every passkey is
left with the recovery key and the host's share, so the host is back in the
recovery path and the email-takeover question returns. A good later upgrade of
B, not a launch item.

### E. Delayed recovery with notification and a cancel window

A property of the email route, not a key. The link does not finish recovery; it
starts a pending one. Choir emails the account address and shows a notice in
every signed-in session. After the window (72 hours proposed) the person
returns and adds the passkey. A cancel from any live session or the email stops
it and kills the link. It is the best defence against email takeover for an
owner who still has a signed-in device or watches mail.

Costs. A person who lost everything waits three days. An attacker who owns the
inbox can delete the warning. It does nothing for locked ROOT. And **the
repository has no push infrastructure** (a search for web push and APNs finds
none), so at launch the second channel is the in-product notice. Push is a later
station and the copy must not imply it.

Honest line: "For your safety, a new device is added only after 72 hours. We
will tell you, and you can cancel."

## 4. Recommended design for launch

Layers, each landable and provable alone.

### Layer 0: say the true thing now (green)

Words only, no behaviour change; exact copy in section 8. Because nothing is
passkey-wrapped today, the true sentence is not "recovery does not restore
passkey-wrapped data". It is that recovery restores sign-in and the computer,
and that Choir holds the key to the computer's storage. One line gets edited
when user-held mode ships.

### Layer 1: harden the email path (red)

Mostly named already in slice 2.

1. Verify the email at sign-up (slice 2 item 2). No recovery for a never-proved
   address.
2. Delayed recovery: 72 hours, cancel from any live session or a second email
   link. One email at start and one 24 hours before completion; a banner in
   every signed-in session. No push at launch.
3. Revoke other sessions when recovery completes (slice 2 item 4).
4. Mark email-authenticated sessions weaker than passkey sessions: they cannot
   add or delete passkeys, change the email, make a recovery key or unwrap ROOT
   without a fresh passkey (slice 2 item 1).
5. Canonicalise email case (slice 2 item 6).

### Layer 2: second passkey and recovery key (red)

In this order:

1. Add a passkey while signed in (begin route, settings screen, AAGUID naming).
   Allow cross-platform authenticators by changing `SelectAuthenticator` at
   `handlers.go:554` and `recovery.go:237`.
2. Wire PRF end to end in the browser: pass `prf` to create and get, send
   extension results at finish, request `unwrap_root`, hold ROOT in memory only.
3. Move ROOT creation to the browser. The server stops generating ROOT; the
   client wraps it and uploads only wraps. Without this, "Choir never sees your
   key" is false.
4. A signed-in, unlocked session adds wraps for each new passkey and for the
   recovery key. This turns a second passkey into a real backup instead of the
   empty state at `handlers.go:1061`.
5. The recovery key: 24 words, browser-made, three words confirmed by typing,
   optional QR. Stored as a wrap with `protector = 'recovery_key'`.
6. Recovery completes in two parts. Email recovery (with its delay) restores
   sign-in. Restoring ROOT needs the recovery key or an existing passkey. With
   neither, sign-in returns, locked data stays locked, and the screen says so
   from the actual wrap status.

### Layer 3: the onboarding requirement

Before an account counts as set up, require a saved recovery key or a second
passkey on another device. Default to the recovery key (no second device or
shared ecosystem needed), with the second passkey offered beside it. A person
may skip; the skip is stated plainly and a banner stays until done. Make it
**required** the day any user-held data mode is on for an outside user, and
strongly nudged before: while the host holds the DEK a skipper loses no data,
and after it does, skipping is a path to permanent loss.

### Host escrow: opt-in for ROOT, operational for the DEK

I recommend **no host copy of ROOT by default.** A "managed" tier for people who
will not hold a key is an explicit toggle with the trade on its label, and should
be a threshold share (option D) built after launch.

The existing DEK custodian escrow is different, and I recommend keeping it. An
always-on computer must restart, update and be rebuilt while its owner sleeps.
Removing the host's ability to deliver the DEK means the computer boots only
when the owner signs in and sends the key: a "sovereign" mode. It is real and
valuable and also black-class, because deleting a custodian wrap is irreversible
(archive §3.2). Not a launch item. Platform computers like the paper's keep the
escrow regardless.

So the honest scope of a user-held ROOT at launch is narrow: the owner's own key
for things Choir should not open (exports, locked backups, future sovereign
data). It does not make a running computer opaque to the host, and the privacy
page must not suggest it does. This is the main honesty risk in the design.
Not for launch: raw ROOT escrow by default, threshold schemes, push as a
requirement, email-only recovery for locked data.

## 5. Migration for existing accounts

Existing accounts have passkeys never PRF-registered, no ROOT, no wraps, and a
host-held DEK. Nothing to re-wrap.

1. Confirm the two counts (section 1) on staging and production. Any non-zero
   rows are test accounts, handled by hand.
2. Existing passkeys cannot become PRF-capable in place; capability is decided
   at creation (`handlers.go:837`). The person adds a new PRF-capable passkey
   while signed in: "Add a passkey that can lock your data".
3. First sign-in after the feature ships offers recovery key setup in a
   dismissible card, then a banner. ROOT is created in the browser then, wrapped
   by the new passkey and the recovery key in one server transaction that stores
   only wraps.
4. No DEK is rewrapped; the custodian wrap stays. A ROOT wrap of a DEK, when
   added, is an extra keyslot written when the owner next unlocks, never a
   replacement (the archive's "lazy per-boot, not big-bang" clamp).
5. Platform accounts (the paper, the owner's computer) stay custodial. The
   owner's account goes first, with the restore drill proven before any outside
   user. Old recovery links expire themselves in 15 minutes.

## 6. Failure modes, class, rollback, proof

### Failure modes (written before code)

1. ROOT or the recovery key reaches a log, response body, mail store or crash
   report. The key is made and used only in the browser; the server never
   receives it. Review every log line near the new routes.
2. The server still generates ROOT (today's behaviour), making the user-held
   claim false. The build deletes server-side generation; it does not add a
   flag.
3. A wrap is written that its credential cannot open, or a typo is wrapped as
   the key. The salt check at `handlers.go:1079` stays, and confirmation proves
   the key by unwrapping the new wrap in the browser before the screen closes.
4. A second passkey is added with no wrap and looks like a backup. Settings
   shows each passkey's wrap status.
5. The delay is bypassed by an early redeem or clock fault. The server stores a
   completion time and refuses earlier, tested with a fake clock.
6. An attacker who owns the inbox silences the cancel window. Open-session
   notices help; the residual is honest: a person with no other session and a
   hostile inbox is not protected by the delay.
7. Support is asked to reset a recovery key. The answer is no, and the support
    copy says why. If any operator tool can, the design has failed.
8. Backup coupling: node-a holds ciphertext, and the escrow key file is in the
    backup set. Anyone with both reads all custodial computers. The backup key
    must stay apart from the escrow key; a standing condition from
    `no-off-host-backup-2026-10-20.md`.

### Mutation class

**Red.** Protected surfaces: auth and session renewal, recovery request and
verify, credential registration, the key escrow table and private key, and the
first user key material in the browser. Layer 0 is **green**. PRF end to end
and moving ROOT creation to the browser is red, and because of the claim it
makes it deserves an agentic-consensus panel.

Heresy delta: **discovered** that the passkey-wrapped root key is referenced by
no browser code and wraps nothing, so the problem doc's "data keys are wrapped
per passkey" describes a plan, not a deployed fact; **discovered** that email
control is account control with no delay; **introduced** none; **repaired**
none yet.

### Rollback

Layer 1: revert the commits; the delay is a state column on the token table and
the immediate behaviour returns. Layer 2: new wraps are additive rows, and
turning the flag off stops ROOT creation and leaves rows inert. **Never delete
wrap rows on rollback**: a wrap made for a real user's ROOT may be the only
copy. The recovery key has no rollback by design, which is why the flag stays
off until the restore drill passes.

### Staging acceptance proof

On `https://choir.news`, throwaway account, real inbox.

1. Auth reports the landed commit; PRF-capable credential and ROOT counts taken
   before and after.
2. Create account, save the recovery key, confirm three words. The server holds
   a recovery wrap and no plaintext; auth and maild journals contain neither the
   words nor key material (grep count zero).
3. Lose-everything drill: delete every passkey on the test authenticator, start
   email recovery. No sign-in until the window ends (a test-only short window by
   config, never production); a notice appears in a signed-in session; cancel
   works.
4. Complete after the window; sign in; the screen says data is locked. Enter the
   recovery key; ROOT unlocks; the same data decrypts. Other sessions were
   revoked.
5. Negative: a wrong key fails closed; no key leaves sign-in restored and data
   locked.
6. The owner runs the drill on their own account with a real phone before any
   outside user.

## 7. Questions for the owner

Answer by number. Recommendation after each.

1. Do you ratify, as a values choice, that the host keeps a custodial copy of
   each computer's key so it can boot and rebuild it, and that Choir staff can
   technically read user data under the two-person rule? **Recommend yes, said
   plainly on the privacy page, with a sovereign mode as a later election.**
2. Is the launch blocker cleared by saying the true thing and hardening the email
   path, with the recovery key required only the day a user-held data mode turns
   on for outside users? **Recommend yes.**
3. Is the recovery key 24 words generated in the browser, never sent to Choir,
   with no support reset? **Recommend yes.**
4. Does onboarding require a recovery key or a second passkey, with a skip that
   leaves a banner? **Recommend yes.**
5. Is email recovery delayed 72 hours, cancellable from any live session or
   email link, with a notice in every signed-in session? **Recommend yes.**
6. Do you accept no push notification at launch (none exists), with the
   in-product notice as the second channel? **Recommend yes; plan push later.**
7. May we relax the platform-only rule so a hardware security key can be a
   second passkey? **Recommend yes.**
8. Does Choir verify the email address at sign-up before recovery is offered for
   it? **Recommend yes.**
9. Does the host hold no copy of ROOT by default, with any "Let Choir help me
   recover" mode a later, explicit, threshold-share option? **Recommend yes.**
10. Is it acceptable that a person with no passkey, no recovery key and no second
    device gets sign-in back but not locked data, told so in advance?
    **Recommend yes.**
11. Does your own account go first through the full lose-everything drill, with a
    printed key kept offline, before any outside user? **Recommend yes.**
12. May the section 8 copy ship now as a green change, true today, with one line
    edited when user-held data turns on? **Recommend yes.**

## 8. The immediate small slice: the plain statement

Not implemented. A copy-and-placement proposal for a green change.

### Where the recovery UI lives

- `frontend/src/lib/AuthEntry.svelte:242-247`: the "Lost your passkey?" button
  on the sign-in view.
- `frontend/src/lib/AuthEntry.svelte:251-254`: the recovery view, titled
  "Recover your account", holding the form component.
- `frontend/src/lib/RecoveryRequestForm.svelte:22`: the lead sentence ("Enter
  the email address on your Choir account. We will send a link that lets you
  add a new passkey."), and `:81-95` the "Check your email" screen.
- `frontend/src/lib/RecoverRoute.svelte:116-138`: the landing page after the
  link, and `:151-157` the "You are back in" panel.

### A finding to read first

The ruling says to state that recovery "restores sign-in, not passkey-wrapped
data". Today no data is passkey-wrapped, so that sentence would warn of a risk
that does not yet exist and say nothing of the one that does: Choir holds the
key to the data. A statement should be true the day it ships, so I propose the
wording below. If the owner prefers the original sentence, it should wait until
user-held data is on.

### Proposed copy and placement

1. **Request form**, a muted second paragraph under the lead
   (`RecoveryRequestForm.svelte`, after the lead, `class="muted"`):

   > Recovery gets you back into your account and your computer. Today Choir
   > holds the key to your computer's storage, so recovering does not lose your
   > data. If you later lock data with a key only you hold, that data will need
   > your recovery key, and recovery by email alone will not open it.

2. **"Check your email" screen**, one added line after the 15-minute sentence
   (`RecoveryRequestForm.svelte`, after line 85):

   > A recovery link adds a new passkey so you can sign in. It cannot open
   > anything you have locked with your own key.

3. **"You are back in" panel**, after the body at `RecoverRoute.svelte:154`:

   > You can sign in again, and your computer is here. Your other passkeys keep
   > working. If you locked data with a recovery key, you will need that key to
   > open it.

   The last sentence is true for no account today. Show it only when the root
   status endpoint (`GET /api/auth/root/status`, `cmd/auth/main.go:69`,
   `handlers.go:1096`) reports a ROOT. Until then show the first two sentences
   only.

4. **Sign-in view**: no change. Legal-sounding text there would crowd the one
   action the person came for.

Tone matches the existing screens: short, second person, no jargon. PRF, wraps
and ROOT appear nowhere a user will see. Give the new paragraphs
`data-recovery-note` so a regression spec can assert the statement is present;
existing `data-recovery-*` and `data-recover-*` selectors are untouched.

## What I did not do

I did not edit code, run any server command, or read or print a secret. The
production counts in section 1 are unverified and are the first thing to check.
