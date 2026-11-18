import Foundation

// The phone's side of the learning loop (station L6; docs/design/autoradio-learned-document-2026-11-17.md; settlement commit 64e1b34c2 of lane
// learn-settle). Two clients, each the one type that knows its route, with fixtures in mobile/fixtures/learning/. Neither route is deployed
// everywhere: a missing route is "not available yet" (404 or 501), never a failure shown to the listener, and nothing is made up.
//
// THE WIRE (every line is read from the design or from 64e1b34c2 except where marked ASSUMED):
//   GET  /api/autoradio/learning?limit=<1..200>&after=<claim id>
//        {scorecard:{forecasts, settled, heard_through, heard_through_rate|null, brier|null, open, overdue, no_verdict, no_verdict_reasons{},
//         by_kind:[{project, claims, settled, resolved, base_rate, ...}], by_cause:[{cause, claims, settled, resolved, base_rate, ...}]},
//         rows:[{claim_id, stretch_id, segment_id, entry_id, kind, title, p_heard, caused_by, forecast_at, deadline,
//                status settled|no_verdict|open|overdue, outcome, heard_through, brier, settled_at, no_verdict_reason,
//                op:{op_id, by, outcome, last_block, ended_at, position_seconds, version}}], next?, truncated}
//   POST /api/autoradio/steer  {entry_id, direction}                          the design (section 6)
//        ASSUMED extra fields: `stories` (routes the tap refers to), `title`, and for a mute `handle`. A server that refuses the body (400)
//        gets it once more with only the design's fields, as the programme route's `ahead_titles` is handled.
//        `direction` is `more`, `less` or `mute` (design section 6 names more/less and `direction: mute`).

// MARK: The learning route

public struct LearningOp: Equatable, Sendable {
    public var opID = "", by = "", outcome = "", endedAt = ""
    public var lastBlock = 0, version: Int64 = 0
    public var positionSeconds = 0.0
}

public struct LearningRow: Equatable, Sendable {
    public var claimID = "", stretchID = "", segmentID = "", entryID = "", kind = "", title = "", causedBy = "", forecastAt = "", deadline = ""
    public var status = "", outcome = "", settledAt = "", noVerdictReason = ""
    public var pHeard: Double?, brier: Double?
    public var heardThrough: Bool?
    public var op: LearningOp?
}

public struct LearningGroup: Equatable, Sendable {
    public var label = ""
    public var claims = 0, settled = 0, resolved = 0
    public var rate: Double?
    /// Of the resolved forecasts, how many were heard through (the server's base rate is that share).
    public var heard: Int { rate.map { Int(($0 * Double(resolved)).rounded()) } ?? 0 }
}

public struct LearningCard: Equatable, Sendable {
    public var forecasts = 0, settled = 0, heardThrough = 0, open = 0, overdue = 0, noVerdict = 0
    public var heardThroughRate: Double?, brier: Double?
    public var noVerdictReasons: [String: Int] = [:]
    public var byKind: [LearningGroup] = [], byCause: [LearningGroup] = []
}

public struct LearningRead: Equatable, Sendable {
    public var card = LearningCard()
    public var rows: [LearningRow] = []
    public var next: String?
    public var truncated = false
}

public enum LearningLoad: Equatable, Sendable {
    case ready(LearningRead)
    /// The server has no such route (404, 501) or answered in a shape this phone does not know.
    case notYet
    /// No network, or the computer did not answer.
    case offline
    case signedOut
}

public enum LearningWire {
    private static func int(_ v: Any?) -> Int { (v as? NSNumber)?.intValue ?? 0 }
    private static func dbl(_ v: Any?) -> Double? { (v as? NSNumber).map(\.doubleValue).flatMap { $0.isFinite ? $0 : nil } }
    private static func str(_ v: Any?) -> String { (v as? String) ?? "" }

    static func group(_ o: [String: Any], label: String) -> LearningGroup {
        LearningGroup(label: label, claims: int(o["claims"]), settled: int(o["settled"]), resolved: int(o["resolved"]), rate: dbl(o["base_rate"]))
    }

    public static func parse(_ data: Data) -> LearningRead? {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { return nil }
        var r = LearningRead()
        if let s = o["scorecard"] as? [String: Any] {
            var c = LearningCard()
            c.forecasts = int(s["forecasts"]); c.settled = int(s["settled"]); c.heardThrough = int(s["heard_through"]); c.open = int(s["open"])
            c.overdue = int(s["overdue"]); c.noVerdict = int(s["no_verdict"])
            c.heardThroughRate = dbl(s["heard_through_rate"]); c.brier = dbl(s["brier"])
            for (k, v) in (s["no_verdict_reasons"] as? [String: Any]) ?? [:] { c.noVerdictReasons[k] = int(v) }
            c.byKind = ((s["by_kind"] as? [[String: Any]]) ?? []).compactMap { g in str(g["project"]).isEmpty ? nil : group(g, label: str(g["project"])) }
            c.byCause = ((s["by_cause"] as? [[String: Any]]) ?? []).compactMap { g in str(g["cause"]).isEmpty ? nil : group(g, label: str(g["cause"])) }
            r.card = c
        } else if o["rows"] == nil { return nil }
        for d in (o["rows"] as? [[String: Any]]) ?? [] {
            guard !str(d["claim_id"]).isEmpty else { continue }
            var row = LearningRow()
            row.claimID = str(d["claim_id"]); row.stretchID = str(d["stretch_id"]); row.segmentID = str(d["segment_id"]); row.entryID = str(d["entry_id"])
            row.kind = str(d["kind"]); row.title = str(d["title"]); row.causedBy = str(d["caused_by"]); row.forecastAt = str(d["forecast_at"])
            row.deadline = str(d["deadline"]); row.status = str(d["status"]); row.outcome = str(d["outcome"]); row.settledAt = str(d["settled_at"])
            row.noVerdictReason = str(d["no_verdict_reason"]); row.pHeard = dbl(d["p_heard"]); row.brier = dbl(d["brier"]); row.heardThrough = d["heard_through"] as? Bool
            if let p = d["op"] as? [String: Any] {
                row.op = LearningOp(opID: str(p["op_id"]), by: str(p["by"]), outcome: str(p["outcome"]), endedAt: str(p["ended_at"]), lastBlock: int(p["last_block"]),
                                    version: (p["version"] as? NSNumber)?.int64Value ?? 0, positionSeconds: dbl(p["position_seconds"]) ?? 0)
            }
            r.rows.append(row)
        }
        r.next = (o["next"] as? String).flatMap { $0.isEmpty ? nil : $0 }; r.truncated = (o["truncated"] as? Bool) ?? false
        return r
    }
}

public struct LearningClient: Sendable {
    let client: ChoirClient
    public init(client: ChoirClient) { self.client = client }

    public func read(limit: Int = 50, after: String? = nil) async -> LearningLoad {
        guard client.hasKey else { return .signedOut }
        var q = ["limit": String(min(max(limit, 1), 200))]
        if let after, !after.isEmpty { q["after"] = after }
        do {
            let r = try await client.send("GET", "/api/autoradio/learning", query: q, route: "/api/autoradio/learning")
            guard let read = LearningWire.parse(r.data) else { return .notYet }
            return .ready(read)
        } catch ChoirError.notSignedIn { return .signedOut }
        catch ChoirError.unauthorized { return .signedOut }
        catch ChoirError.server(let status, _) where status == 404 || status == 501 { return .notYet }
        catch ChoirError.gap { return .notYet }
        catch { return .offline }
    }
}

// MARK: The steer route and its queue

public struct SteerTap: Codable, Equatable, Sendable {
    public var id: String
    /// The entry on air when the listener tapped; "" when none was (a mute of a publisher heard earlier).
    public var entryID: String
    public var direction: String
    public var stories: [String]
    public var title: String
    public var handle: String?
    public var at: String
    public init(id: String, entryID: String, direction: String, stories: [String], title: String, handle: String?, at: String) {
        self.id = id; self.entryID = entryID; self.direction = direction; self.stories = stories; self.title = title; self.handle = handle; self.at = at
    }
}

public enum SteerSend: Equatable, Sendable { case sent, notYet, offline, refused }

public struct SteerClient: Sendable {
    let client: ChoirClient
    public init(client: ChoirClient) { self.client = client }

    static func body(_ t: SteerTap, bare: Bool) -> [String: Any] {
        var b: [String: Any] = ["direction": t.direction]
        if !t.entryID.isEmpty { b["entry_id"] = t.entryID }
        if !bare {
            if !t.stories.isEmpty { b["stories"] = Array(t.stories.prefix(12)) }
            if !t.title.isEmpty { b["title"] = StretchCandidates.cut(t.title, bytes: 300) }
            if let h = t.handle, !h.isEmpty { b["handle"] = h }
        } else if let h = t.handle, !h.isEmpty { b["handle"] = h }
        return b
    }

    public func send(_ t: SteerTap) async -> SteerSend {
        guard client.hasKey else { return .offline }
        func post(bare: Bool) async throws -> SteerSend {
            _ = try await client.postJSON("/api/autoradio/steer", Self.body(t, bare: bare), route: "/api/autoradio/steer")
            return .sent
        }
        do { return try await post(bare: false) }
        catch ChoirError.server(let status, _) where status == 404 || status == 501 { return .notYet }
        catch ChoirError.server(let status, _) where status == 400 || status == 422 {
            // A correctness refusal: the extra fields are new. Once more with the design's own fields.
            do { return try await post(bare: true) }
            catch ChoirError.server(let s2, _) where s2 == 404 || s2 == 501 { return .notYet }
            catch ChoirError.server(let s2, _) where (400..<500).contains(s2) { return .refused }
            catch { return .offline }
        }
        catch ChoirError.server(let status, _) where (400..<500).contains(status) && status != 429 { return .refused }
        catch { return .offline }
    }
}

/// The taps waiting to be sent, in order, kept in a file so a relaunch finds them. A tap leaves the queue when the server took it or refused it for
/// good; while the route is absent or the network is down it stays. At most 200: the oldest go first.
public final class SteerQueue: @unchecked Sendable {
    public static let maxTaps = 200
    nonisolated(unsafe) public static var shared = SteerQueue(file: TakeFiles.url("steer-queue.json"), send: { _ in .offline })

    private let lock = NSLock()
    private var _taps: [SteerTap] = []
    private var flushing = false
    private var _state = "idle"
    private let file: URL?
    private let send: @Sendable (SteerTap) async -> SteerSend
    public var onChange: (@Sendable () -> Void)?

    public init(file: URL?, send: @escaping @Sendable (SteerTap) async -> SteerSend) {
        self.file = file; self.send = send
        if let file, let d = try? Data(contentsOf: file), let a = try? JSONDecoder().decode([SteerTap].self, from: d) { _taps = Array(a.suffix(Self.maxTaps)) }
    }
    public var taps: [SteerTap] { lock.lock(); defer { lock.unlock() }; return _taps }
    /// Sign-out: the waiting taps go with the account, memory and file (a tap in flight is not re-queued: it is removed only when answered).
    public func wipe() {
        lock.lock(); _taps = []; _state = "idle"; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
        onChange?()
    }
    /// idle | sent | notyet | offline
    public var state: String { lock.lock(); defer { lock.unlock() }; return _state }

    public func setSender(_ s: @escaping @Sendable (SteerTap) async -> SteerSend) { lock.lock(); sender = s; lock.unlock() }
    private var sender: (@Sendable (SteerTap) async -> SteerSend)?

    public func enqueue(_ t: SteerTap) {
        lock.lock()
        _taps.append(t); if _taps.count > Self.maxTaps { _taps.removeFirst(_taps.count - Self.maxTaps) }
        lock.unlock()
        save(); onChange?()
    }

    public func flush() async {
        lock.lock(); if flushing { lock.unlock(); return }; flushing = true; let s = sender ?? send; lock.unlock()
        var sent = 0
        while true {
            lock.lock(); let next = _taps.first; lock.unlock()
            guard let tap = next else { break }
            let r = await s(tap)
            lock.lock()
            switch r {
            case .sent: _taps.removeAll { $0.id == tap.id }; _state = "sent"; sent += 1
            case .refused: _taps.removeAll { $0.id == tap.id }
            case .notYet: _state = "notyet"
            case .offline: _state = "offline"
            }
            lock.unlock()
            save()
            if r == .notYet || r == .offline { break }
        }
        lock.lock(); flushing = false; let kept = _taps.count; lock.unlock()
        print(Self.logLine(sent: sent, kept: kept))
        onChange?()
    }

    /// Counts only: never a title, a story or a word the listener said.
    public static func logLine(sent: Int, kept: Int) -> String { "AUTORADIO steer sent=\(sent) queued=\(kept)" }

    private func save() {
        guard let file else { return }
        lock.lock(); let t = _taps; lock.unlock()
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(t) { try? d.write(to: file, options: .atomic) }
    }
}

public extension SteerTap {
    /// The route a `pub:<route>[@v<n>]` ref names, without the version.
    static func story(_ ref: String) -> String {
        let r = ref.hasPrefix("pub:") ? String(ref.dropFirst(4)) : ref
        return TextureAddress.splitRoute(String(r.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first ?? "")).bare
    }
    /// A tap on the entry on air: the entry's id, the stories it cites, and the story the listener tapped on (when it is another one).
    @MainActor static func on(entry: RunSheetEntry?, direction: String, tappedStory: String? = nil, now: Date = Date()) -> SteerTap? {
        guard let e = entry, e.state == .onAir || e.state == .ahead || e.state.isTail else { return nil }
        var stories: [String] = []
        func add(_ r: String) { if !r.isEmpty, !stories.contains(r) { stories.append(r) } }
        for c in e.cites ?? [] where c.ref.hasPrefix("pub:") { add(SteerTap.story(c.ref)) }
        if let t = tappedStory, t.hasPrefix("pub:") { add(SteerTap.story(t)) }
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime]
        return SteerTap(id: "s-" + UUID().uuidString.prefix(12).lowercased(), entryID: e.id, direction: direction, stories: stories, title: e.displayTitle, handle: nil, at: f.string(from: now))
    }
    static func mute(handle: String, now: Date = Date()) -> SteerTap {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime]
        return SteerTap(id: "s-" + UUID().uuidString.prefix(12).lowercased(), entryID: "", direction: "mute", stories: [], title: "", handle: handle, at: f.string(from: now))
    }
}
