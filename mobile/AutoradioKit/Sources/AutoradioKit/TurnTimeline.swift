import Foundation

// The timing trace of a turn, read from the event log (owner, 2026-11-11: "super duper slow to respond"; the next phone walk
// is read from a trace). Everything is derived from events, so the same function reads a live log and an exported one.
//
// WAYS THIS CAN FAIL (each has a test in LocalAckTests):
//  1. Events of one turn are attributed to the next turn: windows are cut at each `turnEnded`, and an event belongs to the
//     window that began at or before it.
//  2. A late event (the desk's audio starts after the turn record was folded) is missing: the timeline reads the LIVE log, not
//     the folded record.
//  3. A typed turn has no speech-ended event: the anchor is `turnEnded`, which both kinds have (a typed turn emits it before its
//     words, like a heard one); the speech-ended time is reported separately when there is one.
//  4. A step that did not happen (no local line, no desk answer) is reported as nil, never as 0.

public struct TurnTimeline: Codable, Sendable, Equatable {
    public var transcript: String
    /// Log time of `turnEnded` (the anchor: end of speech plus the recogniser's hangover; for a typed turn, the send).
    public var turnEndedAt: Double
    /// Log time the speech actually ended, when it was heard (nil for a typed turn).
    public var speechEndedAt: Double?
    /// The line the local model wrote, and when it was ready (ms after the anchor).
    public var localReplyText: String?
    public var localReplyReadyMs: Double?
    /// Why there was no line.
    public var localSkippedReason: String?
    /// First audio of the local reply (or of the non-verbal cue), handed to the player, ms after the anchor.
    public var localAudioStartMs: Double?
    public var cueUsed: Bool
    /// Each sentence of the phone's own stream, when it started to be heard (ms after the anchor).
    public var localSentences: [LocalSentence] = []
    /// Where the stream was cut, why, and when; and the hand-over sound.
    public var localCutMs: Double?
    public var localCutAfterSentences: Int?
    public var localCutReason: String?
    public var transitionMs: Double?
    /// The streamed desk's own clock (ms since it received the request) for its first sentence and its end, and its status.
    public var deskServerFirstSentenceMs: Double?
    public var deskServerDoneMs: Double?
    public var deskStatus: String?
    /// The responder's prompt for this turn: its size, how much was already cached, the time to read the rest, and whether the page was warm.
    public var promptTokens: Int?
    public var reusedTokens: Int?
    public var prefillMs: Double?
    public var cacheWarm: Bool?
    public struct LocalSentence: Codable, Sendable, Equatable { public var text: String; public var ms: Double }
    /// First piece of the desk's answer, and the first moment its audio began.
    public var deskFirstTokenMs: Double?
    public var deskAudioStartMs: Double?
    /// The 700 ms promise: first audio within this long of the end of speech.
    public static let localAudioBudgetMs = 700.0
    public var withinLocalBudget: Bool? { localAudioStartMs.map { $0 <= Self.localAudioBudgetMs } }

    public static func compute(_ events: [TimedEvent]) -> [TurnTimeline] {
        let anchors = events.indices.filter { if case .turnEnded = events[$0].event { return true }; return false }
        var out: [TurnTimeline] = []
        for (n, a) in anchors.enumerated() {
            let end = n + 1 < anchors.count ? anchors[n + 1] : events.count
            let t0 = events[a].t
            var tl = TurnTimeline(transcript: "", turnEndedAt: t0, speechEndedAt: nil, localReplyText: nil, localReplyReadyMs: nil, localSkippedReason: nil,
                                  localAudioStartMs: nil, cueUsed: false, deskFirstTokenMs: nil, deskAudioStartMs: nil)
            // The words come after the anchor for both kinds (a typed turn emits its turn end first).
            for i in a..<end { if case .finalTranscript(let t) = events[i].event { tl.transcript = t; break } }
            let lo = n > 0 ? anchors[n - 1] : 0
            for i in stride(from: a - 1, through: lo, by: -1) { if case .speechEnded = events[i].event { tl.speechEndedAt = events[i].t; break } }
            var revisionSeen = false
            for i in a..<end {
                let te = events[i]
                switch te.event {
                case .localAckReady(let text, let ms): if tl.localReplyReadyMs == nil { tl.localReplyReadyMs = ms; tl.localReplyText = text }
                case .localAckSkipped(let r, _): tl.localSkippedReason = r
                case .cueStarted: tl.cueUsed = true
                case .localSentenceStarted(let t, let ms): tl.localSentences.append(LocalSentence(text: t, ms: ms))
                case .localCut(let n, let r, let ms): tl.localCutMs = ms; tl.localCutAfterSentences = n; tl.localCutReason = r
                case .transitionPlayed(let ms): tl.transitionMs = ms
                case .deskSentenceStarted: if tl.deskAudioStartMs == nil { tl.deskAudioStartMs = (te.t - t0) * 1000 }
                case .deskDone(let st, _, let a, let b): tl.deskStatus = st; tl.deskServerFirstSentenceMs = a; tl.deskServerDoneMs = b
                case .localPrefill(let n, let pre, let re, let ms):
                    tl.promptTokens = n; tl.reusedTokens = re; tl.prefillMs = ms; tl.cacheWarm = pre > 0 && re >= pre
                case .ackStarted(let ms): if tl.localAudioStartMs == nil { tl.localAudioStartMs = ms }
                case .deskFirstToken(let ms): if tl.deskFirstTokenMs == nil { tl.deskFirstTokenMs = ms }
                case .revisionArrived: revisionSeen = true
                case .programmeAudioStart: if revisionSeen, tl.deskAudioStartMs == nil { tl.deskAudioStartMs = (te.t - t0) * 1000 }
                default: break
                }
            }
            out.append(tl)
        }
        return out
    }

    /// One line per step, for the Trace page and the exported file.
    public var lines: [String] {
        func f(_ x: Double?) -> String { x.map { String(format: "%.0f ms", $0) } ?? "none" }
        var out: [String] = []
        out.append("end of speech (turn end): t=" + String(format: "%.3f", turnEndedAt))
        out.append("local audio start: " + f(localAudioStartMs) + (cueUsed ? " (non-verbal cue)" : ""))
        if let n = promptTokens {
            out.append("responder prompt: \(n) tokens, \(reusedTokens ?? 0) already cached (" + (cacheWarm == true ? "WARM page cache" : "cold page cache") + "), read in " + f(prefillMs))
        }
        if localSentences.isEmpty {
            out.append("local sentences: none" + (localSkippedReason.map { " (" + $0 + ")" } ?? ""))
        }
        for (n, s) in localSentences.enumerated() { out.append("local sentence \(n + 1) heard: " + f(s.ms) + " \"" + s.text + "\"") }
        if let c = localCutMs { out.append("local stream cut: " + f(c) + " after \(localCutAfterSentences ?? 0) sentences (" + (localCutReason ?? "") + ")") }
        else { out.append("local stream cut: none (it ended by itself, or never ran)") }
        out.append("hand-over sound: " + f(transitionMs))
        out.append("desk first token: " + f(deskFirstTokenMs) + (deskServerFirstSentenceMs.map { " (the server's clock: first sentence " + f($0) + ", done " + f(deskServerDoneMs) + ")" } ?? ""))
        out.append("desk audio start: " + f(deskAudioStartMs))
        return out
    }
}

/// The file Settings → Developer → Trace exports: the last turns' timelines and their whole event log, as JSON.
public struct TraceExport: Codable, Sendable {
    public struct Turn: Codable, Sendable {
        public var id: String
        public var transcript: String
        public var configName: String
        public var outcome: String
        public var timeline: TurnTimeline?
        public var summary: TurnSummary
    }
    public struct Row: Codable, Sendable { public var ms: Double; public var text: String; public var event: LabEvent }
    public var exportedAt: Double
    public var device: DeviceInfo
    public var turns: [Turn]
    public var events: [Row]
    /// The voice configurator's full record (device probe, plan, every measurement, what was chosen): so a phone's setup can be read afterwards.
    public var configuration: ConfigurationRecord?

    public static func make(records: [TurnRecord], log: [TimedEvent], device: DeviceInfo, describe: (LabEvent) -> String, configuration: ConfigurationRecord? = nil, now: Double = Date().timeIntervalSince1970) -> TraceExport {
        let from = records.first?.startedAt ?? 0
        let live = log.filter { $0.t >= from - 0.001 }
        let timelines = TurnTimeline.compute(live)
        let t0 = live.first?.t ?? 0
        let turns = records.map { r -> Turn in
            // The record's own turn-end anchors the timeline that belongs to it.
            let anchor = r.events.first { if case .turnEnded = $0.event { return true }; return false }?.t
            let tl = timelines.first { anchor != nil && abs($0.turnEndedAt - anchor!) < 0.001 }
            return Turn(id: r.id, transcript: r.summary.transcript ?? "", configName: r.configuration.name, outcome: r.outcome.rawValue, timeline: tl, summary: r.summary)
        }
        return TraceExport(exportedAt: now, device: device, turns: turns,
                           events: live.map { Row(ms: ($0.t - t0) * 1000, text: describe($0.event), event: $0.event) }, configuration: configuration)
    }
    public func encoded() throws -> Data {
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]; return try e.encode(self)
    }
}
