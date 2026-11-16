import Foundation

// The phone's copy of the run sheet (slice B of the Autoradio desk): the ONE source of what plays next. It replaces PlayQueue, the
// stretch's segment list and the paper-list snapshot. Everything that changes what is heard is an op applied here by the one reducer
// (RunSheet.swift): skip, the next button, a swipe on the now-playing card and the lock screen's next are all `skip(on air)`; "play this"
// is `place now`; "queue this" is `place queued`. The deck of pages is not here: opening Mail or a book applies nothing.
//
// The phone plays from this copy without waiting for the computer. Each op the phone makes is also kept as pending, with its op_id, and
// posted to POST /api/autoradio/sheet/ops. A 404 ("not yet") or no network leaves it pending; pending ops survive a relaunch and are
// sent when the route answers (a replay of an op_id changes nothing on the server).
//
// Ways this can fail (written before the code; each has a test in RunSheetStoreTests):
//  1. A dropped op is posted or remembered as pending: only applied ops are.
//  2. The ahead list grows past 100: the phone bounds it. A listener's own placement takes the last entry's place; the desk's is refused.
//  3. A relaunch finds an entry still "on air" (the app died mid-entry): it is closed as `left`, and nothing resumes by itself.
//  4. The same op is applied twice (a retry): the reducer's op_id memory drops the second.
//  5. A post answers 404 or fails: the ops stay pending, in order, and are sent again; none is lost or sent twice after an answer.
//  6. A batch is over 64 ops or 256 KB: it is sent in several posts.
//  7. A transition whose neighbours changed is hidden or removed by the phone: it is shown stale (`isStale`), never changed by the phone.

public enum RunSheetPostOutcome: Sendable {
    /// The route answered 200: every op of the batch is now the server's (replayed op_ids answer the same).
    case answered([RunSheetResult])
    /// 404 or 501: the route is not there yet. The ops stay pending.
    case notYet
    /// No network, or the computer did not answer. The ops stay pending.
    case offline
}

public protocol RunSheetPoster: Sendable {
    func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome
}

/// The run sheet route on the listener's computer.
public struct RouteSheetPoster: RunSheetPoster {
    let baseURL: String
    public init(baseURL: String) { self.baseURL = baseURL }
    public func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome {
        struct Body: Encodable { var ops: [RunSheetOp] }
        struct Answer: Decodable { var results: [RunSheetResult]? }
        guard let data = try? JSONEncoder().encode(Body(ops: ops)) else { return .answered([]) }
        do {
            let r = try await ChoirEnvironment.shared.client(baseURL: baseURL).send("POST", "/api/autoradio/sheet/ops", body: data, route: "/api/autoradio/sheet/ops")
            return .answered((try? JSONDecoder().decode(Answer.self, from: r.data))?.results ?? [])
        } catch ChoirError.server(let status, _) where status == 404 || status == 501 || status == 503 { return .notYet }
        catch ChoirError.server(let status, _) where (400..<500).contains(status) { return .answered([]) }    // a body the server refuses will never be accepted: not retried
        catch { return .offline }
    }
}

@MainActor public final class RunSheetStore {
    public static let shared = RunSheetStore(storage: RunSheetStore.applicationSupportFile())

    public static let maxAhead = 100
    public static let maxBatchOps = 64
    public static let maxBatchBytes = 256 * 1024

    public private(set) var sheet = RunSheet()
    /// Ops the phone applied that the server has not answered yet, oldest first.
    public private(set) var pending: [RunSheetOp] = []
    public var onChange: (() -> Void)?
    public var poster: RunSheetPoster?
    /// The clock for `ended.at`. The reducer reads none.
    public var now: () -> Date = { Date() }
    /// How long to wait before sending pending ops again after the route did not take them.
    public var retryAfter: TimeInterval = 30

    private let storage: URL?
    private var flushing = false
    private var retryTask: Task<Void, Never>?
    /// Tests wait on this.
    public private(set) var flushTask: Task<Void, Never>?

    public init(storage: URL? = nil) { self.storage = storage }

    public static func applicationSupportFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("runsheet.json")
    }

    // MARK: Views

    public func onAir() -> RunSheetEntry? { sheet.onAirEntry }
    public func ahead(limit: Int) -> [RunSheetEntry] { sheet.ahead(limit: limit) }
    public func heard(limit: Int) -> [RunSheetEntry] { sheet.heard(limit: limit) }
    public func entry(_ id: String) -> RunSheetEntry? { sheet.entry(id) }
    /// The seconds the entries ahead will take, by their estimates.
    public var aheadSeconds: Double { sheet.ahead().reduce(0) { $0 + $1.estSeconds } }

    /// A transition was written for two neighbours. It is stale when the entry before it or the entry after it is no longer the one it
    /// was written for. The phone only shows that; the desk's next write may rewrite it.
    public func isStale(_ e: RunSheetEntry) -> Bool {
        guard e.kind == .transition, e.state == .ahead, let b = e.between, let i = sheet.entries.firstIndex(where: { $0.id == e.id }) else { return false }
        let before = i > 0 ? sheet.entries[i - 1].id : ""
        let after = i + 1 < sheet.entries.count ? sheet.entries[i + 1].id : ""
        return before != b.after || after != b.before
    }

    // MARK: Applying

    /// The one way the phone changes the sheet. An applied op is kept pending and posted.
    @discardableResult
    public func apply(_ op: RunSheetOp) -> RunSheetResult { run(op, remote: false) }

    /// An op that came from the desk's side (the feed, later the master stream): applied here, never posted back.
    @discardableResult
    public func applyFromDesk(_ op: RunSheetOp) -> RunSheetResult { run(op, remote: true) }

    private func run(_ op: RunSheetOp, remote: Bool) -> RunSheetResult {
        var op = op
        if op.op == .place, let e = op.entry, e.state == .ahead, op.at != .now, sheet.ahead().count >= Self.maxAhead {
            // The reducer does not cap the ahead list; the phone does. A listener's own placement is never refused: it takes the place
            // of the last entry. The desk's is refused.
            guard op.by == .listener, let last = sheet.ahead().last else {
                return RunSheetResult(opId: op.opId, applied: false, dropped: "ahead_full", version: sheet.version)
            }
            _ = run(RunSheetOp(opId: mintOpId(), by: .phone, op: .remove, id: last.id), remote: false)
        }
        if op.op == .previous || op.op == .clear, !remote { op.baseVersion = sheet.version }
        let (next, result) = sheet.applying(op)
        guard result.applied else { return result }
        sheet = next
        if !remote { pending.append(op) }
        save()
        onChange?()
        if !remote { flush() }
        return result
    }

    public func mintOpId() -> String { RunSheetEntry.mintId("p-") }

    private func stamp() -> String {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime]
        return f.string(from: now())
    }

    // MARK: The ops the app makes (each is one op with a fresh op_id)

    @discardableResult
    public func place(_ e: RunSheetEntry, at: RunSheetAt, after: String? = nil, by: RunSheetBy? = nil, reason: String? = nil) -> RunSheetResult {
        let by = by ?? e.placedBy
        return apply(RunSheetOp(opId: mintOpId(), by: by, op: .place, at: at, after: after, entry: e, reason: by == .desk ? (reason ?? "the desk wrote it") : reason))
    }
    @discardableResult
    public func remove(id: String) -> RunSheetResult { apply(RunSheetOp(opId: mintOpId(), by: .listener, op: .remove, id: id)) }
    @discardableResult
    public func move(id: String, at: RunSheetAt, after: String? = nil) -> RunSheetResult { apply(RunSheetOp(opId: mintOpId(), by: .listener, op: .move, at: at, after: after, id: id)) }
    /// Skip, the next button, a swipe on the now-playing card, the lock screen's next and "skip" by voice: all this one op. Nil when nothing is on air.
    @discardableResult
    public func skipOnAir(lastBlock: Int = 0, by: RunSheetBy = .listener) -> RunSheetResult? {
        guard let e = onAir() else { return nil }
        return apply(RunSheetOp(opId: mintOpId(), by: by, op: .skip, id: e.id, ended: RunSheetEnded(outcome: .skipped, at: stamp(), lastBlock: lastBlock)))
    }
    @discardableResult
    public func previous(by: RunSheetBy = .listener) -> RunSheetResult { apply(RunSheetOp(opId: mintOpId(), by: by, op: .previous, baseVersion: sheet.version)) }
    @discardableResult
    public func clearAhead(by: RunSheetBy = .listener) -> RunSheetResult { apply(RunSheetOp(opId: mintOpId(), by: by, op: .clear, baseVersion: sheet.version)) }
    /// The phone reports that an entry is now on air.
    @discardableResult
    public func airedOnAir(_ id: String, positionSeconds: Double = 0) -> RunSheetResult {
        apply(RunSheetOp(opId: mintOpId(), by: .phone, op: .aired, id: id, state: .onAir, positionSeconds: positionSeconds))
    }
    /// The phone reports how the entry on air ended: heard (it finished, or was left or cut) or skipped.
    @discardableResult
    public func airedEnded(_ id: String, as state: RunSheetState, outcome: RunSheetOutcome, lastBlock: Int = 0) -> RunSheetResult {
        apply(RunSheetOp(opId: mintOpId(), by: .phone, op: .aired, id: id, state: state, ended: RunSheetEnded(outcome: outcome, at: stamp(), lastBlock: lastBlock)))
    }

    // MARK: Posting

    /// Send what is pending. One post at a time; in order; the batch is at most 64 ops and 256 KB.
    public func flush() {
        guard !flushing, poster != nil, !pending.isEmpty else { return }
        flushing = true
        flushTask = Task { @MainActor in
            defer { self.flushing = false }
            while let poster = self.poster, !self.pending.isEmpty {
                let batch = self.nextBatch()
                switch await poster.post(ops: batch) {
                case .answered:
                    let ids = Set(batch.map(\.opId))
                    self.pending.removeAll { ids.contains($0.opId) }
                    self.save()
                case .notYet, .offline:
                    self.scheduleRetry()
                    return
                }
            }
        }
    }

    private func nextBatch() -> [RunSheetOp] {
        var out: [RunSheetOp] = [], bytes = 0
        let enc = JSONEncoder()
        for op in pending {
            let n = (try? enc.encode(op).count) ?? 0
            if !out.isEmpty, out.count >= Self.maxBatchOps || bytes + n > Self.maxBatchBytes { break }
            out.append(op); bytes += n
        }
        return out
    }

    private func scheduleRetry() {
        retryTask?.cancel()
        let wait = retryAfter
        retryTask = Task { @MainActor [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(max(0, wait) * 1_000_000_000))
            guard !Task.isCancelled else { return }
            self?.flush()
        }
    }

    // MARK: Saving

    struct Saved: Codable { var sheet: RunSheet; var pending: [RunSheetOp] }

    public func save() {
        guard let storage else { return }
        if let d = try? JSONEncoder().encode(Saved(sheet: sheet, pending: pending)) { try? d.write(to: storage, options: .atomic) }
    }

    /// The sheet and the pending ops from the last launch. Nothing plays. An entry that was on air when the app died is closed as `left`
    /// (a crash restart never resumes work).
    public func restore() {
        guard let storage, let d = try? Data(contentsOf: storage), let s = try? JSONDecoder().decode(Saved.self, from: d) else { return }
        sheet = s.sheet; pending = s.pending
        if let e = sheet.onAirEntry { airedEnded(e.id, as: .heard, outcome: .left) }
        else { save() }
        onChange?()
        flush()
    }
}
