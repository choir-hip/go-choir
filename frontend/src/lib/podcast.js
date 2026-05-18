export function stripMarkup(value) {
  return String(value || '')
    .replace(/<!\[CDATA\[([\s\S]*?)\]\]>/g, '$1')
    .replace(/<[^>]+>/g, '')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;|&apos;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .trim();
}

export function excerpt(value, limit = 260) {
  const clean = stripMarkup(value).replace(/\s+/g, ' ').trim();
  if (clean.length <= limit) return clean;
  return `${clean.slice(0, limit - 1).trim()}...`;
}

function textFromFirst(parent, tagName) {
  return parent.getElementsByTagName(tagName)[0]?.textContent?.trim() || '';
}

function firstTagText(source, tagName) {
  const match = new RegExp(`<${tagName}(?:\\s[^>]*)?>([\\s\\S]*?)<\\/${tagName}>`, 'i').exec(source);
  return stripMarkup(match?.[1] || '');
}

function firstAttribute(source, tagName, attrName) {
  const tag = new RegExp(`<${tagName}\\b[^>]*>`, 'i').exec(source)?.[0] || '';
  const attr = new RegExp(`${attrName}=["']([^"']+)["']`, 'i').exec(tag);
  return stripMarkup(attr?.[1] || '');
}

export function stableId(value) {
  let hash = 5381;
  for (const char of String(value || '')) {
    hash = ((hash << 5) + hash) ^ char.charCodeAt(0);
  }
  return Math.abs(hash >>> 0).toString(36);
}

function normalizeEpisode(episode, index) {
  const key = episode.guid || episode.audioUrl || episode.link || `${episode.title}:${index}`;
  return {
    id: `episode-${stableId(key)}`,
    title: stripMarkup(episode.title) || 'Untitled episode',
    description: excerpt(episode.description, 420),
    publishedAt: stripMarkup(episode.publishedAt),
    duration: stripMarkup(episode.duration),
    guid: stripMarkup(episode.guid),
    link: stripMarkup(episode.link),
    audioUrl: stripMarkup(episode.audioUrl),
  };
}

function parsePodcastEpisodesLoosely(xmlText) {
  return Array.from(String(xmlText || '').matchAll(/<item\b[\s\S]*?<\/item>/gi))
    .map((match, index) => {
      const source = match[0];
      return normalizeEpisode({
        title: firstTagText(source, 'title') || 'Untitled episode',
        description: firstTagText(source, 'description'),
        publishedAt: firstTagText(source, 'pubDate'),
        duration: firstTagText(source, 'itunes:duration') || firstTagText(source, 'duration'),
        guid: firstTagText(source, 'guid'),
        link: firstTagText(source, 'link'),
        audioUrl: firstAttribute(source, 'enclosure', 'url') || firstAttribute(source, 'media:content', 'url'),
      }, index);
    })
    .filter((episode) => episode.title || episode.audioUrl);
}

function parsePodcastEpisodes(xmlText) {
  try {
    const parsed = new DOMParser().parseFromString(xmlText, 'application/xml');
    if (parsed.querySelector('parsererror')) return parsePodcastEpisodesLoosely(xmlText);
    return Array.from(parsed.getElementsByTagName('item')).map((episode, index) => {
      const enclosure = episode.getElementsByTagName('enclosure')[0];
      const mediaContent = episode.getElementsByTagName('media:content')[0];
      return normalizeEpisode({
        title: textFromFirst(episode, 'title') || 'Untitled episode',
        description: textFromFirst(episode, 'itunes:summary') || textFromFirst(episode, 'description'),
        publishedAt: textFromFirst(episode, 'pubDate'),
        duration: textFromFirst(episode, 'itunes:duration') || textFromFirst(episode, 'duration'),
        guid: textFromFirst(episode, 'guid'),
        link: textFromFirst(episode, 'link'),
        audioUrl: enclosure?.getAttribute('url') || mediaContent?.getAttribute('url') || '',
      }, index);
    }).filter((episode) => episode.title || episode.audioUrl);
  } catch (err) {
    return parsePodcastEpisodesLoosely(xmlText);
  }
}

function parsePodcastFeedLoosely(xmlText, contentItem) {
  const channel = /<channel\b[\s\S]*?<\/channel>/i.exec(String(xmlText || ''))?.[0] || String(xmlText || '');
  return {
    title: firstTagText(channel, 'title') || contentItem?.title || 'Podcast feed',
    description: excerpt(firstTagText(channel, 'description'), 520),
    link: firstTagText(channel, 'link') || contentItem?.canonical_url || contentItem?.source_url || '',
    episodes: parsePodcastEpisodesLoosely(xmlText),
  };
}

export function parsePodcastFeed(xmlText, contentItem = null) {
  try {
    const parsed = new DOMParser().parseFromString(xmlText, 'application/xml');
    if (parsed.querySelector('parsererror')) return parsePodcastFeedLoosely(xmlText, contentItem);
    const channel = parsed.getElementsByTagName('channel')[0] || parsed;
    return {
      title: textFromFirst(channel, 'title') || contentItem?.title || 'Podcast feed',
      description: excerpt(textFromFirst(channel, 'itunes:summary') || textFromFirst(channel, 'description'), 520),
      link: textFromFirst(channel, 'link') || contentItem?.canonical_url || contentItem?.source_url || '',
      episodes: parsePodcastEpisodes(xmlText),
    };
  } catch (err) {
    return parsePodcastFeedLoosely(xmlText, contentItem);
  }
}

export function buildListenPath(feed, contentItem) {
  const pathSource = contentItem?.source_url || feed?.link || '';
  const episodes = feed?.episodes || [];
  const playable = episodes.filter((episode) => episode.audioUrl);
  return {
    id: `listen-${stableId(`${contentItem?.content_id || ''}:${pathSource}:${feed?.title || ''}`)}`,
    title: feed?.title || contentItem?.title || 'Podcast feed',
    sourceUrl: pathSource,
    contentId: contentItem?.content_id || '',
    episodeCount: episodes.length,
    playableCount: playable.length,
    episodes: episodes.map((episode, index) => ({
      ...episode,
      position: index + 1,
    })),
  };
}

export function formatTime(value) {
  const totalSeconds = Math.max(0, Math.floor(Number(value) || 0));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
  }
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}
