import Foundation

// The run sheet (slice A of the Autoradio desk): one ordered list of everything the listener hears, and the one reducer that applies ops
// to it. The Go twin is internal/runsheet; both read the fixtures in mobile/fixtures/runsheet and must agree on every one. The wire
// format is JSON with snake_case keys. Unknown fields are ignored; an unknown kind, state, op or other vocabulary word is a decode
// error. The reducer holds mechanical invariants only; nothing in it interprets meaning.
//
// Interpretations of the frozen interface (the same list is in internal/runsheet/apply.go):
//  - duplicate detection needs memory, so RunSheet carries `applied_ops` (the last 512 applied op_ids); a dropped op is not remembered;
//  - dropped reasons: duplicate, not_ahead, not_on_air, not_phone, bad_transition, other_on_air, unknown_id, stale_version,
//    no_previous, duplicate_id, reason_required, invalid;
//  - the reducer reads no clock: ended.at is whatever the op carried ("" when nothing); skip and `now` take at and last_block from op.ended;
//  - skip, previous and aired never promote an entry to on_air: only an `aired` op from the phone does;
//  - previous returns the entry on air and the latest heard or skipped entry to ahead, in place;
//  - an entry that goes on air moves to the head of the ahead entries (order: tail, on_air, ahead);
//  - heard or skipped entries beyond the most recent 50 are trimmed; the ahead list is not capped here;
//  - place needs state "ahead", a fresh id and the text its kind requires;
//  - check order: op shape, duplicate, aired's authority, reason, then the op's own rules.

public enum RunSheetKind: String, Codable, Equatable, Sendable {
    case opening, transition, story, clip, episode, page, document, mail, result, answer
    /// Entries of these kinds carry words to speak.
    public var textRequired: Bool {
        switch self { case .opening, .transition, .story, .answer, .result: return true; default: return false }
    }
}
public enum RunSheetBy: String, Codable, Equatable, Sendable { case listener, desk, phone }
public enum RunSheetState: String, Codable, Equatable, Sendable {
    case ahead, onAir = "on_air", heard, skipped
    var isTail: Bool { self == .heard || self == .skipped }
}
public enum RunSheetOutcome: String, Codable, Equatable, Sendable { case finished, skipped, cut, left }
public enum RunSheetRole: String, Codable, Equatable, Sendable { case subject, quoted, mentioned }
public enum RunSheetOpName: String, Codable, Equatable, Sendable { case place, move, remove, skip, previous, clear, rewrite, aired }
public enum RunSheetAt: String, Codable, Equatable, Sendable { case now, next, queued, end, after }

/// How an entry stopped being on air. `at` is RFC3339, or "" when the placer supplied none.
public struct RunSheetEnded: Codable, Equatable, Sendable {
    public var outcome: RunSheetOutcome
    public var at: String
    public var lastBlock: Int
    public init(outcome: RunSheetOutcome, at: String = "", lastBlock: Int = 0) { self.outcome = outcome; self.at = at; self.lastBlock = lastBlock }
    enum CodingKeys: String, CodingKey { case outcome, at, lastBlock = "last_block" }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        outcome = try c.decode(RunSheetOutcome.self, forKey: .outcome)
        at = try c.decodeIfPresent(String.self, forKey: .at) ?? ""
        lastBlock = try c.decodeIfPresent(Int.self, forKey: .lastBlock) ?? 0
    }
}

public struct RunSheetSource: Codable, Equatable, Sendable {
    public var ref: String
    public var label: String
    public var url: String?
    public var start: Double?
    public var end: Double?
    public var clipId: String?
    public init(ref: String, label: String, url: String? = nil, start: Double? = nil, end: Double? = nil, clipId: String? = nil) {
        self.ref = ref; self.label = label; self.url = url; self.start = start; self.end = end; self.clipId = clipId
    }
    enum CodingKeys: String, CodingKey { case ref, label, url, start, end, clipId = "clip_id" }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        ref = try c.decodeIfPresent(String.self, forKey: .ref) ?? ""
        label = try c.decodeIfPresent(String.self, forKey: .label) ?? ""
        url = try c.decodeIfPresent(String.self, forKey: .url)
        start = try c.decodeIfPresent(Double.self, forKey: .start)
        end = try c.decodeIfPresent(Double.self, forKey: .end)
        clipId = try c.decodeIfPresent(String.self, forKey: .clipId)
    }
}

public struct RunSheetCite: Codable, Equatable, Sendable {
    public var ref: String
    public var role: RunSheetRole
    public var label: String
    public init(ref: String, role: RunSheetRole, label: String) { self.ref = ref; self.role = role; self.label = label }
}

public struct RunSheetBetween: Codable, Equatable, Sendable {
    public var after: String
    public var before: String
    public init(after: String, before: String) { self.after = after; self.before = before }
}

public struct RunSheetForecast: Codable, Equatable, Sendable {
    public var pHeard: Double
    public var causedBy: String
    public var claimId: String
    public init(pHeard: Double, causedBy: String, claimId: String) { self.pHeard = pHeard; self.causedBy = causedBy; self.claimId = claimId }
    enum CodingKeys: String, CodingKey { case pHeard = "p_heard", causedBy = "caused_by", claimId = "claim_id" }
}

/// One item on the run sheet.
public struct RunSheetEntry: Codable, Equatable, Sendable, Identifiable {
    public var id: String
    public var kind: RunSheetKind
    public var placedBy: RunSheetBy
    public var state: RunSheetState
    public var ended: RunSheetEnded?
    public var title: String
    public var text: String?
    public var source: RunSheetSource?
    public var cites: [RunSheetCite]?
    public var between: RunSheetBetween?
    public var estSeconds: Double
    public var forecast: RunSheetForecast?
    public init(id: String, kind: RunSheetKind, placedBy: RunSheetBy, state: RunSheetState = .ahead, ended: RunSheetEnded? = nil, title: String = "",
                text: String? = nil, source: RunSheetSource? = nil, cites: [RunSheetCite]? = nil, between: RunSheetBetween? = nil,
                estSeconds: Double = 0, forecast: RunSheetForecast? = nil) {
        self.id = id; self.kind = kind; self.placedBy = placedBy; self.state = state; self.ended = ended; self.title = title; self.text = text
        self.source = source; self.cites = cites; self.between = between; self.estSeconds = estSeconds; self.forecast = forecast
    }
    enum CodingKeys: String, CodingKey {
        case id, kind, placedBy = "placed_by", state, ended, title, text, source, cites, between, estSeconds = "est_seconds", forecast
    }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        kind = try c.decode(RunSheetKind.self, forKey: .kind)
        placedBy = try c.decode(RunSheetBy.self, forKey: .placedBy)
        state = try c.decode(RunSheetState.self, forKey: .state)
        ended = try c.decodeIfPresent(RunSheetEnded.self, forKey: .ended)
        title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
        let t = try c.decodeIfPresent(String.self, forKey: .text)
        text = (t?.isEmpty ?? true) ? nil : t    // an empty text is an absent text, as in Go
        source = try c.decodeIfPresent(RunSheetSource.self, forKey: .source)
        let cs = try c.decodeIfPresent([RunSheetCite].self, forKey: .cites)
        cites = (cs?.isEmpty ?? true) ? nil : cs
        between = try c.decodeIfPresent(RunSheetBetween.self, forKey: .between)
        estSeconds = try c.decodeIfPresent(Double.self, forKey: .estSeconds) ?? 0
        forecast = try c.decodeIfPresent(RunSheetForecast.self, forKey: .forecast)
    }
    public func encode(to e: Encoder) throws {
        var c = e.container(keyedBy: CodingKeys.self)
        try c.encode(id, forKey: .id); try c.encode(kind, forKey: .kind); try c.encode(placedBy, forKey: .placedBy)
        try c.encode(state, forKey: .state); try c.encodeIfPresent(ended, forKey: .ended); try c.encode(title, forKey: .title)
        try c.encodeIfPresent(text, forKey: .text); try c.encodeIfPresent(source, forKey: .source); try c.encodeIfPresent(cites, forKey: .cites)
        try c.encodeIfPresent(between, forKey: .between); try c.encode(estSeconds, forKey: .estSeconds); try c.encodeIfPresent(forecast, forKey: .forecast)
    }
}

/// The ordered list: heard or skipped tail (most recent 50), the entry on air, then the entries ahead.
public struct RunSheet: Codable, Equatable, Sendable {
    public var version: Int64
    public var onAir: String?
    public var positionSeconds: Double
    public var entries: [RunSheetEntry]
    /// The last 512 applied op_ids, oldest first: how a repeated op_id is recognised.
    public var appliedOps: [String]
    public init(version: Int64 = 0, onAir: String? = nil, positionSeconds: Double = 0, entries: [RunSheetEntry] = [], appliedOps: [String] = []) {
        self.version = version; self.onAir = onAir; self.positionSeconds = positionSeconds; self.entries = entries; self.appliedOps = appliedOps
    }
    enum CodingKeys: String, CodingKey { case version, onAir = "on_air", positionSeconds = "position_seconds", entries, appliedOps = "applied_ops" }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        version = try c.decodeIfPresent(Int64.self, forKey: .version) ?? 0
        let o = try c.decodeIfPresent(String.self, forKey: .onAir)
        onAir = (o?.isEmpty ?? true) ? nil : o
        positionSeconds = try c.decodeIfPresent(Double.self, forKey: .positionSeconds) ?? 0
        entries = try c.decodeIfPresent([RunSheetEntry].self, forKey: .entries) ?? []
        appliedOps = try c.decodeIfPresent([String].self, forKey: .appliedOps) ?? []
    }
    public func encode(to e: Encoder) throws {
        var c = e.container(keyedBy: CodingKeys.self)
        try c.encode(version, forKey: .version); try c.encodeIfPresent(onAir, forKey: .onAir)
        try c.encode(positionSeconds, forKey: .positionSeconds); try c.encode(entries, forKey: .entries)
        if !appliedOps.isEmpty { try c.encode(appliedOps, forKey: .appliedOps) }
    }

    // Views: Up next, Previously and now-playing read these.
    public var onAirEntry: RunSheetEntry? { entries.first { $0.state == .onAir } }
    public func ahead(limit: Int = Int.max) -> [RunSheetEntry] { Array(entries.filter { $0.state == .ahead }.prefix(limit)) }
    /// The most recent first.
    public func heard(limit: Int = Int.max) -> [RunSheetEntry] { Array(entries.filter { $0.state.isTail }.reversed().prefix(limit)) }
    public func entry(_ id: String) -> RunSheetEntry? { entries.first { $0.id == id } }
}

/// One change to a run sheet: the same vocabulary phone to desk and desk to phone.
public struct RunSheetOp: Codable, Equatable, Sendable {
    public var opId: String
    public var by: RunSheetBy
    public var op: RunSheetOpName
    public var at: RunSheetAt?
    public var after: String?
    public var id: String?
    public var entry: RunSheetEntry?
    public var text: String?
    public var state: RunSheetState?
    public var ended: RunSheetEnded?
    public var positionSeconds: Double?
    public var baseVersion: Int64
    public var reason: String?
    public init(opId: String, by: RunSheetBy, op: RunSheetOpName, at: RunSheetAt? = nil, after: String? = nil, id: String? = nil,
                entry: RunSheetEntry? = nil, text: String? = nil, state: RunSheetState? = nil, ended: RunSheetEnded? = nil,
                positionSeconds: Double? = nil, baseVersion: Int64 = 0, reason: String? = nil) {
        self.opId = opId; self.by = by; self.op = op; self.at = at; self.after = after; self.id = id; self.entry = entry; self.text = text
        self.state = state; self.ended = ended; self.positionSeconds = positionSeconds; self.baseVersion = baseVersion; self.reason = reason
    }
    enum CodingKeys: String, CodingKey {
        case opId = "op_id", by, op, at, after, id, entry, text, state, ended, positionSeconds = "position_seconds", baseVersion = "base_version", reason
    }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        opId = try c.decode(String.self, forKey: .opId)
        by = try c.decode(RunSheetBy.self, forKey: .by)
        op = try c.decode(RunSheetOpName.self, forKey: .op)
        at = try c.decodeIfPresent(RunSheetAt.self, forKey: .at)
        after = try c.decodeIfPresent(String.self, forKey: .after)
        id = try c.decodeIfPresent(String.self, forKey: .id)
        entry = try c.decodeIfPresent(RunSheetEntry.self, forKey: .entry)
        text = try c.decodeIfPresent(String.self, forKey: .text)
        state = try c.decodeIfPresent(RunSheetState.self, forKey: .state)
        ended = try c.decodeIfPresent(RunSheetEnded.self, forKey: .ended)
        positionSeconds = try c.decodeIfPresent(Double.self, forKey: .positionSeconds)
        baseVersion = try c.decodeIfPresent(Int64.self, forKey: .baseVersion) ?? 0
        reason = try c.decodeIfPresent(String.self, forKey: .reason)
    }
    public func encode(to e: Encoder) throws {
        var c = e.container(keyedBy: CodingKeys.self)
        try c.encode(opId, forKey: .opId); try c.encode(by, forKey: .by); try c.encode(op, forKey: .op)
        try c.encodeIfPresent(at, forKey: .at); try c.encodeIfPresent(after, forKey: .after); try c.encodeIfPresent(id, forKey: .id)
        try c.encodeIfPresent(entry, forKey: .entry); try c.encodeIfPresent(text, forKey: .text); try c.encodeIfPresent(state, forKey: .state)
        try c.encodeIfPresent(ended, forKey: .ended); try c.encodeIfPresent(positionSeconds, forKey: .positionSeconds)
        try c.encode(baseVersion, forKey: .baseVersion); try c.encodeIfPresent(reason, forKey: .reason)
    }
}

/// What the reducer says about one op. `dropped` is nil when the op applied; `version` is the sheet's version after the op.
public struct RunSheetResult: Codable, Equatable, Sendable {
    public var opId: String
    public var applied: Bool
    public var dropped: String?
    public var version: Int64
    public init(opId: String, applied: Bool, dropped: String? = nil, version: Int64) {
        self.opId = opId; self.applied = applied; self.dropped = dropped; self.version = version
    }
    enum CodingKeys: String, CodingKey { case opId = "op_id", applied, dropped, version }
}

/// The dropped reasons.
public enum RunSheetDropped {
    public static let duplicate = "duplicate", notAhead = "not_ahead", notOnAir = "not_on_air", notPhone = "not_phone"
    public static let badTransition = "bad_transition", otherOnAir = "other_on_air", unknownId = "unknown_id", staleVersion = "stale_version"
    public static let noPrevious = "no_previous", duplicateId = "duplicate_id", reasonRequired = "reason_required", invalid = "invalid"
}

public enum RunSheetBounds {
    public static let maxTitleBytes = 300, maxTextBytes = 8 * 1024, maxReasonBytes = 160, maxTail = 50, maxAppliedOps = 512
}

extension RunSheet {
    /// [A-Za-z0-9][A-Za-z0-9_.-]{0,63}
    public static func validId(_ s: String) -> Bool {
        let b = Array(s.utf8)
        if b.isEmpty || b.count > 64 { return false }
        for (i, c) in b.enumerated() {
            let alnum = (c >= 0x61 && c <= 0x7a) || (c >= 0x41 && c <= 0x5a) || (c >= 0x30 && c <= 0x39)
            if alnum || (i > 0 && (c == 0x5f || c == 0x2e || c == 0x2d)) { continue }
            return false
        }
        return true
    }

    /// Applies `op` and returns the new sheet and what happened. A dropped op returns the sheet unchanged.
    public func applying(_ op: RunSheetOp) -> (sheet: RunSheet, result: RunSheetResult) {
        func drop(_ why: String) -> (RunSheet, RunSheetResult) { (self, RunSheetResult(opId: op.opId, applied: false, dropped: why, version: version)) }
        if !Self.validId(op.opId) { return drop(RunSheetDropped.invalid) }
        if appliedOps.contains(op.opId) { return drop(RunSheetDropped.duplicate) }
        if op.op == .aired && op.by != .phone { return drop(RunSheetDropped.notPhone) }
        let reason = op.reason ?? ""
        if reason.utf8.count > RunSheetBounds.maxReasonBytes { return drop(RunSheetDropped.invalid) }
        if op.by == .desk && reason.isEmpty { return drop(RunSheetDropped.reasonRequired) }
        var out = self
        if let why = out.perform(op) { return drop(why) }
        out.trimTail()
        out.version = version + 1
        out.appliedOps.append(op.opId)
        if out.appliedOps.count > RunSheetBounds.maxAppliedOps { out.appliedOps.removeFirst(out.appliedOps.count - RunSheetBounds.maxAppliedOps) }
        return (out, RunSheetResult(opId: op.opId, applied: true, dropped: nil, version: out.version))
    }

    // MARK: the rules

    private var firstAhead: Int {
        if let oi = entries.firstIndex(where: { $0.state == .onAir }) { return oi + 1 }
        return entries.filter { $0.state.isTail }.count
    }

    private func slot(_ at: RunSheetAt, after: String?) -> Int {
        let first = firstAhead
        switch at {
        case .next, .now: return first
        case .end: return entries.count
        case .queued:
            var last = -1
            if first < entries.count { for i in first..<entries.count where entries[i].state == .ahead && entries[i].placedBy == .listener { last = i } }
            return last < 0 ? first : last + 1
        case .after:
            if let a = after, let i = entries.firstIndex(where: { $0.id == a }), entries[i].state == .ahead || entries[i].state == .onAir { return i + 1 }
            return first
        }
    }

    private static func endedFor(_ op: RunSheetOp, _ outcome: RunSheetOutcome) -> RunSheetEnded {
        RunSheetEnded(outcome: outcome, at: op.ended?.at ?? "", lastBlock: op.ended?.lastBlock ?? 0)
    }

    private mutating func putAhead(_ entry: RunSheetEntry, at: RunSheetAt, after: String?, op: RunSheetOp) {
        var e = entry
        if at != .now { entries.insert(e, at: slot(at, after: after)); return }
        if let oi = entries.firstIndex(where: { $0.state == .onAir }) {
            entries[oi].state = .skipped
            entries[oi].ended = Self.endedFor(op, .cut)
        }
        e.state = .onAir
        e.ended = nil
        entries.insert(e, at: firstAhead)
        onAir = e.id
        positionSeconds = 0
    }

    private static func validEntry(_ e: RunSheetEntry) -> Bool {
        if !validId(e.id) || e.title.utf8.count > RunSheetBounds.maxTitleBytes || (e.text?.utf8.count ?? 0) > RunSheetBounds.maxTextBytes { return false }
        if e.kind.textRequired && (e.text ?? "").isEmpty { return false }
        return true
    }

    /// Performs `op` on this private copy; returns the dropped reason, or nil when it applied.
    private mutating func perform(_ op: RunSheetOp) -> String? {
        switch op.op {
        case .place:
            guard let e = op.entry, Self.validEntry(e), e.state == .ahead, let at = op.at, !(at == .after && (op.after ?? "").isEmpty) else { return RunSheetDropped.invalid }
            if entries.contains(where: { $0.id == e.id }) { return RunSheetDropped.duplicateId }
            putAhead(e, at: at, after: op.after, op: op)
        case .move:
            guard let id = op.id, !id.isEmpty, let at = op.at, !(at == .after && (op.after ?? "").isEmpty) else { return RunSheetDropped.invalid }
            guard let i = entries.firstIndex(where: { $0.id == id }), entries[i].state == .ahead else { return RunSheetDropped.notAhead }
            let e = entries.remove(at: i)
            putAhead(e, at: at, after: op.after, op: op)
        case .remove:
            guard let id = op.id, !id.isEmpty else { return RunSheetDropped.invalid }
            guard let i = entries.firstIndex(where: { $0.id == id }), entries[i].state == .ahead else { return RunSheetDropped.notAhead }
            entries.remove(at: i)
        case .rewrite:
            guard let id = op.id, !id.isEmpty, let text = op.text, !text.isEmpty, text.utf8.count <= RunSheetBounds.maxTextBytes else { return RunSheetDropped.invalid }
            guard let i = entries.firstIndex(where: { $0.id == id }), entries[i].state == .ahead else { return RunSheetDropped.notAhead }
            entries[i].text = text
        case .skip:
            guard let id = op.id, !id.isEmpty else { return RunSheetDropped.invalid }
            guard let i = entries.firstIndex(where: { $0.id == id }), entries[i].state == .onAir else { return RunSheetDropped.notOnAir }
            entries[i].state = .skipped
            entries[i].ended = Self.endedFor(op, .skipped)
            onAir = nil
            positionSeconds = 0
        case .previous:
            if op.baseVersion != version { return RunSheetDropped.staleVersion }
            guard let last = entries.lastIndex(where: { $0.state.isTail }) else { return RunSheetDropped.noPrevious }
            for i in entries.indices where i == last || entries[i].state == .onAir {
                entries[i].state = .ahead
                entries[i].ended = nil
            }
            onAir = nil
            positionSeconds = 0
        case .clear:
            if op.baseVersion != version { return RunSheetDropped.staleVersion }
            entries.removeAll { $0.state == .ahead }
        case .aired:
            return performAired(op)
        }
        return nil
    }

    private mutating func performAired(_ op: RunSheetOp) -> String? {
        guard let id = op.id, !id.isEmpty, let to = op.state, to != .ahead else { return RunSheetDropped.invalid }
        guard let i = entries.firstIndex(where: { $0.id == id }) else { return RunSheetDropped.unknownId }
        let cur = entries[i].state
        if cur == .ahead && to == .onAir {
            if entries.contains(where: { $0.state == .onAir }) { return RunSheetDropped.otherOnAir }
            var e = entries.remove(at: i)
            e.state = .onAir
            e.ended = nil
            entries.insert(e, at: firstAhead)
            onAir = e.id
            positionSeconds = op.positionSeconds ?? 0
        } else if cur == .onAir && (to == .heard || to == .skipped) {
            entries[i].state = to
            entries[i].ended = op.ended ?? RunSheetEnded(outcome: to == .heard ? .finished : .skipped)
            onAir = nil
            positionSeconds = 0
        } else {
            return RunSheetDropped.badTransition
        }
        return nil
    }

    private mutating func trimTail() {
        var drop = entries.filter { $0.state.isTail }.count - RunSheetBounds.maxTail
        if drop <= 0 { return }
        entries.removeAll { e in
            if drop > 0 && e.state.isTail { drop -= 1; return true }
            return false
        }
    }
}

public enum RunSheetReducer {
    /// The reducer as a function: `Apply(sheet, op) -> (sheet, Result)`.
    public static func apply(_ sheet: RunSheet, _ op: RunSheetOp) -> (sheet: RunSheet, result: RunSheetResult) { sheet.applying(op) }

    public static func applyAll(_ sheet: RunSheet, _ ops: [RunSheetOp]) -> (sheet: RunSheet, results: [RunSheetResult]) {
        var s = sheet
        var rs: [RunSheetResult] = []
        for op in ops { let (n, r) = s.applying(op); s = n; rs.append(r) }
        return (s, rs)
    }
}
