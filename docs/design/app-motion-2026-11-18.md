# App motion: the frame never leaves (2026-11-18)

Standing: built | kind: happened | settled-by: mobile/AutoradioLab/transitions.contract and mobile/scripts/check-no-loading.py exist on origin/main (joined in 33cf20826); NoWaitingWordsTests present; no status line in the doc itself

Owner, 2026-11-17, after tapping Quarantine on the Mail page: the whole page below the title became "Loading your mail..." and the tab row
vanished until the list came back. "Anytime the screen changes, there should be animations... get rid of anywhere it says loading... anything
which stays constant, like the headings, should stay visible during the animation. Ideally, they could even be clickable." And: "we should
start the animation before the new content appears so that we do give immediate visual feedback that your user action is causing an effect."

## The rule

**The frame never leaves.** A page's title, tab row, headings and bars are drawn from state that needs no fetch (the address, the cache, a
fixed list of tabs) and sit outside every transition, so they are on screen and tappable at every moment. Only the content under them is a
`Swap`. The transition **starts at the tap**: the action that names the new tab also changes the Swap's token, so in the same frame the tab
underline moves, the outgoing content leaves in the tab's direction, and a skeleton (quiet bars, no words) holds the place until the answer
arrives; then a second token change plays the incoming half. A tab with a cached list runs the whole transition at once and refreshes underneath.
A tap during a load wins (Exact adopts only the newest request; the kit model refuses a stale generation). An error is still said in words with
a retry. No page says "Loading" or "Opening": `mobile/scripts/check-no-loading.py` fails if one is added (run by `NoWaitingWordsTests` and
`proof-app.sh`).

## The named transitions (`mobile/AutoradioLab/transitions.contract`, the one place)

| Name | Time, curve | Used for |
| --- | --- | --- |
| tabIn / tabOut | 220 / 160 ms, ease-out / ease-in, 24 pt slide plus fade | content under a tab or segment, in the tab's direction |
| fade | 160 / 120 ms | Reduce Motion (every slide and rise becomes this); a skeleton to content |
| rise | 260 / 200 ms, cubic-bezier(.16,1,.3,1) | the tray, the bar's line (now playing, paused, listening), an expanded "Built on" |
| layout | 240 ms ease | rows inserted, removed or reordered: mail rows, Up next, Heard |
| press | 120 ms (`th.motionFastMs`) | colours and tab underlines at a tap (existing) |
| deck spring | `th` spring | opening a row into its detail, and a place switch from the tray (the page deck, existing) |
| skeleton | 1.2 s breathing opacity | `SkeletonRows`, `SkeletonBody`, `SkeletonLine` |

Where each is used: Mail tabs (`Swap`); on-air entry changing on the Autoradio page (the entry block is keyed by the title: old leaves, new
enters); Up next and Heard rows (layout and exit); Built on under a Heard row (rise); the tray and the bar's line (rise, with an exit);
Files, Textures, Podcasts, a file or document that is opening, a message that is opening, and the account line (skeletons in place of words).

Reduce Motion is the system setting (`th.reducedMotion`, set by the theme loader): every slide and rise is a cross-fade and layout transitions
are off. All of it is Exact's compositor motion (`transition`, `keyframes` with `animation`, `-exact-exit-animation`,
`-exact-layout-transition`); no frame loop. A new tap replaces the running transition; nothing blocks input, and nothing touches audio.

## The testable model

`AutoradioKit/MailboxTabs.swift` is the tab row as a pure model: `select(folder)` is synchronous and sets the phase (leaving or entering), the
direction and the frame before any fetch; `accept` discards an answer for a tab that is no longer selected. The contract mirrors it
(`mailTab` in `members-mail.contract` sets `mboxSel` and `mboxDir` in the same action). Tests: `MailboxTabsTests`, `NoWaitingWordsTests`.

## What Exact could not do, and what only a device shows

- A keyed `for` with `-exact-exit-animation` is the nearest thing to a view transition for content swaps; Exact has no cross-view
  "transition between two screens" for content inside a page, and an exit does not run inside a screen being popped. The deck's own spring
  stays the way a row opens into its detail and back.
- `-exact-layout-transition` on rows of the virtualized Up next list is accepted by the compiler; whether the native list animates a reorder
  as well as an insert or removal is for a device to show.
- Single frames cannot prove smoothness. The tour `mobile/fixtures/tours/page-motion.steps` takes mid-transition and after shots for the lead.
