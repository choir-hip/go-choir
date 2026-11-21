// The edition's writer, as shared code: the edition made from the listener's topics, and how finishing ALWAYS ends (the edition, a failure, or the deadline; the topics stay).
// Ported from EditionSetup.swift (LocalEditionWriter, EditionCompletion) with their tests (edition-writer.test.ts: C1 to C4). The topics themselves are edition-picks.ts.
// ---- the edition, written from the choices ----

export type Edition = { title: string; intro: string; writingLines: string[]; sections: { heading: string; lines: string[] }[]; failFirst?: boolean };
export const editionLines = (e: Edition): string[] => e.sections.flatMap((s) => [s.heading, ...s.lines]);
/** The edition on this phone, from the choices alone: deterministic, no network (E6). The computer's desk is not asked. */
export function localEdition(topics: string[]): Edition {
  return { title: 'My edition', intro: 'This is your edition. Change anything by asking.', writingLines: ['Putting your edition together'], sections: [{ heading: 'What I follow', lines: topics.length === 0 ? ['The whole paper, until I say otherwise.'] : topics }] };
}
export type Writer = (topics: string[], progress: (line: string) => void, signal: AbortSignal) => Promise<Edition>;
export const localWriter: Writer = async (topics, progress) => { progress('Putting your edition together'); return localEdition(topics); };

export type Outcome = { kind: 'written'; edition: Edition } | { kind: 'failed'; line: string } | { kind: 'timedOut' };
export const DEADLINE_SECONDS = 20;
export const FAILED_LINE = "I couldn't write your edition just now. Your topics are saved.";
export const TIMED_OUT_LINE = 'Writing your edition is taking too long. Your topics are saved.';

/** Finishing always ends: the edition, a failure, or the deadline (the writer's signal is aborted). */
export async function complete(topics: string[], writer: Writer, progress: (line: string) => void = () => undefined, deadlineSeconds = DEADLINE_SECONDS): Promise<Outcome> {
  const ac = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<Outcome>((res) => { timer = setTimeout(() => res({ kind: 'timedOut' }), Math.max(deadlineSeconds, 0) * 1000); });
  const run = writer(topics, progress, ac.signal).then<Outcome, Outcome>((edition) => ({ kind: 'written', edition }), (e) => (ac.signal.aborted ? { kind: 'timedOut' } : { kind: 'failed', line: FAILED_LINE }));
  const first = await Promise.race([run, timeout]);
  if (timer) clearTimeout(timer);
  ac.abort();
  return first;
}
/** The general edition: what "Continue anyway" writes. It needs nothing from the network and cannot fail. */
export const generalEdition = (): Edition => localEdition([]);
