import Foundation

// Activity (lane 7): what the computer and the desk are doing now and recently. Sources:
//   the owner's changelog      GET /api/changelog?minor=1&limit=50  (revisions, updates, the desk's wake decisions, the paper using
//                              the owner's work)  {events:[{inbox_seq, level, changelog:{kind, occurred_at, summary, actor_label,
//                              subject:{doc_id, route_path}, tape_ref:{doc_id, revision_id}}}], head, seen_seq, unseen_major}
//   the radio's turns          the lab's kept turns (what the listener asked and how it ended), from the engine
//   now                        the radio's phase and the computer's state, from what the app already holds
// Demo mode reads the fixture mobile/fixtures/demo/changelog.json. The changelog "seen" cursor (PUT /api/changelog/seen) is not
// moved by this page yet.
//
// Ways it can fail (written before the code; tests in SettingsDataTests):
//  V1 A summary or a verdict reason is shown as markup or as an instruction: rows carry plain text only, cut to 240 characters.
//  V2 An event whose kind is new to the phone is dropped: it shows under the kind's words with underscores removed.
//  V3 An event with no document or route is shown as tappable: its target is empty and the row is not a button.
//  V4 A bad date shows "1970": the row says nothing about when instead.
//  V5 The route is missing (404) or the key is refused: the page says so in one plain line and still shows the radio's turns.
//  V6 A route path that is an absolute URL or has a scheme is opened as a page: only a path that starts with "/" and has no ":" is
//     turned into `pub:<route>`.

public struct ActivityItem: Codable, Sendable, Equatable {
    public var id = "", kind = "", label = "", detail = "", group = "", when = "", target = "", title = ""
    public var at = 0.0
    public var unseen = false
    public init() {}
}

public protocol ActivityClient: Sendable { func events() async throws -> [ActivityItem] }

public enum ActivityParse {
    static let labels: [String: String] = [
        "parent_used": "Your work was used", "oped_verdict": "Op-ed verdict", "update_started": "Update started",
        "update_progress": "Update progress", "update_finished": "Update finished", "update_interrupted": "Update interrupted",
        "revision_committed": "Revision saved", "wake_decision": "Desk decision", "oped_arrival": "Op-ed arrived",
        "gate_unavailable": "Gate unavailable",
    ]
    /// computer | desk | paper
    static func group(_ kind: String) -> String {
        if kind.hasPrefix("update_") || kind == "gate_unavailable" { return "computer" }
        if kind == "parent_used" || kind.hasPrefix("oped_") { return "paper" }
        return "desk"
    }
    public static func label(_ kind: String) -> String { labels[kind] ?? kind.replacingOccurrences(of: "_", with: " ").capitalized }   // V2

    public static func target(docID: String, route: String) -> String {
        if !docID.isEmpty { return "texture:\(docID)" }
        if route.hasPrefix("/"), !route.contains(":"), route.count > 1 { return "pub:" + String(route.dropFirst()) }                              // V6
        return ""                                                                                                                       // V3
    }

    static func date(_ s: String) -> Double? {
        let f = ISO8601DateFormatter()
        if let d = f.date(from: s) { return d.timeIntervalSince1970 }
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f.date(from: s)?.timeIntervalSince1970
    }

    public static func when(_ at: Double, now: Date = Date()) -> String {
        guard at > 86400 else { return "" }                                                                                              // V4
        let d = Date(timeIntervalSince1970: at), age = now.timeIntervalSince(d)
        if age >= 0, age < 3600 { let m = max(1, Int(age / 60)); return m == 1 ? "a minute ago" : "\(m) minutes ago" }
        if age >= 0, age < 86400 { let h = Int(age / 3600); return h == 1 ? "an hour ago" : "\(h) hours ago" }
        let f = DateFormatter(); f.locale = Locale(identifier: "en_US_POSIX"); f.dateFormat = "d MMM, HH:mm"
        return f.string(from: d)
    }

    /// The changelog route's answer as rows, newest first.
    public static func items(from data: Data, now: Date = Date()) throws -> [ActivityItem] {
        guard let o = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw ChoirError.decode("changelog: not an object") }
        let seen = (o["seen_seq"] as? Double) ?? 0
        return (o["events"] as? [[String: Any]] ?? []).compactMap { e in
            guard let c = e["changelog"] as? [String: Any] else { return nil }
            var it = ActivityItem()
            let seq = (e["inbox_seq"] as? Double) ?? 0
            let kind = c["kind"] as? String ?? "event"
            let subject = c["subject"] as? [String: Any] ?? [:], tape = c["tape_ref"] as? [String: Any] ?? [:]
            it.id = (c["event_id"] as? String) ?? "seq-\(Int(seq))"
            it.kind = kind; it.label = label(kind); it.group = group(kind)
            it.detail = String(((c["summary"] as? String) ?? "").prefix(240))                                                         // V1
            it.title = (c["actor_label"] as? String) ?? ""
            it.at = date(c["occurred_at"] as? String ?? "") ?? 0
            it.when = when(it.at, now: now)
            it.target = target(docID: (subject["doc_id"] as? String) ?? (tape["doc_id"] as? String) ?? "", route: subject["route_path"] as? String ?? "")
            it.unseen = seq > seen && (e["level"] as? String ?? "major") == "major"
            return it
        }.sorted { $0.at > $1.at }
    }
}

public struct ChoirActivityClient: ActivityClient {
    public init() {}
    public func events() async throws -> [ActivityItem] {
        let r = try await ChoirEnvironment.shared.client().send("GET", "/api/changelog", query: ["limit": "50", "minor": "1"], route: "/api/changelog")
        return try ActivityParse.items(from: r.data)
    }
}

public struct DemoActivityClient: ActivityClient {
    public init() {}
    public func events() async throws -> [ActivityItem] {
        guard let d = Demo.fixture("changelog") else { throw ChoirError.decode("no demo fixture") }
        return try ActivityParse.items(from: d)
    }
}

public struct ActivityView: Codable, Sendable, Equatable {
    /// idle | loading | ok | failed
    public var status = "idle"
    public var message = ""
    public var now: [ActivityItem] = []
    public var computer: [ActivityItem] = []
    public var desk: [ActivityItem] = []
    public var paper: [ActivityItem] = []
    public var radio: [ActivityItem] = []
    public var unseen = 0
    public var empty = false
    public init() {}
}

public final class ActivityModel: @unchecked Sendable {
    private let client: ActivityClient
    private let lock = NSLock()
    private var events: [ActivityItem] = []
    private var status = "idle", message = ""
    private var generation = 0
    /// The radio's turns (newest first) and the "now" lines, supplied by the app from the engine.
    public var radioTurns: @Sendable () -> [ActivityItem] = { [] }
    public var nowLines: @Sendable () -> [ActivityItem] = { [] }
    public var signedIn: @Sendable () -> Bool = { true }
    public var onChange: (@Sendable () -> Void)?
    public init(client: ActivityClient) { self.client = client }

    public func view() -> ActivityView {
        lock.lock(); let ev = events, st = status, msg = message; lock.unlock()
        var v = ActivityView()
        v.status = st; v.message = msg
        v.now = nowLines(); v.radio = Array(radioTurns().prefix(12))
        v.computer = ev.filter { $0.group == "computer" }; v.desk = ev.filter { $0.group == "desk" }; v.paper = ev.filter { $0.group == "paper" }
        v.unseen = ev.filter { $0.unseen }.count
        v.empty = ev.isEmpty && v.radio.isEmpty && v.now.isEmpty && st != "loading"
        return v
    }

    public func refresh() async {
        guard signedIn() else { return }
        lock.lock(); generation += 1; let g = generation; status = "loading"; message = ""; lock.unlock(); onChange?()
        var got: [ActivityItem] = [], st = "ok", msg = ""
        do { got = try await client.events() }
        catch { st = "failed"; msg = Self.plain(error) }                                                                                 // V5
        lock.lock(); if g == generation { events = got; status = st; message = msg }; lock.unlock()
        onChange?()
    }

    static func plain(_ e: Error) -> String {
        switch e as? ChoirError {
        case .unauthorized?: return "This phone's sign-in is no longer accepted."
        case .server(let s, _)? where s == 404: return "This server has no activity feed yet."
        case .notSignedIn?: return "Not signed in."
        default: return "Could not load your computer's activity."
        }
    }
}
