# purge-unproved: what it deletes, and how it is run

The owner, 2026-11-24: "delete unproved accounts". Black class. One operator command,
`cmd/purge-unproved` (library `internal/unprovedpurge`), deletes the accounts whose `users.email_verified_at`
is NULL. That is the column, not history: the one-time backfill (`migrateEmailVerifiedAt`) marked every account
that already held a passkey as proved without proof, so an account squatted before then is not in the 199.
Nothing runs the command: no service, boot step or timer. Production on 2026-11-24 (read-only, the lead): 199
of 1,097 accounts, 27 holding a passkey, none owning a computer.

## Where it is and how it is run

Built with the services (`flake.nix`, `goServiceSubPackages`) and installed beside auth:
`/var/lib/go-choir/services/auth/bin/purge-unproved` on Node B (not run through nix here: the flake parses; the
derivation was not built). It needs read and write on the databases and the mail root, so it runs as root on
Node B, **with `go-choir-auth` and `go-choir-maild` stopped** for the real run: a late mail could recreate a
mailbox directory (the run looks once more at the end and says so), and a stopped auth removes any chance of a
proof landing mid-run (the run also takes auth's write lock, below).

```
purge-unproved --auth-db FILE --maild-db FILE --maild-root DIR \
               --pg-control-dsn DSN --pg-graph-dsn DSN --vmctl-url URL [--internal-token-file FILE]
purge-unproved ... --apply --digest <hex> --receipt FILE [--receipt-key-file FILE]
```

- **The full service set is required.** `--maild-db` and `--maild-root` go together; both Postgres DSNs are
  required (the passwords come from the usual credential). `--allow-missing-service maild|control|graph`
  exists and prints a loud warning that service's rows, files and Proof checks are not part of the run. Each
  database must hold its core tables or the run stops (a mistyped path makes an empty file; SQLite paths must
  already exist).
- **vmctl** is required and its list is read with the host-internal token (`--internal-token-file`, else
  `CHOIR_INTERNAL_TOKEN_FILE` or the credentials directory). An **empty** registry is refused (an unloaded
  registry looks the same as one with no computers) unless `--vmctl-empty-ok`.
- **Dry run (the default)** reads only. It prints the services, the accounts to purge and the accounts refused
  (both as `sha256` prefixes, never ids or addresses), the rows per `service.table.column`, the mail
  directories, and the digest. The owner is shown these counts before the real run. The digest is over the
  sorted accounts **with a fingerprint of each one's state** (id, address, creation time, passkey ids), the
  services and the counts: order does not matter, anything else does.
- **Real run** (`--apply --digest --receipt`) recomputes the plan and stops if its digest is not the one given.
  It then **takes auth's write lock and checks, under the lock, that every listed account is still unproved
  and unchanged** (a recovery, a verification, a new passkey since the plan: nothing is touched). Then, still
  holding auth's lock: the other services are deleted (one transaction each, with `secure_delete` on for
  SQLite), the mail directories are removed, and the accounts are deleted and auth commits **last**. A failure
  before auth's commit leaves the accounts in place and the run can be made again; what a service already
  committed stays deleted, and the output says which services committed. The mailbox directories go before the
  accounts do, so a rerun finds both.
- **The receipt** is one JSON line appended and synced to `--receipt`: when, the digest, the services and what
  each did, the rows per table, the full `sha256` of each purged id, and, with `--receipt-key-file`, a keyed
  hash of each address (so a later "was my account deleted" can be answered without keeping the address).

## Refused, never deleted

An account is refused (printed hashed, with the reason) if it is the bootstrap admin, holds an owner key
(`owner_root_keys`), appears in vmctl's list of ownerships in any state (the list is of computers **now**:
`retention_prune` removes pruned entries from it, so "ever owned" is covered only by the rows below), has an
unsafe id (empty, `.`, `..`, a separator, a leading dot, not a path-local single element: nothing is built
from it), or has a row in a store that exists only for an account that used a computer or the platform (the
`Proof` keys below, including the route ledger).

## Every table keyed by a user, and its key

The registry is `AuthKeys`, `MaildKeys` and `PlatformKeys` in `internal/unprovedpurge/purge.go`. The run also
asks each database which tables have a user-keyed column, by name (`requested_by`, `approver`, `target_id`,
`route_slot_id`, `mailbox_owner_id`, `submitter_id`, `email`) **or by suffix** (`owner_id`, `submitter_id`,
`user_id`, `account_id`, `owner_user_id`), and **stops if one is not in the registry**: a table added after this
list was written is a refusal to run, not a table left behind.

**auth (SQLite)**: `credentials`, `challenge_state`, `refresh_sessions`, `desktop_exchange_codes`, `api_keys`,
`recovery_tokens`, `email_signin_attempts`, `device_authorizations`, `account_set_member`, `pending_registrations`
(`user_id`); `owner_root_key_wraps`, `owner_root_keys` (`owner_id`); `pending_registrations.email` and
`registration_waitlist.email` (case-folded, **except an address a surviving account holds in any case**, which
is left alone); `users.id`. `account_set` is **repaired**, not deleted: a set whose member goes keeps its other
members and points `active_user_id` at the most recent survivor; a set left with none is deleted.

**maild (SQLite routing database)**: `email_sender_whitelist`, `email_provider_message_index`,
`email_approval_token_index`, `email_owner_addresses`, `digest_sent`, `digest_runs`, `digest_subscriptions`
(`owner_id`), `email_aliases.target_id` where `target_type = 'user'`. The legacy shared tables
(`email_messages.mailbox_owner_id`, `email_ingress_events`, `email_drafts`, `email_draft_approval_events`,
`email_draft_approval_tokens`, `email_risk_alerts`, `email_draft_edit_requests`, `owner_id`) are `Proof`: if the
routing database still holds rows there for an account, it is **refused** until they are migrated away or the
tables dropped. On disk: `<root>/users/<id>` and `<root>/attachments/outbound/<id>`, removed after the
database commit, only after `<root>` and `users/` are checked to be real directories (no symlink) and each
target to be a direct child.

**PostgreSQL (control and graph; absent tables are skipped), all `Proof`**: the `owner_id` tables `events`,
`choir_events`, `run_memory_entries`, `wire_publish_debounce_entries`, `wire_publish_debounce_state`,
`co_super_slots`, `media_progress`, `media_recents`, `user_preferences`, `desktop_workspaces`,
`desktop_sessions`, `desktop_app_instances`, `desktop_window_placements`, `texture_revisions`,
`texture_source_entities`, `texture_source_refs`, `texture_document_aliases`, `texture_agent_mutations`,
`agent_evidence`, `og_objects`, `platform_texture_documents`, `platform_texture_revisions`,
`proj_capture_resolution`, `proj_capture_index`, `proj_capture_terms`, `vec_captures`, `search_leads`;
`owner_act_receipts.owner_user_id`; `publication_version_proposals.source_owner_id` and `.submitter_id`;
`proposal_delivery_records.target_owner_id`; `computer_key_unwrap_requests.requested_by`;
`computer_key_unwrap_approvals.approver`; and the four route-ledger tables
(`computer_version_route_slots`, `_transition_receipts`, `_authorization_evidence`, `_authority_modes`),
whose `route_slot_id` is `"computer:<owner>:<computer>"` (matched by prefix and id).

## What it does not reach, and why

- **Inside a computer** (a guest's own database, the journal): none of the accounts owns a computer, and an
  account that does is refused.
- **vmctl's registry** is read, not written.
- **Gateway state** is keyed by computer, VM or provider, not by user: `call_tally`, `paid-spend.json`, the
  Anthropic relay's spend, `gateway-identities.json` and the searchplane `provider_health` (read in
  `internal/gateway`; not read on a live gateway).
- **`email_signin_attempts` rows with no user** (registration proofs) are keyed by a keyed hash of the address;
  without the HMAC key they are not matched, and the sweeper deletes them within a day.
- **Backups and free pages.** `scripts/off-host-backup.sh` snapshots `auth.db`, `mail.db`, `<root>/mail` and
  the `control` and `graph` dumps to node-a daily and keeps two sets: the deleted accounts remain in them until
  the weekly prune passes them (and in any permanent `archive` snapshot). The run sets `secure_delete` on its
  SQLite connections so freed pages are overwritten; logs (journald) are not touched. "Deleted" in the product
  means gone from the live stores; the privacy policy's wording on backups is the owner's to settle.
- **Postgres schemas were read from the code, not from a live database**: the first dry run on production is
  also the check that the registry is complete (an unregistered table stops it).

## Proof

`internal/unprovedpurge/purge_test.go` seeds a proved account and unproved accounts (one that goes; a computer
owner; an owner-key holder; the bootstrap admin; one with platform rows; one with a route-ledger row) across
auth, maild (with mailbox and outbound-attachment directories) and a platform database. It checks: the dry run
changes nothing and lists an exact set of tables; the real run needs the digest; only the listed account loses
rows and directories, with auth committed last; an account proved after the plan, and one that gained a
passkey, lose nothing in any service (the hook runs between the plan and the lock); unregistered user-keyed
tables stop both runs, by name and by suffix; proposal, unwrap and route-ledger rows refuse the account; a case
variant of a kept address does not take that account's rows; unsafe ids never reach a path or a pattern and a
symlinked `users/` is refused; account sets are repaired; vmctl down means no plan and no run; an empty
database is not a service; 951 accounts are all deleted across chunks. **Not run**: against a Postgres
database (the `$n` placeholders and `information_schema` queries are untested here), against a real mail or
auth database, or through the command's flags.
