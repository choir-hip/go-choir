# Staging Proxy ICU Runtime Link Problem

Date: 2026-06-05

## Problem

The staging deploy for commit `74b5a5e5f8e309974fda7bf08f31f9c015acde26`
passed build and test gates, installed the selected host services on Node B,
and hot-refreshed active sandbox runtimes, but failed the final public health
probe because the proxy service could not start.

This is a host service packaging problem. The deployed proxy binary links
against ICU through the Go/Dolt dependency chain, but the systemd service
runtime environment does not expose the ICU shared libraries needed by that
binary.

## Evidence

- GitHub Actions run `26994140537` passed Go tests, runtime shards, frontend
  build, vet, and build, then failed `Deploy to Staging (Node B)`.
- Node B health after deploy:
  - `127.0.0.1:8085/health` returned sandbox commit
    `74b5a5e5f8e309974fda7bf08f31f9c015acde26`.
  - `127.0.0.1:8082/health` failed to connect.
- `go-choir-proxy.service` restarted repeatedly with status `127`.
- Proxy journal reported:
  `error while loading shared libraries: libicui18n.so.76: cannot open shared object file`.
- Running `/var/lib/go-choir/services/proxy/bin/proxy` directly on Node B
  produced the same missing `libicui18n.so.76` error.

## Required Fix

The proxy service package or service wrapper must include the same ICU runtime
library path guarantee as other Go/Dolt-linked services.

The fix should be tracked in the repo and deployed through the normal main
branch CI path. Do not patch Node B environment variables by hand as the
durable solution.
