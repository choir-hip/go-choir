// The bar's page model under bun test (bar.ts). Ported in meaning from BarStateTests, the card tests of TalkTests and the overlay tests of RunSheetStoreTests / AudioHeldTests
// (Swift), which this replaces. The failure list, written first:
//  B1  nothing loaded is idle, never paused (a play button with nothing behind it); a loaded programme that is not playing is paused only while the engine runs.
//  B2  every engine phase has its state: candidate listening, addressed/capturing talking, ended heard, acknowledging/holding holding (the holding line, or "On it…"),
//      revised revised; the words are the transcript or "Listening…" / "Heard you".
//  B3  the revised and error windows end by themselves: the state is a pure function of the clock, so asking again later (the task) clears it; a repeat restarts the window.
//  B4  an error beats every phase; an engine error is said in plain words by stage; a line the app said itself is shown as it is.
//  B5  the player's message is new once: it holds the bar for the error window, the same words later do not hold it again, words that went and came back do, and
//      "Pick a story below." never does.
//  B6  a sign-out forgets the held message (it was the old account's): the same words under the next account are new.
//  B7  progress is along the programme, clamped to 1, and 0 with no blocks; a block count past the index never traps.
//  B8  the answer card: working, first, full, failed; the retry flag; cancelled and dismissed entries show no card; the latest visible entry decides.
//  B9  the run sheet's overlay: held for the audio session shows paused; nothing on air shows idle; remote audio shows as the radio with its own play state and progress.
//  B10 `until` is the seconds to the next change of a timed state, 0 when none, so the contract asks again then and not before.
//  B12 a card the listener closed stays closed for that answer: a turn that begins, is cancelled, fails or is pending never brings an older closed card back; a new answer shows; a sign-out forgets.
//  B11 the native answer's real shape: missing fields, wrong types, extra fields and nothing at all give an idle bar, never a throw.
import { beforeEach, describe, expect, test } from 'bun:test';
import { barInfo, forgetBar, dismissAnswer, ERROR_WINDOW, REVISED_WINDOW, PICK_A_STORY } from './bar.ts';
import { nextGeneration } from './api.ts';

const T0 = 1_000_000;
const facts = (o: Record<string, unknown> = {}) => ({
  phase: 'playing', transcript: '', blockIndex: 0, blockCount: 0, blockFraction: 0, running: false, signedIn: true, playing: false, title: '', message: '', handsFree: false,
  holding: '', revisedAt: 0, errorAt: 0, errorStage: '', errorMessage: '', onAir: true, waiting: false, remote: false, remotePlaying: false, remoteTitle: '', remoteFraction: 0, ...o,
});
const play = (o: Record<string, unknown> = {}) => facts({ playing: true, title: 'Harbor strike ends', running: true, blockCount: 4, ...o });
const entry = (o: Record<string, unknown> = {}) => ({ id: 'e1', question: 'what changed?', first: '', firstSource: '', answer: '', status: 'asking', note: '', ...o });
const none = { entries: [] };
const at = (s = 0) => T0 + s;

beforeEach(() => forgetBar());

describe('the bar state', () => {
  test('B1 nothing loaded is idle; loaded and not playing is paused only while running', () => {
    expect(barInfo(facts(), none, at()).state).toBe('idle');
    expect(barInfo(play(), none, at()).state).toBe('playing');
    expect(barInfo(play({ playing: false }), none, at()).state).toBe('paused');
    expect(barInfo(play({ playing: false }), none, at()).text).toBe('Harbor strike ends');
    expect(barInfo(play({ playing: false, running: false }), none, at()).state).toBe('idle');
    expect(barInfo(play({ playing: false, blockCount: 0 }), none, at()).state).toBe('idle');
    expect(barInfo(play({ playing: false, title: '' }), none, at()).state).toBe('idle');
    expect(barInfo(play(), none, at()).text).toBe('Harbor strike ends');
  });
  test('B2 each engine phase has its state and words', () => {
    expect(barInfo(play({ phase: 'candidate' }), none, at())).toMatchObject({ state: 'listening', text: 'Listening…', talking: false });
    expect(barInfo(play({ phase: 'candidate', transcript: 'what a' }), none, at()).text).toBe('what a');
    expect(barInfo(play({ phase: 'addressed' }), none, at())).toMatchObject({ state: 'talking', text: 'Listening…', talking: true });
    expect(barInfo(play({ phase: 'capturing', transcript: 'what about the port' }), none, at())).toMatchObject({ state: 'talking', text: 'what about the port', talking: true });
    expect(barInfo(play({ phase: 'ended' }), none, at())).toMatchObject({ state: 'heard', text: 'Heard you' });
    expect(barInfo(play({ phase: 'ended', transcript: 'hi' }), none, at()).text).toBe('hi');
    expect(barInfo(play({ phase: 'acknowledging' }), none, at())).toMatchObject({ state: 'holding', text: 'On it…' });
    expect(barInfo(play({ phase: 'holding', holding: 'Looking at the third paragraph…' }), none, at()).text).toBe('Looking at the third paragraph…');
    expect(barInfo(play({ phase: 'revised' }), none, at())).toMatchObject({ state: 'revised', text: 'Revised · Show' });
    expect(barInfo(play({ phase: 'something new' }), none, at()).state).toBe('playing');
  });
  test('B3 the revised window ends by itself, and an old revision does not show', () => {
    const f = play({ revisedAt: at() });
    expect(barInfo(f, none, at(0)).state).toBe('revised');
    expect(barInfo(f, none, at(0)).text).toBe('Revised · Show');
    expect(barInfo(f, none, at(REVISED_WINDOW - 0.5)).state).toBe('revised');
    expect(barInfo(f, none, at(REVISED_WINDOW)).state).toBe('playing');
    expect(barInfo(f, none, at(REVISED_WINDOW + 20)).state).toBe('playing');
    expect(barInfo(play({ revisedAt: 0 }), none, at(1)).state).toBe('playing');
  });
  test('B3 a repeated error restarts the window (the native time moves)', () => {
    const e1 = play({ errorAt: at(), errorStage: 'voice', errorMessage: 'x' });
    expect(barInfo(e1, none, at(ERROR_WINDOW - 0.1)).state).toBe('error');
    expect(barInfo(e1, none, at(ERROR_WINDOW)).state).toBe('playing');
    const e2 = play({ errorAt: at(8), errorStage: 'voice', errorMessage: 'x' });
    expect(barInfo(e2, none, at(8 + ERROR_WINDOW - 0.1)).state).toBe('error');
    expect(barInfo(e2, none, at(8 + ERROR_WINDOW + 0.1)).state).toBe('playing');
  });
  test('B4 an error beats every phase; stages and messages are said in plain words', () => {
    const e = (stage: string, message: string, o: Record<string, unknown> = {}) => barInfo(play({ errorAt: at(), errorStage: stage, errorMessage: message, ...o }), none, at(1));
    expect(e('voice', 'boom', { phase: 'capturing', transcript: 'hi' })).toMatchObject({ state: 'error', text: 'The voice stopped. Tap play to carry on.' });
    expect(e('system1', 'boom').text).toBe('Your computer was slow to understand that; carrying on.');
    expect(e('desk', 'boom').text).toBe('Your computer could not act on that. Try asking again.');
    expect(e('programme', 'boom').text).toBe('Something went wrong with the radio.');
    expect(e('desk', 'user is not signed in').text).toBe('Sign in to talk to your computer.');
    expect(e('desk', 'HTTP 429').text).toBe('Your computer is busy right now. Try again in a moment.');
    expect(e('voice', 'the network connection was lost').text).toBe('Could not reach choir.news. Check your connection.');
    expect(e('voice', 'request timed out').text).toBe('Could not reach choir.news. Check your connection.');
    // a line the app said itself (no stage) is shown as it is
    expect(e('', 'That book could not be opened.').text).toBe('That book could not be opened.');
  });
  test('B5 the player message is new once and holds the bar for the error window', () => {
    const m = (msg: string, s: number) => barInfo(play({ message: msg }), none, at(s));
    expect(m('', 0).state).toBe('playing');
    expect(m('Could not load the story.', 10)).toMatchObject({ state: 'error', text: 'Could not load the story.' });
    expect(m('Could not load the story.', 10 + ERROR_WINDOW - 0.1).state).toBe('error');
    expect(m('Could not load the story.', 10 + ERROR_WINDOW).state).toBe('playing');
    expect(m('Could not load the story.', 60).state).toBe('playing');       // the same words still standing: not new
    expect(m('', 61).state).toBe('playing');
    expect(m('Could not load the story.', 70).state).toBe('error');         // gone and back: a new one
    expect(m('Another thing.', 80)).toMatchObject({ state: 'error', text: 'Another thing.' });
  });
  test('B5 "Pick a story below." is not an error', () => {
    expect(PICK_A_STORY).toBe('Pick a story below.');
    expect(barInfo(play({ message: PICK_A_STORY }), none, at()).state).toBe('playing');
  });
  test('B5 a later native error beats the player message, a later message beats an older native error', () => {
    const f = play({ message: 'Message words.', errorAt: at(2), errorStage: 'voice', errorMessage: 'x' });
    expect(barInfo(f, none, at(1)).text).toBe('The voice stopped. Tap play to carry on.');   // the message was first seen at 1 (older than the event at 2)
    forgetBar();
    const g = play({ message: 'Message words.', errorAt: at(-5), errorStage: 'voice', errorMessage: 'x' });
    expect(barInfo(g, none, at(0)).text).toBe('Message words.');
  });
  test('B6 a sign-out forgets the held message', () => {
    const f = play({ message: 'Could not load the story.' });
    expect(barInfo(f, none, at(0)).state).toBe('error');
    expect(barInfo(f, none, at(ERROR_WINDOW + 1)).state).toBe('playing');
    nextGeneration();
    expect(barInfo(f, none, at(ERROR_WINDOW + 2)).state).toBe('error');   // new account, new message
    nextGeneration();
    expect(barInfo(play(), none, at(ERROR_WINDOW + 3)).state).toBe('playing');
  });
  test('B6 the account held before a sign-out does not leak its error into the next', () => {
    expect(barInfo(play({ message: 'Old account words.' }), none, at(0)).state).toBe('error');
    nextGeneration();
    expect(barInfo(play(), none, at(1)).state).toBe('playing');
  });
  test('B7 progress along the programme', () => {
    expect(barInfo(play({ blockIndex: 1, blockFraction: 0.5, blockCount: 4 }), none, at()).progress).toBeCloseTo(0.375, 9);
    expect(barInfo(play({ blockIndex: 3, blockFraction: 0.99, blockCount: 4 }), none, at()).progress).toBeLessThanOrEqual(1);
    expect(barInfo(play({ blockIndex: 9, blockFraction: 0.9, blockCount: 4 }), none, at()).progress).toBe(1);
    expect(barInfo(play({ blockCount: 0, blockIndex: 2, blockFraction: 0.5 }), none, at()).progress).toBe(0);
    expect(barInfo(play({ blockIndex: -3, blockFraction: -1, blockCount: 4 }), none, at()).progress).toBe(0);
    expect(barInfo(play({ phase: 'candidate', blockIndex: 1, blockFraction: 0.5 }), none, at()).progress).toBeCloseTo(0.375, 9);   // carried in every state
  });
  test('B7 the facts the bar passes on: signed in, playing, hands-free', () => {
    expect(barInfo(play({ handsFree: true }), none, at())).toMatchObject({ signedIn: true, playing: true, handsFree: true });
    expect(barInfo(facts({ signedIn: false }), none, at())).toMatchObject({ signedIn: false, playing: false, handsFree: false });
  });
});

describe('the answer card', () => {
  const card = (e: Record<string, unknown>, dismissed = '') => { if (dismissed !== '') dismissAnswer(dismissed); return barInfo(play(), { entries: [entry(e)] }, at()); };
  test('B8 working: asked, nothing yet', () => {
    expect(card({})).toMatchObject({ question: 'what changed?', answerState: 'working', answerText: '', canRetry: false });
  });
  test('B8 first: from the saved stories, labelled; with its source when there is one', () => {
    expect(card({ first: 'The strike ended.' })).toMatchObject({ answerState: 'first', answerText: 'The strike ended.', answerLabel: 'From your saved stories' });
    expect(card({ first: 'The strike ended.', firstSource: "From the paper's story on Harbor" }).answerText).toBe("From the paper's story on Harbor: The strike ended.");
    expect(card({ status: 'answering', first: 'The strike ended.' })).toMatchObject({ answerState: 'first', answerLabel: 'From your saved stories' });
  });
  test('B8 full: the answer, no label, the note, retry only for an answer with a note', () => {
    expect(card({ status: 'answered', answer: 'It ended on Friday.', first: 'old' })).toMatchObject({ answerState: 'full', answerText: 'It ended on Friday.', answerLabel: '', canRetry: false });
    expect(card({ status: 'answered', answer: 'It ended', note: 'The answer was cut off.' })).toMatchObject({ answerState: 'full', answerNote: 'The answer was cut off.', canRetry: true });
    expect(card({ status: 'answering', answer: 'It ended' })).toMatchObject({ answerState: 'full', canRetry: false });
    expect(card({ status: 'answered', first: 'saved', note: 'The full answer did not arrive.' })).toMatchObject({ answerState: 'first', canRetry: true });
  });
  test('B8 failed: plain words and a retry', () => {
    expect(card({ status: 'failed', note: "I can't reach the paper right now." })).toMatchObject({ answerState: 'failed', answerNote: "I can't reach the paper right now.", canRetry: true, answerText: '' });
  });
  test('B8 no card: nothing, only cancelled, or the latest one dismissed', () => {
    expect(barInfo(play(), none, at()).answerState).toBe('');
    expect(barInfo(play(), none, at()).question).toBe('');
    expect(card({ status: 'cancelled' }).answerState).toBe('');
    expect(card({ id: 'e1', status: 'answered', answer: 'a' }, 'e1').answerState).toBe('');
    forgetBar();
    expect(card({ id: 'e1', status: 'answered', answer: 'a' }, 'other').answerState).toBe('full');
  });
  test('B8 the card names the entry it shows, so Close can close that one even when a later entry was cancelled', () => {
    const entries = [entry({ id: 'e1', status: 'answered', answer: 'a' }), entry({ id: 'e2', status: 'cancelled' })];
    const shown = barInfo(play(), { entries }, at());
    expect(shown.answerId).toBe('e1');
    dismissAnswer(shown.answerId);
    expect(barInfo(play(), { entries }, at()).answerState).toBe('');
    expect(barInfo(play(), none, at()).answerId).toBe('');
  });
  test('B8 the latest entry that is not cancelled decides', () => {
    const entries = [entry({ id: 'e1', status: 'answered', answer: 'old answer' }), entry({ id: 'e2', question: 'later?', status: 'failed', note: 'no' }), entry({ id: 'e3', status: 'cancelled' })];
    expect(barInfo(play(), { entries }, at())).toMatchObject({ question: 'later?', answerState: 'failed' });
    dismissAnswer('e2');
    expect(barInfo(play(), { entries }, at()).answerState).toBe('');         // the dismissed one hides the card; the one before it does not come back
  });
});

describe('a closed card stays closed (B12)', () => {
  // The listener's Close is remembered here, in shared code, by the id of the entry the card showed. A turn that begins is not a new answer: System 1 may cancel it as "nothing was asked", and
  // the card of the older answer must not come back then (the owner closed it twice and it kept returning; Conversation.begin used to clear the one native dismissed id).
  const ans = (id: string, extra: Record<string, unknown> = {}) => entry({ id, question: 'q ' + id, status: 'answered', answer: 'answer ' + id, ...extra });
  const shows = (entries: unknown[]) => barInfo(play(), { entries }, at());
  test('B12 dismiss, then a turn that is cancelled: the card stays closed', () => {
    const shown = shows([ans('e1')]);
    expect(shown.answerId).toBe('e1');
    dismissAnswer(shown.answerId);
    expect(shows([ans('e1')]).answerState).toBe('');
    expect(shows([ans('e1'), entry({ id: 'e2', status: 'cancelled' })])).toMatchObject({ answerState: '', answerId: '' });
  });
  test('B12 dismiss, then a turn that is only pending: its own working card shows, and when it is cancelled the old one stays closed', () => {
    dismissAnswer('e1');
    expect(shows([ans('e1'), entry({ id: 'e2' })])).toMatchObject({ answerId: 'e2', answerState: 'working', question: 'what changed?' });
    expect(shows([ans('e1'), entry({ id: 'e2', status: 'cancelled' })]).answerState).toBe('');
  });
  test('B12 dismiss, then a turn that fails before answering shows its own failure, never the older closed answer', () => {
    dismissAnswer('e1');
    expect(shows([ans('e1'), entry({ id: 'e2', status: 'failed', note: 'no' })])).toMatchObject({ answerId: 'e2', answerState: 'failed' });
  });
  test('B12 dismiss, then a new real answer: the new card shows and the old stays closed', () => {
    dismissAnswer('e1');
    const now = shows([ans('e1'), ans('e2')]);
    expect(now).toMatchObject({ answerId: 'e2', answerState: 'full', answerText: 'answer e2' });
    dismissAnswer('e2');
    expect(shows([ans('e1'), ans('e2')]).answerState).toBe('');
  });
  test('B12 dismiss twice in a row with phantom turns between: nothing comes back', () => {
    dismissAnswer('e1');
    expect(shows([ans('e1'), entry({ id: 'p1', status: 'cancelled' })]).answerState).toBe('');
    expect(shows([ans('e1'), entry({ id: 'p1', status: 'cancelled' }), ans('e2')]).answerId).toBe('e2');
    dismissAnswer('e2');
    expect(shows([ans('e1'), entry({ id: 'p1', status: 'cancelled' }), ans('e2'), entry({ id: 'p2', status: 'cancelled' })]).answerState).toBe('');
    expect(shows([ans('e1'), entry({ id: 'p1', status: 'cancelled' }), ans('e2'), entry({ id: 'p2', status: 'cancelled' }), entry({ id: 'p3', status: 'cancelled' })]).answerState).toBe('');
  });
  test('B12 closing one answer does not close a different one', () => {
    dismissAnswer('e1');
    expect(shows([ans('e1'), ans('e2')]).answerId).toBe('e2');
    expect(shows([ans('e2')]).answerId).toBe('e2');
    expect(shows([ans('e1'), ans('e2'), entry({ id: 'p', status: 'cancelled' })]).answerId).toBe('e2');     // an answer not closed stays across a phantom turn
  });
  test('B12 a sign-out forgets what was closed (it was the old account\'s) and bad ids are ignored', () => {
    dismissAnswer('e1'); dismissAnswer(''); dismissAnswer(7 as never);
    expect(shows([ans('e1')]).answerState).toBe('');
    nextGeneration();
    expect(shows([ans('e1')]).answerState).toBe('full');
  });
  test('B12 the memory is bounded and keeps the newest', () => {
    for (let i = 0; i < 500; i++) dismissAnswer('x' + i);
    dismissAnswer('last');
    expect(shows([ans('last')]).answerState).toBe('');
  });
});

describe('the run sheet overlay', () => {
  test('B9 an entry held for the audio session shows paused, not playing', () => {
    expect(barInfo(play({ waiting: true, ready: true }), none, at())).toMatchObject({ state: 'paused', playing: false });
    expect(barInfo(play({ waiting: true, ready: true, remote: true, remotePlaying: true, remoteTitle: 'Episode' }), none, at()).state).toBe('playing');   // remote audio is on its own
  });
  test('B9 nothing on air shows idle with nothing in it', () => {
    expect(barInfo(play({ onAir: false, ready: true, progress: 0.7, blockIndex: 2 }), none, at())).toMatchObject({ state: 'idle', text: '', playing: false, progress: 0 });
    expect(barInfo(play({ onAir: false, ready: true, playing: false }), none, at())).toMatchObject({ state: 'idle', text: '', progress: 0 });
    expect(barInfo(play({ onAir: false, ready: true, phase: 'capturing' }), none, at()).state).toBe('talking');   // a turn is not the programme
    expect(barInfo(play({ onAir: false, ready: true, errorAt: at(), errorStage: 'voice' }), none, at(1)).state).toBe('error');
  });
  test('B9 remote audio shows as the radio: its play state, its title, its place', () => {
    const f = facts({ remote: true, remotePlaying: true, remoteTitle: 'Episode 12', remoteFraction: 0.25, onAir: true, ready: true });
    expect(barInfo(f, none, at())).toMatchObject({ state: 'playing', text: 'Episode 12', playing: true, progress: 0.25 });
    expect(barInfo({ ...f, remotePlaying: false }, none, at())).toMatchObject({ state: 'paused', text: 'Episode 12', playing: false, progress: 0.25 });
    expect(barInfo({ ...f, phase: 'capturing' }, none, at()).state).toBe('talking');            // a phase wins over the overlay
    expect(barInfo({ ...f, remoteFraction: 7 }, none, at()).progress).toBe(1);
  });
});

describe('the timed states', () => {
  test('B10 until: the seconds to the next change, 0 when none', () => {
    expect(barInfo(play(), none, at()).until).toBe(0);
    const err = barInfo(play({ errorAt: at(), errorStage: 'voice' }), none, at(1));
    expect(err.until).toBeGreaterThan(ERROR_WINDOW - 1);
    expect(err.until).toBeLessThanOrEqual(ERROR_WINDOW - 1 + 0.2);
    expect(barInfo(play({ revisedAt: at() }), none, at(2)).until).toBeGreaterThan(REVISED_WINDOW - 2);
    expect(barInfo(play({ revisedAt: at(), errorAt: at(), errorStage: 'voice' }), none, at(1)).until).toBeGreaterThan(ERROR_WINDOW - 1);   // the nearer end
    expect(barInfo(play({ errorAt: at(), errorStage: 'voice' }), none, at(ERROR_WINDOW + 1)).until).toBe(0);
  });
  test('B10 the held message holds the bar, so it counts too', () => {
    expect(barInfo(play({ message: 'Words.' }), none, at(5)).until).toBeGreaterThan(0);
    expect(barInfo(play({ message: 'Words.' }), none, at(5 + ERROR_WINDOW + 1)).until).toBe(0);
  });
});

test('B3 the clock is the facts\' own `at` when none is passed', () => {
  const f = play({ revisedAt: 500, at: 502 });
  expect(barInfo(f, none).state).toBe('revised');
  expect(barInfo({ ...f, at: 500 + REVISED_WINDOW + 1 }, none).state).toBe('playing');
  expect(barInfo({ ...f, at: 501 }, none).until).toBeGreaterThan(REVISED_WINDOW - 1.1);
});

describe('the native answer', () => {
  test('B11 missing, mistyped and extra fields give an idle bar', () => {
    expect(barInfo(undefined, undefined, at())).toMatchObject({ state: 'idle', text: '', progress: 0, answerState: '', until: 0 });
    expect(barInfo(null, null, at()).state).toBe('idle');
    expect(barInfo('x', 5, at()).state).toBe('idle');
    expect(barInfo({}, {}, at()).state).toBe('idle');
    expect(barInfo({ playing: 'yes', blockCount: 'many', phase: 4, running: 1 }, { entries: 'none' }, at()).state).toBe('idle');
    expect(barInfo({ ...play(), surprise: [1, 2] }, { entries: [null, 3, 'x', entry()], dismissed: 7 }, at()).answerState).toBe('working');
  });
});

describe('B2 the review of the pages and the bar', () => {
  const read = (f: string): string => require('node:fs').readFileSync(new URL(f, import.meta.url), 'utf8');
  test('B2-6 an empty run (not yet answered) never blanks a playing or paused bar; an answered one that says not on air does', () => {
    expect(barInfo({ playing: true, title: 'A story', blockCount: 3, running: true, onAir: false }, { entries: [] }, 1).state).toBe('playing');       // no `ready`: the run has not answered
    expect(barInfo({ playing: true, title: 'A story', blockCount: 3, running: true, onAir: false, ready: true }, { entries: [] }, 1).state).toBe('idle');
  });
  test('B2-8 a streaming answer says so, so the card can offer Cancel', () => {
    const b = barInfo({}, { entries: [{ id: 'ask-1', question: 'q', status: 'answering', answer: 'Part of it', first: '', note: '' }] }, 1);
    expect(b).toMatchObject({ answerState: 'full', answering: true });
    expect(barInfo({}, { entries: [{ id: 'ask-2', question: 'q', status: 'answered', answer: 'All', first: '', note: '' }] }, 1).answering).toBe(false);
  });
  test('B2-2 Send and the aim line are not tied to the bar being idle', () => {
    expect(read('members-bar.contract')).toMatch(/derive typing = barFocused and ask != ""\n/);
    expect(read('bar.contract')).toMatch(/\(st == "error" \? "error" : \(typing \? "aim" : \(st == "playing"/);
  });
  test('B2-3 Try again asks the answer the card shows; Close closes the card it was drawn for', () => {
    expect(read('bar.contract')).toContain('press=answerRetry(bi.answerId)'); expect(read('bar.contract')).toContain('press=answerDismiss(bi.answerId)');
    expect(read('members-talk.contract')).toMatch(/action answerDismiss\(id: string\)\n    send dm = answerDismissShared\(id\)/);
  });
  test('B2-5 the speed shown follows the radio again when it changed after the finger chose', () => {
    expect(read('members-bar.contract')).toContain('derive speedNow = speedLive > 0 and speedBase == ri.speedValue ? speedLive : ri.speedValue');
  });
  test('B2-8 Cancel is offered while the answer streams', () => {
    expect(read('bar.contract')).toContain('if bi.answerState == "working" or bi.answerState == "first" or bi.answering');
  });
});
