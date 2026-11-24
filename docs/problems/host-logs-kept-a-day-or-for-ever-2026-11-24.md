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

## The change (third frame, 2026-11-24; class BLACK)

**This change deletes files in production by deploy, so it is black and needs
the owner's word before it lands.** The exact list is under "What the first
run deletes".

Two reviews sent back versions that bounded each log's files by when they
were last written. That is the wrong quantity: a file written yesterday can
hold an entry from months ago, and Caddy rolls a file only after it has
written to it since it started (timberjack 1.4.8 starts its roll timer inside
`Write`), so a quiet host's file has no bound at all. The frame changed, and
a third review accepted the frame and found two rendering slips, which a
gate reading option lists could not see (below).

1. **The promise is an upper bound only.** Nothing promises a minimum: a
   nearly full disk may delete sooner (the deploy preflight trims the journal
   to 2 GiB as an announced last resort, and may still refuse the deploy
   afterwards; journald's keep-free default also trims).
2. **Limits of 21 days, so the worst case has margin.**
3. **Deletion goes by a file's oldest entry**, and files are closed on a
   schedule that does not depend on traffic.
4. **One mechanism for request and service logs.** Caddy's access log goes to
   the journal, not to files.
5. **The gate reads what is rendered**: the unit and timer files, the
   logrotate configuration and journald.conf as NixOS writes them.

| Log | How it is bounded | Worst case |
| --- | --- | --- |
| journal: every service, and the web access log | `MaxRetentionSec=21day`. journald names a closed file by its first entry's time and deletes by that (systemd `journal-vacuum.c`, read). The job `go-choir-log-age` runs `journalctl --rotate` (returns when the rotation is complete, journalctl(1); closes the open file whether or not anything was written), then `--vacuum-time=21d`, then CHECKS that no closed file with a first entry past the limit is left | 1 (a file's span, added though deletion is by first entry) + 21 + 1 (wait for the job) + 1 (a missed run) = **24 days** |
| access-log files left under `/var/log/caddy` | the job deletes `access-*.log` and `access-*.log.gz` whose first line (the oldest entry) is past 21 days, dated in the future, or unreadable. Caddy no longer writes there. Any other file in the folder fails the job | 23 days |
| `btmp`, `wtmp` (login attempts, with addresses) | logrotate: daily files, 21 kept, `maxage 21`, rotated when small and when empty. The age job runs logrotate first, then deletes any rotated file last written past 21 days itself and fails if logrotate's record shows a live file not rotated for over two days: so the job's success covers these too | 1 + 21 + 1 + 1 = **24 days** |
| `lastlog`, `lastlog.migrated`, `lastlog2.db` and its side files | removed at every run of the job; sshd no longer prints it. One record per account, so there is no age to go by | 1 day (2 with a missed run) |

**One job's success means the whole bound.** `go-choir-log-age` deletes or
checks every log in the table, including the login records logrotate
rotates. It deletes what is known to be safe, checks the rest, and fails at
the end if any check failed. `LOG_AGE_DRY_RUN=1` deletes nothing, does not
touch the journal, and prints `would delete <path>` for each file a real run
would delete now: its output on Node B is the list the owner approves.

**Running and failing.** Both the job and logrotate run on their mark (daily,
hourly), make up a missed mark (Persistent), and run two minutes after every
boot. A failure of the job exits non-zero, so the unit shows in
`systemctl --failed` (what the host's liveness probe and hardening check
read), and prints the reason at error priority
(`journalctl -u go-choir-log-age -p err`). Nothing pages anyone: that is the
host's named gap, not new here.

**What still can exceed 30 days, said plainly:**

- A host that is **off** keeps what is on its disk. Off for more than about
  six days with entries near the limit, it has entries over 30 days old from
  boot until the job runs, two minutes later (logrotate the same).
- **The job fails for a week** and nobody reads `systemctl --failed`.
- A **second journal folder** (another machine id after a restore, a journal
  namespace): the trim does not reach it. None exists today; the job fails
  loudly when one appears.

**The two sentences, for the owner to choose:**

- Absolute: "Our servers' request, service and login logs are deleted within
  30 days." True except in the three cases above; the first is minutes after
  a long outage, the other two are failures the job reports.
- Qualified (the reviewer's wording): "We keep our production host's request
  logs, service logs and login records for at most 30 days while the host is
  running and the daily retention job succeeds; a nearly full disk can delete
  them sooner, and a failed job or a host powered off for more than about a
  week can delay deletion."

The lead's condition for the absolute one was that every hole is closed.
They are narrowed, not closed: an off host cannot delete. **So the qualified
sentence is the one that is strictly true; the absolute one is the owner's
call.**

**Neither sentence covers** (copied into `docs/ship/privacy-policy.md`):

- a person's own computer and everything on it, including capsule job logs
  under its artifact root (`internal/capsule/jobs.go`): kept for the life of
  the computer;
- the gateway's usage and receipt files (their own retention row);
- GitHub's CI logs (GitHub's retention);
- build job records under `/var/lib/go-choir/host-build/jobs` (80 small JSON
  files, 2026-11-17 onward);
- one-off operator reports that documents cite and that stay:
  `/var/lib/go-choir/migration-reports/` (table names and row counts),
  `/var/lib/go-choir/taperepack-work/`, `/var/lib/go-choir/dump-20260826/`;
- core dumps (4 files today; not logs; their ageing was not verified);
- login-failure counters (`pam_faillock` is in a PAM stack; no record folder
  exists) and `wtmpdb`, `faillog`, `tallylog` (absent);
- journals of another machine id or a namespace, and per-user journals
  (none of either today; the job fails when one appears).

**What the first run deletes on Node B** (dry listing, read-only,
2026-11-24; nothing was deleted to produce it):

| What | Count | Detail |
| --- | --- | --- |
| access-log files whose first entry is past 21 days | 5 files, 103 MB | one host name's three rolled files and its live file (first entries March to May 2026) and another's live file (May 2026) |
| access-log files kept, then deleted over the next 21 days | 11 files | first entries 2026-11-04 to 11-23; the oldest goes the day after the deploy |
| other files in `/var/log/caddy` | 0 | |
| last-login records | 3 files | `lastlog` (292 bytes), `lastlog.migrated` and `lastlog2.db` (empty) |
| dead operator outputs | 3 files, 29 kB | `rebuild-0333528.log`, `rebuild-near-head-20260909.log`, `paper-repair-20261029T223916Z.log`, directly under `/var/lib/go-choir`; no program or document names them; contents not read |
| by logrotate at its next run | `btmp.1` (19 MB, 2026-10-31) deleted; `btmp` and `wtmp` start daily files | |
| journal | nothing | its oldest entry is a day old |

Journal folders: 1. User journals: none.

**Checked against primary sources.** The host's own Caddy 2.11.7 parsed
`output stderr` with `format json` (`caddy adapt`, nothing written).
timberjack 1.4.8 `timberjack.go`: the scheduled-roll loop starts in `Write`.
systemd `journal-vacuum.c`: an archived file's time is read from its name
(the head entry). The pinned NixOS logrotate module renders a `false`
boolean as `no<name>` (so `notifempty = false` became `nonotifempty`, which
logrotate rejects), and `serviceConfig.Environment` is written raw (systemd
kept only the first path of a list): both seen in the evaluated text, both
fixed, and the gate now fails on either. A header value that contains
`"ts":` is written with escaped quotes, so it cannot be mistaken for the
entry's own time; the job takes the first match anyway.

Tests: `scripts/test-node-b-log-retention.sh` (L1 to L8, on the rendered
files; fails without nix or python3) and `scripts/test-node-b-log-age.sh`
(A1 to A14), both in `scripts/gates`. Each reviewed defect was put back in
turn and the gate seen to fail. The gate evaluates `go-choir-b` only; the
other system in the flake is the guest image and has none of these logs. The
build's own `logrotate --debug` check stays on.

## Remaining error

- **Access logs go from 90 days to at most 24** and login-attempt records
  from about 60 to about 23. That follows from the policy's number, not from
  an owner ruling: the owner's to confirm with the deletion list.
- The access log is read with `sudo journalctl -u caddy -o cat`, mixed with
  Caddy's own messages. A line over journald's 48 KiB limit is split and is
  no longer one JSON line. Caddy now writes each line to journald on the
  request path; whether that slows requests under load was not measured.
  The journal grows from about 0.25 GB to about 8 GB (ceiling 12 GB); a
  flood of requests can push service logs out early.
- `last` no longer shows the previous login at ssh sign-in (a small tamper
  hint, lost).
- **Named risk: `pam_lastlog2` with its database deleted.** It is in the
  `login` and `su` PAM stacks as `session required`. It could not be tested
  (no Linux host to try it on but Node B). If it fails when
  `/var/lib/lastlog/lastlog2.db` is missing, console login and `su` fail;
  ssh sign-in does not use that stack. The database is empty today (0
  bytes), which is the state a deletion leaves it one step from. Check in
  the deployed proof, from an ssh session that stays open:
  `sudo su -s /bin/sh -c true nobody; echo $?` prints 0 after the job's
  first run. Rollback if it does not: revert, or at once on the host
  `sudo touch /var/lib/lastlog/lastlog2.db`, and remove the three
  `lastlog2.db` paths from the job's list in the next change.
- The 2 GiB last resort can trim the journal and still refuse the deploy.
- Caddy's configuration is not parsed in the gate (no Caddy on the Mac); the
  log block was parsed by the host's Caddy by hand.
- Not yet proved on Node B. Before the landing, the lead runs the dry run
  there (read-only) and the owner approves its list. The deployed proof,
  read-only, after the deploy (make one request to the site first):
  `ssh node-b 'sudo systemd-analyze cat-config systemd/journald.conf | grep -E "MaxRetentionSec|MaxFileSec|SystemMaxUse"; sudo grep -c "output file" /etc/caddy/caddy_config; systemctl is-active caddy; sudo journalctl -u caddy -o cat -n 200 | grep -c http.log.access; sudo logrotate -d /etc/logrotate.conf 2>&1 | grep -c -i "error"'`
  expects the three journal lines (21day, 1day, 12G), 0, active, a count
  above 0, and 0. After the job's first run,
  `sudo journalctl -u go-choir-log-age -o cat -n 20` shows the count line.
  **Asserted:** `failed=0`, `journal_folders=1`, `access_other=0`;
  `sudo ls /var/log/lastlog /var/lib/lastlog/lastlog2.db /var/lib/go-choir/rebuild-0333528.log /var/lib/go-choir/rebuild-near-head-20260909.log /var/lib/go-choir/paper-repair-20261029T223916Z.log 2>&1 | grep -c "No such"`
  is 5; `systemctl --failed | grep -c log-age` is 0; the `su` check above
  prints 0. **Recorded as data, not asserted:** the other counts on the line
  (the dry listing of 2026-11-24 gave 5 access-log files, 3 last-login
  files, 3 dead files). After 2026-12-18 the oldest journal entry
  (`sudo journalctl -o short-iso | head -1`) is under 25 days old.
