import Foundation
import CryptoKit

// "My notebook" on the phone (station L6; docs/design/autoradio-learned-document-2026-11-17.md sections 2, 6, 7, 8): what Autoradio has been told and
// has learned about the listener, as the one Texture document it is. This file reads it as lines, writes the topics into it, says what the DJ is
// sent (`edition_text`), and builds the page: the lines, the scorecard, the rows behind a line, and the plain list of what is held about the listener.
//
// ASSUMED wire (the notebook writer is not built on the server yet): the document is found by `GET /api/texture/edition` (title "My notebook", "My
// edition" adopted; the route exists) and read and edited through the ordinary Texture routes (the reader and TextureEditor). The line grammar is
// the design's 2.2: `- <text> [L<n> · heard <h> of <m>]` (lesson), `[T<n> ...]` (trying), the listener's own lines untagged.

// MARK: Lines

public struct NotebookLine: Equatable, Sendable {
    public var id: String
    public var section: String
    public var text: String
    /// `L7` or `T2` when the line carries the server's tag.
    public var tag: String?
    /// `lesson:L7`, `try:T2`, `mine:<8 hex>` (the listener's own line), or nil (steering lines and unknown sections).
    public var ref: String?
    public var heard: Int?, of: Int?
}

public struct NotebookSection: Equatable, Sendable {
    public var key: String
    public var heading: String
    public var lines: [NotebookLine]
}

public struct NotebookDoc: Equatable, Sendable {
    public var title = ""
    public var sections: [NotebookSection] = []
    public func line(_ id: String) -> NotebookLine? { sections.flatMap(\.lines).first { $0.id == id } }
}

public enum NotebookText {
    public static let title = "My notebook"
    public static let askedHeading = "What you asked for"

    /// Letters and digits only, lower case: "U.S." and "us" are one topic (the shared code's `key`, edition-model.ts).
    static func topicKey(_ s: String) -> String { String(s.lowercased().filter { $0.isLetter || $0.isNumber }) }

    static func sectionKey(_ heading: String) -> String {
        let h = heading.lowercased()
        if h.contains("asked") || h.contains("edition") { return "asked" }
        if h.contains("learned") { return "learned" }
        if h.contains("trying") { return "trying" }
        if h.contains("steering") { return "steering" }
        return "other-" + NotebookText.topicKey(heading)
    }

    /// The first 8 hex digits of the SHA-256 of the line's trimmed text: the design's `mine:` ref.
    public static func mineRef(_ text: String) -> String {
        let d = SHA256.hash(data: Data(text.trimmingCharacters(in: .whitespacesAndNewlines).utf8))
        return "mine:" + d.prefix(4).map { String(format: "%02x", $0) }.joined()
    }

    private static let tagPattern = try! NSRegularExpression(pattern: #"\s\[([LT])(\d+) · heard (\d+) of (\d+)\]\s*$"#)

    public static func parse(_ md: String) -> NotebookDoc {
        var doc = NotebookDoc()
        var cur: NotebookSection?
        func close() { if let c = cur { doc.sections.append(c) }; cur = nil }
        for raw in md.replacingOccurrences(of: "\r\n", with: "\n").split(separator: "\n", omittingEmptySubsequences: false) {
            let line = raw.trimmingCharacters(in: .whitespaces)
            if line.hasPrefix("# "), !line.hasPrefix("## ") { if doc.title.isEmpty { doc.title = String(line.dropFirst(2)).trimmingCharacters(in: .whitespaces) }; continue }
            if line.hasPrefix("## ") {
                close()
                let h = String(line.dropFirst(3)).trimmingCharacters(in: .whitespaces)
                cur = NotebookSection(key: sectionKey(h), heading: h, lines: [])
                continue
            }
            guard cur != nil, line.hasPrefix("- ") || line.hasPrefix("* ") else { continue }
            var text = String(line.dropFirst(2)).trimmingCharacters(in: .whitespaces)
            guard !text.isEmpty else { continue }
            var l = NotebookLine(id: "\(cur!.key)-\(cur!.lines.count)", section: cur!.key, text: text, tag: nil, ref: nil, heard: nil, of: nil)
            let ns = text as NSString
            if let m = tagPattern.firstMatch(in: text, range: NSRange(location: 0, length: ns.length)) {
                let letter = ns.substring(with: m.range(at: 1)), n = ns.substring(with: m.range(at: 2))
                l.tag = letter + n; l.ref = (letter == "L" ? "lesson:" : "try:") + letter + n
                l.heard = Int(ns.substring(with: m.range(at: 3))); l.of = Int(ns.substring(with: m.range(at: 4)))
                text = ns.substring(to: m.range.location).trimmingCharacters(in: .whitespaces)
                l.text = text
            } else if cur!.key == "asked" {
                l.ref = mineRef(text)
            }
            cur!.lines.append(l)
        }
        close()
        return doc
    }

    /// The notebook with the topics among "What you asked for": a seed when there is no notebook, the missing topic lines appended to the section
    /// otherwise (the listener's lines and every other section untouched), the same text back when nothing is missing.
    public static func merging(topics: [String], into md: String?) -> String {
        var wanted: [String] = [], seen = Set<String>()
        for t in topics { let k = NotebookText.topicKey(t); if !k.isEmpty, seen.insert(k).inserted { wanted.append(t.trimmingCharacters(in: .whitespacesAndNewlines)) } }
        guard let md, !md.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return "# \(title)\n\n## \(askedHeading)\n" + wanted.map { "- \($0)\n" }.joined()
        }
        let have = Set(parse(md).sections.filter { $0.key == "asked" }.flatMap(\.lines).map { NotebookText.topicKey($0.text) })
        let add = wanted.filter { !have.contains(NotebookText.topicKey($0)) }
        guard !add.isEmpty else { return md }
        var lines = md.replacingOccurrences(of: "\r\n", with: "\n").split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        if let h = lines.firstIndex(where: { $0.hasPrefix("## ") && sectionKey(String($0.dropFirst(3))) == "asked" }) {
            var end = h + 1
            var lastBullet = h
            while end < lines.count, !lines[end].hasPrefix("## ") { if lines[end].hasPrefix("- ") || lines[end].hasPrefix("* ") { lastBullet = end }; end += 1 }
            lines.insert(contentsOf: add.map { "- \($0)" }, at: lastBullet + 1)
        } else {
            let at = lines.firstIndex { $0.hasPrefix("## ") } ?? lines.count
            let block = ["## \(askedHeading)"] + add.map { "- \($0)" } + [""]
            lines.insert(contentsOf: block, at: at)
        }
        return lines.joined(separator: "\n")
    }
}

// MARK: What the DJ is sent

public enum EditionText {
    public static let maxBytes = 32 * 1024
    /// The notebook text when the phone holds it, else the topics as text, else nil.
    public static func make(notebook: String?, topics: [String]) -> String? {
        if let n = notebook?.trimmingCharacters(in: .whitespacesAndNewlines), !n.isEmpty { return StretchCandidates.cut(n, bytes: maxBytes) }
        let t = topics.map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
        guard !t.isEmpty else { return nil }
        return StretchCandidates.cut("What the listener follows:\n" + t.map { "- \($0)" }.joined(separator: "\n"), bytes: maxBytes)
    }
    public static func current() -> String? {
        make(notebook: NotebookCache.shared.text, topics: EditionPicksMirror.shared.topics)    // the picks are shared code's; this is its pushed copy (EditionSetup.swift)
    }
}

/// The last notebook text read, kept on the phone for the programme request and for the page when the computer cannot be reached.
public final class NotebookCache: @unchecked Sendable {
    public static let shared = NotebookCache(file: TakeFiles.url("notebook-cache.md"))
    private let lock = NSLock()
    private var _text: String?
    private let file: URL?
    public init(file: URL?) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file) { _text = String(decoding: d, as: UTF8.self) }
    }
    public var text: String? { lock.lock(); defer { lock.unlock() }; return _text }
    /// Sign-out: the text and the file.
    public func wipe() {
        lock.lock(); _text = nil; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    public func set(_ t: String) {
        lock.lock(); _text = t; lock.unlock()
        guard let file else { return }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? Data(t.utf8).write(to: file, options: .atomic)
    }
}

// MARK: The topics write: the one part of the notebook that stays native
// The page, the scorecard and the learning route are shared code (AutoradioLab/notebook.ts, notebook-run.ts). What stays here is the WRITE of the listener's topics into the notebook:
// a Texture canonical write through TextureEditor (Swift until the texture edit page moves), so it is a red surface and the shared code asks for it through the one native call
// `notebookState` (AutoradioModule+Notebook.swift). The text read and the cache below feed the DJ's request (RadioProgramme.swift), which is built natively.

@MainActor public final class NotebookStore {
    public static let shared = NotebookStore()
    /// nil when the server cannot say (a timeout, a 401, a 5xx): that is not "there is no notebook".
    public var lookupEdition: () async -> EditionInfo? = { await EditionLookup.fetch() }
    /// Makes the notebook document; its id, or nil when it could not be made.
    public var createNotebook: () async -> String? = { await NotebookStore.defaultCreate() }
    /// Where the notebook text is kept on the phone (tests pass their own).
    public var notebookCache: NotebookCache = .shared
    public init() {}

    static func defaultCreate() async -> String? {
        do {
            let r = try await ChoirEnvironment.shared.client().postJSON("/api/texture/documents", ["title": NotebookText.title], route: "/api/texture/documents")
            guard let id = (try r.object())["doc_id"] as? String, !id.isEmpty else { return nil }
            return id
        } catch { return nil }
    }

    private var writing: Task<Bool, Never>?
    /// Sign-out: a write in flight is cancelled.
    public func wipe() { writing?.cancel(); writing = nil }

    /// The topics written on this phone and not yet in the notebook (the marker says when they are): written now, once. Asked for by the shared page on every read.
    public func writePendingTopics() async {
        let topics = EditionPicksMirror.shared.topics      // the picks are shared code's; this is its pushed copy (EditionSetup.swift)
        if EditionPicksMirror.shared.known, !topics.isEmpty { await writeTopics(topics) }
    }

    // MARK: The topics move in (design 2, "The topic list")

    /// Writes the listener's topics as lines of "What you asked for", once: through the ordinary owner revision route (an edit by the listener), creating
    /// the document first when there is none. The local topics file stays as a cache. True when the notebook holds them afterwards.
    @discardableResult
    public func writeTopics(_ topics: [String], marker: URL? = nil, generation: Int? = nil) async -> Bool {
        if let g = generation, g != AccountWipe.generation { return false }        // the account moved on between reading the topics and now: they are not this account's to write
        if let w = writing { return await w.value }      // one in flight: a second would also find nothing and make a second document
        let t = Task { @MainActor in await self.writeTopicsNow(topics, marker: marker, generation: generation) }
        writing = t
        let r = await t.value
        writing = nil
        return r
    }

    private func writeTopicsNow(_ topics: [String], marker: URL?, generation: Int?) async -> Bool {
        let marker = marker ?? TakeFiles.url("notebook-topics-written")
        let key = topics.map { NotebookText.topicKey($0) }.sorted().joined(separator: ",")
        guard !key.isEmpty else { return true }
        if let marker, (try? String(contentsOf: marker, encoding: .utf8)) == key { return true }
        let client = ChoirEnvironment.shared.client()
        guard client.hasKey else { return false }
        var address = "", current: String?
        let gen = generation ?? AccountWipe.generation          // every write below is dropped if the account was wiped meanwhile (the generation the topics were read under, when given)
        guard gen == AccountWipe.generation else { return false }
        guard let ed = await lookupEdition(), gen == AccountWipe.generation else { return false }        // the server cannot say: try later, never create on a guess
        if ed.found {
            address = "texture:" + ed.docID
            let d = await ReaderStore.shared.doc(address)
            guard d.status == "ready", gen == AccountWipe.generation else { return false }
            current = d.source
        } else {
            guard let id = await createNotebook(), gen == AccountWipe.generation else { return false }
            address = "texture:" + id
        }
        let merged = NotebookText.merging(topics: topics, into: current)
        guard gen == AccountWipe.generation else { return false }
        if merged != current {
            await TextureEditor.shared.edit(address: address, markdown: merged)
            await TextureEditor.shared.flush(address: address)
            guard TextureEditor.shared.info(address: address).status == "saved" else { return false }
        }
        guard gen == AccountWipe.generation else { return false }
        notebookCache.set(merged)
        if let marker { try? FileManager.default.createDirectory(at: marker.deletingLastPathComponent(), withIntermediateDirectories: true); try? Data(key.utf8).write(to: marker, options: .atomic) }
        return true
    }
}
