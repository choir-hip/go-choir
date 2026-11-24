# Host logs are kept under a day in one place and with no time bound in others

Recorded 2026-11-24. The privacy policy draft says server logs are kept "up to
30 days" and notes that nothing enforces it. Read on Node B (read-only) and in
the evaluated host configuration the same day:

| Log | What it holds | What kept it | What was on disk |
| --- | --- | --- | --- |
| The systemd journal (every host service's output) | request lines, account and computer ids, errors | No time bound. journald's default size ceiling, and `journalctl --vacuum-size=256M` in the disk sweep and in the deploy preflight's reclaim path | 203 MB; the oldest entry was 2026-11-23T06:52Z, about 21 hours old |
| Caddy access logs, one file per host name under `/var/log/caddy` | visitors' addresses, paths, user agents | Caddy's defaults: roll at 100 MB, keep 10 rolled files, 90 days | 177 MB; rolled files back to 2026-11-06; a 498-byte file for a quiet host last written 2026-05-26; files for a host name no longer configured |
| `btmp`, `wtmp` (login records) | remote addresses of ssh attempts | NixOS default: monthly, one old file, skipped under 1 MB | `btmp.1` 19 MB |

So two defects with one cause (no log has a time bound):

1. **Too short.** At about 250 MB a day a 256 MB trim keeps under a day of the
   journal. The owner on 2026-11-23: "We can't lose logs". The backlog
   measurement of 2026-11-24 could reach back only 3.4 hours for this reason.
2. **Too long.** A quiet host's access log never reaches 100 MB, so it is never
   rolled and its entries never expire; a retired host name's files stay;
   login records reach about 60 days.

## The change

Every host log gets a time bound of at most 30 days and no size trim that
cuts it shorter (`nix/node-b.nix`, `scripts/node-b-deploy-disk-preflight`):

- journal: `MaxRetentionSec=29day`, `MaxFileSec=1day` (a file goes when all
  of it is past the bound, so no entry passes 30 days), `SystemMaxUse=12G`
  (30 days at today's volume is about 7.5 GB); both trims become
  `--vacuum-time=29d`.
- access logs: each host rolls every 24 hours, rolled files go after 29 days,
  no count limit; `/var/log/caddy` is swept of files untouched for 30 days.
- login records: weekly, three old files, 28 days, no minimum size.

The test is `scripts/test-node-b-log-retention.sh` (failure modes L1 to L6),
run by `scripts/gates`.

## Remaining error

- The journal grows from about 0.25 GB to about 7.5 GB on a disk with 147 GB
  free. If volume grows past 400 MB a day the 12 GB ceiling keeps fewer than
  30 days; it never keeps more.
- Caddy applies its age limit when it next rolls a file, so the first cleanup
  of old rolled files is up to a day after the deploy. A quiet host's live
  file is removed by the sweep after 30 untouched days while Caddy may still
  hold it open; lines written before Caddy's next roll would be lost. That
  host gets a few requests a year.
- Not covered: logs inside a person's computer (they are that computer's data
  and go when it is destroyed), the gateway's usage records in its database
  (billing, a different retention row), GitHub's CI logs, and files under
  `/var/lib/go-choir` named `*.log` from August and September builds (nine
  files, build output, no visitor data; left for a separate cleanup).
- Not yet proved on Node B. The deployed proof, after the deploy:
  `ssh node-b 'systemd-analyze cat-config systemd/journald.conf | grep -E "MaxRetentionSec|MaxFileSec|SystemMaxUse"; grep -c "roll_keep_for 696h" /etc/caddy/Caddyfile; grep -c "/var/log/caddy" /etc/tmpfiles.d/*.conf; journalctl --disk-usage'`
  expects the three journal lines, 2, at least 1, and a size that then grows
  day by day; a week later the oldest journal entry is a week old, and after
  2026-12-24 no file under `/var/log` is older than 30 days.
