# MA6.5: one runtime, yaegi with Go modules, for every desk (2026-11-06)

Class red, tier 2. Rollback: revert. Plan row:
docs/design/modular-architecture-plan-2026-11-04.md, MA6.5; rulings R13
(expressivity after security), R26 (per-desk import sets, code mode), R42
(third-party Go modules gated by capability, not by lines); open question 9.
Built on MA6.4 (docs/design/ma64-gvisor-desk-workers-2026-11-06.md): the
widened set is safe to offer only because every desk worker now runs inside
gVisor, as its own uid, under the cell seccomp filter.

## 1. What changes

1. **The desk stdlib set widens by expressivity** (section 2). One list,
   `internal/gomodule.Stdlib()`, is now the source for the yaegi cell
   allowlist (`yaegikernel.DefaultSafeStdlibPackages`), the package desk
   import set (`apppackage.Manifest.ImportSet`) and the module closure check.
   Before this slice the first two were two copies held equal by a test.
2. **A package ships Go modules** (section 3): the manifest names them, the
   release carries their full module source, the platform checks each against
   a curated list carrying the Go checksum database hash, and walks the
   transitive import closure from the packages each desk imports.
3. **The desk worker loads a verified bundle** (section 5) into the cell's
   yaegi interpreter from an in-memory filesystem built from the verified
   bytes. The interpreter no longer has the real filesystem as its source
   filesystem at all, bundle or not (it had `os.DirFS`-like `realfs` by
   default; the allowlist kept cells from reaching it, but nothing else did).

## 2. The widened stdlib set

The rule: a package enters when (a) yaegi v0.16.1 ships its symbols, (b) no
function in it reaches the OS (files, processes, sockets, environment, exit)
or the interpreter's own internals, and (c) it adds expressivity a desk cell
or a pure library needs. Everything else stays out: the list is closed, so a
package not named is refused, whatever the banned list says.

Kept (16): `bytes encoding/base64 encoding/hex encoding/json errors fmt math
math/big math/rand/v2 regexp sort strconv strings time unicode unicode/utf8`.

Added (38), by what they buy:

| Package(s) | Why |
|---|---|
| `sync`, `sync/atomic`, `context` | code mode (R26): fan out a hundred verb calls in goroutines, join them with a WaitGroup, bound them with a context. Cells already start goroutines; without `sync` they cannot join them safely |
| `slices`, `maps`, `cmp` | the generic helpers models reach for first (search, compact, equality, keys; yaegi v0.16.1 lacks `slices.Sort`, section 8) |
| `container/heap`, `container/list`, `container/ring` | ranking and queues without hand-rolled structures |
| `math/bits`, `math/cmplx`, `math/rand` | numeric completeness (`math/rand` v1 for libraries written against it) |
| `unicode/utf16`, `text/tabwriter`, `text/scanner`, `html`, `bufio`, `io`, `path` | text work over values the desk already holds: escaping, line scanning over a `strings.Reader`, aligned tables, slash-path joins (`path`, never `path/filepath`) |
| `encoding`, `encoding/csv`, `encoding/xml`, `encoding/binary`, `encoding/base32`, `encoding/pem` | formats a desk reads through its verbs (feeds are XML, exports are CSV) |
| `hash`, `hash/crc32`, `hash/fnv`, `hash/adler32`, `crypto/sha256`, `crypto/sha1`, `crypto/sha512`, `crypto/md5`, `crypto/hmac` | content digests and dedup keys; hashes compute, they reach nothing |
| `net/url` | URL parsing and building for research; it opens no socket (`net` and `net/http` stay banned) |
| `compress/gzip`, `compress/flate`, `compress/zlib` | unpack bytes a verb returned; memory is bounded by the worker's own limit |

Still out, with the reason:

- banned outright (named in `gomodule.Banned` and `yaegikernel.BannedPackages`):
  `unsafe`, `reflect`, `runtime` and `runtime/*`, `plugin`, `syscall`, `os`,
  `os/*`, `net`, `net/http`, `net/rpc`, `io/ioutil`, `embed`, `debug/*`,
  `C` (cgo);
- not offered, because a function in them reaches the OS: `path/filepath`
  (Walk, Glob, EvalSymlinks), `text/template` and `html/template`
  (ParseFiles, ParseGlob), `log` (writes the process's stderr, `Fatal` exits),
  `mime` (reads system type files), `archive/zip` (`OpenReader`), `crypto/rand`
  (OS entropy; `math/rand/v2` serves cells), `io/fs` (nothing to offer
  without an OS filesystem);
- not offered, for now: `encoding/gob` (reflection-driven global type
  registry), `go/*`, `testing/*`, `iter` and later packages (yaegi v0.16.1
  has no symbols for them), `hash/maphash`.

## 3. How a package ships a Go module

- **Manifest.** Package level:
  `"go_modules": ["github.com/pmezard/go-difflib@v1.0.1-0.20181226105442-5d4384ee4fb2"]`
  (at most 8, one version per module path). Desk level:
  `"go_imports": ["github.com/pmezard/go-difflib/difflib"]` (at most 16): the
  packages that desk's cells may import, each inside a declared module and not
  an `internal` package. A desk that names no `go_imports` loads no module.
- **Vendored source in the release.** Each module's full module content (what
  its module zip holds, as `go mod download` extracts it) lives at
  `gomod/<escaped path>@<version>/` in the package tree (the module cache's
  own escaping: an upper-case letter is `!` plus its lower case). A file under
  `gomod/` outside a declared module is refused. The full content is required
  because the Go checksum database hashes the whole module zip, tests and
  licence included.
- **Pinned and checked against the Go checksum database.** The curated list
  (`internal/gomodule/curated.txt`, checked in) lists `path version h1:hash`,
  the hash as `go.sum` and sum.golang.org record it. At publish (and again at
  install, which runs the same `CheckManifest`) the platform computes the
  module's `h1:` (dirhash Hash1 over `path@version/<file>` names) from the
  release's bytes and refuses a mismatch. A module not on the list, or on it
  at another version, is refused. No network is used at publish: the hash was
  checked against the checksum database when the entry was added
  (`scripts/check-curated-go-modules` re-checks every entry with
  `go mod download`, which verifies against `GOSUMDB`).
- **Behaviour-surface hash.** Every file of the release tree was already in the
  hash; the surface document now also names each module (`path`, `version`,
  `h1`) and each desk's `go_imports`, both omitted when empty so every
  earlier package's hash holds. A version bump is a new hash, so it is an
  update the owner approves (the change discount of R42 is Phase 7's).

## 4. The import-closure check (mechanical)

`gomodule.Resolve` walks, with `go/parser` (imports and comments only; the
code is never type-checked or run at publish), from each desk's `go_imports`
through every package they reach. For every non-test `.go` file of every
reached package, whatever its build constraints (a `//go:build linux` file is
checked like any other), it refuses:

- an import of a banned package (`banned_import`);
- a standard-library import outside the desk stdlib set (`outside_import_set`);
- `import "C"` (`cgo`), a `//go:linkname`, `//go:embed` or `//go:cgo_*`
  directive (`directive`);
- an import of `choir` (`outside_import_set`: a library computes, the cell
  acts; the verbs are the desk's, never a module's), and a reached package
  named `choir` or whose path ends in `/choir` (`reserved_name`: a cell's
  import of such a path is bound to the host verbs by the cell's import
  aliasing, review r1 D8);
- a relative import, or a non-standard import that no declared module
  provides (`unresolved_import`), or an `internal` package imported from
  outside its parent (`internal_import`);
- a reached package directory holding assembly, C or other non-Go build
  inputs (every input the go tool passes to cgo, the assembler or the
  linker: `.s .S .sx .c .cc .cpp .cxx .h .hh .hpp .hxx .m .mm .f .F .for .f90
  .syso .swig .swigcxx`) (`non_go_source`);
- a reached package with no non-test Go file (`unresolved_import`);
- a closure over the caps (section 7) (`too_large`).

Before the walk, `gomodule.Verify` refuses a module path declared twice or
lying inside another declared module's path (`overlapping_modules`, review r1
D6: the module providing a package would be ambiguous).

The refusal names the chain (`a -> b -> c imports "os/exec"`). Packages of a
module that no desk reaches are not checked and are not loaded: a module's
`cmd/` tool importing `os` does not refuse the library.

## 5. How the worker loads a module

- The host passes the desk's `gomodule.Bundle` in
  `SessionWorkerConfig.GoModules`: the whole content of every module the
  desk's closure reaches (tests and licence included, so it can be re-hashed),
  each with its curated `h1:`, and the cell-importable packages; at most
  512 KiB of content. The spawner renders its canonical encoding in the
  worker's environment as base64 parts of at most 120 KiB each
  (`CHOIR_DESK_SESSION_GO_MODULES_PARTS`, `_GO_MODULES_<i>`; at most 9 parts,
  about 1 MiB in all, under execve's 128 KiB per-string and 2 MiB total
  limits) beside its sha256 (`_GO_MODULES_SHA256`), which is required whenever
  bytes are carried (review r1 D3); the environment crosses into the gVisor
  process spec unchanged (MA6.4).
- The worker joins the parts, checks the sha256, decodes strictly
  (unknown fields refused, sizes bounded) and loads the bundle
  (`gomodule.Bundle.Load`) against its own binary's curated list: every
  module re-hashed against its `h1:` and on the list at that version and sum
  (review r1 D2), the imports and the whole closure re-walked, every carried
  module reached. The bytes were checked at publish; the worker trusts only
  what it checks. Any failure ends the worker before it is ready (exit 2): a
  desk with a broken bundle runs no cell, never one without its modules.
- The interpreter's `SourcecodeFilesystem` is a read-only in-memory
  filesystem holding `src/<import path>/<file>` for the non-test Go files of
  the reached packages only, `GoPath` `.`; with no bundle it is empty. yaegi
  resolves a non-symbol import there and nowhere else. A reached package whose
  path is the path of a binary symbol table (any key of yaegi's
  `stdlib.Symbols`, which includes yaegi's own unfiltered table, or a prebound
  package) is refused.
- **Binary symbols come from the base allowlist only** (review r1 S2). The
  bundle's imports widen only the static import check; they never select a
  binary symbol table. Module code imports stdlib through the same filtered
  table the cell has, so a banned import that slipped the static check would
  still find no symbols.
- **No symbol that mentions `unsafe.Pointer`** (review r1 S1). yaegi converts
  `*T` and `uintptr` to and from `unsafe.Pointer`, so a single bound symbol of
  that type brings `unsafe` into a cell without the `unsafe` package.
  `buildFilteredSymbols` drops every symbol whose type reaches
  `unsafe.Pointer` through its elements, signature, exported fields or methods
  (including an interface's unexported methods). On the 54-package set that
  drops `sync/atomic`'s `LoadPointer`, `StorePointer`, `SwapPointer` and
  `CompareAndSwapPointer`, and the error types `json.UnmarshalTypeError`,
  `UnmarshalFieldError`, `UnsupportedValueError`, `UnsupportedTypeError`,
  `InvalidUnmarshalError`, `MarshalerError` and `xml.TagPathError`,
  `UnsupportedTypeError`, whose exported `reflect.Type`/`reflect.Value` fields
  would also hand a cell `reflect` (a cell still sees these errors' text; it
  cannot name their types). A test scans every bound symbol independently.
- The cell allowlist is the desk set plus the bundle's cell-importable
  packages. Each of those packages is imported once when the session starts
  (like `fmt`), each bounded by 5 s (below the 10 s ready timeout; review r1
  D4), refused by name ("its init did not finish within 5s"): module init code
  runs at worker start, where a failure refuses the worker, never inside a
  cell whose failure would be classified as a compile error (nothing
  executed) while module code had run.

## 6. Failure modes, written before the code

1. A module imports a banned package directly or through a dependency, and is
   published: the closure walk visits every reached package (test: transitive
   `os/exec`, `unsafe`, `syscall`, `net`).
2. A banned import hides in a file excluded by build tags or named `_linux.go`:
   every non-test `.go` file is checked, tags ignored (test).
3. cgo, `//go:linkname`, `//go:embed` or assembly reaches the interpreter
   (test for each).
4. A module not on the curated list, or at another version, or with bytes
   that differ from its curated hash, is published (tests: unlisted, wrong
   version, one byte changed).
5. Our `h1:` differs from Go's, so the curated hash proves nothing (test: the
   vendored go-humanize v1.0.1 hashes to its `go.sum` line exactly).
6. A stray Go file under `gomod/` outside a declared module is loaded (refused
   at publish; the worker's FS holds only reached packages).
7. A cell imports a module package its desk did not declare, or an internal
   or transitive package directly: the cell allowlist holds only the desk's
   `go_imports` (test).
8. The bundle is corrupted or truncated in the environment, or a part is
   missing: sha256 mismatch, the worker exits before ready (test).
9. The interpreter reads a package from the real filesystem: the source
   filesystem is the in-memory one, always (test: a cell importing a path
   that exists on disk is refused, and the session's FS is never `realfs`).
10. Module init code panics inside a cell and the session is wrongly kept:
    modules are imported at session start, so a panic refuses the worker.
11. The widened stdlib reaches the OS under the sandbox: only packages with no
    OS-reaching function are added; the confined-worker test runs a cell
    using the new packages under the seccomp filter (Linux).
12. The environment carry exceeds what execve or the sandbox accepts: the
    bundle is capped at 512 KiB of content, at most 9 parts of 120 KiB, under
    the 128 KiB per-string and 2 MiB total limits (a cap-sized bundle crosses
    into gVisor in a test).
13. Packages without modules change hash: the new surface fields are
    omitted when empty (a test pins two hashes the pre-MA6.5 tree computed).
14. A bound symbol brings `unsafe.Pointer` or `reflect` into a cell (review
    r1 S1; the filter above, a scan test, a cell naming `atomic.LoadPointer`
    is refused).
15. A bundle import selects an unfiltered binary symbol table (review r1 S2;
    a module at yaegi's own table path is refused).
16. A module's init never returns and the worker hangs until the spawner's
    ready timeout (review r1 D4; bounded and named).

## 7. Caps and the open items of question 9 (residuals, conservative)

- **Size or depth cap: taken conservatively.** At most 8 modules per package,
  16 `go_imports` per desk, 64 packages in a desk's closure, 512 KiB of
  content (the whole reached modules) in a desk's bundle, 2 MiB per declared
  module's content in a release (the release file cap
  already bounds each file). Raising any of these is a code change under
  review.
- **Who reviews curated-list additions: taken conservatively.** The list is a
  checked-in file, so an addition is a red code change under tier-2 review,
  and `scripts/check-curated-go-modules` must pass for it. No owner act or
  publisher can add to it at runtime. Opening further as module-publisher
  trust builds (R42) is Phase 7's.
- **Seed entry.** `github.com/pmezard/go-difflib` at
  `v1.0.1-0.20181226105442-5d4384ee4fb2`: unified and context diffs of line
  slices (a desk diffing Texture revisions), pure (`bufio bytes fmt io
  strings`, two of them offered only since this slice), already in Choir's
  own `go.sum` (its hash checked against sum.golang.org by every build and
  again by `scripts/check-curated-go-modules`), 44 KiB. The first choice,
  `github.com/dustin/go-humanize v1.0.1`, passes the closure check but does
  not interpret: yaegi v0.16.1 refuses `0.0 - math.MaxFloat64` ("mismatched
  types untyped float and untyped float64", number.go:76). So a curated
  entry also carries its module content in `internal/gomodule/testdata/gomod/`
  and `TestEveryCuratedModuleLoadsInYaegi` loads each of its packages that
  passes the closure check; go-humanize stays in testdata as the test that a
  module yaegi cannot load refuses the session (and so the worker), never a
  cell.

## 8. Residuals

- **A module's goroutine writes into a later cell's output** (review r1 S3).
  A module may start a goroutine (at init or from a call) that outlives the
  cell and prints, or touches shared state, while a later cell runs; its
  output lands in that later cell's transcript, the same as a cell's own
  leftover goroutine today. The sandbox bounds what it can reach (the worker's
  uid, the seccomp filter, gVisor); what it may do inside the desk's own
  session is controlled by curation: a module that starts background work is
  not curated. No mechanical check enforces it.

- **Install to spawn carry.** The worker loads whatever bundle the host
  passes, and `apppackage.Manifest.DeskGoBundle` builds the verified bundle
  from a release's files, but no daemon caller passes one yet: published desk
  packages are still refused by R24 (`CurrentPlatform().DeskSandbox` is
  false, ErrDeskNeedsSandbox), and first-party desks ship no modules. Flipping
  the R24 gate is section 12.1's (after MA6.1 to MA6.5 and the red-team pass
  of R27), not this slice's; when it flips, the install path stores the bundle
  beside the release and the desk spawn passes it.
- **yaegi limits.** Modules are interpreted, not compiled: generics edge cases,
  reflection-heavy code, some constant expressions (go-humanize above) and
  anything that needs `unsafe` will not run. That is a practical bound on what
  can be curated (R42), checked when an entry is added
  (`TestEveryCuratedModuleLoadsInYaegi`). In the stdlib set, yaegi v0.16.1
  carries generic source only for `slices.go`, `maps` and `cmp`: `slices.Sort`,
  `slices.SortFunc`, `slices.BinarySearch` and `slices.Min/Max` (slices'
  sort.go) are undefined; `sort` serves. A yaegi upgrade is its own slice.
- **Root-spawner and d@e proof.** The agents VM runs rootless runsc with a
  non-root spawner, so the root-only confinement tests (worker as the desk's
  uid, required mode) skip there as they did for MA6.4; the widened-stdlib cell
  ran under the seccomp filter in `TestDeskCellWorkerSandboxedWithoutRoot`.
  The deployed proof (the isolation self-test on d@e) waits for the push.
- **Per-desk import sets by grant (MA6.6).** Every desk gets the same stdlib
  set here; MA6.6 derives each desk's set from its grants.
- **The real filesystem fallback** existed before this slice (yaegi's default
  `realfs`) behind the allowlist only. Closing it is in this slice; it is in
  the ledger.

## 9. Measured (2026-11-06)

Agents VM (x86_64, rootless runsc, non-root spawner), `TestGvisorCellBenchmark`
(300 warm cells each, 30 for fib25), research worker. Before is the MA6.4 tree
(b1f3d58de), after is this slice; the `module` cell is a verb round trip plus
a unified diff with the curated go-difflib.

| Cell | before native p50 | before gVisor p50 | after native p50 | after gVisor p50 | after ratio |
|---|---|---|---|---|---|
| trivial | 194 us | 407 us | 224 us | 412 us | 1.83x |
| verb round trip | 320 us | 731 us | 347 us | 718 us | 2.07x |
| allocation loop | 4.10 ms | 5.08 ms | 4.05 ms | 5.28 ms | 1.30x |
| fib(25) | 243 ms | 268 ms | 244 ms | 268 ms | 1.10x |
| module (diff + verb) | - | - | 860 us | 1.45 ms | 1.69x |

The gVisor cells are unchanged within noise. Spawn to ready: native 13 ms
before, 18 ms after without modules and 21 ms with the go-difflib bundle
(about 3 ms to check and load it); under gVisor 82 ms for the first worker
(sandbox start included) and 43 ms for a second worker in the running sandbox
with the bundle. A bundle at the source cap (694 KB encoded, crossing in
several environment parts) reaches ready in 118 ms under gVisor
(`TestGvisorLargestGoModuleBundleCrosses`).

After review r1 (the worker re-hashes whole modules, the unsafe-symbol
filter): gVisor p50 trivial 410 us, verb 712 us, alloc 5.43 ms, fib25 268 ms,
module 1.33 ms; spawn to ready native 19 ms, 24 ms with the bundle; gVisor
88 ms first worker, 46 ms a second worker with the bundle; the cap bundle
124 ms.

Tests (where they ran): `internal/gomodule` (the h1 against go.sum, the
closure refusals, the bundle re-check), `internal/apppackage` and
`internal/pkgrelease` (the publish gate: transitive `os/exec`, `unsafe`,
`syscall`, `net` refused with the chain; unlisted, altered, stray, cgo,
linkname, assembly refused), and `internal/yaegikernel` (the real worker: a
cell calls go-difflib and `choir.ReadCapture` in one cell; a bad carry never
becomes ready; the interpreter never reads source from disk) on macOS and on
the agents VM natively and under gVisor.
