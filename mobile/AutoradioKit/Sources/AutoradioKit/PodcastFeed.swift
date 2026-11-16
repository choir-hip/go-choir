import Foundation

// The podcast app's data: shows and episodes parsed from a publisher's RSS feed on the phone, and the public directory's answers
// (Apple's search and top charts, the same public JSON API the server's search route calls). No audio and no feed text is stored in
// the repo; audio streams from the publisher's enclosure URL and is never re-hosted.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. An enclosure with no type, a type of "audio/mpeg; codecs", several enclosures (the first audio or video one wins), a
//     non-http(s) URL (dropped: it could never stream) or none at all (the episode is listed and cannot be played).
//  2. itunes:duration as "HH:MM:SS", "MM:SS", plain seconds, seconds with a fraction, empty, or nonsense (no duration, never zero).
//  3. Notes in CDATA, in escaped HTML, with entities and tags: shown as plain paragraphs, never as markup.
//  4. A date in RFC 822 with or without weekday or seconds, a zone name, or ISO 8601; a date no format reads: the episode has none.
//  5. An episode with no guid: its enclosure URL is its id, else its title and date. Two items with one id: listed once.
//  6. A feed that is HTML, empty, or has no channel title and no items: an error that says so, never an empty show.
//  7. A feed with thousands of items: the newest 300 are kept (feed order), the parse stops there.
//  8. Directory results with no feed URL or no title: skipped; the same feed twice: once; artwork over http: upgraded to https.

public struct PodcastShow: Codable, Equatable, Sendable {
    public var feedURL: String
    public var title: String
    public var author: String
    public var artworkURL: String
    public var summary: String
    public init(feedURL: String, title: String, author: String = "", artworkURL: String = "", summary: String = "") {
        self.feedURL = feedURL; self.title = title; self.author = author; self.artworkURL = artworkURL; self.summary = summary
    }
}

public struct PodcastEpisode: Codable, Equatable, Sendable {
    /// The feed's guid, else the enclosure URL, else the title and date. Stable across refreshes.
    public var id: String
    public var title: String
    /// Seconds since 1970; nil when the feed's date could not be read.
    public var published: Double?
    public var duration: Double?
    public var enclosureURL: String
    public var enclosureType: String
    public var notes: String
    public var artworkURL: String
    public var link: String
    public var playable: Bool { !enclosureURL.isEmpty }
    public init(id: String, title: String, published: Double? = nil, duration: Double? = nil, enclosureURL: String = "", enclosureType: String = "",
                notes: String = "", artworkURL: String = "", link: String = "") {
        self.id = id; self.title = title; self.published = published; self.duration = duration; self.enclosureURL = enclosureURL
        self.enclosureType = enclosureType; self.notes = notes; self.artworkURL = artworkURL; self.link = link
    }
}

public struct PodcastFeed: Codable, Equatable, Sendable {
    public var show: PodcastShow
    public var episodes: [PodcastEpisode]
    public var fetchedAt: Double
    public init(show: PodcastShow, episodes: [PodcastEpisode], fetchedAt: Double = 0) { self.show = show; self.episodes = episodes; self.fetchedAt = fetchedAt }
}

public enum PodcastError: Error, CustomStringConvertible, Equatable, Sendable {
    case notAFeed(String)
    case tooBig
    case badURL
    public var description: String {
        switch self {
        case .notAFeed(let m): return m
        case .tooBig: return "That feed is too large to open."
        case .badURL: return "That is not a web address."
        }
    }
}

// MARK: Text, durations, dates

public enum PodcastText {
    /// Notes as plain paragraphs: block tags become line breaks, list items a bullet, other tags go, entities are read.
    public static func plain(_ html: String, limit: Int = 8000) -> String {
        var s = html
        for (pattern, repl) in [("(?i)<br\\s*/?>", "\n"), ("(?i)</(p|div|h[1-6]|ul|ol|blockquote)>", "\n\n"), ("(?i)</li>", "\n"), ("(?i)<li[^>]*>", "• ")] {
            s = s.replacingOccurrences(of: pattern, with: repl, options: .regularExpression)
        }
        s = s.replacingOccurrences(of: "<[^>]+>", with: "", options: .regularExpression)
        s = decodeEntities(s)
        let lines = s.components(separatedBy: .newlines).map { $0.split(whereSeparator: { $0 == " " || $0 == "\t" || $0 == "\u{00A0}" }).joined(separator: " ") }
        var out: [String] = [], blank = false
        for l in lines {
            if l.isEmpty { if !blank, !out.isEmpty { out.append("") }; blank = true } else { out.append(l); blank = false }
        }
        while out.last == "" { out.removeLast() }
        let joined = out.joined(separator: "\n")
        return joined.count > limit ? String(joined.prefix(limit - 1)) + "…" : joined
    }

    static func decodeEntities(_ s: String) -> String {
        guard s.contains("&") else { return s }
        var out = "", i = s.startIndex
        let named = ["amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'", "nbsp": " ", "ndash": "–", "mdash": "—", "hellip": "…", "rsquo": "’", "lsquo": "‘", "ldquo": "“", "rdquo": "”"]
        while i < s.endIndex {
            if s[i] == "&", let semi = s[i...].prefix(12).firstIndex(of: ";") {
                let body = String(s[s.index(after: i)..<semi])
                var rep: String?
                if body.hasPrefix("#x") || body.hasPrefix("#X") { rep = UInt32(body.dropFirst(2), radix: 16).flatMap(Unicode.Scalar.init).map { String(Character($0)) } }
                else if body.hasPrefix("#") { rep = UInt32(body.dropFirst(1)).flatMap(Unicode.Scalar.init).map { String(Character($0)) } }
                else { rep = named[body] }
                if let r = rep { out += r; i = s.index(after: semi); continue }
            }
            out.append(s[i]); i = s.index(after: i)
        }
        return out
    }

    /// "1:02:03", "45:10", "2710", "2710.5" are seconds; anything else is nil. Zero is nil (a feed's "0" means unknown).
    public static func duration(_ raw: String) -> Double? {
        let t = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty else { return nil }
        let parts = t.split(separator: ":", omittingEmptySubsequences: false)
        guard parts.count <= 3 else { return nil }
        var total = 0.0
        for p in parts {
            guard let v = Double(p), v >= 0, v.isFinite else { return nil }
            total = total * 60 + v
        }
        return total > 0 ? total : nil
    }

    /// "45 min", "1 h 12 min", "50 s".
    public static func durationLabel(_ seconds: Double?) -> String {
        guard let s = seconds, s > 0 else { return "" }
        let m = Int((s / 60).rounded())
        if s < 60 { return "\(Int(s)) s" }
        if m < 60 { return "\(m) min" }
        return m % 60 == 0 ? "\(m / 60) h" : "\(m / 60) h \(m % 60) min"
    }

    /// A playhead: "12:40" or "1:02:03".
    public static func clock(_ seconds: Double) -> String {
        let t = max(0, Int(seconds.rounded(.down)))
        return t >= 3600 ? String(format: "%d:%02d:%02d", t / 3600, (t % 3600) / 60, t % 60) : String(format: "%d:%02d", t / 60, t % 60)
    }

    static let dateFormats = ["EEE, dd MMM yyyy HH:mm:ss Z", "EEE, dd MMM yyyy HH:mm:ss zzz", "EEE, dd MMM yyyy HH:mm Z", "EEE, dd MMM yyyy HH:mm zzz",
                              "dd MMM yyyy HH:mm:ss Z", "dd MMM yyyy HH:mm:ss zzz", "dd MMM yyyy HH:mm Z", "dd MMM yyyy HH:mm zzz", "EEE, d MMM yyyy HH:mm:ss Z", "EEE, d MMM yyyy HH:mm:ss zzz",
                              "yyyy-MM-dd HH:mm:ss Z", "yyyy-MM-dd"]
    /// RFC 822 (the feed standard, with or without weekday, seconds or numeric zone) and ISO 8601.
    public static func date(_ raw: String) -> Double? {
        let t = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty else { return nil }
        let iso = ISO8601DateFormatter()
        for opts: ISO8601DateFormatter.Options in [[.withInternetDateTime], [.withInternetDateTime, .withFractionalSeconds]] {
            iso.formatOptions = opts
            if let d = iso.date(from: t) { return d.timeIntervalSince1970 }
        }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX"); f.timeZone = TimeZone(identifier: "UTC")
        for fmt in dateFormats { f.dateFormat = fmt; if let d = f.date(from: t) { return d.timeIntervalSince1970 } }
        return nil
    }

    /// "Today", "Yesterday", "3 days ago", then "Nov 3", then "Nov 3, 2025" for another year.
    public static func relative(_ time: Double?, now: Double = Date().timeIntervalSince1970) -> String {
        guard let t = time else { return "" }
        let cal = Calendar(identifier: .gregorian)
        let d = Date(timeIntervalSince1970: t), n = Date(timeIntervalSince1970: now)
        let days = cal.dateComponents([.day], from: cal.startOfDay(for: d), to: cal.startOfDay(for: n)).day ?? 0
        if days == 0 { return "Today" }
        if days == 1 { return "Yesterday" }
        if days > 1, days < 7 { return "\(days) days ago" }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX"); f.timeZone = TimeZone.current
        f.dateFormat = cal.component(.year, from: d) == cal.component(.year, from: n) ? "MMM d" : "MMM d, yyyy"
        return f.string(from: d)
    }

    static func https(_ s: String) -> String {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        return t.lowercased().hasPrefix("http://") ? "https://" + t.dropFirst(7) : t
    }
    static func webURL(_ s: String) -> String {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let u = URL(string: t), let sc = u.scheme?.lowercased(), sc == "http" || sc == "https", u.host?.isEmpty == false else { return "" }
        return t
    }
}

// MARK: RSS

public enum RSSFeedParser {
    public static let maxEpisodes = 300

    public static func parse(_ data: Data, feedURL: String, fetchedAt: Double = Date().timeIntervalSince1970) throws -> PodcastFeed {
        let head = String(decoding: data.prefix(400), as: UTF8.self).lowercased()
        if head.contains("<html") || head.contains("<!doctype html") { throw PodcastError.notAFeed("That address is a web page, not a podcast feed.") }
        let d = RSSDelegate()
        let p = XMLParser(data: data)
        p.delegate = d
        p.shouldProcessNamespaces = false
        p.shouldResolveExternalEntities = false
        _ = p.parse()
        if d.aborted == false, let e = p.parserError, d.episodes.isEmpty { throw PodcastError.notAFeed("That feed could not be read (\(e.localizedDescription)).") }
        guard !d.title.isEmpty || !d.episodes.isEmpty else { throw PodcastError.notAFeed("That address is not a podcast feed.") }
        var seen = Set<String>(), eps: [PodcastEpisode] = []
        for e in d.episodes where seen.insert(e.id).inserted { eps.append(e) }
        let show = PodcastShow(feedURL: feedURL, title: d.title.isEmpty ? feedURL : d.title, author: d.author,
                               artworkURL: PodcastText.https(PodcastText.webURL(d.artwork)), summary: PodcastText.plain(d.summary, limit: 2000))
        return PodcastFeed(show: show, episodes: eps, fetchedAt: fetchedAt)
    }
}

private final class RSSDelegate: NSObject, XMLParserDelegate {
    struct Item {
        var title = "", guid = "", pubDate = "", duration = "", link = "", image = ""
        var description = "", content = "", summary = ""
        var enclosures: [(url: String, type: String)] = []
        var media: [(url: String, type: String)] = []
    }
    var path: [String] = []
    var text = ""
    var item: Item?
    var title = "", author = "", artwork = "", summary = "", channelDescription = ""
    var episodes: [PodcastEpisode] = []
    var aborted = false

    func parser(_ parser: XMLParser, didStartElement name: String, namespaceURI: String?, qualifiedName: String?, attributes a: [String: String]) {
        path.append(name); text = ""
        if name == "item" { item = Item() }
        if name == "enclosure", item != nil, let u = a["url"] { item?.enclosures.append((u, (a["type"] ?? "").lowercased())) }
        if name == "media:content", item != nil, let u = a["url"] { item?.media.append((u, (a["type"] ?? a["medium"] ?? "").lowercased())) }
        if name == "itunes:image", let h = a["href"] { if item != nil { item?.image = h } else if artwork.isEmpty { artwork = h } }
    }
    func parser(_ parser: XMLParser, foundCharacters s: String) { text += s }
    func parser(_ parser: XMLParser, foundCDATA data: Data) { text += String(decoding: data, as: UTF8.self) }

    func parser(_ parser: XMLParser, didEndElement name: String, namespaceURI: String?, qualifiedName: String?) {
        defer { path.removeLast(); text = "" }
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        let parent = path.count >= 2 ? path[path.count - 2] : ""
        if name == "item", let it = item {
            item = nil
            episodes.append(Self.episode(it))
            if episodes.count >= RSSFeedParser.maxEpisodes { aborted = true; parser.abortParsing() }
            return
        }
        if item != nil, parent == "item" {
            switch name {
            case "title": if item?.title.isEmpty == true { item?.title = t }
            case "guid": item?.guid = t
            case "pubDate", "dc:date": if item?.pubDate.isEmpty == true { item?.pubDate = t }
            case "itunes:duration": item?.duration = t
            case "link": if item?.link.isEmpty == true { item?.link = t }
            case "description": item?.description = t
            case "content:encoded": item?.content = t
            case "itunes:summary": item?.summary = t
            case "itunes:subtitle": if item?.summary.isEmpty == true { item?.summary = t }
            default: break
            }
            return
        }
        guard item == nil else { return }
        if parent == "channel" {
            switch name {
            case "title": if title.isEmpty { title = t }
            case "itunes:author": author = t
            case "description": channelDescription = t
            case "itunes:summary": summary = t
            default: break
            }
        } else if name == "url", parent == "image", path.count >= 3, path[path.count - 3] == "channel", artwork.isEmpty { artwork = t }
        if name == "channel", summary.isEmpty { summary = channelDescription }
    }

    static func episode(_ it: Item) -> PodcastEpisode {
        func audioish(_ ty: String) -> Bool { ty.isEmpty || ty.hasPrefix("audio") || ty.hasPrefix("video") || ty.contains("mpeg") || ty.contains("mp4") || ty.contains("ogg") || ty.contains("aac") }
        let all = it.enclosures + it.media
        let pick = all.first { PodcastText.webURL($0.url) != "" && audioish($0.type) && !($0.type == "image" || $0.type.hasPrefix("image")) }
        let url = pick.map { PodcastText.webURL($0.url) } ?? "", type = pick?.type ?? ""
        let notes = [it.description, it.content, it.summary].map { PodcastText.plain($0) }.max { $0.count < $1.count } ?? ""
        let date = PodcastText.date(it.pubDate)
        let title = it.title.isEmpty ? "Untitled episode" : PodcastText.plain(it.title, limit: 300).replacingOccurrences(of: "\n", with: " ")
        let id = !it.guid.isEmpty ? it.guid : (!url.isEmpty ? url : "\(title)@\(date.map { String(Int($0)) } ?? "")")
        return PodcastEpisode(id: id, title: title, published: date, duration: PodcastText.duration(it.duration), enclosureURL: url, enclosureType: type,
                              notes: notes, artworkURL: PodcastText.https(PodcastText.webURL(it.image)), link: PodcastText.webURL(it.link))
    }
}

// MARK: The public directory

public enum PodcastDirectory {
    public static func searchURL(_ query: String, limit: Int = 15) -> URL? {
        var c = URLComponents(string: "https://itunes.apple.com/search")
        c?.queryItems = [URLQueryItem(name: "media", value: "podcast"), URLQueryItem(name: "entity", value: "podcast"),
                         URLQueryItem(name: "term", value: query), URLQueryItem(name: "limit", value: String(limit))]
        return c?.url
    }
    public static let topURL = URL(string: "https://rss.marketingtools.apple.com/api/v2/us/podcasts/top/50/podcasts.json")!
    public static func lookupURL(ids: [String]) -> URL? { URL(string: "https://itunes.apple.com/lookup?entity=podcast&id=" + ids.joined(separator: ",")) }

    /// Shows from a search or lookup answer, in order, without those that have no feed or title or repeat a feed.
    public static func decodeSearch(_ data: Data) throws -> [PodcastShow] {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { throw ChoirError.decode("not a JSON object") }
        guard let list = o["results"] as? [[String: Any]] else { if o["results"] == nil || o["results"] is NSNull { return [] }; throw ChoirError.decode("results is not a list") }
        var seen = Set<String>(), out: [PodcastShow] = []
        for r in list {
            let feed = PodcastText.webURL((r["feedUrl"] as? String) ?? "")
            let title = PodcastText.plain((r["collectionName"] as? String) ?? (r["trackName"] as? String) ?? "", limit: 200).replacingOccurrences(of: "\n", with: " ")
            guard !feed.isEmpty, !title.isEmpty, seen.insert(PodcastKey.feed(feed)).inserted else { continue }
            let art = (r["artworkUrl600"] as? String) ?? (r["artworkUrl100"] as? String) ?? (r["artworkUrl60"] as? String) ?? ""
            out.append(PodcastShow(feedURL: feed, title: title, author: (r["artistName"] as? String) ?? "", artworkURL: PodcastText.https(PodcastText.webURL(art))))
        }
        return out
    }

    /// The chart's ids and names for shows whose genre is News, at most `limit`. The chart has no feed URLs: `lookupURL` adds them.
    public static func decodeChartNews(_ data: Data, limit: Int = 8) throws -> [(id: String, name: String)] {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any], let feed = o["feed"] as? [String: Any] else { throw ChoirError.decode("not a chart") }
        let list = (feed["results"] as? [[String: Any]]) ?? []
        var out: [(String, String)] = []
        for r in list {
            let genres = ((r["genres"] as? [[String: Any]]) ?? []).compactMap { $0["name"] as? String }
            guard genres.contains(where: { $0.localizedCaseInsensitiveContains("news") }), let id = r["id"] as? String, let n = r["name"] as? String else { continue }
            out.append((id, n))
            if out.count == limit { break }
        }
        return out
    }
}

public enum PodcastKey {
    /// A feed address as one identity: trimmed, scheme and host lower-cased, no trailing slash.
    public static func feed(_ s: String) -> String {
        var t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        if let u = URL(string: t), var c = URLComponents(url: u, resolvingAgainstBaseURL: false) {
            c.scheme = c.scheme?.lowercased(); c.host = c.host?.lowercased(); c.fragment = nil
            t = c.string ?? t
        }
        while t.hasSuffix("/") { t.removeLast() }
        return t
    }
    /// One episode as one string: the feed and the guid. The queue's `episodeId` for a podcast play, and the key of its resume position.
    public static func episode(feed: String, id: String) -> String { "pod:" + PathSegment.encode(feed) + "/" + PathSegment.encode(id) }
    public static func parseEpisode(_ key: String) -> (feed: String, id: String)? {
        guard key.hasPrefix("pod:") else { return nil }
        let parts = key.dropFirst(4).split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
        guard parts.count == 2, let f = String(parts[0]).removingPercentEncoding, let i = String(parts[1]).removingPercentEncoding, !f.isEmpty, !i.isEmpty else { return nil }
        return (f, i)
    }
}
