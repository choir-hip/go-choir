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

public final class ReaderStore: @unchecked Sendable {
    public static let shared = ReaderStore()
    private let lock = NSLock()
    private var cache: [String: ReaderDoc] = [:]
    public init() {}

    public func cached(_ address: String) -> ReaderDoc? { lock.lock(); defer { lock.unlock() }; return cache[address] }
    public func put(_ d: ReaderDoc) { lock.lock(); cache[d.address] = d; lock.unlock() }
    public func invalidate(_ address: String) { lock.lock(); cache[address] = nil; lock.unlock() }

    /// The document at an address, from the cache unless `force`.
    public func doc(_ address: String, force: Bool = false) async -> ReaderDoc {
        if !force, let c = cached(address), c.status == "ready" { return c }
        do {
            let d = try await fetch(address)
            put(d)
            return d
        } catch is CancellationError {
            return cached(address) ?? .loading(address)
        } catch ChoirError.notSignedIn {
            return .failed(address, "Sign in to open your documents.")
        } catch {
            return cached(address) ?? .failed(address, "Could not open this page. \(Self.describe(error))")
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
            let r = try await client.send("GET", "/api/platform/publications/resolve", query: ["route": a.route],
                                          authenticated: false, route: "/api/platform/publications/resolve")
            var d = TextureDocParser.parseBundle(try r.object(), address: address)
            let bare = TextureAddress.splitRoute(a.route).bare
            d.related = await related(client: client, route: bare)
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
            return TextureDocParser.parseRevision(revObj, document: docObj, address: address, revisions: revisions)
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

// MARK: Textures place

public struct TextureRow: Codable, Sendable, Equatable {
    public var address: String
    public var title: String
    public var updated: String
    public var meta: String
    public var edition: Bool
    public var revising: Bool
    public init(address: String, title: String, updated: String, meta: String, edition: Bool, revising: Bool) {
        self.address = address; self.title = title; self.updated = updated; self.meta = meta; self.edition = edition; self.revising = revising
    }
}

public struct TextureList: Codable, Sendable, Equatable {
    public var status: String
    public var message: String
    public var rows: [TextureRow]
    public var hasEdition: Bool
    public init(status: String = "loading", message: String = "", rows: [TextureRow] = [], hasEdition: Bool = false) {
        self.status = status; self.message = message; self.rows = rows; self.hasEdition = hasEdition
    }

    /// The list from GET /api/texture/documents (newest first). The edition is pinned first. A document is the listener's
    /// edition when its metadata names that role or its title is "My edition" (the server has no marker yet).
    public static func parse(_ o: [String: Any]) -> TextureList {
        let docs = (o["documents"] as? [[String: Any]]) ?? []
        var rows: [TextureRow] = []
        for d in docs {
            guard let id = d["doc_id"] as? String, !id.isEmpty else { continue }
            if d["archived_at"] != nil, !(d["archived_at"] is NSNull) { continue }
            let title = ((d["title"] as? String) ?? "").rdTrim
            let meta = d["metadata"] as? [String: Any]
            let isEdition = (meta?["role"] as? String) == "edition" || title.lowercased().hasPrefix("my edition")
            let n = TextureDocParser.intValue(d["revision_count"]) ?? 0
            let when = TextureDocParser.shortDate((d["updated_at"] as? String) ?? "")
            let pending = (d["agent_revision_pending"] as? Bool) ?? false
            let metaLine = [pending ? "Being revised" : "", n > 0 ? "\(n) version\(n == 1 ? "" : "s")" : "", when].filter { !$0.isEmpty }.joined(separator: " · ")
            rows.append(TextureRow(address: "texture:\(id)", title: title.isEmpty ? "Untitled" : title, updated: when, meta: metaLine,
                                   edition: isEdition, revising: pending))
        }
        let ed = rows.filter { $0.edition }, rest = rows.filter { !$0.edition }
        return TextureList(status: "ready", rows: ed + rest, hasEdition: !ed.isEmpty)
    }

    public static func load() async -> TextureList {
        do {
            let r = try await ChoirEnvironment.shared.client().send("GET", "/api/texture/documents", query: ["limit": "100"], route: "/api/texture/documents")
            return parse(try r.object())
        } catch ChoirError.notSignedIn {
            return TextureList(status: "error", message: "Sign in to see your documents.")
        } catch {
            return TextureList(status: "error", message: "Could not load your documents. \(ReaderStore.describe(error))")
        }
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
        default: return nil
        }
    }
}
