import Foundation
import Compression

// EPUB to blocks (lane 5). An EPUB is a ZIP of XHTML: container.xml names the package (OPF); the OPF lists the manifest and
// the spine (reading order); each spine item is a chapter. No third-party unzip: the central directory is read here and
// deflate is Apple's Compression (COMPRESSION_ZLIB is raw deflate, which is what a ZIP stores).
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. Not a ZIP, a truncated ZIP, or a central directory pointing outside the data: a typed error, never a crash or a read
//     past the end (every offset is bounds-checked).
//  2. ZIP64, encryption (flag bit 0), or a method other than stored (0) or deflate (8): an "unsupported" error naming it.
//  3. A zip bomb: an entry declaring more than 32 MB uncompressed, or more than 5000 entries, is refused; the declared size
//     must equal what inflates, and the CRC-32 must match (a corrupt entry is an error, not garbled text).
//  4. Entry names with "..", a leading "/" or backslashes: never used to touch the disk (nothing is extracted to disk at all);
//     lookup normalises the path and falls back to a case-insensitive match (some books differ in case).
//  5. No container.xml, no rootfile, no OPF, an empty spine, or a spine item missing from the ZIP: an error for the first
//     three; an empty spine is an error; a missing chapter is skipped with a warning (the rest of the book still reads).
//  6. XHTML that is not well-formed XML (named HTML entities like &nbsp;, a DOCTYPE, an unclosed tag): entities are mapped to
//     characters first, the DOCTYPE is dropped, and a parse error keeps the blocks read so far (rough, not a failure).
//  7. Hrefs that are percent-encoded or carry a fragment ("ch1.xhtml#s2"): decoded and trimmed before lookup.
//  8. A chapter with no heading: its title is the XHTML <title>, else "Chapter N". Navigation documents (properties="nav")
//     are not chapters.
//  9. Images, tables and footnotes: rough. An image becomes "[image: alt]", a table row one line of cells joined by " | ".

public struct EpubBlock: Codable, Sendable, Equatable {
    /// heading | paragraph | quote | item | rule | code
    public var kind: String
    /// 1 to 6 for a heading; 0 otherwise.
    public var level: Int
    /// Plain text with *emphasis*, **strong** and `code` marks, and "\n" for a line break.
    public var text: String
    /// For an item: "- " or "3. ".
    public var marker: String
    public init(kind: String, level: Int = 0, text: String = "", marker: String = "") {
        self.kind = kind; self.level = level; self.text = text; self.marker = marker
    }
}

public struct EpubChapter: Sendable, Equatable {
    public var id: String
    public var href: String
    public var title: String
    public var blocks: [EpubBlock]
}

public struct EpubBook: Sendable, Equatable {
    public var title: String
    public var author: String
    public var language: String
    public var chapters: [EpubChapter]
    public var warnings: [String]
}

public enum EpubError: Error, CustomStringConvertible, Equatable {
    case notAZip, truncated, unsupported(String), tooLarge(String), corrupt(String), missing(String)
    public var description: String {
        switch self {
        case .notAZip: return "This is not a readable e-book (not a ZIP)."
        case .truncated: return "This e-book file is cut short or damaged."
        case .unsupported(let m): return "This e-book uses something that cannot be read here yet: \(m)."
        case .tooLarge(let m): return "This e-book is too large to open: \(m)."
        case .corrupt(let m): return "This e-book is damaged: \(m)."
        case .missing(let m): return "This e-book is incomplete: \(m) is missing."
        }
    }
}

// MARK: ZIP

public struct ZipEntry: Sendable, Equatable {
    public var name: String
    public var method: Int
    public var flags: Int
    public var crc: UInt32
    public var compressedSize: Int
    public var size: Int
    public var localOffset: Int
}

public enum Zip {
    public static let maxEntryBytes = 32_000_000
    public static let maxEntries = 5000

    private static func u16(_ d: Data, _ o: Int) -> Int? { o >= 0 && o + 2 <= d.count ? Int(d[d.startIndex + o]) | Int(d[d.startIndex + o + 1]) << 8 : nil }
    private static func u32(_ d: Data, _ o: Int) -> Int? {
        guard o >= 0, o + 4 <= d.count else { return nil }
        let b = d.startIndex + o
        return Int(d[b]) | Int(d[b + 1]) << 8 | Int(d[b + 2]) << 16 | Int(d[b + 3]) << 24
    }

    public static func entries(_ d: Data) throws -> [ZipEntry] {
        guard d.count >= 22 else { throw EpubError.notAZip }
        if u32(d, 0) != 0x04034b50 && u32(d, 0) != 0x06054b50 { throw EpubError.notAZip }
        // The end-of-central-directory record is within the last 22 + 65535 bytes.
        var eocd = -1
        var i = d.count - 22
        let stop = max(0, d.count - 22 - 65535)
        while i >= stop { if u32(d, i) == 0x06054b50 { eocd = i; break }; i -= 1 }
        guard eocd >= 0, let total = u16(d, eocd + 10), let cdSize = u32(d, eocd + 12), let cdOff = u32(d, eocd + 16) else { throw EpubError.truncated }
        if total == 0xFFFF || cdOff == 0xFFFFFFFF || cdSize == 0xFFFFFFFF { throw EpubError.unsupported("ZIP64") }
        guard total <= maxEntries else { throw EpubError.tooLarge("\(total) files inside") }
        guard cdOff + cdSize <= d.count else { throw EpubError.truncated }
        var out: [ZipEntry] = []
        var p = cdOff
        for _ in 0..<total {
            guard u32(d, p) == 0x02014b50, let flags = u16(d, p + 8), let method = u16(d, p + 10), let crc = u32(d, p + 16),
                  let cs = u32(d, p + 20), let us = u32(d, p + 24), let nl = u16(d, p + 28), let el = u16(d, p + 30),
                  let cl = u16(d, p + 32), let lo = u32(d, p + 42), p + 46 + nl <= d.count else { throw EpubError.truncated }
            let nameData = d.subdata(in: (d.startIndex + p + 46)..<(d.startIndex + p + 46 + nl))
            let name = String(decoding: nameData, as: UTF8.self).replacingOccurrences(of: "\\", with: "/")
            out.append(ZipEntry(name: name, method: method, flags: flags, crc: UInt32(truncatingIfNeeded: crc), compressedSize: cs, size: us, localOffset: lo))
            p += 46 + nl + el + cl
        }
        return out
    }

    private static let crcTable: [UInt32] = (0..<256).map { n -> UInt32 in
        var c = UInt32(n)
        for _ in 0..<8 { c = (c & 1) != 0 ? 0xEDB88320 ^ (c >> 1) : c >> 1 }
        return c
    }
    public static func crc32(_ d: Data) -> UInt32 {
        var c: UInt32 = 0xFFFFFFFF
        for b in d { c = crcTable[Int((c ^ UInt32(b)) & 0xFF)] ^ (c >> 8) }
        return c ^ 0xFFFFFFFF
    }

    public static func read(_ d: Data, _ e: ZipEntry) throws -> Data {
        if e.flags & 1 != 0 { throw EpubError.unsupported("an encrypted file") }
        guard e.size <= maxEntryBytes else { throw EpubError.tooLarge(e.name) }
        guard u32(d, e.localOffset) == 0x04034b50, let nl = u16(d, e.localOffset + 26), let el = u16(d, e.localOffset + 28) else { throw EpubError.truncated }
        let start = e.localOffset + 30 + nl + el
        guard start >= 0, e.compressedSize >= 0, start + e.compressedSize <= d.count else { throw EpubError.truncated }
        let raw = d.subdata(in: (d.startIndex + start)..<(d.startIndex + start + e.compressedSize))
        var out: Data
        switch e.method {
        case 0: out = raw
        case 8:
            if e.size == 0 { out = Data(); break }
            var buf = [UInt8](repeating: 0, count: e.size)
            let n = raw.withUnsafeBytes { (src: UnsafeRawBufferPointer) -> Int in
                guard let sp = src.bindMemory(to: UInt8.self).baseAddress else { return 0 }
                return buf.withUnsafeMutableBufferPointer { compression_decode_buffer($0.baseAddress!, e.size, sp, raw.count, nil, COMPRESSION_ZLIB) }
            }
            guard n == e.size else { throw EpubError.corrupt(e.name) }
            out = Data(buf)
        default: throw EpubError.unsupported("compression method \(e.method)")
        }
        guard out.count == e.size else { throw EpubError.corrupt(e.name) }
        if crc32(out) != e.crc { throw EpubError.corrupt(e.name) }
        return out
    }
}

// MARK: Parsing the package

private final class ContainerParser: NSObject, XMLParserDelegate {
    var rootfile = ""
    func parser(_ p: XMLParser, didStartElement name: String, namespaceURI: String?, qualifiedName: String?, attributes a: [String: String]) {
        if Epub.local(name) == "rootfile", rootfile.isEmpty, let f = a["full-path"] { rootfile = f }
    }
}

private final class OpfParser: NSObject, XMLParserDelegate {
    struct Item { var href: String; var type: String; var props: String }
    var manifest: [String: Item] = [:]
    var spine: [(id: String, linear: Bool)] = []
    var title = "", author = "", language = ""
    private var text = ""; private var capture: String? = nil
    func parser(_ p: XMLParser, didStartElement name: String, namespaceURI: String?, qualifiedName: String?, attributes a: [String: String]) {
        switch Epub.local(name) {
        case "item": if let id = a["id"], let h = a["href"] { manifest[id] = Item(href: h, type: a["media-type"] ?? "", props: a["properties"] ?? "") }
        case "itemref": if let r = a["idref"] { spine.append((r, (a["linear"] ?? "yes") != "no")) }
        case "title", "creator", "language": capture = Epub.local(name); text = ""
        default: break
        }
    }
    func parser(_ p: XMLParser, foundCharacters s: String) { if capture != nil { text += s } }
    func parser(_ p: XMLParser, didEndElement name: String, namespaceURI: String?, qualifiedName: String?) {
        guard let c = capture, c == Epub.local(name) else { return }
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        switch c {
        case "title": if title.isEmpty { title = t }
        case "creator": if author.isEmpty { author = t }
        default: if language.isEmpty { language = t }
        }
        capture = nil
    }
}

// MARK: XHTML to blocks

final class XhtmlBlocks: NSObject, XMLParserDelegate {
    var blocks: [EpubBlock] = []
    var title = ""
    private var inBody = false, inTitle = false, inPre = false
    private var skip = 0
    private var cur = ""
    private var kind: String? = nil
    private var level = 0
    private var quote = 0
    private var lists: [(ordered: Bool, n: Int)] = []
    private var marker = ""
    private var cellStart = true

    private func flush() {
        let t = cur.trimmingCharacters(in: .whitespacesAndNewlines)
        cur = ""
        defer { kind = nil; level = 0; marker = "" }
        if t.isEmpty || t.allSatisfy({ "*`_ ".contains($0) }) { return }
        blocks.append(EpubBlock(kind: kind ?? (quote > 0 ? "quote" : "paragraph"), level: level, text: t, marker: marker))
    }

    func parser(_ p: XMLParser, didStartElement raw: String, namespaceURI: String?, qualifiedName: String?, attributes a: [String: String]) {
        let n = Epub.local(raw)
        if skip > 0 { skip += 1; return }
        switch n {
        case "script", "style", "svg", "math": skip = 1
        case "body": inBody = true
        case "title": if !inBody { inTitle = true }
        case "h1", "h2", "h3", "h4", "h5", "h6": flush(); kind = "heading"; level = Int(n.dropFirst()) ?? 2
        case "p": flush(); kind = quote > 0 ? "quote" : "paragraph"
        case "blockquote": flush(); quote += 1
        case "ul", "ol": flush(); lists.append((n == "ol", 0))
        case "li":
            flush(); kind = "item"
            if var l = lists.popLast() { l.n += 1; lists.append(l); marker = l.ordered ? "\(l.n). " : "- " } else { marker = "- " }
        case "pre": flush(); kind = "code"; inPre = true
        case "hr": flush(); blocks.append(EpubBlock(kind: "rule"))
        case "br": cur += "\n"
        case "img":
            let alt = (a["alt"] ?? "").trimmingCharacters(in: .whitespaces)
            cur += alt.isEmpty ? "[image]" : "[image: \(alt)]"
        case "em", "i", "cite": cur += "*"
        case "strong", "b": cur += "**"
        case "code": if !inPre { cur += "`" }
        case "tr": flush(); cellStart = true
        case "td", "th": if !cellStart { cur += " | " }; cellStart = false
        case "div", "section", "article", "aside", "figure", "figcaption", "dd", "dt", "table", "nav", "header", "footer": flush()
        default: break
        }
    }

    func parser(_ p: XMLParser, didEndElement raw: String, namespaceURI: String?, qualifiedName: String?) {
        let n = Epub.local(raw)
        if skip > 0 { skip -= 1; return }
        switch n {
        case "title": inTitle = false
        case "h1", "h2", "h3", "h4", "h5", "h6", "p", "li", "tr", "div", "section", "article", "aside", "figure", "figcaption", "dd", "dt", "table", "nav", "header", "footer": flush()
        case "pre": flush(); inPre = false
        case "blockquote": flush(); quote = max(0, quote - 1)
        case "ul", "ol": flush(); _ = lists.popLast()
        case "em", "i", "cite": cur += "*"
        case "strong", "b": cur += "**"
        case "code": if !inPre { cur += "`" }
        case "body": flush()
        default: break
        }
    }

    func parser(_ p: XMLParser, parseErrorOccurred parseError: Error) { flush() }

    func parser(_ p: XMLParser, foundCharacters s: String) {
        if skip > 0 { return }
        if inTitle { title += s; return }
        guard inBody else { return }
        if inPre { cur += s; return }
        var out = ""
        var lastSpace = cur.last.map { $0 == " " || $0 == "\n" } ?? true
        for ch in s {
            if ch.isWhitespace { if !lastSpace { out.append(" "); lastSpace = true } }
            else {
                if "\\*_`[]<>".contains(ch) { out.append("\\") }
                out.append(ch); lastSpace = false
            }
        }
        cur += out
    }
}

// MARK: Epub

public enum Epub {
    static func local(_ qualified: String) -> String {
        (qualified.split(separator: ":").last.map(String.init) ?? qualified).lowercased()
    }

    private static let entities: [String: String] = [
        "nbsp": "&#160;", "mdash": "&#8212;", "ndash": "&#8211;", "hellip": "&#8230;", "lsquo": "&#8216;", "rsquo": "&#8217;", "ldquo": "&#8220;",
        "rdquo": "&#8221;", "copy": "&#169;", "reg": "&#174;", "trade": "&#8482;", "eacute": "&#233;", "egrave": "&#232;", "agrave": "&#224;",
        "aacute": "&#225;", "ccedil": "&#231;", "ouml": "&#246;", "uuml": "&#252;", "auml": "&#228;", "ntilde": "&#241;", "times": "&#215;",
        "middot": "&#183;", "bull": "&#8226;", "deg": "&#176;", "laquo": "&#171;", "raquo": "&#187;", "shy": "&#173;", "thinsp": "&#8201;",
        "ensp": "&#8194;", "emsp": "&#8195;", "euro": "&#8364;", "pound": "&#163;", "para": "&#182;", "sect": "&#167;", "dagger": "&#8224;",
    ]

    /// Make an XHTML document safe for XMLParser: no DOCTYPE, HTML named entities as numeric references, unknown ones dropped.
    static func cleanXHTML(_ s: String) -> String {
        var t = s
        if let r = t.range(of: "<!DOCTYPE[^>]*>", options: [.regularExpression, .caseInsensitive]) { t.removeSubrange(r) }
        guard t.contains("&") else { return t }
        let re = try! NSRegularExpression(pattern: "&([A-Za-z][A-Za-z0-9]*);")
        let ns = t as NSString
        var out = ""; var last = 0
        for m in re.matches(in: t, range: NSRange(location: 0, length: ns.length)) {
            out += ns.substring(with: NSRange(location: last, length: m.range.location - last))
            let name = ns.substring(with: m.range(at: 1))
            if ["amp", "lt", "gt", "quot", "apos"].contains(name) { out += ns.substring(with: m.range) }
            else if let rep = entities[name] { out += rep }
            last = m.range.location + m.range.length
        }
        out += ns.substring(from: last)
        return out
    }

    static func resolve(_ base: String, _ href: String) -> String {
        let h = (href.split(separator: "#", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? href).removingPercentEncoding ?? href
        var parts = base.isEmpty ? [] : base.split(separator: "/").map(String.init)
        for c in h.split(separator: "/") {
            if c == "." { continue }
            if c == ".." { if !parts.isEmpty { parts.removeLast() }; continue }
            parts.append(String(c))
        }
        return parts.joined(separator: "/")
    }

    public static func parse(data: Data) throws -> EpubBook {
        let entries = try Zip.entries(data)
        var byName: [String: ZipEntry] = [:], byLower: [String: ZipEntry] = [:]
        for e in entries where !e.name.hasSuffix("/") && !e.name.hasPrefix("/") && !e.name.split(separator: "/").contains("..") {
            byName[e.name] = e; byLower[e.name.lowercased()] = e
        }
        func find(_ n: String) -> ZipEntry? { byName[n] ?? byLower[n.lowercased()] }
        func xml(_ n: String) throws -> String {
            guard let e = find(n) else { throw EpubError.missing(n) }
            return String(decoding: try Zip.read(data, e), as: UTF8.self)
        }

        let cp = ContainerParser()
        let cx = XMLParser(data: Data(try xml("META-INF/container.xml").utf8)); cx.delegate = cp; cx.parse()
        guard !cp.rootfile.isEmpty else { throw EpubError.missing("the package file") }
        let opfPath = resolve("", cp.rootfile)
        let op = OpfParser()
        let ox = XMLParser(data: Data(cleanXHTML(try xml(opfPath)).utf8)); ox.delegate = op
        guard ox.parse() || !op.spine.isEmpty else { throw EpubError.corrupt("the package file") }
        let base = opfPath.split(separator: "/").dropLast().joined(separator: "/")

        var warnings: [String] = []
        var chapters: [EpubChapter] = []
        let readable = op.spine.compactMap { ref -> (String, OpfParser.Item)? in
            guard let item = op.manifest[ref.id], !item.props.split(separator: " ").contains("nav"),
                  item.type.contains("html") || item.type.contains("xml") || item.type.isEmpty else { return nil }
            return (ref.id, item)
        }
        if readable.isEmpty { throw EpubError.missing("the reading order") }
        for (id, item) in readable.prefix(400) {
            let path = resolve(base, item.href)
            guard let e = find(path) else { warnings.append("missing \(path)"); continue }
            guard let raw = try? Zip.read(data, e) else { warnings.append("could not read \(path)"); continue }
            let h = XhtmlBlocks()
            let px = XMLParser(data: Data(cleanXHTML(String(decoding: raw, as: UTF8.self)).utf8)); px.delegate = h
            if !px.parse() { warnings.append("\(path) is not well formed; showing what could be read") }
            if h.blocks.isEmpty { continue }
            let headTitle = h.blocks.first(where: { $0.kind == "heading" })?.text
            let t = (headTitle ?? (h.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? "Chapter \(chapters.count + 1)" : h.title.trimmingCharacters(in: .whitespacesAndNewlines)))
                .replacingOccurrences(of: "\\", with: "").replacingOccurrences(of: "*", with: "").replacingOccurrences(of: "`", with: "")
            chapters.append(EpubChapter(id: id, href: path, title: t, blocks: h.blocks))
        }
        return EpubBook(title: op.title, author: op.author, language: op.language, chapters: chapters, warnings: warnings)
    }

    /// The reader's typography needs blocks; until lane 3's reader takes them, a chapter is Markdown for `text markup="markdown"`.
    public static func markdown(_ blocks: [EpubBlock]) -> String {
        var out = ""; var prevItem = false
        for b in blocks {
            var piece: String
            switch b.kind {
            case "heading": piece = String(repeating: "#", count: min(max(b.level, 1), 6)) + " " + b.text
            case "quote": piece = b.text.split(separator: "\n", omittingEmptySubsequences: false).map { "> " + $0 }.joined(separator: "\n")
            case "item": piece = b.marker + b.text.replacingOccurrences(of: "\n", with: " ")
            case "rule": piece = "---"
            case "code": piece = "```\n" + b.text + "\n```"
            default:
                var t = b.text.replacingOccurrences(of: "\n", with: "  \n")
                if let f = t.first, "#-+>".contains(f) || (f.isNumber && t.drop(while: { $0.isNumber }).hasPrefix(". ")) { t = "\\" + t }
                piece = t
            }
            let isItem = b.kind == "item"
            if !out.isEmpty { out += (isItem && prevItem) ? "\n" : "\n\n" }
            out += piece; prevItem = isItem
        }
        return out
    }
}
