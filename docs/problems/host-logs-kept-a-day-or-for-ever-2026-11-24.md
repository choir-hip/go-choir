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

## The change (third frame, 2026-11-24)

Two reviews sent back versions that bounded each log's files by when they
were last written. That is the wrong quantity: a file written yesterday can
hold an entry from months ago, and Caddy rolls a file only after it has
written to it since it started (timberjack 1.4.8 starts its roll timer inside
`Write`), so a quiet host's file has no bound at all. The frame changed:

1. **The promise is an upper bound only.** No host log entry is kept longer
   than 30 days. Nothing promises a minimum: a nearly full disk may delete
   sooner (the deploy preflight trims the journal to 2 GiB as an announced
   last resort; journald's keep-free default also trims).
2. **Limits of 21 days, so the worst case has margin.**
3. **Deletion goes by a file's oldest entry**, and files are closed on a
   schedule that does not depend on traffic.
4. **One mechanism for request and service logs.** Caddy's access log goes to
   the journal, not to files, so there is nothing for Caddy to roll or
   forget.

| Log | How it is bounded | Worst case |
| --- | --- | --- |
| journal: every service, and the web access log | `MaxRetentionSec=21day`. journald names a closed file by its first entry's time and deletes by that (systemd `journal-vacuum.c`). The daily job `go-choir-log-age` runs `journalctl --rotate` (closes the open file whether or not anything was written) and `--vacuum-time=21d` | 21 + 1 (wait for the job) + 1 (a missed run) = **23 days** |
| access-log files left under `/var/log/caddy` from before | the daily job deletes a file when its first line (the oldest entry) is past 21 days or cannot be read. Caddy no longer writes there, so no open file is unlinked | 23 days |
| `btmp`, `wtmp` (login attempts, with addresses) | logrotate: daily files, 21 kept, `maxage 21`, rotated when small and when empty | 1 (a file) + 21 + 1 + 1 = **24 days** |
| `lastlog`, `lastlog.migrated`, `lastlog2.db` (last login per account, with an address) | removed every day by the job; sshd no longer prints it (`PrintLastLog no`). One record per account, so there is no age to go by | 1 day (2 with a missed run) |

A host that is off keeps what is on its disk: the margin is 6 to 7 days, and
the timers are Persistent, so the job runs when the host comes back.
`scripts/test-node-b-log-retention.sh` computes these worst cases from the
evaluated settings and fails over 30 days or with under 5 days of margin. Its
7-day lower limit is our operating floor against a typo, not policy.

**Login records found on Node B (read-only, 2026-11-24):** `btmp` 4 MB written
that day, `btmp.1` 19 MB from 2026-10-31, `wtmp` 23 kB last written
2026-10-22, `lastlog` 292 bytes last written 2026-10-20, `lastlog.migrated`
and `/var/lib/lastlog/lastlog2.db` empty. No faillog, tallylog or wtmpdb.
PAM: `pam_lastlog2` in the `login` and `su` stacks, `pam_faillock` in one;
there is no `/run/faillock` folder.

**Checked against primary sources.** The host's own Caddy 2.11.7 parsed
`output stderr` with `format json` (`caddy adapt`, nothing written): one
access logger, writer `stderr`, encoder `json`. timberjack 1.4.8
`timberjack.go`: the scheduled-roll loop starts in `Write`. systemd
`journal-vacuum.c`: an archived file's time is read from its name (the head
entry) and compared with the retention limit.

**The exact sentence the policy may use:** "Our servers' request, service and
login logs are deleted within 30 days." It does not cover:

- a person's own computer (its logs are that computer's data, gone when the
  computer is destroyed);
- the gateway's usage and receipt files (their own retention row);
- GitHub's CI logs (GitHub's retention);
- build job records under `/var/lib/go-choir/host-build/jobs` (80 small JSON
  files, 2026-11-17 onward; build inputs and results, not requests);
- one-off operator outputs that documents cite as evidence and that stay:
  `/var/lib/go-choir/migration-reports/` (the 2026-10-30 data cutover: table
  names and row counts) and `/var/lib/go-choir/taperepack-work/`.

**Seven dead files are deleted by the job**, by exact path: three one-off
operator outputs directly under `/var/lib/go-choir` (two rebuild logs from
August and September, one repair log from October) and the four files of
`dump-20260826/` (three are empty). No program or document names them. Their
contents were not read.

Tests: `scripts/test-node-b-log-retention.sh` (L1 to L8; fails without nix or
python3; runs a copy of the preflight beside a stub, never the real store
cleanup) and `scripts/test-node-b-log-age.sh` (A1 to A8), both in
`scripts/gates`. Each setting was broken in turn and the gate seen to fail.

## Remaining error

- **Access logs go from 90 days to at most 23**, and the first run deletes
  the files older than 21 days at once. That follows from the policy's
  number, not from an owner ruling on access logs: the owner's to confirm.
- **Deleting files on Node B by deploy** (old access logs, the last-login
  records, the seven dead files) is deletion in production. It is what the
  policy asks for; the lead and the owner should see this list before it
  lands.
- The access log's lines are read with `journalctl -u caddy -o cat`, mixed
  with Caddy's own messages, and no longer from a file per host name. The
  journal grows from about 0.25 GB to about 8 GB on a disk with 147 GB free;
  if volume outgrows the 12 GB ceiling it keeps fewer days.
- `last` no longer shows the previous login at ssh sign-in. That line is a
  small tamper hint for an operator; it is lost.
- `pam_faillock` is in a PAM stack; no record folder exists today. If one
  appears it is not aged by this change.
- The wording of journald's own rotation (`MaxFileSec`) is not relied on:
  the job's `--rotate` is. If the job fails for three weeks the open file is
  not bounded; the job exits non-zero on any failure, so the unit shows
  failed.
- Not yet proved on Node B. The deployed proof, read-only, after the deploy:
  `ssh node-b 'systemd-analyze cat-config systemd/journald.conf | grep -E "MaxRetentionSec|MaxFileSec|SystemMaxUse"; sudo grep -c "output file" /etc/caddy/caddy_config; systemctl is-active caddy; systemctl list-timers go-choir-log-age --no-pager | grep -c go-choir-log-age; journalctl -u caddy -o cat -n 50 | grep -c http.log.access'`
  expects the three journal lines (21day, 1day, 12G), 0, active, 1, and a
  count above 0. After the job's first run:
  `journalctl -u go-choir-log-age -o cat -n 3` shows its count line with
  `failed=0`, and `sudo ls /var/log/lastlog 2>&1` says no such file. After
  2026-12-17 the oldest journal entry (`journalctl -o short-iso | head -1`)
  is under 24 days old.
