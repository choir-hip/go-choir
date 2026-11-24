// Learning the person's voice, the page's side (owner, 2026-11-24: "we should learn user voice in voice onboarding"). After the downloads the person reads three short sentences aloud (about ten seconds in all), the profile is built
// on the phone, and one more sentence checks it before it says "stored". The same page is the first-run step and Settings' "Learn again". The microphone, the frames and the embedding model are native (VoiceEnrolment.swift);
// this file is the words, the order of the sentences and what each button does. Nothing leaves the phone. In this build the later check only LOGS its verdict per turn and rejects nothing.
//
// Failure list (each line has a test in voicelearn.test.ts):
//  Y1 The sentences are in a fixed order, three to learn from and one to confirm; the page shows the one that is next, and never the confirming one before the voice is built.
//  Y2 Nothing opens the microphone before the person presses Read it; the first press asks for the microphone with the pre-prompt line first (Continue, Not now), only when the microphone has not been allowed.
//  Y3 A sentence that was not heard is asked again with a line saying so; a take that never produced speech counts for nothing.
//  Y4 "Stored" is said only when the confirming sentence matched; an unsure or failed build says so and offers to learn again, never "stored".
//  Y5 No model on the phone: the page says so and presses start nothing.
//  Y6 "Forget my voice" and "Learn again" call native first (it deletes the old profile) and the page shows the start again.
//  Y7 A refused microphone shows the declined line and no button that would ask again until the person presses again.
export type Enrol = { phase: string; taken: number; needed: number; again: boolean; level: number; learned: boolean; ready: boolean; call: string };
export const noEnrol = (): Enrol => ({ phase: 'idle', taken: 0, needed: 3, again: false, level: 0, learned: false, ready: false, call: '' });

export const SENTENCES = [
  'Good morning, this is my voice on the radio.',
  'Play me the news about the weather and the local team.',
  'Skip this one and read me the next story.',
] as const;
export const CONFIRM_SENTENCE = 'Turn the volume down a little, please.';

export const MIC_PRE_LINE = 'To talk to the radio or record a take, Autoradio 1 needs your microphone and speech recognition.';
export const MIC_OFF_LINE = 'Microphone is off for Autoradio 1. You can turn it on in Settings.';

export type Learn = {
  step: string; title: string; line: string; sentence: string; count: string; notice: string; level: number;
  primary: string; primaryOp: string; secondary: string; busy: boolean; stored: boolean;
};

export function learnView(f: { enrol: Enrol; mic: string }): Learn {
  const e = f.enrol, n = Math.min(e.taken, e.needed);
  const base: Learn = { step: 'intro', title: 'Teach the radio your voice', line: '', sentence: '', count: '', notice: '', level: e.level, primary: '', primaryOp: '', secondary: '', busy: false, stored: false };
  if (!e.ready && e.phase !== 'stored') return { ...base, step: 'noModel', line: 'The voice model is not on this phone yet. It comes with the downloads, in Settings.', primary: '', secondary: 'Not now' };       // Y5
  if (f.mic === 'refused') return { ...base, step: 'refused', line: MIC_OFF_LINE, primary: '', secondary: 'Not now' };                                                       // Y7
  switch (e.phase) {
    case 'listening':
      return { ...base, step: 'reading', title: 'Read this aloud', sentence: SENTENCES[Math.min(n, SENTENCES.length - 1)], count: `Sentence ${n + 1} of ${e.needed}`, line: 'Listening. Stop at the end of the sentence.', busy: true };
    case 'building': return { ...base, step: 'building', title: 'Learning your voice', line: 'One moment.', busy: true };
    case 'confirm': return { ...base, step: 'confirm', title: 'One more, to check it', sentence: CONFIRM_SENTENCE, line: 'Read this sentence, and the radio will say whether it knows you.', notice: e.again ? 'I did not hear that. Read it again.' : '', primary: 'Read it', primaryOp: 'read', secondary: 'Not now' };
    case 'confirming': return { ...base, step: 'confirming', title: 'One more, to check it', sentence: CONFIRM_SENTENCE, line: 'Listening.', busy: true };
    case 'stored': return { ...base, step: 'stored', title: 'Your voice is stored', line: 'Kept on this phone only, never uploaded. You can forget it or learn it again in Settings.', stored: true, primary: 'Done', primaryOp: 'close' };             // Y4
    case 'unsure': return { ...base, step: 'unsure', title: 'Not sure that was you', line: 'The radio could not tell. Learn your voice again, in a quiet place.', primary: 'Learn again', primaryOp: 'again', secondary: 'Not now' };
    case 'failed': return { ...base, step: 'failed', title: 'That did not work', line: 'The three sentences did not sound like one voice, or were too quiet. Try again in a quiet place.', primary: 'Learn again', primaryOp: 'again', secondary: 'Not now' };
    default: {
      if (n === 0 && !e.again) return { ...base, line: `Read ${e.needed} short sentences out loud, about ten seconds in all. Kept on this phone only, never uploaded.${f.mic === 'unasked' ? ' ' + MIC_PRE_LINE : ''}`, primary: f.mic === 'unasked' ? 'Continue' : 'Start', primaryOp: 'start', secondary: 'Not now' };
      return { ...base, step: 'reading', title: 'Read this aloud', sentence: SENTENCES[Math.min(n, SENTENCES.length - 1)], count: `Sentence ${n + 1} of ${e.needed}`, line: '', notice: e.again ? 'I did not hear that. Read it again.' : '', primary: 'Read it', primaryOp: 'read', secondary: 'Not now' };      // Y1, Y3
    }
  }
}

type Later = ((r: { op: string; [k: string]: unknown }) => Promise<unknown>) | null;
/** Source: the page, from the native facts (the voice side's, which carry `enrol`). */
export function learnPage(f: { enrol?: Enrol; mic?: string } | null | undefined): Learn { return learnView({ enrol: { ...noEnrol(), ...(f?.enrol ?? {}) }, mic: f?.mic ?? 'unasked' }); }

/** One press. `start`: native deletes the old profile, then the first sentence is listened to; `read`: listen to the next one; `again` (Learn again): as start; `forget`: delete it; `close`: stop listening. Y2, Y5, Y6. */
export async function learnAct(op: string, f: { enrol?: Enrol; mic?: string } | null | undefined, later: Later): Promise<Learn> {
  const v = learnPage(f);
  if (later && (v.primaryOp !== '' || op === 'forget' || op === 'close')) {
    try {
      if (op === 'forget') await later({ op: 'obVoiceForget' });
      else if (op === 'close') await later({ op: 'obVoiceStop' });
      else if (op === 'start' && v.primaryOp === 'start') { await later({ op: 'obVoiceStart' }); await later({ op: 'obVoiceBegin' }); }
      else if (op === 'again' && v.primaryOp === 'again') { await later({ op: 'obVoiceStart' }); await later({ op: 'obVoiceBegin' }); }
      else if (op === 'read' && v.primaryOp === 'read') await later({ op: 'obVoiceBegin' });
    } catch { /* the facts say what happened */ }
  }
  return v;
}
