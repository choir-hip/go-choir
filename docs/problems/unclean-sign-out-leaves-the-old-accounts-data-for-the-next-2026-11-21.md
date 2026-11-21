# A sign-out that does not finish cleanly leaves the old account's data for the next account

Recorded 2026-11-21T04:04Z by the lead, from seat 2's sixth review of the takes lane. On
main today (`mobile/AutoradioLab/account.ts`). Not fixed; the fix is its own
red change (lane `account-settle-memo`), not part of the takes move.

## Problem

Shared code settles the account once at launch and remembers the answer
(`settled`, read before any page's source runs, `app.ts:135`). A sign-out
that ends other than cleanly leaves that remembered answer in place. If a
second account then signs in without a relaunch, the settle step answers
"nothing to do", the first account's shared files and stamp remain, and the
second account's pages read them.

## Evidence

The reviewer ran the real modules with the test phone's own failure switches:

- sign-out times out after the key is deleted (`account.ts:244`): result
  `done:false, failed:''`, marker and stamp still the first account's;
- native throws or reports not done (`:246-251`);
- the late continuation (`:240`) returns on any present key without checking
  whose, so a second sign-in in the window suppresses the late wipe;
- a failed re-settle is itself remembered (`settleAccount`, `:266`).

In each: key = B, `settleAccount` returns the launch answer, action
"nothing", and `app:/data/takes.json` and `podcasts-library.json` of A are
still present.

## Belief

Reachable only when the native half of a sign-out hangs or throws after the
key delete, and the app is not relaunched before the next sign-in. Rare, but
it is private data crossing accounts, which the product forbids outright.
The app is not launched and has one user, so nothing has leaked.

## Remaining error

Whether the account page offers sign-in while a sign-out is still pending
was read, not run. The same review notes `Conversation.begin` has no
generation guard (a late question could be added after a sign-out); not
reproduced.

## Next

Tests for the four exits first, then: forget the remembered answer on every
unclean result, forget it when a settle reports a failure, and make the late
continuation check whose key it sees. Red: two seats, proof on the simulator
drill.

## Two builds, both sent back by both seats; the lane is stopped for a design (2026-11-21T05:32Z)

Lane `account-settle-memo`. Nothing of it is on main.

1. `eb0604d12` (three guards that reset the remembered answer at exits).
   Seat 2: one guard's retry wipes the current account's data on every page
   answer. Seat 1: three more paths where the answer stays remembered.
2. `579ceb2db` (the answer remembered only for the observed state; a list of
   stores still to wipe kept in the stamp). Seat 2: `app.ts:135-139` discards
   the settle result, so when a wipe or the stamp write fails the page source
   answers anyway and a listed store is readable by the next account. Seat 1:
   the same fail-open, plus a failed marker removal remembered as clean, two
   settles racing through an account change, and an unknown name in the list
   looping forever.

Belief. Both builds worked on what settle remembers. Neither changed what a
page source does when settle has not finished cleanly, and that is where the
data crosses: the gate fails open. The questions a design has to answer
before any code: (a) what a page source is allowed to read while the account
state is not known clean, and what the person sees then; (b) who is the one
writer of the account state, so two settles cannot race; (c) what is durable
and in what order, so a crash at any point restarts into a state that
refuses rather than reads; (d) how native stores (`HeldPages`, `TakeMirror`
and the rest of the wipe list) take the same boundary. The reviews are in
the session scratchpad (`review-acct/r1`, `seat1`, `r2`, `seat1b`).

Remaining error: the defect is still on main. It needs a sign-out whose
native half hangs or throws, then a second account without a relaunch; the
app has one user and is not launched.

Next: a written design (the account boundary in shared code), reviewed
before code, then tests from its table.

## Landing 1 built, not landed (2026-11-21)

Lane `account-boundary-l1`. Shared code and contracts only, from `docs/design/account-boundary-2026-11-21.md`:
the gate in `app.ts` is default-refuse with a named exempt list; the boundary is derived from the key and the
record before every gated answer (`account.ts`); one writer wipes, then removes the record; the marker file and
the remembered settle answer are gone. Seen: the whole `bun test`, the contract build and `test web` pass in the
lane. Not seen: the simulator drills D1 to D4 (written into `proof-real.sh`, not run), two review seats, a
device (the Exact error code on a data-protected file before first unlock is unread: design section 10). The
defect stays open for the native side until landings 2 and 3; Remaining error: a background Clear's deadline
timer may be cut when the answer that started it ends (Exact timers end with the answer, EXACT.md); a Clear that
hangs in native then holds the one writer until the host drops the call. Next: two seats, then the drills.

Recorded from seat 1's review of landing 1 (not fixed here; landings 2 and 3): native `wipe` answers `done:true`
unconditionally and sends no `failed` and reads no generation (`ModelsEntry.swift`); native stores swallow removal
errors; a launch-time wipe with key B also runs `MailSeal().forget()`; native read sources stay ungated until
landing 3; a theme installed from account A's Texture document stays visible to B (owner decision 4).
Fixed in landing 1 after the review: the proof's record checks are taken after their own drive; the epoch is
`state|wipes` so sign-in re-asks the pages; a throwing key read is LOCKED with words and buttons; the shared half of a
Clear has a deadline (`timing.sharedMs`) and counts as failed; the card's Adopt write is stated in the design.

Fixed in landing 1 after seat 2's review (fe027e0fc): a late read no longer plants the old account's data in a module
cache (`sameAccount()` in `api.ts`, one helper; files recents, podcast library and cached feeds, clips, mail folder,
alias and saved drafts, demo leftovers' write); the proof's D3 plants in a file `reset_state` leaves; a Clear the card
has seen running for six reads is abandoned on a fresh chain (no clock is read); `survivors` stats `content.db-wal`
and `-shm`; Texture-installed themes are wiped; the card never waits on a running job; sign-out forgets a refused key.

Mechanism change after the second review round (2026-11-21): the per-site guards and the abandon rule are deleted.
The boundary moving closes the account's world (`api.ts`); every I/O choke point is inert for a closed world; the
Clear drains started writes before it wipes; a timed-out Clear stays CLEARING with the close-and-reopen words. See
the design section "The mechanism: the world". Named regressions: `account-world.test.ts` (C8, C9, C9b, M1 to M4).

Third round (seat 1 on c222cbf80): module-level single-flight and mutex state outlived its world (a save chain that never
settles blocked every later page change). Module state now lives in the world (`scoped()`, `api.ts`); the gate takes the
world `enter` confirmed; native calls of account chains are inert for a closed world. Residual: the exempt theme sources
can show a Texture theme between a key swap and the card's next read (at most a few seconds); `opts.world` has no caller
until the run sheet's poster uses it (the train).

Fourth round (seat 2 on c222cbf80): the old-theme carry-over no longer writes a Texture theme once the boundary moved;
a host download is tracked by the drain and its cleanup uses the raw storage; the record write and removal have
deadlines and run on the current world's storage (a late landing is waited for by the next boundary move); the card's
epoch carries the world's number; `authedJSON`, `authedBytes` and `authedToFile` take the world as a required field of
their options (the type checker lists every caller); the takes session's requests belong to the world its answer began
in, and the Texture theme install carries the world it began in. Every remaining raw native op is listed with its reason
in the M3 test. Not verified: the real host's `fs.rm` on a non-empty folder (every Clear removes folders); a Clear would
stay CLEARING if the device refuses it.

Fifth round (seat 1's residuals on 0ced3a4ee): an Adopt waits for the writes a closed world started and is not stacked on
a write of its own world that has not ended; every folder wipe goes through `rmTree`; the device theme store loads on
the unbound storage; a signed-in request that names no world is refused (the tests state theirs); `content` running,
`deck-ops` posting and `runsheet-entries` longURLs are world state; a download that ends after its world closed removes
nothing itself. What remains, stated: the non-I/O-await gap (a chain resumed from a microtask or a promise another chain
resolves, which then starts new I/O, begins it in the new world); M3 is regex-based and does not see state inside
functions or classes, or `const x = makeThing()`; the native landings 2 and 3; the real device's behaviour (no device run).

Sixth round (seat 2 on 0ced3a4ee): themes are two stores. The DEVICE store (bundled themes, the person's own theme files,
the choice) is device settings on raw storage; a Texture theme can never enter it. The TEXTURE store is account data:
`app:/data/theme-textures`, on the wipe list, state in the world (`scoped`), every op (list, install, remove) through the
world it began in; a Texture theme is never carried over from the old build; the Clear drops a choice that named one.
A Clear no longer waits for a download longer than its deadline (late bytes are removed by the next Clear's removal of the
cache folder). An Adopt stamps the new world with the digest of the key it wrote. `tsc` (Exact's, with a throwaway
config over the non-test sources) reports 0 errors touching `world` and 9 unrelated ones (node types in tooling files and
one comparison in `deck-ops.ts`); it is not in the gates.

Seventh round (seat 1 on a7a46fb10): the Texture-theme status line ("Added <theme>") is the world's; the files cache path
names the account (`<digest16>/<hash>/<name>`, `cacheDest` in `files.ts`) because a download is written by the host and
may land after the Clear; a Remove that resumes after the boundary moved does nothing; the Clear decides the theme choice
by its recorded origin and fails if it cannot clear it; `rmTree` retries a folder that refused because a child appeared;
a leftover mail original is counted as "mail copies", never named. Remaining, stated: for the first frame after an
account change the theme sources can still answer with the previous account's colours before the new answer (the contract
re-asks them with `acctTick`); a download that lands after the Clear stays under the old account's digest folder until the
next Clear removes the whole cache.

Round six (seat 1 on fad376d02, sent back). Fixed in this commit: (1) an exempt source is handed a DEVICE storage handle
(`deviceStorage` in `api.ts`, paths in `themes.ts` `DEVICE_PATHS`): only the choice file and the person's themes folder, any
other path or function throws, the Texture folder only through a world the boundary has confirmed
(`confirmed(w)`: digest set, not owed, not closed); the card (`peek`) confirms a world as the gate does, and the epoch
carries it so the theme sources are asked again. Cold launch with a changed key: nothing of A's Texture themes is listed,
choosable or removable (test C16.cold-launch). (3) An unstamped world refuses a signed request (after the key read, so
the reason stays `locked` or `none` when that is true); an Adopt stamps its world itself. (5) `closeWorld` resets the
computer-id lookup. (6) `lastTarget` in `mail-sanitize.ts` is deleted (the target is returned). (7) The M3 scanner is a
small lexer (`module-scan.ts`): Exact's TypeScript is the native 7.x, which gives a bun test no parser. Every top-level
binding is provably immutable, an unchanged literal table (checked in every importing file), or listed with its reason.
Limits of the scanner: it does not follow an alias (`const m = TABLE; m.set()`), sees only a module's top level, and
trusts its mask of regular expressions by the previous token.
Found, not fixed (moot if the Clear is removed): a Clear that keeps failing restarts itself without bound. Each attempt
moves the generation and the epoch; every gated page re-asks when the epoch moves; a refused gated page calls `enter`,
which starts a Clear when none runs (`account.ts` `enter`, `startClear`); only the card's re-read is capped. Not
reproduced by a test. Also not fixed: a choice write on the device store (`saving`) or a first load (`loading`) that began
before a Clear is not drained by it, so a Texture choice with A's id can land after `wipeTextureThemes` rewrote the
choice file.
Residuals, stated: a late download can leave A's bytes under `app:/cache/files/<digestA>/` until the next Clear (B never
reads that path); an Adopt is refused after a hung host write until the app restarts (the card says so); a chain resumed
from a non-I/O await after the close still starts its I/O in the new world.
Native wipe and the run sheet: covered. `AccountWipe.swift:83` registers `run-sheet` and `RunSheetStore.wipe()`
(`RunSheetStore.swift:323-328`) cancels the retry, empties the sheet and pending ops, drops the poster and removes the file.
The shared `runsheet-store.ts` keeps no file under `app:/` and its state is scoped to the world.
