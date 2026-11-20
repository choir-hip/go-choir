// The conversation page's rows under bun test (conversation.ts). Ported from the Swift tests this replaces: TalkTests.testAFailureIsKeptWithARetry (the row can be retried),
// testAThankYouLeavesNoCardAndCancelLeavesNoFailure (nothing left: empty), and the engine tests' reads of `ConversationInfo.make` (a failed turn, a cancelled one). The failure list, written first:
//  C1 newest first: the entries come oldest first, the rows go newest first.
//  C2 cancelled entries are dropped; all cancelled is empty (the empty words), no entries is empty.
//  C3 canRetry: failed yes; answered with a note yes ("cut off", "full answer did not arrive"); answered with no note no; asking and answering no.
//  C4 every field is carried (first, firstSource, answer, note); a first answer with no full answer stays a row.
//  C6 (review 2026-11-20) the source takes the ENTRIES array (`cvraw.entries`), as every other source takes a field and not a whole resource; the page's contract passes exactly that.
//  C5 the real shape of the native source: an extra field (startedAt, a number of seconds) and a missing one change nothing; not an object is empty.
import { describe, expect, test } from 'bun:test';
import { info } from './conversation.ts';
import { readFileSync } from 'node:fs';

const e = (o: Record<string, unknown>) => ({ id: 't', question: 'q', first: '', firstSource: '', answer: '', status: 'asking', note: '', startedAt: 785_000_000.5, ...o });

describe('the conversation rows', () => {
  test('C1 newest first', () => {
    const i = info([e({ id: 't1', question: 'first?' }), e({ id: 't2', question: 'second?' }), e({ id: 't3', question: 'third?' })]);
    expect(i.rows.map((r) => r.id)).toEqual(['t3', 't2', 't1']);
    expect(i.empty).toBe(false);
  });
  test('C2 cancelled entries are dropped; none left is empty', () => {
    expect(info([e({ id: 't1', status: 'cancelled' })])).toEqual({ rows: [], empty: true });
    expect(info([]).empty).toBe(true);
    const i = info([e({ id: 't1', status: 'answered', answer: 'a' }), e({ id: 't2', status: 'cancelled' })]);
    expect(i.rows.map((r) => r.id)).toEqual(['t1']);
  });
  test('C3 canRetry', () => {
    const can = (o: Record<string, unknown>) => info([e(o)]).rows[0].canRetry;
    expect(can({ status: 'failed', note: "I can't reach the paper right now." })).toBe(true);
    expect(can({ status: 'failed' })).toBe(true);
    expect(can({ status: 'answered', answer: 'x', note: 'The answer was cut off.' })).toBe(true);
    expect(can({ status: 'answered', first: 'saved words', note: 'The full answer did not arrive.' })).toBe(true);
    expect(can({ status: 'answered', answer: 'x' })).toBe(false);
    expect(can({ status: 'asking' })).toBe(false);
    expect(can({ status: 'answering', answer: 'part' })).toBe(false);
  });
  test('C4 every field is carried', () => {
    const r = info([e({ id: 't1', question: 'What changed?', first: 'The strike ended.', firstSource: "From the paper's story on Harbor", status: 'answered', note: 'The full answer did not arrive.' })]).rows[0];
    expect(r).toEqual({ id: 't1', question: 'What changed?', first: 'The strike ended.', firstSource: "From the paper's story on Harbor", answer: '', status: 'answered', note: 'The full answer did not arrive.', canRetry: true });
  });
  test('C5 the native shape: extra and missing fields, not an object', () => {
    expect(info([{ id: 'x', status: 'failed' }]).rows[0]).toEqual({ id: 'x', question: '', first: '', firstSource: '', answer: '', status: 'failed', note: '', canRetry: true });
    expect(info([null, 3, 'x'])).toEqual({ rows: [], empty: true });
    expect(info(null)).toEqual({ rows: [], empty: true });
    expect(info({ entries: [] })).toEqual({ rows: [], empty: true });
    expect(info({})).toEqual({ rows: [], empty: true });
  });
  test('C6 the source takes the entries array, and the page passes exactly that', () => {
    expect(readFileSync(new URL('./members-talk.contract', import.meta.url), 'utf8')).toContain('conversationInfo(cvraw.entries)');
    expect(info([e({ id: 'a', status: 'failed' })]).rows.length).toBe(1);
  });
});
