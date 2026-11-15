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
    /// The page in front of the listener right now (title and kind), for the acknowledgement to name. nil: none known.
    public var frontPage: @Sendable () -> FrontPage?
    /// The text of the page at an address, as far as the phone has it (the local stream may say something from it).
    public var pageText: @Sendable (String) async -> String
    public init(registry: StageRegistry, log: EventLog, playback: PlaybackControl, session: AudioSessionOwner = NullSessionOwner(),
                device: DeviceInfo = .current, onRecord: @escaping @Sendable (TurnRecord) -> Void = { _ in },
                onOutcome: @escaping @Sendable (String, Outcome) -> Void = { _, _ in },
                frontPage: @escaping @Sendable () -> FrontPage? = { nil },
                pageText: @escaping @Sendable (String) async -> String = { _ in "" }) {
        self.registry = registry; self.log = log; self.playback = playback; self.session = session
        self.device = device; self.onRecord = onRecord; self.onOutcome = onOutcome; self.frontPage = frontPage; self.pageText = pageText
    }
}

/// Tunables of the engine itself (not stages).
public struct EngineLimits: Sendable {
    public var prerollSeconds = 0.5
    // No deadline around any model call (owner ruling 28, 2026-11-11): what ends a stage is its own end, a real error, or the listener's
    // Cancel. Every stage's time is still recorded in the timeline.
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
    /// A cancelled hold's release must not start a turn.
    var ignoreNextButtonUp = false
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
        /// The page in front when the listener asked (a typed turn names it; a spoken one is read from the environment).
        var front: FrontPage?
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
        ignoreNextButtonUp = false          // a new press: the release of a cancelled hold that never came must not swallow this turn's Send
        buttonDownAt = now
        if phase == .acknowledging || phase == .holding || phase == .revised { await interruptPipeline() }
        await prepareIdle()
        await route(.buttonDown, now: now)
    }
    /// A typed question (the bar's text field): the same pipeline as a spoken turn (System 1, the holder's acknowledgement, the
    /// desk), with the words given instead of heard, so there is no microphone session and nothing to transcribe.
    /// WAYS THIS CAN FAIL (written first): empty text: ignored; a spoken turn is already open: ignored (the voice wins, the
    /// text stays in the field); the radio is mid-acknowledgement: that pipeline is interrupted like a new button press does;
    /// a stage cannot be made: the error is logged and no turn starts (never a stuck phase).
    /// Returns whether a turn started.
    @discardableResult
    public func typedTurn(_ text: String, front: FrontPage? = nil) async -> Bool {
        let words = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !words.isEmpty else { return false }
        let now = env.log.clock.now
        if phase == .acknowledging || phase == .holding || phase == .revised { await interruptPipeline() }
        await prepareIdle()
        guard turn == nil else { return false }
        let cfg = config
        let cut = await env.playback.spoken()
        do {
            let transcriber = try env.registry.make(.transcriber, as: Transcriber.self, cfg)
            let yield = try env.registry.make(.yieldPolicy, as: YieldPolicy.self, cfg)
            let t = Turn(id: "turn-" + UUID().uuidString.prefix(8).lowercased(), config: cfg, transcriber: transcriber, yield: yield,
                         session: EmptyTranscription(), firstEventIndex: env.log.count)
            t.cut = cut; t.addressed = true; t.addressedAt = now; t.front = front
            turn = t
            setPhase(.candidate)
            // The turn ends the moment it is sent; its words follow, as for a heard turn (the trace anchors on `turnEnded`).
            t.ended = true; setPhase(.ended); env.log.emit(.turnEnded)
            env.log.emit(.finalTranscript(words))
            await runTurn(t, transcript: words)
            return true
        } catch {
            env.log.emit(.error(stage: "turn-start", message: "\(error)"))
            return false
        }
    }

    /// `held` overrides the measured hold (replay passes the recorded one: compute time must not count as holding).
    public func buttonUp(held explicit: Double? = nil) async {
        if ignoreNextButtonUp { ignoreNextButtonUp = false; buttonDownAt = nil; return }
        let now = env.log.clock.now
        let held = explicit ?? (now - (buttonDownAt ?? now))
        buttonDownAt = nil
        await route(.buttonUp(heldSeconds: held), now: now)
    }

    /// Discard the turn the listener is talking (Cancel while listening). The microphone capture and the words so far are dropped; nothing
    /// is sent to the desk, System 1 or the local model; the programme carries on. WAYS THIS CAN FAIL (each has a test): nothing to
    /// cancel (no event, no change); the release of the same hold arrives afterwards (ignored, it must not start a turn); a turn already
    /// sent (the pipeline is running) is not cancelled here: that is the listener speaking again, which already interrupts it.
    public func cancelTurn() async {
        // A turn already sent: Cancel is the listener's stop. The pipeline (desk, System 1, the local voice) is cancelled with everything it started.
        if phase == .acknowledging || phase == .holding || phase == .revised {
            env.log.emit(.turnCancelled)
            await interruptPipeline()
            await restoreProgramme(reason: "turn cancelled")
            return
        }
        guard turn != nil || buttonDownAt != nil else { return }
        if buttonDownAt != nil { ignoreNextButtonUp = true }
        buttonDownAt = nil
        env.log.emit(.turnCancelled)
        if turn != nil { await dropTurn(reason: nil) }
        else { await bounds?.reset(); setPhase(.playing) }
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
        // The recogniser finishes when it finishes (or throws); the listener's Cancel or speaking again cancels the turn.
        let text: String? = try? await log.timed("transcribe") { try await t.session.finish() }
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

    /// The pipeline was cancelled before it reached its own tail (which resumes the programme and takes the listening session back):
    /// do what the tail would have done. Not part of `interruptPipeline`: a new press or typed turn starts straight into the next turn.
    func restoreProgramme(reason: String) async {
        await setLevelTo(.play, reason: .resume)
        await env.playback.resume()
        do { try await env.session.claim(.listening, preferBuiltInMic: false, reason: reason) } catch {
            env.log.emit(.error(stage: "audio-session", message: "\(error)"))
        }
        if let b = bounds { await b.reset() }
    }

    /// The pipeline and every task it starts live and die together: when the pipeline is cancelled (the listener spoke again)
    /// or ends, the desk, System 1, the relay, the voice and the verdict tasks are cancelled with it, so a revision that
    /// arrives late for an interrupted turn is never applied to the next one.
    func runPipeline(_ t: Turn, transcript: String, endedAt: Double) async {
        let kids = TaskBag()
        await withTaskCancellationHandler {
            await runPipelineBody(t, transcript: transcript, endedAt: endedAt, kids: kids)
        } onCancel: { kids.cancelAll() }
        kids.cancelAll()
    }

    private func runPipelineBody(_ t: Turn, transcript: String, endedAt: Double, kids: TaskBag) async {
        let log = env.log
        let cfg = t.config
        let pos = await env.playback.position()
        let front = t.front ?? env.frontPage()
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
        let cutLocal = Flag()                 // set the moment real desk speech is ready: the local stream stops at a sentence end
        let heard = HeardLines()              // what the phone actually said aloud, handed to the desk source
        var relayTask: Task<Void, Never>?
        var deskTask: Task<Void, Never>?
        var deskSpeakTask: Task<Void, Never>?
        let deskHeard = HeardLines()                                   // what the streamed desk said aloud
        let localPlay = Locked<Task<Void, Never>?>(nil)                // the local stream's player and speaker: the desk's speaker waits for them
        let localSpeakerBox = Locked<LocalSpeaker?>(nil)
        var streaming = false
        if let desk {
            let baseReq = DeskTurn(turnID: t.id, programmeID: pos.programmeID, blockIndex: pos.blockIndex, listenerWords: transcript,
                                 interrupted: t.cut != nil, spokenText: t.cut?.spoken ?? "", cutBlockIndex: t.cut?.blockIndex)
            if let sd = desk as? StreamingDesk {
                // A streamed desk: each sentence is spoken as it arrives, in order, after the local stream's sentence in flight.
                streaming = true
                let (deskOut, deskCont) = AsyncStream.makeStream(of: String.self)
                let pageFetch = env.pageText
                deskTask = Task { [self] in
                    var turnReq = baseReq
                    if let f = front { turnReq.page = DeskPage(address: f.address, title: f.title, text: await pageFetch(f.address)) }
                    var firstSeen = false
                    do {
                        for try await e in sd.speech(turnReq, heard: heard) {
                            if Task.isCancelled { break }
                            switch e {
                            case .sentence(_, let text), .failed(let text, _):
                                if !firstSeen {
                                    firstSeen = true
                                    log.emit(.deskFirstToken(msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
                                    revisionSeen.set(); cutLocal.set(); relayTask?.cancel(); holder?.cancel()    // real desk speech is ready: the local stream stops
                                    await self.markRevised()
                                }
                                deskCont.yield(text)
                            case .done(let status, let n, let a, let b):
                                log.emit(.deskDone(status: status, sentences: n, serverFirstSentenceMs: a, serverDoneMs: b))
                            }
                        }
                    } catch { log.emit(.error(stage: "desk", message: "\(error)")) }
                    deskCont.finish()
                }
                if let d = deskTask { kids.add(d) }
                if let voice {
                    let spD = LocalSpeaker(voice: voice, playback: env.playback, log: log, endedAt: endedAt, heard: deskHeard, cut: Flag(), local: false)
                    deskSpeakTask = Task { [self] in
                        await spD.run(cue: [], sentences: deskOut, before: {
                            await localPlay.value?.value                       // the local sentence in flight finishes, then
                            if let sp = localSpeakerBox.value, sp.sentencesHeard > 0 {
                                log.emit(.transitionPlayed(msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
                                await self.env.playback.speak(LocalSpeaker.stream(NonVerbalCue.handover()))
                            }
                        })
                    }
                    if let d = deskSpeakTask { kids.add(d) }
                }
            } else {
                deskTask = Task { [self] in
                    do {
                        var firstSeen = false
                        for try await r in desk.submit(baseReq, heard: heard) {
                            if Task.isCancelled { break }       // an interrupted turn's revision is not applied
                            if !firstSeen { firstSeen = true; log.emit(.deskFirstToken(msSinceTurnEnd: (log.clock.now - endedAt) * 1000)) }
                            await self.onRevision(r, flag: revisionSeen, endedAt: endedAt, relay: { cutLocal.set(); relayTask?.cancel(); holder?.cancel() })   // real desk speech is ready: the local stream stops
                        }
                    } catch { log.emit(.error(stage: "desk", message: "\(error)")) }
                }
                if let d = deskTask { kids.add(d) }
            }
        }

        // The acknowledgement is asked for BEFORE System 1 starts, so when both want the phone's one language model the
        // acknowledgement goes first (the model announces it synchronously; a decision waits until no acknowledgement is pending).
        let immediate = cfg.ack == .immediate
        let s1Box = Locked<Task<[System1Answer], Never>?>(nil)
        let pageTitle = front?.title ?? "", pageKind = front?.kind ?? ""
        var pageTextSource: (@Sendable () async -> String)?
        if let f = front { let fetch = env.pageText; pageTextSource = { await fetch(f.address) } }
        var earlySrc: AsyncStream<String>?
        if immediate, let holder, voice != nil {
            earlySrc = holder.lines(for: HolderInput(listenerWords: transcript, answers: [], pageTitle: pageTitle, pageKind: pageKind, pageAddress: front?.address ?? "", pageText: pageTextSource),
                                    later: { await s1Box.value?.value ?? [] })
        }

        // System 1 runs as a task beside the acknowledgement; it has no deadline. The acknowledgement and the desk never wait for it (only
        // the `afterSystem1` timing does, by choice), and its time is recorded.
        let s1Task: Task<[System1Answer], Never> = Task {
            guard let system1 else { return [] }
            let input = System1Input(context: pos.context, listenerWords: transcript, questions: pos.questions)
            var got: [System1Answer] = []
            do { got = try await log.timed("system1") { try await system1.answer(input) } }
            catch is CancellationError {}
            catch { log.emit(.error(stage: "system1", message: "\(error)")) }
            for a in got { log.emit(.system1Answer(questionID: a.questionID, option: a.option, score: a.score, ms: a.ms)) }
            return got
        }
        s1Box.value = s1Task
        kids.add(s1Task)
        var answers: [System1Answer] = []
        if !immediate { answers = await s1Task.value }   // afterSystem1: the line is chosen from the answers

        // Acknowledge, then think aloud until the desk speaks.
        var playTask: Task<Void, Never>?
        var verdictTask: Task<Void, Never>?
        var speaker: LocalSpeaker?
        if let holder, let voice {
            let (relayOut, relayCont) = AsyncStream.makeStream(of: String.self)
            // "Heard you": the soft cue plays at once at the end of speech, whatever the model does; the model's sentences follow
            // as they are ready (unless the desk speaks first). No timer decides between them.
            log.emit(.cueStarted(msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
            log.emit(.ackStarted(msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
            let src = earlySrc ?? holder.lines(for: HolderInput(listenerWords: transcript, answers: answers, pageTitle: pageTitle, pageKind: pageKind, pageAddress: front?.address ?? "", pageText: pageTextSource))
            relayTask = Task {
                var said = 0
                for await l in src {
                    said += 1
                    log.emit(.localAckReady(text: l, msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
                    log.emit(.holdingSaid(l)); relayCont.yield(l)
                }
                if let r = holder.report() {
                    log.emit(.localPrefill(promptTokens: r.promptTokens, prefixTokens: r.prefixTokens, reusedTokens: r.reusedTokens, prefillMs: r.prefillMs))
                }
                // No sentence at all (model not loaded, it failed, or the desk spoke first): say why in the trace.
                if said == 0, !Task.isCancelled {
                    let why = holder.diagnostics()
                    log.emit(.localAckSkipped(reason: why.isEmpty ? "no line" : why, msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
                }
                relayCont.finish()
            }
            if let r = relayTask { kids.add(r) }
            let rt = relayTask
            relayCont.onTermination = { _ in rt?.cancel() }
            setPhase(.holding)
            let sp = LocalSpeaker(voice: voice, playback: env.playback, log: log, endedAt: endedAt, heard: heard, cut: cutLocal)
            speaker = sp
            localSpeakerBox.value = sp
            let cue = NonVerbalCue.frames()
            let play = Task { await sp.run(cue: cue, sentences: relayOut) }
            playTask = play
            localPlay.value = play
            kids.add(play)
            if immediate {
                // System 1 selects what follows. If it says the turn needs nothing, the local stream stops at the end of the
                // sentence being heard, and the desk is not engaged.
                let (dt, rl) = (deskTask, relayTask)
                verdictTask = Task {
                    let a = await s1Task.value
                    guard Self.needsNothing(a) else { return }
                    log.emit(.ackCancelled(reason: "System 1: the listener asked for nothing", audible: true))
                    cutLocal.set(); holder.cancel(); rl?.cancel()
                    if let dt { log.emit(.note("desk not engaged: System 1 said nothing was asked")); dt.cancel() }
                }
                if let v = verdictTask { kids.add(v) }
            }
            await withTaskCancellationHandler { await play.value } onCancel: { play.cancel() }
            if cutLocal.isSet {
                log.emit(.localCut(afterSentences: sp.sentencesHeard, reason: revisionSeen.isSet ? "the desk's speech was ready" : "System 1: nothing was asked",
                                   msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
            }
        } else { setPhase(.holding) }
        if immediate, holder == nil || voice == nil {
            let a = await s1Task.value
            if Self.needsNothing(a) { log.emit(.note("desk not engaged: System 1 said nothing was asked")); deskTask?.cancel() }
        }

        // Hold until the desk's stream ends. No timer: the desk ends its stream, fails with a real error, or the listener cancels or
        // speaks again (which cancels this pipeline and everything in it, System 1 included).
        if let deskTask { await deskTask.value }
        if let deskSpeakTask { await deskSpeakTask.value }
        relayTask?.cancel()
        _ = playTask
        if Task.isCancelled { return }

        // The transition: if the phone spoke, a soft falling pair so the switch to the desk's answer sounds intended.
        if !streaming, let sp = speaker, sp.sentencesHeard > 0, revisionSeen.isSet {
            log.emit(.transitionPlayed(msSinceTurnEnd: (log.clock.now - endedAt) * 1000))
            await env.playback.speak(LocalSpeaker.stream(NonVerbalCue.handover()))
        }
        await env.playback.resume()
        // What the radio said aloud this turn goes to the desk's next request as history.
        TurnHistory.shared.add(listener: transcript, radio: heard.snapshot + deskHeard.snapshot)
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

    func markRevised() { setPhase(.revised) }

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

/// Stands in when the transcriber could not start: hears nothing, says nothing.
final class EmptyTranscription: TranscriptionSession, @unchecked Sendable {
    let partials = AsyncStream<String> { $0.finish() }
    func append(_ frame: AudioFrame) {}
    func finish() async throws -> String { "" }
    func cancel() {}
}
