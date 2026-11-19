// A publisher's RSS feed as a show and its episodes. Moved from Swift (RSSFeedParser, RSSDelegate); the failure list of the Swift file is kept:
//  1. An enclosure with no type, "audio/mpeg; codecs", several enclosures (the first audio or video one wins), a non-http(s) URL (dropped) or none.
//  2. itunes:duration as HH:MM:SS, MM:SS, seconds, seconds with a fraction, empty or nonsense (no duration, never zero).
//  3. Notes in CDATA, in escaped HTML, with entities and tags: plain paragraphs, never markup.
//  4. A date in RFC 822 with or without weekday or seconds, a zone name, or ISO 8601; a date no format reads: the episode has none.
//  5. An episode with no guid: its enclosure URL is its id, else its title and date. Two items with one id: listed once.
//  6. A feed that is HTML, empty, or has no channel title and no items: an error that says so, never an empty show.
//  7. A feed with thousands of items: the newest 300 are kept (feed order), the parse stops there.
//  8. A hostile feed: one element's text stops growing at 200,000 characters; elements nested deeper than 64 are ignored with their text.

import { readXML } from './xml.ts';
import { date, duration, https, plain, show as makeShow, webURL, type Episode, type Feed } from './podcast-model.ts';

export const MAX_EPISODES = 300;
const MAX_TEXT = 200_000;
const MAX_DEPTH = 64;
const INLINE = new Set(['a', 'b', 'i', 'u', 'em', 'strong', 'span', 'code', 'small', 'sup', 'sub', 'abbr', 'cite', 'mark', 's']);

export class NotAFeed extends Error {}

type Item = {
  title: string; guid: string; pubDate: string; duration: string; link: string; image: string; description: string; content: string; summary: string;
  enclosures: { url: string; type: string }[]; media: { url: string; type: string }[];
};
const newItem = (): Item => ({ title: '', guid: '', pubDate: '', duration: '', link: '', image: '', description: '', content: '', summary: '', enclosures: [], media: [] });

function episodeOf(it: Item): Episode {
  const audioish = (ty: string): boolean => ty === '' || ty.startsWith('audio') || ty.startsWith('video') || ty.includes('mpeg') || ty.includes('mp4') || ty.includes('ogg') || ty.includes('aac');
  const all = [...it.enclosures, ...it.media];
  const pick = all.find((e) => webURL(e.url) !== '' && audioish(e.type) && !e.type.startsWith('image'));
  const url = pick ? webURL(pick.url) : '';
  const type = pick ? pick.type : '';
  let notes = '';
  for (const n of [it.description, it.content, it.summary].map((x) => plain(x))) if (n.length > notes.length) notes = n;
  const published = date(it.pubDate);
  const title = it.title === '' ? 'Untitled episode' : plain(it.title, 300).replace(/\n/g, ' ');
  const id = it.guid !== '' ? it.guid : url !== '' ? url : `${title}@${published === null ? '' : Math.trunc(published)}`;
  return {
    id, title, published, duration: duration(it.duration), enclosureURL: url, enclosureType: type, notes,
    artworkURL: https(webURL(it.image)), link: webURL(it.link), playable: url !== '',
  };
}

/** Parses a feed. Throws NotAFeed with a sentence the page can show. */
/** The most of a feed's text that is parsed. A page's turn is 100 ms of CPU and the device parsed a 2.2 MB feed in 378 ms and 103 ms at 300 000 and 102 ms at 150 000 (real runs 3, 5 and 14: `podcastHome` was refused as an
 * Overrun and the page drew nothing). Feeds list the newest episodes first, so a prefix cut at the last whole `</item>` keeps the newest; the rest is not read. */
export const MAX_PARSE = 80_000;

export function parseFeed(fullText: string, feedURL: string, fetchedAt: number): Feed {
  let text = fullText;
  if (text.length > MAX_PARSE) { const cut = text.lastIndexOf('</item>', MAX_PARSE); text = text.slice(0, cut > 0 ? cut + 7 : MAX_PARSE) + '</channel></rss>'; }
  const head = text.slice(0, 400).toLowerCase();
  if (head.includes('<html') || head.includes('<!doctype html')) throw new NotAFeed('That address is a web page, not a podcast feed.');

  const path: string[] = [];
  const buffers: string[] = [];
  let tooDeep = 0;
  let item: Item | null = null;
  let title = '', author = '', artwork = '', summary = '', channelDescription = '';
  const episodes: Episode[] = [];

  const add = (s: string): void => {
    const last = buffers.length - 1;
    if (last < 0 || buffers[last].length >= MAX_TEXT) return;
    buffers[last] += s;
  };

  readXML(text, {
    start(name, a) {
      if (path.length >= MAX_DEPTH) { tooDeep++; return; }
      path.push(name); buffers.push('');
      if (name === 'item') item = newItem();
      if (name === 'enclosure' && item && a.url !== undefined) item.enclosures.push({ url: a.url, type: (a.type ?? '').toLowerCase() });
      if (name === 'media:content' && item && a.url !== undefined) item.media.push({ url: a.url, type: (a.type ?? a.medium ?? '').toLowerCase() });
      if (name === 'itunes:image' && a.href !== undefined) { if (item) item.image = a.href; else if (artwork === '') artwork = a.href; }
    },
    text(s) { if (tooDeep === 0) add(s); },
    cdata(s) { if (tooDeep === 0) add(s); },
    end(name) {
      if (tooDeep > 0) { tooDeep--; return; }
      const raw = buffers.pop() ?? '';
      const t = raw.trim();
      const parent = path.length >= 2 ? path[path.length - 2] : '';
      const grand = path.length >= 3 ? path[path.length - 3] : '';
      let stop = false;
      if (name === 'item' && item) {
        const it = item; item = null;
        episodes.push(episodeOf(it));
        if (episodes.length >= MAX_EPISODES) stop = true;
      } else if (item && parent === 'item') {
        switch (name) {
          case 'title': if (item.title === '') item.title = t; break;
          case 'guid': item.guid = t; break;
          case 'pubDate': case 'dc:date': if (item.pubDate === '') item.pubDate = t; break;
          case 'itunes:duration': item.duration = t; break;
          case 'link': if (item.link === '') item.link = t; break;
          case 'description': item.description = t; break;
          case 'content:encoded': item.content = t; break;
          case 'itunes:summary': item.summary = t; break;
          case 'itunes:subtitle': if (item.summary === '') item.summary = t; break;
        }
      } else if (!item) {
        if (parent === 'channel') {
          switch (name) {
            case 'title': if (title === '') title = t; break;
            case 'itunes:author': author = t; break;
            case 'description': channelDescription = t; break;
            case 'itunes:summary': summary = t; break;
          }
        } else if (name === 'url' && parent === 'image' && grand === 'channel' && artwork === '') artwork = t;
        if (name === 'channel' && summary === '') summary = channelDescription;
      }
      path.pop();
      if (buffers.length > 0 && name !== 'item') {
        if (name === 'br') add('\n');
        else if (INLINE.has(name.toLowerCase())) add(raw);
        else if (t !== '') {
          const last = buffers.length - 1;
          const sep = buffers[last] === '' || buffers[last].endsWith('\n') ? '' : '\n\n';
          add(sep + t + '\n\n');
        }
      }
      if (stop) return false;
    },
  });

  if (title === '' && episodes.length === 0) throw new NotAFeed('That address is not a podcast feed.');
  const seen = new Set<string>();
  const eps = episodes.filter((e) => (seen.has(e.id) ? false : (seen.add(e.id), true)));
  return {
    show: makeShow(feedURL, title === '' ? feedURL : title, { author, artworkURL: https(webURL(artwork)), summary: plain(summary, 2000) }),
    episodes: eps,
    fetchedAt,
  };
}
