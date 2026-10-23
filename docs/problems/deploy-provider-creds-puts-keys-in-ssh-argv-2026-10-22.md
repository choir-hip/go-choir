# deploy-provider-creds.sh's full mode puts every provider key in ssh's arguments

Status: open. This record documents the problem first; this commit has no
fix. Mutation class of a fix: orange (operator script). The replacement is
the provider key vault
(`docs/provider-vault-and-model-policy-design-2026-10-22.md` §2.8).

Found 2026-10-22 by reading code. No value was read or printed.

## What the script does

- `nix/deploy-provider-creds.sh` sources the repo `.env` (`:50-57`) and
  builds `ENV_CONTENT` from the provider keys (`:150-193`).
- Full mode passes that content inside the remote command string of
  `ssh "$HOST" "cat > … << 'PROVIDER_ENV_EOF' ${ENV_CONTENT} …"`
  (`:208-215`). Every key value is therefore part of the local `ssh`
  process's argument vector for the life of the call. Any local user who
  can list processes can see it, as can process accounting or audit
  logging if either is on. The remote shell receives it as a command
  string.
- Upsert mode (`CHOIR_PROVIDER_ONLY_KEYS`, `:59-89`) sends values on
  stdin. That path is safe from this problem. The script's comment at
  `:75` says so.
- The Codex auth file is copied with `scp`, which is not affected
  (`:197-204`).

## Impact

The exposure lasts only while the command runs, and only on the owner's
laptop and the remote shell. It is still a credential in argv: the class
of leak that the kernel-command-line token records closed for gateway
tokens (`docs/problems/s0-gateway-token-on-kernel-cmdline-2026-10-01.md`).

## Interim rule

Until the vault lands, send a key with upsert mode only, for example
`CHOIR_PROVIDER_ONLY_KEYS="SYNTHETIC_API_KEY"`. Do not run full mode.
