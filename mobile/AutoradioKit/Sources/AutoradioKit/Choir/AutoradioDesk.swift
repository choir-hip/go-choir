import Foundation

// The Autoradio desk: one request, one streamed answer, spoken sentence by sentence as it is written.
// Contract: docs/reference/AUTORADIO-TURNS.md, `POST /api/autoradio/turns` (read it, do not guess it).
//
// WAYS THIS CAN FAIL (written before the code; each has a test in AutoradioDeskTests):
//  1. The stream carries an `error` event: its `spoken` line is spoken (it is the server's own plain line), the reason goes to the log, and
//     the `done` that follows ends the turn.
//  2. The stream ends or drops with no `done`: with nothing heard yet it is retried once or twice with the SAME turn_id and the SAME
//     words (the only case a turn_id repeats: the server replays a completed answer and answers a failed one anew); with sentences
//     already heard it is NOT retried (the listener would hear them twice): the turn ends "interrupted".
//  3. A 409 (the same id is still being answered, or was used for other words): the first time a fresh turn_id is used; on a retry the same
//     id is tried again after a real backoff (the server says to), a few times, then it is an error.
//  4. 401/400/413/503 before any stream: a typed error, nothing spoken. 404 and 503 (the route is not deployed, or no streaming model is
//     bound) fall back to the polling Texture desk for this turn, so a computer without the route still answers.
//  5. `text_sha256`, if sent, must be the SHA-256 of the exact UTF-8 bytes sent, or the server answers 400: it is omitted (the server hashes
//     the text itself).
//  6. A field over the contract's bound (page text 48 KB, words 2000 bytes, heard text 2000 characters, history lines 1000 bytes) is
//     refused by the server: the app keeps inside each bound itself (it drops history lines that are too long, never half of one).
//  7. Unknown event names (the reserved `handoff`), comment lines, blank lines: ignored. An `error` or `speech` with a bad body: skipped
//     with a log line, never a crash.
//  8. Cancelling (the listener speaks again or presses Cancel) closes the connection: the server's context is cancelled and the turn is
//     settled `interrupted` there.
//  9. There is no timer: the stream ends when the server ends it; the idle transport timeout (no bytes at all) is the only one.

/// What was said in recent turns, oldest first, for the desk's `history` (at most 8 lines).
public final class TurnHistory: @unchecked Sendable {
    public static let shared = TurnHistory()
    public struct Line: Equatable, Sendable { public var role: String, text: String }
    private let lock = NSLock()
    private var lines: [Line] = []
    public init() {}
    /// The listener's words and what the radio said aloud in answer (the local sentences heard, then the desk's).
    public func add(listener: String, radio: [String]) {
        lock.lock(); defer { lock.unlock() }
        let l = listener.trimmingCharacters(in: .whitespacesAndNewlines)
        if !l.isEmpty { lines.append(Line(role: "listener", text: l)) }
        for r in radio { let t = r.trimmingCharacters(in: .whitespacesAndNewlines); if !t.isEmpty { lines.append(Line(role: "radio", text: t)) } }
        if lines.count > 24 { lines.removeFirst(lines.count - 24) }
    }
    /// The last lines that fit the contract (at most 8, each 1 to 1000 bytes); a line over the bound is left out, never cut.
    public func snapshot(max count: Int = 8) -> [Line] {
        lock.lock(); defer { lock.unlock() }
        return Array(lines.filter { !$0.text.isEmpty && $0.text.utf8.count <= 1000 }.suffix(count))
    }
    public func clear() { lock.lock(); lines = []; lock.unlock() }
}

struct SSEEvent: Equatable { var name: String; var data: String }

/// Server-sent events, one event per `data:` line (the contract's events are single-line JSON). Blank lines may or may not reach it
/// (`AsyncLineSequence` drops them), so an event is complete at its `data:` line.
struct SSEParser {
    private var name = "message"
    mutating func push(_ line: String) -> SSEEvent? {
        if line.isEmpty { name = "message"; return nil }
        if line.hasPrefix(":") { return nil }                         // a comment (": autoradio")
        if line.hasPrefix("event:") { name = line.dropFirst(6).trimmingCharacters(in: .whitespaces); return nil }
        if line.hasPrefix("data:") {
            var d = String(line.dropFirst(5)); if d.hasPrefix(" ") { d.removeFirst() }
            defer { name = "message" }
            return SSEEvent(name: name, data: d)
        }
        return nil
    }
}

public final class AutoradioDeskStage: StreamingDesk, @unchecked Sendable {
    public static let stageID = "autoradio-desk"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Autoradio desk (your computer, streamed)",
                        summary: "Sends what you said and the page in front to your computer's Autoradio desk and speaks its answer sentence by sentence as it is written. Falls back to the Texture desk when the route is not there.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL)],
                        needsNetwork: true)
    }
    let baseURL: String
    let history: TurnHistory
    /// Parameters handed to the polling desk when the route is not there (poll and settle times, for tests).
    let fallbackParams: [String: JSONValue]
    public init(_ p: StageParams, history: TurnHistory = .shared) {
        baseURL = p.string("baseURL", ChoirEnvironment.defaultBaseURL); self.history = history
        fallbackParams = p.values.filter { ["pollSeconds", "settleSeconds"].contains($0.key) }
    }

    /// A turn id the route accepts: `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}`.
    static func routeID(_ s: String) -> String {
        var t = String(s.unicodeScalars.map { ($0.isASCII && (CharacterSet.alphanumerics.contains($0) || "_.-".unicodeScalars.contains($0))) ? Character($0) : "-" })
        while let f = t.first, !f.isLetter && !f.isNumber { t.removeFirst() }
        if t.isEmpty { t = "turn-" + UUID().uuidString.prefix(8).lowercased() }
        return String(t.prefix(64))
    }

    /// The request body: exactly the contract's fields, each inside its bound. `text_sha256` is omitted on purpose.
    static func body(_ turn: DeskTurn, id: String, history: [TurnHistory.Line]) -> [String: Any] {
        var b: [String: Any] = ["turn_id": id, "words": String(decoding: Array(turn.listenerWords.trimmingCharacters(in: .whitespacesAndNewlines).utf8.prefix(2000)), as: UTF8.self)]
        if let p = turn.page, !(p.address.isEmpty && p.title.isEmpty && p.text.isEmpty) {
            var page: [String: Any] = [:]
            if !p.address.isEmpty, p.address.utf8.count <= 512 { page["address"] = p.address }
            if !p.title.isEmpty { page["title"] = String(decoding: Array(p.title.utf8.prefix(300)), as: UTF8.self) }
            if !p.text.isEmpty { page["text"] = String(decoding: Array(p.text.utf8.prefix(48_000)), as: UTF8.self) }
            if !page.isEmpty { b["page"] = page }
        }
        // What the listener had heard of the programme when they cut in: the contract's `heard`.
        if turn.interrupted, !turn.spokenText.isEmpty {
            b["heard"] = ["block_index": max(0, turn.cutBlockIndex ?? turn.blockIndex), "text": String(turn.spokenText.suffix(2000))] as [String: Any]
        }
        if !turn.programmeID.isEmpty, turn.programmeID.utf8.count <= 128 { b["programme"] = turn.programmeID }
        if !history.isEmpty { b["history"] = history.map { ["role": $0.role, "text": $0.text] } }
        return b
    }

    /// The streamed answer as sentences.
    public func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error> {
        let (baseURL, history, fb) = (self.baseURL, self.history.snapshot(), self.fallbackParams)
        return AsyncThrowingStream { c in
            let task = Task {
                do { try await Self.run(turn, baseURL: baseURL, history: history, fallbackParams: fb) { c.yield($0) }; c.finish() }
                catch is CancellationError { c.finish() }
                catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }

    /// The `Desk` shape for a consumer that wants revisions: every sentence is a revision, inserted after the one before.
    public func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        let events = speech(turn, heard: HeardLines())
        return AsyncThrowingStream { c in
            let task = Task {
                var n = 0
                do {
                    for try await e in events {
                        var text: String?
                        if case .sentence(_, let t) = e { text = t } else if case .failed(let spoken, _) = e { text = spoken }
                        guard let text else { continue }
                        c.yield(Revision(id: "autoradio-\(turn.turnID)-\(n)", fromBlock: turn.blockIndex + 1 + n, blocks: [.speech(text: text)], keepRest: true, isFinal: false))
                        n += 1
                    }
                    c.finish()
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }

    static func run(_ turn: DeskTurn, baseURL: String, history: [TurnHistory.Line], fallbackParams: [String: JSONValue] = [:], deliver: (DeskEvent) -> Void) async throws {
        let env = ChoirEnvironment.shared
        let client = env.client(baseURL: baseURL)
        var id = routeID(turn.turnID)
        var attempt = 0, freshIDUsed = false
        while true {
            attempt += 1
            try Task.checkCancellation()
            var sentences = 0, sawDone = false
            do {
                let (_, lines) = try await client.postEventStream("/api/autoradio/turns", body(turn, id: id, history: history), route: "/api/autoradio/turns")
                var parser = SSEParser()
                do {
                    for try await line in lines {
                        try Task.checkCancellation()
                        guard let e = parser.push(line), let obj = (try? JSONSerialization.jsonObject(with: Data(e.data.utf8))) as? [String: Any] else { continue }
                        switch e.name {
                        case "speech":
                            guard let text = obj["text"] as? String, !text.isEmpty else { env.log?.emit(.note("autoradio desk: a speech event without text, skipped")); continue }
                            deliver(.sentence(seq: obj["seq"] as? Int ?? sentences, text: text)); sentences += 1
                        case "error":
                            let spoken = obj["spoken"] as? String ?? ""
                            env.log?.emit(.error(stage: "desk", message: "the Autoradio desk reported: \(obj["reason"] as? String ?? "(no reason)")"))
                            if !spoken.isEmpty { deliver(.failed(spoken: spoken, reason: obj["reason"] as? String ?? "")); sentences += 1 }
                        case "done":
                            sawDone = true
                            let t = obj["timings_ms"] as? [String: Any]
                            deliver(.done(status: obj["status"] as? String ?? "completed", sentences: obj["sentences"] as? Int ?? sentences,
                                          serverFirstSentenceMs: (t?["first_sentence"] as? NSNumber)?.doubleValue, serverDoneMs: (t?["done"] as? NSNumber)?.doubleValue))
                        default: continue                                    // handoff (reserved), anything unknown
                        }
                        if sawDone { return }
                    }
                } catch is CancellationError { throw CancellationError() }
                catch let e as URLError where e.code == .cancelled { throw CancellationError() }
                catch let e as URLError {
                    env.log?.emit(.note("autoradio desk: the stream dropped (\(e.code.rawValue)) after \(sentences) sentences"))
                }
                // The stream ended or dropped without `done`.
                if sentences > 0 { deliver(.done(status: "interrupted", sentences: sentences, serverFirstSentenceMs: nil, serverDoneMs: nil)); return }
                if attempt >= 3 { throw ChoirError.transport("the Autoradio stream ended without a done event, \(attempt) times") }
                env.log?.emit(.note("autoradio desk: the stream ended before any sentence; asking again with the same turn id"))
                try await Task.sleep(nanoseconds: 300_000_000 * UInt64(attempt))     // a backoff after a real failure, not a limit on the model
            } catch ChoirError.server(let status, let message) where status == 409 {
                if !freshIDUsed && attempt == 1 { freshIDUsed = true; id = routeID("turn-" + UUID().uuidString.prefix(8).lowercased()); env.log?.emit(.note("autoradio desk: 409, using a fresh turn id")); continue }
                if attempt >= 4 { throw ChoirError.server(status: 409, message: message) }
                try await Task.sleep(nanoseconds: 500_000_000 * UInt64(attempt))     // the server asks for a short wait before retrying a turn still being answered
            } catch ChoirError.server(let status, _) where (status == 404 || status == 503) && sentences == 0 && attempt == 1 {
                env.log?.emit(.note("autoradio desk: the route answered \(status); this turn goes to the Texture desk"))
                try await fallback(turn, baseURL: baseURL, params: fallbackParams, deliver: deliver)
                return
            }
        }
    }

    /// The polling Texture desk for a computer without the route: each revision's spoken blocks become sentences.
    static func fallback(_ turn: DeskTurn, baseURL: String, params: [String: JSONValue], deliver: (DeskEvent) -> Void) async throws {
        var pv = params; pv["baseURL"] = .string(baseURL)
        let desk = ChoirDeskStage(StageParams(pv))
        var n = 0
        for try await rev in desk.submit(turn) {
            for b in rev.blocks { if let t = b.spokenText, !t.isEmpty { deliver(.sentence(seq: n, text: t)); n += 1 } }
        }
        deliver(.done(status: "completed", sentences: n, serverFirstSentenceMs: nil, serverDoneMs: nil))
    }
}
