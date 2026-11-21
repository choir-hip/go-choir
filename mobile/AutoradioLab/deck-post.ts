// The deck journal's poster over the signed-in seam (api.ts `authedJSON`): POST /api/autoradio/sheet/ops on the listener's computer.
// A 401 (signed out) and a transport error leave the journal whole, as offline: nothing is acknowledged, so nothing is lost, and the next call
// posts it. The answers are read as the Swift RouteSheetPoster read them (deck-ops.ts outcomeFromHTTP).
import { HTTPError, SignedOutError, authedJSON, currentWorld, type Authed, type Reply } from './api.ts';
import { outcomeFromHTTP, type PostOutcome, type Poster } from './deck-ops.ts';

export const OPS_PATH = '/api/autoradio/sheet/ops';

/** The poster over the signed-in seam. A batch captured in one world posts for THAT world: pass it (the run sheet's poster passes the world of the batch it holds); without one the requests belong to the world current at each post. */
export function routePoster(request: (path: string, opts?: Authed) => Promise<Reply> = (path, opts) => authedJSON(path, { ...opts, world: currentWorld() })): Poster {
  return {
    async post(ops, epoch): Promise<PostOutcome> {
      try {
        const body: { ops: unknown; sheet_epoch?: number } = { ops };
        if (epoch > 0) body.sheet_epoch = epoch;
        const r = await request(OPS_PATH, { method: 'POST', body });
        return outcomeFromHTTP(200, r.body);
      } catch (e) {
        if (e instanceof HTTPError) return outcomeFromHTTP(e.status, null);
        if (e instanceof SignedOutError) return { kind: 'offline' };
        return { kind: 'offline' };
      }
    },
  };
}
