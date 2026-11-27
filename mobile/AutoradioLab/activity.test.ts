// Activity under bun test: the Swift ActivityFeedTests (V1 to V6), on a canned changelog (invented content).
import { describe, expect, test } from 'bun:test';
import { items, target, when, activityInfo, view, label, type Item } from './activity.ts';
import { HTTPError, SignedOutError } from './api.ts';
import changelog from '../fixtures/activity/changelog.json';

const NOW = 1_800_000_000_000;
const turn: Item = { id: 't1', kind: 'turn', label: 'Answered', detail: '', group: 'radio', ago: '', target: '', title: '', at: 0, unseen: false };

describe('activity', () => {
  test('the changelog parses newest first with groups, targets and unseen', () => {
    const rows = items(changelog, NOW);
    expect(rows.length).toBe(5);
    expect(rows.map((r) => r.at)).toEqual(rows.map((r) => r.at).sort((a, b) => b - a));
    expect(rows[0]).toMatchObject({ kind: 'revision_committed', target: 'texture:sample-doc-edition', label: 'Revision saved', group: 'desk' });
    const used = rows.find((r) => r.kind === 'parent_used')!;
    expect(used).toMatchObject({ target: 'pub:sample-river-festival', group: 'paper' });
    expect(rows.find((r) => r.kind === 'update_finished')?.group).toBe('computer');
    expect(rows.filter((r) => r.unseen).length).toBe(2);
  });
  test('V2 an unknown kind is shown, not dropped', () => {
    const r = items({ events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e', kind: 'brand_new_thing', occurred_at: '2026-11-10T10:00:00Z', summary: 's', subject: {} } }], seen_seq: 0 }, NOW);
    expect(r.length).toBe(1); expect(r[0].label).toBe('Brand New Thing'); expect(r[0].target).toBe(''); expect(label('wake_decision')).toBe('Desk decision');
  });
  test('V6 and V3 targets refuse absolute addresses and schemes', () => {
    expect(target('', 'https://evil.example/x')).toBe(''); expect(target('', '/a:b')).toBe(''); expect(target('', '/')).toBe(''); expect(target('d', '/r')).toBe('texture:d');
  });
  test('V1 and V4 a long summary is cut; a bad date says nothing', () => {
    const r = items({ events: [{ inbox_seq: 1, level: 'major', changelog: { event_id: 'e', kind: 'wake_decision', occurred_at: 'garbage', summary: 'x'.repeat(500), subject: {} } }] }, NOW);
    expect(r[0].detail.length).toBe(240); expect(r[0].ago).toBe('');
  });
  test('relative times', () => {
    expect(when(NOW - 300000, NOW)).toBe('5 minutes ago'); expect(when(NOW - 7300000, NOW)).toBe('2 hours ago'); expect(when(0, NOW)).toBe('');
    expect(when(Date.parse('2026-01-02T03:04:00Z'), NOW)).toBe('2 Jan, 03:04');
  });
  test('V5 a missing route leaves the radio\'s turns visible and says so in one line', async () => {
    const v = await activityInfo({ now: NOW, signedIn: true, radio: [turn], live: [] }, async () => { throw new HTTPError(404, 'HTTP 404'); });
    expect(v.status).toBe('failed'); expect(v.message).toContain('no activity feed'); expect(v.radio.length).toBe(1); expect(v.blank).toBe(false);
    expect((await activityInfo({ now: NOW, signedIn: true, radio: [], live: [] }, async () => { throw new SignedOutError(); })).message).toBe('Not signed in.');
    expect((await activityInfo({ now: NOW, signedIn: true, radio: [], live: [] }, async () => { throw new HTTPError(401, 'x'); })).message).toContain('no longer accepted');
    expect((await activityInfo({ now: NOW, signedIn: true, radio: [], live: [] }, async () => { throw new Error('net'); })).message).toContain('Could not load');
    expect((await activityInfo({ now: NOW, signedIn: false, radio: [], live: [] })).status).toBe('idle');
  });
  test('a signed-in page fills the sections from the changelog it is given; the route is /api/changelog?limit=50&minor=1', async () => {
    const asked: string[] = [];
    const v = await activityInfo({ now: NOW, signedIn: true, radio: [], live: [] }, async (p, q) => { asked.push(p + JSON.stringify(q)); return changelog; });
    expect(v).toMatchObject({ status: 'ok', unseen: 2 }); expect([v.computer.length, v.desk.length, v.paper.length]).toEqual([2, 2, 1]);
    expect(asked).toEqual(['/api/changelog{"limit":50,"minor":1}']);
    expect(view([], 'loading', '', [], []).blank).toBe(false); expect(view([], 'ok', '', [], []).blank).toBe(true);
  });
});

import { radioItems, liveItems, activityPage, noFacts } from './activity.ts';
describe('the rows from native facts', () => {
  test('a turn is its outcome and its words; none heard says so', () => {
    const r = radioItems({ ...noFacts(), turns: [{ id: '1', outcome: 'answered', transcript: 'hello', configName: 'c1' }, { id: '2', outcome: '', transcript: '', configName: '' }] });
    expect(r[0]).toMatchObject({ id: 'turn-1', group: 'radio', label: 'Answered', detail: '“hello”', title: 'c1' });
    expect(r[1]).toMatchObject({ label: 'A turn', detail: '(no words heard)' });
  });
  test('the now lines: radio by phase, computer only when known', () => {
    const f = { ...noFacts(), radio: { known: true, running: true, phase: 'listening' } };
    expect(liveItems(f, null).map((i) => i.detail)).toEqual(['The radio is in a turn (listening).']);
    expect(liveItems({ ...f, radio: { known: true, running: false, phase: '' } }, null)[0].detail).toBe('The radio is stopped.');
    expect(liveItems({ ...f, radio: { known: true, running: true, phase: 'playing' } }, { state: 'running', updateAvailable: true }).map((i) => i.detail))
      .toEqual(['The radio is playing.', 'Your computer is running. An update is waiting.']);
    expect(liveItems(noFacts(), { state: '', updateAvailable: false })[0].detail).toBe('Status is not known yet.');
    expect(liveItems(noFacts(), null)).toEqual([]);
  });
  test('signed out, the page still shows the radio and says nothing of the changelog', async () => {
    const v = await activityPage({ ...noFacts(), turns: [{ id: '1', outcome: 'answered', transcript: 'x', configName: '' }] }, false, 0, '', false, false, 0);
    expect(v.status).toBe('idle'); expect(v.radio.length).toBe(1); expect(v.blank).toBe(false);
  });
});
