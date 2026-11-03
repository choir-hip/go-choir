# Capsule overlay S0: d@e state measured (2026-11-03)

Slice S0 of `docs/design/capsule-overlay-computer-state-2026-11-03.md`:
read-only, metadata only. Computer: d@e (computer-59c902c5..., VM
vm-70070cf7...). Mutation class: green.

## Method

On Node B, as root under `nice -n 19`: `cp --reflink=always` of d@e's live
`data.img` into `/var/lib/go-choir/drill-scratch/ovl-s0-<ts>/` (btrfs, no
extra space, the guest not stopped, the live file never opened for
writing). The copy was loop-mounted (the journal replays on the copy) and
measured with `find`, `du`, `stat` and `sqlite3`. Nothing was printed but
counts, sizes, names of top-level directories and column names. The
scratch directory was deleted at the end; host free space was 145 GB
before and after.

## Results

| What | Value |
| --- | --- |
| Data volume | 32 GiB sparse, 1.2 GB allocated; 1.0 GB used, 30.8 GB free inside the guest |
| Files root | 7,698 regular files, 334 directories, no symlinks, sockets or devices; 216 MB |
| `Source/platform` | 4,529 files, 216 MB (all but 107 KB of the files root) |
| `mail/` | empty |
| `.git` directories | one (`Source/platform/.git`); none elsewhere |
| Files without `o+r` | none outside `Source/platform` |
| Directories 0700 | 289 of 334 (hydration's mode, as the design expects) |
| Files the snapshot keeps | 3,169 (outside `Source/platform`, `mail/` and `.git`) |
| `state.db` | 211 MB, WAL 4.3 MB |
| `VACUUM INTO` (host disk, copy) | 0.66 s, 199 MB result |
| `state.texture/` | 9.1 MB (mostly `telemetry/`) |
| `state.image-blobs/` | 2.0 MB |
| Text and blob column bytes | about 137 MB; the largest are `og_bodies` 76 MB, `run_memory_entries` 33 MB, `og_objects` 14 MB, `computer_event_index` 13 MB |
| Stat walk of the kept set | 0.012 s (warm cache) |
| Hardlink walk of the kept set | 13.3 s with one `mkdir -p` and `ln` process per file; an upper bound dominated by forks |

Credential-shaped column names in the store (name scan of the design):
`store_meta.key`, `run_memory_entries.tokens_before`,
`user_preferences.preference_key`, `desktop_sessions.session_id`,
`desktop_app_instances.created_by_session_id`,
`desktop_window_placements.session_id`,
`computer_event_projection_heads.credential_revocation_epoch`,
`computer_event_index.idempotency_key`,
`computer_event_index.next_credential_revocation_epoch`,
`self_development_start_intents.idempotency_key`,
`self_development_operations.idempotency_key`, `og_key_index.key_name`,
`og_key_index.key_value`.

## Reading

- The snapshot is small: about 3,200 links. The stat walk takes 12 ms, so
  an in-process `linkat` walk should hold the files lock for well under a
  second; the shell figure is fork cost. S2 records the real hold in each
  receipt.
- The store copy is about 200 MB and well under a second to vacuum on d@e;
  the capsule's disk charge is that plus the upper allowance, against 30 GB
  free. The paper's store (23 GB) is the case the charge refuses.
- The secret-shape scan of the copy reads about 137 MB of text on d@e. S3
  measures its in-process cost and records it per spawn.
- The column scan has 13 names to rule on. None holds a credential (the
  inventory below); S3's column test allowlists each with its reason.

## Secret-shaped material (owner, 2026-11-03: "there shouldn't be credentials on computers anyway")

Names, owners, modes, sizes and kinds only; no file was opened. Sources:
the data volume copy above, a reflink copy of d@e's `credential.img`
(loop-mounted read-only, deleted after), and, for the in-memory `/run`
files, the code that writes them (`nix/autoputer-vm.nix`,
`internal/vmmanager`). Classes: (a) computer identity or key material,
kept out of every capsule because a capsule must not act as the computer;
(b) a credential that unlocks something outside the computer; (c)
neither.

| Item | Where | Mode, owner, size | Class |
| --- | --- | --- | --- |
| `privacy-key` | `/mnt/persistent/choir-credentials/` | 0400 root, 123 B | (a) the computer's data key (seals owner payloads) |
| `verifier/key.ed25519` (+ provenance) | `/mnt/persistent/choir-signers/` | 0600 uid 998, 64 B | (a) verifier signing key |
| `guest-core/key.ed25519` (+ provenance) | `/mnt/persistent/choir-signers/` | 0600 uid 999, 64 B | (a) tape and receipt signing key |
| `gateway-token` | credential disk, `/run/choir-bootstrap/` | 0400 root, 101 B | (a) the computer's identity at the gateway |
| computer-event envelope | credential disk | read and unlinked at boot | (a) appender credential |
| `host-fetch.json`, `capsule-mirrors.json` | credential disk | 0400 root, 128 B, 198 B | (c) grant records (computer id, status, basis, time) |
| `/run/go-choir-autoputer.env`, `/run/go-choir-updater.env` | guest tmpfs | written at boot | (c) ids, ports and URLs only |
| `choir-updater/` | `/mnt/persistent/` | releases and carried records | (c) no secret-shaped name |
| files store | `/mnt/persistent/files` | no secret-shaped name outside `Source/platform` | (c); `Source/platform/.envrc` is the repo's tracked file, and its `.git/config` remote has no userinfo |
| store columns | `state.db` | the 13 names above | (c); the desktop session ids are layout keys the client picks (`X-Choir-Session`, `desktopstate/handler.go:127`), not an authenticator |

Not on d@e, found in the fleet config: the owner's Claude OAuth token,
`claude-oauth-token` on the credential disk of the computers in
`CHOIR_CLAUDE_DELEGATE_COMPUTERS` (three). Class (b). Recorded as
`docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md`.

Reading: every (a) item sits under `/mnt/persistent` or `/run`, which the
capsule masks. Nothing in the files store or the store needs a name-based
leave-out on d@e, so the design's speculative credential-name lists are
cut (design revision 4).

## S3 local smoke: the autoputer over a store copy (2026-11-03)

On a workstation, not a capsule: an autoputer built from the S3 branch
started once over an empty store (the "live" store), then `Store.SnapshotTo`
took a copy (557 KB, copy 7 ms, scan 6 ms over 472 bytes of text; the
sidecar carried `store-schema.json` and `vocab-migration-report.json`).
A second autoputer started over the copy with only
`RUNTIME_STORE_PATH`, `AUTOPUTER_FILES_ROOT`, `AUTOPUTER_PORT` and
`CHOIR_GUEST_TRUST_LOOPBACK=1` in its environment. `/health` answered
`ready` with `active_provider: stub`; `GET /api/files/notes` with an
`X-Authenticated-User` header listed the seeded file; its log said
"host fetch route off (no credential directory)" and "using stub provider
(no gateway configured)", and named no other credential path. The real
proof is P1 and P6 on d@e.
