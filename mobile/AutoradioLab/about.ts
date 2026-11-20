// The About page's build rows: the commit this app was built from (baked in by scripts/gen-build-info.sh into build-info.gen.ts)
// and the commit the server reports (GET /health, public, no key). Replaces the Swift `readServerBuild`, which looked for a
// top-level `commit` in /health; the route nests it as `build.commit` (and `platform.deployed_commit`), so it always read a dash.
// Failure list: about.test.ts (B1 to B4).

import { getJSON } from './api.ts';
import { BUILD_INFO } from './build-info.gen.ts';

export type BuildInfo = { commit: string; dirty: boolean; committed: string };
export type AboutBuild = { commit: string; server: string };

const SHORT = 9;
const HEX = /^[0-9a-f]{7,64}$/;

/** "9char[-dirty] · YYYY-MM-DD HH:MM UTC", or `unknown` when the build knew no commit. */
export function appCommitLine(info: BuildInfo | null | undefined): string {
  const o = info as Partial<BuildInfo> | null | undefined;
  const sha = o && typeof o.commit === 'string' && HEX.test(o.commit) ? o.commit.slice(0, SHORT) : '';
  if (sha === '') return 'unknown';
  const head = o?.dirty === true ? `${sha}-dirty` : sha;
  const when = typeof o?.committed === 'string' && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/.test(o.committed) ? o.committed : '';
  return when === '' ? head : `${head} · ${when} UTC`;
}

const str = (v: unknown): string => (typeof v === 'string' ? v : '');

/** The server's commit line from a /health body: `build.commit`, else `platform.deployed_commit`; '' when it reports none. */
export function serverCommitLine(body: unknown): string {
  const o = (body && typeof body === 'object' ? body : {}) as Record<string, Record<string, unknown> | undefined>;
  const build = o.build ?? {}; const platform = o.platform ?? {};
  const full = HEX.test(str(build.commit)) ? str(build.commit) : HEX.test(str(platform.deployed_commit)) ? str(platform.deployed_commit) : '';
  if (full === '') return '';
  const at = str(build.deployed_at) || str(platform.deployed_at);
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})/.exec(at);
  return m ? `${full.slice(0, SHORT)} · ${m[1]} ${m[2]} UTC` : full.slice(0, SHORT);
}

export async function aboutBuild(_args: { tick: number }): Promise<AboutBuild> {
  const commit = appCommitLine(BUILD_INFO);
  try {
    const r = await getJSON('/health');
    return { commit, server: serverCommitLine(r.body) || 'not reported' };
  } catch {
    return { commit, server: 'not reported' };
  }
}
