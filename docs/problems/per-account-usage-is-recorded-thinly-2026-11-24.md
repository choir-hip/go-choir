# Per-account usage is recorded thinly: enough for a daily look, not for a history

Recorded 2026-11-24. The owner ruled monitoring, not a spend bound, for
friendly testers ("just good monitoring of usage"). Before building a view,
what Node B records per account was read from code and from the host
(read-only).

| Wanted | What is recorded | Where | Gap |
| --- | --- | --- | --- |
| Model calls per account | calls and input tokens per hour, computer and provider, as a log line | gateway journal (`internal/gateway/call_tally.go`) | a log line, not a table; reaches back only as far as the journal (under a day on 2026-11-24; 28 days once `lane/log-retention` lands); keyed by VM id, not account |
| Tokens | input tokens in that line; input, cached and output per provider per day in the spend file | journal; `/var/lib/go-choir/paid-spend.json` | output tokens per computer are not recorded anywhere |
| Cost | dollars per provider and computer | the spend file (`internal/gateway/paid_budget.go`) | today and the previous UTC day only; no history; free providers are $0 |
| Whose computer | computer, owner, state, last active, memory | vmctl `ownerships.json` | none for this purpose |
| Computer hours | nothing | | not recorded: only the current state and last-active time |
| Storage | the data image's size per VM | growth meter ledger | hourly rows; some rows say "mixed" instead of a size |
| Accounts | the users table | auth.db | none |

**What exists now:** `scripts/status-account-usage`, a read-only report over
one ssh that joins those sources by owner: per account, computers, calls and
input tokens over 24 hours and the window, dollars today and yesterday, disk,
memory and the top providers. It reads no email address (accounts are user
ids), raises no flag and sets no bound. It says how far back the call lines
reach, so a short journal is not read as low use. First run, 2026-11-24:
1,097 accounts, 4 with a computer; 29,103 calls in the reach; the paper's
account made 28,559 of them.

**Belief:** this is enough to watch a handful of friendly testers day by day.
It is not enough to answer "what did this account cost last week", and it
cannot say how long a computer ran.

**Remaining error, in order of worth:**
1. No durable per-account history. The smallest fix is the gateway writing
   its hourly tally (with output tokens and dollars) to a table or an
   append-only file kept 13 months, keyed by computer; the report then reads
   that instead of the journal.
2. Computer hours are not recorded. vmctl knows every start and stop; an
   hourly "running" row per computer in the growth meter's ledger would do.
3. The report is a developer's script over ssh. An operator route or a page
   is worth building only once there are testers to watch.
4. An account that owns no computer makes no gateway call, so it does not
   appear; 1,093 of 1,097 accounts are such (most are test accounts, to be
   deleted by the held operator command).
