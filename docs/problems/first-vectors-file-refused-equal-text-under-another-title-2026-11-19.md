# The first vectors file was refused whole: equal text under another title carried the wrong vector

2026-11-19. Class: a defect in the Mac's pass tool, caught by corpusd's
sample check. No row was written; nothing wrong reached the index.

## Problem

The file of vectors made on the Mac overnight (278,048 rows: 62,122 story
passages and 215,926 captures) was placed in corpusd's import inbox at
13:08:20Z and refused whole at 13:11:37Z:

    op=vector_import shape=inbox ms=85366 rows=278048 written=0
    sample_min_cosine=0.95817 err=vector import refused: a sample of 32 rows
    agrees only to cosine 0.95817, need 0.999

The rule that refused it is the one in
[the reference page](../reference/EMBEDDINGGEMMA.md), "The first pass file":
a sample re-embedded by the Node B embedder must agree with the file.

## Evidence

- The pass tool (`mobile/EmbedTrial`, `embed-trial pass`, on
  `lane/embed-pass`) kept one vector per `text_sha` and gave it to every
  later row with the same passage. The model's input is
  `title: <headline> | text: <passage>`. A row with the same passage under
  another headline therefore got a vector made from another input.
- Counted from the exports: 2,661 rows were reused from a row with another
  title (341 story passages, where a story's later version changed its
  headline and kept a passage; 2,320 captures, where two pages share a lead
  passage). 19,203 rows were reused from a row with the same title, which is
  correct.
- Read against the server's embedder (the exact prompt each side builds):
  sixteen never-reused rows agree to cosine 1.000 (one at 0.99999); six
  rows reused under another title agree to 0.915, 0.918, 0.933, 0.945,
  0.973 and 0.979.
- A sample of 32 from a file with 0.96% bad rows holds at least one of them
  about one time in four. This file would have been accepted three times in
  four with 2,661 wrong vectors in it.

## Belief

The sample check did its job, by luck of the draw as much as by design. The
defect is one line of the pass tool. The Mac's model and the server's model
agree on equal input; the cut of passages and the titles agree for the rows
checked.

## Repair

- The pass tool keys reuse by `text_sha` and title together, for the run
  only (`lane/embed-pass` at `02ff59b63`; two tests written first, both
  failed on the old code).
- Only the 2,661 rows were embedded again, with the fixed tool, and merged
  into the file by row key. The corrected file is
  `mac-first-pass-2.jsonl` (278,048 rows, sha256 `d268dc60e642238a...`).
  Thirty-two of its rows (twelve of the re-embedded ones and twenty drawn
  across the file) agree with the server's embedder to 1.000.
- It was placed in the inbox at 13:44:54Z. A running pass reads the inbox
  only when it starts, and the pass under way has about 256,000 rows to
  embed, so the file is taken when corpusd next restarts (the next deploy
  that restarts it). `lane/inbox-retry` makes a running pass yield to a
  waiting file; it is in review.

## Remaining error

- A sample of 32 cannot see a defect in fewer than a few percent of rows.
  The rule guards against a wrong model, a wrong prompt and a wrong cut,
  which touch every row; it does not guard against a rare wrong row. The
  count of affected rows above comes from the exports and the tool's rule,
  not from a full comparison; a third cause in a small share of rows would
  not show in 32 samples either.
- The import writes a row only when the server cuts the same passage
  (`text_sha` equal). It does not compare titles, so a row whose title the
  export and the server derive differently would be written with a vector
  made under the export's title. Both take the title from the same fields
  (read in both sources), and the 32 rows checked agree; that is the only
  evidence.
- The lead restarted corpusd by hand once, at 13:09:41Z
  (`systemctl restart go-choir-corpusd` on Node B), so that the first file
  would be taken at once. That is outside the standing rule that Node B
  access is metadata and logs only, and was not repeated for the corrected
  file. The restart ended the worker's pass eleven minutes in; rows already
  embedded were kept.
- The import's write of 278,048 rows has still never run on Node B.
