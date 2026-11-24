// What shared code hands the voice about the radio. The run sheet is shared code's; the voice turn and the take recorder are native and used to decode the sheet
// themselves (RunSheet.swift, RadioPage.swift: deleted). Now shared code pushes what each of them reads, with `radioSet` (radio-bridge.ts), and native decodes nothing:
// `facts` for the voice's retrieval (the entry on air and the entries ahead) and `responds` for the take recorder (the story on air when a take begins).
import { openAddress, builtOn } from './radio-model.ts';
import { displayTitle } from './runsheet-entries.ts';
import type { Entry } from './runsheet-model.ts';

export type VoiceFact = { id: string; title: string; text: string; state: 'on_air' | 'ahead'; ref: string };

export function voiceFacts(entries: Entry[]): VoiceFact[] {
  const out: VoiceFact[] = [];
  for (const e of entries) {
    if (e.state !== 'on_air' && e.state !== 'ahead') continue;
    out.push({ id: e.id, title: e.title, text: e.text ?? '', state: e.state, ref: e.source?.ref ?? '' });
  }
  return out;
}

/** The story an entry on air is about: its page, else the first story it cites. Nothing for an entry with none, a take or a transition (as the Swift `TakeResponds.from` did: an answer or a result that cites a story still gives it). */
export function respondsOf(e: Entry | undefined): { route: string; title: string } | undefined {
  if (!e || e.kind === 'take' || e.kind === 'transition') return undefined;
  const addr = openAddress(e);
  if (addr.startsWith('pub:')) return { route: addr.slice(4), title: displayTitle(e) };
  const c = builtOn(e).find((b) => b.address.startsWith('pub:'));
  return c ? { route: c.address.slice(4), title: c.title } : undefined;
}

/** The page the entry on air is reading (the reader's follow-along matches the spoken sentence against it): a story's or a Texture's address; nothing for the station's own words. */
export function airAddressOf(e: Entry | undefined): string {
  if (!e || e.kind === 'opening' || e.kind === 'transition' || e.kind === 'answer' || e.kind === 'result' || e.kind === 'take') return '';
  const a = openAddress(e);
  if (a.startsWith('pub:')) return a;
  const r = e.source?.ref ?? '';
  if (r.startsWith('texture:')) return r.split('@')[0];      // a revision of the document is read in the page the document's own address holds
  return '';
}
