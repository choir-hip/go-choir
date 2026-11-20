import Foundation

// What is left of the Texture editor in Swift: ONE save of a whole document's Markdown as a revision, for the notebook's topics sync (Notebook.swift `syncTopics`), which is
// native until the notebook's topic handling moves to shared code. The editor itself (the session, the pause in typing, the offline copy, the page's words, the Markdown to
// body_doc merge's twin) is AutoradioLab/texture-edit.ts and texture-body.ts: shared code, both platforms. TextureBody.swift is this save's merge and goes with it.

public struct TextureHead {
    public var content: String
    public var body: [String: Any]?
    public var revisionId: String
    public var version: Int
    public init(content: String, body: [String: Any]?, revisionId: String, version: Int = 0) { self.content = content; self.body = body; self.revisionId = revisionId; self.version = version }
}

public protocol TextureAPI: AnyObject {
    func head(docId: String) async throws -> TextureHead
    /// Post a revision; the new head's id and version.
    func post(docId: String, content: String, body: [String: Any], parent: String) async throws -> (revisionId: String, version: Int)
}

public enum TextureSave {
    /// Post `markdown` as the next revision of the document at `address` (`texture:<doc>`, a head). True when a revision was posted: false when the text is the head's already, the address is no
    /// document's head, or the route did not answer (the notebook tries again at its next change).
    @MainActor public static func post(address: String, markdown: String, api: TextureAPI = RouteTextureAPI()) async -> Bool {
        let a = TextureAddress.parse(address)
        guard a.kind == .document, a.rev.isEmpty, let head = try? await api.head(docId: a.doc), head.content != markdown else { return false }
        let merged = TextureBodyMerge.merge(original: head.body, originalMarkdown: head.content, newMarkdown: markdown)
        guard let r = try? await api.post(docId: a.doc, content: markdown, body: merged.body, parent: head.revisionId) else { return false }
        ReaderStore.shared.invalidate(address)
        TextureSave.kept(doc: a.doc, text: markdown, version: r.version)
        return true
    }
    /// What the owner wrote is kept and searchable at once (also called by the module when shared code saved a document).
    public static func kept(doc: String, text: String, title: String? = nil, version: Int) {
        var heading = title ?? ""
        if heading.isEmpty { heading = text.split(whereSeparator: \.isNewline).map { $0.trimmingCharacters(in: CharacterSet(charactersIn: "# ").union(.whitespaces)) }.first { !$0.isEmpty } ?? "Document" }
        ContentStore.shared.put(StoredItem(id: "texture:" + doc, kind: .document, title: String(heading.prefix(120)), text: text, date: Date(), address: "texture:" + doc,
                                           source: "Texture", version: "saved-\(version)", pinned: true, syncedAt: Date()))
    }
}

/// The routes the web uses, on the listener's computer.
public final class RouteTextureAPI: TextureAPI {
    public init() {}
    func enc(_ s: String) -> String { s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s }

    public func head(docId: String) async throws -> TextureHead {
        let client = ChoirEnvironment.shared.client()
        var doc = try await client.send("GET", "/api/texture/documents/\(enc(docId))", route: "/api/texture/documents/{id}").object()
        if let inner = doc["document"] as? [String: Any] { doc = inner }
        let rid = doc["current_revision_id"] as? String ?? ""
        guard !rid.isEmpty else { return TextureHead(content: "", body: nil, revisionId: "") }
        var rev = try await client.send("GET", "/api/texture/revisions/\(enc(rid))", route: "/api/texture/revisions/{id}").object()
        if let inner = rev["revision"] as? [String: Any] { rev = inner }
        let body = rev["body_doc"] as? [String: Any]
        let n = (rev["version_number"] as? NSNumber)?.intValue ?? -1
        return TextureHead(content: rev["content"] as? String ?? "", body: body, revisionId: rid, version: n + 1)
    }

    public func post(docId: String, content: String, body: [String: Any], parent: String) async throws -> (revisionId: String, version: Int) {
        let client = ChoirEnvironment.shared.client()
        var payload: [String: Any] = ["content": content, "body_doc": body]
        if !parent.isEmpty { payload["parent_revision_id"] = parent }
        // As the web does: a document with a trajectory is saved against its lifecycle (the version it read) with an idempotency key.
        var doc = try await client.send("GET", "/api/texture/documents/\(enc(docId))", route: "/api/texture/documents/{id}").object()
        if let inner = doc["document"] as? [String: Any] { doc = inner }
        if let traj = doc["trajectory_id"] as? String, !traj.isEmpty {
            let life = try await client.send("GET", "/api/texture/lifecycle/\(enc(traj))", route: "/api/texture/lifecycle/{id}").object()
            payload["idempotency_key"] = UUID().uuidString
            if let v = (life["trajectory"] as? [String: Any])?["lifecycle_version"] { payload["expected_lifecycle_version"] = v }
            let head = ((life["current_document_head"] as? [String: Any])?["revision_id"] as? String)
                ?? ((life["document"] as? [String: Any])?["current_revision_id"] as? String)
                ?? ((life["head_revision"] as? [String: Any])?["revision_id"] as? String) ?? ""
            if !head.isEmpty { payload["parent_revision_id"] = head }
        }
        let r = try await client.postJSON("/api/texture/documents/\(enc(docId))/revisions", payload, route: "/api/texture/documents/{id}/revisions")
        var o = try r.object()
        if let inner = o["revision"] as? [String: Any] { o = inner }
        let n = (o["version_number"] as? NSNumber)?.intValue ?? -1
        return (o["revision_id"] as? String ?? "", n + 1)
    }
}

/// The unsent edit the build before shared-code editing kept: Application Support/Autoradio/texture-pending.json, `{"<doc>": "<markdown>"}`. Only native code can reach this place (it is outside every Exact
/// grant), so shared code (AutoradioLab/texture-edit.ts) asks for it through the app module's `textureOldPending` call, merges it, and only then asks `textureOldPendingRemove`. Sign-out removes it too (AccountWipe).
public enum LegacyTexturePending {
    public static var file: URL? {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.appendingPathComponent("Autoradio/texture-pending.json")
    }
    /// The file's text, or nil when there is none (or it cannot be read).
    public static func read(_ url: URL? = file) -> String? {
        guard let url, let d = try? Data(contentsOf: url) else { return nil }
        return String(data: d, encoding: .utf8) ?? ""      // undecodable: an empty text, which shared code finds damaged and removes
    }
    public static func remove(_ url: URL? = file) { if let url { try? FileManager.default.removeItem(at: url) } }
}
