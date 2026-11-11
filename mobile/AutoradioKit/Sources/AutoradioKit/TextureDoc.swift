import Foundation

// The reader's document model (lane 3). The server's document shapes (a published bundle's `artifact.body_doc`, a private
// revision's `body_doc`, or plain Markdown text) become flat blocks of styled runs the Contract renders: no recursion in the
// model, because a Contract shape cannot nest itself. A list item is a block with a depth and a marker; a block quote
// is a block with a quote depth; a table is a block of rows of cells. Citations are runs that carry an anchor.
//
// WAYS THIS CAN FAIL (written before the code; TextureDocTests covers each):
//  1. body_doc missing, empty, not JSON, or an unknown schema: fall back to the Markdown text (`content`, `payload.text`);
//     with neither, a ready document with a visible "no text yet" block; never a crash, never a blank page.
//  2. An unknown block or inline node type (a newer schema): its text is kept as a paragraph or run; text is never dropped.
//  3. A source_ref with no entity, no id, or a duplicate id: it still draws a numbered mark, its card says the source is
//     unavailable; ids are made unique so two marks never open one card.
//  4. Empty text nodes, whitespace-only paragraphs, a hard_break at either end: no empty runs, no empty paragraph blocks.
//  5. Nested lists, ordered lists with a start, list items with several blocks: depth and numbering are right, the
//     second block of an item has no marker, the numbering continues per list.
//  6. Block quotes with several paragraphs or nested quotes: each paragraph is a quote block with the depth.
//  7. Code blocks keep their newlines and indentation; no sentence splitting, no styling.
//  8. A very long document (10 000 blocks): one pass, linear; a paragraph over 20 000 characters is not sentence-split.
//  9. Sentence indices come from the plain text without marks, so a citation inside a sentence belongs to that sentence;
//     abbreviations ("Dr.", "U.S.") are the system tokenizer's business and a wrong split only moves a highlight.
// 10. Tables with ragged rows, an empty row, a header row; a Markdown table without a separator line is prose.
// 11. Markdown: unmatched `*`, `_` in snake_case, `5 * 3`, an unterminated code fence are literal text, never swallowed.
// 12. Versions: prev/next/latest of 0 mean none; a revision above latest is clamped; "newer" only when latest > revision.
// 13. A first heading equal to the title is dropped (the page already shows the headline).
// 14. Addresses: texture:<doc>@<rev> with an empty doc is an error; a revision that is all digits is a version number.

// MARK: Shapes (the Contract reads these by field name)

public struct ReaderRun: Codable, Sendable, Equatable {
    public var id: String
    public var text: String
    /// t (text), code, link, cite (a numbered source mark), ref (a Texture transclusion chip), br (hard break)
    public var kind: String
    public var weight: Int
    public var slant: String
    public var anchor: String
    public var label: String
    /// the sentence this run belongs to within its block
    public var sent: Int
    public var href: String
    public init(id: String, text: String, kind: String = "t", weight: Int = 400, slant: String = "normal", anchor: String = "",
                label: String = "", sent: Int = 0, href: String = "") {
        self.id = id; self.text = text; self.kind = kind; self.weight = weight; self.slant = slant
        self.anchor = anchor; self.label = label; self.sent = sent; self.href = href
    }
}

public struct ReaderCell: Codable, Sendable, Equatable {
    public var runs: [ReaderRun]
    public var header: Bool
    public init(runs: [ReaderRun], header: Bool) { self.runs = runs; self.header = header }
}

public struct ReaderRow: Codable, Sendable, Equatable {
    public var id: String
    public var cells: [ReaderCell]
    public init(id: String, cells: [ReaderCell]) { self.id = id; self.cells = cells }
}

public struct ReaderBlock: Codable, Sendable, Equatable {
    public var id: String
    /// p, h, li, quote, code, rule, table, embed
    public var kind: String
    public var level: Int
    public var depth: Int
    public var marker: String
    public var runs: [ReaderRun]
    public var rows: [ReaderRow]
    /// plain text of the block (aim, share, read-aloud matching)
    public var text: String
    /// ordinal among the blocks that carry text; -1 for none
    public var ordinal: Int
    /// anchors of the citations in this block (the cards open after it)
    public var cites: [String]
    public var sentences: Int
    public init(id: String, kind: String, level: Int = 0, depth: Int = 0, marker: String = "", runs: [ReaderRun] = [],
                rows: [ReaderRow] = [], text: String = "", ordinal: Int = -1, cites: [String] = [], sentences: Int = 0) {
        self.id = id; self.kind = kind; self.level = level; self.depth = depth; self.marker = marker; self.runs = runs
        self.rows = rows; self.text = text; self.ordinal = ordinal; self.cites = cites; self.sentences = sentences
    }
}

/// One citation anchor: a mark in the text, its card, and how it checked.
public struct ReaderCite: Codable, Sendable, Equatable {
    public var anchor: String
    public var number: Int
    public var blockId: String
    public var entityId: String
    public var title: String
    public var host: String
    public var url: String
    /// web | texture | unknown
    public var kind: String
    /// for a Texture transclusion: the address of the cited document, opened as its own page
    public var target: String
    public var quote: String
    public var before: String
    public var hit: String
    public var after: String
    public var check: String
    public var checkLabel: String
    public init(anchor: String, number: Int, blockId: String, entityId: String = "", title: String = "", host: String = "", url: String = "",
                kind: String = "unknown", target: String = "", quote: String = "", before: String = "", hit: String = "", after: String = "",
                check: String = "pending", checkLabel: String = "Not yet checked") {
        self.anchor = anchor; self.number = number; self.blockId = blockId; self.entityId = entityId; self.title = title; self.host = host
        self.url = url; self.kind = kind; self.target = target; self.quote = quote; self.before = before; self.hit = hit; self.after = after
        self.check = check; self.checkLabel = checkLabel
    }
}

/// One source in the Sources list: a distinct entity, numbered in reading order.
public struct ReaderSource: Codable, Sendable, Equatable {
    public var number: Int
    public var entityId: String
    public var title: String
    public var host: String
    public var url: String
    public var kind: String
    public var target: String
    public var quotes: [String]
    /// the first citation of this source (the Sources list opens its card)
    public var anchor: String
    public init(number: Int, entityId: String, title: String, host: String, url: String, kind: String, target: String, quotes: [String], anchor: String = "") {
        self.number = number; self.entityId = entityId; self.title = title; self.host = host; self.url = url
        self.kind = kind; self.target = target; self.quotes = quotes; self.anchor = anchor
    }
}

public struct ReaderVersion: Codable, Sendable, Equatable {
    public var address: String
    public var number: Int
    public var label: String
    public var when: String
    public var author: String
    public var current: Bool
    public init(address: String, number: Int, label: String, when: String, author: String, current: Bool) {
        self.address = address; self.number = number; self.label = label; self.when = when; self.author = author; self.current = current
    }
}

public struct ReaderRelated: Codable, Sendable, Equatable {
    public var address: String
    public var title: String
    public var dek: String
    public init(address: String, title: String, dek: String) { self.address = address; self.title = title; self.dek = dek }
}

public struct ReaderDoc: Codable, Sendable, Equatable {
    public var address: String
    /// ready | loading | notyet | error
    public var status: String
    public var message: String
    public var title: String
    public var kicker: String
    public var dek: String
    public var byline: String
    public var meta: String
    /// story | opinion | document
    public var kind: String
    public var disclosure: String
    public var blocks: [ReaderBlock]
    public var cites: [ReaderCite]
    public var sources: [ReaderSource]
    public var versions: [ReaderVersion]
    public var related: [ReaderRelated]
    public var revision: Int
    public var latest: Int
    public var newer: Bool
    public var historical: Bool
    public var revising: Bool
    public var revisingNote: String
    public var trajectory: String
    public var permalink: String
    public var plain: String
    public var textBlocks: Int
    /// the same document as Markdown with citations as `[n](cite:anchor)` links (the atom-pill experiment)
    public var markdown: String

    public init(address: String, status: String = "loading", message: String = "", title: String = "", kicker: String = "", dek: String = "",
                byline: String = "", meta: String = "", kind: String = "document", disclosure: String = "", blocks: [ReaderBlock] = [],
                cites: [ReaderCite] = [], sources: [ReaderSource] = [], versions: [ReaderVersion] = [], related: [ReaderRelated] = [],
                revision: Int = 0, latest: Int = 0, newer: Bool = false, historical: Bool = false, revising: Bool = false,
                revisingNote: String = "", trajectory: String = "", permalink: String = "", plain: String = "", textBlocks: Int = 0, markdown: String = "") {
        self.address = address; self.status = status; self.message = message; self.title = title; self.kicker = kicker; self.dek = dek
        self.byline = byline; self.meta = meta; self.kind = kind; self.disclosure = disclosure; self.blocks = blocks; self.cites = cites
        self.sources = sources; self.versions = versions; self.related = related; self.revision = revision; self.latest = latest
        self.newer = newer; self.historical = historical; self.revising = revising; self.revisingNote = revisingNote
        self.trajectory = trajectory; self.permalink = permalink; self.plain = plain; self.textBlocks = textBlocks; self.markdown = markdown
    }

    public static func loading(_ address: String) -> ReaderDoc { ReaderDoc(address: address, status: "loading") }
    public static func failed(_ address: String, _ message: String) -> ReaderDoc { ReaderDoc(address: address, status: "error", message: message) }

    /// The lines the spine's plain renderer and the radio's text source use.
    public var lines: [String] { blocks.filter { !$0.text.isEmpty && $0.kind != "code" }.map { $0.text } }
}

// MARK: Addresses

public struct TextureAddress: Equatable, Sendable {
    public enum Kind: Sendable { case publication, document, none }
    public var kind: Kind
    public var route: String        // publication route (pub:)
    public var doc: String          // document id (texture:)
    public var rev: String          // revision id or "" for the head
    public static func parse(_ address: String) -> TextureAddress {
        let rest = PageAddress.rest(of: address)
        if address.hasPrefix("pub:") {
            return rest.isEmpty ? .init(kind: .none, route: "", doc: "", rev: "") : .init(kind: .publication, route: rest, doc: "", rev: "")
        }
        if address.hasPrefix("texture:") {
            let parts = rest.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).map(String.init)
            let doc = parts.first ?? ""
            if doc.isEmpty { return .init(kind: .none, route: "", doc: "", rev: "") }
            return .init(kind: .document, route: "", doc: doc, rev: parts.count > 1 ? parts[1] : "")
        }
        return .init(kind: .none, route: "", doc: "", rev: "")
    }
    /// "slug/v3" -> ("slug", 3); a bare route -> (route, 0)
    public static func splitRoute(_ route: String) -> (bare: String, version: Int) {
        guard let r = route.range(of: "/v", options: .backwards) else { return (route, 0) }
        let tail = route[r.upperBound...]
        if let n = Int(tail), n > 0 { return (String(route[..<r.lowerBound]), n) }
        return (route, 0)
    }
}

// MARK: Parser

public enum TextureDocParser {
    /// The most version rows one document gets, whatever count the server claims.
    static let maxVersions = 500
    static let maxSentenceChars = 20_000
    static let disclosureStory = "Written by AI. Autopaper\u{2019}s agent desks wrote this story from the sources it links; no human editor reviewed it before publication."
    static let noticeOpinion = "Opinion. Written and published by a reader; Autopaper\u{2019}s agent desks did not write it and no editor reviewed it."

    /// Context from the bundle that the body's source_ref nodes point into.
    struct Context {
        var entities: [String: [String: Any]] = [:]            // source_entity_id -> entity
        var transclusions: [String: [String: Any]] = [:]       // anchor id -> transclusion
        var resolved: [String: [String: Any]] = [:]            // cid -> resolved block
        var checks: [String: [String: Any]] = [:]              // anchor id -> check
        var manifestAnchors: [String: [String: Any]] = [:]     // anchor id -> manifest anchor (its ref holds the quoted words)
    }

    // MARK: Published bundle (GET /api/platform/publications/resolve)

    public static func parseBundle(_ o: [String: Any], address: String) -> ReaderDoc {
        let pub = o["publication"] as? [String: Any] ?? [:]
        let artifact = o["artifact"] as? [String: Any] ?? [:]
        let payload = o["payload"] as? [String: Any] ?? [:]
        let nav = o["nav"] as? [String: Any] ?? [:]
        let route = o["route"] as? [String: Any] ?? [:]
        let title = (pub["title"] as? String)?.rdTrim ?? ""
        let kindRaw = pub["kind"] as? String ?? ""

        var ctx = Context()
        for e in (o["source_entities"] as? [[String: Any]]) ?? [] {
            if let id = e["source_entity_id"] as? String, let ent = e["entity"] as? [String: Any] { ctx.entities[id] = ent }
        }
        for e in (artifact["source_entities"] as? [[String: Any]]) ?? [] {
            if let id = e["source_entity_id"] as? String, ctx.entities[id] == nil { ctx.entities[id] = e }
        }
        for t in (o["transclusions"] as? [[String: Any]]) ?? [] {
            if let a = t["anchor_id"] as? String, !a.isEmpty { ctx.transclusions[a] = t }
        }
        for r in (payload["resolved_blocks"] as? [[String: Any]]) ?? [] {
            if let c = r["cid"] as? String { ctx.resolved[c] = r }
        }
        for c in ((o["checks"] as? [String: Any])?["citations"] as? [[String: Any]]) ?? [] {
            if let a = c["anchor_id"] as? String { ctx.checks[a] = c }
        }

        // The manifest's anchors, as the web builds its published citation context (texture-published-citations.ts).
        var manifest = payload["manifest"]
        if let ms = manifest as? String, let d = ms.data(using: .utf8) { manifest = try? JSONSerialization.jsonObject(with: d) }
        for a in ((manifest as? [String: Any])?["anchors"] as? [[String: Any]]) ?? [] {
            if let id = a["anchor_id"] as? String, !id.isEmpty { ctx.manifestAnchors[id] = a }
        }

        let text = (artifact["content"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? (payload["text"] as? String) ?? ""
        var doc = build(address: address, title: title, bodyDoc: artifact["body_doc"], markdown: text, ctx: ctx)
        doc.kind = kindRaw == "opinion" ? "opinion" : (kindRaw == "story" ? "story" : "document")
        let author = (pub["author_label"] as? String)?.rdTrim ?? ""
        switch doc.kind {
        case "story": doc.kicker = (pub["section"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "Autopaper"; doc.disclosure = disclosureStory
        case "opinion": doc.kicker = "Opinion"; doc.byline = author.isEmpty ? "" : "by \(author)"
            doc.disclosure = author.isEmpty ? noticeOpinion : noticeOpinion + " Author: \(author)."
        default: doc.kicker = "Texture"
        }
        doc.dek = (pub["dek"] as? String)?.rdTrim ?? ""
        // Versions and navigation: nav {revision, prev, next, latest}; a bare version summary otherwise.
        let version = o["version"] as? [String: Any] ?? [:]
        var rev = intValue(nav["revision"]) ?? intValue(version["revision"]) ?? intValue(payload["revision"]) ?? 0
        let latest = min(max(0, intValue(nav["latest"]) ?? rev), TextureDocParser.maxVersions)   // a corrupt count must not become millions of rows
        if rev > latest { rev = latest }
        doc.revision = rev
        doc.latest = latest
        doc.newer = latest > rev && rev > 0
        doc.historical = doc.newer
        let bare = (route["bare"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? TextureAddress.splitRoute(PageAddress.rest(of: address)).bare
        doc.permalink = (route["permalink"] as? String) ?? ""
        if latest > 0 && !bare.isEmpty {
            doc.versions = (1...latest).reversed().map { n in
                ReaderVersion(address: "pub:\(bare)/v\(n)", number: n, label: n == latest ? "Version \(n) · Latest" : "Version \(n)",
                              when: "", author: "", current: n == rev)
            }
        }
        if let published = version["published_at"] as? String, !published.isEmpty {
            let when = shortDate(published)
            doc.meta = latest > 0 ? "Version \(rev) of \(latest) · \(when)" : when
            if let i = doc.versions.firstIndex(where: { $0.number == rev }) { doc.versions[i].when = when }
        } else if latest > 0 {
            doc.meta = "Version \(rev) of \(latest)"
        }
        if doc.title.isEmpty { doc.title = "Untitled" }
        return doc
    }

    // MARK: Private document revision (GET /api/texture/revisions/{id} or the document's head revision)

    public static func parseRevision(_ rev: [String: Any], document: [String: Any], address: String, revisions: [[String: Any]]) -> ReaderDoc {
        let title = ((document["title"] as? String) ?? (rev["title"] as? String) ?? "").rdTrim
        var ctx = Context()
        let entsRaw = rev["source_entities"]
        if let arr = entsRaw as? [[String: Any]] {
            for e in arr { if let id = e["source_entity_id"] as? String { ctx.entities[id] = e } }
        }
        for c in (rev["citations"] as? [[String: Any]]) ?? [] {
            if let a = (c["anchor_id"] as? String) ?? (c["id"] as? String) { ctx.checks[a] = c }
        }
        var doc = build(address: address, title: title, bodyDoc: rev["body_doc"], markdown: (rev["content"] as? String) ?? "", ctx: ctx)
        doc.kind = "document"
        doc.kicker = "Texture"
        let ta = TextureAddress.parse(address)
        // Versions, newest first. A revision's number is its version_number + 1 (the store counts from zero).
        var rows: [(rid: String, n: Int, author: String, when: String)] = []
        for r in revisions {
            guard let rid = r["revision_id"] as? String else { continue }
            let n = (intValue(r["version_number"]) ?? rows.count) + 1
            let author = ((r["author_label"] as? String) ?? (r["author_kind"] as? String) ?? "").rdTrim
            rows.append((rid, n, author, shortDate((r["created_at"] as? String) ?? "")))
        }
        rows.sort { $0.n > $1.n }
        let latest = rows.first?.n ?? 0
        let thisRev = (rev["revision_id"] as? String) ?? ""
        let thisNumber = rows.first(where: { $0.rid == thisRev })?.n ?? (intValue(rev["version_number"]).map { $0 + 1 } ?? latest)
        doc.revision = min(thisNumber, max(latest, thisNumber))
        doc.latest = max(latest, doc.revision)
        doc.newer = doc.latest > doc.revision && doc.revision > 0
        doc.historical = doc.newer
        let docId = ta.doc.isEmpty ? ((document["doc_id"] as? String) ?? "") : ta.doc
        doc.versions = rows.map { r in
            ReaderVersion(address: r.n == latest ? "texture:\(docId)" : "texture:\(docId)@\(r.rid)", number: r.n,
                          label: r.n == latest ? "Version \(r.n) · Latest" : "Version \(r.n)", when: r.when, author: r.author, current: r.n == doc.revision)
        }
        let cur = rows.first(where: { $0.n == doc.revision })
        let when = cur?.when ?? shortDate((rev["created_at"] as? String) ?? "")
        doc.byline = cur?.author ?? ""
        doc.meta = [doc.latest > 0 ? "Version \(doc.revision) of \(doc.latest)" : "", when].filter { !$0.isEmpty }.joined(separator: " · ")
        doc.revising = (document["agent_revision_pending"] as? Bool) ?? (document["revising"] as? Bool) ?? false
        doc.trajectory = (document["trajectory_id"] as? String) ?? ""
        doc.revisingNote = (document["revising_note"] as? String) ?? (doc.revising ? "The desk is writing a new version." : "")
        if doc.title.isEmpty { doc.title = "Untitled" }
        return doc
    }

    // MARK: Shared builder

    static func build(address: String, title: String, bodyDoc: Any?, markdown: String, ctx: Context) -> ReaderDoc {
        var b = Builder(ctx: ctx)
        var parsed = false
        if let root = decodeBodyDoc(bodyDoc), let content = root["content"] as? [[String: Any]], !content.isEmpty {
            for n in content { b.block(n, listDepth: 0, quoteDepth: 0) }
            parsed = !b.blocks.isEmpty
        }
        if !parsed {
            b = Builder(ctx: ctx)
            for blk in Markdown.blocks(markdown) { b.add(markdownBlock: blk) }
        }
        var blocks = b.blocks
        // A leading heading that repeats the headline is dropped (the page already shows it).
        if let f = blocks.first, f.kind == "h", f.text.rdTrim.lowercased() == title.lowercased(), !title.isEmpty { blocks.removeFirst() }
        // Ordinals over the blocks that carry text.
        var ord = 0
        for i in blocks.indices where blocks[i].kind != "rule" && blocks[i].kind != "table" && blocks[i].kind != "embed" && !blocks[i].text.isEmpty {
            blocks[i].ordinal = ord; ord += 1
        }
        if blocks.isEmpty {
            blocks = [ReaderBlock(id: "empty", kind: "p", runs: [ReaderRun(id: "empty-0", text: "This document has no text yet.", slant: "italic")],
                                  text: "This document has no text yet.")]
        }
        // The mark each source carries and the Sources list, in reading order.
        var doc = ReaderDoc(address: address, status: "ready", title: title, blocks: blocks)
        doc.cites = b.finishCites(blockIds: Set(blocks.map { $0.id }))
        doc.sources = Citations.sources(from: doc.cites)
        doc.plain = blocks.filter { $0.kind != "code" }.map { $0.text }.filter { !$0.isEmpty }.joined(separator: "\n\n")
        doc.textBlocks = ord
        doc.markdown = markdownSource(blocks)
        return doc
    }

    /// The blocks as Markdown, a citation as a `cite:` link, for the reader that draws citations as atom pills.
    static func markdownSource(_ blocks: [ReaderBlock]) -> String {
        func md(_ runs: [ReaderRun]) -> String {
            runs.map { r -> String in
                switch r.kind {
                case "cite": return "[\(r.label)](cite:\(r.anchor))"
                case "ref": return "[\(r.text.rdTrim)](cite:\(r.anchor))"
                case "br": return "  \n"
                case "code": return "`\(r.text)`"
                case "link": return "[\(r.text)](\(r.href))"
                default:
                    var t = r.text
                    if t.rdTrim.isEmpty { return t }
                    let lead = String(t.prefix(while: { $0 == " " })), trail = String(String(t.reversed().prefix(while: { $0 == " " })))
                    t = t.rdTrim
                    if r.weight >= 700 { t = "**\(t)**" }
                    if r.slant == "italic" { t = "*\(t)*" }
                    return lead + t + trail
                }
            }.joined()
        }
        var out: [String] = []
        for b in blocks {
            switch b.kind {
            case "h": out.append(String(repeating: "#", count: max(1, min(6, b.level))) + " " + md(b.runs))
            case "li": out.append(String(repeating: "  ", count: max(0, b.depth - 1)) + (b.marker.isEmpty ? "  " : (b.marker.hasSuffix(".") ? b.marker : "-")) + " " + md(b.runs))
            case "quote": out.append(String(repeating: "> ", count: max(1, b.depth)) + md(b.runs))
            case "code": out.append("```\n" + b.text + "\n```")
            case "rule": out.append("---")
            case "table":
                var lines: [String] = []
                for (i, r) in b.rows.enumerated() {
                    lines.append("| " + r.cells.map { md($0.runs) }.joined(separator: " | ") + " |")
                    if i == 0 { lines.append("|" + String(repeating: "---|", count: r.cells.count)) }
                }
                out.append(lines.joined(separator: "\n"))
            default: out.append(md(b.runs))
            }
        }
        return out.joined(separator: "\n\n")
    }

    static func decodeBodyDoc(_ v: Any?) -> [String: Any]? {
        var obj: Any? = v
        if let s = v as? String {
            guard let d = s.data(using: .utf8), let j = try? JSONSerialization.jsonObject(with: d) else { return nil }
            obj = j
        }
        guard let o = obj as? [String: Any] else { return nil }
        if let d = o["doc"] as? [String: Any] { return d }           // {schema, doc:{type:"doc", content:[...]}}
        if o["type"] as? String == "doc" { return o }
        return nil
    }

    static func intValue(_ v: Any?) -> Int? {
        if let i = v as? Int { return i }
        if let d = v as? Double { return Int(d) }
        if let s = v as? String { return Int(s) }
        return nil
    }

    static func shortDate(_ iso: String) -> String {
        if iso.isEmpty { return "" }
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let f2 = ISO8601DateFormatter()
        guard let d = f.date(from: iso) ?? f2.date(from: iso) else { return "" }
        let out = DateFormatter()
        out.locale = Locale(identifier: "en_US_POSIX"); out.timeZone = TimeZone(identifier: "UTC"); out.dateFormat = "MMM d, yyyy"
        return out.string(from: d)
    }

    // MARK: Builder (body_doc nodes to blocks)

    struct Builder {
        var ctx: Context
        var blocks: [ReaderBlock] = []
        var counter = 0
        var anchors: [String: Int] = [:]               // anchor -> index in cites
        var cites: [ReaderCite] = []
        var entityNumber: [String: Int] = [:]
        var usedAnchorIds = Set<String>()
        var currentBlock = ""

        init(ctx: Context) { self.ctx = ctx }

        mutating func nextId(_ p: String) -> String { counter += 1; return "\(p)\(counter)" }

        mutating func block(_ n: [String: Any], listDepth: Int, quoteDepth: Int, marker: String? = nil) {
            let type = n["type"] as? String ?? ""
            let content = n["content"] as? [[String: Any]] ?? []
            let attrs = n["attrs"] as? [String: Any] ?? [:]
            switch type {
            case "paragraph", "heading":
                let level = type == "heading" ? max(1, min(6, TextureDocParser.intValue(attrs["level"]) ?? 2)) : 0
                let kind: String = type == "heading" ? "h" : (quoteDepth > 0 ? "quote" : (listDepth > 0 ? "li" : "p"))
                emit(kind: kind, level: level, depth: kind == "li" ? listDepth : quoteDepth, marker: marker ?? "", inline: content, nodeId: attrs["id"] as? String)
            case "bullet_list", "ordered_list":
                var number = max(1, TextureDocParser.intValue(attrs["start"]) ?? 1)
                for item in content {
                    let ordered = type == "ordered_list"
                    let m = ordered ? "\(number)." : (listDepth % 2 == 0 ? "\u{2022}" : "\u{2013}")
                    number += 1
                    var first = true
                    for child in (item["content"] as? [[String: Any]]) ?? [] {
                        let ct = child["type"] as? String ?? ""
                        if ct == "bullet_list" || ct == "ordered_list" { block(child, listDepth: listDepth + 1, quoteDepth: quoteDepth) }
                        else if ct == "paragraph" {
                            emit(kind: "li", level: 0, depth: listDepth + 1, marker: first ? m : "", inline: (child["content"] as? [[String: Any]]) ?? [],
                                 nodeId: (child["attrs"] as? [String: Any])?["id"] as? String)
                            first = false
                        } else { block(child, listDepth: listDepth + 1, quoteDepth: quoteDepth, marker: first ? m : ""); first = false }
                    }
                }
            case "blockquote":
                for child in content {
                    let ct = child["type"] as? String ?? ""
                    if ct == "paragraph" || ct == "heading" {
                        emit(kind: ct == "heading" ? "h" : "quote", level: ct == "heading" ? max(1, min(6, TextureDocParser.intValue((child["attrs"] as? [String: Any])?["level"]) ?? 3)) : 0, depth: quoteDepth + 1, marker: "",
                             inline: (child["content"] as? [[String: Any]]) ?? [], nodeId: (child["attrs"] as? [String: Any])?["id"] as? String)
                    } else { block(child, listDepth: listDepth, quoteDepth: quoteDepth + 1) }
                }
            case "code_block":
                let text = content.compactMap { $0["text"] as? String }.joined(separator: "\n")   // the web joins a code block's text nodes with a newline
                if !text.isEmpty { blocks.append(ReaderBlock(id: nextId("b"), kind: "code", text: text)) }
            case "horizontal_rule":
                blocks.append(ReaderBlock(id: nextId("b"), kind: "rule"))
            case "table":
                var rows: [ReaderRow] = []
                for (ri, r) in content.enumerated() {
                    var cells: [ReaderCell] = []
                    for c in (r["content"] as? [[String: Any]]) ?? [] {
                        var inl = (c["content"] as? [[String: Any]]) ?? []
                        // a cell holds paragraphs: flatten their inlines
                        if inl.first?["type"] as? String == "paragraph" { inl = inl.flatMap { ($0["content"] as? [[String: Any]]) ?? [] } }
                        let runs = TextureDocParser.parseInline(inl, builder: &self, bare: true).runs
                        let ct = c["type"] as? String ?? ""
                        cells.append(ReaderCell(runs: runs, header: ct == "table_header" || ct == "th"))
                    }
                    if !cells.isEmpty { rows.append(ReaderRow(id: "r\(ri)", cells: cells)) }
                }
                if !rows.isEmpty { blocks.append(ReaderBlock(id: nextId("b"), kind: "table", rows: rows)) }
            case "embed":
                // The label the web draws: the caption, else the title (file, audio), else the alt text. ReaderBlock has no
                // alt field: the shape is declared in exact-package/exact.json and interface/reader.json, and adding a field
                // there stalls the reader (seen in the app proof), so the alt text is the label only when there is no caption.
                func nonEmpty(_ k: String) -> String? { (attrs[k] as? String).map { $0.rdTrim }.flatMap { $0.isEmpty ? nil : $0 } }
                let label = nonEmpty("caption") ?? nonEmpty("title") ?? nonEmpty("alt") ?? nonEmpty("kind") ?? "Embedded item"
                var b = ReaderBlock(id: nextId("b"), kind: "embed", text: label)
                b.marker = attrs["kind"] as? String ?? ""
                blocks.append(b)
            default:
                // Unknown block type (a newer schema): keep its text, never drop it.
                if !content.isEmpty {
                    let hasInline = content.contains { ($0["type"] as? String) == "text" || ($0["type"] as? String) == "source_ref" }
                    if hasInline { emit(kind: "p", level: 0, depth: 0, marker: "", inline: content, nodeId: attrs["id"] as? String) }
                    else { for c in content { block(c, listDepth: listDepth, quoteDepth: quoteDepth) } }
                } else if let t = n["text"] as? String, !t.rdTrim.isEmpty {
                    blocks.append(ReaderBlock(id: nextId("b"), kind: "p", runs: [ReaderRun(id: "u\(counter)", text: t)], text: t.rdTrim))
                }
            }
        }

        mutating func emit(kind: String, level: Int, depth: Int, marker: String, inline: [[String: Any]], nodeId: String?) {
            let id = nextId("b")
            currentBlock = id
            var (runs, plainText) = TextureDocParser.parseInline(inline, builder: &self, bare: false)
            // trim hard breaks at both ends
            while runs.first?.kind == "br" { runs.removeFirst() }
            while runs.last?.kind == "br" { runs.removeLast() }
            if runs.isEmpty || (plainText.rdTrim.isEmpty && !runs.contains { $0.kind == "cite" || $0.kind == "ref" }) { return }
            let sentences = TextureDocParser.assignSentences(&runs, plain: plainText)
            let anchorsHere = runs.filter { $0.kind == "cite" || $0.kind == "ref" }.map { $0.anchor }
            blocks.append(ReaderBlock(id: id, kind: kind, level: level, depth: depth, marker: marker, runs: runs, text: plainText.rdTrim,
                                      cites: anchorsHere, sentences: sentences))
        }

        // MARK: Inline

        mutating func addCite(anchorHint: String?, entityId: String, blockId: String, nodeRef: [String: Any]? = nil) -> (anchor: String, number: Int, kind: String, title: String) {
            var anchor = (anchorHint ?? "").rdTrim
            if anchor.isEmpty { anchor = "c\(cites.count + 1)" }
            if usedAnchorIds.contains(anchor) { var k = 2; while usedAnchorIds.contains("\(anchor)-\(k)") { k += 1 }; anchor = "\(anchor)-\(k)" }
            usedAnchorIds.insert(anchor)
            let number: Int
            if let n = entityNumber[entityId], !entityId.isEmpty { number = n } else { number = entityNumber.count + 1; if !entityId.isEmpty { entityNumber[entityId] = number } else { entityNumber["anon-\(anchor)"] = number } }
            let c = Citations.cite(anchor: anchor, number: number, blockId: blockId, entityId: entityId, ctx: ctx, nodeRef: nodeRef)
            anchors[anchor] = cites.count
            cites.append(c)
            return (anchor, number, c.kind, c.title)
        }

        func finishCites(blockIds: Set<String>) -> [ReaderCite] { cites.filter { blockIds.contains($0.blockId) || $0.blockId.isEmpty } }

        // MARK: Markdown blocks

        mutating func add(markdownBlock m: Markdown.Block) {
            let id = nextId("b")
            switch m.kind {
            case "rule": blocks.append(ReaderBlock(id: id, kind: "rule"))
            case "code": blocks.append(ReaderBlock(id: id, kind: "code", text: m.text))
            case "table":
                let rows = m.rows.enumerated().map { (i, r) in
                    ReaderRow(id: "r\(i)", cells: r.map { ReaderCell(runs: Markdown.inline($0, idPrefix: "t\(id)\(i)"), header: i == 0 && m.header) })
                }
                blocks.append(ReaderBlock(id: id, kind: "table", rows: rows))
            default:
                var runs = Markdown.inline(m.text, idPrefix: id)
                let raw = runs.map { $0.text }.joined()
                let plain = raw.rdTrim
                if runs.isEmpty || plain.isEmpty { return }
                let sentences = TextureDocParser.assignSentences(&runs, plain: raw)
                blocks.append(ReaderBlock(id: id, kind: m.kind, level: m.level, depth: m.depth, marker: m.marker, runs: runs, text: plain, sentences: sentences))
            }
        }
    }

    // MARK: Inline parsing (free functions so Builder can call without overlapping access)

    static func parseInline(_ nodes: [[String: Any]], builder: inout Builder, bare: Bool) -> (runs: [ReaderRun], plain: String) {
        var runs: [ReaderRun] = []
        var plain = ""
        // The web collapses adjacent published citations of one source into one marker (renderInlineContent): only when the
        // bundle carries manifest anchors; whitespace-only text between them does not break the run, anything else does.
        var previousPublished = 0
        func walk(_ nodes: [[String: Any]]) {
            for n in nodes {
                let type = n["type"] as? String ?? ""
                switch type {
                case "text":
                    let t = n["text"] as? String ?? ""
                    if t.isEmpty { continue }
                    let marks = (n["marks"] as? [[String: Any]]) ?? []
                    let types = Set(marks.compactMap { $0["type"] as? String })
                    var href = ""
                    if let link = marks.first(where: { ($0["type"] as? String) == "link" }) { href = ((link["attrs"] as? [String: Any])?["href"] as? String) ?? "" }
                    if !t.rdTrim.isEmpty { previousPublished = 0 }
                    // Paired ** in an unmarked text node is bold written before Write parsed marks; the web draws it bold
                    // (renderLegacyStrong), with the same flanking rule.
                    if marks.isEmpty, t.contains("**") {
                        let ns = t as NSString
                        var last = 0
                        for m in legacyStrong.matches(in: t, range: NSRange(location: 0, length: ns.length)) {
                            let g2 = m.range(at: 2)
                            let before = ns.substring(with: NSRange(location: last, length: g2.location - 2 - last))
                            if !before.isEmpty { runs.append(ReaderRun(id: "x\(runs.count)", text: before)); plain += before }
                            let bold = ns.substring(with: g2)
                            runs.append(ReaderRun(id: "x\(runs.count)", text: bold, weight: 700)); plain += bold
                            last = m.range.location + m.range.length
                        }
                        if last < ns.length { let rest = ns.substring(from: last); runs.append(ReaderRun(id: "x\(runs.count)", text: rest)); plain += rest }
                        continue
                    }
                    let kind = types.contains("code") ? "code" : (href.isEmpty ? "t" : "link")
                    runs.append(ReaderRun(id: "x\(runs.count)", text: t, kind: kind, weight: types.contains("strong") ? 700 : 400,
                                          slant: types.contains("emphasis") ? "italic" : "normal", href: href))
                    plain += t
                case "hard_break":
                    previousPublished = 0
                    runs.append(ReaderRun(id: "x\(runs.count)", text: "\n", kind: "br")); plain += "\n"
                case "source_ref":
                    let attrs = n["attrs"] as? [String: Any] ?? [:]
                    let entity = (attrs["source_entity_id"] as? String) ?? ""
                    if bare {
                        // a table cell: a plain bracket mark with no card
                        runs.append(ReaderRun(id: "x\(runs.count)", text: "\u{2020}", kind: "t"))
                    } else {
                        let c = builder.addCite(anchorHint: attrs["id"] as? String, entityId: entity, blockId: builder.currentBlock,
                                                nodeRef: Citations.refObject(attrs["ref"]))
                        let published = (attrs["id"] as? String).flatMap { builder.ctx.manifestAnchors[$0] }.map { Citations.refObject($0["ref"])?["doc"] != nil } ?? false
                        if published {
                            if c.number == previousPublished { continue }   // the card stays in the Sources list
                            previousPublished = c.number
                        } else { previousPublished = 0 }
                        let isRef = c.kind == "texture"
                        runs.append(ReaderRun(id: "x\(runs.count)", text: isRef ? "\u{00A0}\(c.title)\u{00A0}" : "\u{00A0}\(c.number)\u{00A0}",
                                              kind: isRef ? "ref" : "cite", weight: 700, anchor: c.anchor, label: "\(c.number)"))
                    }
                default:
                    if let t = n["text"] as? String, !t.isEmpty { runs.append(ReaderRun(id: "x\(runs.count)", text: t)); plain += t }
                    else if let c = n["content"] as? [[String: Any]] { walk(c) }
                }
            }
        }
        walk(nodes)
        return (runs, plain)
    }

    /// The web's legacy-bold pattern (texture-structured-editor-doc.ts LEGACY_STRONG_RE): the opener is not preceded by a letter,
    /// digit or *, and is followed by a non-space; the closer is preceded by a non-space and not followed by a letter, digit or *.
    static let legacyStrong = try! NSRegularExpression(pattern: #"(^|[^\p{L}\p{N}*])\*\*(?=[^\s*])([^*]*?[^\s*])\*\*(?![\p{L}\p{N}*])"#)

    // MARK: Sentences

    /// Splits runs at sentence boundaries of the plain text and numbers each piece with its sentence. Returns the count.
    @discardableResult
    static func assignSentences(_ runs: inout [ReaderRun], plain: String) -> Int {
        guard !plain.isEmpty, plain.utf16.count <= maxSentenceChars else {
            for i in runs.indices { runs[i].sent = 0 }
            for i in runs.indices { runs[i].id = "x\(i)" }
            return plain.isEmpty ? 0 : 1
        }
        var ends: [Int] = []          // utf16 end offset of each sentence
        plain.enumerateSubstrings(in: plain.startIndex..<plain.endIndex, options: .bySentences) { _, _, enclosing, _ in
            ends.append(NSRange(enclosing, in: plain).upperBound)
        }
        if ends.isEmpty { ends = [plain.utf16.count] }
        var out: [ReaderRun] = []
        var offset = 0
        var sent = 0
        for r in runs {
            if r.kind == "cite" || r.kind == "ref" { var c = r; c.sent = min(sent, ends.count - 1); out.append(c); continue }
            let len = r.text.utf16.count
            var piece = r
            if len == 0 { continue }
            var consumed = 0
            let u = Array(r.text.utf16)
            while consumed < len {
                while sent < ends.count - 1 && offset >= ends[sent] { sent += 1 }
                let remainInSentence = ends[sent] - offset
                let take = (remainInSentence <= 0 || sent == ends.count - 1) ? len - consumed : min(len - consumed, remainInSentence)
                piece = r
                piece.text = String(decoding: u[consumed..<(consumed + take)], as: UTF16.self)
                piece.sent = sent
                out.append(piece)
                consumed += take; offset += take
            }
        }
        // A cite right after a boundary belongs to the sentence it closes: the sentence of the previous text run.
        for i in out.indices where out[i].kind == "cite" || out[i].kind == "ref" {
            if let j = (0..<i).reversed().first(where: { out[$0].kind != "cite" && out[$0].kind != "ref" }) { out[i].sent = out[j].sent }
        }
        for i in out.indices { out[i].id = "x\(i)" }
        runs = out
        return ends.count
    }
}

extension ReaderBlock {
    /// The text of each sentence (for matching what the radio reads).
    public func sentenceTexts() -> [String] {
        var by: [Int: String] = [:]
        for r in runs where r.kind != "cite" && r.kind != "ref" { by[r.sent, default: ""] += r.kind == "br" ? "\n" : r.text }
        return by.keys.sorted().map { by[$0]!.rdTrim }
    }
}

extension String {
    var rdTrim: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}
