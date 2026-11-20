#!/usr/bin/env bash
# Test for scripts/search-sufficiency-replay. Failure list, written before the code:
#  T7 a search the corpus could not answer, or with unknown counts, is excluded from the shares and counted
#  T8 row 0 is printed at 10 and at 30 minutes (the news window and the default); a search 20 minutes after the same terms is held at 30 and sent at 10
#  T9 a line cut off before its tail fields is skipped, not decided
#  T1 a strong unread entry is row 3, a strong all-read entry row 5, no strong row 6
#  T2 a repeated line (same run and q) is counted once
#  T3 same terms within 30 minutes is row 0 before everything; a corpus that did not answer is row 1
#  T4 unread leads at K hold (row 4) when no strong unread entry does
#  T5 without --outcomes the wrongly-held share says "not computed" and prints no number
#  T6 with --outcomes it is the share of new-page searches a row 0, 3 or 4 would hold
set -u
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
L='search sufficiency op=search_sufficiency'
cat > "$tmp/log" <<LOG
2026/11/20 10:00:00 $L run=r1 desk=research conv=c q=aaaa0001 words=3 ms=90 corpus=answered best=0.900 strong=1 newest_strong_age_s=3600 unread_strong=1 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:3600:u,0.500:100:u decision=sent why=guest_gate_passed err=none
2026/11/20 10:00:00 $L run=r1 desk=research conv=c q=aaaa0001 words=3 ms=90 corpus=answered best=0.900 strong=1 newest_strong_age_s=3600 unread_strong=1 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:3600:u,0.500:100:u decision=sent why=guest_gate_passed err=none
2026/11/20 10:01:00 $L run=r2 desk=research conv=c q=aaaa0002 words=3 ms=90 corpus=answered best=0.900 strong=1 newest_strong_age_s=7200 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:7200:r decision=sent why=guest_gate_passed err=none
2026/11/20 10:02:00 $L run=r3 desk=research conv=c q=aaaa0003 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.400:9000:u decision=sent why=guest_gate_passed err=none
2026/11/20 10:03:00 $L run=r4 desk=research conv=c q=aaaa0004 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=20 unread_leads=16 unread_leads_conv=2 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=none decision=sent why=guest_gate_passed err=none
2026/11/20 10:04:00 $L run=r5 desk=research conv=c q=aaaa0005 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=true same_terms_age_s=600 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:60:u decision=sent why=guest_gate_passed err=none
2026/11/20 10:05:00 $L run=r6 desk=research conv=c q=aaaa0006 words=3 ms=4000 corpus=vector_unavailable best=0.000 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=false same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=none decision=sent why=guest_gate_passed err=none
2026/11/20 10:06:00 $L run=r5 desk=research conv=c q=aaaa0005 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 same_terms=true same_terms_age_s=400 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:60:u decision=sent why=guest_gate_passed err=none
2026/11/20 10:07:00 $L run=r7 desk=research conv=c q=aaaa0007 words=3 ms=90 corpus=answered best=0.900 strong=1 newest_strong_age_s=60 unread_strong=-1 leads_matched=-1 unread_leads=-1 unread_leads_conv=-1 same_terms=unknown same_terms_age_s=0 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=0.900:60:? decision=sent why=guest_gate_passed err=leads_by_words:not_ready
2026/11/20 10:08:00 $L run=r8 desk=research conv=c q=aaaa0008 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 unread_leads_words=0 same_terms=true same_terms_age_s=1200 reuse_min=30 tau_s=0.80 h_h=168 lag_h=6 top=none decision=sent why=guest_gate_passed err=none
2026/11/20 10:09:00 $L run=r9 desk=research conv=c q=aaaa0009 words=3 ms=90 corpus=answered best=0.400 strong=0 newest_strong_age_s=0 unread_strong=0 leads_matched=0 unread_leads=0 unread_leads_conv=0 unread_leads_words=0 same_terms=true
LOG
out="$("$here/search-sufficiency-replay" "$tmp/log" --tau 0.80 --horizon-h 168 --lag-h 0,6 2>&1)"
check "T2 nine distinct lines: the console read twice is counted once, a genuine repeat of a query (another time) is counted" 'grep -q "distinct lines): 9" <<<"$out"'
check "T7 two searches excluded from the shares: the vector half did not answer, and one with unknown counts" 'grep -q "excluded from the shares: 2 " <<<"$out" && grep -q "vector unavailable or call failed: 1" <<<"$out" && grep -q "unknown counts: 1" <<<"$out"'
row30="$(grep -E '^0.80 +168 +30 ' <<<"$out")"
row10="$(grep -E '^0.80 +168 +10 ' <<<"$out")"
# 7 decided: r1 row3, r2 row5, r3 row6, r4 row4, r5 and r5b row0 at both windows, r8 (20 minutes) row0 at 30 and row6 at 10
check "T1/T3/T4 at 30 minutes: row0 42.9%, rows 3, 4, 5, 6 14.3% of the 7 decided" '[ "$(grep -o "14.3%" <<<"$row30" | wc -l)" -ge 4 ] && grep -q "42.9%" <<<"$row30"'
check "T8 at 10 minutes the 20-minute repeat is not held: row0 28.6%, row6 28.6%" '[ "$(grep -o "28.6%" <<<"$row10" | wc -l)" -ge 2 ]'
check "T4/T8 held share rows 0,3,4: 5 of 7 = 71.4% at 30, 4 of 7 = 57.1% at 10 (the conservative bound)" 'grep -q "71.4%" <<<"$row30" && grep -q "57.1%" <<<"$row10" && grep -q "conservative" <<<"$out"'
check "T9 a line cut off before its tail is skipped" '! grep -q "r9\|aaaa0009" <<<"$out" && grep -q "skipped as incomplete: 1" <<<"$out"'
check "T1 row 5 cuts 2 hours with lag 0 and -4 with lag 6 (median newest strong age less lag)" 'grep -q "lag0=2.0 lag6=-4.0" <<<"$row30"'
check "T5 no outcomes: not computed" 'grep -q "wrongly held.*not computed" <<<"$out" && ! grep -q "of those" <<<"$out"'
printf 'r1\taaaa0001\t1\nr2\taaaa0002\t1\nr3\taaaa0003\t0\nr4\taaaa0004\t1\n' > "$tmp/outcomes"
out2="$("$here/search-sufficiency-replay" "$tmp/log" --tau 0.80 --horizon-h 168 --lag-h 0 --outcomes "$tmp/outcomes" 2>&1)"
# new-page searches: r1 (row3 held), r2 (row5 not held), r4 (row4 held): 2 of 3
check "T6 wrongly held is 2 of 3 new-page searches" 'grep -q "66.7% (2 of 3)" <<<"$out2"'
exit $fail
