import Foundation
// The Swift callers that still need a document's text read it here: the notebook (the edition's Markdown) and the content sync (a document's text). The reader PAGE does not: it is shared code
// (AutoradioLab/reader.ts, reader-parse.ts), which also hands the page in front to the voice (HeldPages). Nothing here parses a document: the revision's own Markdown (`content`) is the text.
// GET /api/texture/documents/{id}, /api/texture/revisions/{rev}. Not signed in or a refused key: a failed result that says so, never another account's data.

public struct TextureText: Sendable {
    public var status = "loading", message = "", source = ""
    /// The document at a `texture:` address, read now. Nothing is held: a caller that needs it again asks again.
    public static func fetch(_ address: String) async -> TextureText {
        do { return try await read(address) }
        catch is CancellationError { return TextureText() }
        catch ChoirError.notSignedIn { return TextureText(status: "error", message: "Sign in to open your documents.") }
        catch { return TextureText(status: "error", message: "Could not open this page. \((error as? ChoirError).map { "\($0)" } ?? error.localizedDescription)") }
    }
    static func read(_ address: String) async throws -> TextureText {
        let a = TextureAddress.parse(address)
        guard a.kind == .document else { return TextureText(status: "error", message: "This address does not name a document.") }
        let client = ChoirEnvironment.shared.client()
        func enc(_ s: String) -> String { s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s }
        var doc = try await client.send("GET", "/api/texture/documents/\(enc(a.doc))", route: "/api/texture/documents/{id}").object()
        if let inner = doc["document"] as? [String: Any] { doc = inner }
        let rid = a.rev.isEmpty ? ((doc["current_revision_id"] as? String) ?? "") : a.rev
        var rev: [String: Any] = [:]
        if !rid.isEmpty {
            rev = try await client.send("GET", "/api/texture/revisions/\(enc(rid))", route: "/api/texture/revisions/{id}").object()
            if let inner = rev["revision"] as? [String: Any] { rev = inner }
        }
        return TextureText(status: "ready", source: (rev["content"] as? String) ?? "")
    }
}

/// `pub:<route>[@..]` and `texture:<doc>[@<rev>]`: the parts native callers need (the editor's document id, the learning loop's story route, the sync's route).
public struct TextureAddress: Equatable, Sendable {
    public enum Kind: Sendable { case publication, document, none }
    public var kind: Kind, route = "", doc = "", rev = ""
    public static func parse(_ address: String) -> TextureAddress {
        let rest = PageAddress.rest(of: address)
        if address.hasPrefix("pub:") { return rest.isEmpty ? TextureAddress(kind: .none) : TextureAddress(kind: .publication, route: rest) }
        guard address.hasPrefix("texture:") else { return TextureAddress(kind: .none) }
        let parts = rest.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).map(String.init)
        return (parts.first ?? "").isEmpty ? TextureAddress(kind: .none) : TextureAddress(kind: .document, doc: parts[0], rev: parts.count > 1 ? parts[1] : "")
    }
    /// "slug/v3" -> ("slug", 3); a bare route -> (route, 0)
    public static func splitRoute(_ route: String) -> (bare: String, version: Int) {
        guard let r = route.range(of: "/v", options: .backwards), let n = Int(route[r.upperBound...]), n > 0 else { return (route, 0) }
        return (String(route[..<r.lowerBound]), n)
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
        guard let doc, !spoken.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        let key = HeldPages.normalised(String(spoken.trimmingCharacters(in: .whitespacesAndNewlines).prefix(60)))
        if key.count < 6 { return nil }
        for b in doc.blocks where !b.text.isEmpty {
            let nb = HeldPages.normalised(b.text)
            guard nb.contains(key) else { continue }
            var best = 0
            for (i, s) in b.sentences.enumerated() where HeldPages.normalised(s).contains(String(key.prefix(24))) { best = i; break }
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
    public var onChange: (@Sendable () -> Void)?
    public var info: ReaderAimInfo { lock.lock(); defer { lock.unlock() }; return _info }
    public func set(_ i: ReaderAimInfo) { lock.lock(); _info = i; lock.unlock(); onChange?() }
    public func clear() { set(ReaderAimInfo()) }
}

/// The sentence the radio is saying, by address, so the reader lights it (the app module's `radioView` feeds it). Only a CHANGE is passed on; when nothing plays the follow is cleared. A follow that was
/// cleared (the sign-out wipe) is set again by the very next feed even when the address and the sentence are the same as before.
public final class ReaderFeed: @unchecked Sendable {
    public static let shared = ReaderFeed()
    private let lock = NSLock()
    private var last = ""
    public init() {}
    public func feed(_ r: RadioInfo, address: String, follow: ReaderFollow = .shared) {
        let spoken = r.playing && !address.isEmpty && r.sentences.indices.contains(r.sentenceIndex) ? r.sentences[r.sentenceIndex] : ""
        let key = spoken.isEmpty ? "" : "\(address)|\(spoken)"
        lock.lock(); let changed = key != last || (!key.isEmpty && follow.info.address.isEmpty); last = key; lock.unlock()
        guard changed else { return }
        if key.isEmpty { follow.clear() } else { follow.set(address: address, spoken: spoken) }
    }
}
