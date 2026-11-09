# Paper control: acting on the Autopaper computer (2026-11-09)

How to change something on the paper (the `public_platform` computer that
publishes Autopaper) today, and what is missing. The designed replacement is
the operator role in
[platform-operator-account-2026-11-06.md](../design/platform-operator-account-2026-11-06.md)
section 5 (slices O2 to O5, not built); the CLI layer for it is in
[paper-control-cli-2026-11-09.md](../design/paper-control-cli-2026-11-09.md).

Acting on the paper needs the owner's word for that act. Reading does not.

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
restart.

1. Save the current file as the rollback. The server keeps no earlier
   revision.

   ```sh
   choir --computer <paper id> --api-key-file <key file> \
     files get System/model-policy.toml \
     > ~/.config/choir-qa/traces/paper-model-policy-before-$(date +%Y%m%d).toml
   ```

2. Edit a copy. Keep the `# owner-edited` line: without it the platform
   overwrites the file with its default at the next policy version. Say in
   the file's header what changed, when, and where the rollback copy is.
3. Parse the copy locally (`python3 -c "import tomllib; tomllib.load(open(f,'rb'))"`).
   The server does not validate; a file that fails to parse falls back to
   the last valid policy without telling the writer.
4. Write it: `choir ... files put System/model-policy.toml --local <copy>`
   (`PUT /api/files/System/model-policy.toml`, raw body).
5. Read back: `files get` matches the copy byte for byte, and
   `GET /api/model-policy/resolve?role=<role>` shows the model and effort
   for texture, management, management_task, research and engineering.
   `POST /api/model-policy/verify` makes a real model call; use it when a
   model or provider changed, not for an effort change.
6. Record the change in the mission ledger: what, why, the rollback file.

Rollback: `files put` the saved copy and read back.

## What this path lacks

- No sign-in for the owner: no GUI, and the CLI works only from the machine
  that holds the saved session.
- No policy verbs in the CLI (`get`, `set`, `verify`, `diff`, `rollback`);
  no schedule, health or restart verbs.
- No server-side validation, no earlier revisions, and no record on the
  paper's tape that the policy changed or who changed it.

## Acts on record

- 2026-10-14, 2026-10-22, 2026-11-05: policy rewrites (see the ledgers of
  those dates; rollback copies under `~/.config/choir-qa/traces/`).
- 2026-11-09: texture, management and management_task from reasoning high
  to low, at the owner's word. Rollback copy
  `paper-model-policy-before-20261109.toml`.
