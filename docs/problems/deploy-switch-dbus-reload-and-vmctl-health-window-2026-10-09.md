# Deploy failed: NixOS switch dbus reload, then vmctl health window (2026-10-09)

CI run 37990333473 (cb138a59) failed in "Deploy to Staging (Node B)" at
21:07:46Z. Written 21:09Z. Problem first; no fix in this commit.

## Evidence (deploy job log, vmctl journal)

- 21:04:05 switch stops auth, corpusd, gateway, maild, proxy, vmctl;
  "activating the configuration" only at 21:05:35 (90 s of stopping).
- 21:05:36 reload dbus-broker; 21:07:06 "Failed to reload
  dbus-broker.service"; switch exit status 4. The script dumps
  diagnostics and retries once.
- 21:07:16 vmctl starts; it reattaches running VMs one by one with a
  guest health check, 3–10 s each. The deploy's 30-second health wait
  ended at 21:07:46, before vmctl finished (last reattach 21:07:58), and
  the job failed. vmctl answers health now; staging serves cb138a59; the
  release manifest and active-computer refresh steps did not run.
- Reattach skipped seven VMs ("guest health check failed for reattach"):
  the evening's disposable test computers. The owner's computer
  (`computer-03335285…`) reattached at 21:07:42 and answers health.

## What this shows

1. The vmctl health wait (30 s) is shorter than reattach takes with
   several VMs running; the deploy can fail although nothing is wrong.
2. The switch's dbus-broker reload can stall 90 s and fail; the retry
   path exists (logind check), but the whole switch costs ~3 min.
3. Unknown yet: why seven guests failed the reattach health check. They
   may have been idle-stopped or killed during the 90 s stop; to read
   from vmctl and console logs before calling it a deploy defect.

## Fix directions

- Wait for vmctl's reattach pass to finish (or scale the wait with the
  number of VMs) before judging health.
- Investigate the seven reattach skips before the next host deploy.

## First look at the skips

The vmctl stop left every firecracker process running ("remains running
after unit stopped", nine processes). Of the skipped guests, the run-5
computer's console ends at 20:04:24, an hour before the deploy, so that
skip is not the deploy's doing. The run-6 computer's console ends at
21:03:50, seconds before the switch began, mid-loop (Texture occurrence
`deferrals=53`, on the build before the research budget). Whether the
switch stopped it or it hung on its own is not settled.
