# The podcast worker cannot fetch from the build VM: its only way out is a proxy

2026-11-16, 06:55Z.

## Problem

The podcast worker was designed to run as a service on the build VM (the
guest on Node A). Started there at 06:40Z it recorded nothing: every feed
fetch failed with `lookup <host> on 127.0.0.53:53: i/o timeout`.

## Evidence

- The VM's environment sets `HTTPS_PROXY`/`HTTP_PROXY` to a proxy on the
  host's side of its network (`10.77.200.1:3128`). `curl` works only through
  it (`time_namelookup` near zero; with `--noproxy '*'` it times out).
- Port 53 does not answer anywhere from the VM: `dig` to the local stub and
  to public resolvers times out over UDP and TCP.
- The worker's guarded client resolves a name itself, checks the address is
  public, and dials that address. With no DNS and no direct route it cannot
  work there. Using the proxy as it stands would hand name resolution to the
  proxy and void the rule "the checked address is the dialled address".
- The operator proof of the 15th fetched audio with tools that follow the
  proxy, so the constraint was never met by the worker before deployment.
  The worker's tests use local test servers.

## Belief

The worker is correct for a host with direct egress and wrong for this one.
A design fact (how the build VM reaches the internet) was not read before
the service was designed for it.

## What was done tonight

The worker runs on the lead's Mac (direct egress): the transcriber built
from the same source revision, the same model files, the worker
cross-built from the landed tree. The operator key stays in a loopback
proxy on the Mac that forwards only the podcast routes; no key was placed
on the shared VM. First intake at 06:50Z recorded 183 episodes of 61 feeds
and transcription runs at about 45 times real time.

## Remaining error

This is not the durable service the owner asked for: it stops when the
lead's session ends. A durable home needs two owner decisions: where the
worker runs (a host with direct egress, or the VM through its proxy with
the guard redesigned: resolve over the proxy, then `CONNECT` to the checked
address), and where an operator key (or a narrower worker key) may live.
