# Research leaves leads unresolved and unexplained

Found 2026-10-11 in S1d (staging c713d63a, document 596395c7). Evidence
class: final revision text plus research run traces (scratchpad
s1d-final).

## What happened

The final brief ends with a "Contested and unverified reports" list:
"Reflection Beam — circulated, but no primary record was retrieved",
"Binance AI — reported, unconfirmed", "An alleged Anthropic incident, and
an alleged export-control plea — both reported, both unverified". Three of
the eight research rounds searched for these leads (efd81261, a232777e,
2fe8ca50). None reached a verdict, and the brief never says what the leads
claim, who reported them, or how likely they are. Earlier revisions also
held claims in two states at once: the October 8 Waymo financing appeared
both as a fact and as "resting on the week's reporting" until a later
round found Waymo's own post.

The owner (2026-10-11): an unconfirmed finding should go to Texture, and
research should then keep working to confirm it. Research should also
make sensible inferences rather than refusing to conclude.

## Cause (belief)

The research prompt (internal/promptstore/defaults/research.yaml, about
500 words) states the evidence rule several ways ("model recall is not
evidence", "a title or snippet is never a fact", "a plain fetch result is
not a citation"). It never says what to do with a lead between "found"
and "confirmed", and it never permits inference. The safe move under that
prompt is to label a lead "unverified" and stop. Much of the prompt also
repeats surface facts that choir.Help() already gives.

## Test

In the next research-only demo, every lead the brief mentions should
carry a status and a reason: confirmed (with source), refuted, or likely or
unlikely with what that judgment rests on. No bare "unverified" entries,
and no claim held in two states across one revision.
