import Foundation

// Where the desk's entries come from (slice B), behind one small type so that swapping the source is one file.
//
// Tonight the source is the programme route: the phone asks `POST /api/autoradio/programme` for `cause: open`, then `extend`, and maps each
// `segment` event to one entry placed at the end (`ProgrammeRouteFeed`). Coming next is the public master run sheet: one station written
// ahead on the server, a listener's sheet being the master's entries plus the listener's own ops. That will be another `RunSheetFeed`
// that yields the master's ops as they arrive; nothing outside this file needs to know.
//
// The feed yields OPS (placed by the desk, each with a reason), not entries: the same shape the master stream will carry, so the player
// and the store do not change when the feed does.

public struct RunSheetAsk: Sendable, Equatable {
    /// `open` (the first write of a listening: exactly one opening) or `extend` (more, when the time ahead is short).
    public var cause: String
    /// The titles of the entries ahead, so the desk avoids repeating them and writes its transition into them (at most 20).
    public var aheadTitles: [String]
    /// Stories already told or on the sheet: left out of an extend.
    public var excludeRoutes: Set<String>
    public init(cause: String, aheadTitles: [String] = [], excludeRoutes: Set<String> = []) {
        self.cause = cause; self.aheadTitles = aheadTitles; self.excludeRoutes = excludeRoutes
    }
}

public enum RunSheetFeedEvent: Sendable {
    /// Ops of the desk to apply, in order (each a `place` at the end, `by: desk`, with a reason).
    case ops([RunSheetOp])
    case done
    /// The desk said it could not finish. What arrived is kept.
    case error(String)
}

public protocol RunSheetFeed: Sendable {
    /// Throws before any event when there is nothing to be had (no route, no sign-in, no connection, no story to tell).
    func ask(_ a: RunSheetAsk) async throws -> AsyncThrowingStream<RunSheetFeedEvent, Error>
}

/// The programme route, as entries. Old kind names map as the interface says: `ident` is `opening`, `bridge` is `transition`;
/// `story`, `update` and `headlines` are `story`; `podcast_clip` is `clip`.
public struct ProgrammeRouteFeed: RunSheetFeed {
    let source: StretchSource
    let clips: ClipStore
    public init(source: StretchSource, clips: ClipStore = .shared) { self.source = source; self.clips = clips }

    public func ask(_ a: RunSheetAsk) async throws -> AsyncThrowingStream<RunSheetFeedEvent, Error> {
        let stream = try await source.events(cause: a.cause, excludingRoutes: a.excludeRoutes, onlyRoutes: nil, aheadTitles: Array(a.aheadTitles.prefix(20)))
        let clips = self.clips
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    for try await e in stream {
                        try Task.checkCancellation()
                        switch e {
                        case .segment(let s):
                            if let entry = Self.entry(from: s, clips: clips) { c.yield(.ops([Self.placeOp(entry, cause: a.cause)])) }
                        case .done: c.yield(.done)
                        case .error(let m): c.yield(.error(m))
                        }
                    }
                    c.finish()
                } catch is CancellationError { c.finish() }
                catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }

    /// `d-<segment id>`, kept to the characters an id may have and 64 bytes.
    static func entryID(_ segmentID: String) -> String {
        var out = "d-"
        for u in segmentID.unicodeScalars {
            let ok = (u.value >= 0x30 && u.value <= 0x39) || (u.value >= 0x41 && u.value <= 0x5a) || (u.value >= 0x61 && u.value <= 0x7a) || u == "_" || u == "." || u == "-"
            out.append(ok ? Character(u) : "-")
            if out.utf8.count >= 64 { break }
        }
        return out
    }

    static func placeOp(_ e: RunSheetEntry, cause: String) -> RunSheetOp {
        // The op id follows the entry id: a segment seen twice is one op, and the reducer drops the second.
        RunSheetOp(opId: String(("o" + e.id).prefix(64)), by: .desk, op: .place, at: .end, entry: e, reason: cause == "open" ? "written to open the listening" : "written to extend the programme")
    }

    /// The entry a segment becomes; nil when it cannot be one (a clip the phone does not have).
    static func entry(from s: IncomingSegment, clips: ClipStore) -> RunSheetEntry? {
        let text = StretchCandidates.cut(s.text.trimmingCharacters(in: .whitespacesAndNewlines), bytes: RunSheetBounds.maxTextBytes)
        guard !text.isEmpty else { return nil }
        let kind: RunSheetKind
        switch s.kind {
        case "opening", "ident": kind = .opening
        case "transition", "bridge": kind = .transition
        case "podcast_clip": kind = .clip
        case "answer": kind = .answer
        case "result": kind = .result
        default: kind = .story                            // story, update, headlines, mail, podcast_note
        }
        var cites: [RunSheetCite] = []
        for c in s.cites where !c.ref.isEmpty && !cites.contains(where: { $0.ref == c.ref }) {
            cites.append(RunSheetCite(ref: c.ref, role: RunSheetRole(rawValue: c.role) ?? .mentioned, label: SegmentPrep.label(c.label)))
        }
        // The listener's words: an opening is "Opening"; a transition is the station speaking between two entries and has no title of its own.
        let title = kind == .opening ? "Opening" : (kind == .transition ? "" : SegmentPrep.title(s.title, kind: s.kind))
        var e = RunSheetEntry(id: entryID(s.segmentID), kind: kind, placedBy: .desk, title: title, text: text, cites: cites.isEmpty ? nil : cites,
                              estSeconds: RunSheetEntry.estimateSeconds(words: text))
        if let p = cites.first(where: { $0.ref.hasPrefix("pub:") }) { e.source = RunSheetSource(ref: p.ref, label: p.label) }
        if let b = s.between { e.between = RunSheetBetween(after: entryID(b.after), before: entryID(b.before)) }
        if kind == .clip {
            guard let ref = s.clip, let c = clips.clip(ref.clipID) else { return nil }          // a clip the phone does not have is not played
            e.source = RunSheetSource(ref: "clip:" + c.clipID, label: c.show, url: c.audioURL.absoluteString, start: c.start, end: c.end, clipId: c.clipID)
            e.estSeconds += c.seconds
            if e.title.isEmpty || e.title == SegmentKinds.label(s.kind) { e.title = c.episodeTitle.isEmpty ? c.show : c.episodeTitle }
        }
        return e
    }
}

/// Asks the feed for entries and places them: `open` first, then `extend` whenever the time ahead falls below twice the last measured write
/// latency, with one request in flight. Nothing is ever swapped in: a feed that fails leaves the sheet as it is, and says so once.
@MainActor public final class RunSheetWriter {
    public let store: RunSheetStore
    /// The feed, resolved at each ask (the base URL can change with the sign-in).
    public var feed: () -> RunSheetFeed?
    /// Seconds from the last request to its first entry: the write latency the extend trigger reads.
    public private(set) var latency: TimeInterval = 0
    public private(set) var inFlight = false
    public private(set) var asks: [String] = []
    /// A quiet status line while the desk could not be reached or wrote nothing; nil when all is well. It is never an entry: the listener
    /// cannot skip to it, it is not in Up next, and nothing is spoken for it.
    public private(set) var notice: String?
    /// Told when `notice` changes (the host shows it in the player's status line).
    public var onNotice: ((String?) -> Void)?
    /// Told when a write starts and ends (the host shows "the desk is writing what comes next").
    public var onWriting: ((Bool) -> Void)?
    /// True when the last ask returned nothing new (a repeat, or nothing), and no ask is in flight: only then is "nothing is coming" true.
    public private(set) var dry = false
    public var onDry: ((Bool) -> Void)?
    /// Sees each desk op before it is applied (the player decides about a second opening); nil drops it.
    public var admit: ((RunSheetOp, String) -> RunSheetOp?)?
    public var noticeWords: ((Error?) -> String?) = { RunSheetWriter.plainWords($0) }
    public var clock: () -> Date = { Date() }
    private var task: Task<Void, Never>?
    private var quietOn: String?          // the entry that was on air when the last ask added nothing: not asked again until another one is

    public init(store: RunSheetStore, feed: @escaping () -> RunSheetFeed?) { self.store = store; self.feed = feed }

    public func settle() async { await task?.value }

    /// The routes of the stories the sheet already holds, from its entries' sources and cites.
    func toldRoutes() -> Set<String> {
        var out = Set<String>()
        for e in store.sheet.entries {
            for ref in (e.cites ?? []).map(\.ref) + [e.source?.ref ?? ""] where ref.hasPrefix("pub:") {
                out.insert(String(ref.dropFirst(4).split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first ?? ""))
            }
        }
        out.remove("")
        return out
    }

    /// Ask for the opening write. Called when the radio opens.
    public func open() { opened = true; ask(cause: "open") }

    /// The sheet changed or an entry started: is the time ahead short? Called by the player after every change.
    /// Writing ahead is the rule: ask when the spoken time ahead is under max(twice the measured write latency, `aheadFloorSeconds`), or at once
    /// when fewer than two playable entries (transitions do not count) are ahead. One ask in flight; an all-repeat answer waits for the next entry.
    public func checkAhead() {
        guard opened, !inFlight else { return }
        if let q = quietOn, q == store.onAir()?.id { return }
        let playable = store.ahead(limit: Int.max).filter { $0.kind != .transition }.count
        let estimate = latency > 0 ? latency : Self.unmeasuredLatencySeconds
        guard playable < 2 || store.aheadSeconds < max(2 * estimate, Self.aheadFloorSeconds) else { return }
        ask(cause: "extend")
    }
    /// The least spoken time kept written ahead of the listener.
    public static let aheadFloorSeconds: Double = 180
    /// The write latency assumed until one has been measured.
    public static let unmeasuredLatencySeconds: Double = 30
    private var opened = false

    private func ask(cause: String) {
        guard !inFlight, let feed = feed() else { return }
        inFlight = true
        onWriting?(true)
        asks.append(cause)
        let a = RunSheetAsk(cause: cause, aheadTitles: store.ahead(limit: 20).map(\.displayTitle), excludeRoutes: cause == "open" ? [] : toldRoutes())
        let started = clock()
        let airId = store.onAir()?.id
        task = Task { @MainActor in
            var added = 0, received = 0, first: Date?
            var failure: Error?
            do {
                let stream = try await feed.ask(a)
                for try await e in stream {
                    switch e {
                    case .ops(let ops):
                        if first == nil { first = self.clock() }
                        received += ops.count
                        for op in ops {
                            guard let admitted = self.admit?(op, cause) ?? (self.admit == nil ? op : nil) else { continue }
                            if self.store.applyFromDesk(admitted).applied { added += 1 }
                        }
                    case .done: break
                    case .error(let m): if added == 0 { failure = WriterError.said(m) }
                    }
                }
            } catch is CancellationError {
            } catch { failure = error }
            if let f = first { self.latency = max(0.5, f.timeIntervalSince(started)) }
            self.inFlight = false
            self.onWriting?(false)
            self.setDry(added == 0 && failure == nil)
            if added > 0 { self.quietOn = nil; self.setNotice(nil) }
            else {
                self.quietOn = airId
                // The desk answered but everything it wrote is already on the sheet (a repeat): that is not a failure.
                if received > 0 && failure == nil { self.setNotice(nil) }
                else if failure != nil || cause == "open" { self.setNotice(self.noticeWords(failure)) }
            }
            if added > 0 { self.checkAhead() }
        }
    }

    enum WriterError: Error { case said(String) }

    private func setDry(_ d: Bool) { guard d != dry else { return }; dry = d; onDry?(d) }
    private func setNotice(_ n: String?) {
        guard n != notice else { return }
        notice = n
        onNotice?(n)
    }

    /// Plain words for what went wrong, or nil when there is nothing to say (the front page has no story; a cancelled ask).
    public static func plainWords(_ e: Error?) -> String? {
        if e is CancellationError { return nil }
        if let c = e as? ChoirError {
            switch c {
            case .empty: return nil
            case .notSignedIn, .unauthorized: return "Sign in to hear the desk."
            case .transport, .timeout: return "The desk can't be reached right now."
            default: break
            }
        }
        return "The desk isn't answering right now."
    }
}
