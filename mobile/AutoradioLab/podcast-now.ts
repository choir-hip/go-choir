// What the player is playing, when it is a podcast: the episode's key, whether it is a clip, where the clip starts, and the deck's seconds and duration. It was the native source `podcastNow`
// (AutoradioModule+Podcasts.swift, answered from the Swift deck and the mirror of the run sheet's on-air entry, touched once a second by a native timer). Shared code has both: the run sheet's
// on-air entry and the deck's state. The pages read it with the radio's pulse (members-podcasts.contract), which moves once a second while something sounds in front.
import type { Storage as Store } from './app.contract.d.ts';
import type { NativeReader } from './api.ts';
import { audioSpec } from './runsheet-entries.ts';
import { parseEpisodeKey } from './podcast-model.ts';
import { getRadio } from './radio-bridge.ts';
import type { Entry } from './runsheet-model.ts';
import type { DeckState } from './deck.ts';

export type PodNow = { episodeId: string; clip: boolean; start: number; elapsed: number; duration: number };
const NONE: PodNow = { episodeId: '', clip: false, start: 0, elapsed: 0, duration: 0 };

/** The answer for the entry on air and the deck's state. Empty unless the deck holds that entry and it is a podcast episode (its id parses as an episode key). */
export function podcastNowOf(onAir: Entry | undefined, deck: DeckState): PodNow {
  if (!onAir || deck.item !== onAir.id || deck.why === 'stopped' || deck.why === 'empty') return NONE;
  const a = audioSpec(onAir);
  const id = a?.episode_id;
  if (!a || id === undefined || parseEpisodeKey(id) === null) return NONE;
  return { episodeId: id, clip: a.end !== undefined, start: a.start ?? 0, elapsed: deck.seconds, duration: deck.duration };
}

/** The source: the pulse's arguments only make it asked again (a data source may not read the clock). Never throws. */
export async function podcastNow(storage: Store | undefined, native: NativeReader | undefined): Promise<PodNow> {
  try {
    const r = await getRadio(storage as never, native, false);
    const e = r.rt.store.onAir();
    if (!e) return NONE;
    return podcastNowOf(e, await r.rt.deckNow());
  } catch { return NONE; }
}
