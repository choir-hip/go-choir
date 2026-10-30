# System Briefings

A system briefing is a read-aloud letter for the owner, about 30 to 40
minutes of listening, that gives depth back for the streams of ideas the
owner puts in: what changed, where the system stands, which decisions wait
on the owner, where the work can go next, and the concepts underneath,
pitched just past the owner's current level. It is v0 of the system-briefing
programme designed for Autoradio
([programming engine design §5](../archive/reports/autoradio-programming-engine-2026-10-14.md)):
a script and a written briefing, before any phone app or Autoradio desk
exists.

## Shape

Every briefing has the same chapters:

1. **What changed** since the last briefing.
2. **State of the system.** Every claim carries a spoken label and a short
   ref in parentheses (a commit or a doc path) so it can be checked. The
   labels: *built and deployed* (an ancestor of staging's deployed commit),
   *built but not landed* (on a branch, or on main but not yet on staging),
   *decided* (an owner ruling or accepted design, not built), and
   *hypothesis* (not verified from the repo).
3. **Decisions waiting on the owner**, collected from the open-question
   sections of the reports, deduplicated, and ranked by what each unblocks.
4. **Pathways**: three to five typed alternatives for the next week, each
   with a probability, what it predicts, and the named signal that would
   show it is working.
5. **Concepts**: five to seven ideas the owner touched, each with one
   concrete Choir example.
6. **What I predict you'll ask next**: the learner-model precommitment.

Read-aloud rules: no tables, no code blocks, short paragraphs, abbreviations
spelled out on first use, product names never camel case, retired names
(World Wire, Universal Wire, sourcecycled) never used for Autopaper, and no
gendered pronouns for the owner.

## How to generate one

1. Gather the source pack (no model calls; reads only `docs/` Markdown and
   YAML from `origin/main`, git metadata, and two fields of the staging
   health endpoint):

   ```sh
   git fetch origin
   python3 -I scripts/system_briefing.py --since 2026-10-14T00:00:00Z \
     --out /tmp/choir-briefing/pack.json
   ```

   `--since` takes an ISO time or a git ref (default: the last 24 hours).
   The default output lands outside the repository, because the pack
   carries local branch names and worktree paths and this repository is
   public. Do not commit packs.

2. Write the briefing from the pack and from reading the sources it points
   to. The pack is an index, not the truth: problem-doc status lines go
   stale within hours, so check each against later commits, and prefer the
   code and the deploy identity over a document when they disagree.

3. Save it twice: here as `system-briefing-YYYY-MM-DD.md`, and as a
   read-aloud copy in iCloud Drive at
   `Choir Reports/Choir System Briefing - YYYY-MM-DD.md`.

## Settling the predicted questions

The last section of each briefing lists the questions the writer predicts
the owner will ask next, each with a probability. That list is a
precommitment about the owner's understanding and interests, recorded
before the evidence exists (the learner model of design §5.4, in its
smallest form).

The next briefing settles it before writing anything new:

- For each prediction, look at what the owner actually asked or directed
  since: owner notes and rulings in `docs/reports/`, commit messages that
  quote the owner, and the owner's own messages in the working sessions.
- Mark each prediction **asked** (the owner raised it, in any words),
  **not asked**, or **unscorable** (no record of the owner's words in the
  window), citing the evidence.
- Score the asked/not-asked rows with the Brier score, (p - outcome)^2,
  and report the count and mean in the "What changed" chapter, beside the
  running total across briefings.
- Note questions the owner asked that the list missed; they are the
  learner model's surprises and the best guide to the next pitch.

Never edit an earlier briefing's predictions; settle them in the next one.
Once the Autoradio desk exists, these predictions become precommitment
records on the owner's computer and settle the same way.

## Briefings

- [2026-10-14](../archive/briefings/system-briefing-2026-10-14.md): the foundations push,
  security hardening, Autopaper's front page and synthesis intake, and the
  calibration work.
