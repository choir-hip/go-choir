import Foundation

/// System 2 on the listener's computer. The listener's raw turn (not System 1's answer) goes to the
/// Texture desk, then the document's stream is followed and each revision the desk writes is delivered to
/// the engine as spoken blocks.
///
/// Routes (all exist today): `POST /api/texture/fork` (first turn on a published story),
/// `POST /api/prompt-bar` with a Texture focus (later turns), `GET /api/texture/documents/{id}/stream?format=json&after=`,
/// `GET /api/texture/revisions/{id}`. See mobile/README.md "Server gaps" for what cannot be done yet.
public final class ChoirDeskStage: Desk, @unchecked Sendable {
    public static let stageID = "choir-desk"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Choir desk (your computer)",
                        summary: "Sends what you said to your computer and speaks the revision it writes.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL),
                                 .number("pollSeconds", "Poll every (s)", 1.5, 0.05, 10),
                                 .number("timeoutSeconds", "Give up after (s)", 110, 5, 600),
                                 .number("settleSeconds", "Turn is over after quiet (s)", 4, 0, 60, note: "after the last revision, with no newer one"),
                                 .flag("placeInPrompt", "Say where in the story you were", true)],
                        needsNetwork: true)
    }
    static let textureDesk = "texture"

    let baseURL: String, poll: Double, timeout: Double, placeInPrompt: Bool, settle: Double
    public init(_ p: StageParams) {
        settle = p.double("settleSeconds", 4)
        baseURL = p.string("baseURL", ChoirEnvironment.defaultBaseURL); poll = p.double("pollSeconds", 1.5)
        timeout = p.double("timeoutSeconds", 110); placeInPrompt = p.bool("placeInPrompt", true)
    }

    /// The listener's words exactly as spoken, then (optionally) one plain line saying where they were.
    /// Block-addressed prompts do not exist yet (Server gaps), so the place rides in the text.
    static func prompt(_ t: DeskTurn, playing: PlayingProgramme?, include: Bool) -> String {
        guard include, let p = playing, p.blocks.indices.contains(t.blockIndex) else { return t.listenerWords }
        let at = String(p.blocks[t.blockIndex].prefix(300))
        return t.listenerWords + "\n\n(The listener was hearing paragraph \(t.blockIndex + 1) of \(p.blocks.count) of \"\(p.title)\": \"\(at)\")"
    }

    public func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        let (baseURL, poll, timeout, include, settle) = (self.baseURL, self.poll, self.timeout, self.placeInPrompt, self.settle)
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    try await Self.run(turn, baseURL: baseURL, poll: poll, timeout: timeout, include: include, settle: settle) { c.yield($0) }
                    c.finish()
                } catch is CancellationError { c.finish() }
                catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }

    static func enc(_ s: String) -> String { s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed.subtracting(CharacterSet(charactersIn: "/"))) ?? s }

    static func run(_ turn: DeskTurn, baseURL: String, poll: Double, timeout: Double, include: Bool, settle: Double, deliver: (Revision) -> Void) async throws {
        let env = ChoirEnvironment.shared
        let client = env.client(baseURL: baseURL, timeout: 20)
        let started = ProcessInfo.processInfo.systemUptime
        let playing = env.playing(turn.programmeID)
        let text = prompt(turn, playing: playing, include: include)

        // 1. Send the turn.
        var docID = playing?.docID ?? ""
        var userRevision = ""
        if let p = playing, docID.isEmpty, !p.publicationVersionID.isEmpty, !p.manifestHash.isEmpty {
            let r = try await client.postJSON("/api/texture/fork", ["text": text, "client_request_id": "autoradio-" + turn.turnID,
                                                                   "source": ["publication_version_id": p.publicationVersionID, "manifest_hash": p.manifestHash]],
                                              route: "/api/texture/fork")
            let o = try r.object()
            docID = o["doc_id"] as? String ?? ""; userRevision = o["revision_id"] as? String ?? ""
        } else if !docID.isEmpty {
            let r = try await client.postJSON("/api/prompt-bar", ["text": text, "command_id": "autoradio-" + turn.turnID,
                                                                 "focus": ["app_agent_id": textureDesk, "object_ref": docID]],
                                              route: "/api/prompt-bar")
            let o = try r.object()
            docID = (o["doc_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? docID; userRevision = o["revision_id"] as? String ?? ""
        } else {
            let r = try await client.postJSON("/api/prompt-bar", ["text": text, "command_id": "autoradio-" + turn.turnID], route: "/api/prompt-bar")
            let o = try r.object()
            docID = o["doc_id"] as? String ?? ""; userRevision = o["revision_id"] as? String ?? ""
            if docID.isEmpty, let status = o["status_url"] as? String, status.hasPrefix("/api/prompt-bar/submissions/") {
                // The routed submit did not name a document yet: ask the submission, briefly.
                for _ in 0..<12 {
                    try await Task.sleep(nanoseconds: UInt64(max(poll, 0.05) * 1e9))
                    let s = try await client.send("GET", status, route: "/api/prompt-bar/submissions/{id}").object()
                    if let d = (s["decision"] as? [String: Any])?["doc_id"] as? String, !d.isEmpty {
                        docID = d; userRevision = (s["decision"] as? [String: Any])?["user_revision_id"] as? String ?? userRevision; break
                    }
                    if let e = s["error"] as? String, !e.isEmpty { throw ChoirError.server(status: 200, message: e) }
                }
            }
            if docID.isEmpty { throw ChoirError.gap("the prompt bar routed the words but named no document to follow (no doc_id in the submit or its status)") }
        }
        if docID.isEmpty { throw ChoirError.decode("the computer returned no document id") }
        env.updatePlaying(turn.programmeID) { $0.docID = docID }

        // 2. The listener's own revision number: only the desk's revisions above it are spoken.
        var userVersion = 0
        if !userRevision.isEmpty,
           let rev = try? await client.send("GET", "/api/texture/revisions/\(enc(userRevision))", route: "/api/texture/revisions/{id}").object() {
            userVersion = rev["version_number"] as? Int ?? 0
        }

        // 3. Follow the stream.
        var cursor: Int64 = 0
        var seen = Set<String>()
        var delivered = 0, failures = 0, terminalPolls = 0
        var lastDelivery = 0.0
        // The turn is over when: the document's trajectory settles after a delivered revision; or `settle`
        // seconds pass after the last delivered revision with no newer one (the stream of a live
        // document never closes); or `timeout` passes (an error if nothing was delivered).
        while ProcessInfo.processInfo.systemUptime - started < timeout {
            try Task.checkCancellation()
            if delivered > 0, ProcessInfo.processInfo.systemUptime - lastDelivery >= settle {
                env.log?.emit(.note("choir desk: turn over, \(delivered) revision(s), quiet for \(Int(settle)) s")); return
            }
            do {
                let r = try await client.send("GET", "/api/texture/documents/\(enc(docID))/stream",
                                              query: ["format": "json", "after": String(cursor), "limit": "100"],
                                              route: "/api/texture/documents/{id}/stream", okStatuses: [409])
                failures = 0
                let page = try r.object()
                if r.status == 409 || (page["cursor_expired"] as? Bool) == true { cursor = 0; env.log?.emit(.note("choir desk: cursor expired, replaying the stream")) }
                else {
                    var terminal = false
                    for e in page["events"] as? [[String: Any]] ?? [] {
                        if (e["work_state"] as? String) == "terminal" { terminal = true }
                        guard (e["event_type"] as? String) == "version", let rid = e["revision_id"] as? String, !seen.contains(rid) else { continue }
                        seen.insert(rid)
                        let rev = try await client.send("GET", "/api/texture/revisions/\(enc(rid))", route: "/api/texture/revisions/{id}").object()
                        guard (rev["author_kind"] as? String) == "appagent", (rev["version_number"] as? Int ?? 0) > userVersion else { continue }
                        // The desk answered (even when its text changes nothing to speak): that starts the quiet window.
                        if let r = revision(id: rid, content: rev["content"] as? String ?? "", turn: turn, programmeID: turn.programmeID, env: env) { deliver(r) }
                        delivered += 1; lastDelivery = ProcessInfo.processInfo.systemUptime
                    }
                    if let n = page["next_cursor"] as? Double { cursor = max(cursor, Int64(n)) }
                    if terminal {
                        if delivered > 0 { return }
                        terminalPolls += 1
                        if terminalPolls >= 3 { env.log?.emit(.note("choir desk: the document settled with no revision")); return }
                    }
                }
            } catch is CancellationError { throw CancellationError() }
            catch let e as ChoirError {
                switch e {
                case .notSignedIn, .unauthorized, .forbidden, .rateLimited: throw e
                default:
                    failures += 1
                    if failures >= 3 { throw e }
                    try await Task.sleep(nanoseconds: UInt64(poll * Double(1 << failures) * 1e9))
                }
            }
            try await Task.sleep(nanoseconds: UInt64(max(poll, 0.01) * 1e9))
        }
        if delivered == 0 { throw ChoirError.timeout("the desk wrote no revision in \(Int(timeout)) s") }
    }

    /// Turn a revision body into a Revision from the first paragraph that differs, never before the block after the listener's.
    static func revision(id: String, content: String, turn: DeskTurn, programmeID: String, env: ChoirEnvironment) -> Revision? {
        let old = env.playing(programmeID)?.blocks ?? []
        let title = env.playing(programmeID)?.title ?? ""
        let new = ChoirText.blocks(title: title, text: content, maxBlocks: 400)
        if new.isEmpty || new == old { return nil }
        var diff = 0
        while diff < old.count, diff < new.count, old[diff] == new[diff] { diff += 1 }
        let from = max(diff, turn.blockIndex + 1)
        if old.isEmpty || from >= new.count {
            if old.isEmpty { env.updatePlaying(programmeID) { $0.blocks = new }; return Revision(id: id, fromBlock: turn.blockIndex + 1, blocks: new.map { .speech(text: $0) }, keepRest: false) }
            return nil
        }
        env.updatePlaying(programmeID) { $0.blocks = new }
        return Revision(id: id, fromBlock: from, blocks: new[from...].map { .speech(text: $0) }, keepRest: false)
    }
}
