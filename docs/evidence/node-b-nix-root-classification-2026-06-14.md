# Node B Nix Root Classification Report

- mode: `report-only; no root deletion, generation pruning, nix gc, service restart, or guest image deletion`
- captured_at: `2026-06-14T20:15:21Z`
- hostname: `choiros-b`
- evidence_dir: `/tmp/node-b-nix-root-report-final`
- root_available: `122.16 GiB`
- root_used: `350.96 GiB`
- current_system: `/nix/store/n1m3bfv8cgglvwy7i0spd2ifxv64iiy7-nixos-system-go-choir-b-26.05.20260409.4c1018d`
- booted_system: `/nix/store/jmlg9chm4l2s8wn4synl464bvq16l9yw-nixos-system-choiros-b-26.05.20260306.aca4d95`
- dead_store_paths: `9423`
- nix min-free/max-free: `0` / `9223372036854775807`
- auto-optimise-store: `true`

## Root Classes

| class | roots | direct target allocation | policy |
| --- | ---: | ---: | --- |
| `running_process_root` | 451 | 5.56 GiB | keep: process runtime root disappears when process exits/restarts |
| `retained_system_generation` | 8 | 640.00 KiB | keep newest 3-4 after rollback rationale; prune older generations through declared service only |
| `temporary_service_build_root` | 4 | 35.44 MiB | formalize TTL/owner or delete after latest deploy/rollback manifest |
| `temporary_guest_image_build_root` | 2 | 5.78 GiB | formalize TTL/owner or delete after latest deploy/rollback manifest |
| `booted_system_root` | 1 | 80.00 KiB | keep until reboot catches current; mismatch is rollback evidence |
| `current_system_root` | 1 | 80.00 KiB | keep: current system must never be a prune candidate |
| `invalid_root` | 1 | 0 B | formalize or delete: invalid old result root after deploy identity proof |
| `root_user_nix_cache_root` | 1 | 8.00 KiB | keep or ignore: small root-owned Nix cache root, not deploy rollback state |
| `temporary_host_system_build_root` | 1 | 80.00 KiB | formalize TTL/owner or delete after latest deploy/rollback manifest |
| `temporary_playwright_guest_image_build_root` | 1 | 3.53 GiB | formalize TTL/owner or delete after latest deploy/rollback manifest |

## System Generations

| generation | date | current | policy |
| ---: | --- | --- | --- |
| 489 | `2026-06-13 07:45:44` | `false` | prune after rollback review |
| 490 | `2026-06-13 07:58:04` | `false` | prune after rollback review |
| 491 | `2026-06-13 08:02:59` | `false` | prune after rollback review |
| 492 | `2026-06-13 08:30:22` | `false` | prune after rollback review |
| 493 | `2026-06-14 05:34:14` | `false` | keep as recent rollback candidate |
| 494 | `2026-06-14 16:07:05` | `false` | keep as recent rollback candidate |
| 495 | `2026-06-14 16:41:09` | `false` | keep as recent rollback candidate |
| 496 | `2026-06-14 17:24:30` | `true` | keep current |

## Invalid Roots

| root | target | policy |
| --- | --- | --- |
| `/opt/go-choir/result` | `/nix/store/srzb724qbl2s871jjkjja61zdfqwzv2j-prefetch-npm-deps-0.1.0` | formalize or delete: invalid old result root after deploy identity proof |


## Temporary Result Roots

| root record |
| --- |
| `/tmp/go-choir-service-corpusd-result	/nix/store/0wbic0glg8dspxb5ravpxlkgil0kaddb-corpusd-0.1.0	2026-06-10T07:44:31.2805019980Z` |
| `/tmp/go-choir-service-sourcecycled-result		2026-06-10T18:58:17.8061642640Z` |
| `/tmp/go-choir-guest-image-new	/nix/store/1w0ym05w71cg9w94hp882ch2ywpjcjl8-go-choir-guest-image	2026-06-10T20:05:12.0335341820Z` |
| `/tmp/guest-image-result	/nix/store/6d9f24qi8ij02ikfy2kavsrdk1qbh0d0-go-choir-guest-image	2026-06-14T16:06:54.7957782520Z` |
| `/tmp/guest-image-playwright-result	/nix/store/5qvkvn63qjjz2yn2y5spdz5vv124r944-go-choir-guest-image-playwright	2026-06-14T16:07:05.4669103640Z` |
| `/tmp/go-choir-nixos-result	/nix/store/n1m3bfv8cgglvwy7i0spd2ifxv64iiy7-nixos-system-go-choir-b-26.05.20260409.4c1018d	2026-06-14T17:24:30.8211480250Z` |
| `/tmp/go-choir-service-gateway-result		2026-06-14T19:33:56.0362363210Z` |
| `/tmp/go-choir-service-sandbox-result		2026-06-14T19:34:08.7553953560Z` |
| `/tmp/go-choir-service-vmctl-result	/nix/store/lpnz0j20gsymnx4k0psg92ar18pj3nwy-vmctl-0.1.0	2026-06-14T19:34:19.0055235130Z` |
| `/tmp/go-choir-service-proxy-result	/nix/store/cmh5bm3ilk5n7qsg6hgswmr28vm48yf7-proxy-0.1.0	2026-06-14T19:34:19.4625292270Z` |
| `/tmp/go-choir-frontend-result	/nix/store/pyf9pifdzy1qd6k9x0ysmrv71ny3s2ha-go-choir-frontend-0.1.0	2026-06-14T19:39:19.4252775950Z` |

## Dead Path Buckets

| bucket | paths |
| --- | ---: |
| `other` | 3450 |
| `derivation` | 2528 |
| `go-choir-service` | 1638 |
| `go-choir-frontend` | 745 |
| `nixos-system` | 449 |
| `runtime-deps` | 205 |
| `closure-info` | 204 |
| `go-choir-guest-image-playwright` | 103 |
| `go-choir-guest-image` | 101 |

## Proposed Keep/Delete/Formalize Policy

- keep: current system, booted system until reboot convergence is understood, four newest system generations, active service pointers, active guest image files, and running-process roots.
- formalize: `/tmp/go-choir-*result`, `/tmp/guest-image-*`, and `/opt/go-choir/result` roots must have type, owner, TTL, and rollback purpose if they survive a deploy.
- delete after explicit approval and identity proof: invalid `/opt/go-choir/result`, temporary service/frontend roots from old deploys, and guest-image build result roots superseded by installed guest paths and rollback manifest.
- configure: Nix daemon `min-free=120GiB`, `max-free=180GiB`; routine scheduled GC before low-space emergencies; weekly off-peak optimise or keep auto-optimise only if IO evidence stays acceptable.
- refuse: no manual GC, generation pruning, or root deletion from this report alone.
