# The paper's computer filled its 32 GB disk, wedged, failed to reboot during a deploy, and would have stayed down for 24 h

Date: 2026-10-15 (~00:25 UTC). Class: red (VM lifecycle, the always-on
paper). Owner rule: "we need paper publishing 24/7, even after crashes."

## Timeline (vmctl journal, guest console, deploy log; counts and states only)

- **21:53**: deploy 2d9e2b00c's health probe of vm-1669020a…
  (computer-1355b83b…, the always-on computer that runs Autopaper; it belongs to a dedicated Autopaper account, not the owner's, per reports/reeng-autopaper-paper-computer-2026-10-13.md; wording corrected 2026-10-21)
  reports `persistent_disk.used_bytes` ≈ 33.4 GB of a 33.5 GB cap. The
  disk is nearly full hours before any failure.
- **23:22**: vmmanager health checks start failing every 15 s.
- **23:29:31**: vmctl's wedge detector stops the computer ("refused
  connections for 5m0s"). The warmness policy spends the always-on
  computer's one automatic retry; the next is allowed after 24 h.
- **~23:30**: a boot of the same computer starts while deploy 299cdb645 is
  restarting vmctl and replacing the guest image. The guest initrd fails
  `Find NixOS closure` and drops to emergency mode. vmctl (restarted
  23:30:47) logs "protected VM … runs outside the registry; reattach
  pending" three times.
- **23:36:55**: vmctl kills it as an orphan (`cause=reap-unmanaged:orphan`).
  The computer is stopped, with no automatic retry for 24 h.
- **00:06**: the lead resumes it through `/internal/vmctl/resolve`, the
  path a visit uses. It boots, and the runtime logs
  `store: dolt gc emergency: avail=0 MiB; running despite store size`.
  `data.img` is 32 GiB, fully allocated. The resolve call timed out at
  300 s while the guest ran GC at ~300% CPU and 8.3 GB RSS.
- The public front page still served its last 20 stories throughout
  (served by the platform, not this computer), but nothing new was
  published.

## Problems (separate, each needs its own fix)

1. **Unbounded disk growth on the paper's computer.** It reached the
   32 GB cap. The health signal (`warning` and `critical` disk flags)
   existed, but nothing acted on it. Hypothesis: Dolt history and
   artifacts grow with 24/7 publishing faster than GC reclaims; not yet
   measured by component.
   **Measured 2026-10-15:** 27.2 GB of the 33.5 GB was Dolt chunk-journal
   garbage. The periodic GC never ticked (its ctx was cancelled on return),
   and the 5 GiB guard skipped every boot GC from 10-14 13:43. See
   [`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md).
2. **The wedge detector plus warmness policy left the always-on paper
   down for up to 24 h.** That contradicts the owner's 24/7 rule. A
   standing intent should survive a crash; the computer itself has to
   come back for that to happen.
3. **A boot during a deploy's vmctl restart and guest-image swap failed
   in initrd** (`Find NixOS closure`), and the restarted vmctl then
   orphan-killed it. Hypothesis: a race with the image swap. Other
   computers booted cleanly at 23:30:59 and 23:31:28, after the swap.
4. **Resume blocks on a long startup GC:** health is not ready until the
   emergency GC finishes, which outlasts callers' timeouts.

## Belief and next steps

- Recover first: let the GC finish and confirm the paper publishes.
- Then measure disk use by component inside the guest, through the
  computer's own API (no block-device access while it runs).
- Fixes to weigh:
  - disk-pressure action before full: GC thresholds, retention, or
    growing the disk while the computer is stopped;
  - an always-on retry policy that never leaves the paper down for 24 h;
  - deploys never restarting vmctl while a protected computer is mid-boot,
    or vmctl adopting such a boot instead of killing it.
