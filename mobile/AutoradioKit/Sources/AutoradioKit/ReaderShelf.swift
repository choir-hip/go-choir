import Foundation
import CryptoKit
#if canImport(PDFKit)
import PDFKit
#endif

// The Reader: EPUBs and PDFs the listener imported, kept on the phone, read on screen, read aloud and searched.
// Files live in Application Support/Autoradio/reader/<id>/ (original, text.json, index.json) with one index.json of documents.
// No third-party code: Kit/Epub.swift parses books (ZIP + Compression), PDFKit reads PDFs, the index is a token -> blocks map.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. Not an EPUB or PDF (by name or by first bytes): refused with a sentence, nothing stored.
//  2. A PDF with no text layer (a scan): refused with "This PDF has no text to read" (OCR is out of scope).
//  3. A protected EPUB (encryption.xml covering content), a damaged or truncated file: refused with the Epub error's sentence.
//  4. The same file imported twice: one document (the id is the SHA-256 of the bytes), the second import returns the first.
//  5. A failure half-way (disk full, parse error): the document's folder is removed; the index never names a half-written one.
//  6. A position beyond the end, or negative: clamped; a document removed while its position is saved: ignored.
//  7. Search: empty query, only punctuation, case, accents, a phrase, a word prefix as the last word; a result always names a
//     document that exists and a block inside it.
//  8. A corrupt reader index or text file: the document is skipped (never a crash); the others still open.
//  9. A huge file (over 100 MB): refused before it is read.

/// The write options for every shelf file: encrypted at rest, readable after the first unlock. (Asserted by a test; the platform applies it.)
public enum ShelfFiles { public static let writeOptions: Data.WritingOptions = [.atomic, .completeFileProtectionUntilFirstUserAuthentication] }

public enum ShelfError: Error, CustomStringConvertible, Equatable {
    case notSupported, noText, tooLarge, unreadable(String), protected, download
    public var description: String {
        switch self {
        case .notSupported: return "Only EPUB and PDF files can be added to Reader."
        case .noText: return "This PDF has no text to read."
        case .tooLarge: return "This file is too large to add (the limit is 100 MB)."
        case .unreadable(let m): return m.isEmpty ? "This file could not be read." : m
        case .protected: return "This book is protected and cannot be opened."
        case .download: return "The file could not be downloaded."
        }
    }
}

public struct ShelfDoc: Codable, Equatable, Sendable {
    public var id: String
    public var title: String
    public var author: String
    /// pdf | epub
    public var kind: String
    public var added: Date
    /// The block the listener is at (an index into the document's blocks, headings included).
    public var position: Int
    /// The number of blocks.
    public var length: Int
    /// Pages (PDF) or chapters (EPUB).
    public var sections: Int
    public var fileName: String
    /// Where a document came from when it came from the Files place: the file's path on the computer, and the size and modified time it had
    /// then. Opening the same file again finds this document; a changed file replaces it. Nil for a document added any other way.
    public var origin: String?
    public var etag: String?
    public var address: String { ShelfAddress.address(id) }
    public var progress: Double { length > 1 ? min(1, max(0, Double(position) / Double(length - 1))) : 0 }
}

public struct DocBlock: Codable, Equatable, Sendable {
    /// heading | paragraph | quote | item | code
    public var kind: String
    public var level: Int
    public var text: String
    public var marker: String
    public var section: Int
}

public struct DocSection: Codable, Equatable, Sendable {
    public var title: String
    public var start: Int
    public var count: Int
}

public struct DocText: Codable, Equatable, Sendable {
    public var sections: [DocSection]
    public var blocks: [DocBlock]
    public func section(of block: Int) -> Int {
        guard !blocks.isEmpty else { return 0 }
        return blocks[min(max(block, 0), blocks.count - 1)].section
    }
}

/// A passage found in a document (for the page's find, the launcher and the ask lane).
public struct ShelfPassage: Equatable, Sendable {
    public var docId: String
    public var title: String
    public var block: Int
    public var section: Int
    public var text: String
    public var snippet: String
}

public enum ShelfAddress {
    public static func address(_ id: String) -> String { "doc:" + id }
    /// "doc:<id>" or "doc:<id>@<block>".
    public static func parse(_ address: String) -> (id: String, block: Int?)? {
        guard address.hasPrefix("doc:") else { return nil }
        let rest = String(address.dropFirst(4))
        let parts = rest.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).map(String.init)
        guard let id = parts.first, !id.isEmpty else { return nil }
        return (id, parts.count == 2 ? Int(parts[1]) : nil)
    }
}

/// A search result that opened a document at a block: held until the document's page reads it once.
public final class ShelfJump: @unchecked Sendable {
    public static let shared = ShelfJump()
    private let lock = NSLock()
    private var pending: [String: Int] = [:]
    public func set(_ id: String, block: Int) { lock.lock(); pending[id] = block; lock.unlock() }
    public func take(_ id: String) -> Int? { lock.lock(); defer { lock.unlock() }; return pending.removeValue(forKey: id) }
}

/// The block the radio is reading now (set by whoever plays a document; the page lights it and follows it).
public final class ShelfFollow: @unchecked Sendable {
    public static let shared = ShelfFollow()
    private let lock = NSLock()
    private var id = "", block = -1
    public var onChange: (@Sendable () -> Void)?
    public var current: (id: String, block: Int)? { lock.lock(); defer { lock.unlock() }; return id.isEmpty ? nil : (id, block) }
    public func set(id: String, block: Int) {
        lock.lock(); let same = self.id == id && self.block == block; self.id = id; self.block = block; lock.unlock()
        if !same { ReaderShelf.shared.setPosition(id: id, block: block); onChange?() }
    }
    public func clear() { lock.lock(); let was = !id.isEmpty; id = ""; block = -1; lock.unlock(); if was { onChange?() } }
}

// MARK: text and index

public struct DocIndex: Codable, Equatable, Sendable {
    public var postings: [String: [Int]]

    /// Lower case, accents and widths folded, anything that is not a letter or digit is a separator.
    public static func tokens(_ s: String) -> [String] {
        let f = s.folding(options: [.caseInsensitive, .diacriticInsensitive, .widthInsensitive], locale: nil)
        var out: [String] = [], cur = ""
        for ch in f {
            if ch.isLetter || ch.isNumber { cur.append(ch) } else if !cur.isEmpty { out.append(cur); cur = "" }
        }
        if !cur.isEmpty { out.append(cur) }
        return out
    }

    public static func build(_ blocks: [DocBlock]) -> DocIndex {
        var map: [String: [Int]] = [:]
        for (i, b) in blocks.enumerated() {
            for t in Set(tokens(b.text)) { map[t, default: []].append(i) }
        }
        return DocIndex(postings: map)
    }

    /// Blocks holding every word of the query (the last word may be the start of a word), phrase matches first.
    public func search(_ query: String, in blocks: [DocBlock], limit: Int) -> [Int] {
        let q = Self.tokens(query)
        guard !q.isEmpty, limit > 0 else { return [] }
        var candidates: Set<Int>? = nil
        for (n, t) in q.enumerated() {
            var hit = Set<Int>()
            if n == q.count - 1 { for (k, v) in postings where k.hasPrefix(t) { hit.formUnion(v) } } else if let v = postings[t] { hit.formUnion(v) }
            candidates = candidates.map { $0.intersection(hit) } ?? hit
            if candidates!.isEmpty { return [] }
        }
        let phrase = q.joined(separator: " ")
        var scored: [(Int, Int)] = []
        for i in candidates ?? [] where blocks.indices.contains(i) {
            let norm = Self.tokens(blocks[i].text).joined(separator: " ")
            scored.append((i, q.count > 1 && norm.contains(phrase) ? 2 : (q.count == 1 ? 2 : 1)))
        }
        scored.sort { $0.1 != $1.1 ? $0.1 > $1.1 : $0.0 < $1.0 }
        return scored.prefix(limit).map(\.0)
    }
}

public enum DocExtract {
    /// An EPUB block's text has `*em*`, `**strong**`, backticks and backslash escapes; the reader keeps plain words.
    public static func plain(_ s: String) -> String {
        var out = ""; var it = s.makeIterator()
        while let c = it.next() {
            if c == "\\" { if let n = it.next() { out.append(n) } } else if c == "*" || c == "`" { continue } else { out.append(c) }
        }
        return out.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    public static func epub(_ book: EpubBook) -> DocText {
        var sections: [DocSection] = [], blocks: [DocBlock] = []
        for ch in book.chapters {
            let start = blocks.count, s = sections.count
            for b in ch.blocks where b.kind != "rule" {
                let t = plain(b.text)
                // An image has no words; it is left out (tonight the reader shows text only).
                if t.isEmpty || (t.hasPrefix("[image") && t.hasSuffix("]")) { continue }
                blocks.append(DocBlock(kind: b.kind, level: b.level, text: t, marker: b.marker, section: s))
            }
            if blocks.count > start { sections.append(DocSection(title: ch.title, start: start, count: blocks.count - start)) }
        }
        return DocText(sections: sections, blocks: blocks)
    }

    /// One page's text as paragraphs: a blank line ends a paragraph; otherwise lines are joined (a hyphen at the end of a line
    /// joins the word); a page with no blank lines is cut at sentence ends into pieces of about 500 characters.
    public static func paragraphs(_ page: String) -> [String] {
        let text = page.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        func join(_ chunk: String) -> String {
            var out = ""
            for line in chunk.split(separator: "\n", omittingEmptySubsequences: true) {
                let l = line.trimmingCharacters(in: .whitespaces)
                if l.isEmpty { continue }
                if out.hasSuffix("-"), let f = l.first, f.isLowercase { out.removeLast(); out += l }
                else if out.isEmpty { out = l } else { out += " " + l }
            }
            return out
        }
        var parts = text.components(separatedBy: "\n\n").map(join).filter { !$0.isEmpty }
        if parts.count == 1, parts[0].count > 900 { parts = cut(parts[0], about: 500) }
        return parts
    }

    static func cut(_ s: String, about n: Int) -> [String] {
        var out: [String] = [], cur = ""
        var sentence = ""
        for ch in s {
            sentence.append(ch)
            if ".!?".contains(ch) { cur += sentence; sentence = ""; if cur.count >= n { out.append(cur.trimmingCharacters(in: .whitespaces)); cur = "" } }
        }
        cur += sentence
        if !cur.trimmingCharacters(in: .whitespaces).isEmpty { out.append(cur.trimmingCharacters(in: .whitespaces)) }
        return out
    }

    /// Pages to text: a section per page that has any (titled "Page N" with N the real page number).
    public static func pages(_ pageTexts: [String]) -> DocText {
        var sections: [DocSection] = [], blocks: [DocBlock] = []
        for (i, t) in pageTexts.enumerated() {
            let ps = paragraphs(t)
            guard !ps.isEmpty else { continue }
            let start = blocks.count
            for p in ps { blocks.append(DocBlock(kind: "paragraph", level: 0, text: p, marker: "", section: sections.count)) }
            sections.append(DocSection(title: "Page \(i + 1)", start: start, count: ps.count))
        }
        return DocText(sections: sections, blocks: blocks)
    }

    #if canImport(PDFKit)
    /// Text of each page of the PDF in `data` (an empty string for a page with no text layer), with its title and author.
    public static func pdf(_ data: Data) throws -> (title: String, author: String, pages: [String]) {
        guard let d = PDFDocument(data: data) else { throw ShelfError.unreadable("This PDF could not be read.") }
        if d.isLocked { throw ShelfError.unreadable("This PDF is locked and cannot be opened.") }
        var out: [String] = []
        for i in 0..<d.pageCount { out.append(d.page(at: i)?.string ?? "") }
        let a = d.documentAttributes ?? [:]
        return ((a[PDFDocumentAttribute.titleAttribute] as? String) ?? "", (a[PDFDocumentAttribute.authorAttribute] as? String) ?? "", out)
    }
    #endif
}

// MARK: the store

public final class ReaderShelf: @unchecked Sendable {
    public static let maxBytes = 100_000_000
    /// The app's reader; the host sets it at launch (the default is an in-memory-safe temporary folder).
    public static var shared = ReaderShelf(root: FileManager.default.temporaryDirectory.appendingPathComponent("autoradio-reader-default"))

    let root: URL
    private let lock = NSLock()
    private var docs: [ShelfDoc] = []
    private var cache: [String: (DocText, DocIndex)] = [:]
    public var onChange: (@Sendable () -> Void)?

    public init(root: URL) {
        self.root = root
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        Self.protect(root)
        var v = URLResourceValues(); v.isExcludedFromBackup = true       // the folder's flag covers everything inside it
        var r = root; try? r.setResourceValues(v)
        loadIndex()
    }

    /// The content store's norm: the protection class on everything the reader keeps (imported books and files are the listener's own).
    static func protect(_ url: URL) {
        try? FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: url.path)
    }

    /// Documents whose text and index stay in memory (most recently used); the rest are read from disk when opened.
    static let cacheLimit = 3
    private var recent: [String] = []
    private func remember(_ id: String, _ v: (DocText, DocIndex)) {         // caller holds the lock
        cache[id] = v; recent.removeAll { $0 == id }; recent.append(id)
        while recent.count > Self.cacheLimit { cache[recent.removeFirst()] = nil }
    }
    var cachedCount: Int { lock.lock(); defer { lock.unlock() }; return cache.count }

    private var indexFile: URL { root.appendingPathComponent("index.json") }
    private func dir(_ id: String) -> URL { root.appendingPathComponent(id, isDirectory: true) }

    private func enc() -> JSONEncoder { let e = JSONEncoder(); e.dateEncodingStrategy = .iso8601; return e }
    private func dec() -> JSONDecoder { let d = JSONDecoder(); d.dateDecodingStrategy = .iso8601; return d }

    private func loadIndex() {
        guard let d = try? Data(contentsOf: indexFile), let l = try? dec().decode([ShelfDoc].self, from: d) else { return }
        docs = l.filter { FileManager.default.fileExists(atPath: dir($0.id).appendingPathComponent("text.json").path) }
    }
    private func saveIndexLocked() {
        if let d = try? enc().encode(docs) { try? d.write(to: indexFile, options: ShelfFiles.writeOptions) }
    }

    // MARK: reading

    public func list() -> [ShelfDoc] { lock.lock(); defer { lock.unlock() }; return docs.sorted { $0.added > $1.added } }
    public func doc(_ id: String) -> ShelfDoc? { lock.lock(); defer { lock.unlock() }; return docs.first { $0.id == id } }
    public func fileURL(_ id: String) -> URL? {
        guard let d = doc(id) else { return nil }
        let u = dir(id).appendingPathComponent("original." + d.kind)
        return FileManager.default.fileExists(atPath: u.path) ? u : nil
    }

    private func loaded(_ id: String) -> (DocText, DocIndex)? {
        lock.lock()
        if let c = cache[id] { recent.removeAll { $0 == id }; recent.append(id); lock.unlock(); return c }
        let known = docs.contains { $0.id == id }
        lock.unlock()
        guard known, let td = try? Data(contentsOf: dir(id).appendingPathComponent("text.json")), let t = try? JSONDecoder().decode(DocText.self, from: td) else { return nil }
        let idx = (try? Data(contentsOf: dir(id).appendingPathComponent("index.json"))).flatMap { try? JSONDecoder().decode(DocIndex.self, from: $0) } ?? DocIndex.build(t.blocks)
        lock.lock(); remember(id, (t, idx)); lock.unlock()
        return (t, idx)
    }
    public func text(_ id: String) -> DocText? { loaded(id)?.0 }

    /// The words to read aloud from `block` to the end, one string per block (a heading is read as its own paragraph).
    public func blocks(id: String, from block: Int) -> [String] {
        guard let t = text(id), !t.blocks.isEmpty else { return [] }
        return t.blocks[min(max(block, 0), t.blocks.count - 1)...].map(\.text)
    }

    // MARK: position

    public func setPosition(id: String, block: Int) {
        lock.lock()
        guard let i = docs.firstIndex(where: { $0.id == id }) else { lock.unlock(); return }
        let p = min(max(block, 0), max(0, docs[i].length - 1))
        let changed = docs[i].position != p
        docs[i].position = p
        if changed { saveIndexLocked() }
        lock.unlock()
        if changed { onChange?() }
    }

    // MARK: importing

    private static func kind(name: String, data: Data) -> String? {
        let ext = (name as NSString).pathExtension.lowercased()
        let isPDF = data.prefix(5) == Data("%PDF-".utf8) || data.prefix(1024).range(of: Data("%PDF-".utf8)) != nil
        let isZip = data.prefix(2) == Data("PK".utf8)
        if ext == "pdf" && isPDF { return "pdf" }
        if ext == "epub" && isZip { return "epub" }
        if ext.isEmpty || !["pdf", "epub"].contains(ext) { if isPDF && ext == "pdf" { return "pdf" } }
        return nil
    }

    /// Copy the file into the reader, read its text and build its index. Heavy: call it off the main thread.
    @discardableResult
    public func importFile(at url: URL, name: String? = nil) throws -> ShelfDoc {
        let fileName = name ?? url.lastPathComponent
        let size = (try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        guard size <= Self.maxBytes else { throw ShelfError.tooLarge }
        guard let data = try? Data(contentsOf: url, options: .mappedIfSafe) else { throw ShelfError.unreadable("") }
        guard let kind = Self.kind(name: fileName, data: data) else { throw ShelfError.notSupported }
        let id = SHA256.hash(data: data).prefix(6).map { String(format: "%02x", $0) }.joined()
        if let existing = doc(id) { return existing }

        var title = "", author = ""
        let text: DocText
        if kind == "epub" {
            do {
                let book = try Epub.parse(data: data)
                title = book.title; author = book.author
                text = DocExtract.epub(book)
            } catch EpubError.protected { throw ShelfError.protected }
            catch let e as EpubError { throw ShelfError.unreadable(e.description) }
            if text.blocks.isEmpty { throw ShelfError.unreadable("This book has no readable chapters.") }
        } else {
            #if canImport(PDFKit)
            let r = try DocExtract.pdf(data)
            title = r.title; author = r.author
            text = DocExtract.pages(r.pages)
            if text.blocks.isEmpty { throw ShelfError.noText }
            #else
            throw ShelfError.notSupported
            #endif
        }
        if title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { title = ((fileName as NSString).deletingPathExtension).replacingOccurrences(of: "_", with: " ") }
        let folder = dir(id)
        do {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            Self.protect(folder)
            let original = folder.appendingPathComponent("original." + kind)
            try FileManager.default.copyItem(at: url, to: original)
            Self.protect(original)
            try JSONEncoder().encode(text).write(to: folder.appendingPathComponent("text.json"), options: ShelfFiles.writeOptions)
            let index = DocIndex.build(text.blocks)
            try JSONEncoder().encode(index).write(to: folder.appendingPathComponent("index.json"), options: ShelfFiles.writeOptions)
            let d = ShelfDoc(id: id, title: title.trimmingCharacters(in: .whitespacesAndNewlines), author: author.trimmingCharacters(in: .whitespacesAndNewlines),
                               kind: kind, added: Date(), position: 0, length: text.blocks.count, sections: text.sections.count, fileName: fileName)
            lock.lock(); docs.append(d); remember(id, (text, index)); saveIndexLocked(); lock.unlock()
            onChange?()
            return d
        } catch {
            try? FileManager.default.removeItem(at: folder)
            throw ShelfError.unreadable("The file could not be stored.")
        }
    }

    /// The document that came from this Files path, if any.
    public func doc(origin: String) -> ShelfDoc? { lock.lock(); defer { lock.unlock() }; return docs.first { $0.origin == origin } }

    /// A document now also comes from this Files path (the same bytes, whoever brought them first). Any other copy that came from that path
    /// (an older version of the file) is removed: the changed file replaces it.
    @discardableResult
    public func adopt(id: String, origin: String, etag: String) -> ShelfDoc? {
        lock.lock()
        guard let i = docs.firstIndex(where: { $0.id == id }) else { lock.unlock(); return nil }
        docs[i].origin = origin; docs[i].etag = etag
        let stale = docs.filter { $0.origin == origin && $0.id != id }.map(\.id)
        saveIndexLocked(); let d = docs[i]
        lock.unlock()
        for old in stale { remove(id: old) }
        onChange?()
        return d
    }

    public func remove(id: String) {
        lock.lock()
        docs.removeAll { $0.id == id }; cache[id] = nil; recent.removeAll { $0 == id }; saveIndexLocked()
        lock.unlock()
        try? FileManager.default.removeItem(at: dir(id))
        onChange?()
    }

    /// Whether a URL names something the reader can take (a file or http(s) address ending in .pdf or .epub).
    public static func canImport(_ url: URL) -> Bool {
        ["pdf", "epub"].contains(url.pathExtension.lowercased()) && (url.isFileURL || ["http", "https"].contains(url.scheme?.lowercased() ?? ""))
    }

    /// The one entry the bar's URL handling calls: a file URL is copied in, an http(s) URL ending in .pdf or .epub is downloaded first.
    @discardableResult
    public static func importFrom(url: URL) async throws -> ShelfDoc {
        guard canImport(url) else { throw ShelfError.notSupported }
        if url.isFileURL { return try await Task.detached { try shared.importFile(at: url) }.value }
        let tmp: URL
        do {
            let (t, resp) = try await URLSession.shared.download(from: url)
            if let h = resp as? HTTPURLResponse, !(200..<300).contains(h.statusCode) { throw ShelfError.download }
            tmp = t
        } catch let e as ShelfError { throw e } catch { throw ShelfError.download }
        defer { try? FileManager.default.removeItem(at: tmp) }
        let name = url.lastPathComponent.removingPercentEncoding ?? url.lastPathComponent
        return try await Task.detached { try shared.importFile(at: tmp, name: name) }.value
    }

    // MARK: search

    private func snippet(_ text: String, query: String) -> String {
        let one = text.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ")
        guard let word = DocIndex.tokens(query).first, let r = one.range(of: word, options: [.caseInsensitive, .diacriticInsensitive]) else {
            return one.count > 140 ? String(one.prefix(139)) + "…" : one
        }
        let at = one.distance(from: one.startIndex, to: r.lowerBound)
        let from = max(0, at - 60), to = min(one.count, at + 80)
        let a = one.index(one.startIndex, offsetBy: from), b = one.index(one.startIndex, offsetBy: to)
        return (from > 0 ? "…" : "") + String(one[a..<b]) + (to < one.count ? "…" : "")
    }

    /// Passages of one document holding the query.
    public func find(in id: String, query: String, limit: Int = 50) -> [ShelfPassage] {
        guard let d = doc(id), let (t, idx) = loaded(id) else { return [] }
        return idx.search(query, in: t.blocks, limit: limit).map { i in
            ShelfPassage(docId: id, title: d.title, block: i, section: t.blocks[i].section, text: t.blocks[i].text, snippet: snippet(t.blocks[i].text, query: query))
        }
    }

    /// Passages across the reader (the ask lane answers from these). Best phrase matches first within a document; documents in reader order.
    public func passages(matching query: String, limit: Int) -> [ShelfPassage] {
        var out: [ShelfPassage] = []
        for d in list() {
            out += find(in: d.id, query: query, limit: limit)
            if out.count >= limit { break }
        }
        return Array(out.prefix(limit))
    }

    /// The launcher's rows: documents whose title or author matches, then passages from the body.
    public func hits(query: String, limit: Int = 8) -> [SearchHit] {
        let q = DocIndex.tokens(query).joined(separator: " ")
        guard !q.isEmpty else { return [] }
        var out: [SearchHit] = []
        for d in list() {
            let hay = DocIndex.tokens(d.title + " " + d.author).joined(separator: " ")
            if hay.contains(q) {
                out.append(SearchHit(address: d.address, title: d.title, kind: "document", detail: [d.author, d.kind == "pdf" ? "PDF" : "Book"].filter { !$0.isEmpty }.joined(separator: " · ")))
            }
        }
        for p in passages(matching: query, limit: limit) where out.count < limit {
            out.append(SearchHit(address: "doc:\(p.docId)@\(p.block)", title: p.title, kind: "document", detail: p.snippet))
        }
        return Array(out.prefix(limit))
    }
}

