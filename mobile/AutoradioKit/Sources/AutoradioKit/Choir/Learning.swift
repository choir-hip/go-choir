import Foundation

// The phone's side of steering (station L6; docs/design/autoradio-learned-document-2026-11-17.md section 6): the more / less / mute taps, queued on the phone and sent to the computer.
// The learning route (the scorecard) and the notebook page are shared code now (AutoradioLab/notebook.ts). The steer stays native only because the taps are made from the Swift cards and
// the Swift run sheet (AutoradioModule+Cards.swift `tell`, +Takes.swift `takeSteerMute`); the shared page asks how many wait through `notebookState`.
//   POST /api/autoradio/steer  {entry_id, direction}                          the design (section 6)
//        ASSUMED extra fields: `stories` (routes the tap refers to), `title`, and for a mute `handle`. A server that refuses the body (400)
//        gets it once more with only the design's fields, as the programme route's `ahead_titles` is handled.
//        `direction` is `more`, `less` or `mute` (design section 6 names more/less and `direction: mute`).

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
