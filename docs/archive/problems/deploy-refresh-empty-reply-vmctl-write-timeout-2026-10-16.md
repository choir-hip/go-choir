# A deploy fails when a computer's refresh boot takes over 2 minutes: vmctl's HTTP write timeout (120 s) is shorter than its boot bound (2026-10-16)

Status: documented; fix in the next commit.

## Evidence

Deploy of b8feaa147, run 38931321339 attempt 1:

- 16:29:49: the deploy POSTs `/internal/vmctl/deploy-refresh` for the paper
  (`timeout 300 curl …`).
- 16:29:59 to 16:34:00: the paper boots with a replay tail. The full
  vocabulary rescan runs, and replay completes at 16:34:00.
- 16:34:00: vmctl logs `vmmanager: booted VM vm-1669…` and
  `deploy refresh … outcome=refreshed planned_restart=true`.
- 16:34:00: the deploy's curl reports `(52) Empty reply from server`,
  and the step fails ("Refresh failed … 000curl_error_52"). The job
  records incomplete evidence, and no deploy receipt is written.

The refresh succeeded. Only the HTTP answer was lost.

## Mechanism

- vmctl serves through `internal/server`, whose `WriteTimeout` comes from
  `SERVER_WRITE_TIMEOUT`, default 120 s (`internal/server/server.go:69`).
- vmctl's unit sets no `SERVER_WRITE_TIMEOUT`. It does set
  `VM_BOOT_READY_TIMEOUT=30m` and `VM_REPLAY_STALL_TIMEOUT=1500s`, so a
  refresh handler can legitimately run for many minutes.
- When the handler writes after 120 s, Go has already passed the
  connection's write deadline. The client sees an empty reply.
- The deploy's own bound is `timeout 300` on that curl.

Any refresh whose boot takes over 2 minutes fails the deploy. A replaying
boot of the paper takes about 4 minutes (the S0 rescan finding).

## Second defect, same step

The response body goes to a fixed path,
`/tmp/go-choir-vm-refresh-<vm>.json`, that is never cleared. With an
empty reply, the failure message printed the body from the 12:49 deploy
(the old `capture_label_override` refusal and IP 10.200.69.2). The
diagnosis it offered was wrong.

## Fix direction (red: vmctl, deployment routing)

- Give vmctl `SERVER_WRITE_TIMEOUT=10m30s`, as the gateway has.
- Raise the deploy's refresh curl bound from 300 s to 600 s, so the
  client gives up before the server does and a slow boot still answers.
- Remove the body file before each refresh call.

A boot over 10 minutes still fails the deploy, which is honest at that
length. The longer-term fix is the S0 one: stop the full rescan on a
replaying boot.
