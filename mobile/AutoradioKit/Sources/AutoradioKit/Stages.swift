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
    /// The segments of a programme (station P1): ranges of `blocks`. Additive: a document saved before this decodes as empty and is one article.
    public var segments: [ProgrammeSegment]
    public init(id: String, title: String, blocks: [ProgrammeBlock], segments: [ProgrammeSegment] = []) {
        self.id = id; self.title = title; self.blocks = blocks; self.segments = segments
    }
    private enum CodingKeys: String, CodingKey { case id, title, blocks, segments }
    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id); title = try c.decode(String.self, forKey: .title)
        blocks = try c.decode([ProgrammeBlock].self, forKey: .blocks)
        segments = try c.decodeIfPresent([ProgrammeSegment].self, forKey: .segments) ?? []
    }

    /// A revision replaces everything from `fromBlock` on (the desk revises "from that place onward").
    /// Segments keep their ranges: an insertion shifts the later ones, and a segment the change cuts through is dropped.
    public func applying(_ r: Revision) -> ProgrammeDocument {
        var d = self
        let from = min(max(r.fromBlock, 0), d.blocks.count)
        d.blocks = Array(d.blocks[..<from]) + r.blocks + (r.keepRest ? Array(d.blocks[from...]) : [])
        d.segments = segments.compactMap { s in
            let end = s.firstBlock + s.blockCount
            if end <= from { return s }                                  // wholly before the change
            if s.firstBlock >= from, r.keepRest {                        // wholly after an insertion
                var t = s; t.firstBlock += r.blocks.count; return t
            }
            return nil                                                   // cut through, or replaced
        }
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
    /// One line per option saying when it is the right answer (Jev `criteria`). Nil: the client writes a plain default.
    public var criteria: [String: String]?
    public init(id: String, text: String, options: [String], criteria: [String: String]? = nil) {
        self.id = id; self.text = text; self.options = options; self.criteria = criteria
    }
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
    /// The page in front of the listener when they spoke (title and kind), so the acknowledgement can name it.
    public var pageTitle: String
    public var pageKind: String
    public var pageAddress: String
    /// The text of the page the phone has (fetched by the holder when it needs it; nil: none).
    public var pageText: (@Sendable () async -> String)?
    /// What the phone retrieved for this turn (Retrieval.swift), built once by the engine so the local model and the server desk see the same passages.
    public var retrieved: RetrievalContext?
    public init(listenerWords: String, answers: [System1Answer], pageTitle: String = "", pageKind: String = "", pageAddress: String = "",
                pageText: (@Sendable () async -> String)? = nil, retrieved: RetrievalContext? = nil) {
        self.listenerWords = listenerWords; self.answers = answers; self.pageTitle = pageTitle; self.pageKind = pageKind; self.pageAddress = pageAddress; self.pageText = pageText
        self.retrieved = retrieved
    }
}

public struct DeskCandidate: Codable, Sendable, Equatable { public var route: String, title: String
    public init(route: String, title: String) { self.route = route; self.title = title } }

public struct DeskTurn: Codable, Sendable {
    public var turnID: String
    public var programmeID: String
    public var blockIndex: Int
    public var listenerWords: String
    /// The listener cut into a line the radio was speaking (set by the engine at the moment the listener began, from the audio
    /// actually played). `spokenText` is the part of that line they heard, whole words only; `cutBlockIndex` is the line's block.
    public var interrupted: Bool
    public var spokenText: String
    public var cutBlockIndex: Int?
    /// The page in front when the listener asked (address, title, and the text the phone has), for a desk that answers about it.
    public var page: DeskPage?
    /// The play queue as the wire's `queue` object (JSON bytes) and the paper stories the DJ may add (`candidates`, up to 40). Both optional:
    /// a server that ignores them behaves as before.
    public var queue: Data?
    public var candidates: [DeskCandidate] = []
    public init(turnID: String, programmeID: String, blockIndex: Int, listenerWords: String,
                interrupted: Bool = false, spokenText: String = "", cutBlockIndex: Int? = nil, page: DeskPage? = nil) {
        self.turnID = turnID; self.programmeID = programmeID; self.blockIndex = blockIndex; self.listenerWords = listenerWords
        self.interrupted = interrupted; self.spokenText = spokenText; self.cutBlockIndex = cutBlockIndex; self.page = page
    }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        turnID = try c.decode(String.self, forKey: .turnID); programmeID = try c.decode(String.self, forKey: .programmeID)
        blockIndex = try c.decode(Int.self, forKey: .blockIndex); listenerWords = try c.decode(String.self, forKey: .listenerWords)
        interrupted = try c.decodeIfPresent(Bool.self, forKey: .interrupted) ?? false
        spokenText = try c.decodeIfPresent(String.self, forKey: .spokenText) ?? ""
        cutBlockIndex = try c.decodeIfPresent(Int.self, forKey: .cutBlockIndex)
        page = try c.decodeIfPresent(DeskPage.self, forKey: .page)
    }
}

public struct DeskPage: Codable, Sendable, Equatable {
    public var address: String, title: String, text: String
    public init(address: String = "", title: String = "", text: String = "") { self.address = address; self.title = title; self.text = text }
}

/// One event of a streamed desk answer.
public enum DeskEvent: Sendable, Equatable {
    /// One complete sentence to speak, in order (`seq` from 0).
    case sentence(seq: Int, text: String)
    /// The turn failed: `spoken` is the plain line to speak (the server's own fallback line), `reason` is for the log.
    case failed(spoken: String, reason: String)
    /// The last event. `status`: completed, failed or interrupted. `serverFirstSentenceMs` and `serverDoneMs`: the server's own clock.
    case done(status: String, sentences: Int, serverFirstSentenceMs: Double?, serverDoneMs: Double?)
    /// The server's `queue_ops` event, its JSON unread: handed to the queue lane's apply. Nothing is spoken for it (the server speaks).
    case queueOps(json: Data)
}

/// A desk that answers as a stream of spoken sentences (the Autoradio desk's `POST /api/autoradio/turns`). The engine speaks each
/// sentence as it arrives instead of waiting for a whole revision.
public protocol StreamingDesk: Desk {
    func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error>
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
    /// The immediate form: the first line comes at once, without System 1's answers; `later` resolves to them (empty
    /// if System 1 failed or timed out) and selects only what follows.
    func lines(for input: HolderInput, later: @escaping @Sendable () async -> [System1Answer]) -> AsyncStream<String>
    /// Stop producing lines (System 1 said the turn needs nothing, or the desk answered).
    func cancel()
    /// Why the last call produced the line it did, or nothing (for the trace). Empty when there is nothing to say.
    func diagnostics() -> String
    /// What the local model's generation cost (prompt tokens, how many were already cached), for the trace.
    func report() -> GenerationReport?
    /// True when what the holder is saying is itself an answer (from the reader's saved stories): the desk's answer waits for it to end
    /// instead of cutting it at a sentence. False: the holder only fills the wait and is cut when the desk speaks.
    var keepsSpeakingForDesk: Bool { get }
    /// Search what the phone holds for this question (nil: this holder does not retrieve). The engine calls it once per turn and hands the result to
    /// the holder (HolderInput.retrieved) and to the server desk (DeskContext), so both answer from the same passages.
    func retrieve(words: String, pageTitle: String, pageAddress: String) async -> RetrievalContext?
}
public extension Holder {
    func retrieve(words: String, pageTitle: String, pageAddress: String) async -> RetrievalContext? { nil }
}
public extension Holder {
    var keepsSpeakingForDesk: Bool { false }
    func lines(for input: HolderInput, later: @escaping @Sendable () async -> [System1Answer]) -> AsyncStream<String> { lines(for: input) }
    func cancel() {}
    func diagnostics() -> String { "" }
    func report() -> GenerationReport? { nil }
}

/// Text in, audio out. The first frame's time is measured by whoever consumes the stream.
public protocol Voice: Stage {
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error>
}

/// System 2 over the network: submit the turn at a place in the document, receive revisions.
public protocol Desk: Stage {
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error>
    /// The same, with what the phone has actually said aloud so far (read `heard.snapshot` when the request is built). A desk that
    /// can use it (the streaming route's `heard` list) must not repeat it; the polling desk ignores it.
    func submit(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<Revision, Error>
}
public extension Desk {
    func submit(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<Revision, Error> { submit(turn) }
}

public protocol Programme: Stage {
    func load() async throws -> ProgrammeDocument
}
