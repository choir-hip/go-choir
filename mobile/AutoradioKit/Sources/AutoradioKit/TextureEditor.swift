import Foundation

// Editing your own Texture documents on the phone (S1). The editor is Exact's `textarea markup="markdown"` over the document's Markdown;
// this file is everything behind it: the session for one document, the debounced save, and the call that saves.
//
// The rules:
//  - Who can edit what: `texture:<doc>` (the head of your own document, including one opened from Files) is editable. A revision
//    (`texture:<doc>@<rev>`) and a published story (`pub:`) are read-only: they have no session.
//  - An edit becomes a revision through the web's own call, POST /api/texture/documents/{id}/revisions (frontend texture.js createRevision):
//    `content` (the Markdown) and `body_doc` (TextureBodyMerge), `parent_revision_id`, and for a document with a trajectory the lifecycle
//    version and an idempotency key. No second write path. A file-backed document is aliased to its Files path on the server (open file
//    document), so its revisions are its file's.
//  - When it is posted: 2 seconds after the last keystroke (the pause in typing), at once when the listener leaves the page, closes it,
//    taps Done or the app goes to the background (`flushAll`), and never two at once. A burst of typing is one revision.
//  - Offline: a failed post keeps the newest text on the phone (saved in Application Support) and posts it when the route answers (retried
//    every 30 seconds and at launch). The newest text wins; the parent is the head at that moment.
//
// Ways this can fail (each has a test in TextureEditorTests):
//  1. A keystroke per revision: the burst is one post.
//  2. An edit lost on a bad network, or posted twice after a relaunch.
//  3. A document nobody may edit (a revision, a story) opens a session.
//  4. A save that changes nothing is posted.
//  5. Typing while a save is in flight is dropped: it saves right after.

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

public struct TextureEditInfo: Codable, Sendable, Equatable {
    /// idle | dirty | saving | saved | offline | failed
    public var status = "idle"
    public var label = ""
    public var address = ""
    public var editable = false
    public init() {}
}

@MainActor public final class TextureEditor {
    public static let shared = TextureEditor(api: RouteTextureAPI(), storage: TextureEditor.applicationSupportFile())

    final class Session {
        var docId: String
        var base: TextureHead
        var draft: String?
        var info = TextureEditInfo()
        var task: Task<Void, Never>?
        var saving = false
        init(docId: String, base: TextureHead) { self.docId = docId; self.base = base }
    }

    let api: TextureAPI
    private let storage: URL?
    private var sessions: [String: Session] = [:]
    /// The pause in typing after which an edit is posted.
    public var idle: TimeInterval = 2
    public var retryAfter: TimeInterval = 30
    public var onChange: (() -> Void)?
    /// Newest text not yet posted, by document, kept across launches.
    public private(set) var pending: [String: String] = [:]
    private var retryTask: Task<Void, Never>?

    public init(api: TextureAPI, storage: URL? = nil) {
        self.api = api; self.storage = storage
        if let storage, let d = try? Data(contentsOf: storage), let p = try? JSONDecoder().decode([String: String].self, from: d) { pending = p }
    }

    public static func applicationSupportFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("texture-pending.json")
    }

    private var generation = 0
    /// Sign-out: the sessions, the retry timer and the pending text (memory and file). A save in flight finds the generation changed and keeps nothing.
    public func wipe() {
        generation += 1
        retryTask?.cancel(); retryTask = nil
        for s in sessions.values { s.task?.cancel() }
        sessions = [:]; pending = [:]
        if let storage { try? FileManager.default.removeItem(at: storage) }
        changed()
    }

    private func savePending() {
        guard let storage else { return }
        if pending.isEmpty { try? FileManager.default.removeItem(at: storage) }
        else if let d = try? JSONEncoder().encode(pending) { try? d.write(to: storage, options: .atomic) }
    }

    // MARK: Who can edit

    /// The document id of an address the listener may edit (the head of a Texture document), else nil.
    public static func editableDoc(_ address: String) -> String? {
        let a = TextureAddress.parse(address)
        return a.kind == .document && a.rev.isEmpty ? a.doc : nil
    }

    public func info(address: String) -> TextureEditInfo {
        guard let doc = Self.editableDoc(address) else { return TextureEditInfo() }
        var i = sessions[doc]?.info ?? TextureEditInfo()
        i.address = address; i.editable = true
        if sessions[doc] == nil, pending[doc] != nil { i.status = "offline"; i.label = "Edits kept on this phone" }
        return i
    }

    // MARK: Editing

    private func session(_ doc: String) async -> Session? {
        if let s = sessions[doc] { return s }
        do {
            let head = try await api.head(docId: doc)
            if let s = sessions[doc] { return s }
            let s = Session(docId: doc, base: head)
            sessions[doc] = s
            return s
        } catch { return nil }
    }

    /// The listener typed: the text is now `markdown`. It is posted after the pause in typing.
    public func edit(address: String, markdown: String) async {
        guard let doc = Self.editableDoc(address), let s = await session(doc) else { return }
        s.draft = markdown
        if markdown == s.base.content { s.info.status = "idle"; s.info.label = ""; s.task?.cancel(); changed(); return }
        s.info.status = "dirty"; s.info.label = "Editing"
        s.task?.cancel()
        let wait = idle
        s.task = Task { @MainActor [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(max(0, wait) * 1_000_000_000))
            guard !Task.isCancelled else { return }
            await self?.save(doc)
        }
        changed()
    }

    /// Post now what is waiting (leaving the page, Done, the background).
    public func flush(address: String) async { if let doc = Self.editableDoc(address) { await save(doc) } }
    public func flushAll() async { for doc in Array(sessions.keys) { await save(doc) } }

    func save(_ doc: String) async {
        guard let s = sessions[doc], !s.saving, let draft = s.draft, draft != s.base.content else { return }
        s.task?.cancel()
        s.saving = true; s.info.status = "saving"; s.info.label = "Saving"; changed()
        let merged = TextureBodyMerge.merge(original: s.base.body, originalMarkdown: s.base.content, newMarkdown: draft)
        let gen = generation
        do {
            let r = try await api.post(docId: doc, content: draft, body: merged.body, parent: s.base.revisionId)
            guard gen == generation else { s.saving = false; return }          // signed out meanwhile: nothing of it is kept
            s.base = TextureHead(content: draft, body: merged.body, revisionId: r.revisionId, version: r.version)
            pending[doc] = nil; savePending()
            s.info.status = "saved"; s.info.label = r.version > 0 ? "Saved · version \(r.version)" : "Saved"
        } catch {
            guard gen == generation else { s.saving = false; return }
            pending[doc] = draft; savePending()
            if Self.isOffline(error) { s.info.status = "offline"; s.info.label = "Offline: kept on this phone, posts when it can"; scheduleRetry() }
            else { s.info.status = "failed"; s.info.label = "Not saved: \(textureSaveSentence(error))"; scheduleRetry() }
        }
        s.saving = false
        changed()
        if let d = s.draft, d != s.base.content, s.info.status == "saved" { await save(doc) }          // typed while it was posting
    }

    static func isOffline(_ e: Error) -> Bool {
        if let c = e as? ChoirError { switch c { case .transport, .timeout: return true; case .server(let st, _): return st >= 500 || st == 404 || st == 429; default: return false } }
        return e is URLError
    }

    // MARK: Offline

    private func scheduleRetry() {
        retryTask?.cancel()
        let wait = retryAfter
        retryTask = Task { @MainActor [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(max(0, wait) * 1_000_000_000))
            guard !Task.isCancelled else { return }
            await self?.retryPending()
        }
    }

    /// Post what an earlier launch or a bad network kept. The newest text wins over the head as it is now.
    public func retryPending() async {
        for (doc, text) in pending {
            if let s = sessions[doc] { s.draft = text; await save(doc); continue }
            guard let head = try? await api.head(docId: doc) else { scheduleRetry(); continue }
            let s = Session(docId: doc, base: head); sessions[doc] = s
            s.draft = text
            await save(doc)
        }
    }

    private func changed() { onChange?() }
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

/// One plain sentence for a failed save (this was the Files service's; the Files page is shared code now).
func textureSaveSentence(_ error: Error) -> String {
    if let e = error as? ChoirError {
        switch e {
        case .unauthorized, .notSignedIn: return "Sign in again to save."
        case .transport, .timeout: return "Could not reach your computer. Check the connection and try again."
        case .forbidden: return "This device is not allowed to save here."
        default: return "Could not save. \(e)"
        }
    }
    return "Something went wrong saving the document."
}
