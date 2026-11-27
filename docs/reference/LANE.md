# Lane: copy-on-write worktrees

What it is: `lane`, a Rust command by Luke Edwards (MIT,
`https://github.com/lukeed/lane`), version 0.2.0 at commit `3cebe2c`, read
and built from source on 2026-11-15 (`cargo install --path crates/lane --root
~/.local --locked`; the clone is `~/Dev/third-party/lane`). The binary is
`~/.local/bin/lane`. It calls no model and has no network code (**verified**
by reading the source, 2026-11-15); it runs `git`, `df`, `sync` and the
hooks you install.

It also keeps notes about files (`lane note`, `lane why`, `lane check`). We
do not use that part: `lane init` is not run here, because it writes
`AGENTS.md` and a `.lane/memory` store.

## How we use it

A worktree for any change, made from the primary checkout:

```sh
cd /Users/wiz/Dev/go-choir && git fetch -q
lane new <name> --base origin/main     # prints the path: .lane/trees/<name>
lane ls
lane rm <name>                         # refuses if work would be lost; --force discards
```

`lane new` makes a branch `<name>` and a git worktree under `.lane/trees/`,
then clones the primary checkout's ignored files into it by reference
(`clonefile(2)` on APFS). Measured here on 2026-11-15 (**verified**): 2.2 s,
21,488 files, 1,067 MiB shared, about 110 MB of new disk. Other lanes and
nested worktrees (`.claude/worktrees/`) are skipped. `.lane/trees/` is added
to `.git/info/exclude`, so nothing under it is committed by the primary
checkout.

Push from a lane as from any worktree (`git push origin HEAD:main` for docs,
`scripts/land` for a landing, `scripts/land-queue` for several: it lands them
as one batch, one verify and one push, finds a conflicting or failing lane by
splitting and sets it aside, and does not wait for CI between pushes (it runs in
a lane of its own, `land-queue-<pid>-<epoch>`, made and removed with `lane`, and
never resets a tree given with `--tree`);
`--serial` lands one at a time, each waiting for the last one's CI and staging,
for lanes that each need a deployed proof). We do not use `lane merge` or `lane push`: the landing loop is `scripts/land`.

## Rules

- Always pass `--base origin/main` after a fetch. The default base is the
  local `main`, which can lag behind the remote. Fast-forward a clean main
  checkout after a landing.
- A lane holds a copy of `.env` and `.envrc.local`. They are ignored files
  and stay local; the secret rules apply to a lane as to the primary
  checkout.
- A lane starts as warm as the primary checkout and no warmer. The primary
  checkout holds no mobile build caches, so a lane does not either. Mobile
  builds happen in one build worktree, run by the lead
  (`CLAUDE.md`, "One Mac build at a time").
- A lane is cheap when made and grows with what is compiled in it. Lane
  shares only what the primary checkout held at the moment of the clone. On
  2026-11-17 (**verified**, `du` and `df` before and after) 59 lanes and 184
  older worktrees held 124 GiB of per-tree Exact copies and compiled output;
  deleting it took the Mac from 30 to 154 GiB free. `scripts/gc-build-output`
  removes that output from idle trees and is safe to rerun
  (`CLAUDE.md`, "Build output lives in one place").
- Untracked, non-ignored files are not carried (`--dirty` carries them). Do
  not use `--dirty` from the primary checkout: it would carry the kept
  secret backup file.

## Mistakes already made

- 2026-11-14: adopted "when convenient" and never installed. Workers kept
  making full worktrees with cold caches; `.claude/worktrees` reached 95 GB
  across 282 worktrees.
- `lane rm` checks local ancestry. A lane can already be on `origin/main`
  even if local `main` lags or the lane was rebased or squash-merged. Fetch
  before comparing. An empty `git log --right-only --cherry-pick --no-merges
  origin/main...<name>` accounts for equivalent patches but says nothing
  about merge commits or uncommitted changes. For a squash, compare the
  paths the lane changed against the landing tree.

- **Relative worktree paths broke the landing gate (2026-11-15).** Lane was
  first installed with relative paths, which set the repository extension
  `extensions.relativeworktrees`. Nix refuses a repository with an extension
  it does not know, so `nix eval` in `scripts/land` failed for every lane.
  The installed lane is built from a local patch that writes absolute
  paths; the extension was unset and `worktree.useRelativePaths` set to
  false. A rebuild of lane from upstream must carry that patch or pass the
  equivalent flag, and the test is `nix eval` in a fresh lane, not `lane
  new` succeeding.
## Retiring lanes

Retire a lane after its work lands or is explicitly dropped. Check tracked
and untracked changes, review receipts, pending notes and whether the lane
is in use. A pushed branch alone is not evidence its work is on main.
Before forcing removal of an accounted-for lane, save its tip with a verified
Git bundle or an archive ref. A bundle saves commits, not a dirty worktree.
Before deleting a pushed lane, check open PRs and use a lease against the
inspected remote tip. Leave the pinned `qa-mac` build tree in place.

The 2026-11-25 audit's inventory, tip bundles and deletion logs live under
`~/.local/share/choir/lane-archive-2026-11-25/` on the Mac. The inventory
records the original refs and SHAs; the cleanup plans map bundled refs back
to lane names. Check `git bundle verify` and `git bundle list-heads` first.
Tips already reachable from main may be recorded by SHA only. Recover an
archived tip to a new branch, without moving `main`.

That pass removed 78 local lanes and 88 pushed `lane/*` branches, and
pruned 98 registrations for directories already missing. A remaining branch
can carry real work or an old draft that still needs comparison. In
particular, keep the unfinished `unified-voice-download` edits and the
deferred `move-b` and `settle-once` work out of a bulk landing.

## Sources

- `https://github.com/lukeed/lane` readme, fetched 2026-11-15.
- Source at `3cebe2c`: `crates/lane/src/cow.rs` (what is cloned and
  skipped), `crates/lane/Cargo.toml` (dependencies).
