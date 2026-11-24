# Host logs are kept under a day in one place and with no time bound in others

Recorded 2026-11-24. The privacy policy draft says server logs are kept "up to
30 days" and notes that nothing enforces it. Read on Node B (read-only) and in
the evaluated host configuration the same day:

| Log | What it holds | What kept it | What was on disk |
| --- | --- | --- | --- |
| The systemd journal (every host service's output) | request lines, account and computer ids, errors | No time bound. journald's default size ceiling, and `journalctl --vacuum-size=256M` in the disk sweep and in the deploy preflight's reclaim path | 203 MB; the oldest entry was 2026-11-23T06:52Z, about 21 hours old |
| Caddy access logs, one file per host name under `/var/log/caddy` | visitors' addresses, paths, user agents | Caddy's defaults: roll at 100 MB, keep 10 rolled files, 90 days | 177 MB; rolled files back to 2026-11-06; a 498-byte file for a quiet host last written 2026-05-26; files for a host name no longer configured |
| `btmp`, `wtmp` (login records) | remote addresses of ssh attempts | NixOS default: monthly, one old file, skipped under 1 MB | `btmp.1` 19 MB |

Read-only checks the same day: one journal folder (one machine id); coredumps are
already aged by systemd (two weeks); postgres and every service log to the
journal; there is no nginx, auditd or rsyslog.

So two defects with one cause (no log has a time bound):

1. **Too short.** At about 250 MB a day a 256 MB trim keeps under a day of the
   journal. The owner on 2026-11-23: "We can't lose logs". The backlog
   measurement of 2026-11-24 could reach back only 3.4 hours for this reason.
2. **Too long.** A quiet host's access log never reaches 100 MB, so it is never
   rolled and its entries never expire; a retired host name's files stay;
   login records reach about 60 days.

## The change

Every host log gets a time bound, and no size trim cuts it shorter in normal
running (`nix/node-b.nix`, `scripts/node-b-deploy-disk-preflight`,
`scripts/node-b-caddy-log-age`).

**The arithmetic, the same for every log.** A file is deleted whole, when its
newest entry is past the limit, by a deleter that runs once a day. So the
oldest entry is at most 1 day (the span of one file) + 28 days (the limit) +
1 day (the wait for the deleter) = **30 days**, and nothing is deleted before
it is 28 days old. The first version of this change used 29 days and so
reached 31 (and 32 for the sweep); the review caught it.

| Log | File span | Limit | Deleter | Worst case |
| --- | --- | --- | --- | --- |
| journal | `MaxFileSec=1day` | `MaxRetentionSec=28day` | journald, at each rotation (daily); also `--vacuum-time=28d` in the disk sweep | 30 days |
| access log, rolled files | a roll at 00:00 UTC (`roll_at`, a timer in Caddy) and at 100 MB | `roll_keep_for 672h` from the roll time in the name | Caddy, at each roll; and the age job daily by last write | 30 days |
| access log nobody rolls (quiet or retired host name) | set aside by the age job once unwritten for a day | 28 days from its last write | `go-choir-caddy-log-age`, daily | 30 days |
| btmp, wtmp | weekly | three old files, `maxage 28` | logrotate | 28 days |

- Journal ceiling `SystemMaxUse=12G` (30 days at today's volume is about
  7.5 GB).
- `roll_keep 64` is a count limit that must not bite before the time limit:
  two files a day for 28 days. Caddy's default of 10 would keep ten days.
- **Checked against the host's own Caddy 2.11.7** (2026-11-24, `caddy adapt`
  on the log block, nothing written): `roll_at ["00:00"]`, `roll_keep 64`,
  `roll_keep_days 28`. Read from Caddy 2.11.7 `modules/logging/filewriter.go`
  and timberjack 1.4.8: `roll_keep_for` is whole days rounded up; cleanup
  goes by the time in a rolled file's name and runs only at a roll or at the
  first write after a start; `roll_interval` rolls only when a request
  arrives, counted from the start of the process, which is why `roll_at` is
  used; a rolled name is `<name>-<UTC time>-<reason>.log`.
- **The tmpfiles sweep of the first version is gone.** It could unlink a file
  Caddy held open. The age job instead deletes by last write, and renames a
  live file not written for over a day to a rolled name: a file Caddy is
  rolling is never that old, and if Caddy does hold it, its writes follow
  the rename and its next roll starts a new file, so no line is lost.
- **A nearly full disk may shorten the journal.** The deploy preflight trims
  by time in its reclaim path, and only if that reclaim has not freed enough
  does it trim the journal to 2 GiB, announced as a last resort. journald's
  own keep-free default also trims under disk pressure. So the promise is
  "up to 30 days", not "30 days".

Tests: `scripts/test-node-b-log-retention.sh` (L1 to L7; both ends of every
bound; fails without nix or python3; runs a copy of the preflight beside a
stub, never the real store cleanup) and `scripts/test-node-b-caddy-log-age.sh`
(A1 to A8), both in `scripts/gates`.

## Remaining error

- **Access logs go from 90 days to 30.** That is a loss for abuse
  investigation and follows from the policy draft's number, not from an
  owner ruling on access logs: it is the owner's to confirm.
- The journal grows from about 0.25 GB to about 7.5 GB on a disk with 147 GB
  free.
- An entry can outlive the bound in one case: Caddy restarts every day
  before midnight for weeks and a quiet host gets a request less than a day
  apart each time, so neither Caddy nor the age job ever rolls the live
  file. A busy host rolls by size; a deploy does not restart Caddy daily.
- The journal's active file is only aged once rotated; Node B writes
  constantly, so it rotates daily.
- `last` and `lastb` show the current week by default; older weeks are in
  `wtmp.1` to `wtmp.3`.
- Not covered: logs inside a person's computer (that computer's data, gone
  when it is destroyed), the gateway's usage and receipt files (their own
  retention row), GitHub's CI logs, and nine files named `*.log` under
  `/var/lib/go-choir` from August and September (names say build and import
  output; their contents were not read). The policy sentence should say
  "request, service and login logs".
- Not yet proved on Node B. The deployed proof, read-only, after the deploy:
  `ssh node-b 'systemd-analyze cat-config systemd/journald.conf | grep -E "MaxRetentionSec|MaxFileSec|SystemMaxUse"; sudo grep -c "roll_keep_for 672h" /etc/caddy/caddy_config; systemctl is-active caddy; systemctl list-timers go-choir-caddy-log-age --no-pager | grep -c caddy-log-age; journalctl --disk-usage'`
  expects the three journal lines (28day, 1day, 12G), 2, active, 1. After the
  first midnight UTC: `sudo ls /var/log/caddy | grep -c -- "-time.log"` is at
  least 1 (a roll happened) and `journalctl -u go-choir-caddy-log-age -n 3`
  shows the job's count line. A week later the oldest journal entry is a
  week old; after 2026-12-24 `sudo find /var/log -type f -mtime +30` is empty.
