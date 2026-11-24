import Foundation

// Hands-free listening: the microphone is open while a voice session runs, no button. The turn-taking brain is the Mike speech
// engine's `TurnPolicy` (Voice/TurnPolicy.swift, unchanged), driven here by the stages the app already has:
//   speech or silence   the configured SpeechBounds stage (energy VAD): 0.9 while it says speech, 0 otherwise, one input per frame
//   live words          the configured Transcriber's partials, one session per utterance (started at the first speech)
//   end of turn         the policy: silence after speech (no Smart Turn model yet: `fallbackSilenceMs`), then the session's final text
//   cutting in          the policy: two words that are not in the sentence being heard, 300 ms of speech; the answer stops at once
// The back half of the turn (System 1, holder, desk, voice) is `runTurn` / `runPipeline`, unchanged.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in HandsFreeTests):
//  H1 A turn is never sent: the policy commits after `fallbackSilenceMs` of silence, the session's final text goes to `runTurn`.
//  H2 An empty final text (a cough, a door) is sent: it is discarded and the policy goes back to listening; nothing reaches the desk.
//  H3 The radio's own words count as the listener cutting in: words in the sentence being heard (and the one before it) never count.
//  H4 The listener speaks over the radio and it goes on: two new words stop the voice and cancel the pipeline at once; the turn
//     that follows is told what was heard (`interrupted`, `spokenText`).
//  H5 Partials of an earlier utterance count as new words (a session's text is cumulative): the words already cleared are skipped.
//  H6 A session is left open when no turn follows: it is cancelled after silence outside a turn.
//  H7 The answer ends and the policy stays in `thinking`/`speaking`: `runPipeline` tells it (`answerEnded`).
//  H8 No echo cancellation: the policy is told, and then needs three words and ignores the answer's own words.

struct HandsFreeState {
    var policy = TurnPolicy(config: HandsFreeState.voiceConfig)
    var ready = false
    /// The listening settings the stages were built from; a different configuration rebuilds them when no turn is open (review B1).
    var builtBounds: StageSetting?, builtTranscriber: StageSetting?
    var echoCancelled = true
    var bounds: SpeechBounds?
    var transcriber: Transcriber?
    var session: TranscriptionSession?
    var partialTask: Task<Void, Never>?
    var generation = 0
    var nowMs = 0
    var inSpeech = false
    var lastSpeechMs = 0
    /// The session's words that the policy has already cleared (its text is cumulative).
    var wordBase = 0
    var lastWords: [String] = []
    /// The turn being answered, and what the radio has said aloud for it (one list per speaker).
    var answerTurnID: String?
    var heardLists: [HeardLines] = []
    var heardCounts: [Int] = []
    /// What the listener cut into, for the turn that follows a cut-in.
    var cut: SpokenCut?
    /// Hands-free on or off (HandsFreeSwitch). Off: no frame is looked at.
    var enabled = true
    /// After Cancel: no new turn opens until the speech that was running has paused (a TV goes on talking).
    var needQuiet = false
    /// Speech heard in the open session, in ms (what `ListeningVisibility` calls sustained).
    var speechMs = 0
    /// The open session's audio (only while a voice model listens), the verdict on whose voice it is, and whether a check is running.
    var samples: [Float] = []
    var speakerCall: SpeakerCall = .unknown
    var speakerPending = false

    static var voiceConfig: VoiceConfig {
        var c = VoiceConfig.defaults
        // No Smart Turn model yet: silence alone ends the turn, after this long (the default 1.5 s feels slow).
        c.fallbackSilenceMs = 1000
        // Speech that never goes quiet (a TV) cannot hold a turn open: at 30 s the turn closes and is sent as it stands (the policy's
        // `maxTurn` commit; the stock limit is 60 s).
        c.maxTurnMs = 30_000
        return c
    }
}

extension TurnEngine {
    /// Tell the policy whether the microphone has echo cancellation (the audio engine's voice processing is on).
    public func handsFreeConfigure(echoCancelled: Bool) { hf.echoCancelled = echoCancelled; hf.policy.echoCancelled = echoCancelled }

    /// One microphone frame in a voice session.
    public func ingestHandsFree(_ f: AudioFrame) async {
        // Three minutes with no turn from the reader: voice turns itself off, silently (HandsFreeSwitch).
        if HandsFreeSwitch.shared.expireIfIdle() { await handsFreeSet(false) }
        guard hf.enabled else { return }        // off: the microphone is closed, nothing is looked at
        lastFrameT = f.t
        if !hf.ready || hfListenerChanged { await hfPrepare() }
        let startMs = hf.nowMs
        hf.nowMs += max(1, Int((f.duration * 1000).rounded()))
        let endMs = hf.nowMs
        if let s = hf.session { s.append(f); hfKeepAudio(f) } else {
            preroll.append(f)
            var total = preroll.reduce(0.0) { $0 + $1.duration }
            while total > limits.prerollSeconds, preroll.count > 1 { total -= preroll.removeFirst().duration }
        }
        if let bounds = hf.bounds {
            switch await bounds.feed(f) {
            case .speechStarted where hf.needQuiet:
                break                          // speech that was running when the turn was cancelled: not a new turn
            case .speechStarted:
                hf.inSpeech = true
                env.log.emit(.speechStarted)
                await hfBeginSession()
                if pipeline == nil, hf.cut == nil { hf.cut = await env.playback.spoken() }   // a programme line the listener cut into
            case .speechEnded(_, let secs):
                hf.needQuiet = false
                hf.inSpeech = false
                env.log.emit(.speechEnded(seconds: secs))
            case .none: break
            }
        }
        if hf.inSpeech { hf.lastSpeechMs = endMs; if hf.session != nil { hf.speechMs += endMs - startMs } }
        await hfFeed(.vad(probability: hf.inSpeech ? 0.9 : 0, startMs: startMs, endMs: endMs))
        await hfNoteSpokenLines()
        await hfFeed(.tick(nowMs: endMs))
        // A session with no turn behind it (speech that was not for the radio) is dropped after a quiet spell.
        if hf.session != nil, !hf.inSpeech, hf.policy.state != .userSpeaking, hf.policy.state != .confirmingEnd,
           endMs - hf.lastSpeechMs > 3000 {
            hfEndSession(cancel: true)
        }
        hfUpdateVisibility()
    }

    /// The listening state (the bar's "talking") shows only when there is good reason to think the reader has begun saying something
    /// meant for the radio: sustained speech with words (`ListeningVisibility`). Voice activity alone shows nothing.
    func hfUpdateVisibility() {
        let words = max(0, hf.lastWords.count - hf.wordBase)
        // The reader's learned voice: after about a second of speech the turn is judged (off this actor). Another voice never shows the
        // listening state; the reader's own voice is the missing "addressed" evidence once there is a word.
        let gate = env.speakerGate
        if hf.session != nil, hf.speakerCall == .unknown, !hf.speakerPending, hf.speechMs >= 1200, hf.samples.count >= 16_000, gate.learned, gate.canListen {
            hf.speakerPending = true
            let (gen, samples) = (hf.generation, hf.samples)
            Task { [weak self] in let c = await gate.judge(samples); await self?.hfSpeakerCall(c, generation: gen) }
        }
        // Log only in this build (owner, 2026-11-24): the learned voice's verdict is written per turn and nothing is shown or dropped because of it.
        let show = hf.session != nil
            && ListeningVisibility.visible(handsFree: hf.enabled, speechMs: hf.speechMs, words: words, addressed: hf.speakerCall == .match && words >= 1, notATurn: false)
        if show { if pipeline == nil, turn == nil { setPhase(.capturing) } }
        else if phase == .capturing, pipeline == nil, turn == nil { setPhase(.playing) }
    }

    func hfKeepAudio(_ f: AudioFrame) {
        guard env.speakerGate.canListen, f.sampleRate == busSampleRate, hf.samples.count < SpeakerGate.maxSamples else { return }
        hf.samples.append(contentsOf: f.samples.prefix(SpeakerGate.maxSamples - hf.samples.count))
    }

    func hfSpeakerCall(_ c: SpeakerCall, generation: Int) {
        guard generation == hf.generation, hf.session != nil else { return }
        hf.speakerCall = c; hf.speakerPending = false
        if c == .mismatch { hfUpdateVisibility() }
    }

    func hfNewPolicy() -> TurnPolicy {
        var p = TurnPolicy(config: HandsFreeState.voiceConfig)
        p.echoCancelled = hf.echoCancelled
        p.smartTurnAvailable = false      // no Smart Turn model yet: silence alone ends a turn
        p.checksSpeaker = false
        return p
    }

    /// Hands-free on or off, at once, in the middle of a turn too: off drops the open turn and hears nothing more.
    public func handsFreeSet(_ on: Bool) async {
        let was = hf.enabled
        hf.enabled = on
        if !on && was { await hfDropTurn(requireQuiet: false) }
        if on && !was { hf.needQuiet = false; await hf.bounds?.reset() }
    }

    /// Drop the turn being heard (Cancel, or hands-free off). Nothing is sent; the switch is not touched. `requireQuiet`: the speech
    /// that is going on must pause before another turn can open.
    func hfDropTurn(requireQuiet: Bool) async {
        hfEndSession(cancel: true)
        hf.speechMs = 0; hf.inSpeech = false; hf.needQuiet = requireQuiet; hf.cut = nil
        preroll = []
        if hf.answerTurnID == nil { hf.policy = hfNewPolicy() }
        await hf.bounds?.reset()
        await env.playback.setLevel(.play)
        if phase == .capturing, pipeline == nil, turn == nil { setPhase(.playing) }
    }

    /// The configuration names other listening stages than the ones built, and nothing is being heard or answered, so rebuilding drops no turn.
    var hfListenerChanged: Bool {
        hf.session == nil && !hf.inSpeech && hf.answerTurnID == nil
            && (config.setting(.speechBounds) != hf.builtBounds || config.setting(.transcriber) != hf.builtTranscriber)
    }

    func hfPrepare() async {
        hf.ready = true
        hf.builtBounds = config.setting(.speechBounds); hf.builtTranscriber = config.setting(.transcriber)
        // A stage the configuration names that this build cannot make (a model stage in a build without the model libraries, or a name that
        // changed) must never leave the radio deaf: the floor stages, which are always built in, listen instead, and the log says so.
        do { hf.bounds = try env.registry.make(.speechBounds, as: SpeechBounds.self, config) }
        catch {
            env.log.emit(.error(stage: "hands-free", message: "\(error); the level detector listens instead"))
            var floor = config; floor.set(.speechBounds, StageSetting(id: ListenerPipeline.floorBounds, params: ListenerPipeline.boundsParams[ListenerPipeline.floorBounds] ?? [:]))
            hf.bounds = try? env.registry.make(.speechBounds, as: SpeechBounds.self, floor)
        }
        do { hf.transcriber = try env.registry.make(.transcriber, as: Transcriber.self, config) }
        catch {
            env.log.emit(.error(stage: "hands-free", message: "\(error); Apple's recogniser listens instead"))
            var floor = config; floor.set(.transcriber, StageSetting(id: ListenerPipeline.floorTranscriber))
            hf.transcriber = try? env.registry.make(.transcriber, as: Transcriber.self, floor)
        }
        await hf.bounds?.reset()
        env.log.emit(.note(ListenerPipeline.summary(of: config)))
        hf.policy = hfNewPolicy()
    }

    func hfBeginSession() async {
        guard hf.session == nil, let tr = hf.transcriber else { return }
        let s: TranscriptionSession
        do { s = try await env.log.timed("transcriber.begin") { try await tr.begin() } }
        catch { env.log.emit(.error(stage: "transcriber", message: "\(error); this turn has no transcript")); s = EmptyTranscription() }
        // The begin can take a model load; voice may have been turned off, or another turn begun, while it ran: that session is not this turn's.
        guard hf.session == nil, hf.enabled else { s.cancel(); preroll = []; return }
        hf.samples = []; hf.speakerCall = .unknown; hf.speakerPending = false
        for f in preroll { s.append(f); hfKeepAudio(f) }
        preroll = []
        hf.session = s
        hf.generation += 1
        hf.wordBase = 0
        hf.lastWords = []
        let gen = hf.generation
        hf.partialTask = Task { [weak self] in for await p in s.partials { await self?.hfPartial(p, generation: gen) } }
        await hfFeed(.partial(text: ""))
    }

    func hfEndSession(cancel: Bool) {
        hf.partialTask?.cancel(); hf.partialTask = nil
        if cancel { hf.session?.cancel() }
        hf.session = nil
        hf.speechMs = 0
        hf.samples = []; hf.speakerCall = .unknown; hf.speakerPending = false
        hf.generation += 1
        hf.wordBase = 0
        hf.lastWords = []
    }

    func hfPartial(_ text: String, generation: Int) async {
        guard generation == hf.generation, hf.session != nil else { return }
        let words = text.split(whereSeparator: \.isWhitespace).map(String.init)
        hf.lastWords = words
        await hfFeed(.partial(text: words.dropFirst(min(hf.wordBase, words.count)).joined(separator: " ")))
    }

    /// The radio started saying a new sentence: the policy must not take its words for the listener's.
    func hfNoteSpokenLines() async {
        guard let id = hf.answerTurnID else { return }
        for (i, list) in hf.heardLists.enumerated() where i < hf.heardCounts.count {
            let lines = list.snapshot
            if lines.count > hf.heardCounts[i] {
                hf.heardCounts[i] = lines.count
                await hfFeed(.clausePlaying(turnId: id, text: lines[lines.count - 1]))
            }
        }
    }

    func hfWatchHeard(_ lists: [HeardLines]) { hf.heardLists = lists; hf.heardCounts = lists.map { $0.snapshot.count } }

    func hfAnswerEnded(turnID: String) async {
        await hfFeed(.answerEnded(turnId: turnID))
        if hf.answerTurnID == turnID { hf.answerTurnID = nil; hf.heardLists = []; hf.heardCounts = [] }
    }

    func hfFeed(_ input: TurnPolicy.Input) async {
        let decisions = hf.policy.handle(input)
        for d in decisions { await hfApply(d) }
    }

    func hfApply(_ d: TurnPolicy.Decision) async {
        switch d {
        case .state(let s):
            switch s {
            case .userSpeaking: break          // voice activity alone shows nothing (hfUpdateVisibility decides)
            case .listening: if pipeline == nil { setPhase(.playing) }
            default: break
            }
        case .partial(let text):
            if !text.isEmpty { env.log.emit(.partialTranscript(text)) }
        case .commit: await hfCommit()
        case .bargeIn: await hfBargeIn()
        case .pausePlayback:
            print("AUTORADIO voice speech start")
            await env.playback.setLevel(.pause)
            // A pause for speech that comes to no turn is given back: nothing heard for 8 s and the radio plays on.
            Task { [weak self] in
                try? await Task.sleep(nanoseconds: UInt64(TurnEngine.nothingHeardSeconds * 1_000_000_000))
                await self?.hfResumeIfNothingHeard()
            }
        case .resumePlayback: await env.playback.setLevel(.play)
        case .clearTranscript: hf.wordBase = hf.lastWords.count
        case .runSmartTurn, .checkSpeaker: break          // neither model is loaded (H1)
        }
    }

    /// After a pause for speech with no turn coming of it, the radio plays on after this long.
    nonisolated(unsafe) public static var nothingHeardSeconds: Double = 8
    func hfResumeIfNothingHeard() async {
        guard turn == nil, hf.session == nil, pipeline == nil, phase == .playing else { return }
        print("AUTORADIO voice resume why=nothing heard in \(Int(TurnEngine.nothingHeardSeconds)) s")
        await setLevelTo(.play, reason: .resume)
        await env.playback.setLevel(.play)
    }

    /// The policy says the turn is over: the session's final text becomes the turn.
    func hfCommit() async {
        guard let session = hf.session else { await hfFeed(.turnDiscarded); return }
        let (samples, heardCall) = (hf.samples, hf.speakerCall)
        let transcriber = hf.transcriber
        hfEndSession(cancel: false)
        let text = ((try? await env.log.timed("transcribe") { try await session.finish() }) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        env.log.emit(.finalTranscript(text))
        print("AUTORADIO voice heard chars=\(text.count)")
        guard !text.isEmpty, let transcriber, let yield = try? env.registry.make(.yieldPolicy, as: YieldPolicy.self, config) else {
            env.log.emit(.notATurn(reason: "empty transcript"))
            hf.cut = nil
            await setLevelTo(.play, reason: .resume)
            await hfFeed(.turnDiscarded)                                                  // H2
            return
        }
        // Whose voice: with a learned voice, a turn that does not sound like the reader (a TV) is dropped here and never sent.
        var call = heardCall
        let gate = env.speakerGate
        if call == .unknown, gate.learned, gate.canListen { call = await gate.judge(samples) }
        // The verdict is logged for every turn and rejects nothing in this build (owner, 2026-11-24: "the check must only log"); a later build may act on it.
        if gate.learned { env.log.emit(.note("speaker: \(call.rawValue) (\(text.count) chars)")) }
        else { env.log.emit(.note("speaker: no learned voice")) }
        let id = "turn-" + UUID().uuidString.prefix(8).lowercased()
        let t = Turn(id: id, config: config, transcriber: transcriber, yield: yield, session: session, firstEventIndex: env.log.count)
        t.speakerSamples = samples
        t.cut = hf.cut
        hf.cut = nil
        t.addressed = true; t.speechSeen = true
        hf.answerTurnID = id
        HandsFreeSwitch.shared.readerSpoke()            // a turn that was sent: the three minutes start again
        await hfFeed(.turnCommitted(turnId: id, text: text))
        await runTurn(t, transcript: text)
    }

    /// The listener spoke over the radio: its voice stops now, the pipeline of that answer is cancelled, and the next turn knows
    /// what was heard.
    func hfBargeIn() async {
        var cut: SpokenCut?
        let lines = hf.heardLists.flatMap { $0.snapshot }
        if let last = lines.last {
            let pos = await env.playback.position()
            cut = SpokenCut(blockIndex: pos.blockIndex, full: last, spoken: last, fraction: 1)
        }
        hf.cut = cut
        hf.answerTurnID = nil
        hf.heardLists = []; hf.heardCounts = []
        await interruptPipeline()
        await env.playback.stopVoice()
        env.log.emit(.note("hands-free: the listener spoke over the radio"))
    }
}
