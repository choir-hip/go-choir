# CI deploys to Node B as root, and stdin is the deploy

Date: 2026-10-21. Item: SDC SEC-14, second half (review F13,
`docs/reports/computer-isolation-security-review-2026-10-14.md` §3, "CI is
root on Node B"). Mutation class of the fix: red (deployment routing).

## Problem

The GitHub Actions deploy key (`github-actions-deploy@go-choir`) is one of
root's two authorized keys on Node B (`nix/node-b.nix`, the
`users.users.root.openssh.authorizedKeys.keys` list), with no key options.
Anyone holding the key gets an unrestricted root shell. The workflow uses
it like a shell: it sends the whole deploy program over ssh stdin
(`ssh go-choir-node-b "<env> bash -s" <<'REMOTE_DEPLOY'`), so Node B runs
whatever arrives on stdin as root.

The key sits in the `OVH_DEPLOY_SSH_KEY` secret and is written to disk in three
jobs. Each of these hands an attacker root on the host that holds every
tenant's computer:

- a leaked secret, or a runner that leaks it;
- a compromised third-party Action in a job that holds the key;
- a workflow change that reaches a job holding the key.

SEC-14's first half (6bfe06fcf) pinned Node B's host keys, so CI now
authenticates the host. It did not narrow what the key can do.

## Evidence (code, read 2026-10-21 at 961588290)

Every remote command CI runs on Node B, all as `root@$OVH_NODE_B_HOST`:

1. `deploy-impact`, "Read deployed base receipt from Node B" (best effort):
   `cat /var/lib/go-choir/deploy-receipt.json`. Read only.
2. `prebuild-staging` (continue-on-error, never activates): `<env> bash -s`
   with the `REMOTE_PREBUILD` heredoc on stdin. It runs:
   - git clone, fetch, reset and clean in `/opt/go-choir-prebuild`, plus
     `git config --global` safe.directory;
   - `install -d /var/lib/go-choir/prebuild-results`, then `nix store gc`;
   - `nix build -o` for `.#frontend`, `.#<service>` or the
     `go-choir-b` toplevel.
   Its environment holds DEPLOY_COMMIT, DEPLOY_FRONTEND, DEPLOY_HOST,
   DEPLOY_HOST_OS and HOST_SERVICES.
3. `deploy-staging`: `<env> bash -s` with the `REMOTE_DEPLOY` heredoc
   (about 900 lines) on stdin. It runs:
   - git in `/opt/go-choir`, then `scripts/node-b-deploy-disk-preflight`;
   - `nix build`, `nix-env -p /nix/var/nix/profiles/system --set`,
     `switch-to-configuration switch`, and `nix hash path`;
   - `systemctl` (reset-failed, restart, start, reload, daemon-reload, show,
     is-active), `loginctl` and `journalctl`;
   - `scripts/node-b-sync-service-pointers`, which writes
     `/var/lib/go-choir/services/*`;
   - the frontend swap under `/var/www/go-choir`;
   - receipt writes (`deploy-receipt*.json`, `deploy-failures/`,
     `deploy.env`, `platform-release.json` via `go run`);
   - a read of the root-only `/var/lib/go-choir/secrets/internal-service-token.header`
     for curl calls to vmctl's internal routes;
   - `python3 scripts/verify-host-hardening.py`.
   Its environment holds the five prebuild variables plus DEPLOY_RUN_ID,
   DEPLOY_RUN_ATTEMPT, DEPLOY_VMCTL_RESTART, DEPLOY_ACTIVE_VM_REFRESH and
   REFRESH_ACTIVE_COMPUTERS.

A NixOS switch, systemd control and the root-only token need root, so a
sudo rule per command cannot express the deploy. What can be narrowed is
who chooses the program: today the caller does.

## Belief state

- A deploy can stay root on the host. The program it runs has to come from
  the repository at a commit on `main`, chosen by the host, not sent by the
  caller. With that rule, a stolen key can at most deploy a commit already
  on main, and pick its booleans: an availability and downgrade risk, not
  arbitrary root.
- GitHub serves any commit reachable in the repository network by SHA,
  including `refs/pull/*` commits from forks. Fetching the SHA is not
  enough; the host must check that the commit is an ancestor of `main`.

## Remaining error field

- Downgrade: a key holder can redeploy an older main commit. CI's
  stale-guard stops honest reruns; the host does not refuse them.
- The booleans: a key holder can force `REFRESH_ACTIVE_COMPUTERS=true`,
  which interrupts busy computers.
- Root keeps accepting the CI key until that key line is removed. That is
  a separate step, after the switch is proven.
- Actions are pinned by commit SHA already (checkout at d23441a4). Whether
  every Action in the key-holding jobs is pinned is not re-audited here.

Fix: `docs/definitions/choir-launch-fp10-sec14-deploy-user-2026-10-21.md`.
