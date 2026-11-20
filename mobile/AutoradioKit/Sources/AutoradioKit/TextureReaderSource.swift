import Foundation

// Loading for the reader and the Textures place (lane 3): the routes the web uses, behind the kit's PageSource.
//   pub:<route>            GET /api/platform/publications/resolve?route=            (public) and .../related?route=&limit=5
//   texture:<doc>[@<rev>]  GET /api/texture/documents/{id}, /revisions, /api/texture/revisions/{rev}   (signed in)
//   textures:              GET /api/texture/documents
// In demo mode the session answers from fixtures (Demo.swift).
//
// WAYS THIS CAN FAIL (written before the code):
//  1. Not signed in, or the key is refused: the page says so in one sentence; it never shows another account's data.
//  2. A route that is not there yet (related stories, a revision list): the document still opens; the missing piece is empty.
//  3. A response of an unexpected shape (wrapped in {document:...} or bare): both are read; anything else is an error page.
//  4. The same address requested twice at once: both get a result; the cache keeps the latest.
//  5. The listener stops a revision that already finished: the cancel error is swallowed and the document is re-read.
//  6. Follow-along text that matches no block: nothing is highlighted (never the wrong block).
//  7. Prefetch of a neighbour fails: nothing is cached for it and the card loads on demand when it is reached.
//  8. The listener swipes before a prefetch is done: the page asks for the same address and waits for the fetch in flight (one request).
//  9. A long session: the cache holds at most `limit` documents, the least recently used goes first, the one just stored never does.
// 10. The paper republishes while a story is held: the held copy is shown for the session (reload in the page menu re-reads it).

public final class ReaderStore: @unchecked Sendable {
    public static let shared = ReaderStore()
    /// How many documents the session keeps (the least recently used goes first). A story is a few tens of kilobytes decoded.
    public static let defaultLimit = 40
    private let lock = NSLock()
    private var cache: [String: ReaderDoc] = [:]
    /// Addresses in cache, least recently used first.
    private var recency: [String] = []
    /// A fetch in progress for an address, so the page, the page model and a prefetch share one request instead of three.
    private var inflight: [String: Task<ReaderDoc, Never>] = [:]
    private let limit: Int
    private let fetcher: (@Sendable (String) async throws -> ReaderDoc)?
    /// `fetcher` replaces the network (tests).
    public init(limit: Int = ReaderStore.defaultLimit, fetcher: (@Sendable (String) async throws -> ReaderDoc)? = nil) {
        self.limit = max(1, limit); self.fetcher = fetcher
    }

    /// Sign-out: every cached document (an offline page must not show the previous account's), and the fetches in flight.
    public func wipe() {
        lock.lock(); cache = [:]; recency = []; let t = Array(inflight.values); inflight = [:]; lock.unlock()
        for x in t { x.cancel() }
    }
    public func cached(_ address: String) -> ReaderDoc? {
        lock.lock(); defer { lock.unlock() }
        guard let d = cache[address] else { return nil }
        use(address)
        return d
    }
    public func put(_ d: ReaderDoc) {
        lock.lock(); defer { lock.unlock() }
        cache[d.address] = d
        use(d.address)
        while cache.count > limit, let oldest = recency.first, oldest != d.address {
            recency.removeFirst(); cache[oldest] = nil
        }
    }
    public func invalidate(_ address: String) {
        lock.lock(); defer { lock.unlock() }
        cache[address] = nil; recency.removeAll { $0 == address }
    }
    public var cachedCount: Int { lock.lock(); defer { lock.unlock() }; return cache.count }
    /// Every document the session holds that has its text (the saved stories the phone can answer from).
    public func readyDocuments() -> [ReaderDoc] { lock.lock(); defer { lock.unlock() }; return cache.values.filter { $0.status == "ready" } }
    private func use(_ address: String) { recency.removeAll { $0 == address }; recency.append(address) }

    /// The document at an address, from the cache unless `force`. A request for an address already being fetched waits for that fetch.
    public func doc(_ address: String, force: Bool = false) async -> ReaderDoc {
        if !force, let c = cached(address), c.status == "ready" { return c }
        if force { return await load(address) }
        return await sharedFetch(address).value
    }

    /// The fetch in flight for an address, or a new one.
    private func sharedFetch(_ address: String) -> Task<ReaderDoc, Never> {
        lock.lock(); defer { lock.unlock() }
        if let t = inflight[address] { return t }
        let t = Task<ReaderDoc, Never> { [self] in
            let d = await load(address)
            finish(address)
            return d
        }
        inflight[address] = t
        return t
    }
    private func finish(_ address: String) { lock.lock(); inflight[address] = nil; lock.unlock() }

    /// Fetch and decode stories ahead of the listener (the neighbours of the card on top). A story already held, a document that is
    /// not a publication, and an empty address are skipped. A fetch that fails is not kept: the card loads on demand when it is reached.
    @discardableResult
    public func prefetch(_ addresses: [String]) -> [Task<Void, Never>] {
        var seen = Set<String>()
        return addresses.compactMap { a in
            guard !a.isEmpty, seen.insert(a).inserted, TextureAddress.parse(a).kind == .publication else { return nil }
            if let c = cached(a), c.status == "ready" { return nil }
            return Task { [self] in _ = await doc(a) }
        }
    }

    /// A page held on this phone (the ContentStore), shown when the network cannot be reached. Set by the app.
    public var offline: (@Sendable (String) -> ReaderDoc?)?

    func load(_ address: String) async -> ReaderDoc {
        do {
            let d: ReaderDoc
            if let fetcher { d = try await fetcher(address) } else { d = try await fetch(address) }
            put(d)
            return d
        } catch is CancellationError {
            return cached(address) ?? .loading(address)
        } catch ChoirError.notSignedIn {
            return .failed(address, "Sign in to open your documents.")
        } catch {
            return cached(address) ?? offline?(address) ?? .failed(address, "Could not open this page. \(Self.describe(error))")
        }
    }

    static func describe(_ e: Error) -> String {
        if let c = e as? ChoirError { return "\(c)" }
        return e.localizedDescription
    }

    func fetch(_ address: String) async throws -> ReaderDoc {
        let a = TextureAddress.parse(address)
        let client = ChoirEnvironment.shared.client()
        switch a.kind {
        case .none:
            return .failed(address, "This address does not name a story or a document.")
        case .publication:
            // The related list does not depend on the story, so both are asked at once (they were one after the other).
            let bare = TextureAddress.splitRoute(a.route).bare
            async let rel = related(client: client, route: bare)
            let r = try await client.send("GET", "/api/platform/publications/resolve", query: ["route": a.route],
                                          authenticated: false, route: "/api/platform/publications/resolve")
            var d = TextureDocParser.parseBundle(try r.object(), address: address)
            d.related = await rel
            return d
        case .document:
            let docResp = try await client.send("GET", "/api/texture/documents/\(enc(a.doc))", route: "/api/texture/documents/{id}")
            var docObj = try docResp.object()
            if let inner = docObj["document"] as? [String: Any] { docObj = inner }
            let rid = a.rev.isEmpty ? ((docObj["current_revision_id"] as? String) ?? "") : a.rev
            var revObj: [String: Any] = [:]
            if !rid.isEmpty {
                let rr = try await client.send("GET", "/api/texture/revisions/\(enc(rid))", route: "/api/texture/revisions/{id}")
                revObj = try rr.object()
                if let inner = revObj["revision"] as? [String: Any] { revObj = inner }
            }
            var revisions: [[String: Any]] = []
            if let lr = try? await client.send("GET", "/api/texture/documents/\(enc(a.doc))/revisions", route: "/api/texture/documents/{id}/revisions"),
               let o = try? lr.object() {
                revisions = (o["revisions"] as? [[String: Any]]) ?? []
            }
            var d = TextureDocParser.parseRevision(revObj, document: docObj, address: address, revisions: revisions)
            d.docId = a.doc
            d.source = (revObj["content"] as? String) ?? ""
            d.editable = a.rev.isEmpty && d.status == "ready"
            return d
        }
    }

    func related(client: ChoirClient, route: String) async -> [ReaderRelated] {
        guard let r = try? await client.send("GET", "/api/platform/publications/related", query: ["route": route, "limit": "5"],
                                             authenticated: false, route: "/api/platform/publications/related"),
              let o = try? r.object() else { return [] }
        return Self.parseRelated(o)
    }

    /// The related list. The real route answers `links` (checked against choir.news, 2026-11-11); the older guesses stay readable.
    static func parseRelated(_ o: [String: Any]) -> [ReaderRelated] {
        let arr = (o["links"] as? [[String: Any]]) ?? (o["related"] as? [[String: Any]]) ?? (o["stories"] as? [[String: Any]]) ?? (o["items"] as? [[String: Any]]) ?? []
        return arr.compactMap { s in
            guard let title = s["title"] as? String, !title.isEmpty else { return nil }
            let rt: String
            if let str = s["route"] as? String { rt = str } else if let ro = s["route"] as? [String: Any] { rt = (ro["bare"] as? String) ?? (ro["path"] as? String) ?? "" } else { rt = "" }
            let clean = rt.hasPrefix("/") ? String(rt.dropFirst()) : rt
            if clean.isEmpty { return nil }
            return ReaderRelated(address: "pub:\(clean)", title: title, dek: (s["dek"] as? String) ?? "")
        }
    }

    func enc(_ s: String) -> String { s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s }

    // MARK: Actions on a document

    /// Restore a version as the latest (the web's "Restore"): POST /api/texture/documents/{id}/restore.
    public func restore(document address: String, revisionAddress: String) async -> Bool {
        let a = TextureAddress.parse(address), r = TextureAddress.parse(revisionAddress)
        guard a.kind == .document else { return false }
        let client = ChoirEnvironment.shared.client()
        let rid = r.rev
        if rid.isEmpty { return true }   // the head is already the latest
        let body = try? JSONSerialization.data(withJSONObject: ["revision_id": rid, "mode": "restore_as_latest"])
        let ok = (try? await client.send("POST", "/api/texture/documents/\(enc(a.doc))/restore", body: body,
                                          route: "/api/texture/documents/{id}/restore")) != nil
        invalidate(address)
        return ok
    }

    /// Stop the desk's revision in flight (the web's "Stop"): POST /api/texture/lifecycle/{traj}/cancel.
    public func cancelRevision(document address: String) async {
        guard let d = cached(address), !d.trajectory.isEmpty else { invalidate(address); return }
        let client = ChoirEnvironment.shared.client()
        _ = try? await client.send("POST", "/api/texture/lifecycle/\(enc(d.trajectory))/cancel", route: "/api/texture/lifecycle/{id}/cancel", okStatuses: [404, 409])
        invalidate(address)
    }
}

/// `pub:` and `texture:` as pages: the lines are the plain text (the spine's text renderer and other lanes read them).
public struct ReaderPageSource: PageSource {
    public let store: ReaderStore
    public init(store: ReaderStore = .shared) { self.store = store }
    public func handles(_ address: String) -> Bool { address.hasPrefix("pub:") || address.hasPrefix("texture:") }
    public func load(_ address: String) async throws -> PageModel {
        let d = await store.doc(address)
        return PageModel(address: address, kind: "reader", title: d.title.isEmpty ? PageAddress.defaultTitle(address) : d.title,
                         status: d.status, lines: d.lines, message: d.message)
    }
}

// The Textures place's list (TextureRow, TextureList: parse, the pinned edition, the words) is shared code now: AutoradioLab/textures.ts. What stays here is the edition lookup below, which the
// notebook and the onboarding still ask Swift for.

// MARK: The listener's edition (GET /api/texture/edition)

/// `{found:false}` before an edition exists (the normal first-run answer, not an error); `{found:true, doc_id, revision_id, updated_at}` after.
public struct EditionInfo: Sendable, Equatable {
    public var found = false
    public var docID = "", revisionID = "", updatedAt = ""
    public init(found: Bool = false, docID: String = "", revisionID: String = "", updatedAt: String = "") {
        self.found = found; self.docID = docID; self.revisionID = revisionID; self.updatedAt = updatedAt
    }
    /// A `found` answer with no document id cannot be opened, so it counts as not found.
    public static func parse(_ data: Data) throws -> EditionInfo {
        guard let o = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw ChoirError.decode("edition: not an object") }
        let id = (o["doc_id"] as? String) ?? ""
        guard (o["found"] as? Bool) == true, !id.isEmpty else { return EditionInfo() }
        return EditionInfo(found: true, docID: id, revisionID: (o["revision_id"] as? String) ?? "", updatedAt: (o["updated_at"] as? String) ?? "")
    }
}

public enum EditionLookup {
    /// nil when the server cannot say (offline, not signed in, a server without the route): callers then behave as before.
    public static func fetch() async -> EditionInfo? {
        guard let r = try? await ChoirEnvironment.shared.client().send("GET", "/api/texture/edition", route: "/api/texture/edition") else { return nil }
        return try? EditionInfo.parse(r.data)
    }
}

// MARK: Read along (set by the radio) and the prompt's aim (set by the reader)

public struct ReaderFollowInfo: Codable, Sendable, Equatable {
    public var address: String
    public var blockId: String
    public var sentence: Int
    public init(address: String = "", blockId: String = "", sentence: Int = -1) { self.address = address; self.blockId = blockId; self.sentence = sentence }
}

/// Which block and sentence the radio is reading. Lane 1 calls `set(address:spoken:)` with the text it is saying; the
/// match is by text, so the radio's own chunking does not matter.
public final class ReaderFollow: @unchecked Sendable {
    public static let shared = ReaderFollow()
    private let lock = NSLock()
    private var _info = ReaderFollowInfo()
    public var onChange: (@Sendable () -> Void)?
    public init() {}
    public var info: ReaderFollowInfo { lock.lock(); defer { lock.unlock() }; return _info }

    public func clear() { lock.lock(); _info = ReaderFollowInfo(); lock.unlock(); onChange?() }

    @discardableResult
    public func set(address: String, spoken: String, store: ReaderStore = .shared) -> ReaderFollowInfo {
        let found = Self.locate(spoken: spoken, in: store.cached(address))
        lock.lock(); _info = found.map { ReaderFollowInfo(address: address, blockId: $0.block, sentence: $0.sentence) } ?? ReaderFollowInfo(); let v = _info; lock.unlock()
        onChange?()
        return v
    }

    /// The block whose text contains the spoken text, and the sentence that contains its start. Nil when nothing matches.
    public static func locate(spoken: String, in doc: ReaderDoc?) -> (block: String, sentence: Int)? {
        guard let doc, !spoken.rdTrim.isEmpty else { return nil }
        let key = Citations.normalised(String(spoken.rdTrim.prefix(60))).text
        if key.count < 6 { return nil }
        for b in doc.blocks where !b.text.isEmpty {
            let nb = Citations.normalised(b.text).text
            guard nb.contains(key) else { continue }
            var best = 0
            for (i, s) in b.sentenceTexts().enumerated() where Citations.normalised(s).text.contains(String(key.prefix(24))) { best = i; break }
            return (b.id, best)
        }
        return nil
    }
}

public struct ReaderAimInfo: Codable, Sendable, Equatable {
    public var address: String
    public var blockId: String
    public var label: String
    public init(address: String = "", blockId: String = "", label: String = "") { self.address = address; self.blockId = blockId; self.label = label }
}

/// What the bar's prompt is aimed at: a document, or one block of it. Cleared when the listener leaves the document.
public final class ReaderAim: @unchecked Sendable {
    public static let shared = ReaderAim()
    private let lock = NSLock()
    private var _info = ReaderAimInfo()
    public init() {}
    public var info: ReaderAimInfo { lock.lock(); defer { lock.unlock() }; return _info }
    public func set(_ i: ReaderAimInfo) { lock.lock(); _info = i; lock.unlock() }
    public func clear() { set(ReaderAimInfo()) }
}

// MARK: Demo routes (the reader's fixtures; Demo.swift calls these from its one `case`)

enum TextureDemo {
    /// The revisions the phone posted in demo mode, in order: (document, Markdown content, body_doc).
    nonisolated(unsafe) static var posted: [(doc: String, content: String, body: [String: Any]?)] = []
    private static let postLock = NSLock()

    /// POST /api/texture/documents/{id}/revisions answers the revision it made; any other POST is not ours.
    static func post(path: String, body: Data) -> Data? {
        let parts = path.split(separator: "/").map(String.init)
        guard parts.count == 5, parts[0] == "api", parts[1] == "texture", parts[2] == "documents", parts[4] == "revisions" else { return nil }
        let o = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] ?? [:]
        postLock.lock(); posted.append((parts[3], o["content"] as? String ?? "", o["body_doc"] as? [String: Any])); let n = posted.count; postLock.unlock()
        return Data(#"{"revision_id":"demo-posted-\#(n)","version_number":\#(n + 3)}"#.utf8)
    }

    /// "demo-ev-charging/v2" -> "demo-ev-charging": every version of a demo story is the one fixture.
    static func bareRoute(_ route: String) -> String { TextureAddress.splitRoute(route).bare }

    static func route(_ path: String) -> Data? {
        let parts = path.split(separator: "/").map(String.init)      // api texture documents <id> [revisions|restore]
        guard parts.count >= 3, parts[0] == "api", parts[1] == "texture" else { return nil }
        func fx(_ n: String) -> Data? { Demo.fixture(n) }
        switch (parts[2], parts.count) {
        case ("documents", 3): return fx("texture-documents")
        case ("documents", 4): return fx("texture-doc-" + parts[3])
        case ("documents", 5):
            if parts[4] == "revisions" { return fx("texture-revs-" + parts[3]) }
            return Data("{}".utf8)                                   // restore
        case ("revisions", 4): return fx("texture-rev-" + parts[3])
        case ("lifecycle", _): return Data("{}".utf8)                // cancel
        case ("files", 4): return Data(#"{"doc_id":"demo-notes"}"#.utf8)   // POST /api/texture/files/open: demo opens the notes document
        default: return nil
        }
    }
}
