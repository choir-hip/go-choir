# Handover to node-a's agent: a backup target for Choir (2026-10-20)

For the agent that manages node-a for the other project. Written by the Choir
side; Choir does not log in to node-a or change it. Nothing here is applied
until you apply it. Read it, check the module, and tell the Choir owner what
you did.

## What this is

On 2026-10-14 the Choir owner asked for a stopgap: keep an encrypted copy of
Choir's Node B data on node-a until Choir gets a real offsite target.
(`docs/node-a-shared-host.md` has the earlier arrangement: node-a is also
Choir's Nix builder; that is unchanged and untouched.) Terms the owner set:
a quota-limited subvolume, ciphertext only, and a key that is never on
node-a. You have root here, so the design assumes you could read anything
stored in plaintext. You cannot read this: Node B encrypts with restic before
anything leaves it, and the password exists only on Node B and in the owner's
offline copy.

## What to apply

One file: `nix/node-a-choir-backup-target.nix` (self-contained NixOS module;
copy it into your config). Import it and enable it:

```nix
imports = [ ./node-a-choir-backup-target.nix ];
services.choirBackupTarget = {
  enable = true;
  # Supplied by the Choir owner (the output of `off-host-backup.sh keygen`
  # on Node B). One public key line, ssh-ed25519.
  nodeBPublicKeys = [ "ssh-ed25519 AAAA... node-b-offhost-backup" ];
  # quotaGiB = 450;        # default; the owner offered half to two thirds of the disk
  # dataDir  = "/data/choir-backup";   # must be on btrfs
};
```

Ask the Choir owner for the key line before you apply. Then place one more
thing by hand, not in Nix: `/var/lib/choir-backup/htpasswd` (root, mode 0600),
one line `nodeb:<bcrypt hash>`, also from the owner (it is a hash of a random
password; the password stays on Node B). Until that file exists the
rest-server unit stays inactive ("condition failed"); that is intended, it
never starts without authentication. After you place it, run
`systemctl start choir-backup-rest`.

Return to the Choir owner, out of band: your SSH host key fingerprint
(`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`) so Node B can pin it,
and the final quota you set.

## What it does

- **`choir-backup-subvolume.service`** (oneshot, root): creates the btrfs
  subvolume at `dataDir` if it does not exist (and refuses, doing nothing, if
  a plain directory is already there), owns it by the `choir-backup` user at
  mode 0700, enables btrfs quotas and sets a qgroup limit of `quotaGiB`.
  Idempotent. It never deletes or shrinks anything.
- **`choir-backup-rest.service`**: `rest-server` from nixpkgs, as user
  `choir-backup`, listening on `127.0.0.1:8000` only (the unit also has
  `IPAddressDeny=any`, `IPAddressAllow=localhost`), `--append-only`
  (it cannot delete or overwrite stored data), `--private-repos` (the Choir
  user reaches only its own `nodeb/` directory), HTTP auth from your
  `htpasswd` file, and a built-in size ceiling at 95% of the quota.
- **`choir-backup` SSH user**: no shell (`nologin`), no password. Its key is
  `from="147.135.70.196"` (Node B), `restrict`, may open a forward to
  `127.0.0.1:8000` and nothing else, and any command it asks for is
  `false`. An sshd `Match User` block repeats those limits and ends with
  `Match all` so it cannot capture lines you add after it.
- **`choir-backup.slice`**: `MemoryMax=2G`, `CPUQuota=100%` (one core),
  `IOWeight=20` (default is 100). It is low priority on purpose. Tune the
  `memoryMax`, `cpuQuota`, `ioWeight` options if you need more headroom.
- **`/var/lib/choir-backup`** (root, 0700) for the htpasswd files.
- Optional **prune window**, only if `prune.keys` is set (default: not
  defined at all): a second rest-server on `127.0.0.1:8001` **without**
  `--append-only`, not started by default, stops itself after
  `prune.maxRuntime` (2h). It exists so the owner can delete old backups a
  few times a year with a key Node B does not have. Start it only when the
  Choir owner asks (`systemctl start choir-backup-rest-prune`). Leave
  `prune.keys` empty unless they do.

## What it never does

- It never sees a decryption key, password or plaintext of Choir data.
- It opens no public port and changes no firewall rule (port 22 is already
  yours); it listens on loopback only.
- It gives Choir no shell, no root, no Nix access, and no access to anything
  outside the subvolume. It does not touch your services, your Caddy, your
  users, or the Nix builder module.
- It does not delete data, ever. The module has no cleanup and no garbage
  collection. Disabling it leaves the subvolume and its contents in place.
- It does not make node-a a standby, a log sink or a failover target.

## What we ask of you

- Do not delete or modify files under the `dataDir` without telling the Choir
  owner. It is Choir's only copy outside Node B.
- Do not run `restic` against it or try to open it. You cannot (no password),
  and a failed attempt only adds lock files.
- If you set `AllowUsers` or `AllowGroups` in sshd, add `choir-backup` (and
  `choir-backup-prune` if used). The `Match` block does not widen your
  existing policy.
- If disk pressure or the qgroup cost on the shared filesystem becomes a
  problem, tell the owner and lower `quotaGiB` or set `enforceBtrfsQuota =
  false` (the application ceiling still applies; see the module comment).
- Keep root's access to the box; nothing here needs it, but you will want
  it for the checks below.

## How to verify (read-only, none of these need Choir's secrets)

```sh
# 1. The subvolume and its limit
btrfs subvolume show /data/choir-backup
btrfs qgroup show -re /data/choir-backup        # max_rfer should be the quota

# 2. Listening only on loopback (after the htpasswd file is placed)
systemctl is-active choir-backup-rest
ss -ltnp | grep -E ':800[01]\b'                 # 127.0.0.1 only, nothing on 0.0.0.0

# 3. The service's restrictions
systemctl show choir-backup-rest -p IPAddressDeny -p IPAddressAllow -p Slice -p User
systemd-analyze security choir-backup-rest | tail -3

# 4. The SSH user can only forward to the rest port
sshd -T -C user=choir-backup,host=h,addr=147.135.70.196 | \
  grep -Ei '^(permitopen|allowtcpforwarding|forcecommand|permittty|authenticationmethods)'
getent passwd choir-backup                      # shell is nologin
grep -c '' /etc/ssh/authorized_keys.d/choir-backup   # one line, with restrict/permitopen

# 5. The prune server is absent or stopped
systemctl is-active choir-backup-rest-prune     # "inactive" or "could not be found"

# 6. Files and resources
stat -c '%a %U' /var/lib/choir-backup /var/lib/choir-backup/htpasswd   # 700 root, 600 root
systemctl show choir-backup.slice -p MemoryMax -p CPUQuotaPerSecUSec -p IOWeight
```

After the owner's first run from Node B (the owner will tell you), expect
under `dataDir`: `nodeb/daily/` and `nodeb/archive/`, each with
`config`, `keys/`, `data/`, `index/`, `snapshots/`, `locks/`. File contents
are random bytes (`head -c 64 <file> | xxd` shows no text). `du -sh` grows;
the first full upload is roughly 110 to 180 GB, then 4 to 12 GB a day, and
the repository reaches the quota in weeks to a couple of months (the Choir
side alerts at 80% and will tell you before it matters).

The append-only claim and the 403 on delete are checked from Node B's side in
the owner's first drill (`restic forget` must be refused), because only Node B
has the credentials to try. You can confirm the server was started with the
flag in `systemctl cat choir-backup-rest`.

## Removing it

Set `enable = false` and rebuild. That stops the services and removes the
SSH user and key. The subvolume and its contents stay; the module never
deletes them. Delete the subvolume yourself (`btrfs subvolume delete
/data/choir-backup`) only when the Choir owner says the backup is no longer
needed or has been moved to a real offsite target.

## References

- Design, risks and the client: `docs/problems/no-off-host-backup-2026-10-20.md`
- Owner ruling: `docs/reports/owner-notes-2026-10-14.md`, section 27 item 17
- Shared-host rules: `docs/node-a-shared-host.md`
