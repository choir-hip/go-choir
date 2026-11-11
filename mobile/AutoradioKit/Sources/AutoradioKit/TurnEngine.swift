import Foundation

// MARK: Host-supplied seams

public enum AudioState: String, Codable, Sendable {
    /// Full-quality playback, no voice processing. Mic feeds detectors only.
    case listening
    /// Programme paused or ducked, voice processing on, so the listener can interrupt a spoken answer.
    case talking
}

/// The only thing that touches the system audio session. The kit never calls AVAudioSession itself:
/// the Exact app adapts Exact's session claims to this protocol; the Mac replay command uses `NullSessionOwner`.
public protocol AudioSessionOwner: AnyObject, Sendable {
    /// Make the session `state`. A change of state is release-then-claim (the first claim's mode wins in Exact).
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws
    /// What the session is now, for the trace: category, mode, input and output ports.
    func describe() -> String
    /// Give the claim up (Stop). The session falls back to what the app's own sound wants.
    func release()
    /// The system told the claim something: "interrupted", "resumed", "reset", "route" (detail: the route now).
    func observe(_ handler: @escaping @Sendable (_ kind: String, _ detail: String) -> Void)
}
public extension AudioSessionOwner {
    func describe() -> String { "" }
    func release() {}
    func observe(_ handler: @escaping @Sendable (String, String) -> Void) {}
}

/// Used by the macOS replay command and tests: there is no session to claim.
public final class NullSessionOwner: AudioSessionOwner, @unchecked Sendable {
    public init() {}
    public func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {}
}

public struct ProgramPosition: Codable, Sendable, Equatable {
    public var programmeID: String
    public var blockIndex: Int
    public var context: String
    public var questions: [Question]
    public init(programmeID: String, blockIndex: Int, context: String, questions: [Question] = Question.defaults) {
        self.programmeID = programmeID; self.blockIndex = blockIndex; self.context = context; self.questions = questions
    }
}
extension Question {
    /// Early structure lives in questions, not types (turns note). These are examples the desk package will own.
    public static let defaults = [
        Question(id: "asks_for_something", text: "Does the listener ask for something to be done or looked up?", options: ["yes", "no"],
                 criteria: ["yes": "The listener asks for something to be done, looked up, changed or explained.",
                            "no": "The listener only comments, reacts or asks nothing that needs doing."]),
        Question(id: "answer_in_prepared", text: "Is the answer in the prepared material for this passage?", options: ["yes", "no"],
                 criteria: ["yes": "The context already contains what the listener asks about.",
                            "no": "The context does not contain what the listener asks about."]),
        Question(id: "carry_on", text: "Should the programme carry on meanwhile?", options: ["yes", "no"],
                 criteria: ["yes": "The programme can keep playing while the request is handled.",
                            "no": "The programme should stay paused until the request is handled."]),
    ]
}

/// The player, as the engine sees it.
public protocol PlaybackControl: AnyObject, Sendable {
    func position() async -> ProgramPosition
    func setLevel(_ level: PlayLevel) async
    /// Play voice frames (acknowledgement, holding lines) now, independent of the programme level. Returns when done.
    func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async
    /// The desk revised the document from a place onward.
    func apply(_ revision: Revision) async
    /// Return to the programme at the right place.
    func resume() async
    /// The programme line being spoken this instant and how much of it has been heard; nil when nothing is being spoken
    /// (paused, a clip, nothing playing). Asked by the engine at the moment a listener begins, so a turn knows what it cut into.
    func spoken() async -> SpokenCut?
}
public extension PlaybackControl { func spoken() async -> SpokenCut? { nil } }

/// Replay's player: nothing audible, the document is tracked.
public final class SimulatedPlayback: PlaybackControl, @unchecked Sendable {
    public var pos: ProgramPosition
    public private(set) var spokenSeconds = 0.0
    public private(set) var revisions: [Revision] = []
    public init(position: ProgramPosition) { pos = position }
    public func position() async -> ProgramPosition { pos }
    public func setLevel(_ level: PlayLevel) async {}
    public func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async {
        do { for try await f in frames { spokenSeconds += f.duration } } catch {}
    }
    public func apply(_ r: Revision) async { revisions.append(r) }
    public func resume() async {}
}

// MARK: Engine

public enum TurnPhase: String, Codable, Sendable {
    case playing, candidate, addressed, capturing, ended, acknowledging, holding, revised
}

public struct EngineEnvironment: Sendable {
    public var registry: StageRegistry
    public var log: EventLog
    public var playback: PlaybackControl
    public var session: AudioSessionOwner
    public var device: DeviceInfo
    public var onRecord: @Sendable (TurnRecord) -> Void
    public var onOutcome: @Sendable (String, Outcome) -> Void
    public init(registry: StageRegistry, log: EventLog, playback: PlaybackControl, session: AudioSessionOwner = NullSessionOwner(),
                device: DeviceInfo = .current, onRecord: @escaping @Sendable (TurnRecord) -> Void = { _ in },
                onOutcome: @escaping @Sendable (String, Outcome) -> Void = { _, _ in }) {
        self.registry = registry; self.log = log; self.playback = playback; self.session = session
        self.device = device; self.onRecord = onRecord; self.onOutcome = onOutcome
    }
}

/// Tunables of the engine itself (not stages).
public struct EngineLimits: Sendable {
    public var prerollSeconds = 0.5
    public var stageDeadline = 15.0       // transcription final, System 1
    public var system1Deadline = 2.0
    public var holdTimeout = 120.0
    public var spokeAgainWindow = 15.0
    public init() {}
}

public actor TurnEngine {
    let env: EngineEnvironment
    let limits: EngineLimits
    var config: LabConfiguration
    var configVersion = 0, idleVersion = -1
    public private(set) var phase: TurnPhase = .playing
    var addressing: Addressing?
    var bounds: SpeechBounds?
    var preroll: [AudioFrame] = []
    var turn: Turn?
    var level: PlayLevel = .play
    var idleWaiters: [CheckedContinuation<Void, Never>] = []
    var lastFinished: (id: String, t: Double)?
    var lastFrameT = 0.0
    var buttonDownAt: Double?
    var pipeline: Task<Void, Never>?

    final class Turn {
        let id: String
        let config: LabConfiguration
        let transcriber: Transcriber
        let yield: YieldPolicy
        let session: TranscriptionSession
        let firstEventIndex: Int
        var addressed = false
        var speechSeen = false
        var addressedAt = 0.0
        var partialTask: Task<Void, Never>?
        var ended = false
        /// What the listener cut into when this turn began (nil: the radio was not speaking).
        var cut: SpokenCut?
        init(id: String, config: LabConfiguration, transcriber: Transcriber, yield: YieldPolicy, session: TranscriptionSession, firstEventIndex: Int) {
            self.id = id; self.config = config; self.transcriber = transcriber; self.yield = yield; self.session = session; self.firstEventIndex = firstEventIndex
        }
    }

    public init(env: EngineEnvironment, configuration: LabConfiguration, limits: EngineLimits = EngineLimits()) {
        self.env = env; self.config = configuration; self.limits = limits
        ChoirEnvironment.shared.log = env.log   // the Choir stages record request timings on this turn's log
    }

    // MARK: Public API

    /// Applies to the next turn (idle stages are re-resolved when the engine is next idle).
    public func setConfiguration(_ c: LabConfiguration) {
        config = c; configVersion += 1
        env.log.emit(.configurationChanged(c.name))
    }
    public var configuration: LabConfiguration { config }

    public func buttonDown() async {
        let now = env.log.clock.now
        buttonDownAt = now
        if phase == .acknowledging || phase == .holding || phase == .revised { await interruptPipeline() }
        await prepareIdle()
        await route(.buttonDown, now: now)
    }
    /// `held` overrides the measured hold (replay passes the recorded one: compute time must not count as holding).
    public func buttonUp(held explicit: Double? = nil) async {
        let now = env.log.clock.now
        let held = explicit ?? (now - (buttonDownAt ?? now))
        buttonDownAt = nil
        await route(.buttonUp(heldSeconds: held), now: now)
    }

    /// One mic frame, in order. The engine is frame-driven so replay can run in lockstep.
    public func ingest(_ f: AudioFrame) async {
        lastFrameT = f.t
        await prepareIdle()
        if phase == .acknowledging || phase == .holding || phase == .revised || phase == .ended { return }
        if turn == nil {
            preroll.append(f)
            var total = preroll.reduce(0.0) { $0 + $1.duration }
            while total > limits.prerollSeconds, preroll.count > 1 { total -= preroll.removeFirst().duration }
        } else {
            turn!.session.append(f)
        }
        guard let bounds else { return }
        switch await bounds.feed(f) {
        case .speechStarted: await onSpeechStarted(f)
        case .speechEnded(_, let secs): await onSpeechEnded(secs, now: f.t + f.duration)
        case .none: break
        }
        await tickYield(now: f.t + f.duration)
        if let t = turn, t.addressed, !t.speechSeen, let a = addressing, f.t - t.addressedAt > a.noSpeechTimeout {
            await dropTurn(reason: "no speech after being addressed")
        }
    }

    /// Wait until no turn is in flight (used by replay after the audio ends).
    public func waitUntilIdle() async {
        if phase == .playing && turn == nil && pipeline == nil { return }
        await withCheckedContinuation { idleWaiters.append($0) }
    }
    public func noteOutcome(_ o: Outcome) { if let l = lastFinished { env.onOutcome(l.id, o) } }

    // MARK: Idle stages

    func prepareIdle() async {
        guard turn == nil, phase == .playing, idleVersion != configVersion else { return }
        do {
            addressing = try env.registry.make(.addressing, as: Addressing.self, config)
            bounds = try env.registry.make(.speechBounds, as: SpeechBounds.self, config)
            await bounds?.reset()
            idleVersion = configVersion
        } catch { env.log.emit(.error(stage: "idle-stages", message: "\(error)")); idleVersion = configVersion }
    }

    func setPhase(_ p: TurnPhase) {
        guard p != phase else { return }
        env.log.emit(.phase(from: phase.rawValue, to: p.rawValue))
        phase = p
        if p == .playing && turn == nil && pipeline == nil { let w = idleWaiters; idleWaiters = []; for c in w { c.resume() } }
    }

    // MARK: Routing

    func route(_ input: AddressInput, now: Double) async {
        guard let addressing else { return }
        let d = await addressing.decide(input, now: now)
        switch d {
        case .none: break
        case .candidate:
            if turn == nil { await beginTurn(addressed: false, now: now) }
        case .addressed(let by):
            if turn == nil { await beginTurn(addressed: true, now: now) }
            if let t = turn, !t.addressed { t.addressed = true; t.addressedAt = now; env.log.emit(.addressed(by: by)); setPhase(t.speechSeen ? .capturing : .addressed); await claimTalking() }
        case .endTurn:
            if turn != nil { await endTurn() }
        }
    }

    func onSpeechStarted(_ f: AudioFrame) async {
        env.log.emit(.speechStarted)
        if let t = turn { t.speechSeen = true; if t.addressed { setPhase(.capturing) }; return }
        await route(.speechStarted, now: f.t)
        if let t = turn { t.speechSeen = true }
    }

    func onSpeechEnded(_ secs: Double, now: Double) async {
        env.log.emit(.speechEnded(seconds: secs))
        guard let t = turn else { return }
        if t.addressed {
            if addressing?.holdsTurnOpen == true { return }   // the hand keeps the turn open
            await endTurn()
            return
        }
        // Candidate: judge it by its words.
        let text = await finalTranscript(t)
        guard let addressing else { return }
        switch await addressing.decide(.finalTranscript(text, speechSeconds: secs), now: now) {
        case .addressed(let by):
            t.addressed = true; t.addressedAt = now
            env.log.emit(.addressed(by: by)); await claimTalking()
            await runTurn(t, transcript: text)
        default:
            env.log.emit(.notATurn(reason: "candidate speech did not count: \"\(text)\""))
            await dropTurn(reason: nil, notATurn: true)
        }
    }

    func claimTalking() async {
        do { try await env.session.claim(.talking, preferBuiltInMic: false, reason: "addressed") }
        catch { env.log.emit(.error(stage: "audio-session", message: "\(error)")) }
    }

    // MARK: Turn lifecycle

    func beginTurn(addressed: Bool, now: Double) async {
        let id = "turn-" + UUID().uuidString.prefix(8).lowercased()
        let cfg = config
        // First, before anything else takes time or yields the programme: what was being said, and how far.
        let cut = await env.playback.spoken()
        do {
            let transcriber = try env.registry.make(.transcriber, as: Transcriber.self, cfg)
            let yield = try env.registry.make(.yieldPolicy, as: YieldPolicy.self, cfg)
            // A transcriber that cannot start (no permission, no model) must not end the turn: the turn goes on with an
            // empty transcript, the error is on the record, and the button-addressed path still works.
            let session: TranscriptionSession
            do { session = try await env.log.timed("transcriber.begin") { try await transcriber.begin() } }
            catch {
                env.log.emit(.error(stage: "transcriber", message: "\(error); this turn has no transcript"))
                session = EmptyTranscription()
            }
            let t = Turn(id: id, config: cfg, transcriber: transcriber, yield: yield, session: session, firstEventIndex: env.log.count)
            for f in preroll { session.append(f) }
            preroll = []
            t.partialTask = Task { [log = env.log] in for await p in session.partials { log.emit(.partialTranscript(p)) } }
            t.cut = cut
            turn = t
            if let l = lastFinished, now - l.t < limits.spokeAgainWindow { env.onOutcome(l.id, .spokeAgain); lastFinished = nil }
            setPhase(.candidate)
        } catch {
            env.log.emit(.error(stage: "turn-start", message: "\(error)"))
        }
    }

    func finalTranscript(_ t: Turn) async -> String {
        let log = env.log
        let text = await withDeadline(limits.stageDeadline) { [session = t.session] in
            try? await log.timed("transcribe") { try await session.finish() }
        } ?? nil
        let s = (text ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        log.emit(.finalTranscript(s))
        return s
    }

    func endTurn() async {
        guard let t = turn, !t.ended else { return }
        t.ended = true
        setPhase(.ended)
        env.log.emit(.turnEnded)
        let text = await finalTranscript(t)
        if text.isEmpty { env.log.emit(.notATurn(reason: "empty transcript")); t.ended = false; await dropTurn(reason: nil, notATurn: true); return }
        await runTurn(t, transcript: text)
    }

    /// A candidate that counted becomes ended, then the pipeline.
    func runTurn(_ t: Turn, transcript: String) async {
        if !t.ended { t.ended = true; setPhase(.ended); env.log.emit(.turnEnded) }
        if let a = addressing { _ = await a.decide(.turnFinished, now: env.log.clock.now) }
        let endedAt = env.log.clock.now
        turn = nil
        t.partialTask?.cancel()
        pipeline = Task { [self] in await self.runPipeline(t, transcript: transcript, endedAt: endedAt) }
    }

    func dropTurn(reason: String?, notATurn: Bool = false) async {
        if let r = reason { env.log.emit(.notATurn(reason: r)) }
        if let t = turn { t.session.cancel(); t.partialTask?.cancel() }
        if let a = addressing { _ = await a.decide(.turnFinished, now: env.log.clock.now) }
        turn = nil
        await setLevelTo(.play, reason: .resume)
        do { try await env.session.claim(.listening, preferBuiltInMic: false, reason: "not a turn") } catch {}
        setPhase(.playing)
        await bounds?.reset()
    }

    // MARK: Yield

    func tickYield(now: Double) async {
        guard let t = turn, let bounds else { return }
        let secs = await bounds.speechSeconds(now: now)
        let target = t.yield.target(for: YieldSituation(current: level, speechSeconds: secs, addressed: t.addressed, notATurn: false))
        if target != level {
            await setLevelTo(target, reason: target == .play ? .resume : (target == .dip ? .dip : .pause))
        }
    }

    func setLevelTo(_ l: PlayLevel, reason: YieldKind) async {
        guard l != level else { return }
        level = l
        env.log.emit(.yielded(reason))
        await env.playback.setLevel(l)
        env.log.emit(.playbackState(l == .play ? .playing : (l == .dip ? .dipped : .paused)))
    }

    // MARK: After the turn: acknowledge, hold, revise

    func interruptPipeline() async {
        pipeline?.cancel(); pipeline = nil
        if let l = lastFinished { env.onOutcome(l.id, .spokeAgain) }
        setPhase(.playing)
    }

    func runPipeline(_ t: Turn, transcript: String, endedAt: Double) async {
        let log = env.log
        let cfg = t.config
        let pos = await env.playback.position()
        setPhase(.acknowledging)

        var system1: System1?, holder: Holder?, voice: Voice?, desk: Desk?
        do {
            system1 = try env.registry.make(.system1, as: System1.self, cfg)
            holder = try env.registry.make(.holder, as: Holder.self, cfg)
            voice = try env.registry.make(.voice, as: Voice.self, cfg)
            desk = try env.registry.make(.desk, as: Desk.self, cfg)
        } catch { log.emit(.error(stage: "resolve", message: "\(error)")) }

        // The desk starts at once; System 1 and the acknowledgement do not wait for it.
        let revisionSeen = Flag()
        var relayTask: Task<Void, Never>?
        var deskTask: Task<Void, Never>?
        if let desk {
            let turnReq = DeskTurn(turnID: t.id, programmeID: pos.programmeID, blockIndex: pos.blockIndex, listenerWords: transcript,
                                 interrupted: t.cut != nil, spokenText: t.cut?.spoken ?? "", cutBlockIndex: t.cut?.blockIndex)
            deskTask = Task { [self] in
                do {
                    for try await r in desk.submit(turnReq) {
                        await self.onRevision(r, flag: revisionSeen, endedAt: endedAt, relay: { relayTask?.cancel() })
                    }
                } catch { log.emit(.error(stage: "desk", message: "\(error)")) }
            }
        }

        // System 1, with a deadline: a slow answer must not hold the acknowledgement. It runs as a task so the
        // immediate acknowledgement can start beside it.
        let system1Limit = limits.system1Deadline
        let s1Task: Task<[System1Answer], Never> = Task {
            guard let system1 else { return [] }
            let system1Threw = Flag()
            let input = System1Input(context: pos.context, listenerWords: transcript, questions: pos.questions)
            let got: [System1Answer]? = await withDeadline(system1Limit) {
                do { return try await log.timed("system1") { try await system1.answer(input) } }
                catch { system1Threw.set(); log.emit(.error(stage: "system1", message: "\(error)")); return nil }
            } ?? nil
            if got == nil, !system1Threw.isSet { log.emit(.error(stage: "system1", message: "no answer within \(system1Limit)s; acknowledging without it")) }
            let answers = got ?? []
            for a in answers { log.emit(.system1Answer(questionID: a.questionID, option: a.option, score: a.score, ms: a.ms)) }
            return answers
        }
        let immediate = cfg.ack == .immediate
        var answers: [System1Answer] = []
        if !immediate { answers = await s1Task.value }   // afterSystem1: the line is chosen from the answers

        // Acknowledge, then hold.
        var speakTask: Task<Void, Never>?
        var verdictTask: Task<Void, Never>?
        let audible = Flag()
        if let holder, let voice {
            let (relayOut, relayCont) = AsyncStream.makeStream(of: String.self)
            let src = immediate
                ? holder.lines(for: HolderInput(listenerWords: transcript, answers: []), later: { await s1Task.value })
                : holder.lines(for: HolderInput(listenerWords: transcript, answers: answers))
            relayTask = Task {
                for await l in src { log.emit(.holdingSaid(l)); relayCont.yield(l) }
                relayCont.finish()
            }
            let rt = relayTask
            relayCont.onTermination = { _ in rt?.cancel() }
            let frames = voice.speak(relayOut)
            let (tapOut, tapCont) = AsyncThrowingStream.makeStream(of: AudioFrame.self)
            let start = log.clock.now
            speakTask = Task {
                var first = true
                do {
                    for try await f in frames {
                        if first {
                            first = false
                            audible.set()
                            let now = log.clock.now
                            log.emit(.stageTiming(stage: "voice.firstAudio", startedAt: start, endedAt: now))
                            log.emit(.ackStarted(msSinceTurnEnd: (now - endedAt) * 1000))
                        }
                        tapCont.yield(f)
                    }
                    tapCont.finish()
                } catch { log.emit(.error(stage: "voice", message: "\(error)")); tapCont.finish(throwing: error) }
            }
            setPhase(.holding)
            let pb = env.playback
            let play = Task { await pb.speak(tapOut) }
            if immediate {
                // System 1 selects what follows. If it says the turn needs nothing, stop the acknowledgement and the
                // desk. An ack not yet audible is cancelled outright; one already speaking finishes its line (never
                // cut mid-word) and no further line starts.
                let (st, dt, rl) = (speakTask, deskTask, relayTask)
                verdictTask = Task {
                    let a = await s1Task.value
                    guard Self.needsNothing(a) else { return }
                    let wasAudible = audible.isSet
                    log.emit(.ackCancelled(reason: "System 1: the listener asked for nothing", audible: wasAudible))
                    holder.cancel(); rl?.cancel()
                    if !wasAudible { st?.cancel(); play.cancel(); tapCont.finish() }
                    if let dt { log.emit(.note("desk not engaged: System 1 said nothing was asked")); dt.cancel() }
                }
            }
            await withTaskCancellationHandler { await play.value } onCancel: { play.cancel(); speakTask?.cancel() }
        } else { setPhase(.holding) }
        if immediate, holder == nil || voice == nil {
            let a = await s1Task.value
            if Self.needsNothing(a) { log.emit(.note("desk not engaged: System 1 said nothing was asked")); deskTask?.cancel() }
        }
        _ = await verdictTask?.value
        _ = await s1Task.value

        // Hold until the desk finishes (or the timeout), never silent: more holding lines already came from the holder.
        if let deskTask {
            let done: Void? = await withDeadline(limits.holdTimeout) { await deskTask.value }
            if done == nil { log.emit(.error(stage: "desk", message: "no revision within \(limits.holdTimeout)s")); deskTask.cancel() }
        }
        relayTask?.cancel()
        _ = speakTask
        if Task.isCancelled { return }

        await env.playback.resume()
        let tid = t.id
        let events = Array(log.events(since: t.firstEventIndex))
        let rec = TurnRecord.fold(id: tid, configuration: cfg, device: env.device, events: events)
        env.onRecord(rec)
        lastFinished = (tid, log.clock.now)
        pipeline = nil
        await setLevelTo(.play, reason: .resume)
        do { try await env.session.claim(.listening, preferBuiltInMic: false, reason: "turn over") } catch {
            log.emit(.error(stage: "audio-session", message: "\(error)"))
        }
        if let b = bounds { await b.reset() }
        setPhase(.playing)
    }

    /// System 1 answered "asks_for_something: no": the utterance needs nothing, the programme carries on.
    static func needsNothing(_ answers: [System1Answer]) -> Bool {
        answers.first { $0.questionID == "asks_for_something" }?.option == "no"
    }

    func onRevision(_ r: Revision, flag: Flag, endedAt: Double, relay: () -> Void) async {
        env.log.emit(.revisionArrived(id: r.id, fromBlock: r.fromBlock, blocks: r.blocks.count, summary: r.blocks.first?.summary ?? "(empty)"))
        flag.set()
        relay()    // no new holding lines; the one in flight finishes
        setPhase(.revised)
        await env.playback.apply(r)
    }
}

final class Flag: @unchecked Sendable {
    private let l = NSLock(); private var v = false
    func set() { l.lock(); v = true; l.unlock() }
    var isSet: Bool { l.lock(); defer { l.unlock() }; return v }
}

/// Run `body` with a deadline; nil if it did not finish in time. The body is cancelled but not awaited.
func withDeadline<T: Sendable>(_ seconds: Double, _ body: @escaping @Sendable () async -> T) async -> T? {
    await withCheckedContinuation { (c: CheckedContinuation<T?, Never>) in
        let once = Once<T?>(c)
        let work = Task { once.resume(await body()) }
        Task { try? await Task.sleep(nanoseconds: UInt64(seconds * 1e9)); work.cancel(); once.resume(nil) }
    }
}
/// Stands in when the transcriber could not start: hears nothing, says nothing.
final class EmptyTranscription: TranscriptionSession, @unchecked Sendable {
    let partials = AsyncStream<String> { $0.finish() }
    func append(_ frame: AudioFrame) {}
    func finish() async throws -> String { "" }
    func cancel() {}
}
final class Once<T>: @unchecked Sendable {
    private let l = NSLock(); private var c: CheckedContinuation<T, Never>?
    init(_ c: CheckedContinuation<T, Never>) { self.c = c }
    func resume(_ v: T) { l.lock(); let x = c; c = nil; l.unlock(); x?.resume(returning: v) }
}
