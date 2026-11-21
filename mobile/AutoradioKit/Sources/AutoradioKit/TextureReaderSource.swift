import Foundation
// The Swift callers that still need a document's text read it here: the notebook (the edition's Markdown) and the content sync (a document's text). The reader PAGE does not: it is shared code
// (AutoradioLab/reader.ts, reader-parse.ts), which also hands the page in front to the voice (HeldPages). GET /api/texture/documents/{id}, /revisions, /api/texture/revisions/{rev}.
//
// WAYS THIS CAN FAIL: not signed in or the key is refused: a failed document that says so, never another account's data; a revision list that is not there: the document still opens with no versions;
// an answer wrapped in {document:...} or bare: both are read; a cancelled read is "loading"; any other address is an error (only Texture documents are read here).

public final class ReaderStore: @unchecked Sendable {
    public static let shared = ReaderStore()
    public init() {}

    /// The document at a `texture:` address, read now. Nothing is held: a caller that needs it again asks again.
    public func doc(_ address: String) async -> ReaderDoc {
        do { return try await fetch(address) }
        catch is CancellationError { return .loading(address) }
        catch ChoirError.notSignedIn { return .failed(address, "Sign in to open your documents.") }
        catch { return .failed(address, "Could not open this page. \(Self.describe(error))") }
    }

    static func describe(_ e: Error) -> String {
        if let c = e as? ChoirError { return "\(c)" }
        return e.localizedDescription
    }

    func fetch(_ address: String) async throws -> ReaderDoc {
        let a = TextureAddress.parse(address)
        let client = ChoirEnvironment.shared.client()
        guard a.kind == .document else { return .failed(address, "This address does not name a document.") }
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

    func enc(_ s: String) -> String { s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s }
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

/// Which block and sentence the radio is reading. The bar calls `set(address:spoken:)` with the text it is saying; the match is by text, so the radio's own chunking does not matter, against the page
/// the shared reader handed over (HeldPages), so the block ids are the ones the page draws. (Moves to shared code with the bar's read-along.)
public final class ReaderFollow: @unchecked Sendable {
    public static let shared = ReaderFollow()
    private let lock = NSLock()
    private var _info = ReaderFollowInfo()
    public var onChange: (@Sendable () -> Void)?
    public init() {}
    public var info: ReaderFollowInfo { lock.lock(); defer { lock.unlock() }; return _info }

    public func clear() { lock.lock(); _info = ReaderFollowInfo(); lock.unlock(); onChange?() }

    @discardableResult
    public func set(address: String, spoken: String, held: HeldPages = .shared) -> ReaderFollowInfo {
        let found = Self.locate(spoken: spoken, in: held.page(address))
        lock.lock(); _info = found.map { ReaderFollowInfo(address: address, blockId: $0.block, sentence: $0.sentence) } ?? ReaderFollowInfo(); let v = _info; lock.unlock()
        onChange?()
        return v
    }

    /// The block whose text contains the spoken text, and the sentence that contains its start. Nil when nothing matches.
    public static func locate(spoken: String, in doc: HeldPage?) -> (block: String, sentence: Int)? {
        guard let doc, !spoken.rdTrim.isEmpty else { return nil }
        let key = Citations.normalised(String(spoken.rdTrim.prefix(60))).text
        if key.count < 6 { return nil }
        for b in doc.blocks where !b.text.isEmpty {
            let nb = Citations.normalised(b.text).text
            guard nb.contains(key) else { continue }
            var best = 0
            for (i, s) in b.sentences.enumerated() where Citations.normalised(s).text.contains(String(key.prefix(24))) { best = i; break }
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
