# purge-unproved: what it deletes, and how it is run

The owner, 2026-11-24: "delete unproved accounts". Black class. One operator command,
`cmd/purge-unproved` (library `internal/unprovedpurge`), deletes every account whose address was never
proved (`users.email_verified_at IS NULL`). Nothing runs it: no service, boot step or timer. The accounts
are the ones the proof rule left inert (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md);
production on 2026-11-24 (read-only, the lead): 199 of 1,097, 27 holding a passkey, none owning a computer.

## Use

```
purge-unproved --auth-db FILE [--maild-db FILE --maild-root DIR] [--pg-dsn DSN ...] --vmctl-url URL
purge-unproved ... --apply --digest <hex>
```

- **Dry run (the default)** reads only. It prints the accounts to purge and the accounts refused (both as
  `sha256` prefixes, never ids or addresses), the rows per `service.table.column`, the mailbox directories,
  and the digest of the list. The owner is shown these counts before the real run.
- **Real run** (`--apply --digest`) recomputes the plan; if its digest is not the one given it deletes
  nothing. The other services are deleted first, one transaction each, and auth (the accounts) last, so a
  failure before the end leaves the accounts in place and the run can be made again. Inside the auth
  transaction every listed account is checked unproved once more; if one was proved meanwhile, nothing goes.
- **No vmctl, no run**: `--vmctl-url` is required. Whether an account owns a computer is not guessed.

## Refused, never deleted

An account is refused (printed hashed, with the reason) if it is the bootstrap admin, holds an owner key
(`owner_root_keys`), appears in vmctl's list of ownerships in any state (owns or owned a computer), or has a
row in any store that exists only for an account that used a computer (the `Proof` keys below).

## Every table keyed by a user, and its key

The registry is `AuthKeys`, `MaildKeys` and `PlatformKeys` in `internal/unprovedpurge/purge.go`. The run
also asks each database which tables have a user-keyed column (`user_id`, `owner_id`, `owner_user_id`,
`account_id`, `active_user_id`, `target_id`) and **stops if one is not in the registry**: a table added after
this list was written is a refusal to run, not a table left behind.

**auth (SQLite, `AUTH_DB_PATH`)**: `credentials.user_id`, `challenge_state.user_id`, `refresh_sessions.user_id`,
`desktop_exchange_codes.user_id`, `api_keys.user_id`, `recovery_tokens.user_id` and `.email`,
`email_signin_attempts.user_id`, `device_authorizations.user_id`, `account_set_member.user_id`,
`account_set.active_user_id` (cleared, the set belongs to others too), `owner_root_key_wraps.owner_id`,
`owner_root_keys.owner_id`, `pending_registrations.user_id` and `.email`, `registration_waitlist.email`
(case-folded), `users.id`.

**maild (SQLite, `MAILD_DB_PATH`)**: `email_sender_whitelist.owner_id`, `email_provider_message_index.owner_id`,
`email_approval_token_index.owner_id`, `email_owner_addresses.owner_id`, `email_aliases.target_id` where
`target_type = 'user'`, `digest_sent`, `digest_runs`, `digest_subscriptions` (`owner_id`); and the mailbox
directory `<storage root>/users/<id>` (the per-account mailbox database and attachments), removed after the
database commit.

**PostgreSQL (the control and graph databases, `--pg-dsn`, each tried; tables absent from a database are
skipped)**, all `Proof` (a row refuses the account): `events`, `choir_events`, `run_memory_entries`,
`wire_publish_debounce_entries`, `wire_publish_debounce_state`, `co_super_slots`, `media_progress`,
`media_recents`, `user_preferences`, `desktop_workspaces`, `desktop_sessions`, `desktop_app_instances`,
`desktop_window_placements`, `texture_revisions`, `texture_source_entities`, `texture_source_refs`,
`texture_document_aliases`, `texture_agent_mutations`, `agent_evidence`, `og_objects`,
`platform_texture_documents`, `platform_texture_revisions`, `proj_capture_resolution`, `proj_capture_index`,
`proj_capture_terms`, `vec_captures`, `search_leads` (`owner_id`), `owner_act_receipts.owner_user_id`.

## What it does not reach, and why

- **Inside a computer** (a guest's own database, the journal): none of the accounts owns a computer, and an
  account that does is refused.
- **vmctl's registry** is read, not written: an account with an entry is refused, so there is nothing to delete.
- **Gateway usage**: the gateway keeps its per-caller tally in memory (`call_tally`); a search of every
  `CREATE TABLE`, `sql.Open`, `OpenSQLite` and `OpenPostgres` in `cmd` and `internal` found no gateway table
  keyed by a user (only `searchplane` health). Not read on a live gateway.
- **`email_signin_attempts` rows with no user** (registration proofs) are keyed by a keyed hash of the address;
  without the HMAC key they are not matched, and the sweeper deletes them within a day.
- **Backups and logs**: not touched.
- **Postgres schemas were read from the code, not from a live database**: the first dry run on production
  is also the check that the registry is complete (an unregistered table stops it).

## Proof

`internal/unprovedpurge/purge_test.go` seeds a proved account and unproved accounts (one that goes; a computer
owner; an owner-key holder; the bootstrap admin; one with platform rows) across auth, maild and a platform
database, and checks: the dry run changes nothing and counts exactly; the real run needs the digest; only the
listed account loses rows (and its mailbox directory), and every other account's rows and mailbox are as they
were; an unregistered user-keyed table stops both runs; an account proved between the runs is not deleted;
no vmctl answer, no plan. **Not run against a real database**: the dry run on production is the owner's to see.
