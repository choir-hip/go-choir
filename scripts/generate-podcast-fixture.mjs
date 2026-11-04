#!/usr/bin/env node
// generate-podcast-fixture: writes scripts/fixtures/capsule-podcast/ from the
// synthetic set below. Everything is invented: fictional shows and authors, RSS
// written here, covers drawn by ImageMagick (200x200 jpg, distinct per show), a
// 20 s tone from ffmpeg. No third-party feed text or cover art is stored. The
// URLs have real-looking shapes on hosts this fixture owns, under the reserved
// .invalid TLD (RFC 2606), so they can never resolve. choir-shot serves them to a
// capsule's Chromium (docs/problems/capsule-cannot-load-podcast-artwork-2026-11-04.md).
//   node scripts/generate-podcast-fixture.mjs      (needs magick and ffmpeg on the host)
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const OUT = path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'capsule-podcast');
const FEEDS = 'https://feeds.podcast-fixture.choir.invalid';
const ART = 'https://art.podcast-fixture.choir.invalid';
const MEDIA = 'https://media.podcast-fixture.choir.invalid';
const SHOWS = [
  { slug: 'lantern-hour', title: 'The Lantern Hour', author: 'Marguerite Voss', cat: 'Society &amp; Culture', color: '#c2410c', accent: '#fde68a', shape: 'circle',
    about: 'Quiet conversations with people who keep small, old things running.',
    eps: ['The clockmaker next door', 'A harbor light, restored', 'Why the ferry bell rings twice'] },
  { slug: 'deep-field-notes', title: 'Deep Field Notes', author: 'Ibrahim Okafor-Lind', cat: 'Science', color: '#1e3a8a', accent: '#93c5fd', shape: 'stars',
    about: 'A weekly walk through one odd corner of astronomy, in plain words.',
    eps: ['Counting faint galaxies', 'The telescope that fit in a van', 'Dust, and what it hides'] },
  { slug: 'ledger-and-loaf', title: 'Ledger &amp; Loaf', author: 'Tamsin Perrault', cat: 'Business', color: '#166534', accent: '#bbf7d0', shape: 'bars',
    about: 'Small-business money stories, told from behind the counter.',
    eps: ['Pricing a croissant', 'The week the oven broke', 'Hiring your first cousin'] },
  { slug: 'night-bus-atlas', title: 'Night Bus Atlas', author: 'Odalys Reinhardt', cat: 'Arts', color: '#581c87', accent: '#f0abfc', shape: 'diagonal',
    about: 'Short audio postcards recorded on the last bus home, in made-up cities.',
    eps: ['Route 9 to nowhere', 'The driver who knew every dog', 'Rain on the upper deck'] },
  { slug: 'field-recordings-club', title: 'Field Recordings Club', author: 'Joaquim Beaumont', cat: 'Music', color: '#0f766e', accent: '#99f6e4', shape: 'waves',
    about: 'Listeners send sounds; we guess what they are, and are often wrong.',
    eps: ['A kettle or a train?', 'Frogs at the quarry', 'The sound of a library closing'] },
];

function cover(show, file) {
  const d = { circle: 'circle 100,100 100,38', stars: 'polygon 100,30 112,80 165,80 122,110 138,165 100,132 62,165 78,110 35,80 88,80',
    bars: 'rectangle 40,110 70,170 rectangle 85,70 115,170 rectangle 130,40 160,170', diagonal: 'polygon 0,200 200,60 200,110 0,200',
    waves: 'ellipse 100,100 80,22 0,360 ellipse 100,100 55,14 0,360' }[show.shape];
  const r = spawnSync('magick', ['-size', '200x200', `gradient:${show.color}-#000000`, '-fill', show.accent, '-draw', d, '-strip', '-quality', '70', path.join(OUT, file)]);
  if (r.status !== 0) throw new Error(`magick failed for ${show.slug}: ${r.stderr}`);
}

fs.rmSync(OUT, { recursive: true, force: true });
fs.mkdirSync(OUT, { recursive: true });
const manifest = { fixture: 'capsule-podcast', note: 'Synthetic shows, artwork and audio written by scripts/generate-podcast-fixture.mjs; fixture data, not live. Hosts end in .invalid.', clip: { file: 'clip.mp3', type: 'audio/mpeg' }, shows: [] };
SHOWS.forEach((show, i) => {
  const id = `${String(i + 1).padStart(8, '0')}-5a1e-4c0d-9f00-${String(i + 1).padStart(12, '0')}`;
  const feedUrl = `${FEEDS}/${show.slug}/rss`;
  const artwork = [
    `${ART}/images/${id}/3000x3000/${show.slug}-cover.jpg?aid=rss_feed`,
    `${ART}/images/${id}/600x600bb.jpg`,
    `${ART}/images/${id}/100x100bb.jpg`,
  ];
  const episodes = show.eps.map((_, n) => `${MEDIA}/e/${show.slug}/episodes/${n + 1}/audio/128/default.mp3?aid=rss_feed`);
  const items = show.eps.map((t, n) => `    <item>
      <title>${t}</title>
      <description>Episode ${n + 1} of ${show.title}: ${t.toLowerCase()}. A short fictional description written for the fixture.</description>
      <pubDate>${new Date(Date.UTC(2026, 9, 28 - 7 * n, 6, 0, 0)).toUTCString()}</pubDate>
      <guid isPermaLink="false">${show.slug}-ep-${n + 1}</guid>
      <enclosure url="${episodes[n].replace(/&/g, '&amp;')}" length="80370" type="audio/mpeg"/>
      <itunes:duration>00:00:20</itunes:duration>
      <itunes:episode>${show.eps.length - n}</itunes:episode>
      <itunes:image href="${artwork[0]}"/>
    </item>`).join('\n');
  const rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>${show.title}</title>
    <link>${FEEDS}/${show.slug}</link>
    <language>en-us</language>
    <description>${show.about} (Fictional show in a fixture; not live.)</description>
    <itunes:author>${show.author}</itunes:author>
    <itunes:category text="${show.cat}"/>
    <itunes:explicit>false</itunes:explicit>
    <itunes:image href="${artwork[0]}"/>
    <image><url>${artwork[0]}</url><title>${show.title}</title><link>${FEEDS}/${show.slug}</link></image>
${items}
  </channel>
</rss>
`;
  fs.writeFileSync(path.join(OUT, `${show.slug}.rss`), rss);
  cover(show, `${show.slug}-1.jpg`);
  manifest.shows.push({ slug: show.slug, title: show.title.replace(/&amp;/g, '&'), author: show.author, feed_url: feedUrl, feed_file: `${show.slug}.rss`, artwork: artwork.map((url) => ({ url, file: `${show.slug}-1.jpg` })), episodes });
});
const f = spawnSync('ffmpeg', ['-y', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=20', '-c:a', 'libmp3lame', '-b:a', '32k', path.join(OUT, 'clip.mp3')], { stdio: 'ignore' });
if (f.status !== 0) throw new Error('ffmpeg failed');
fs.writeFileSync(path.join(OUT, 'manifest.json'), JSON.stringify(manifest, null, 1) + '\n');
console.log(`${manifest.shows.length} shows in ${OUT}`);
