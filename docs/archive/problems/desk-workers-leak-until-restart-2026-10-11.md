# Desk workers leak until the computer restarts

Found 2026-10-11 while reading the desk worker lifecycle for PD2
(definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md). Evidence
class: code reading at 701b0273; not yet measured on a live computer.

## What happens

`desk_go_eval` (internal/agentcore/tools_desk.go) spawns a host session
worker per run (a re-exec of the autoputer binary, 8 GiB address-space cap)
and keeps it in a pool keyed by run ID. The pool releases a worker in only
two places: after a cell fails and the worker is already dead, and when a
`choir.Research` child returns (research_children.go). A run that ends
normally leaves its worker process alive and idle in the pool. Nothing
reclaims it until the computer restarts.

Every Texture turn, every research run and every management wake adds one
idle process. A research-only demo with a follow-up makes dozens of desk
runs; a computer that stays on for days makes thousands.

## Why it matters

The persistent computer tier exists so computers stay on. A per-run leak
turns uptime into memory pressure, and the failure would show up far from
its cause: as slow cells, OOM kills or a crash restart that ends work.

## Belief and test

I believe the idle workers' resident memory is what fills a long-lived
guest. The test is the process count of desk-session workers on a computer
after a run of N desk runs, before and after the fix.

## Fix direction

PD2 keys desk sessions by agent (one per persistent desk) and closes
sessions idle past a retention window, and the least recently used idle
sessions beyond a bound. That reclaims workers for every key, including
per-run ones.
