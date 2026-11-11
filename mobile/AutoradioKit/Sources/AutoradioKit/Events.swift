import Foundation

// MARK: Clock

/// The host clock. Events are stamped from it. Replay drives a virtual one so stamps follow the audio.
public final class LabClock: @unchecked Sendable {
    private let lock = NSLock()
    private var virtual: Double?
    private var wallAtSet = 0.0
    public init() {}
    /// Host time, or virtual time plus the wall time elapsed since it was last set (so compute shows in the stamps).
    public var now: Double {
        lock.lock(); defer { lock.unlock() }
        let w = ProcessInfo.processInfo.systemUptime
        if let v = virtual { return v + (w - wallAtSet) }
        return w
    }
    /// Switch to virtual time (seconds). Only moves forward.
    public func setVirtual(_ t: Double) {
        let cur = now
        lock.lock(); defer { lock.unlock() }
        virtual = Swift.max(cur, t); wallAtSet = ProcessInfo.processInfo.systemUptime
    }
    public var isVirtual: Bool { lock.lock(); defer { lock.unlock() }; return virtual != nil }
}

// MARK: Events

public enum YieldKind: String, Codable, Sendable { case dip, pause, resume }
public enum PlaybackStateName: String, Codable, Sendable { case playing, dipped, paused, stopped }

/// Everything a stage concludes. Audio frames are not events.
public enum LabEvent: Codable, Sendable, Equatable {
    case phase(from: String, to: String)
    case speechStarted
    case speechEnded(seconds: Double)
    case addressed(by: String)
    case notATurn(reason: String)
    case partialTranscript(String)
    case finalTranscript(String)
    case turnEnded
    case system1Answer(questionID: String, option: String, score: Double, ms: Double)
    case ackStarted(msSinceTurnEnd: Double)
    case holdingSaid(String)
    case revisionArrived(id: String, fromBlock: Int, blocks: Int, summary: String)
    case playbackState(PlaybackStateName)
    case yielded(YieldKind)
    case stageTiming(stage: String, startedAt: Double, endedAt: Double)
    case modelLoaded(id: String, ms: Double, residentBytes: Int64)
    case thermal(String)
    case audioSession(category: String, mode: String, note: String)
    /// A claim moved the session between states (or made the first claim): `from` is "none" for the first.
    case audioSwitch(from: String, to: String, ms: Double)
    /// The system told the claim something: interrupted, resumed, reset, route (detail: the route now).
    case audioNotice(kind: String, detail: String)
    case configurationChanged(String)
    case note(String)
    case error(stage: String, message: String)
}

public struct TimedEvent: Codable, Sendable, Equatable {
    public var t: Double
    public var event: LabEvent
    public init(t: Double, event: LabEvent) { self.t = t; self.event = event }
}

/// The spine. Multi-subscriber, append-only, stamped from the clock.
public final class EventLog: @unchecked Sendable {
    private let lock = NSLock()
    private var all: [TimedEvent] = []
    private var subs: [UUID: AsyncStream<TimedEvent>.Continuation] = [:]
    public let clock: LabClock
    public init(clock: LabClock) { self.clock = clock }

    @discardableResult
    public func emit(_ e: LabEvent, at t: Double? = nil) -> TimedEvent {
        let te = TimedEvent(t: t ?? clock.now, event: e)
        lock.lock(); all.append(te); let cs = Array(subs.values); lock.unlock()
        for c in cs { c.yield(te) }
        return te
    }
    public var events: [TimedEvent] { lock.lock(); defer { lock.unlock() }; return all }
    public func events(since index: Int) -> [TimedEvent] { let e = events; return index < e.count ? Array(e[index...]) : [] }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return all.count }

    public func stream() -> AsyncStream<TimedEvent> {
        AsyncStream { cont in
            let id = UUID()
            lock.lock(); subs[id] = cont; lock.unlock()
            cont.onTermination = { [weak self] _ in self?.lock.lock(); self?.subs[id] = nil; self?.lock.unlock() }
        }
    }

    /// Time a piece of work and emit stageTiming. In virtual time the end stamp is start + wall elapsed.
    public func timed<T>(_ stage: String, _ body: () async throws -> T) async rethrows -> T {
        let start = clock.now
        defer { emit(.stageTiming(stage: stage, startedAt: start, endedAt: clock.now)) }
        return try await body()
    }
}

// MARK: Turn record

public enum Outcome: String, Codable, Sendable {
    case heardThrough, spokeAgain, skipped, corrected, unknown
}

public struct DeviceInfo: Codable, Sendable, Equatable {
    public var model: String, os: String, appBuild: String
    public init(model: String, os: String, appBuild: String) { self.model = model; self.os = os; self.appBuild = appBuild }
    public static var current: DeviceInfo {
        var size = 0
        sysctlbyname("hw.machine", nil, &size, nil, 0)
        var m = [CChar](repeating: 0, count: max(size, 1))
        sysctlbyname("hw.machine", &m, &size, nil, 0)
        let build = (Bundle.main.infoDictionary?["CFBundleVersion"] as? String) ?? "dev"
        return DeviceInfo(model: String(cString: m), os: ProcessInfo.processInfo.operatingSystemVersionString, appBuild: build)
    }
}

public struct StageTimingEntry: Codable, Sendable, Equatable {
    public var stage: String; public var startedAt: Double; public var endedAt: Double
    public var ms: Double { (endedAt - startedAt) * 1000 }
}

public struct AnswerEntry: Codable, Sendable, Equatable {
    public var questionID: String; public var option: String; public var score: Double; public var ms: Double
}

/// What a turn amounted to: derived entirely from the events, so it can always be rebuilt.
public struct TurnSummary: Codable, Sendable, Equatable {
    public var transcript: String?
    public var addressedBy: String?
    public var notATurn: String?
    public var speechSeconds: Double?
    public var timings: [StageTimingEntry]
    public var answers: [AnswerEntry]
    public var holdingLines: [String]
    public var revisions: [String]
    /// Milliseconds from the end of the turn to the first audio of the acknowledgement.
    public var turnEndToAckMs: Double?
    /// Milliseconds from the end of the turn to the first revision.
    public var turnEndToRevisionMs: Double?
    public var errors: [String]
    public var peakResidentBytes: Int64?
    /// Milliseconds the audio session took to change state during the turn (the sum of its switches).
    public var audioSwitchMs: Double?
}

public struct TurnRecord: Codable, Sendable {
    public var id: String
    public var startedAt: Double
    public var configuration: LabConfiguration
    public var device: DeviceInfo
    public var events: [TimedEvent]
    public var summary: TurnSummary
    public var outcome: Outcome

    /// Fold the events of one turn into a record.
    public static func fold(id: String, configuration: LabConfiguration, device: DeviceInfo,
                            events: [TimedEvent], outcome: Outcome = .unknown) -> TurnRecord {
        var s = TurnSummary(timings: [], answers: [], holdingLines: [], revisions: [], errors: [])
        var turnEnd: Double?
        var peak: Int64 = 0
        for te in events {
            switch te.event {
            case .finalTranscript(let t): s.transcript = t
            case .addressed(let by): s.addressedBy = by
            case .notATurn(let r): s.notATurn = r
            case .speechEnded(let sec): s.speechSeconds = sec
            case .turnEnded: turnEnd = te.t
            case .stageTiming(let st, let a, let b): s.timings.append(StageTimingEntry(stage: st, startedAt: a, endedAt: b))
            case .system1Answer(let q, let o, let sc, let ms): s.answers.append(AnswerEntry(questionID: q, option: o, score: sc, ms: ms))
            case .ackStarted(let ms): s.turnEndToAckMs = ms
            case .holdingSaid(let l): s.holdingLines.append(l)
            case .revisionArrived(let id, _, _, let sum):
                s.revisions.append("\(id): \(sum)")
                if s.turnEndToRevisionMs == nil, let e = turnEnd { s.turnEndToRevisionMs = (te.t - e) * 1000 }
            case .audioSwitch(let from, _, let ms) where from != "none": s.audioSwitchMs = (s.audioSwitchMs ?? 0) + ms
            case .modelLoaded(_, _, let b): peak = max(peak, b)
            case .error(let st, let m): s.errors.append("\(st): \(m)")
            default: break
            }
        }
        s.peakResidentBytes = peak > 0 ? peak : nil
        return TurnRecord(id: id, startedAt: events.first?.t ?? 0, configuration: configuration, device: device,
                          events: events, summary: s, outcome: outcome)
    }

    public static func decode(_ data: Data) throws -> TurnRecord { try JSONDecoder().decode(TurnRecord.self, from: data) }
    public func encoded() throws -> Data {
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]; return try e.encode(self)
    }
}
