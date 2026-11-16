# The hourly status script says no search was answered while search is answering

2026-11-16, 03:45Z.

## Problem

`scripts/status-hourly` printed, three hours running (01:39Z, 02:35Z, 03:35Z),
"search, last hour: N requests, 0 answered by 0 providers (0.0%)", with Brave
`quota_limited` on nearly every request.

## Evidence

The gateway's own lines for the paper at 03:40:02Z say otherwise: "search for
autoputer vm-1669… answered 1 of 1 providers (2 cooling, 0 pending)
skipped=[searxng=success_empty brave=quota_limited] served=miss results=40
unique=40". The paper's searches return 40 results each.

## Belief

The script's count of answers is wrong (it likely counts only the providers
named in the `skipped` list, or a line shape that changed), not the search
plane. Brave is out of quota and is skipped; another provider answers.

## Remaining error

Which provider is answering was not read (the line is cut before it). The
script's counting code has not been read. Fix is a script change (yellow).
A second fact rides here, unexplained: at 02:35Z the paper's data image read
43.2 GiB allocated, down from 84 GiB at 01:39Z, and Node B's free space rose
36 GiB in that hour. The 02:15Z deploy refreshed the paper in between. The
likely reading is that the refresh released the snapshot's whole copy of the
store (see the 40 GiB record of the same date); the mechanism is not confirmed.
