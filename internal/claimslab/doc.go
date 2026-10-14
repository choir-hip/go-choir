// Package claimslab is the claims layer's offline laboratory (slices E0 and
// E6a of docs/reports/claims-events-entities-ontology-2026-10-14.md). It has
// no runtime wiring: nothing in the computer imports it, it writes no store
// and calls no provider. Scripts under scripts/claims_* drive it through
// its command (internal/claimslab/cmd/claimslab) over JSON files kept
// outside the repo, because they hold article text.
//
// It holds three things:
//
//   - the extraction schema the study measures (Extraction, Claim, Mention,
//     the five uncertainties of ontology §6a), versioned as SchemaVersion so
//     results name the schema they were measured under;
//   - the quote check, which is internal/quotematch unchanged: a claim whose
//     quote does not match one block of its capture is dropped and counted
//     (attempted versus retained, ontology §12.3 E0);
//   - the arithmetic the results report: span matching of extracted claims
//     against labels (precision and recall), proper scores for forecasts
//     (Brier with the Murphy decomposition), and intervals over weighted,
//     possibly fractional outcomes with n_eff pseudo-counts (ontology §6a).
//
// Failure modes, written before the code (CLAUDE.md, Testing):
//
//  1. A quote that matches only after normalization is counted as retained
//     but must be reported by tier, so tolerant matches are visible.
//
//  2. A quote that appears in several blocks matches the first exact one;
//     span matching against labels must use the matched block, not the
//     block the model named, or a correct claim reads as a miss.
//
//  3. A model that names a block id that does not exist, or no block, must
//     still be checked against every block (the model's block is a hint).
//
//  4. Empty quotes, punctuation-only quotes and quotes over the 300-char cap
//     are not retained; the cap is reported separately from not_found.
//
//  5. Two extracted claims can overlap one label; each label is recalled at
//     most once and each extracted claim matches at most one label
//     (greedy by overlap, largest first), so duplicates cost precision.
//
//  6. Overlap is measured in bytes of one block's original text; claims in
//     different blocks never match, even with identical glosses.
//
//  7. Brier on an empty set, intervals with zero weight and probabilities
//     outside [0,1] must not produce NaN; they return an explicit "n too
//     small" (Valid=false) result.
//
//  8. The Murphy decomposition must sum (reliability - resolution +
//     uncertainty) to the Brier score when forecasts are binned by their
//     distinct values; binned decompositions are approximate and say so.
//
//  9. Weights that are all equal must give n_eff = n; one dominant weight
//     must give n_eff near 1, so a single heavy claim cannot narrow an
//     interval.
//
//  10. A beta interval on fractional outcomes uses m from the weighted mean
//     and n_eff; it is never computed from raw counts of a fractional
//     outcome.
package claimslab
