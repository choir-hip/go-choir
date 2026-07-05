# Tape-to-Autoputer Materializer Design Proposal v1

## Vision

The autoputer is a persistent, owner-identified computer. Agents live in the
autoputer. Risky effects run in Nucleus capsules inside the autoputer. The
autoputer is a VM; capsules are containers inside that VM.

The tape (journal + blobs) is the artifact program — the tamper-evident
transaction history that computes the computer's durable state. Today the
autoputer's `data.img` is "durable_legacy_opaque": a 32GB mutable ext4 disk
whose contents are not provably derived from the tape. This is the fundamental
gap: **the autoputer's base state is not auditable.**

The fix: materialize the tape as an **immutable EROFS base image**, sealed with
dm-verity, with an overlayfs upper layer for writes. The kernel enforces
integrity at read time. Reset to tape state = drop the overlay. Capsules inside
the VM share the read-only base and get their own write layers.

## Architecture

```
                    tape (journal + blobs)
                           │
                           ▼
              StateGenerator.Generate(stagingDir)
                           │
                           ▼
              ErofsImageBuilder.Build(stagingDir, base.erofs)
                           │
                           ▼
              veritysetup format → root hash (sealed)
                           │
                    ┌──────┴──────┐
                    │             │
                    ▼             ▼
            base.erofs      verity hash tree
            (immutable)     (Merkle tree over 4k blocks)
                    │
                    ▼
         ┌─────────────────────────────────┐
         │       autoputer VM (Firecracker) │
         │                                 │
         │  /base  ← EROFS (ro, dm-verity) │
         │  /upper ← overlayfs (rw, tmpfs) │
         │  /      ← overlay mount         │
         │                                 │
         │  ┌───────────────────────────┐  │
         │  │   Nucleus capsule          │  │
         │  │   /base ← bind-ro /base   │  │
         │  │   /upper ← own overlay    │  │
         │  │   /  ← overlay mount       │  │
         │  └───────────────────────────┘  │
         └─────────────────────────────────┘
```

### Why EROFS + overlay + dm-verity

| Property | ext4 (mutable) | EROFS + overlay + verity |
|---|---|---|
| Base state audit | extract + compare (one-time) | dm-verity enforces at read time (continuous) |
| Reset to tape | re-materialize entire disk | drop overlay layer |
| Determinism | pinned mke2fs flags | native (EROFS is reproducible by design) |
| Pure Go builder | go-ext4fs (immature, 2 stars) | go-erofs (official EROFS project, 24 stars) |
| Codebase alignment | matches current data.img | matches current store disk (already EROFS) |
| Capsule sharing | copy-on-write per capsule | shared read-only base + per-capsule overlay |
| Image size | 32GB sparse | sized to content (EROFS is compact) |
| Compression | no | LZ4/zstd built in |
| Integrity tampering | undetected until next audit | kernel rejects bad blocks immediately |

EROFS is already the codebase's substrate for the Nix store disk. Using it for
the autoputer base unifies the architecture: both read-only layers are EROFS,
both are shared across VMs, both are Nix-built or tape-built.

### The three-layer model

```
host substrate
  → autoputer VM (EROFS base + overlay upper)
    → Nucleus capsule (bind-ro base + own overlay)
```

Each layer:
- **Host**: manages VM lifecycle, attaches EROFS base + overlay backing
- **Autoputer VM**: boots from overlay mount, runs durable agents, hosts capsules
- **Capsule**: ephemeral, effect-fenced, shares read-only base, own write layer

This matches the handoff doc's principle: *"Capsules inside a candidate VM may
share read-only base state. Each capsule gets its own write layer unless a
mutation transaction explicitly grants shared mutation."*

## Components

### Component 1: ErofsImageBuilder

```go
// ErofsImageBuilder packages a filesystem directory into an immutable EROFS
// disk image using the pure-Go go-erofs library.
type ErofsImageBuilder struct {
    // Compress enables LZ4 compression (default: true for base images)
    Compress bool
    // Uid/Gid override for all files (default: 0:0 for system images)
    RootOwner bool
}

// Build creates an EROFS image at imagePath containing the filesystem
// from stagingDir. The image is reproducible: same input → identical bytes.
func (b ErofsImageBuilder) Build(ctx context.Context, stagingDir, imagePath string) error
```

Implementation: use `github.com/erofs/go-erofs` (pure Go, official EROFS project).
The staging directory from StateGenerator satisfies `fs.FS` via `os.DirFS`.

```go
func (b ErofsImageBuilder) Build(ctx context.Context, stagingDir, imagePath string) error {
    outFile, err := os.Create(imagePath)
    if err != nil { return err }
    defer outFile.Close()

    w := erofs.Create(outFile)
    defer w.Close()

    // Walk stagingDir and create entries in the EROFS image
    return filepath.WalkDir(stagingDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        rel, _ := filepath.Rel(stagingDir, path)
        if rel == "." { return nil }

        info, _ := d.Info()
        mode := info.Mode()

        switch {
        case mode.IsDir():
            _, err := w.Mkdir("/"+rel, mode.Perm())
            return err
        case mode.IsRegular():
            f, err := w.Create("/" + rel)
            if err != nil { return err }
            data, err := os.ReadFile(path)
            if err != nil { return err }
            _, err = f.Write(data)
            f.Close()
            return err
        case mode&fs.ModeSymlink != 0:
            target, _ := os.Readlink(path)
            return w.Symlink(target, "/"+rel)
        default:
            return fmt.Errorf("unsupported file type: %s", mode)
        }
    })
}
```

**Why go-erofs over mkfs.erofs:**
- Pure Go, no external deps, cross-platform (builds on macOS)
- Official EROFS project repo (not a third-party reimplementation)
- Reproducible by design
- Read support too — can be used for audit without mounting
- Already aligns with codebase's EROFS store disk

### Component 2: VeritySealer

```go
// VeritySealer seals an EROFS image with a dm-verity hash tree.
// The root hash is the cryptographic commitment to the image's content.
type VeritySealer struct {
    BlockSize  int    // default 4096
    HashAlgo   string // default "sha256"
    Salt       []byte // optional; default none
}

// Seal computes the Merkle hash tree for the image and returns the root hash.
// The hash tree is appended to the image (or written to a sidecar file).
type VerityResult struct {
    RootHash   string // hex-encoded SHA-256 root hash
    HashOffset int64  // where hash tree starts in the image
    ImagePath  string // path to the sealed image
}

func (s VeritySealer) Seal(ctx context.Context, imagePath string) (VerityResult, error)
```

Implementation: shell out to `veritysetup format` (from cryptsetup, widely
available). A pure-Go dm-verity implementation is possible but not needed for
v0 — veritysetup is standard on Linux and the sealing happens at build time,
not at runtime.

```bash
veritysetup format <image> <hashfile> --hash sha256 --data-block-size 4096
```

The root hash is the **tape-faithfulness commitment**: if the image matches the
tape, the root hash matches the tape-predicted hash. The kernel enforces this
at every read via dm-verity.

### Component 3: AutoputerMaterializer

```go
// AutoputerMaterializer materializes an autoputer base image from the tape.
// It is a Materializer that produces an EROFS base image + verity seal.
type AutoputerMaterializer struct {
    Generator  StateGenerator
    Builder    ErofsImageBuilder
    Sealer     VeritySealer
    Extractor  FirecrackerStateExtractor  // for predicted observations
}

func (m AutoputerMaterializer) Materialize(
    ctx context.Context,
    version ComputerVersion,
    manifest CapabilityManifest,
) (Realization, error) {
    // 1. Generate staging directory from tape
    stagingDir, err := os.MkdirTemp("", "autoputer-stage-*")
    if err != nil { return Realization{}, err }
    defer os.RemoveAll(stagingDir)

    if err := m.Generator.Generate(ctx, version, stagingDir); err != nil {
        return Realization{}, fmt.Errorf("materialize: generate: %w", err)
    }

    // 2. Build EROFS image from staging directory
    imagePath := stagingDir + ".erofs"
    if err := m.Builder.Build(ctx, stagingDir, imagePath); err != nil {
        return Realization{}, fmt.Errorf("materialize: build erofs: %w", err)
    }

    // 3. Seal with dm-verity
    verity, err := m.Sealer.Seal(ctx, imagePath)
    if err != nil {
        return Realization{}, fmt.Errorf("materialize: seal verity: %w", err)
    }

    // 4. Compute predicted observations directly from the verified tree
    //    (NOT from the staging directory — use the tree as source of truth)
    obsSet := m.predictedObservations(version)

    // 5. Return realization
    return Realization{
        Version:      version,
        Observations: obsSet,
        Capabilities: manifest,
        Artifacts: AutoputerArtifacts{
            BaseImage:    imagePath,
            VerityRoot:   verity.RootHash,
            HashOffset:   verity.HashOffset,
            ImageSize:    fileSize(imagePath),
        },
    }, nil
}
```

### Component 4: AutoputerAuditor

```go
// AutoputerAuditor proves an EROFS base image is tape-faithful.
// It does NOT need to mount the image — it reads the EROFS directly
// using go-erofs and compares to the tape-predicted observations.
type AutoputerAuditor struct {
    Checker EquivalenceChecker
}

func (a AutoputerAuditor) Audit(
    ctx context.Context,
    imagePath string,
    predicted ObservationSet,
) (EquivalenceResult, error) {
    // 1. Read the EROFS image using go-erofs (no mount needed!)
    img, err := os.Open(imagePath)
    if err != nil { return EquivalenceResult{}, err }
    defer img.Close()

    fs, err := erofs.Open(img)  // go-erofs read support
    if err != nil { return EquivalenceResult{}, err }

    // 2. Walk the EROFS filesystem and extract observations
    actual := a.extractFromErofs(fs, predicted.Version)

    // 3. Compare predicted vs actual
    return a.Checker.CheckObservationSets(predicted, actual), nil
}
```

**Key advantage over ext4 approach:** go-erofs can **read** EROFS images
without mounting. No root, no loop device, no FUSE. The audit is pure Go,
cross-platform, and runs in CI on macOS. This is impossible with ext4 (which
requires mounting or debugfs for content verification).

### Component 5: OverlayProvisioner (VM launch side)

```go
// OverlayProvisioner sets up the overlayfs mount for an autoputer VM.
// Called by vmmanager when launching a VM with a tape-materialized base.
type OverlayProvisioner struct{}

// Prepare creates the overlay upper/work directories and returns the
// mount configuration for the VM's init system.
type OverlayConfig struct {
    LowerDir string // the EROFS mount point (read-only)
    UpperDir string // per-VM writable layer
    WorkDir  string // overlayfs work directory
    Merged   string // the overlay mount point (root or /data)
}

func (p OverlayProvisioner) Prepare(vmDataDir, baseImagePath string) (OverlayConfig, error) {
    cfg := OverlayConfig{
        LowerDir: "/base",  // EROFS mounted here by kernel
        UpperDir: filepath.Join(vmDataDir, "upper"),
        WorkDir:  filepath.Join(vmDataDir, "work"),
        Merged:   "/",
    }
    os.MkdirAll(cfg.UpperDir, 0755)
    os.MkdirAll(cfg.WorkDir, 0755)
    return cfg, nil
}
```

The VM's init system mounts:
1. EROFS base at `/base` (read-only, dm-verity enforced)
2. overlayfs at `/` with lowerdir=/base, upperdir=/data/upper, workdir=/data/work
3. Capsules bind-mount `/base` read-only and get their own upper layers

## The audit chain

```
1. Tape → StateGenerator → staging dir (predicted filesystem)
2. Tape → basetree observations → predicted manifest (from tree, not staging)
3. Staging dir → ErofsImageBuilder → base.erofs (immutable image)
4. base.erofs → veritysetup → root hash (cryptographic commitment)
5. base.erofs → go-erofs read → actual manifest (no mount needed)
6. predicted manifest ≡ actual manifest? (EquivalenceChecker)
7. predicted root hash ≡ actual root hash? (verifies the seal)
```

If steps 6 and 7 pass, the image is **tape-faithful** and **verity-sealed**.
The kernel enforces the seal at every read during VM operation.

## Reset and recovery

**Reset to tape state:** drop the overlayfs upper layer. The VM immediately
reverts to the tape-predicted base. No re-materialization needed — the EROFS
base is still there, still sealed, still mounted.

**Recovery from corruption:** re-materialize from tape. The EROFS image is
reproducible, so the new image will be byte-identical (same root hash). The
verity seal detects any divergence.

**Promotion:** a candidate autoputer's overlay changes are committed to the
tape as new journal entries. The next materialization produces a new EROFS
base incorporating those changes. The old base becomes a rollback point.

## Capsule integration

When Nucleus capsules are integrated (Mission C), each capsule:
- Bind-mounts the autoputer's `/base` read-only (shares the EROFS base)
- Gets its own overlayfs upper layer (ephemeral writes)
- Can be reset instantly by dropping its upper layer

This is exactly the model from the handoff doc: *"Capsules inside a candidate
VM may share read-only base state. Each capsule gets its own write layer."*

The EROFS base is the shared read-only state. The overlay upper is the
per-capsule write layer. The tape is the source of truth for the base.

## What this does NOT require

- **No ext4 image building**: EROFS replaces ext4 for the base layer
- **No loop mounting for audit**: go-erofs reads EROFS without mounting
- **No root for audit**: pure Go, cross-platform
- **No 32GB sparse file**: EROFS is sized to content
- **No extractor invertibility**: we don't reconstruct the tape from the image
- **No mke2fs/e2fsprogs dependency**: pure Go via go-erofs

## What this DOES require

- **go-erofs dependency**: `github.com/erofs/go-erofs` (pure Go, official)
- **veritysetup at build time**: for dm-verity sealing (Linux, build-time only)
- **dm-verity in guest kernel**: for runtime integrity enforcement
- **overlayfs in guest kernel**: for the writable upper layer
- **Deterministic generation**: StateGenerator must be deterministic (it is)

## EROFS vs ext4: the overlay tradeoff

The main tradeoff is that the autoputer's writes go to an overlayfs upper
layer instead of directly to the disk. This means:

1. **Writes are not persistent across VM reboots** unless the upper layer is
   backed by a persistent file. Solution: back the overlay upper with a
   per-VM ext4 file (small, e.g., 1-4GB, just for writes).

2. **The overlay can grow unbounded**. Solution: quota or periodic commit
   to tape (overlay changes → journal entries → new EROFS base on next
   materialization).

3. **fsync semantics differ** on overlayfs. Solution: for databases (Dolt),
   back the overlay with a real ext4 file, or mount Dolt's data directory
   directly on a separate ext4 disk (not through the overlay).

For v0, the overlay upper is backed by a per-VM ext4 file in the VM data
directory. This gives persistence without compromising base integrity:

```
/base     ← EROFS (tape-materialized, dm-verity sealed, read-only)
/data     ← ext4 (per-VM, mutable, NOT tape-derived — "durable_legacy_opaque")
/         ← overlayfs (lowerdir=/base, upperdir=/data/upper, workdir=/data/work)
```

The `/data` ext4 disk is the same as today's `data.img` — it holds the
overlay upper layer and any direct-mount paths (e.g., Dolt). It is NOT
audited by the tape; only the base is. But the base is now auditable,
which is the advance.

## Mutation class

Orange — adds new materializer, new builder, new sealer, new auditor. Does
not change existing StateGenerator or TreeToFS. Does not change vmmanager
yet (that's gate 7). Rollback path: don't use the new materializer; existing
opaque data.img flow still works.

## SIAC gate advancement

- **Gate 2 (substrate boundary)**: ✅ EROFS base is behind the materializer
  boundary; vmmanager consumes it as a standard block device
- **Gate 3 (typed durable state slice)**: ✅ the EROFS base is a typed,
  tape-derived, content-addressed state slice (verity root hash)
- **Gate 4 (cross-substrate proof)**: ✅ same tape → EROFS base on
  Firecracker; same tape → directory on host; equivalence checker proves
  they match
- **Gate 5 (failure proof)**: ✅ seeded mismatch in tape → different EROFS
  image → different verity root hash → audit fails
- **Gate 7 (staging proof)**: ❌ blocked until vmmanager is wired to boot
  from the materialized base (separate mission)

## Open questions

1. Should the EROFS base be the entire root filesystem, or just the durable
   data layer (mounted at /data or /persist)?
2. How does the overlay upper interact with Dolt? Does Dolt need a direct
   ext4 mount (not through overlay)?
3. Should capsules get their own EROFS base (tape-derived) or share the
   autoputer's base?
4. How often should overlay changes be committed to the tape? On every
   promotion? Periodically? On shutdown?
5. Does the guest kernel need dm-verity built in? (Most modern Linux
   kernels do, but the Nix-built kernel config should be verified.)
6. Should we use go-erofs for both build AND read (audit), or use
   mkfs.erofs for build and go-erofs for read only?
7. How does veritysetup work on macOS for build-time sealing? (It doesn't —
   sealing must happen on Linux. Is that acceptable, or do we need a pure-Go
   verity implementation?)
