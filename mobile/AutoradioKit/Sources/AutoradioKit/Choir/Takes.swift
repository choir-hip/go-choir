import Foundation
import CryptoKit

// Takes, the phone's side (docs/design/autoradio-learned-document-2026-11-17.md section 11, on origin/lane/learn-design; station L7).
// A take is the listener's recorded voice, published so another listener's DJ may play it.
//
// THE WIRE, as built on origin/lane/takes-server (docs/reference/AUTORADIO-TAKES.md). Every route goes through the signed-in proxy with the
// device key (`Authorization: Bearer`, `X-Choir-Computer`), as the clips list does. All of it is `TakesClient`.
//   POST   /api/platform/takes                multipart/form-data: `audio` (take.m4a, audio/mp4; a real MP4 whose `mvhd` is 3 to 60 s and within 2 s of
//                                             the claim), `transcript` (1 to 2,000 bytes), `duration_seconds`, `sha256`, `responds_to` (JSON {route,title},
//                                             absent when none). Any other field is refused. 201 answers the take. 400 and 429 carry `{error}`, shown plainly.
//                                             There is no separate upload to the listener's computer.
//   GET    /api/platform/takes?limit=50       `{generated_at, takes:[...]}`: every `choir.take` field but the publisher key, plus `audio_url`, `record`.
//   GET    /api/platform/takes?mine=1         the caller's own live takes `{takes:[... state, published_at, outcomes{heard,skipped,drill_*}]}`.
//   GET    /api/platform/takes/{id}/audio     audio/mp4 bytes; 404 once deleted or yanked.
//   DELETE /api/platform/takes/{id}           the publisher deletes their own take (409 once already gone).
//   GET    /api/platform/takes/record?publisher=<handle>   flat: publisher, new, offered, heard, skipped, listeners, takes, summary (+ drill, rows for the publisher).
//   POST   /api/platform/takes/{id}/outcome   NOT the phone's: it refuses a browser session and wants an API key naming the listener's computer; the
//                                             listener's computer posts it after settlement. The phone never calls it.
// A 404 on the takes routes means the server is not deployed yet: `TakesUnavailable`.
//
// Ways this can fail (written before the code; each is a test in TakesTests):
//   T1  the 3 s floor, the 60 s ceiling, the 512 KB bound and the 2,000 byte transcript bound are the spec's, and only those
//   T2  a take list row without an id, a handle or a live state is not a take; a deleted or yanked take is never a candidate
//   T3  candidates: never the listener's own (by id or by handle), never a muted publisher's, at most 3, newest first, each inside the
//       request's bounds, kind `take`; and they count toward the 40
//   T4  a take line without a take the phone holds is dropped; an entry for a held take names the publisher and cites its `responds_to`
//   T5  the upload is a multipart body whose audio, transcript and sha256 agree; the audio is never put in a log
//   T6  a mute is kept on the phone, listed, undone, and survives a relaunch; the same handle twice is one mute
//   T7  the listener's own record is read by their own handle; an unknown handle reads as "no outcomes yet", not a failure

/// The server does not have the takes routes yet (404).
public struct TakesUnavailable: Error, CustomStringConvertible, Equatable, Sendable {
    public init() {}
    public var description: String { "Takes are not available yet." }
}
/// The server's own refusal of a take, in its words.
public struct TakesRefused: Error, CustomStringConvertible, Equatable, Sendable {
    public var description: String
    public init(_ d: String) { description = d }
}

/// The length the file's own `moov/mvhd` box states: what the server checks the claim against. Nil when the file is not a readable MP4.
public enum Mp4Header {
    public static func durationSeconds(_ d: Data) -> Double? {
        func u32(_ o: Int) -> Int? { o >= 0 && o + 4 <= d.count ? d[o..<o+4].reduce(0) { $0 << 8 | Int($1) } : nil }
        func u64(_ o: Int) -> Int? { o >= 0 && o + 8 <= d.count ? d[o..<o+8].reduce(0) { $0 << 8 | Int($1) } : nil }
        func find(_ type: String, _ from: Int, _ to: Int) -> (body: Int, end: Int)? {
            var o = from
            while o + 8 <= to {
                guard var size = u32(o) else { return nil }
                var head = 8
                if size == 1 { guard let big = u64(o + 8) else { return nil }; size = big; head = 16 } else if size == 0 { size = to - o }
                guard size >= head, o + size <= to else { return nil }
                if String(decoding: d[o+4..<o+8], as: UTF8.self) == type { return (o + head, o + size) }
                o += size
            }
            return nil
        }
        guard d.count >= 12, String(decoding: d[4..<8], as: UTF8.self) == "ftyp", let moov = find("moov", 0, d.count), let mv = find("mvhd", moov.body, moov.end),
              mv.body < d.count else { return nil }
        let v = d[mv.body]
        let scale: Int?, dur: Int?
        if v == 1 { scale = u32(mv.body + 20); dur = u64(mv.body + 24) } else { scale = u32(mv.body + 12); dur = u32(mv.body + 16) }
        guard let sc = scale, sc > 0, let du = dur else { return nil }
        return Double(du) / Double(sc)
    }
}

public enum TakeBounds {
    public static let minSeconds = 3.0, maxSeconds = 60.0
    /// The recorder stops this much before the ceiling: a tick is late and the encoder adds about 0.11 s, and the server refuses a claim above 60.
    public static let stopEarly = 0.5
    public static let maxBytes = 512 * 1024
    public static let maxTranscriptBytes = 2000
    public static let maxCandidates = 3
    public static let maxTitleBytes = 120
    public static let listLimit = 50
    /// AAC-LC in .m4a, mono, 24 kHz, 32 kbit/s (11.2).
    public static let sampleRate = 24_000.0, bitRate = 32_000
    public static let publishLine = "Your voice will be offered to other Autoradio listeners. You can delete it at any time."
    public static let rightsLine = "A take with music or another person\u{2019}s recording will be removed."
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
        return StretchCandidates.cut(SegmentPrep.oneLine(first, maxBytes: TakeBounds.maxTitleBytes), bytes: TakeBounds.maxTitleBytes)
    }

    static let iso: ISO8601DateFormatter = { let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]; return f }()
    static let isoPlain = ISO8601DateFormatter()
    static func date(_ s: String?) -> Date? { s.flatMap { iso.date(from: $0) ?? isoPlain.date(from: $0) } }

    /// Nil when the row is not a take the phone can offer (no id, no handle, not live).
    public static func parse(_ o: [String: Any]) -> Take? {
        guard let id = o["take_id"] as? String, validID(id), let who = o["publisher"] as? String, validHandle(who) else { return nil }
        let state = (o["state"] as? String) ?? "live"
        guard state == "live" else { return nil }
        let t = StretchCandidates.cut(SegmentPrep.stripUnsafe((o["transcript"] as? String) ?? ""), bytes: TakeBounds.maxTranscriptBytes)
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

/// One of the listener's own takes (the computer's list).
public struct MyTake: Equatable, Sendable {
    public var takeID: String
    public var duration: Double
    public var transcript: String
    public var published: Bool
    public var publisher: String
    public var at: Date?
    public var title: String { Take(takeID: takeID, publisher: publisher, publishedAt: at, duration: duration, transcript: transcript, respondsTo: nil, record: nil, audioPath: "", state: "live").title }

    public static func parse(_ o: [String: Any]) -> MyTake? {
        guard let id = o["take_id"] as? String, Take.validID(id) else { return nil }
        let state = (o["state"] as? String) ?? ""
        if state == "deleted" || state == "yanked" { return nil }
        let d = (o["duration_seconds"] as? NSNumber)?.doubleValue ?? 0
        return MyTake(takeID: id, duration: d.isFinite ? max(0, d) : 0, transcript: StretchCandidates.cut(SegmentPrep.stripUnsafe((o["transcript"] as? String) ?? ""), bytes: TakeBounds.maxTranscriptBytes),
                      published: (o["published"] as? Bool) ?? ((o["published_at"] as? String) != nil), publisher: (o["publisher"] as? String) ?? "",
                      at: Take.date((o["published_at"] as? String) ?? (o["created_at"] as? String)))
    }
}

// MARK: The client: the one type that knows the server's shapes

public struct TakesClient: Sendable {
    let client: ChoirClient
    public init(client: ChoirClient) { self.client = client }

    /// What a take upload carries: the file's bytes and the facts the computer checks them against.
    public struct Upload: Sendable {
        public var audio: Data
        public var transcript: String
        public var duration: Double
        public var respondsTo: TakeResponds?
        public var sha256: String { Self.hex(audio) }
        public static func hex(_ d: Data) -> String { SHA256.hash(data: d).map { String(format: "%02x", $0) }.joined() }
        public init(audio: Data, transcript: String, duration: Double, respondsTo: TakeResponds?) {
            self.audio = audio; self.transcript = transcript; self.duration = duration; self.respondsTo = respondsTo
        }
    }

    /// The multipart body and its content type. Pure, so a test reads the parts.
    static func multipart(_ u: Upload, boundary: String = "choir-take-" + UUID().uuidString.lowercased()) -> (body: Data, contentType: String) {
        var d = Data()
        func field(_ name: String, _ value: String) {
            d.append(Data("--\(boundary)\r\nContent-Disposition: form-data; name=\"\(name)\"\r\n\r\n\(value)\r\n".utf8))
        }
        field("transcript", u.transcript)
        field("duration_seconds", String(format: "%.2f", u.duration))
        field("sha256", u.sha256)
        if let r = u.respondsTo, let j = try? JSONSerialization.data(withJSONObject: r.json, options: [.sortedKeys]) { field("responds_to", String(decoding: j, as: UTF8.self)) }
        d.append(Data("--\(boundary)\r\nContent-Disposition: form-data; name=\"audio\"; filename=\"take.m4a\"\r\nContent-Type: audio/mp4\r\n\r\n".utf8))
        d.append(u.audio)
        d.append(Data("\r\n--\(boundary)--\r\n".utf8))
        return (d, "multipart/form-data; boundary=\(boundary)")
    }

    /// 404 on a takes route is a server that does not have them yet.
    static func unavailable(_ r: ChoirResponse) throws { if r.status == 404 { throw TakesUnavailable() } }

    /// Publish a take: one multipart POST that carries the audio. The server's own refusal (a bound, the daily limit of 10) is thrown as its words.
    public func publish(_ u: Upload) async throws -> Take {
        guard u.audio.count <= TakeBounds.maxBytes else { throw TakesRefused("The take is over \(TakeBounds.maxBytes / 1024) KB.") }
        let m = Self.multipart(u)
        let r = try await client.send("POST", "/api/platform/takes", body: m.body, route: "/api/platform/takes", timeout: 120, okStatuses: [400, 404, 409, 413, 429], contentType: m.contentType)
        try Self.unavailable(r)
        guard (200..<300).contains(r.status) else {
            let msg = ((try? r.json()) as? [String: Any])?["error"] as? String ?? ""
            throw TakesRefused(msg.isEmpty ? "The server refused the take (HTTP \(r.status))." : String(msg.prefix(300)))
        }
        guard let t = Take.parse((try r.object())) else { throw ChoirError.decode("not a take") }
        return t
    }

    /// The newest live takes of the last 7 days.
    public func list() async throws -> [Take] {
        let r = try await client.send("GET", "/api/platform/takes", query: ["limit": String(TakeBounds.listLimit)], route: "/api/platform/takes", okStatuses: [404])
        try Self.unavailable(r)
        return Self.takes(try r.json())
    }
    static func takes(_ json: Any) -> [Take] {
        let rows = ((json as? [String: Any])?["takes"] as? [[String: Any]]) ?? (json as? [[String: Any]]) ?? []
        var seen = Set<String>()
        return rows.compactMap { Take.parse($0) }.filter { seen.insert($0.takeID).inserted }
    }

    /// The listener's own live takes.
    public func mine() async throws -> [MyTake] {
        let r = try await client.send("GET", "/api/platform/takes", query: ["mine": "1"], route: "/api/platform/takes?mine", okStatuses: [404])
        try Self.unavailable(r)
        let json = try r.json()
        let rows = ((json as? [String: Any])?["takes"] as? [[String: Any]]) ?? (json as? [[String: Any]]) ?? []
        return rows.compactMap { MyTake.parse($0) }
    }

    /// Delete a take (404 and 409: it is already gone).
    public func delete(takeID: String) async throws {
        let id = PathSegment.encode(takeID)
        try await client.send("DELETE", "/api/platform/takes/\(id)", route: "/api/platform/takes/{id}", okStatuses: [404, 409])
    }

    /// A publisher's whole record. A handle with no outcomes reads as new, not as a failure.
    public func record(publisher: String) async throws -> TakeRecordSummary {
        let r = try await client.send("GET", "/api/platform/takes/record", query: ["publisher": publisher], route: "/api/platform/takes/record", okStatuses: [404])
        if r.status == 404 { return TakeRecordSummary() }       // an unknown handle, or no server yet: no outcomes
        let o = try r.object()
        return TakeRecordSummary.parse((o["record"] as? [String: Any]) ?? o) ?? TakeRecordSummary()
    }

    /// The audio bytes of a take (signed in; 404 once deleted or yanked). `path` is the list's `audio_url` path.
    public func audio(path: String) async throws -> Data {
        let r = try await client.send("GET", path, route: "/api/platform/takes/{id}/audio", timeout: 60, accept: "audio/mp4")
        guard !r.data.isEmpty, r.data.count <= TakeBounds.maxBytes * 2 else { throw ChoirError.decode("no audio") }
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

/// The listener's own takes (ids and the handle the platform gave them), so the public list never offers their own voice back to them.
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
    public func owns(_ t: Take) -> Bool { lock.lock(); defer { lock.unlock() }; return ids.contains(t.takeID) || (!_handle.isEmpty && t.publisher == _handle) }
    public func add(id: String, handle: String) {
        lock.lock(); ids.insert(id); if Take.validHandle(handle) { _handle = handle }; lock.unlock(); save()
    }
    public func remove(id: String) { lock.lock(); ids.remove(id); lock.unlock(); save() }
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

/// Voices the listener will not hear. On the phone, in one small file; listed and undone from the page.
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
    @discardableResult public func mute(_ h: String) -> Bool {
        guard Take.validHandle(h) else { return false }
        lock.lock(); let added = !handles.contains(h); if added { handles.append(h) }; lock.unlock()
        if added { save() }
        return added
    }
    @discardableResult public func unmute(_ h: String) -> Bool {
        lock.lock(); let had = handles.contains(h); handles.removeAll { $0 == h }; lock.unlock()
        if had { save() }
        return had
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let a = handles; lock.unlock()
        if let d = try? JSONSerialization.data(withJSONObject: a), (try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)) != nil { try? d.write(to: file, options: .atomic) }
    }
    /// The notebook's own words for the same thing (11.7). The phone has no route to write the notebook yet; this is what that line says.
    public static func notebookLine(_ h: String) -> String { "Never play takes by \(h)." }
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
    public static func select(_ takes: [Take], own: TakeOwn, mutes: TakeMutes, limit: Int = TakeBounds.maxCandidates) -> [Take] {
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
        let ts = takes.prefix(TakeBounds.maxCandidates).map(json)
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
