# One compromise opens too much (2026-10-20)

**Status:** documented; design in panel review. Mutation class of the fixes:
red, with two black items. Owner ruling: owner notes §31 ("we should do
number one as a priority").

## The problem

The owner asked that a break-in be less catastrophic before Autoradio
launches with signed-in users, each of whom gets a computer. A code reading of
main at e2e124213 (an inventory of keys, secrets, service privileges, backups
and the deploy path) finds that, today, several single compromises each yield
far more than the unit compromised:

- a stolen disk or a leaked backup yields user data, because the keys that
  protect user data are stored beside it;
- one compromised host service yields routes and data belonging to other
  services, because services share privileges and credentials;
- a leaked deploy credential yields the host;
- the escrow that lets the host boot and rebuild computers is protected by
  ordinary file permissions rather than sealed key storage.

Live root on the host remains total in any design that uses Firecracker
without confidential-computing hardware; that limit is stated, not hidden.

## Why the detail is not here

This repository is public, and the inventory maps live attack paths. The full
inventory and the design are held privately until the items they name are
remediated, then published here with the receipts. This is a deliberate
exception to problem-documentation-first: the problem is documented here; the
exploitable specifics wait.

## The plan, in outline

1. Turn on hardening that is already built (owner-act enforcement, the
   database lock, the Firecracker jailer, desk-cell confinement and guest
   egress), one item at a time, each proven on staging.
2. Remove whole paths: narrow what each service can see and call, restrict
   the deploy credential, make sessions revocable.
3. Move keys off the disks they protect: encrypt each computer's volume with
   a key delivered at boot, seal the escrow key, split backups by class so no
   single backup secret opens everything.
4. After launch: an opt-in user-held vault, per-service users, and
   confidential VMs where the hardware allows.

Each item carries its own rollback and staging proof in the private design.
