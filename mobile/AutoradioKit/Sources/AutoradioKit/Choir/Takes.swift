import Foundation

// Takes, what the Swift PROGRAMME still needs. The takes page (record, review, publish, delete, mute, the listener's own takes and record) and the wire for them are shared code now
// (AutoradioLab/takes.ts, take-session.ts). What stays here, with the reason: the programme request (RadioProgramme.candidates, native: it builds the request the run sheet feeds from) lists the
// public takes (`TakesClient.list`), filters them by the listener's own ids and muted voices (`TakeOwn`, `TakeMutes`: now READ-ONLY copies that shared code pushes after every change, `replace`),
// and holds them by id (`TakeStore`) for the run sheet entry and the audio fetch (`TakesClient.audio`). They go when the programme request moves.
//
// THE WIRE (docs/reference/AUTORADIO-TAKES.md): GET /api/platform/takes?limit=50 `{generated_at, takes:[...]}`; GET /api/platform/takes/{id}/audio audio/mp4 (404 once deleted or yanked).
// A 404 on the takes routes means the server is not deployed yet: `TakesUnavailable`.

/// The server does not have the takes routes yet (404).
public struct TakesUnavailable: Error, CustomStringConvertible, Equatable, Sendable {
    public init() {}
    public var description: String { "Takes are not available yet." }
}
/// The story that was on air when recording began (`responds_to`): provenance for the take.
public struct TakeResponds: Codable, Equatable, Sendable {
    public var route: String
    public var title: String
    public init(route: String, title: String) { self.route = route; self.title = title }

    var json: [String: Any] { ["route": route, "title": title] }
    static func parse(_ v: Any?) -> TakeResponds? {
        guard let o = v as? [String: Any], let r = o["route"] as? String, !r.isEmpty else { return nil }
        return TakeResponds(route: PodcastClip.bare(r), title: (o["title"] as? String) ?? "")
    }

    /// The story an entry on air is about: its page, else the first story it cites. Nil for an entry with none (an opening, an answer, a take).
    @MainActor public static func from(entry e: RunSheetEntry?) -> TakeResponds? {
        guard let e, e.kind != .take, e.kind != .transition else { return nil }
        var route = ""
        let addr = RadioPage.openAddress(e)
        if addr.hasPrefix("pub:") { route = String(addr.dropFirst(4)) }
        if route.isEmpty, let c = RadioPage.builtOn(e).first(where: { $0.address.hasPrefix("pub:") }) { route = String(c.address.dropFirst(4)); return TakeResponds(route: route, title: c.title) }
        guard !route.isEmpty else { return nil }
        return TakeResponds(route: route, title: e.displayTitle)
    }
}

/// A publisher's record, as the DJ and the listener read it (11.6).
public struct TakeRecordSummary: Equatable, Sendable {
    public var offered = 0, heard = 0, skipped = 0, listeners = 0, takes = 0
    public var isNew = true
    public var summary = ""
    public init() {}

    /// "heard through by 7 of 11 listeners over 5 takes"; "new: no outcomes yet" when nothing has been reported.
    public var words: String {
        if !summary.isEmpty { return summary }
        if isNew || (heard + skipped) == 0 { return "new: no outcomes yet" }
        let n = max(listeners, 1)
        return "heard through by \(heard) of \(heard + skipped) listeners over \(takes) take\(takes == 1 ? "" : "s")" + (n > 0 ? "" : "")
    }

    public static func parse(_ v: Any?) -> TakeRecordSummary? {
        guard let o = v as? [String: Any] else { return nil }
        var r = TakeRecordSummary()
        func int(_ k: String) -> Int { (o[k] as? NSNumber)?.intValue ?? 0 }
        r.offered = int("offered"); r.heard = int("heard"); r.skipped = int("skipped"); r.listeners = int("listeners"); r.takes = int("takes")
        r.summary = StretchCandidates.cut(StretchCandidates.oneLine((o["summary"] as? String) ?? ""), bytes: 200)
        if let n = o["new"] as? Bool { r.isNew = n } else { r.isNew = (r.heard + r.skipped) == 0 }
        return r
    }
}

/// A take as the public list serves it.
public struct Take: Equatable, Sendable {
    public var takeID: String
    public var publisher: String
    public var publishedAt: Date?
    public var duration: Double
    public var transcript: String
    public var respondsTo: TakeResponds?
    public var record: TakeRecordSummary?
    /// The audio route the list names (a path on the Choir host); the default is the spec's.
    public var audioPath: String
    public var state: String

    public static func audioPath(_ id: String) -> String { "/api/platform/takes/\(PathSegment.encode(id))/audio" }
    static func validID(_ s: String) -> Bool { !s.isEmpty && s.utf8.count <= 64 && s.unicodeScalars.allSatisfy { ($0.value < 128) && (CharacterSet.alphanumerics.contains($0) || $0 == "-" || $0 == "_") } }
    static func validHandle(_ s: String) -> Bool { !s.isEmpty && s.utf8.count <= 64 && !s.unicodeScalars.contains { CharacterSet.whitespacesAndNewlines.contains($0) || $0.properties.generalCategory == .control } }

    /// The first line of the transcript, cut to 120 bytes.
    public var title: String {
        let first = transcript.components(separatedBy: .newlines).first(where: { !$0.trimmingCharacters(in: .whitespaces).isEmpty }) ?? transcript
        return StretchCandidates.cut(SegmentPrep.oneLine(first, maxBytes: 120), bytes: 120)
    }

    static let iso: ISO8601DateFormatter = { let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]; return f }()
    static let isoPlain = ISO8601DateFormatter()
    static func date(_ s: String?) -> Date? { s.flatMap { iso.date(from: $0) ?? isoPlain.date(from: $0) } }

    /// Nil when the row is not a take the phone can offer (no id, no handle, not live).
    public static func parse(_ o: [String: Any]) -> Take? {
        guard let id = o["take_id"] as? String, validID(id), let who = o["publisher"] as? String, validHandle(who) else { return nil }
        let state = (o["state"] as? String) ?? "live"
        guard state == "live" else { return nil }
        let t = StretchCandidates.cut(SegmentPrep.stripUnsafe((o["transcript"] as? String) ?? ""), bytes: 2000)
        let d = (o["duration_seconds"] as? NSNumber)?.doubleValue ?? 0
        return Take(takeID: id, publisher: who, publishedAt: date(o["published_at"] as? String), duration: d.isFinite ? max(0, d) : 0, transcript: t,
                    respondsTo: TakeResponds.parse(o["responds_to"]), record: TakeRecordSummary.parse(o["record"]),
                    audioPath: Take.pathOf(o["audio_url"] as? String) ?? audioPath(id), state: state)
    }
    /// Only a path on the Choir host is followed: the key goes to that host and nowhere else.
    static func pathOf(_ s: String?) -> String? {
        guard let s, !s.isEmpty else { return nil }
        if s.hasPrefix("/"), !s.hasPrefix("//") { return s }
        if let u = URL(string: s), u.scheme != nil, u.host != nil { return u.path.isEmpty ? nil : u.path + (u.query.map { "?" + $0 } ?? "") }
        return nil
    }
}

// MARK: The client: the one type that knows the server's shapes

public struct TakesClient: Sendable {
    let client: ChoirClient
    public init(client: ChoirClient) { self.client = client }

        /// 404 on a takes route is a server that does not have them yet.
    static func unavailable(_ r: ChoirResponse) throws { if r.status == 404 { throw TakesUnavailable() } }

    /// The newest live takes of the last 7 days.
    public func list() async throws -> [Take] {
        let r = try await client.send("GET", "/api/platform/takes", query: ["limit": String(50)], route: "/api/platform/takes", okStatuses: [404])
        try Self.unavailable(r)
        return Self.takes(try r.json())
    }
    static func takes(_ json: Any) -> [Take] {
        let rows = ((json as? [String: Any])?["takes"] as? [[String: Any]]) ?? (json as? [[String: Any]]) ?? []
        var seen = Set<String>()
        return rows.compactMap { Take.parse($0) }.filter { seen.insert($0.takeID).inserted }
    }

    /// The audio bytes of a take (signed in; 404 once deleted or yanked). `path` is the list's `audio_url` path.
    public func audio(path: String) async throws -> Data {
        let r = try await client.send("GET", path, route: "/api/platform/takes/{id}/audio", timeout: 60, accept: "audio/mp4")
        guard !r.data.isEmpty, r.data.count <= (512 * 1024) * 2 else { throw ChoirError.decode("no audio") }
        return r.data
    }
}

// MARK: What the phone keeps: its own ids, its handle, the muted voices

/// Where the phone keeps these (its own support folder).
enum TakeFiles {
    static func url(_ name: String) -> URL? {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.appendingPathComponent("Autoradio").appendingPathComponent(name)
    }
}

/// The listener's own takes (ids and the handle the platform gave them), so the public list never offers their own voice back to them. A READ-ONLY copy: shared code owns the data
/// (app:/data/takes.json) and pushes it with `replace` after every change (take-session.ts `keep`); the file is also what `takeLegacy` reads once on an upgraded phone, so it is never removed here
/// except at sign-out.
public final class TakeOwn: @unchecked Sendable {
    public static let shared = TakeOwn(file: TakeFiles.url("takes-own.json"))
    private let lock = NSLock()
    private var ids: Set<String> = []
    private var _handle = ""
    private let file: URL?
    public init(file: URL? = nil) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
            ids = Set((o["ids"] as? [String]) ?? []); _handle = (o["handle"] as? String) ?? ""
        }
    }
    public var handle: String { lock.lock(); defer { lock.unlock() }; return _handle }
    /// The ids as the file holds them (the carry-over's read).
    public var allIDs: [String] { lock.lock(); defer { lock.unlock() }; return ids.sorted() }
    public func owns(_ t: Take) -> Bool { lock.lock(); defer { lock.unlock() }; return ids.contains(t.takeID) || (!_handle.isEmpty && t.publisher == _handle) }
    /// Shared code's copy: the whole state, kept in the same file as before.
    public func replace(ids new: [String], handle h: String) {
        lock.lock(); ids = Set(new); _handle = Take.validHandle(h) ? h : ""; lock.unlock(); save()
    }
    /// Sign-out: the ids and the handle are the account's.
    public func wipe() {
        lock.lock(); ids = []; _handle = ""; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let o: [String: Any] = ["ids": Array(ids).sorted(), "handle": _handle]; lock.unlock()
        if let d = try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys]) { try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true); try? d.write(to: file, options: .atomic) }
    }
}

/// Voices the listener will not hear: a READ-ONLY copy of shared code's list (app:/data/takes.json), pushed with `replace`; the file is the old Swift one, kept current so a reverted build and the
/// carry-over find it.
public final class TakeMutes: @unchecked Sendable {
    public static let shared = TakeMutes(file: TakeFiles.url("takes-muted.json"))
    private let lock = NSLock()
    private var handles: [String] = []
    private let file: URL?
    public init(file: URL? = nil) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file), let a = (try? JSONSerialization.jsonObject(with: d)) as? [String] { handles = a.filter(Take.validHandle) }
    }
    public var all: [String] { lock.lock(); defer { lock.unlock() }; return handles }
    /// Sign-out: the muted voices are the account's.
    public func wipe() {
        lock.lock(); handles = []; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    public func isMuted(_ h: String) -> Bool { lock.lock(); defer { lock.unlock() }; return handles.contains(h) }
    public func replace(_ new: [String]) {
        lock.lock(); handles = new.filter(Take.validHandle); lock.unlock()
        save()
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let a = handles; lock.unlock()
        if let d = try? JSONSerialization.data(withJSONObject: a), (try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)) != nil { try? d.write(to: file, options: .atomic) }
    }
}

/// The takes the phone holds from the public list, by id: the DJ's line names one, and the phone plays it from here.
public final class TakeStore: @unchecked Sendable {
    public static let shared = TakeStore()
    private let lock = NSLock()
    private var byID: [String: Take] = [:]
    public init() {}
    public func replace(_ takes: [Take]) { lock.lock(); byID = Dictionary(takes.map { ($0.takeID, $0) }, uniquingKeysWith: { a, _ in a }); lock.unlock() }
    public func take(_ id: String) -> Take? { lock.lock(); defer { lock.unlock() }; return byID[id] }
    public func wipe() { lock.lock(); byID = [:]; lock.unlock() }
}

// MARK: Candidates (11.5 step 1)

public enum TakeCandidates {
    /// At most 3 takes for the request: live, not the listener's own, not a muted publisher's, newest first.
    public static func select(_ takes: [Take], own: TakeOwn, mutes: TakeMutes, limit: Int = 3) -> [Take] {
        let sorted = takes.enumerated().sorted { a, b in
            switch (a.element.publishedAt, b.element.publishedAt) {
            case let (x?, y?) where x != y: return x > y
            case (_?, nil): return true
            case (nil, _?): return false
            default: return a.offset < b.offset
            }
        }.map(\.element)
        return Array(sorted.filter { $0.state == "live" && !own.owns($0) && !mutes.isMuted($0.publisher) && !$0.transcript.isEmpty }.prefix(max(0, limit)))
    }

    /// `{kind, take_id, publisher, title, transcript, duration_seconds, responds_to, record}`. The audio address stays on the phone.
    public static func json(_ t: Take) -> [String: Any] {
        var o: [String: Any] = ["kind": "take", "take_id": t.takeID, "publisher": t.publisher, "title": t.title, "transcript": t.transcript,
                                "duration_seconds": (t.duration * 10).rounded() / 10, "record": (t.record ?? TakeRecordSummary()).words]
        if let r = t.respondsTo { o["responds_to"] = r.json }
        return o
    }

    /// The request's candidates: the stories, then the takes, together at most 40 (a take counts toward the bound).
    public static func merged(stories: [[String: Any]], takes: [Take]) -> [[String: Any]] {
        let ts = takes.prefix(3).map(json)
        return Array(stories.prefix(max(0, StretchCandidates.maxItems - ts.count))) + ts
    }
}

// MARK: The take entry on the run sheet (11.5 step 4)

extension RunSheetEntry {
    /// The id of the take this entry plays (`take:<id>`).
    public var takeID: String? {
        guard kind == .take, let r = source?.ref, r.hasPrefix("take:") else { return nil }
        let id = String(r.dropFirst(5))
        return Take.validID(id) ? id : nil
    }
    /// A take on the sheet: the lead-in, then the publisher's voice. `cites` are the stories it was built on.
    public static func take(_ t: Take, leadIn: String, id: String, cites: [RunSheetCite] = [], forecast: RunSheetForecast? = nil, by: RunSheetBy = .desk) -> RunSheetEntry {
        var cs = cites
        if let r = t.respondsTo, !cs.contains(where: { $0.ref.hasPrefix("pub:" + r.route) }) {
            cs.append(RunSheetCite(ref: "pub:" + r.route, role: .mentioned, label: SegmentPrep.label(r.title)))
        }
        var e = RunSheetEntry(id: id, kind: .take, placedBy: by, title: t.title, text: leadIn.isEmpty ? nil : leadIn, source: RunSheetSource(ref: "take:" + t.takeID, label: t.publisher),
                              cites: cs.isEmpty ? nil : cs, estSeconds: t.duration + (leadIn.isEmpty ? 0 : RunSheetEntry.estimateSeconds(words: leadIn)), forecast: forecast)
        e.origin = nil
        return e
    }
}
