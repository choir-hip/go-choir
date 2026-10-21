# More research is slow, hidden from the desktop, and reports once (2026-10-21)

Status: open. Problem documentation first; no fix in this commit.
Found by: the owner on a phone, 11:21 UTC ("I tried to use more research
and it said it couldn't ask now"), and the traces below.
Mutation class of the fixes: orange (guest store query, frontend, prompts);
the Texture skip touches the desk handoff and is red.

## Owner direction (2026-10-21, this session)

- "More research should be a button in the texture toolbar. And the
  toolbar should be responsive for mobile — currently it overflows with
  large text."
- "Yes, it should skip texture. But research should also be faster, and
  send iterative reports to texture, not wait until after multiple
  searches. (This is our ordinary policy)"
- "How is corpus search so slow? That's a bug to investigate and fix."
- The owner does not like the standalone `/autopaper/<story>` page.

## Evidence

Traces exported from the paper with `choir trace export` (bundles kept
outside the repo): the Apu story (trajectory `dbe27f57…`, ask
`rx-c4e206c940e3`, 04:46:58) and the France protest story (trajectory
`94dd4735…`, the owner's ask `rx-04797dbb765e`, 11:21:33). Paper console
read from the host, read-only.

### E1. Corpus search takes 30 to 90 seconds, and the story half is all of it

The paper's `corpus search` log lines for 2026-10-21 (272 searches):
`stories_ms` p50 32.0 s, p90 67.8 s, max 194.2 s; `captures_ms` is
0.2 to 1.2 s. 165 of the 272 searches (61%) returned no story at all.

In the two asks: France 53.5 s of research's 108 s first round; Apu 87.5 s
of research's 156 s, after a first attempt was killed by the 10 s cell
timeout (the host-side search kept running for 36 s and found nothing,
"captures not searched": its context was cancelled).

Cause, from code: `store.SearchDocuments` (`internal/store/texture.go`)
loads every Texture document object (2,752 on the paper today), matches
titles, and then, for every document whose title did not match, reads its
head revision one object at a time and checks that the content contains
every query term. Research queries carry five to eight terms, so the
all-terms match almost never hits, and the loop visits every story. It
is a full scan of 2,752 serial object reads per query, about 12 to 30 ms
each on a busy paper, with no index and no ranking. Several research
desks search at once (three searches ended together at 04:50:02), so
they also contend.

This was named and deferred on 2026-10-14
([corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md](corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
"`store.SearchDocuments` … scans every Texture document in the guest:
guest-side, not touched here"). The paper has grown since.

### E2. The button is only on the standalone page

`AutopaperExpand` is mounted only in `AutopaperRoute.svelte`'s header
(the public `/autopaper/<story>` page, from 2026-10-13, commit 7584664d2).
On the desktop a story opens as a Texture window (`AutopaperApp.svelte`
`launchapp`), which has no control. The owner reads in the desktop and
never saw the button.

### E3. The phone showed "Could not ask just now" for an ask the server accepted

Caddy logged the iPhone's `POST /api/platform/publications/expand` as 202
after 11.56 s; the proxy logged `outcome=asked`; the follow-up read
started about a second later and showed `asked:true`. The deployed bundle
returns that notice only when the status is not 202 with `asked`, or when
`fetch` throws. Cause unknown. Hypotheses: the phone's request failed on
its side after the server wrote the answer (11.6 s wait on 5G), or the
body read failed. The 11.6 s is itself three sequential calls to a busy
paper (document read, chain check, revise).

### E4. Texture spends a turn deciding to ask research

Every ask opens with a Texture turn that reads the story and casts
research: Apu 27 s (04:47:33 to 04:48:00, of which 11 s before the first
model call), France 30 s (one compile error in the cast packet, then a
retry). The reader already asked for research.

### E5. Research reports once, at the end

The research overlay says "send interim packets" and "work iteratively",
but in both traces research sends one packet, after every search and
read: Apu at 04:50:33 (2.5 min in), France round 1 at 11:24:02 and
round 2 at 11:25:55. Texture can do nothing until then.

### E6. Waits between desks

- Ask to Texture run: Apu 35 s (04:46:58 to 04:47:33), France 10 s.
- France: research's second report at 11:25:55, its run completed at
  11:27:17 (76 s after its last model call), and Texture's next model
  call was 11:30:04. About four minutes with no desk working on the
  story. The console shows many 1 to 1.8 minute `desk_go_eval` calls in
  that window (other stories' corpus searches) and no wake line for this
  story. Cause unknown; hypothesis: the run's completion write and the
  report's wake wait behind the scans in E1.

### E7. Texture asked for a second round

France: Texture judged the first report thin ("The first research pass
produced no readable, non-d…", 11:24:34) and cast again. Two of research's
reads failed (Guardian live page, a prefecture PDF). Reasonable judgment,
but it doubles the research time.

### E8. Malformed packets cost turns

Texture's cast and research's report each failed to compile once or
twice on map-literal syntax (France 11:22:08, 11:25:32, 11:25:43), five
to ten seconds each.

### Totals

Apu: ask 04:46:58, published 04:51:55, about 5 minutes. France: ask
11:21:33, v14 published 11:30:30, about 9 minutes. Model time is a small
share: research's model calls are 1 to 3 s each at low effort.

## Fix plan (each lands after this record)

1. Corpus story search (orange, E1): replace the scan with a ranked
   lexical index (BM25) over title and head text, kept in the guest
   runtime, built in the background at boot and updated when a revision
   commits; the search answers from titles until the index is built.
   Embeddings stay behind the 2026-10-14 ladder's criterion unless the
   owner overrides it.
2. The button in the Texture toolbar for published stories, and a
   responsive toolbar that wraps at phone width and large text (orange,
   E2). The standalone page's fate is the owner's call.
3. The ask goes straight to research, which reports to Texture (red,
   E4); research sends a packet as soon as it has a read source and
   finishes with the final report (prompt, E5).
4. E3, E6: instrument before fixing (client error kind on the notice,
   wake latency per report).

## Heresy delta

- discovered: E1 to E8.
- introduced: none.
- repaired: none.
