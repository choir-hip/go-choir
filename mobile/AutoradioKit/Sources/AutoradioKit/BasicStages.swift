import Foundation

// Baseline implementations that need no model: addressing, bounds, yield, and the lab stand-ins the Mac replay proof configs name (fixed, stub-echo, fixture).

// MARK: MicSource

public final class BuiltInMic: MicSource, @unchecked Sendable {
    public static let stageID = "builtin"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "Phone microphone", summary: "The phone's own microphone, even when a headset is connected.") }
    public var inputPreference: InputPreference { .builtIn }
    public init() {}
}
public final class RouteMic: MicSource, @unchecked Sendable {
    public static let stageID = "route"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "Current route", summary: "Whatever input the system has selected (headset, Bluetooth).") }
    public var inputPreference: InputPreference { .currentRoute }
    public init() {}
}

// MARK: Addressing

/// Button logic shared by the three addressing stages: a press to start talking, then VAD or a second press to end.
struct ButtonLogic {
    var active = false
    var holdSeconds: Double
    var holding = false
    mutating func decide(_ i: AddressInput) -> AddressDecision? {
        switch i {
        case .buttonDown:
            if active { active = false; holding = false; return .endTurn }  // second tap ends a tap-to-talk turn
            active = true; holding = true
            return .addressed(by: "button")
        case .buttonUp(let held):
            holding = false
            if held >= holdSeconds { active = false; return .endTurn }
            return AddressDecision.none
        case .turnFinished: active = false; holding = false; return AddressDecision.none
        default: return nil
        }
    }
}

public final class ButtonAddressing: Addressing, @unchecked Sendable {
    public static let stageID = "button"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Button", summary: "Tap to talk, and tap again (or pause) to end. Speech without the button is ignored.",
                        params: [.number("holdSeconds", "Hold counts after (s)", 0.6, 0.1, 3),
                                 .number("noSpeechTimeout", "No-speech timeout (s)", 6, 1, 30)])
    }
    var logic: ButtonLogic
    public let noSpeechTimeout: Double
    public init(_ p: StageParams) { logic = ButtonLogic(holdSeconds: p.double("holdSeconds", 0.6)); noSpeechTimeout = p.double("noSpeechTimeout", 6) }
    public var holdsTurnOpen: Bool { logic.holding }
    public func decide(_ input: AddressInput, now: Double) async -> AddressDecision { logic.decide(input) ?? .none }
}

public final class OpenAddressing: Addressing, @unchecked Sendable {
    public static let stageID = "open"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Open", summary: "Any speech that passes the bounds and has enough words counts. The button still works.",
                        params: [.number("minWords", "Minimum words", 2, 1, 10),
                                 .number("holdSeconds", "Button hold counts after (s)", 0.6, 0.1, 3),
                                 .number("noSpeechTimeout", "No-speech timeout (s)", 6, 1, 30)])
    }
    var logic: ButtonLogic
    let minWords: Int
    public let noSpeechTimeout: Double
    public init(_ p: StageParams) { logic = ButtonLogic(holdSeconds: p.double("holdSeconds", 0.6)); minWords = p.int("minWords", 2); noSpeechTimeout = p.double("noSpeechTimeout", 6) }
    public var holdsTurnOpen: Bool { logic.holding }
    public func decide(_ input: AddressInput, now: Double) async -> AddressDecision {
        if let d = logic.decide(input) { return d }
        switch input {
        case .speechStarted: return .candidate
        case .finalTranscript(let t, _): return wordCount(t) >= minWords ? .addressed(by: "open") : .none
        default: return .none
        }
    }
}

public final class HybridAddressing: Addressing, @unchecked Sendable {
    public static let stageID = "hybrid"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Hybrid window", summary: "Button to start; then open for a window after each addressed turn; then closed again.",
                        params: [.number("windowSeconds", "Window (s)", 300, 5, 1800),
                                 .number("minWords", "Minimum words (in window)", 2, 1, 10),
                                 .number("holdSeconds", "Button hold counts after (s)", 0.6, 0.1, 3),
                                 .number("noSpeechTimeout", "No-speech timeout (s)", 6, 1, 30)])
    }
    var logic: ButtonLogic
    let window: Double, minWords: Int
    var windowEnd: Double = 0
    public let noSpeechTimeout: Double
    public init(_ p: StageParams) {
        logic = ButtonLogic(holdSeconds: p.double("holdSeconds", 0.6)); window = p.double("windowSeconds", 300)
        minWords = p.int("minWords", 2); noSpeechTimeout = p.double("noSpeechTimeout", 6)
    }
    public var holdsTurnOpen: Bool { logic.holding }
    public func decide(_ input: AddressInput, now: Double) async -> AddressDecision {
        if case .turnFinished = input { windowEnd = now + window }   // an addressed turn (re)opens the window
        if let d = logic.decide(input) { return d }
        guard now < windowEnd else { return .none }
        switch input {
        case .speechStarted: return .candidate
        case .finalTranscript(let t, _): return wordCount(t) >= minWords ? .addressed(by: "hybrid-window") : .none
        default: return .none
        }
    }
}

func wordCount(_ s: String) -> Int { s.split { !$0.isLetter && !$0.isNumber && $0 != "'" }.count }

// MARK: Speech bounds (baseline)

/// BASELINE: an energy detector with an adaptive noise floor and a hangover. Replaced by Silero / Smart Turn later.
public final class EnergyVAD: SpeechBounds, @unchecked Sendable {
    public static let stageID = "energy-vad"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Energy VAD (baseline)", summary: "Baseline only: loudness over an adaptive noise floor, with a hangover before the end.",
                        params: [.number("minDb", "Absolute floor (dBFS)", -42, -70, -10),
                                 .number("marginDb", "Margin over noise (dB)", 12, 3, 30),
                                 .number("hangoverSeconds", "End after silence (s)", 0.8, 0.2, 5),
                                 .number("minStartSeconds", "Voiced to start (s)", 0.1, 0.02, 0.5)])
    }
    let minDb, marginDb, hangover, minStart: Double
    var window: [Double] = []   // last ~1.5 s of frame levels; their minimum is the noise floor
    var inSpeech = false
    var startT = 0.0, lastVoicedEnd = 0.0, voicedRun = 0.0, runStart = 0.0
    public init(_ p: StageParams) {
        minDb = p.double("minDb", -42); marginDb = p.double("marginDb", 12)
        hangover = p.double("hangoverSeconds", 0.8); minStart = p.double("minStartSeconds", 0.1)
    }
    public func reset() async { inSpeech = false; voicedRun = 0; window = [] }
    public func speechSeconds(now: Double) async -> Double { inSpeech ? max(0, lastVoicedEnd - startT) : 0 }
    public func feed(_ f: AudioFrame) async -> BoundsUpdate {
        let db = f.rmsDb
        window.append(db); if window.count > 47 { window.removeFirst() }
        let thr = max(minDb, (window.min() ?? db) + marginDb)
        let voiced = db > thr
        let end = f.t + f.duration
        if !inSpeech {
            if voiced {
                if voicedRun == 0 { runStart = f.t }
                voicedRun += f.duration
                if voicedRun >= minStart { inSpeech = true; startT = runStart; lastVoicedEnd = end; return .speechStarted(at: runStart) }
            } else { voicedRun = 0 }
            return .none
        }
        if voiced { lastVoicedEnd = end; return .none }
        if end - lastVoicedEnd >= hangover { inSpeech = false; voicedRun = 0; return .speechEnded(at: end, seconds: lastVoicedEnd - startT) }
        return .none
    }
}

public final class NoSpeakerCheck: SpeakerCheck, @unchecked Sendable {
    public static let stageID = "none"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "None", summary: "Every voice counts.") }
    public init() {}
    public func isListener(_ frames: [AudioFrame]) async -> Bool { true }
}

// MARK: Yield policies

public final class DipThenPause: YieldPolicy, @unchecked Sendable {
    public static let stageID = "dip-then-pause"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Dip, then pause", summary: "Lower playback after dipAfter s of speech; pause when addressed or after pauseAfter s; return if it was not a turn.",
                        params: [.number("dipAfter", "Dip after (s)", 1.0, 0.2, 5), .number("pauseAfter", "Pause after (s)", 10, 2, 60)])
    }
    let dipAfter, pauseAfter: Double
    public init(_ p: StageParams) { dipAfter = p.double("dipAfter", 1.0); pauseAfter = p.double("pauseAfter", 10) }
    public func target(for s: YieldSituation) -> PlayLevel {
        if s.notATurn { return .play }
        if s.addressed || s.speechSeconds >= pauseAfter { return .pause }
        if s.current == .pause { return .pause }
        return s.speechSeconds >= dipAfter ? .dip : .play
    }
}
public final class PauseAtOnce: YieldPolicy, @unchecked Sendable {
    public static let stageID = "pause-at-once"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "Pause at once", summary: "Pause as soon as speech starts; return if it was not a turn.") }
    public init() {}
    public func target(for s: YieldSituation) -> PlayLevel {
        if s.notATurn { return .play }
        return (s.addressed || s.speechSeconds > 0 || s.current == .pause) ? .pause : .play
    }
}

// MARK: System 1

public final class FixedSystem1: System1, @unchecked Sendable {
    public static let stageID = "fixed"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "Fixed (baseline)", summary: "Always chooses each question's first option.") }
    public init() {}
    public func answer(_ i: System1Input) async throws -> [System1Answer] {
        i.questions.map { System1Answer(questionID: $0.id, option: $0.options.first ?? "", score: 1, ms: 0) }
    }
}

// MARK: Holder

// The holder is LocalAckHolder (LocalAck.swift): a line written by the on-device model, never a stock phrase.

// MARK: Desk

/// Clearly labelled stand-in so the loop runs before the server lane: a revision after a delay.
public final class StubEchoDesk: Desk, @unchecked Sendable {
    public static let stageID = "stub-echo"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Stub echo (no server)", summary: "Not the real desk. Answers after a delay by echoing the listener's words.",
                        params: [.number("delaySeconds", "Delay (s)", 2, 0, 120)])
    }
    let delay: Double
    public init(_ p: StageParams) { delay = p.double("delaySeconds", 2) }
    public func submit(_ t: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        let d = delay
        return AsyncThrowingStream { c in
            let task = Task {
                try? await Task.sleep(nanoseconds: UInt64(d * 1e9))
                if Task.isCancelled { c.finish(); return }
                c.yield(Revision(id: "stub-\(t.turnID)", fromBlock: t.blockIndex + 1,
                                 blocks: [.speech(text: "This is the stub desk. You said: \(t.listenerWords)")]))
                c.finish()
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
}

// MARK: Programme

public final class FixtureProgramme: Programme, @unchecked Sendable {
    public static let stageID = "fixture"
    public static var descriptor: StageDescriptor { StageDescriptor(displayName: "Fixture edition", summary: "Eight spoken blocks and one clip; the same every run.") }
    public init() {}
    public func load() async throws -> ProgrammeDocument { Self.document }
    public static let document = ProgrammeDocument(id: "fixture-1", title: "Fixture edition", blocks: [
        .speech(text: "Good morning. This is the fixture edition, written only to test the radio."),
        .speech(text: "The first story is about a harbour that moved its cranes inland to make room for a park."),
        .speech(text: "Residents said the quiet was strange at first, and then they stopped noticing it."),
        .clip(url: "fixture:clip.caf", start: 0, end: 4, source: "fixture-clip-1"),
        .speech(text: "The second story is a small one. A library lent out its first telescope."),
        .speech(text: "Within a week every night slot was booked, and the waiting list ran to the spring."),
        .speech(text: "Third, the weather. Mild, with a light wind from the west, and no rain until Thursday."),
        .speech(text: "That is the edition. Thank you for listening, and speak any time."),
    ])
}

// MARK: Built-in registration

extension StageRegistry {
    /// The Apple-framework and baseline stages of lane K. Other builders call `register` for their models.
    public static func builtIn() -> StageRegistry {
        let r = StageRegistry()
        r.register(.micSource, BuiltInMic.self) { _ in BuiltInMic() }
        r.register(.micSource, RouteMic.self) { _ in RouteMic() }
        r.register(.addressing, ButtonAddressing.self) { ButtonAddressing($0) }
        r.register(.addressing, OpenAddressing.self) { OpenAddressing($0) }
        r.register(.addressing, HybridAddressing.self) { HybridAddressing($0) }
        r.register(.speechBounds, EnergyVAD.self) { EnergyVAD($0) }
        r.register(.speakerCheck, NoSpeakerCheck.self) { _ in NoSpeakerCheck() }
        r.register(.yieldPolicy, DipThenPause.self) { DipThenPause($0) }
        r.register(.yieldPolicy, PauseAtOnce.self) { _ in PauseAtOnce() }
        r.register(.system1, FixedSystem1.self) { _ in FixedSystem1() }
        r.register(.holder, LocalAckHolder.self) { LocalAckHolder($0) }
        r.register(.desk, StubEchoDesk.self) { StubEchoDesk($0) }
        r.register(.programme, FixtureProgramme.self) { _ in FixtureProgramme() }
        registerAppleStages(r)
        registerChoirStages(r)
        registerExternalModels(r)
        return r
    }
}
