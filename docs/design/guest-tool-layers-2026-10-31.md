# Guest tool layers: heavy tools leave the base (2026-10-31)

Status 2026-10-31: design and build, branch `lane/tool-layers`. Owner,
2026-10-31: "start the tool layers work". Mission: minimize storage growth
while minimizing memory spikes
([lean system](../definitions/choir-lean-system-2026-10-30.md)). It follows
the [layered guest image](layered-guest-image-2026-10-31.md) §9, which named
LibreOffice, gcc and Go as the base's largest "is it needed in every guest"
paths. Mutation class of the build: red (vmctl launch drive set, snapshot
binding, guest boot mounts). Rollback: §8.

The layered image split the guest store into a base (3.34 GB, reused across
deploys) and an app layer (95 MB per deploy). The base still carries
everything, and it moves whenever `nix/autoputer-vm.nix`, `flake.lock`, the
kernel or the guest scripts move. Each move writes a new 3.34 GB base. This
design takes the heavy tools out of the base into **tool layers**: one
read-only erofs image per tool family, whose identity moves only when the
tool itself moves, shared by every VM through one host file, and attached at
boot according to the computer's profile.

## 1. What can go wrong (read first)

1. **A tool is invoked but its layer is absent.** The store path does not
   exist in the guest. Every caller must fail at once, with a message that
   names the layer, never hang:
   - office (`soffice`, `libreoffice`, …) and `go`, `gcc`, `cc`: the
     exec-log wrapper on PATH (§6) prints "the `<layer>` tool layer is not
     attached to this computer" and exits 127;
   - Chromium: `visualcapture.ReadGrant` already stats the binary and reports
     "the Chromium binary is missing from the guest image"; the browser layer
     follows the visual capture grant (§4), so a granted computer always has it;
   - Claude Code: `internal/capsule/claude_delegate.go` already stats
     `CHOIR_CLAUDE_CODE_BIN` and reports the delegate unavailable; the
     delegate layer follows the delegate allowlist;
   - anything else that names a tool store path by absolute path gets ENOENT
     at exec, which is immediate.
2. **The guest waits for a drive that never comes.** A guest whose initrd
   mounts `nix-tool-<name>` by label would wait for the device timeout and
   fail the boot if vmctl did not attach it. Guard: vmctl always attaches
   **every** tool slot a release names. An absent tool is an *empty
   placeholder* erofs with the same label, never a missing drive. The guest's
   mount set is fixed by its image; only the bytes behind a slot vary. The
   tool mounts carry `x-systemd.device-timeout=20s`, so even a vmctl bug
   fails the boot in seconds, not minutes.
3. **An old vmctl launches a release with tool slots.** It would attach base
   and app only, and the guest would wait for the tool labels. Guard: a
   release with tool layers writes `layers.json` with `"schema": 2`. The
   vmctl of the layered image accepts only schema 1, so it refuses the
   release (`release_missing`) and the computer cold boots the single image.
4. **A tool layer is deleted while a release or snapshot still names it.**
   The release fails to load (`releaseByID` checks every tool file, real and
   empty), so its snapshots cold boot and a ring computer falls back to the
   single image. Retention keeps a tool layer while any retained release, an
   in_use entry or a live `fc-config.json` names it (§7), never by its own age.
5. **A resumed VM sees different bytes behind a drive.** A Firecracker
   snapshot's memory holds the guest's erofs page cache and mount state for
   each slot. If the slot's file changed between hibernate and wake (real
   layer for empty, or another layer id), the guest reads garbage. Guards:
   - the tool disk paths are in the launch config, so the launch hash moves
     when the profile or the layer moves, and the wake refuses;
   - `snapshotBoundPaths` includes every tool disk, so a file replaced under
     the same path refuses the wake (disk identity);
   - a reattached VM (vmctl restart) takes its tool disks from its launch
     record, never from today's profile, so its next snapshot binds what it
     actually runs.
6. **The data drive moves.** The guest mounts the data volume by position
   (`/dev/vdb`). Tool drives are appended after base, which is after data
   and credential. The data drive stays second.
7. **A path needed by the core is moved into a tool layer.** A tool's
   closure shares libraries with the core (glibc, openssl, python). If a
   shared path went only into the tool layer, a computer without that layer
   would lose a core library. Guard: the base is
   `closure(full probe) − (∪ tool layers − closure(core probe))`, where the
   core probe is the same guest with every tool replaced by an empty stub.
   Every path the core needs stays in the base, even when a tool layer also
   carries it. The tool layer subtracts only a *floor* (a fixed list of core
   packages), and the base build fails if the floor is not inside the core
   closure.
8. **The guest references a different tool than the layer carries.** If the
   guest's `pkgs.libreoffice` and the layer's root differed (overlays,
   config), the layer would hold unused paths and the real tool would stay
   in the base. Guard: the roots come from one file
   (`nix/guest-tools.nix`) evaluated with the guest's own `pkgs`, and the
   base build fails if a root is not in the full probe's closure.
9. **Tool layers rebuild on every base move.** If a tool layer's derivation
   depended on the guest config, every `nix/autoputer-vm.nix` edit would
   rebuild 1.5 GB of office layer and materialize a new file. Guard: a tool
   layer is `closure(roots) − closure(floor)`, both from nixpkgs only, so
   its store path (its id) moves only with nixpkgs or the root list.
10. **Overlay depth and lower count.** Tool layers are extra *lowers* of the
    one store overlay, not stacked overlays, so the depth stays at 1 (2 with
    the updater's layering overlay, which also takes the tool lowers, §5).
    Overlayfs allows 500 lowers; this adds four.
11. **Duplicated bytes.** A path in two layers (office and browser share
    GTK, ffmpeg, flite: 208 paths, 633 MB NAR) or in a layer and the base is
    stored and cached once per layer file. That costs bytes once per layer
    id, never per deploy, and is correct: identical store paths have
    identical content, and overlay serves the upper one. A shared
    `desktop-libs` layer is a follow-up (§9).
12. **The guest Nix database lists absent paths.** The guest registers its
    whole closure (`closure-info`), so `nix-store --verify` would report a
    detached tool's paths as missing. Nothing in the guest verifies the
    store. A capsule `nix build` that needs a detached tool would fail at
    exec, not silently.
13. **The exec log carries user data.** Arguments can hold file names and
    content. The wrapper logs the tool name, its layer and whether the layer
    is present: never arguments, environment or working directory.
14. **The exec log floods the journal.** A cgo build calls `gcc` hundreds of
    times. The wrapper logs at most one line per tool per 60 seconds per boot
    (a stamp under `/run/choir-tool-exec`), and only when it could write the
    stamp: a mount namespace without a writable stamp directory logs nothing
    rather than a line per call.
15. **Host page cache.** Each layer file is one inode on the host, shared by
    every VM that attaches it (and by the jail's `.shared` copy, keyed by
    source inode). A layer that no VM reads costs no page cache. The empty
    placeholders are a few KB each.
16. **The deploy itself on Node B (added at the rebase onto main, epoch 24).**
    - **Who boots which layout.** Tool slots exist only in the layered image.
      The layered ring is `guestLayeredComputers` = d@e alone; every other
      computer, the paper included, launches the single image through the
      unchanged guest link (`/var/lib/go-choir/guest`), which still carries
      every tool, so no existing VM gets a new slot layout and the paper's
      drive set is unchanged. The Mac guest (aarch64) is built with
      `toolLayers = false`: no tool slots are mounted or attached, and its base
      is the full probe, as before.
    - **What does move for everyone.** `nix/autoputer-vm.nix` changes (exec-log
      wrappers ahead of the real tools on PATH, `/run/choir-tool-exec`), so the
      single image's toplevel, initrd and floor digest move, and
      `security_epoch` is 24 (main took 23 with tape group commit). Every
      release-bound snapshot therefore refuses at wake and cold boots, the
      same as any epoch raise; running VMs keep running their old release
      until their planned refresh (the paper takes the marker-backed refresh,
      never mid self-development apply or with a live capsule). Nothing
      here changes a drive position: data stays `/dev/vdb`.
    - **d@e.** It leaves its snapshot (epoch and layout), cold boots the new
      layered release, and gets `toolchain` (and `browser` only under a
      capture grant). If the new layered release fails to materialize or load
      (a missing tool file, an unreadable `layers.json`), `releaseByID`
      refuses it and d@e cold boots the single image, logged: the ring falls
      back, nothing hangs.
    - **Rollback.** (a) d@e alone: remove its id from `guestLayeredComputers`
      (or name the layer for it in `guestToolProfile`) and redeploy; its next
      cold boot is the single image. (b) The mechanism: revert the squash
      commit; the epoch must not fall (the gate refuses), so the revert
      lands as epoch 25 with the tool layers removed, which again cold boots
      every snapshot. The `guest-tools/` directory is swept by retention once
      no retained release names it.
    - **Staging proof right after the deploy (all read-only).**

      ```text
      curl -s https://choir.news/health          # commit == the pushed SHA
      ssh node-b readlink /var/lib/go-choir/guest /var/lib/go-choir/guest-layered
      ssh node-b 'cat /var/lib/go-choir/guest/release.json'   # security_epoch 24 (single image)
      ssh node-b 'ls /var/lib/go-choir/guest-tools/*/'        # tool.erofs per id (real + empty)
      ssh node-b 'cat /var/lib/go-choir/guest-layered/layers.json'  # "schema":2, four tools
      ssh node-b 'ls /var/lib/go-choir/guest-layered/'        # storedisk-base/app, no storedisk.erofs
      ssh node-b 'systemctl is-active go-choir-vmctl'
      ssh node-b 'journalctl -u go-choir-vmctl --since "-30min" --no-pager | grep -E "tool layers:|release_missing|falling back|refus"'
      ```

      Expected: a `tool layers: toolchain attached; browser,delegate,office
      empty` line for d@e (never for any other computer); no
      `release_missing` for the single-image computers; the paper's VM is
      running and healthy (`/health` of the paper computer through the
      gateway, then the paper's own latest publication still ticking). On d@e
      itself (it is the drill computer): `go version` and `which soffice`
      through its shell must answer (go present) and the "office tool layer is
      not attached" message (exit 127) respectively; the tool exec log shows
      as `journalctl -t choir-tool-exec` inside the guest. A d@e boot
      that shows the single-image release instead means the layered release
      was refused: read the vmctl line above it.

## 2. Inventory (measured 2026-10-31, cache.nixos.org NAR sizes)

Callers, from `git grep` over Go, scripts, prompts and Nix (docs excluded):

| Tool | Callers | Who needs it |
|---|---|---|
| LibreOffice (`soffice`) | **none**. Only `nix/autoputer-vm.nix` PATH and systemPackages, and a comment in `internal/capfloor/capfloor.go` | nobody, by code; the operator console's `!` shell could run it |
| pandoc | `internal/content/extract.go` (docx, epub), `internal/doctool` | every computer that ingests documents |
| poppler (`pdftotext`, `pdfinfo`) | `internal/content/extract.go`, `internal/content/pdf_blocks.go`, `internal/doctool` | every computer that ingests documents |
| Chromium | `CHOIR_CAPTURE_BROWSER_BIN`, read by `internal/autoputer/capsule_executor_linux.go` and `internal/visualcapture/grant.go` | computers granted visual capture (`CHOIR_VISUAL_CAPTURE_COMPUTERS`: two ids and `class:test`) |
| gcc, binutils, Go | engineering capsule builds (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`: `go build`, `go test`), `CHOIR_CAPSULE_BUILD_PATH` (`internal/yaegikernel/build_toolchain.go`) | every computer whose engineering desk runs a build |
| Claude Code | `CHOIR_CLAUDE_CODE_BIN`, `internal/capsule/claude_delegate.go` | the delegate allowlist (`CHOIR_CLAUDE_DELEGATE_COMPUTERS`: two ids) |

Closure sizes (NAR bytes, from `nix path-info --store https://cache.nixos.org
-r` on the guest's own package outputs; "beyond floor" subtracts the floor of
§3, which the core always carries):

| Layer | Roots | Closure | Beyond floor (leaves the base) |
|---|---|---|---|
| office | libreoffice 26.8.0.3 | 364 paths, 2,844 MB | 292 paths, **2,520 MB** (libreoffice itself 1,556; openjdk jre 95; flite 63; gtk3 44; ffmpeg 36) |
| browser | chromium 155 | 339 paths, 1,833 MB | 240 paths, **1,451 MB** (chromium-unwrapped 743) |
| toolchain | gcc 16.2, binutils 2.46, Go 1.26.10 | gcc+binutils 27 paths, 407 MB; Go ~230 MB (the flake's own `choirGo`, not on the cache; the layered design measured `go-1.26.10` at 0.23 GB) | gcc+binutils 13 paths, **350 MB** + Go ~230 MB |
| delegate | claude-code 2.1.300 | unfree, not on the cache; the layered design measured 0.27 GB | ~**270 MB** |
| (docs, stays in base) | pandoc 3.7, poppler-utils 26.09 | 67 paths, 384 MB | 33 paths, 289 MB |

The office and browser closures beyond the floor share 208 paths (633 MB).
The union of office, browser and toolchain beyond the floor is 337 paths,
3,687 MB NAR, plus Go and Claude Code: about **4.2 GB of the base's 6.1 GB
NAR** leaves the base, less the paths the core itself needs (failure mode 7
keeps those in the base). The erofs lz4 image is about 0.55× NAR on this
closure (3.34 GB image for 6.10 GB NAR), so the expected base image is
roughly 1.2 to 1.6 GB instead of 3.34 GB. The exact figure comes from the
first Node B build (`nix build .#guest-image-layered`, then read
`tool-layers` and the image sizes); this Mac cannot build x86_64 images.

Not measured here: the base and tool image bytes themselves, and the
overlap between tool closures and the real core closure (the core contains
custom paths not on the cache).

## 3. Layer format and ids

- **A tool layer** is an erofs image built exactly like the base
  (`mkfs.erofs -zlz4 -T 0 --all-root --mount-point=/nix/store`), label
  `nix-tool-<name>`, holding `closure(roots) − closure(floor)`.
- **The floor** is a fixed list of nixpkgs packages the core always has
  (glibc, bash, coreutils, systemd, util-linux, curl, git, nodejs, perl,
  python3, openssl, zlib). It only trims the layer; the base build checks
  `closure(floor) ⊆ closure(core probe)`.
- **The empty placeholder** for each slot is an erofs with the same label
  and no store paths.
- **Id** = the first 32 characters of the image's store path (its Nix hash),
  as for bases. The derivation depends only on nixpkgs, the root list and
  the floor list, so a deploy that changes no tool yields the same id, the
  Nix store already holds the image, and the install finds the id present
  and copies nothing ("dedupe by content id").
- **Names** (`^[a-z][a-z0-9]{0,15}$`) and roots live in `nix/guest-tools.nix`:
  `browser`, `delegate`, `office`, `toolchain`, in that (sorted) order. The
  order is the drive order and the overlay lower order.
- **Image**: `guest-image-layered` gains `tools/<name>.erofs` and
  `tools/<name>-empty.erofs` (symlinks into their store paths), a
  line-oriented `tool-layers` file (`<name> <layer store path> <empty store
  path>`) for the install script, and `tools` in its `layers.json`.
- **Host**: `/var/lib/go-choir/guest-tools/<id>/tool.erofs` (0444, root, never
  rewritten), the same atomic `.tmp` and `mv` as bases.
  `VM_GUEST_TOOLS_DIR` overrides the directory.
- **Release** `layers.json`, schema 2:
  `{"schema":2,"layout":"layered","base_id","base_store_path","tools":[{"name","id","empty_id"}]}`.

## 4. The profile

The profile is chosen by vmctl at each launch, from one host selector, for
computers in the layered ring only (a single-image computer carries every
tool in its one image, as today):

```text
VM_GUEST_TOOL_PROFILE  layer=selector;layer=selector;...
```

A selector is the standard computer selector (ComputerIDs, VM ids,
`class:<warmness class>`, `*`), plus `none`, `grant:visual-capture` (the
computer holds the visual capture grant vmctl writes onto its credential
disk) and `grant:claude-delegate` (the delegate allowlist names it). A layer
the variable does not name is attached for every computer: an unconfigured
or new layer never takes a tool away.

Shipped in `nix/node-b.nix`:

```text
toolchain=*;office=none;browser=grant:visual-capture;delegate=grant:claude-delegate
```

- **d@e** (`computer-59c902c5…`, the layered ring R0) gets `toolchain`. It is
  not on the visual capture or delegate lists by id, so it gets `browser`
  only if vmctl boots it under warmness class `test` (`class:test` is on the
  capture list); the repo cannot tell which class it boots under, and the
  launch log line names the profile. It never gets `office`.
- "On, never off everywhere": the mechanism is on for the whole ring (d@e)
  as shipped; widening the ring widens the tool layers with it. Office is
  attached to nobody because nothing calls it; the exec log (§6) measures
  whether anything does over a week on the single-image fleet, where every
  tool is still present.
- The profile is read at a cold boot. A change reaches a hibernated
  computer at its next cold boot: the wake refuses because the launch hash
  moves (failure mode 5).
- vmctl logs the profile at every layered launch:
  `vmmanager: computer <id> (VM <vm>) tool layers: toolchain attached; browser,delegate,office empty`.

## 5. How the layers compose with base, app and the data drive

- **Drive order** (Firecracker): `store` (app), `data` (/dev/vdb), `credential`,
  `base`, then `tool-<name>` for every slot in name order, each read-only.
  The data drive does not move.
- **Guest mounts** (`nix/guest-layered-store.nix`): `/nix/.ro-tool-<name>`
  (label `nix-tool-<name>`, erofs, needed for boot, 20 s device timeout) for
  each name, and the store overlay
  `lowerdir=/nix/.ro-app:/nix/.ro-base:/nix/.ro-tool-browser:…:/nix/.ro-tool-toolchain`.
- **The updater's layering overlay** takes the same lowers (`/proc/mounts`
  order) with its private store as the upper, still one overlay.
- `debug_pagecache` reports `layers: ["app","base","tool-browser",…]`.
- **App layer** is unchanged: `closure(real) − closure(full probe)`. Tool
  paths are in the full probe, so they never land in app.

## 6. The exec log

Every guest (single image and layered) gets thin wrappers on PATH ahead of
the real binaries for the office binaries and `go`, `gcc`, `cc`, `g++`,
`c++`. A wrapper:

- writes `choir-tool-exec: tool=<name> layer=<layer> present=<0|1>` to the
  journal through `logger -t choir-tool-exec`, at most once per tool per 60 s
  per boot and only when it can write its stamp, never arguments, never
  failing the tool if logging fails;
- if the real binary is absent, prints the layer message and exits 127;
- otherwise `exec`s the real binary with the original arguments.

To measure use over a week, on Node B for each computer:
`journalctl -t choir-tool-exec` inside the guest (the guest debug endpoint
or the console). Under-counted: invocations inside capsules whose mount
namespace has no `/dev/log` or no writable `/run/choir-tool-exec` (nothing is
logged there), and callers
that use an absolute store path (Chromium, Claude Code), whose existing
grant logs cover them.

## 7. Install, retention, hibernate

- **Install** (`scripts/node-b-guest-release-materialize`): for each line of
  the image's `tool-layers`, copy the real and the empty image into
  `guest-tools/<id>/tool.erofs` unless the id is present (reflink, 0444, an
  existing id is kept), then write the schema 2 `layers.json`. Any failure
  leaves no release (its snapshots cold boot; the ring falls back to single).
- **Retention** (`scripts/node-b-guest-release-retention`): a tool layer stays
  while a remaining release's `layers.json` names it (real or empty id), the
  in_use answer names it (`tool_ids`), a `fc-config.json` holds
  `guest-tools/<id>`, or it is under two hours old. An unreadable
  `layers.json` of a remaining release deletes no tool layer.
- **Hibernate**: the drive set comes from the release (every slot) and the
  choice from the launch config, which the snapshot meta carries. The wake
  rebuilds the expected config against the snapshot's own release and
  today's profile; a different profile moves the hash and the wake refuses.
  `snapshotBoundPaths` binds every tool disk.

## 8. Rollback

- **One computer**: remove it from `guestLayeredComputers` in
  `nix/node-b.nix` (it cold boots the single image, which has every tool),
  or add a layer to it in `VM_GUEST_TOOL_PROFILE` (its next cold boot
  attaches the layer).
- **The mechanism**: revert the branch. The layered image goes back to
  schema 1 (base carries every tool); retained schema 2 releases stay
  loadable only by the new vmctl, so after a vmctl revert they refuse
  (`release_missing`) and their snapshots cold boot. That is the designed
  refusal, not data loss.
- **Bytes**: the tool directories are swept by retention once no retained
  release names them.

## 9. Later options (not this build)

- **On-demand attach** through Firecracker's drive `PATCH`: boot every slot
  empty and swap the backing file of a slot when a tool is first invoked.
  Firecracker can update a drive's `path_on_host` while the VM runs, but the
  guest has the empty erofs mounted; it would need an unmount and remount
  inside the overlay's lower, which overlayfs does not allow under a live
  mount. It needs a per-tool overlay or a lazy mount point outside the
  store overlay, and hibernate would then bind a drive set that changes
  after boot.
- **Fetching tools from a host binary cache** into the data volume (a
  substituter on the host): costs per-computer bytes and a network path into
  the guest's store, and gives up the shared page cache. Worth it only for
  tools used by one computer.
- **A shared `desktop-libs` layer** for the 633 MB office and browser share,
  attached whenever either is.
- **docs layer** (pandoc + poppler, 289 MB beyond the floor) if the exec log
  shows computers that never extract documents.
- **Delete LibreOffice** from the guest entirely if the week's exec log shows
  no use: the office layer is then removed and the single image shrinks by
  about 1.4 GB as well.
