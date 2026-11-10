import Foundation

// One protocol per job. Keep each small. A new model is a new class conforming to one of these,
// registered in a StageRegistry. See mobile/README.md.

// MARK: Programme (the document being played)

public enum ProgrammeBlock: Codable, Sendable, Equatable {
    case speech(text: String)
    case clip(url: String, start: Double, end: Double?, source: String)
    case listenerPassage(text: String, afterBlock: Int?)

    public var spokenText: String? {
        switch self {
        case .speech(let t): return t
        case .listenerPassage(let t, _): return t
        case .clip: return nil
        }
    }
    public var summary: String {
        switch self {
        case .speech(let t): return "speech: " + String(t.prefix(60))
        case .clip(let u, let s, let e, _): return "clip \(URL(string: u)?.lastPathComponent ?? u) \(s)-\(e.map { String($0) } ?? "end")"
        case .listenerPassage(let t, _): return "listener: " + String(t.prefix(60))
        }
    }
}

public struct ProgrammeDocument: Codable, Sendable, Equatable {
    public var id: String
    public var title: String
    public var blocks: [ProgrammeBlock]
    public init(id: String, title: String, blocks: [ProgrammeBlock]) { self.id = id; self.title = title; self.blocks = blocks }

    /// A revision replaces everything from `fromBlock` on (the desk revises "from that place onward").
    public func applying(_ r: Revision) -> ProgrammeDocument {
        var d = self
        let from = min(max(r.fromBlock, 0), d.blocks.count)
        d.blocks = Array(d.blocks[..<from]) + r.blocks + (r.keepRest ? Array(d.blocks[from...]) : [])
        return d
    }
}

/// What the desk sends back: new blocks from a place onward. "Resume" is a revision that changes nothing.
public struct Revision: Codable, Sendable, Equatable {
    public var id: String
    public var fromBlock: Int
    public var blocks: [ProgrammeBlock]
    /// When true the blocks are inserted and what followed stays; otherwise they replace the rest.
    public var keepRest: Bool
    public var isFinal: Bool
    public init(id: String, fromBlock: Int, blocks: [ProgrammeBlock], keepRest: Bool = true, isFinal: Bool = true) {
        self.id = id; self.fromBlock = fromBlock; self.blocks = blocks; self.keepRest = keepRest; self.isFinal = isFinal
    }
}

// MARK: System 1 types

public struct Question: Codable, Sendable, Equatable {
    public var id: String
    public var text: String
    public var options: [String]
    public init(id: String, text: String, options: [String]) { self.id = id; self.text = text; self.options = options }
}
public struct System1Input: Codable, Sendable {
    public var context: String
    public var listenerWords: String
    public var questions: [Question]
    public init(context: String, listenerWords: String, questions: [Question]) {
        self.context = context; self.listenerWords = listenerWords; self.questions = questions
    }
}
public struct System1Answer: Codable, Sendable, Equatable {
    public var questionID: String
    public var option: String
    public var score: Double
    public var ms: Double
    public init(questionID: String, option: String, score: Double, ms: Double) {
        self.questionID = questionID; self.option = option; self.score = score; self.ms = ms
    }
}

public struct HolderInput: Sendable {
    public var listenerWords: String
    public var answers: [System1Answer]
    public init(listenerWords: String, answers: [System1Answer]) { self.listenerWords = listenerWords; self.answers = answers }
}

public struct DeskTurn: Codable, Sendable {
    public var turnID: String
    public var programmeID: String
    public var blockIndex: Int
    public var listenerWords: String
    public init(turnID: String, programmeID: String, blockIndex: Int, listenerWords: String) {
        self.turnID = turnID; self.programmeID = programmeID; self.blockIndex = blockIndex; self.listenerWords = listenerWords
    }
}

// MARK: Audio-facing stages

public enum InputPreference: String, Codable, Sendable { case builtIn, currentRoute }

/// Which microphone, and how it is opened. Live sources only; replay feeds a FileAudioSource.
public protocol MicSource: Stage {
    var inputPreference: InputPreference { get }
}

public enum BoundsUpdate: Sendable, Equatable {
    case none
    case speechStarted(at: Double)
    /// `seconds` is the speech length, hangover excluded.
    case speechEnded(at: Double, seconds: Double)
}

/// Start and end of speech from audio frames. Stateful; one instance per listening period.
public protocol SpeechBounds: Stage {
    func feed(_ frame: AudioFrame) async -> BoundsUpdate
    func reset() async
    /// Seconds of continuous speech so far (0 when silent).
    func speechSeconds(now: Double) async -> Double
}

public protocol SpeakerCheck: Stage {
    /// Is this audio the listener's voice? `none` always says yes.
    func isListener(_ frames: [AudioFrame]) async -> Bool
}

public enum AddressInput: Sendable, Equatable {
    case buttonDown, buttonUp(heldSeconds: Double)
    case speechStarted
    case speechEnded(seconds: Double)
    /// The final transcript of a candidate segment.
    case finalTranscript(String, speechSeconds: Double)
    case turnFinished
}
public enum AddressDecision: Sendable, Equatable {
    case none            // not for the radio (or nothing to decide)
    case candidate       // keep listening, buffer, judge at the end
    case addressed(by: String)
    case endTurn         // the listener's hand ends the turn
}

/// Is the listener talking to the radio. Stateful per session; it also owns window timers.
public protocol Addressing: Stage {
    func decide(_ input: AddressInput, now: Double) async -> AddressDecision
    /// True while the listener's hand keeps the turn open (button held).
    var holdsTurnOpen: Bool { get }
    /// An addressed turn that never gets speech is dropped after this long.
    var noSpeechTimeout: Double { get }
}

public protocol TranscriptionSession: AnyObject, Sendable {
    var partials: AsyncStream<String> { get }
    func append(_ frame: AudioFrame)
    func finish() async throws -> String
    func cancel()
}
public protocol Transcriber: Stage {
    func begin() async throws -> TranscriptionSession
}

public enum PlayLevel: String, Codable, Sendable { case play, dip, pause }
public struct YieldSituation: Sendable {
    public var current: PlayLevel
    public var speechSeconds: Double
    public var addressed: Bool
    public var notATurn: Bool
    public init(current: PlayLevel, speechSeconds: Double, addressed: Bool, notATurn: Bool) {
        self.current = current; self.speechSeconds = speechSeconds; self.addressed = addressed; self.notATurn = notATurn
    }
}
/// What playback does while the listener speaks. A pure function of the situation, called every frame.
public protocol YieldPolicy: Stage {
    func target(for s: YieldSituation) -> PlayLevel
}

public protocol System1: Stage {
    func answer(_ input: System1Input) async throws -> [System1Answer]
}

/// Produces the acknowledgement and holding lines, as text, as they become known.
public protocol Holder: Stage {
    func lines(for input: HolderInput) -> AsyncStream<String>
}

/// Text in, audio out. The first frame's time is measured by whoever consumes the stream.
public protocol Voice: Stage {
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error>
}

/// System 2 over the network: submit the turn at a place in the document, receive revisions.
public protocol Desk: Stage {
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error>
}

public protocol Programme: Stage {
    func load() async throws -> ProgrammeDocument
}
