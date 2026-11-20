# Duplicate stories on the paper (2026-11-20, window 2026-11-19 22:18Z to 2026-11-20 22:18Z)

Read-only investigation of the owner's hypothesis of 2026-11-20 ("I think the
stories we may be doing duplicate stories of the same topics ... consolidating
overlapping stories ... will end up reducing searches"). Reads ran from about
22:17Z to 22:50Z on 2026-11-20 (`date -u` at the end 22:42Z for the last
read; the paper's console read at 22:3xZ). Nothing was changed on Node B or on
the paper. No secret, key or address is printed here. Counts below are mine
from the sources named in section 7; every figure marked "estimate" is one.

## 1. The answer in one paragraph

The hypothesis is true, and it is larger than the owner guessed on quality and
smaller than he hoped on searches. In the 24 hours ending 22:18Z the paper
opened 988 stories (982 after removing six test or placeholder documents),
and 458 of them are published. Reading them against everything opened in the
72 hours before each, **171 (17.4%) are a true duplicate of an earlier story
(same happening, same news), 26 more (2.6%) are follow-ups that add a real
development, and 41 more (4.2%) are borderline follow-ups**; so 20.1% are the
second or later story on a happening already covered (24.2% counting the
borderline ones), and 744 stories (76% of the 982) are first or distinct.
Duplicates are no less likely to publish than other stories (91 of 171 = 53%,
against 44% for distinct stories), so about one published story in five
(107 of 458 = 23%; 28% with the borderline ones) is a repeat a reader can
see on choir.news. Research spent on them: 528 of the window's 3,557 gateway
web searches (14.8%) went to duplicate stories and 86 more (2.4%) to
follow-ups, 614 together (17.3%); with the borderline follow-ups 736 (20.7%).
**Consolidation would cut the paper's searches by about 7% to 21% (estimate;
basis in section 3), most likely 10% to 15%; it cannot by itself bring 3,557
searches a day near 1,000.** The paper opens the second story because the
host does not check at all: the story-open path has no similarity check (the
only guard is a race check on the library head), and the one model that
decides open or feed (a management task, model `swe-2-flash`) is shown only
the 50 most recently updated stories, not the 8-hour window the prompt says
(142 of 142 `management story choice` lines in the paper's console, 21:11Z to
22:31Z, show `opening_shown=50`). The paper revises its stories so often that the
50 newest reach back a median 27 minutes (10th to 90th percentile 20 to 44, estimate from revision times) and the 200 newest a median 3.6 hours, while a median 383 stories were updated in the last 8 hours: of the 171 duplicates, the earlier
story was inside that 50 for **6 (3.5%)**, inside an 8-hour page capped at 200 stories for
68 (40%), and inside the intake labeller's 8-hour 480-story shortlist for 85
(50%); the earlier story's last update was over 8 hours old for 86 of 171
(50%). A search over held stories (story vectors, 72 hours) would have had
every pair in reach, but a vector alone is not a safe fold (precision 46% at
cosine 0.85 on this data), so it belongs in front of the judge, not in place
of it.

## 2. How many stories, how many duplicates (question 1)

**Window and population.** Stories whose Texture document was created from
2026-11-19T22:18Z to 2026-11-20T22:18Z (`GET /api/texture/documents`, paged
by cursor, created_at). **988 stories (41 an hour).** Six are test or
placeholder documents ("Test headline", "Test", "Headline", "Probe"; they drew
27 searches) and are excluded below: **982 stories**. 458 of the 988 are
published (a publication object exists for the document, read from corpusd's
graph database at about 22:25Z); 530 are not. Each was compared with every
story created in the 72 hours before it (the earlier 3,000 documents, created
from 2026-11-16T22:18Z). Duplicates older than 72 hours are not counted.

| Kind | Stories | Share of 982 | Published | Mean web searches a story (since opening) |
| --- | --- | --- | --- | --- |
| (a) true duplicate: same happening, same news | 171 | 17.4% | 91 (53%) | 3.1 |
| (b) follow-up with a real development, clear | 26 | 2.6% | 16 (62%) | 3.3 |
| (b) follow-up, borderline (marked `b`) | 41 | 4.2% | 23 (56%) | 3.0 |
| (c) distinct, or first on its happening | 744 | 75.8% | 328 (44%) | 3.4 |

- **Second or later on a happening covered in the previous 72 hours:
  197 of 982 = 20.1% (a plus clear b); 238 = 24.2% with the borderline
  follow-ups.** Among published stories: 107 of 458 = 23.4% (130 = 28.4%).
- **Clusters.** Linking each later story to the earlier story it repeats
  (and merging through shared partners): 160 clusters of two or more stories
  (strict), sizes counting the earlier partners that were opened before the
  window: 2 stories 135, 3 stories 17, 4 stories 5, 5 stories 2, 6 stories 1;
  the other 730 clusters (of 890 touched by a window story) are single
  stories. Counting only window stories: 823 singletons, 48 pairs, 15
  triples, 3 fours, 1 six. With the borderline follow-ups: 185 clusters of two or
  more (sizes 2:150, 3:24, 4:7, 5:2, 6:1, 7:1). These sizes are a lower
  bound: I linked each later story to one earlier story, so two earlier
  stories on the same happening that I never compared with each other stay
  separate (the Pew projection has at least five stories; the Poland false-flag
  plot at least six).
- **Gap to the earlier story.** For the 171 duplicates, the gap to the most
  recent earlier story on the same happening has median 11.8 hours (25th to
  75th percentile 3.0 to 35.9; 90th percentile 51.2). 44 are within 3 hours, 71
  within 8, 114 within 24, 146 within 48, 25 beyond 48. For the 26 clear
  follow-ups the median is 26.6 hours (none within 3). The earlier story was
  already published when the second was opened in 105 of 171 duplicates (61%).
- **Largest clusters** (strict): the overnight Russian strike on Ukraine
  (6 stories in 19 hours, 79 searches; `a6c6229c`, `78f154ae`, `f55fd0bb`,
  `0ddc9249`, `344ccb67`, `dbee98f6`), the "Ruby2Shell" Ruby flaw (5 stories,
  all unpublished), the Senate permitting bill passing (5), the German
  conscription protests (4), Pas-de-Calais floods (4), Musk's "pretty foolish"
  remark (4), a Las Vegas teen shooting (4), Trump's arch blocked by judges
  (4).
- **What the 744 distinct stories include.** Look-alikes I did not count as
  duplicates: the Philippine charge against Thomas Markle and the judge ruling
  him fit for trial (different events), the third US carrier arriving and the
  USS George Washington leaving, successive ransomware victim claims,
  successive earthquakes, successive Houthi strikes on Riyadh and Gulf drone
  attacks (different strikes of one war).

### 2.1 How reliable is the clustering

One reader (me, a model), titles plus first paragraphs, no second human. Three
checks, all weak in the same way (same reader):

1. **Candidate generation.** Each window story was matched to earlier stories
   by title-and-first-paragraph TF-IDF cosine (at least 0.20) or by story
   vector cosine (at least 0.80) and I read the 467 stories with a candidate.
   The other 521 had no earlier story above those scores. I read a random 60 of
   those 521 with their best candidate: **0 of 60 were duplicates** (95%
   upper bound on the miss rate 5%, so at most about 26 more duplicates, an
   estimate; cross-language pairs with no shared words are the likeliest miss).
2. **Re-judging 20 random pairs** (8 duplicates, 5 follow-ups, 7 distinct,
   shuffled, labels hidden, now with both first paragraphs): **18 of 20 same
   kind, 20 of 20 agree on duplicate-or-follow-up against distinct.** The two
   differences are a clear-versus-borderline follow-up (NASA's Moon plan) and
   follow-up versus duplicate (the Italian CGIL strike called, then marched).
   It was not blind in the strict sense (I had read the titles earlier).
3. **Borderline marking.** 41 follow-ups and 5 duplicates I flagged `b`; the
   headline share is 20.1% without the `b` follow-ups and 24.2% with them.

### 2.2 Examples (question 1)

Format: row number in my reading (its id is the first eight hex of the
document id), kind, earlier story (UTC created, id, public URL if
published), later story, hours between, and web searches / research runs the
later story has cost since it opened. "unpublished" stories have no public
URL; their document ids are given. A path `/pub/texture/...` is under
`https://choir.news`.

| # | Kind | Earlier story | Later story | Gap h | Later: searches / runs |
| --- | --- | --- | --- | --- | --- |
| 258 | D | Six killed in suspected gas explosion at Georgia apartment building (11-19 23:43Z, 34a12f07; https://choir.news/pub/texture/six-killed-in-suspected-gas-explosion-at-georgia-apartment-building-pub40bd5f446) | Six killed in suspected gas explosion at Georgia apartment building (11-20 05:09Z, 6e03de7e; unpublished (doc 6e03de7e)) | 5.4 | 0 / 0 |
| 144 | D | Michigan home health agency owner sentenced to 4 years for $1.6M Medicare fraud scheme (11-19 21:18Z, c6cd6aa5; unpublished (doc c6cd6aa5)) | Michigan home health agency owner sentenced to 4 years for $1.6M Medicare fraud scheme (11-20 01:47Z, 7300cfb3; unpublished (doc 7300cfb3)) | 4.5 | 0 / 0 |
| 18 | D | Islam projected to become the world's largest religion by 2035, Pew forecasts (11-19 02:38Z, f18b9050; https://choir.news/pub/texture/islam-projected-to-become-the-worlds-largest-religion-by-2035-pew-forecasts-pubbbec12f3f) | Pew projects Islam will overtake Christianity as the world's largest religion by 2035 (11-19 22:47Z, bc908433; https://choir.news/pub/texture/pew-projects-islam-will-overtake-christianity-as-the-worlds-largest-religion-by-2035-pub51c84394b) | 20.2 | 2 / 1 |
| 1 | D | Paris 2024's economic legacy was 'very limited', France's audit court finds (11-18 19:49Z, 3b211976; unpublished (doc 3b211976)) | French audit finds Paris 2024 had very limited short-term economic impact (11-19 22:23Z, 135f21c4; unpublished (doc 135f21c4)) | 26.6 | 0 / 0 |
| 49 | D | UK sanctions eight Israeli settlement firms; Australia joins, Israel vows countermeasures (11-17 23:52Z, 00cd2e2c; https://choir.news/pub/texture/uk-sanctions-eight-israeli-settlement-firms-australia-joins-israel-vows-countermeasures-pubd18049757) | UK sanctions eight Israeli firms building West Bank settlements; Sa'ar vows countermeasure (11-19 23:35Z, d432666f; https://choir.news/pub/texture/uk-sanctions-eight-israeli-firms-building-west-bank-settlements-warns-against-e1-pub0d03f8a58) | 47.7 | 14 / 2 |
| 532 | D | One company wins rights to mine off American Samoa at first U.S. deep-sea mining lease auc (11-18 19:04Z, 932674e6; https://choir.news/pub/texture/trump-administration-schedules-first-u-s-deep-sea-mining-lease-auction-off-american-samoa-pub74d9b1eba) | US sells first deep-sea mining leases off American Samoa for $6m, days after court voided  (11-20 11:57Z, 202c1118; https://choir.news/pub/texture/us-sells-first-deep-sea-mining-leases-off-american-samoa-despite-court-ruling-and-local-oppositi-pub73c89dec8) | 40.9 | 69 / 1 |
| 225 | D | Russia launches mass missile strike on Ukraine after Putin vows revenge (11-20 00:46Z, a6c6229c; https://choir.news/pub/texture/russia-launches-mass-missile-strike-on-ukraine-after-putin-vows-revenge-pub5a2f8d744) | Russia launches massive missile and drone attack on Ukraine overnight (11-20 04:17Z, 78f154ae; https://choir.news/pub/texture/russia-launches-massive-missile-and-drone-attack-on-ukraine-overnight-pub2864e2484) | 3.5 | 43 / 1 |
| 158 | D | UK sanctions eight Israeli settlement firms; Australia joins, Israel vows countermeasures (11-17 23:52Z, 00cd2e2c; https://choir.news/pub/texture/uk-sanctions-eight-israeli-settlement-firms-australia-joins-israel-vows-countermeasures-pubd18049757) | Australia sanctions settler leader Daniella Weiss and a construction company over West Ban (11-20 02:15Z, b7d3bf3e; https://choir.news/pub/texture/australia-sanctions-settler-leader-daniella-weiss-and-a-construction-company-over-west-bank-sett-pub320eaa3e8) | 50.4 | 33 / 1 |
| 806 | D | Indian national Nikhil Gupta sentenced to 19 years 7 months for plot to kill Sikh leader P (11-20 17:16Z, 06516652; https://choir.news/pub/texture/indian-national-nikhil-gupta-sentenced-to-19-years-7-months-for-plot-to-kill-sikh-leader-pannun-pub83eb90ff2) | Indian man sentenced to 19½ years for plot to kill Sikh separatist leader in New York (11-20 18:32Z, f96cf1fe; https://choir.news/pub/texture/indian-man-sentenced-to-19½-years-for-plot-to-kill-sikh-separatist-leader-in-new-york-pub8d1145e6d) | 1.3 | 26 / 1 |
| 491 | D | Five aid groups grade Trump's Gaza peace plan 4 out of 26 and call it 'displacement by des (11-19 10:58Z, a13b9fc8; https://choir.news/pub/texture/five-aid-groups-grade-trumps-gaza-peace-plan-4-out-of-26-and-call-it-displacement-by-design-pub45cf4503f) | Humanitarian groups give Trump's Gaza 'peace' plan 4 out of 26 one year on (11-20 10:45Z, 79a23da8; https://choir.news/pub/texture/humanitarian-groups-give-trumps-gaza-peace-plan-4-out-of-26-one-year-on-pubcf1c4a56d) | 23.8 | 18 / 1 |
| 515 | D | ICJ to deliver verdict on Thursday in historic genocide case against Myanmar over Rohingya (11-18 23:21Z, 5f74419a; https://choir.news/pub/texture/icj-to-deliver-verdict-on-thursday-in-historic-genocide-case-against-myanmar-over-rohingya-pubf0b734358) | ICJ finds Myanmar committed genocide against Rohingya in historic ruling (11-20 11:20Z, f389a9ce; https://choir.news/pub/texture/icj-finds-myanmar-committed-genocide-against-rohingya-in-historic-ruling-pubf070c7a2f) | 36.0 | 2 / 1 |
| 674 | D | Matan Kahana elected Knesset speaker, giving Israel's opposition bloc control of parliamen (11-19 03:42Z, 01b9e91a; https://choir.news/pub/texture/matan-kahana-elected-knesset-speaker-giving-israels-opposition-bloc-control-of-parliament-pub65b183efd) | Matan Kahana elected speaker of Israel's Knesset as opposition bloc takes control (11-20 15:52Z, 3c72d714; https://choir.news/pub/texture/matan-kahana-elected-speaker-of-israels-knesset-as-opposition-bloc-takes-control-pub22738ecef) | 36.2 | 2 / 1 |
| 148 | D | Germ DM, a Signal alternative built on “burner cards,” is reported now on Android (11-18 11:12Z, 20f8c146; https://choir.news/pub/texture/germ-dm-a-signal-alternative-built-on-burner-cards-is-reported-now-on-android-pub74e28ee21) | Germ DM launches on Android: Bluesky's encrypted messenger offers phone-number-free "cards (11-20 02:03Z, e45d435c; unpublished (doc e45d435c)) | 38.9 | 17 / 1 |
| 552 | D | Ukraine confirms strikes on three oil tanks at Tatarstan pipeline station on Druzhba netwo (11-20 11:19Z, 2391941d; unpublished (doc 2391941d)) | Ukraine strikes Kaleykino Druzhba pipeline hub in Tatarstan, hitting three oil tanks (11-20 12:24Z, faffd320; https://choir.news/pub/texture/ukraine-strikes-kaleykino-druzhba-pipeline-hub-in-tatarstan-hitting-three-oil-tanks-pub6356fcae5) | 1.1 | 16 / 1 |
| 343 | D | US loses measles-free status after 26 years as PAHO confirms endemic transmission (11-18 01:11Z, 77dd0944; https://choir.news/pub/texture/us-loses-measles-free-status-after-26-years-as-paho-confirms-endemic-transmission-pub4340d797c) | United States loses measles-free status as Germany's former health minister warns of a vac (11-20 07:25Z, dc47e897; https://choir.news/pub/texture/united-states-loses-measles-free-status-as-germanys-former-health-minister-warns-of-a-vaccinatio-pub79cd40fef) | 54.2 | 15 / 1 |
| 167 | D | Russia reportedly planned a mass-casualty false-flag attack in Poland to be blamed on Ukra (11-19 21:13Z, e701b034; unpublished (doc e701b034)) | Russia reportedly planned mass-casualty sabotage in Poland to be blamed on Ukraine (11-20 02:41Z, 775f0093; unpublished (doc 775f0093)) | 5.5 | 0 / 0 |
| 337 | D | Critical pre-authentication RCE in Ruby disclosed as 'Ruby2Shell', fixes released (11-20 03:43Z, b905efb2; unpublished (doc b905efb2)) | Critical Ruby string-conversion bug allows pre-authentication RCE on stock Rails, Veria La (11-20 07:17Z, b47973dc; unpublished (doc b47973dc)) | 3.6 | 0 / 0 |
| 160 | F | Political scientist Alexander Kynev arrested; says his election analysis led to it (11-18 08:08Z, 7e3fcbd7; unpublished (doc 7e3fcbd7)) | Moscow extends detention of election expert Alexander Kynev to December 24 (11-20 02:24Z, 22f07375; https://choir.news/pub/texture/moscow-extends-detention-of-election-expert-alexander-kynev-to-december-24-pub4018cea5b) | 42.3 | 17 / 1 |
| 228 | F | India, Japan set to review defence ties at 2+2 in Tokyo (11-18 07:31Z, 323c7453; https://choir.news/pub/texture/india-japan-set-to-review-defence-ties-at-22-in-tokyo-pub71ae23c59) | Japan and India agree to expand defense cooperation, including SDF live-fire missile train (11-20 04:21Z, 710c765d; unpublished (doc 710c765d)) | 44.8 | 15 / 1 |
| 197 | F | Senate clears way for Senate passage of 400-page permitting overhaul; House is the hard te (11-18 11:54Z, 537e6d5f; unpublished (doc 537e6d5f)) | Senate passes major permitting reform bill 62-33, sends it to the House (11-20 03:41Z, 08cc8722; unpublished (doc 08cc8722)) | 39.8 | 0 / 0 |
| 163 | F | Nvidia's Colette Kress passes her first test as interim CEO, but the board keeps Jensen Hu (11-18 03:49Z, e914139e; https://choir.news/pub/texture/nvidias-colette-kress-passes-her-first-test-as-interim-ceo-but-the-board-keeps-jensen-huang-out-pub2c46a92c2) | Nvidia confirms Jensen Huang resigned as CEO after 33 years (11-20 02:28Z, 572a0598; https://choir.news/pub/texture/nvidia-confirms-jensen-huang-resigned-as-ceo-after-33-years-pubf7bd1df9a) | 46.6 | 3 / 1 |
| 546 | F | Nations League draw set for Friday — England, Spain and France await quarter-final opponen (11-20 09:11Z, b2e3d7fa; https://choir.news/pub/texture/nations-league-draw-set-for-friday-england-spain-and-france-await-quarter-final-opponents-pub810f25837) | England drawn against Norway, Spain to face France in Nations League quarter-finals (11-20 12:16Z, 482fd1e1; unpublished (doc 482fd1e1)) | 3.1 | 0 / 0 |
| 954 | F | French Senate delays vote on police weapon-use presumption to Friday (11-20 01:29Z, a24e0a5e; unpublished (doc a24e0a5e)) | French Parliament approves a legal presumption for police use of firearms, ahead of a new  (11-20 21:29Z, 140fdb0b; unpublished (doc 140fdb0b)) | 20.0 | 0 / 0 |
| 255 | F | Bukele due to register for unprecedented third El Salvador presidential term (11-20 00:35Z, aa6415cc; https://choir.news/pub/texture/bukele-due-to-register-for-unprecedented-third-el-salvador-presidential-term-pubf9bec58e5) | Bukele registers for an unprecedented third term in El Salvador (11-20 05:03Z, 8b6f1da2; https://choir.news/pub/texture/bukele-registers-for-an-unprecedented-third-term-in-el-salvador-pub959e513f4) | 4.5 | 3 / 1 |
| 123 | F | Eight killed in Brooklyn building collapse; city orders demolition of neighboring damaged  (11-19 05:15Z, e867c7cd; https://choir.news/pub/texture/eight-killed-in-brooklyn-building-collapse-city-orders-demolition-of-neighboring-damaged-buildin-pub512e04317) | New video captures deadly Brooklyn building collapse that killed eight (11-20 01:21Z, c852e275; https://choir.news/pub/texture/new-video-captures-deadly-brooklyn-building-collapse-that-killed-eight-pub5bb7352c1) | 20.1 | 0 / 0 |

Three look-alikes counted as distinct, so the owner can see the line I drew:
(28) the Philippine charge against Thomas Markle (11-17 23:21Z) against the
US judge finding him fit for trial (11-19 23:12Z); (76) the third carrier
reaching the Middle East against the USS George Washington leaving; (3)
(borderline `b`, counted only in the broad numbers) South Korea's AI cheating
crackdown stories against the nationwide disaster alerts on exam day.
Identical-title pairs are the plainest cases: "Six killed in suspected gas
explosion at Georgia apartment building" was opened twice, 5.4 hours apart,
the first published and the second not; "Michigan home health agency owner
sentenced to 4 years for $1.6M Medicare fraud scheme" twice, 4.5 hours apart,
neither published. A pair may also be a larger cluster: the
overnight strike on Ukraine has six stories.

## 3. What the later stories cost (question 2)

Sources: the gateway journal's `search request from autoputer <paper>`
lines (`served=miss`, one line per real web search; 3,557 of them from
2026-11-19T22:18Z to 2026-11-20T22:18Z, all `desk=research`), joined to the
story by `conv=` (equal to the story's document id; 3,419 of the 3,557 join to
a story created since 11-16 22:18Z), and by `run=` to the research run; the
run API for the run's token totals.

| Kind of later story | Searches in the window | Share of 3,557 | Research runs that searched | Mean input / output tokens a run |
| --- | --- | --- | --- | --- |
| duplicates (a) | 528 | 14.8% | 97 | 1.01M / 22k (n = 97) |
| clear follow-ups (b) | 86 | 2.4% | 18 | 0.83M / 30k (n = 18) |
| borderline follow-ups | 122 | 3.4% | 30 | 1.03M / 21k (n = 30) |
| distinct stories | 2,491 | 70.0% | 462 | 1.07M / 22k (n = 160 sampled of 462) |
| test or placeholder documents | 27 | 0.8% | 5 | |
| stories opened before the window | 303 | 8.5% | | |

- **Share of the day's searches on duplicates and follow-ups: 17.3% (614) for
  (a) plus clear (b); 20.7% (736) with the borderline ones; 14.8% for
  duplicates alone.** Runs: 115 of 612 searching runs (18.8%). Texture
  revisions (model turns of the story's desk, from the revision counts):
  875 of 4,070 (21.5%). A duplicate's run costs the same as a distinct story's
  (about 1.0M input tokens including cache, 22k output); the journal has no
  `run=` on inference lines, so model calls are not joinable per story and
  tokens from the run API stand in.
- **Concentration.** The ten costliest duplicates and follow-ups carry 44% of
  their searches: the US deep-sea mining lease sale (69 searches, `532`), the
  overnight strike on Ukraine (43 and 31), the Australian sanctions (33), the
  Gupta sentencing (26), the humanitarian groups grading Trump's Gaza plan
  (18), Kynev's detention (17).
- **Estimate of the searches consolidation would remove, with its basis:**
  *low* about 7% (half of the duplicates' 528, because a story that is fed a
  new capture instead of opened still gets some research on the new
  material); *high* about 21% (all 736, if folding removed every search of
  every duplicate and follow-up, which it will not: a follow-up needs research on
  its new development). Most likely 10% to 15%. Of 3,557 a day that leaves 2,800
  to 3,200 searches, against the owner's 1,000; consolidation is a quality fix
  with a modest search saving, and the first-run searches of distinct stories
  (the earlier finding: 90% of research runs are a story's first run) stay the
  main cost. Not measured here: how much of a fed story's research would be
  cut by `choir.Feed` (the feed path exists and was taken 73 times of 142 in
  the console window).

## 4. Why the paper opens a second story (question 3)

### 4.1 The path, from the code (commit 19519df2b)

1. Captures arrive from source polls, social feeds and research reads. The
   intake labeller (Jev, one call a flush) labels each polled capture
   against a shortlist: stories updated in the last 8 hours, newest first,
   at most 480 and at least the newest 30 (`internal/agentcore/intake_labels.go:1378,
   1381,1385`; `intakeShortlistSplit` :172), plus up to ten older stories of
   the 72-hour horizon that share title words (`intakeOverlapPicks` :207,
   horizon :129). The label is "same event as texture:<doc>", "new" or "new,
   same topic as ...", and it is advice only ("labels never gate arrival",
   file header). A capture a research desk reads on the web is not one a poll
   labels (a traced run says "unlabelled by Jev", section 4.3).
2. Management delegates the captures to a **management task** (a sub-run, model
   `swe-2-flash`, reasoning low in the traced runs). The task is the desk that
   decides "open, feed or decline every one of them": its prompt says "Feed the
   story Jev names unless you read otherwise" and "Open a story for a new
   event ... One event, one story, across languages"
   (`internal/runtimeprompts/overlays/rlm_management_task_runtime.yaml`;
   `management_tasks.go:68`).
3. **What it is shown, and the first look the host builds**:
   `managementTaskOpening` (`management_tasks.go:107`) appends the task's
   records and a story page to the first user turn. The prompt promises the
   page as "the last 8 hours in full" (:68; window constant :153,
   `storiesWindow = 8 * time.Hour`). The page is built by `storiesOpeningPage`
   (:157): the window page, but only when it is at most
   `managementTaskOpeningSectionTokens` = 20,000 estimated tokens (:73; four
   characters a token, `run_memory.go:1545`); otherwise it falls back to the
   default page, which is **the newest 50 stories** (`storiesDefaultLimit = 50`,
   `intake_view.go:23`). The window page itself is capped at 200 stories
   (`storiesMaxLimit = 200`, `intake_view.go:24`, :142), each with a headline
   and a lede of up to 400 characters (`store/stories.go:36`): 200 stories are
   about 30,000 tokens (estimate, 150 tokens a story), over the bound, and a
   median 383 stories are updated in 8 hours. So the page is always the newest
   50. **Observed: 142 of 142 `management story choice` console lines (21:11Z to
   22:31Z) carry `opening_shown=50`** (`management_story_choice.go:77`, logged
   by `logManagementStoryChoice` :83). In that window the console logged `choice=open` 69
   times and `choice=feed` 73 times (log lines, one per committed record, so a
   record is not always a distinct story).
4. **What the host checks when the task casts `texture:new`: only a race.**
   `checkStoryLibrary` (`internal/store/management_tasks.go:656`) refuses the
   open if the library head moved since the run last read the page and lists
   the stories opened meanwhile (`ErrStoryLibraryMoved` :693); it compares no
   headline, capture, event or vector. `planStoryOpenings`
   (`internal/store/story_open.go`) then creates the document; no step looks
   for an existing story on the same event. The event layer (`event_layer.go`)
   decides capture-to-event, and "no code links a story to an event" (earlier
   finding, not re-read here).

### 4.2 Would the earlier story have been found

For the 171 duplicates, at the moment the later story was opened (revision
times from Texture history stand in for the story index's update time;
estimate, the page is read about 1 to 3 minutes before the commit):

| What reaches the earlier story | Duplicates (of 171) | Clear follow-ups (of 26) |
| --- | --- | --- |
| management's page as it is (newest 50 by update) | 6 (3.5%) | 0 |
| an 8-hour page capped at 200 (what the prompt promises) | 68 (39.8%) | 5 |
| the intake labeller's shortlist (8 hours, 480, floor 30) | 85 (49.7%) | 6 |
| any search over the last 72 hours | 171 (by construction; my lookback is 72 h) | 26 |
| the earlier story's last update older than 8 hours | 86 (50.3%) | 20 |

A search of held stories by story vector (corpusd `vec_story_passages`, one
vector set per published version) can only reach **published** stories: the
earlier story was published in 105 of 171 duplicates before the later one
opened (61%), and a published story's vector exists after the first version
publishes (the first version publishes a median 33 minutes after opening, 25th to 75th percentile 24 to 46 minutes, n = 458). On the pairs where
both stories are published (post-hoc, using the later story's own vector as a
stand-in for what the opening capture's vector would find, which is
weaker), the story-to-story cosine against 72 hours of earlier published
stories gives:

| cosine at least | candidate pairs (published) | same happening (a or b, broad) | precision | published stories with a published earlier mate that have it in the list |
| --- | --- | --- | --- | --- |
| 0.80 | 565 | 126 | 0.22 | 100 of 100 (biased: I generated candidates from this score) |
| 0.85 | 236 | 109 | 0.46 | 93 of 100 |
| 0.88 | 147 | 86 | 0.59 | 79 of 100 |
| 0.90 | 101 | 64 | 0.63 | 61 of 100 |
| 0.92 | 58 | 41 | 0.71 | 40 of 100 |
| 0.94 | 24 | 19 | 0.79 | 19 of 100 |

So story vectors find nearly every published pair at 0.85 but with 54% wrong
guesses; no threshold folds safely without a judge. The entity layer and the
event ids were not testable here: events carry no story id, and
`GET /api/autopaper/events` returns only the 300 most recently touched events
(about 47 minutes at 22:38Z). I did not read the entity layer that landed
today.

### 4.3 Six traced openings (management task runs, run API, run events)

Each is a duplicate (kind a) opened inside the console window, so the run is
readable. The first user turn (the page and labels) is not stored in the run
events, so what the model saw is inferred from the host's page size and from
its cells. "Rank" is the earlier story's rank among stories by last update at
the run's start (estimate); "looked" is whether the run's code called
`choir.Stories(...)` or printed labels.

| Run (start) | Later story | Earlier story, when, published | Earlier's rank and age at run start | Looked | What the run did |
| --- | --- | --- | --- | --- | --- |
| `76f87579` (21:48:56Z) | "Poland says Russian missiles and a drone violated its airspace" (`36870dbf`, opened 21:51:53) | "Russian missiles and drone land in Poland during strike on Ukraine" (`c241c482`, 19:48, published 20:17Z) and "Poland's PM calls ... 'intentional provocation'" (`4ee73f79`, 10:16) | rank 99 (1.0 h old) and 189 | no `choir.Stories`, no labels in 14 cells | opened one story from one Mastodon capture of an AP post, after declining 11 others; the first cast was refused by the library-head race check (a Bolsonaro story opened meanwhile) and it recast the same story without re-reading the page. Story-vector cosine to the earlier two: 0.944 and 0.913. 14 cells, 16 model calls, 307k input tokens |
| `25061806` (21:38:03Z) | "Trump Threatens 'Tremendous Tax' on US Tech Firms That Build AI Data Centres Abroad" (`eb6a08b4`, 21:39:03) | "Trump threatens 'tremendous tax' on U.S. companies building data centers overseas, with no details yet" (`2fa155ce`, 18:33, not published) | rank 50 (0.3 h old), at the edge of the page | no | opened it with p_success 0.3, a near-identical headline; refused once by the race check, recast. This story has then cost 8+ searches since (first query: a Finland data-centre tax comment) |
| `414bc52e` (21:45:55Z) | "Second federal judge blocks Trump's memorial arch" (`8516296b`, 21:47:19) | three earlier arch stories, the nearest "Second judge blocks Trump's triumphal arch, calling it a 'rushed job'" (`d6ee39e4`, 19:48, unpublished) | ranks 146, 187 and 308 (1.8 to 4.6 h old) | yes: called `choir.Stories(50, "")` after a race refusal | opened a new story and fed two other captures to existing stories (so it does feed when it sees a match) |
| `df323509` (22:04:06Z) | "Trump holds White House press conference after Asia trip, hails Stephen Miller" (`02af6105`, 22:04:44) | "Trump calls chief of staff Stephen Miller 'the most powerful man in the world'" (`f13f9f38`, 19:52, published 20:18Z) | rank 14 (0.2 h), inside the 50 | no calls | its note: "a distinct event, unlabelled by Jev, despite existing Miller/Asia-adjacent stories already covering fragments"; so the page showed the earlier story and the desk judged it a different event. I count the pair a duplicate; a reader could call it a follow-up. The event layer minted a one-member event "The latest from Trump's press conference" (NPR, 21:58:32Z) for the same capture |
| `00fa2fb4` (22:07:49Z) | "Google redesigns search page: no search button, new AI sidebar" (`3914c08a`, 22:13:02) | "Google adds an AI chat sidebar to its desktop search homepage" (`f335f2f8`, 11-19 20:17, published 21:25Z) | rank 1109 (24.7 h old), out of any window | yes: `choir.Stories(10, "")`, then `(30, "")` | opened; the page it read (newest 10, 30) could not hold a story 25 hours old |
| `c82a457c` (22:16:01Z) | "Lufthansa to cut 1,000 jobs" (`2d5832c2`, 22:16:59) and "Princeton PhD student Eva Xie detained by ICE" (`077891ef`) | "Lufthansa to cut jobs as it struggles to turn a profit" (`f7b31aa4`, 06:51, unpublished); "Princeton PhD student Eva Yi Xie detained by ICE after neuroscience conference" (`a6b3fb74`, 18:51, published 19:15Z) | rank 714 (14.8 h) and 183 (2.8 h) | no | fed 2 captures to existing stories, opened 3 new ones, declined 7; the Lufthansa earlier story was 14.8 h old, the Princeton one was 2.8 h old and outside the 50 |

Reading: in 2 of the 6 (`df323509`, `25061806` at the edge) the earlier story
was in the page the task was given and the task still opened; in 3 (`76f87579`,
`414bc52e`, `c82a457c`) the earlier story was 99 to 308 positions down, outside
the 50; in 1 (`00fa2fb4`) it was a day old. Not determinable here: whether the
intake label for each opening capture named the earlier story (labels are not
in the graph database, the run events or the console window for these runs), and
what the model thought (its text is empty; only code cells are recorded).

Problem record: the same fault was recorded on 2026-11-10
([unsupported-cites-duplicates-and-churn-2026-11-10.md](../problems/unsupported-cites-duplicates-and-churn-2026-11-10.md),
Q2, "Windows are counts"): eight duplicate groups in 749 stories, pairs 40
minutes to 6.5 hours apart, fix D2 "windows by time, not count", which landed
for the intake shortlist (the 8-hour, 480 window) but, as read above, does
not reach management's page. My count is by reading, not by title-word
groups, so 8 of 749 and 197 of 982 are not comparable measures.

## 5. What would consolidate (question 4)

Options for the lead to take to the owner; not designs. Each says where it
sits, what it would have done to this data, what it risks, and what sets its
threshold. The labelled pairs (appendix) are a ready test set.

| # | Option | Where in the code | Cost | What it does to the 197 pairs | Risk | Threshold from |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Make management's page the window it promises: headlines only (ids and one-line headline), 8 hours, so it fits the 20,000-token bound | `management_tasks.go:73,153-170`; `intake_view.go:24,142` | no new model call; about 200 headlines is roughly 5,000 to 6,000 tokens (estimate, 30 tokens a headline) | page reach rises from 6 to 68 of 171 duplicates (40%), 5 of 26 follow-ups; half the duplicates are older than 8 hours so it cannot reach them | the run's code called `choir.Stories` in only 2 of 6 traced runs (the page came with the first message, so that does not show it ignored it); a longer list may be used no better than the short one; no effect on follow-ups | share of duplicates whose earlier story is on the page (measured now: 3.5%), and feed share of decisions (now 73 of 142, 51%) |
| 2 | Retrieve, do not list: at story-open (and in the intake label's shortlist) offer the top candidates from the last 72 hours by story vector, event overlap, entities and title words, for the judge to choose, and make `texture:new` refuse when a held story is a candidate the run has not answered | `intake_labels.go` shortlist build (:172-:230); `checkStoryLibrary` (`store/management_tasks.go:656`, which already refuses with a story list) ; corpusd `vec_story_passages`, captures' vectors | same call (the intake call already takes up to 255 choices), more options | all 197 are within 72 hours; vectors reach 93 of 100 published pairs at cosine 0.85 | folding two events of one war (precision 0.46 at 0.85, 0.71 at 0.92 on this data), and vectors exist only for published stories (61% of earlier partners), so unpublished earlier stories (39%) need the title and entity route | precision and recall curve per signal on the 197 labelled pairs against the 744 distinct; accept a list size that holds at least 90% of duplicates; refusal only when the judge's p passes a bar set on the false-fold rate |
| 3 | A capture that joins the event of an existing story feeds that story (a new version) instead of waking an open | needs a story-to-event link that does not exist (earlier finding); `event_layer.go` (`resolveEvents` :346, join p 0.92 at :78, line numbers from the hand-read record, not re-read); `choir.Feed` exists | an orange change; no extra call | covers only duplicates whose capture shares an event with a capture the earlier story holds; the event layer's join rate was 2.8% joined and 84% minted over 03:28Z to 05:12Z on 2026-11-20 and a hand read found 10 of 50 events should have joined ([event-join-rate-hand-read-2026-11-20.md](event-join-rate-hand-read-2026-11-20.md)), so it would reach few today; the events route cannot be read back far enough to test | event false joins; a follow-up has its own event ("follows" is already a field on events), which suits (b): a follow-up event could feed the story holding the event it follows | story-event link coverage; join precision from the event hand read (false merges at or under the event-wakes bar) |
| 4 | Route follow-ups as new versions: when the opener is a follow-up (a development of a story already held), cast the held story's desk with the new capture instead of `texture:new` | same decision point as 1 and 2; Texture's version path | no new call | the 26 clear follow-ups (and up to 41 more); follow-ups sit 12.7 hours or more behind the earlier story (median 26.6 h), mostly beyond the 8-hour page | starves a real follow-up that deserves its own story (a new fact, a new headline: Huang resigned, the Senate passed the bill); the earlier story's Texture desk must accept a version that changes its headline | 20-pair blind re-judgment of follow-up against duplicate (agreement 18 of 20 here); hold-out set of follow-ups the owner would keep as stories |
| 5 | Merge already-published duplicates (supersede or redirect to the first) | the `superseded_by` column exists on graph objects (`og_objects`); publication routes `/pub/texture/...`; front page | a periodic job over the last 72 hours, one judge call per cluster | removes the 91 published duplicates (20% of 458) from the reader's view; saves no research already spent | wrongly merged distinct stories (precision at 0.92 is 0.71 on vectors alone); a soft pointer is reversible | judge p plus vector at 0.92 and same entities; false-merge bar from a 50-pair hand read |
| 6 | Platform publish-time check of the first version against the last 72 hours of published stories (design D3 of the 2026-11-10 record; I did not check whether any of it was built) | platform publication path | refusal returned to the computer as a record | stops 91 published duplicates after the research has been spent, so no search saving | false refusals of real follow-ups | same pairs |

Order by what it would save: options 1 and 2 act before research (the 7% to
21% search saving) and are the ones that touch the open decision; 3 and 4
are the same decision made from the event and from the follow-up
relation; 5 and 6 repair the reader's page and cannot save searches. The
owner's rule on model calls (no extra Jev call per capture) is met by 1 to 3:
all ride the calls that exist.

## 6. Also found

- Six test or placeholder documents ("Test headline", "Test", "Headline",
  "Probe") opened as stories in the window and drew 27 searches in five
  research runs; one was opened as late as 18:11Z.
- 46% of the day's stories (530 of 988) never publish; duplicates publish
  more often than distinct stories (53% against 44%), so publication does not
  filter them.
- The code comment and the prompt say management's page is "the last 8
  hours in full"; the host delivers the newest 50 in every observed run.
  That is a defect in a deployed path that the 2026-11-10 fix was meant to
  close, so a problem record is due (lead's call).

## 7. Method

- **Stories**: `GET /api/texture/documents?limit=500` with the paper's read
  key, paged by `next_cursor` until updated_at passed 2026-11-16T22:00Z
  (4,500 documents; 3,876 created in 2026-11-16T22:18Z to 2026-11-20T22:18Z;
  window stories 988). Text: `GET /api/texture/documents/{id}/export` (current
  revision, markdown; 43 just-opened documents had empty text). Revision times:
  `GET /api/texture/documents/{id}/history` for all 3,876.
- **Published, routes, story vectors**: corpusd's `graph` database on Node B
  (`set default_transaction_read_only=on`): `og_objects` kinds
  `choir.publication` and `choir.publication_version` (source document, route,
  first version time), and `vec_story_passages` (one row per passage of a
  published version, `story_id` is the publication). A story's vector is the
  mean of its latest version's passages; pairs with cosine of 0.75 or more
  (14,989 directed pairs) were read.
- **Searches**: `journalctl -u go-choir-gateway` on Node B,
  `search request from autoputer vm-1669020adcece51c2f5ff9b6df399ef5`,
  fields `run=`, `desk=`, `conv=`, 2026-11-18T16:50Z to 2026-11-20T22:21Z
  (9,482 lines, 1,288 without a run on 2026-11-18); query text was not kept.
  Run tokens from `GET /api/runs/{id}` metadata (`input_tokens`,
  `output_tokens`), 305 runs; traced runs from `GET /api/runs/{id}/events`.
- **Console**: `management story choice` lines from
  `/var/lib/go-choir/vm-state/vm-1669020adcece51c2f5ff9b6df399ef5/console.log*`
  (142 lines, 21:11:49Z to 22:31:36Z).
- **Reading**: stories ordered by creation; for each window story the best
  earlier candidates (title and first paragraph TF-IDF, story-vector cosine) were
  printed with scores and I judged each: same happening and same news (a),
  same happening with a real development (b, flagged `b` when borderline), or
  different. I did not use the vector or TF-IDF score as a verdict. Reading
  counts: 467 candidate stories read, 60 of the other 521 sampled.
- **Rank at open**: revision timestamps from Texture history, rank of the
  earlier story by last revision at the run's start (or the opening
  commit minus 90 seconds in the aggregate), over every story in the 3,876.

## 8. What I could not determine

- Whether Jev's intake label named the earlier story for any given opening
  capture (labels are not in the graph database, the run record or the
  console window for those runs), and what the management model read or
  thought (the first user turn and the reasoning text are not stored).
- Whether `swe-2-flash` read the 50-story page at all; its code called
  `choir.Stories` in only 2 of 6 traced runs, and the page came with the first
  message, which is not recorded.
- Whether the event layer or the entity layer would have found the earlier
  story: no story-to-event link exists; the events route reaches 47 minutes;
  the entity layer was not read.
- Duplicates older than 72 hours, other computers, other days: the
  window is one day. Today (a Friday with an Anthropic IPO, the ICJ ruling and
  the Shenzhen summit) may have more repeats than an ordinary day.
- Per-story model-call counts (the gateway's inference lines have no run id);
  tokens stand in.
- The 41 borderline follow-ups and the 5 borderline duplicates are judgment
  calls; a second human reader would change the 20.1% to 24.2% range, not the
  conclusion.

## Appendix: labelled pairs

`later earlier kind gap_hours`, the first eight hex of the document ids
(earlier is the one earlier story I linked, not always the nearest one); kind
`D` duplicate, `F` follow-up, `(b)` borderline. 238 rows: 166 `D`, 5 `D(b)`
(duplicates I flagged uncertain; counted in the 171), 26 `F`, 41 `F(b)`
(the borderline follow-ups).

```
135f21c4 3b211976 D 26.6
84c5e667 b546efad D 15.8
84e00957 12145da3 F(b) 35.3
fd6c4fd3 50a861e7 F(b) 34.2
bc908433 f18b9050 D 20.2
8100bcd0 5f2c7e11 F 39.0
b8ec6876 8574374e F 12.1
d432666f 00cd2e2c D 47.7
c99b67a4 1f4178a4 D 49.5
9dc8c3aa 60f8b260 D 52.8
d8a82252 537e6d5f F(b) 35.7
0ddaf76b 9630e510 D 3.6
956642f1 43b27427 F 42.5
8d829b37 18da72a4 D 2.2
1a84fb6b beb97d2b D 51.2
e34a9a63 98a5b264 D 49.2
a5d49451 2cd145d7 D 50.2
16cc8eff 34c197da D 1.7
12f15d44 0c45347a D 1.1
c852e275 e867c7cd F 20.1
1a3ec0c1 b0551254 D 3.3
0531eede 8ced5172 D 3.2
a323d1a1 46611a37 F 48.7
b83beea1 76fa4cca F(b) 22.0
7300cfb3 c6cd6aa5 D 4.5
93b638b9 aa352dfe D 3.0
4d6402b7 54fac56d F(b) 33.0
e45d435c 20f8c146 D 38.9
58b2ffde a91d8c5d F(b) 55.8
23e76752 7b29ef34 D 3.0
65964ff8 3344fdae D 1.1
b7d3bf3e 00cd2e2c D 50.4
22f07375 7e3fcbd7 F 42.3
572a0598 e914139e F 46.6
09d3f24c fadfbbdd D 12.7
cc8e9e24 1db528b4 D 17.8
775f0093 e701b034 D 5.5
edc0adc9 85d9d841 D 26.0
e6cd10b7 36b7ac04 F(b) 47.0
9976650a 3cb832bb D 5.9
08cc8722 537e6d5f F 39.8
b905efb2 cc8e9e24 D 1.2
656ea6ba aa1b4674 D 3.1
71f01133 640f88f1 F(b) 32.6
d2200df0 b8b50e1d F 51.2
530c487d 8aee5d25 D 70.7
c2b11a2f f6108c06 D 14.3
5a0edebe 6403f385 F 42.5
78f154ae a6c6229c D 3.5
710c765d 323c7453 F 44.8
df5a34fd b43bba83 D 2.4
110bee73 10543629 D 12.6
f55fd0bb 78f154ae D 0.6
8b6f1da2 aa6415cc F 4.5
6e03de7e 34a12f07 D 5.4
873c2166 7995de6e D 29.6
85c0bb91 6d58a7a7 D 38.7
89018200 c4edd05c D 35.3
46bbdde8 fc18204a D 2.2
ae12e400 3cb832bb D 8.1
219c1276 775f0093 D 2.7
818d8611 3ee3dc2e F(b) 27.7
f41a3e98 2b08ee37 F(b) 7.7
84789852 5d147aee D(b) 54.0
86eae6ec eda815ca D 59.2
9c524016 8c0af904 D 0.5
4b495f4d ee55652e D 39.3
b6cc576b c0313144 D 1.0
c8e15c0a 84789852 D(b) 0.3
5ffc712a cfe03f6f D 41.9
7cacd632 cdb9deea D 57.2
2e95c897 500b8ce6 F 53.9
14904e3f 6f2bed73 D 58.0
9f48d4fe 84789852 D(b) 1.2
2a31bb87 cf57b2c7 D 54.3
7955308b fad9703d D 8.0
b47973dc b905efb2 D 3.6
380fb7ca 00db3adb D 38.2
dc47e897 77dd0944 D 54.2
76efae56 06de10b1 D 38.3
e66d5af7 3ff73681 D 32.1
d2ce2707 8c0af904 D 2.6
42168f75 e357bdad D 8.3
1f5c0def 4d6402b7 D 5.7
a67fc560 c02d885c F 10.3
b4d1da3e 1964c203 F(b) 42.7
c7a68c07 87b5431f D 23.7
d4373f9b 481b905c F(b) 27.5
64405d2a beb447de D 8.0
72fc0389 f3857566 D 10.5
1d144115 d9506f78 D 22.9
e80da993 3b1475b3 D 16.4
76b40a1e 2ca933cf F(b) 12.5
dc4b2961 55b141a8 D 35.9
2fa9659f 362fea29 F(b) 37.9
891f394d 7955308b D 1.6
d9eab831 173dbfa1 D 4.7
0f1c5e1b e13a96cf D 25.3
52036273 035b4044 F 23.9
54a83c99 e9e44905 F(b) 19.6
55e351c6 5e57a9ad F(b) 17.4
a2ce066c fdb3eb60 D 1.3
058699b5 42cb247f D 11.9
c6602287 aa0d0ab6 D 13.0
bafccb89 3972d55d F(b) 25.9
60bfdd16 05ddfa29 D 19.8
34b62b96 09e0f7bf D 13.7
b75d8643 9c18e256 D 18.5
dac3df2a 7955308b D 2.8
2287b239 ec76adaf F(b) 42.0
fdefaefa 0613d12e D 36.7
1230168f ff5106b0 D 48.3
9a938997 b54445d1 D 46.3
d1779a24 387807d0 F(b) 53.6
56cdbcd8 cb324e6e F 12.7
5dd81503 c1d58f61 D 50.2
4ae9cf2f ca5cb3d3 D 47.5
d83f00c3 b727308f D 16.1
79a23da8 a13b9fc8 D 23.8
9d42b148 956642f1 D 11.0
2391941d 2c676bae D 30.2
b5e2bb94 a48d46fe D 56.4
f389a9ce 5f74419a D 36.0
f45e9c33 4fb6fc5f F(b) 11.1
03786d96 08cc8722 D 7.9
c0de9eb0 18ab9e3d F 59.8
2ec810bb f6e44136 D 69.4
740124af 808bdcb9 D 11.2
202c1118 932674e6 D 40.9
3927152b e77bdefa D 26.1
3d917b9d 0f1c5e1b F(b) 3.2
482fd1e1 b2e3d7fa F 3.1
4cc27ec8 f04172ce D 19.6
faffd320 2391941d D 1.1
ecc99910 8159dd1f D 12.9
720e7155 eb35cf55 D 31.1
84abb2bb 9cacab8e D 60.8
6ae89388 f2cf1bfb D 1.4
e73927a9 a74bb716 D 3.0
50a4e312 86945f55 F(b) 32.3
44bfd8db 45eecea5 D 15.5
7f48b919 c99b67a4 F(b) 13.6
1e6285c3 4fb6fc5f D 13.2
13b6fa34 04eba505 D 36.4
063e29b2 f389a9ce D 2.5
0ddc9249 f55fd0bb D 8.9
2e1a3dfd edc0adc9 D 11.2
f52fcc7b fa870a16 F 13.4
42d90e02 3d1d1f69 D 59.8
c55e9e4b eda815ca F(b) 68.1
f920390b 18feea2b D 16.7
d0fa8dee 69bab36e D 55.4
31ae8eea 03786d96 D 3.8
4130ee15 edc0adc9 D 12.8
18f8387f 19d61299 D 34.1
eebe52b2 b83587f5 F 43.4
05816b8c 7903c6a1 D 50.2
65f7473c c94dd746 D 52.8
344ccb67 78f154ae D 11.5
1f7ae6d2 5b2ae51e D 38.5
3c72d714 01b9e91a D 36.2
b67c0e91 3cb832bb F(b) 18.6
0a6d6a5e fed47fb5 D 5.7
8f260696 50a4e312 D 2.9
36423d56 33035fac D 44.8
d388663c 4e5ea35f D 10.4
4647a59f 00d92165 F(b) 7.7
744a73f2 ecc99910 D 3.4
9841f570 50a4e312 D 3.3
48c4e5be 621ee3f3 D 0.9
e8e90cdd eb2e79b6 D 15.9
1cbe9cde f6624725 F(b) 45.4
b750e053 add6053a D 36.2
23b3a7b2 f54ff8f1 D 19.0
7770d5cb aa6415cc D 16.0
92938d46 b47973dc D 9.3
cbf0974f 46bbdde8 D 11.8
8556b5a9 9cf54ad3 D 57.7
b6bb735b 00d92165 D 9.3
df5eeb5b 572a0598 D 15.1
45f56657 09d1238b D 11.5
d3a39e36 e64f2cb3 D 50.6
fc5b277a c2740066 D 7.7
0d287ae3 a31fff31 D 17.3
13e9b4a7 62d732cb D 35.5
7479292f 2af1a7a2 D 1.2
14ecd00a 8a5e54c6 D 12.2
2edb7201 a8a8dceb F(b) 14.8
f96cf1fe 06516652 D 1.3
21202618 76b40a1e D(b) 10.0
4451d577 0a6d6a5e D 2.7
91b22145 31ae8eea D 3.3
ffb1ef90 48c4e5be D 2.4
52f3b148 75d01644 D 1.0
77f1fb9a b67c0e91 D 2.9
a30ff21a 927f564d D 61.4
2fd602eb 8e6d49a4 D 3.7
13d334d5 e66a7e23 D 52.0
03410a2a 6920178a D(b) 22.1
3b80a4ee d82f5062 D 28.1
d5ad8a01 598eb959 F(b) 39.3
79cb8570 c2e8fb96 D 4.1
c241c482 4ee73f79 D 9.5
ad24fa97 d4373f9b F(b) 11.7
d6ee39e4 fdefaefa F 9.5
b17d96dc 1f5c0def D 12.1
3bf843bb 569c389b D 25.9
064b1713 9e77c945 F 29.4
654f980a 22507fa9 F(b) 71.6
dbee98f6 f55fd0bb D 15.1
fa1b1ae6 a34ba920 F 17.4
4d21e0bc fa6ac54a F(b) 54.3
12d33829 3a600321 F 48.4
93036095 9d42b148 D 9.1
6b2a2206 7282f441 F(b) 56.3
95d71669 e8bf5631 D 3.8
1ddd5f77 14ecd00a D 1.9
5ef3628f a6cb22de F(b) 48.7
0c891c03 10d74419 D 0.6
3d847ac5 cb2695b0 F(b) 5.8
5a97a063 5c1b5c64 D 3.4
9c810d91 06516652 D 3.3
926286d4 dbee98f6 F(b) 0.6
831b9b4d 31cf2a5f F(b) 30.6
e278f1c4 a46e4c78 D 10.6
c858a881 fadfbbdd F(b) 31.0
140fdb0b a24e0a5e F 20.0
a4cf292e 4d21e0bc F(b) 1.4
d6f28f99 cb2695b0 F(b) 6.9
eb6a08b4 2fa155ce D 3.1
5b88ba3c 8841d2d0 F(b) 41.0
3138f7b2 8f348525 F 18.7
8516296b d6ee39e4 D 2.0
36870dbf c241c482 D 2.0
02af6105 f13f9f38 D 2.2
3914c08a f335f2f8 D 25.9
2d5832c2 f7b31aa4 D 15.4
077891ef a6b3fb74 D 3.4```
