import Foundation

// Citations for the reader (lane 3): a source_ref anchor becomes a numbered mark and a card (the source's host and title, the
// pinned quote with its check badge, the quote inside its source block). No hover: a tap expands the card in place.
//
// WAYS THIS CAN FAIL (written before the code; TextureDocTests covers each):
//  1. The entity is missing or has no title or URL: the card says "Source unavailable" and still opens; nothing crashes.
//  2. The URL has no scheme, a user:pass@ part, a port, upper case, or is not a URL at all: host() returns a clean host
//     (no "www.", no credentials, no port) or "" when there is none.
//  3. The check state is unknown, empty or in a different case: it reads as "pending" ("Not yet checked").
//  4. The quote is empty, longer than the block, appears twice (the first is used), differs by curly versus straight quotes,
//     by whitespace or by case, or has an ellipsis ("a ... b" or "a … b"): hit/before/after are cut from the ORIGINAL
//     text through an index map, never from the normalised copy; when nothing matches, hit is "" and before is the block
//     (shown without a highlight).
//  5. A transclusion without a snapshot: the quote falls back to the check's quote, then the ref's quote, then "".
//  6. Two anchors cite one entity: one source number, two cards (each with its own quote); the Sources list has one row
//     with both quotes.
//  7. A Texture-to-Texture transclusion: kind "texture", target "texture:<id>"; never a web URL.

public enum Citations {
    /// "https://www.Example.com:8080/a?b" -> "example.com"; "" when there is no host.
    public static func host(_ raw: String) -> String {
        var s = raw.rdTrim
        if s.isEmpty { return "" }
        if !s.contains("://") { s = "https://" + s }
        guard let u = URL(string: s), var h = u.host?.lowercased(), !h.isEmpty else { return "" }
        if h.hasPrefix("www.") { h.removeFirst(4) }
        return h.contains(".") || h == "localhost" ? h : ""
    }

    public static func checkLabel(_ state: String) -> (state: String, label: String) {
        switch state.rdTrim.lowercased() {
        case "exact": return ("exact", "Exact quote")
        case "tolerant": return ("tolerant", "Close match")
        case "failed": return ("failed", "Quote not found")
        case "unread": return ("unread", "Source not read")
        default: return ("pending", "Not yet checked")
        }
    }

    static func isTextureKind(_ kind: String, _ uri: String) -> Bool {
        ["texture_span", "publication_span", "texture", "texture_doc"].contains(kind) || uri.hasPrefix("texture:")
    }

    /// A ref given as an object or as JSON text (the web's parseJSON of attrs.ref).
    static func refObject(_ v: Any?) -> [String: Any]? {
        if let o = v as? [String: Any] { return o }
        if let s = v as? String, let d = s.data(using: .utf8) { return (try? JSONSerialization.jsonObject(with: d)) as? [String: Any] }
        return nil
    }

    /// The server's check carries `quote` = the outcome word, not the quoted words.
    static let outcomeWords: Set<String> = ["exact", "tolerant", "failed", "unread", "pending"]

    static func cite(anchor: String, number: Int, blockId: String, entityId: String, ctx: TextureDocParser.Context, nodeRef: [String: Any]? = nil) -> ReaderCite {
        let entity = ctx.entities[entityId]
        let target = entity?["target"] as? [String: Any] ?? [:]
        let display = entity?["display"] as? [String: Any] ?? [:]
        let kindRaw = (target["kind"] as? String) ?? ""
        let uri = (target["uri"] as? String) ?? ""
        let tid = (target["id"] as? String) ?? ""
        var c = ReaderCite(anchor: anchor, number: number, blockId: blockId, entityId: entityId)
        if entity == nil {
            c.title = "Source unavailable"
            c.kind = "unknown"
        } else if isTextureKind(kindRaw, uri) {
            c.kind = "texture"
            let raw = uri.hasPrefix("texture:") ? String(uri.dropFirst(8)) : tid
            c.target = raw.isEmpty ? "" : "texture:" + raw
            c.title = firstNonEmpty([display["title"], display["label"]], "A Texture")
        } else {
            c.kind = "web"
            c.url = uri
            c.host = host(uri)
            c.title = firstNonEmpty([display["title"], display["label"]], c.host.isEmpty ? "Source" : c.host)
        }
        // The quote: the transclusion's snapshot, else the check's, else the ref's.
        let tr = ctx.transclusions[anchor]
        let ck = ctx.checks[anchor]
        // The ref: the manifest anchor's (as the web's published context reads it), else the citation node's own attrs.ref,
        // else the one a transclusion or a check carries.
        let manifestRef = refObject(ctx.manifestAnchors[anchor]?["ref"])
        let ref = (tr?["ref"] as? [String: Any]) ?? manifestRef ?? nodeRef ?? (ck?["ref"] as? [String: Any]) ?? [:]
        let ckQuote = (ck?["quote"] as? String).flatMap { outcomeWords.contains($0.rdTrim.lowercased()) ? nil : $0 }
        c.quote = firstNonEmpty([tr?["snapshot_text"], ref["quote"], manifestRef?["quote"], nodeRef?["quote"], ckQuote], "")
        let (state, label) = checkLabel((ck?["state"] as? String) ?? "")
        c.check = state; c.checkLabel = label
        // The quote inside its source block (resolved_blocks, by cid).
        if let cid = ref["cid"] as? String, let block = ctx.resolved[cid] {
            let text = (block["text"] as? String) ?? ""
            if c.url.isEmpty, let u = block["url"] as? String, c.kind == "web" { c.url = u; c.host = host(u) }
            if c.title == c.host || c.title == "Source", let t = block["title"] as? String, !t.rdTrim.isEmpty { c.title = t.rdTrim }
            let parts = context(of: c.quote, in: text)
            c.before = parts.before; c.hit = parts.hit; c.after = parts.after
        }
        return c
    }

    static func firstNonEmpty(_ vals: [Any?], _ fallback: String) -> String {
        for v in vals { if let s = v as? String, !s.rdTrim.isEmpty { return s.rdTrim } }
        return fallback
    }

    // MARK: Quote inside its block

    /// Split `block` into (before, hit, after) around `quote`, matching through a normalisation (case, curly quotes, runs of
    /// white space) and cutting from the original text. An ellipsis quote highlights from its first segment to its last.
    public static func context(of quote: String, in block: String) -> (before: String, hit: String, after: String) {
        let q = quote.rdTrim
        if q.isEmpty || block.isEmpty { return (block, "", "") }
        let segments = q.replacingOccurrences(of: "\u{2026}", with: "...").components(separatedBy: "...")
            .map { $0.rdTrim }.filter { !$0.isEmpty }
        guard let firstSeg = segments.first, let lastSeg = segments.last else { return (block, "", "") }
        let (norm, map) = normalised(block)
        let nFirst = normalised(firstSeg).text, nLast = normalised(lastSeg).text
        guard !nFirst.isEmpty, let r1 = norm.range(of: nFirst) else { return (block, "", "") }
        let startN = norm.distance(from: norm.startIndex, to: r1.lowerBound)
        var endN = norm.distance(from: norm.startIndex, to: r1.upperBound)
        if segments.count > 1, let r2 = norm.range(of: nLast, range: r1.upperBound..<norm.endIndex) {
            endN = norm.distance(from: norm.startIndex, to: r2.upperBound)
        }
        guard startN < map.count, endN > startN, endN <= map.count else { return (block, "", "") }
        let chars = Array(block)
        let s = map[startN], e = map[endN - 1] + 1
        guard s < e, e <= chars.count else { return (block, "", "") }
        return (String(chars[0..<s]), String(chars[s..<e]), String(chars[e...]))
    }

    /// Lower-cased, curly quotes straightened, white space runs collapsed; `map[i]` is the original Character index of char i.
    static func normalised(_ s: String) -> (text: String, map: [Int]) {
        var out = "", map: [Int] = []
        var lastSpace = true
        for (i, ch) in s.enumerated() {
            var c = ch
            switch c {
            case "\u{2018}", "\u{2019}": c = "'"
            case "\u{201C}", "\u{201D}": c = "\""
            case "\u{2013}", "\u{2014}": c = "-"
            default: break
            }
            if c.isWhitespace || c == "\u{00A0}" {
                if lastSpace { continue }
                out.append(" "); map.append(i); lastSpace = true
            } else {
                for u in String(c).lowercased() { out.append(u); map.append(i) }
                lastSpace = false
            }
        }
        while out.hasSuffix(" ") { out.removeLast(); map.removeLast() }
        return (out, map)
    }

    // MARK: Sources list

    public static func sources(from cites: [ReaderCite]) -> [ReaderSource] {
        var order: [String] = []
        var by: [String: ReaderSource] = [:]
        for c in cites {
            let key = c.entityId.isEmpty ? "anon-\(c.anchor)" : c.entityId
            if var s = by[key] {
                if !c.quote.isEmpty, !s.quotes.contains(c.quote) { s.quotes.append(c.quote) }
                by[key] = s
            } else {
                order.append(key)
                by[key] = ReaderSource(number: c.number, entityId: c.entityId, title: c.title, host: c.host, url: c.url, kind: c.kind,
                                       target: c.target, quotes: c.quote.isEmpty ? [] : [c.quote], anchor: c.anchor)
            }
        }
        return order.compactMap { by[$0] }.sorted { $0.number < $1.number }
    }
}
