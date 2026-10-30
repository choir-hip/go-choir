# Grounding the paper overnight 2026-10-19

I wrote this on the morning of October nineteenth. It covers about eight hours of work and watching, from about two in the morning to ten, UTC, after you asked how the paper could become more grounded than outlets quoting each other.

New stories on the paper no longer read as "the BBC says, Al Jazeera says": the share of their sentences built around an outlet fell from about four in ten to about one in ten.

Here is where it stands. Two changes to how Texture writes are live, along with a change to how its citations are checked. Stories written since about four in the morning state what happened, credit whoever made a claim, such as a ministry, a council or a company, and let the citation number name the outlet. Revisions are better in the sentences they add, but they keep much of the old prose they carry forward, so long-running stories like the France and Albania protest stories still read the old way. Primary sources are still rare. A few new failure shapes appeared, and I recorded them rather than chasing each one. The decisions waiting for you are how to clear the old prose, whether to start the primary-source work, and a short list of older items.

## Where we started

When you sent the phone screenshots, I measured the top twenty-five stories on the front page. Thirty-eight percent of their sentences had an outlet as the subject: "DW reports", "Al Jazeera says". The France protests story had seventeen such sentences out of twenty. Only four of a hundred and thirteen cited sources were primary records, like a World Health Organization notice or an elections office page. And seven sentences spoke in the pipeline's own voice, things like "its captured text gives no further details".

I believed our own checks trained this style, and the night partly confirmed it. Every citation is judged by a second model, Jev, which asks whether the quoted passage supports the words. Until last night, Jev judged a whole paragraph against one quote. So a paragraph that combined two outlets always failed, and Texture learned to retreat into "outlet X reports", sentence by sentence. A story about Jacqui Lambie went through seven published versions in three minutes that way.

## What I changed at two in the morning

I changed four things together, and they went live at quarter past two.

First, Jev now judges the words a citation closes, not the whole paragraph, and it sees the paragraph only as context. A plain sentence with two citations is judged against each source alone.

Second, I rewrote Texture's news rules: report the news, not the coverage; credit a claim to whoever made it, never to the outlet that relayed it; and never describe the sources or the work.

Third, research is now told to fetch the original statement or document when a report relays one.

Fourth, the old whole-article embeds you objected to on the sixteenth are now dropped the next time a story is written. Eight front-page stories still carried one from before the embed was retired.

## The first hour, and a loophole of my own

The first new story after the change was clean. It was about a programming language built to confuse AI, and it named no outlet in nine sentences while citing the project's own repository.

Revisions were another matter. At four in the morning I separated the sentences Texture newly wrote in revisions from the ones it carried forward. Forty-one percent of the new ones named an outlet, which is no better than before. The cause was my own wording. I had let Texture name an outlet "where one alone reports a lead", and almost every claim in a revision has one source, so the exception covered nearly everything.

I fixed the wording and it went live at quarter past four. One report of an event or a statement is now enough to state it plainly, with its citation. An outlet is named only for its own unconfirmed finding, such as an unnamed source or its own analysis, or where outlets disagree. I also widened the rule against describing the sources. And I asked each revision to rewrite any paragraph that breaks these rules.

## What the morning showed

From five to six, seven new stories appeared, and only four of their forty-seven sentences named an outlet. That is nine percent, against thirty-eight at the start. The takeover story about EQT and Cleanaway ran fourteen sentences without one. The water story from Longyearbyen credited the town council and a named permafrost expert. In revisions, the newly written sentences named an outlet between fifteen and twenty-seven percent of the time over the next three hours. The Netanyahu endorsement story went from three outlet sentences out of four to none out of seven.

The last hour before I wrote this, from nine to ten, was the busiest of the night, and the best. Texture wrote ten new sentences into older stories, and none named an outlet. The Alberta referendum story was rewritten whole: it went from five outlet sentences out of seven to none out of four. Across every front-page story that changed in that hour, twelve percent of sentences named an outlet.

I looked for the main risk of the change: an outlet's own scoop stated as plain fact. I did not find one in the stories I read.

The citation checks moved too. The text Jev rejects is now about a hundred and fifty characters long instead of four hundred, so Texture gets told exactly which words failed. I had predicted that the share of Texture's turns spent correcting failed citations would fall below a tenth. It did not at first: it stayed near a third. It has since come down, to between twelve and twenty-three percent in most hours. The lesson is that the paragraph rule made failures long, but it did not cause most of them. The sentences that fail now are mostly sentences about outlets and sources, which no quote can support. So the writing rule, not the judge, was the real lever.

## What is still wrong

The biggest remaining problem is old prose. The Alberta rewrite shows the rule can work, but usually Texture edits the paragraphs that new evidence touches and copies the rest. In the long-running stories, most carried sentences still name an outlet. The Albania protest story carries nine sentences that talk about "captured reports", and eight of them were written before the rules changed.

Two new shapes appeared. A midterm campaign story was built entirely from analysis pieces, so every claim was credited to an outlet under the "its own analysis" exception. And twice, a story cited an outlet that was about something else, then explained why. The Saudi story about Youssef al-Manasif added a BBC report about two other men and then said so, and its headline turned into "report leaves his fate unclear". I first suspected the two-outlet publishing rule, but that story already had two outlets, so that was wrong. My current guess is that a loosely related report was routed to the story, and Texture worked it in instead of rejecting it. I have not traced that yet.

One story also said "The full article remained inaccessible with an HTTP 401 error". That is the pipeline talking, in words my rule did not name.

And primary sources barely moved. Research is asked to fetch originals, but the front page still cites only a few percent primary records. That needs the source registry work you have been planning: ministries, courts, election offices, statistics offices, and wire services that originate reporting.

## What I would do next

The one thing to remember is that the writing rules mattered more than the checks. When Texture is told to state the news and credit the maker, it does, on gpt-6-luna, within the hour.

If I were holding the phone, I would decide how to clear the old prose. There are three ways. We can let it age out, which costs nothing but leaves the long stories looking old for days. We can run a one-time rewrite pass, one cast per front-page story, which is about sixty Texture turns and one new published version each. Or we can show Texture, in its frame, which paragraphs break the rules, so every revision fixes them. That last one needs a detector that leaves real disagreements alone, like the France paragraph where four outlets give arrest counts from twelve hundred to twenty-four hundred. I lean toward the one-time pass now and the frame note later.

Then I would start the primary-source registry, because that is the step from not repeating outlets to actually knowing more than they do. And I would trace the off-topic source on the Saudi story before touching the wording again.

The older open items have not changed: the support-failure wake budget, which matters less now; the event definition; evidence-store growth on the disk; the witness key; off-host backups; the Store A squash; rollback images; OpenCode and search credit; the yaegi bug upstream; and search trimming.

The paper ran all night on gpt-6-luna with a clean console. Disk metadata sits near eighty-seven percent, steady.

## Names and receipts

- Problem docs: docs/problems/articles-relay-outlets-instead-of-grounding-them-2026-10-19.md (baseline, changes, hourly measures, options for the carried prose, the Saudi correction); docs/problems/support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md; docs/problems/support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md.
- First change set, deployed 02:15 UTC: 7a43a3bb5 (span pair unit), 3c006b881 (news rules), 11da38c0b (embed drop), b5dd95722 (test); research prompt version 10.
- Second rules, deployed 04:14:35 UTC: 8be39bb36, CI run 39197635727; problem recorded first in ff6ab680f.
- Hourly records: 07b64ab61, 97628ffa8, a8a0f8fd7, 6ce9615ed, bb9dbea1b, bd280e507, 8013d7d1e.
- Measures: baseline 38 percent outlet-subject sentences (67 of 176), 4 of 113 primary sources; new stories 05:15 to 06:11, 4 of 47; new revision sentences 9 of 22 before the fix, then 2 of 10, 2 of 13, 3 of 11, 3 of 20, 0 of 10; all changed front stories 09:01 to 09:47, 4 of 34.
- Failing-pair median text: 417, 183, 160, 220, 146, 129, 146, 176 characters. Support records per Texture run: 33, 32, 18, 22, 33, 12, 19, 23 percent.
- Stories named: Lambie c0fb708e; Alberta referendum v14 to v18; Saudi pub62337ab28 v7; midterm travel v5; Albania v29; France protests v25 to v35.
- Scripts (session scratchpad): grounding/measure.py, grounding/carried.py, grounding/show.py, grounding/support.py, baseline grounding/baseline-front-20261019T0152.json.
- Handoff: docs/archive/reports/handoff-2026-10-15.md.
