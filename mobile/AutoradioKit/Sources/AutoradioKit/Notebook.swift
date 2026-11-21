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

// MARK: The page

public enum NotebookDocLoad {
    case loading
    case found(address: String, notebook: NotebookDoc)
    /// The computer answered and there is no notebook yet.
    case none
    /// No answer.
    case unavailable
}

public struct NotebookPageInput {
    public var doc: NotebookDocLoad
    public var learning: LearningLoad?
    public var selected = ""
    public var steerQueued = 0
    public init(doc: NotebookDocLoad, learning: LearningLoad?, selected: String = "", steerQueued: Int = 0) {
        self.doc = doc; self.learning = learning; self.selected = selected; self.steerQueued = steerQueued
    }
}

public struct NotebookLineRow: Codable, Sendable, Equatable { public var id = "", text = "", tag = "", canWhy = false, selected = false; public init() {} }
public struct NotebookSectionRow: Codable, Sendable, Equatable { public var key = "", heading = "", lines: [NotebookLineRow] = []; public init() {} }
public struct NotebookPair: Codable, Sendable, Equatable { public var label = "", value = ""; public init(label: String = "", value: String = "") { self.label = label; self.value = value } }
public struct NotebookEvidenceRow: Codable, Sendable, Equatable { public var title = "", outcome = "", detail = ""; public init() {} }
public struct NotebookHeldRow: Codable, Sendable, Equatable {
    public var what = "", whereRead = ""
    public init(_ what: String, _ whereRead: String) { self.what = what; self.whereRead = whereRead }
}

public struct NotebookPageInfo: Codable, Sendable, Equatable {
    /// loading | ready | none | offline
    public var docState = "loading"
    public var docAddress = "", docNotice = ""
    public var sections: [NotebookSectionRow] = []
    /// loading | ready | notyet | offline | signedout
    public var scoreState = "loading"
    public var scoreNotice = ""
    public var score: [NotebookPair] = []
    public var byKind: [NotebookPair] = []
    public var noVerdict = ""
    public var selectedText = "", evidenceNote = ""
    public var evidence: [NotebookEvidenceRow] = []
    public var held: [NotebookHeldRow] = []
    public var steerLine = ""
    public init() {}
}

public enum NotebookPage {
    public static let held: [NotebookHeldRow] = [
        NotebookHeldRow("autoradio.turn: what you said to the radio, and what it answered", "On your computer's tape. This page does not list turns yet: the server's wider ledger route is not built."),
        NotebookHeldRow("autoradio.request, autoradio.stretch_start, autoradio.segment, autoradio.stretch: what was written for you", "On your computer's tape. The title of each is in the rows behind a notebook line; the full text is not listed yet."),
        NotebookHeldRow("autoradio.forecast and autoradio.settlement: each guess the DJ made, and how it came out", "The scorecard and the rows behind a line, on this page."),
        NotebookHeldRow("autoradio.sheet_op and autoradio.sheet_snapshot: the air log (what played, skipped, cut or left, and where)", "The Heard list on the Autoradio page; the op that settled a forecast is on its row here."),
        NotebookHeldRow("autoradio.no_verdict: a forecast that settled nothing because you left or the app could not play it", "The scorecard's line about forecasts with no verdict, and the row."),
        NotebookHeldRow("autoradio.lesson, autoradio.strike and autoradio.notebook_patch: what was learned, struck or written into the notebook", "The notebook lines on this page; tap a line for the forecasts behind it."),
        NotebookHeldRow("autoradio.note: your more and less taps, as sent to your computer", "Recent steering in the notebook, once the server writes it; taps waiting to be sent are counted on this page."),
        NotebookHeldRow("autoradio.take: takes you recorded", "Takes, on the Autoradio page."),
        NotebookHeldRow("Model call traces (the assembled prompt for a stretch)", "Held on your computer's trace surfaces, not on this page."),
        NotebookHeldRow("More and less taps (a record on this phone)", "Settings, What I've told the paper. Only on this phone, apart from the copy sent as a steer."),
        NotebookHeldRow("Topics you follow (edition-topics.json on this phone; the notebook gets them when you finish editing your edition)", "Edit my edition, and \"What you asked for\" on this page."),
        NotebookHeldRow("The run sheet: what is on air, ahead and heard", "The Autoradio page. A copy is on your computer."),
        NotebookHeldRow("Muted voices", "Takes, on the Autoradio page. Only on this phone, apart from the steer sent for a mute."),
        NotebookHeldRow("Mail", "The Mail page; only on this phone. It is never sent to the DJ or the scorecard."),
    ]

    static func kindLabel(_ s: String) -> String {
        let w = s.replacingOccurrences(of: "_", with: " ")
        return w.prefix(1).uppercased() + w.dropFirst()
    }

    static func outcomeWords(_ r: LearningRow) -> String {
        switch r.status {
        case "open": return "Not yet heard"
        case "overdue": return "Not heard in time"
        case "settled":
            if r.heardThrough == true { return "Heard through" }
            return r.op?.outcome == "cut" ? "Cut off" : "Skipped"
        case "no_verdict":
            switch r.noVerdictReason { case "left": return "You left"; case "failed": return "The app could not play it"; case "late": return "Settled too late"; default: return "No verdict" }
        default: return "Not yet heard"
        }
    }

    public static func make(_ i: NotebookPageInput) -> NotebookPageInfo {
        var o = NotebookPageInfo()
        o.held = held
        var doc: NotebookDoc?
        switch i.doc {
        case .loading: o.docState = "loading"
        case .found(let a, let nb): o.docState = "ready"; o.docAddress = a; doc = nb
        case .none: o.docState = "none"; o.docNotice = "You have no notebook yet. It is started when you choose your topics, and Autoradio adds to it as it learns."
        case .unavailable: o.docState = "offline"; o.docNotice = "Your notebook could not be reached right now."
        }
        if let doc {
            o.sections = doc.sections.map { s in
                var r = NotebookSectionRow(); r.key = s.key; r.heading = s.heading
                r.lines = s.lines.map { l in
                    var x = NotebookLineRow(); x.id = l.id; x.text = l.text
                    if let h = l.heard, let m = l.of { x.tag = "heard \(h) of \(m)" }
                    x.canWhy = l.ref != nil; x.selected = l.id == i.selected
                    return x
                }
                return r
            }
        }
        var read: LearningRead?
        switch i.learning {
        case .none: o.scoreState = "loading"
        case .some(.ready(let r)): o.scoreState = "ready"; read = r
        case .some(.notYet): o.scoreState = "notyet"; o.scoreNotice = "The scorecard is not available yet. Your computer does not have it, so nothing is shown rather than a guess."
        case .some(.offline): o.scoreState = "offline"; o.scoreNotice = "The scorecard could not be reached. It will show when your computer answers."
        case .some(.signedOut): o.scoreState = "signedout"; o.scoreNotice = "Sign in to see the scorecard."
        }
        if let read {
            let c = read.card
            if c.settled == 0 { o.score = [NotebookPair(label: "Forecasts settled", value: "0"), NotebookPair(label: "Still open", value: "\(c.open)")] }
            else {
                let pct = c.heardThroughRate.map { " (\(Int(($0 * 100).rounded()))%)" } ?? ""
                o.score = [NotebookPair(label: "Forecasts settled", value: "\(c.settled)"), NotebookPair(label: "Heard through", value: "\(c.heardThrough) of \(c.settled)\(pct)"),
                           NotebookPair(label: "Still open", value: "\(c.open)")]
            }
            o.byKind = c.byKind.map { NotebookPair(label: kindLabel($0.label), value: $0.resolved == 0 ? "none settled yet" : "\($0.heard) of \($0.resolved) heard through") }
            if c.noVerdict > 0 { o.noVerdict = "\(c.noVerdict) forecast\(c.noVerdict == 1 ? "" : "s") with no verdict: you left, or the app could not play it." }
        }
        if let line = doc?.line(i.selected) {
            o.selectedText = line.text
            if let ref = line.ref, let read {
                o.evidence = read.rows.filter { $0.causedBy == ref }.map { r in
                    var e = NotebookEvidenceRow(); e.title = r.title.isEmpty ? "A segment" : r.title; e.outcome = outcomeWords(r)
                    if let p = r.pHeard { e.detail = "The desk forecast \(Int((p * 100).rounded())) in 100." }
                    return e
                }
                if o.evidence.isEmpty { o.evidenceNote = "No forecast has been made from this line yet." }
            } else if line.ref == nil { o.evidenceNote = "This line is not one the desk cites, so no forecast is listed for it." }
            else { o.evidenceNote = "The forecasts behind a line are not available yet." }
        }
        if i.steerQueued > 0 { o.steerLine = "\(i.steerQueued) more or less tap\(i.steerQueued == 1 ? "" : "s") waiting to reach your computer." }
        return o
    }
}

// MARK: The store behind the page

@MainActor public final class NotebookStore {
    public static let shared = NotebookStore()
    public var doc: NotebookDocLoad = .loading
    public var learning: LearningLoad?
    public var selected = ""
    public var onChange: (() -> Void)?
    /// Injected for tests; the defaults ask the computer.
    public var loadDoc: () async -> NotebookDocLoad = { await NotebookStore.defaultLoadDoc() }
    public var loadLearning: () async -> LearningLoad = { await LearningClient(client: ChoirEnvironment.shared.client()).read(limit: 200) }
    public var steerQueued: () -> Int = { SteerQueue.shared.taps.count }
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

    /// Sign-out: what the page holds belongs to the account.
    public func wipe() {
        writing?.cancel(); writing = nil; refreshRunning = false
        doc = .loading; learning = nil; selected = ""; lastRefresh = nil; refreshing = false
        onChange?()
    }

    public func info() -> NotebookPageInfo { NotebookPage.make(NotebookPageInput(doc: doc, learning: learning, selected: selected, steerQueued: steerQueued())) }

    private var refreshRunning = false
    private var writing: Task<Bool, Never>?
    public func refresh() async {
        guard !refreshRunning else { return }          // one at a time: two would each see "no notebook" and make one
        refreshRunning = true; defer { refreshRunning = false }
        // The topics written on this phone and not yet in the notebook (the marker says when they are): written now, once.
        let gen = AccountWipe.generation                       // bound WITH the topics: a sign-out after this line refuses the write
        let topics = EditionPicksMirror.shared.topics
        if EditionPicksMirror.shared.known, !topics.isEmpty { await writeTopics(topics, generation: gen) }
        doc = await loadDoc(); onChange?()
        learning = await loadLearning(); onChange?()
        await SteerQueue.shared.flush()
        onChange?()
    }

    private var lastRefresh: Date?
    private var refreshing = false
    /// Called when the page is read: the first read, and one a minute after, ask the computer. Never two at once.
    public func refreshIfStale(now: Date = Date()) {
        guard !refreshing, lastRefresh.map({ now.timeIntervalSince($0) > 60 }) ?? true else { return }
        refreshing = true; lastRefresh = now
        Task { @MainActor in await self.refresh(); self.refreshing = false }
    }

    public func select(_ id: String) { selected = selected == id ? "" : id; onChange?() }

    static func defaultLoadDoc() async -> NotebookDocLoad {
        guard ChoirEnvironment.shared.client().hasKey else { return .unavailable }
        guard let ed = await EditionLookup.fetch() else {
            if let t = NotebookCache.shared.text { return .found(address: "", notebook: NotebookText.parse(t)) }
            return .unavailable
        }
        guard ed.found else { return .none }
        let address = "texture:" + ed.docID
        let d = await ReaderStore.shared.doc(address)
        guard d.status == "ready" else { return NotebookCache.shared.text.map { .found(address: address, notebook: NotebookText.parse($0)) } ?? .unavailable }
        NotebookCache.shared.set(d.source)
        return .found(address: address, notebook: NotebookText.parse(d.source))
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
