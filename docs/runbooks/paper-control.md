# Paper control: acting on the Autopaper computer (2026-11-09)

How to change something on the paper (the `public_platform` computer that
publishes Autopaper) today, and what is missing. The designed replacement is
the operator role in
[platform-operator-account-2026-11-06.md](../design/platform-operator-account-2026-11-06.md)
section 5 (slices O2 to O5, not built); the CLI layer for it is in
[paper-control-cli-2026-11-09.md](../design/paper-control-cli-2026-11-09.md).

Acting on the paper needs the owner's word for that act. Reading does not.

## The operator path (from the owner's own login)

Built 2026-11-09 (slices O2 and O3 of the operator design). After the owner
names their account as platform operator (once, below), none of the saved
session, the 7-day key or `qa_renew_key.mjs` is needed for the policy:

```
choir login                       # the owner's own account
choir paper status                # build, state, schedules, resolved policy per role
choir paper policy get
choir paper policy set --local f  # validated, old file kept, a receipt on the paper's tape
choir paper policy revisions
choir paper policy rollback [--to n]
choir paper policy verify [--role r]
choir paper schedule list
choir paper schedule start --source rss:<id> [--interval-minutes n] [--max-items n] [--reason text]
choir paper schedule stop --source rss:<id> [--reason text]
choir paper restart --reason text
```

Schedules and restart (O4 and O5, built 2026-11-10, not yet proved on d@e or
staging): each start, stop and restart records a
`platform.computer.schedule` or `platform.computer.restart` receipt on the
paper's tape before it acts, and the answer prints the receipt id. A schedule
start takes a registry source only (no feed URL). `restart` is vmctl's planned,
marker-backed owner update: it is refused, with a reason code and exit 1, mid
self-development apply, with a live engineering capsule or a snapshot in
flight, for a held or not-active computer, while a refresh is in flight, or
while the ring has not opened. There is no force; it never stops and starts
the computer the plain way. The paper (public_platform) takes the refresh while
busy and resumes its work. The answer's `receipt` records the ask, not the
outcome.

`--computer <id>` names another platform computer; with one in the roster it
is implied. Each write prints `revision` (the file kept) and `receipt` (the
event id of the `platform.computer.model_policy` record on the paper's tape,
naming the operator's account id). A paper still on a build from before the
receipt answers 409 and nothing is written; it takes the write after its next
update.

**The one-time step.** The roster is the GitHub Actions secret
`CHOIR_PLATFORM_OPERATOR_USERS` (value: the owner's account id, lower-case
UUID, comma-separated up to eight, or `none` to revoke). Read the id from
`https://choir.news/auth/session` (`user.id`) while signed in. The next
deploy writes `/var/lib/go-choir/unit-env/corpusd-operators.env` on Node B;
corpusd logs `platform operator accounts: 1`. The proxy holds no copy. Details:
[the operator design](../design/platform-operator-account-2026-11-06.md) 4.1.
A key bound to one computer, or lacking `read:runtime` or `write:runtime`,
is refused for the matching act; a browser session must be a passkey session.
(`choir login` approves a full-access key; whether it is unbound on staging
is unverified.)

The sections below are the fallback: the old key path.

## The key

The paper is owned by a test account with no person behind it (a virtual
passkey, no inbox), so nobody can sign in to it in a browser. The only way
to act as its owner is an API key on the developer machine:

- file: `~/.config/choir-qa/test-computers/autopaper-paper.json` (field
  `api_key`; the file also names the computer id);
- scopes include `write:runtime` and `owner:acts`;
- it expires after 7 days. Renew with
  `node scripts/qa_renew_key.mjs autopaper-paper`, which replays the saved
  Playwright passkey session on this machine. If that session is lost, the
  paper has no owner who can act.

Never print the key. Read it inside the command, or put the raw key in a
mode-0600 file and pass `--api-key-file`.

## Model policy

The policy is one file in the paper's own store, `System/model-policy.toml`.
The computer reads it at the start of every run, so a change needs no
restart. One command changes it (`choir ... ` below stands for
`choir --computer <paper id> --api-key-file <key file>`):

1. Read the current policy and what each role resolves to:
   `choir ... policy get`.
2. Edit a copy. Keep the `# owner-edited` line: without it the platform
   overwrites the file with its default at the next policy version. Say in
   the file's header what changed and when.
3. Write it: `choir ... policy set --local <copy>`. The CLI refuses a copy
   without the `# owner-edited` line (`--allow-unmarked` overrides it on
   purpose). The server then parses the file with the loader's own parser
   and refuses, with the line or field named and nothing saved, a file that
   does not parse, names an unknown role, pool, key or effort, or names a
   model the catalog does not list for its provider. A file that passes
   replaces the old one, and the old one is kept as a revision. The command
   reads the file back, requires it to match byte for byte, and prints the
   revision name and what each of texture, management, management_task,
   research and engineering now resolves to.
4. When a model or provider changed (not for an effort change), make a real
   call: `choir ... policy verify --role <role>`.
5. Record the change in the mission ledger: what, why, the revision name.

Rollback: `choir ... policy rollback` restores the latest kept revision, or
`policy rollback --to <name>` one named by `choir ... policy revisions`. It
goes through the same validated write. The server keeps the last 20
revisions under `System/model-policy.revisions/`, named by UTC time.

The same five verbs are MCP tools (`policy_get`, `policy_set` with a
`content` argument, `policy_revisions`, `policy_rollback`, `policy_verify`).

Fallback, if the verbs are unavailable (an older build): save the current
file with `files get System/model-policy.toml`, edit a copy that keeps the
`# owner-edited` line, parse it locally
(`python3 -c "import tomllib; tomllib.load(open(f,'rb'))"`; an older server
does not validate, and a file that fails to parse falls back to the last
valid policy without telling the writer), `files put System/model-policy.toml
--local <copy>`, read it back with `files get` and compare byte for byte,
check `GET /api/model-policy/resolve?role=<role>` for each role, and roll
back by putting the saved copy.

## What this path lacks

- No sign-in for the owner: no GUI, and the CLI works only from the machine
  that holds the saved session.
- No schedule, health or restart verbs, and no policy `diff` verb.
- No record on the paper's tape that the policy changed or who changed it.

## Acts on record

- 2026-10-14, 2026-10-22, 2026-11-05: policy rewrites (see the ledgers of
  those dates; rollback copies under `~/.config/choir-qa/traces/`).
- 2026-11-09: texture, management and management_task from reasoning high
  to low, at the owner's word. Rollback copy
  `paper-model-policy-before-20261109.toml`.
