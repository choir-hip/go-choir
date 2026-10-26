# Design review: numbered addresses and autopaper@ newsletter ingest (2026-10-26)

Read-only review at `204b66ff5`. Nothing was edited, deployed or sent.

## Verdicts

**Numbered addresses: sound with changes.** The data model is mostly in place. `email_aliases` already has `canonical_number`, and the receive policy is named "public numeric inbound". What's missing is everything that makes a number an identity. There is no allocator, no uniqueness on the number, and no normalization rule. Allocating when the user row is created would also hand numbers to abandoned sign-ups and to the 31 deployed Playwright specs that register fresh accounts on choir.news. On top of that, an incrementing public namespace on a catch-all Resend domain turns spam into a quota problem that can block sign-in recovery mail. The changes below fix the identity gaps. Enumerability can't be designed out; it can only be priced in.

**autopaper@ newsletter ingest: sound with changes.** The right path already exists and needs no new mechanism. It is a source-schedule poll by the research desk, writing captures the same way RSS polls do. Four things are missing: a sender policy at the granularity of a single publication, a rule that confirmation and unsubscribe clicks are owner acts, a canonical-URL rule so the email and web copies of one post aren't counted as two outlets, and an explicit decision on copyright posture.

**Provider: Resend for both directions, and the free tier is not enough.** Inbound mail counts against the same 100-a-day, 3,000-a-month pool as outbound, and that pool is shared with recovery and digest mail.

## Current state (facts)

**Provider and transport**
- One provider, Resend, handles both directions. The API base is `https://api.resend.com` (`internal/maild/config.go:19`). Inbound arrives as the `email.received` webhook at `/api/email/resend/webhook` (`webhook.go:70`, Caddy route `nix/node-b.nix:739`). The webhook is Svix-signed, and maild then fetches the parsed email from the Resend API (`webhook.go:201-218`).
- DNS is on Gandi. The MX/TXT records come from Resend's domain JSON (`scripts/mail-gandi-plan-records`, `scripts/mail-resend-setup` `--ensure-domain` "enable sending+receiving").
- No SMTP server, SES or other provider is in the repo. SES appears only as a future cost lever (`docs/reports/agentic-email-desk-2026-10-14.md` Q12).

**Addresses today**
- `000` is seeded and reconciled on every boot to `MAILD_ROOT_OWNER_ID` (`store.go:908-931`). The repo default is the literal `root` (`config.go:18`). The deployed value lives in `maild.env`, which I did not read. `/health` reports `root_owner_id_configured` (`health.go:40`).
- Fresh accounts get no address. The only other path is the operator command `maildctl configure-workflow-alias`, which requires plus-codes (`store.go:945-960`, `frontend/tests/mail-attachments-deployed.spec.js:20-38`).
- The table is `unique(domain, local_part)` (`store.go:339-352`). `canonical_number` has no unique index. `canonicalNumberFromLocalPart` returns 0 both for `000` and for any non-numeric local part (`ingest.go:319-329`), so 0 is ambiguous.
- **Namespace defect:** any owner can claim `000+anything`. The deployed test does exactly this for fresh test accounts (`spec.js:23`). Under numbering, `001+x` could therefore be owned by someone other than 001.
- Routing is an exact lowercase string match (`ingest.go:280-290`, `store.go:1008-1029`). `0@`, `01@` and `0000@` resolve nothing.
- An unknown recipient returns `sql.ErrNoRows`, which is non-retryable (`webhook.go:191-199`). The result is `accepted_ingest_failed`: silent loss with no bounce. Resend has already counted the message against quota.
- Mailboxes are keyed by **account (user_id)**, not by computer: `users/<owner>/mail.db` (`store.go:283`). A computer reads its mailbox over the guest tap, bound by vmctl to its `user_id` (`api.go:131-160`).

**Accounts and identity**
- The `users` row is created at `register_begin`, before any passkey exists (`internal/auth/handlers.go:512-521`). The cap counts only users who hold a passkey (`registration_cap.go:15-21`).
- Identity is passkey plus the signup email (`users.email UNIQUE`). Recovery mail goes to that email.
- No product code deletes users (no `DELETE FROM users` outside tests).
- Transactional and digest mail already send as `updates@choir.news` (`auth_mail.go:171`, `digest.go:217`). That is the same domain the numbered addresses would use.

**Autopaper ingest**
- Ingest is a set of standing per-source schedules (`internal/store/source_schedule.go:74-111`). Boot recovery and the crash-loop bound live in `agentcore/source_schedule.go:338-466`.
- Polls go through `researchtools.PollSourceOnce`. Pollers exist only for `rss`, `telegram` and `gdelt` (`poll_source.go:317-331`).
- Captures are written through corpusd with the key `canonicalURL#docHash` (`sourcegraph/captures.go:171`).
- Registry rows carry `License`, `TOSClass` and `StoreBodyPolicy` fields (`sources/types.go:51-61`). There are 256 rows, none of them email or Substack.
- The paper computer `computer-1355b83b…` belongs to a dedicated Autopaper account, not to the owner (`docs/problems/paper-computer-disk-full-outage-2026-10-15.md:9`). FP0 proof scripts refuse every mutating step against it.
- Sender authentication requires aligned DMARC/DKIM from a trusted authserv-id, and with none configured it fails closed (`sender_auth.go:11-41`). `MAILD_TRUSTED_AUTHSERV_IDS` is unset in nix (`node-b.nix:1052-1057`).

**Resend limits (verified 2026-10-26)**
- Free tier: 3,000 emails a month, 100 a day, 3 domains, 1 webhook endpoint, 30-day retention (https://resend.com/pricing).
- "Both sent emails and received emails (inbound) count towards your account's email quota… Each received email counts as 1 email against your daily and monthly limits" (https://resend.com/docs/knowledge-base/account-quotas-and-limits).
- Receiving accepts "any address at your Resend domain", meaning catch-all (https://resend.com/docs/dashboard/receiving/introduction).
- Rate limit is 10 requests a second per team.
- Pro is $20 a month for 50,000.
- **Unverified:** what happens to *inbound* mail once the quota is exhausted. The docs don't say. Outbound returns a 429 `daily_quota_exceeded`.

## Risks, ranked

1. **Quota exhaustion blocks account recovery.** Recovery, the digest, newsletters, every numbered inbox and all spam to any choir.news address draw on one 100-a-day pool. Mailing 000 through 999 once is ten days of free quota. Newsletter ingest alone (20 to 50 subscriptions) can use half the daily pool.
2. **Shared sending reputation.** Numbered accounts that send outbound on `choir.news` share a domain with `updates@` recovery mail. One complaint-heavy account degrades sign-in for everyone.
3. **Hostile publishers can reach the paper.** Allowlisting `substack.com` admits *every* Substack, and a Substack writer can import arbitrary subscriber addresses. A forged or hostile newsletter becomes publishable evidence on a public paper.
4. **Numbers burned or stolen.**
   - Allocating at `register_begin` gives numbers to abandoned ceremonies.
   - Test accounts would consume 001 through 999 within weeks.
   - Through the `000+x` defect, a plus-alias can be claimed under someone else's number.
5. **Double counting.** The email and RSS copies of one post become two captures (different docHash) and may satisfy the two-outlet publish rule alone.
6. **Copyright.** Full-text and paid newsletter bodies feed a logged-out public paper. Flagged, not adjudicated.
7. **Circular recovery.** A user whose signup email is their own `NNN@choir.news` can't recover the account.

## Recommended design

### Numbered addresses

- **Single authority:** the maild routing DB. Auth stores nothing.
- **Sequence table:** add `email_number_sequence(next integer)` seeded to 1, plus a unique partial index on `email_aliases(canonical_number) WHERE canonical_number IS NOT NULL`. Non-numeric aliases store NULL; fix `ingest.go:319-329` accordingly.
- **Allocation:** one SQLite transaction reads `next`, increments it, and inserts the alias. Never compute `max+1`, because that reuses the top number after a delete.
- **Concurrency:** maild is the single writer, and the transaction serializes concurrent signups.
- **Idempotency:** a unique index on numbered `(target_id)` means one number per account.
- **When:** at the first passkey (`register_finish` success). Auth calls a new host-internal maild route, `POST /internal/maild/accounts/{user_id}/number`, using hostauth. The call is idempotent, and if it fails, the number is provisioned lazily on the first Mail API call. Skip ephemeral test domains (`example.com,example.test`, as in `VMCTL_RETENTION_EPHEMERAL_EMAIL_DOMAINS`).
- **Never reuse a number.** Closing an account sets `disabled_at`, and the sequence never goes backwards. Resend can't reject at SMTP time, so "bounce" means drop plus a receipt.
- **Normalization:**
  - The canonical form of *n* is `fmt.Sprintf("%03d", n)`: `000` through `999`, then `1000`, `1001` and so on, with no ceiling.
  - A local part whose base (before `+`) is all digits routes only if it equals the canonical form of its own value. So `0`, `00`, `01`, `0001` and `01000` route nowhere: no alias, recorded as unrouted.
  - Cap the digit length (for example 12) before parsing.
  - Outbound always uses the canonical form.
- **Plus aliases:** `NNN+tag` may be created only by the owner of `NNN`. Fix the `000+uitest` fixture to use the test account's own base, or a non-numeric test namespace.
- **Identity:** the number is a mailbox, not an auth identifier. Refuse `@choir.news` signup and recovery emails.
- **Account vs. computer:** bind the address to the account, as maild already does. Computers come and go, and forks will mint new ComputerIDs. Delivery to a computer is a routing decision made later (E3).
- **Default receive policy:** the existing public numeric policy, which quarantines and blocks attachments. Ship numbered accounts receive-only. Outbound needs paid status and the per-user quotas in the email desk report §3.4. Move `updates@` to `notify.choir.news` before outbound opens up; that is the second of the 3 free domains.

### autopaper@ ingest

- **Route:** an alias `autopaper@choir.news` with `target_id` set to the Autopaper account's `user_id`, created with maildctl. Use non-numeric local parts and a NULL number.
- **Receive policy:** a new "newsletter ingest" policy. Public inbound is on, attachments are off, and all mail is quarantined until approved.
- **Sender allowlist:** a message is promoted only when it passes aligned DKIM/DMARC from a trusted authserv-id **and** its `(DKIM d=, List-Id)` pair is on the list. That means one row per publication, not per domain, and adding a row is an owner act. Set `MAILD_TRUSTED_AUTHSERV_IDS` first.
- **Ingest path:** a new `SourceTypeMailbox` poller in `internal/sources`. It runs under the existing source schedule on the paper computer, read by the research desk, which has read-only message authority (I26). The poller lists approved messages for that alias through maild's guest-tap API. Then it:
  - converts the HTML through the existing reader extraction into text blocks: no remote images, attachments stay quarantined metadata;
  - strips tracking parameters;
  - sets `CanonicalURL` to the post's web permalink (`List-Post`, `<link rel=canonical>`, or the "view online" link), so the capture key and outlet match the web copy;
  - sets `Outlet` to the publication.

  Because it's pull-based, maild never needs to wake the paper. Crash bounds and 24/7 recovery come for free from `recoverSourceSchedulesAtBoot`. Because it reads through maild's API, E3 (moving the mailbox into the computer) can replace the backend underneath without changing the poller.
- **Confirmation and unsubscribe clicks:** these change state in the outside world, so they are owner acts in the Mail app, never research and never management. The desk may *propose* them.
- **Outlet counting:** an email capture and an RSS capture with the same canonical URL count as one outlet. Give newsletters a `source_standing` (commentary versus reporting).
- **Copyright posture (flag):** set `StoreBodyPolicy` to private evidence. Publish synthesis with attribution and a link only. Default paid posts to excluded.
- **Desk authority:** management owns the schedule and the email policy, research reads and captures, and Texture alone writes the Textures. No Mail app desk is needed for this.

## Open owner questions

1. **Quota:** stay on the free tier, or move to Pro ($20) before launch? The repo can't tell which plan is active, and the free tier's inbound behavior after exhaustion is undocumented.
2. **Enumerability:** accept guessable sequential numbers as a deliberate brand choice? (The alternative contradicts the request.)
3. **Backfill:** do existing accounts (the Autopaper account, QA accounts, your other accounts) get numbers, in creation order, or only new accounts?
4. **Outbound:** may numbered accounts send at all at launch, and only on paid plans?
5. **Confirmation mail:** who logs into the Autopaper account to click confirmation links, or should those messages surface in your own Mail app?
6. **Republishing:** what is the policy for newsletter text, and are paid posts in or out?
7. **Unrouted digit forms** like `1@`: drop them (recommended), or alias them to `001`?

## Mutation class

- **Numbered allocation:** **red**, tier 2 (Devin and DeepSeek). It touches the auth `register_finish` path, the store schema, and an irreversible public assignment.
- **Normalization and the plus-alias fix:** orange to red, folded into the same review.
- **autopaper@ alias and policy rows:** orange (maildctl config, rollback by `disabled_at`).
- **Mailbox poller:** orange, tier 1 (Devin). It is runtime code on the protected paper. Prove it first on a test account with its own alias. Roll back by stopping the schedule.
- **Setting `MAILD_TRUSTED_AUTHSERV_IDS` and the `notify.` subdomain:** red. These change the auth/evidence verdict and deploy routing.
- **The problem docs below:** green.

## Build order

1. Green problem docs: the `000+` namespace defect, the ambiguous `canonical_number` 0, silent drops of unknown recipients, and inbound counting against the shared quota.
2. Owner decision on the Resend plan, plus a per-day received counter in `maildctl stats`.
3. Set `MAILD_TRUSTED_AUTHSERV_IDS` from a real message's header.
4. Create the autopaper@ alias, the newsletter policy and quarantine. The owner subscribes and clicks the confirmations.
5. Build the mailbox poller and prove it on a test account. Then start the schedule on the paper.
6. Numbered addresses: the sequence, the provisioning route, the auth hook, normalization, the plus-alias ownership check and the ephemeral-domain skip. Prove it with a deployed signup that gets `001`, then a second that gets `002`, then a delivery to `002`.
7. Move transactional mail to `notify.choir.news` before outbound opens for numbered accounts.
