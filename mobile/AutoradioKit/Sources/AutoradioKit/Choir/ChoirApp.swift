import Foundation

extension StageRegistry {
    public static func registerChoirStages(_ r: StageRegistry) {
        r.register(.system1, ChoirSystem1.self) { ChoirSystem1($0) }
        r.register(.programme, RadioProgramme.self) { RadioProgramme($0) }       // the run sheet decides what plays; this stage loads nothing
        r.register(.desk, ChoirDeskStage.self) { ChoirDeskStage($0) }
        r.register(.holder, SavedAnswerHolder.self) { SavedAnswerHolder($0) }
        r.register(.desk, AutoradioDeskStage.self) { AutoradioDeskStage($0) }
    }

    /// What the listener's app runs: the real programme, System 1 and desk on the listener's computer; the listening stages come from the model list (`ListenerPipeline.choose`): the best ready models, the level detector and Apple's recogniser until then. The voice is the saved choice, or on a fresh install bundled Tiny Voca (the speaker
    /// falls back to it while a downloaded voice's model is not on the phone, `makeSpeaker`). Stubs and fixtures stay registered and selectable in the Lab tab.
    public static func listenerConfiguration(voice: StageSetting = VoiceCatalog.defaultOption.setting, registry: StageRegistry? = nil, readiness: VoiceReadiness = Voices.readiness) -> LabConfiguration {
        let heard = ListenerPipeline.choose(registry: registry, readiness: readiness)
        var c = LabConfiguration(name: "listener", stages: [
            .micSource: .init(id: "builtin"), .addressing: .init(id: "open"), .speechBounds: heard.bounds,
            .speakerCheck: .init(id: "none"), .transcriber: heard.transcriber, .yieldPolicy: .init(id: "dip-then-pause"),
            .system1: .init(id: ChoirSystem1.stageID), .holder: .init(id: SavedAnswerHolder.stageID), .voice: voice,
            .desk: .init(id: AutoradioDeskStage.stageID), .programme: .init(id: RadioProgramme.stageID),
        ])
        c.audioPolicy = .alwaysTalking          // hands-free: voice processing for the whole session, so the radio's voice is cancelled from the microphone
        return c
    }
}
func registerChoirStages(_ r: StageRegistry) { StageRegistry.registerChoirStages(r) }

// MARK: Views for the host (plain value types)

public struct ChoirPlayerView: Codable, Sendable {
    public var signedIn = false
    public var loading = false
    public var title = ""
    public var passage = ""
    public var playing = false
    public var talking = false      // the listener's turn is in progress
    public var heard = ""           // what the radio heard, while talking
    public var message = ""         // plain words about anything wrong
}

/// The listener-facing controller: sign-in, the audio engine for the deck, plain-words status. (The paper's story list and the run sheet are shared code.)
public final class ChoirApp: @unchecked Sendable {
    public let runtime: LabRuntime
    public let signIn: ChoirSignIn
    private let lock = NSLock()
    private var message = ""
    private var loading = false; private var signOutTask: Task<Void, Never>?
    private var eventTask: Task<Void, Never>?
    public var onChange: (@Sendable () -> Void)?
    nonisolated(unsafe) var continuity: PlaybackContinuity?

    public init(runtime: LabRuntime, keys: KeyStore, baseURL: String = ChoirEnvironment.defaultBaseURL) {
        self.runtime = runtime
        LaunchTimings.shared.begin()
        ChoirEnvironment.shared.keys = keys
        ChoirEnvironment.shared.log = runtime.log
        signIn = ChoirSignIn(baseURL: baseURL)
        registerAccountWipe()
        #if canImport(Security)
        let legacySignIn = signIn
        Task.detached { await LegacyKey.retire(store: KeychainKeyStore(service: LegacyKey.service)) { id, key in await legacySignIn.revokeNow(keyID: id, key: key) } }
        #endif
        // The deck is told what its engine says (it journals it; shared code reads the journal): a document that ran out of blocks, an engine lost mid-entry.
        runtime.player.onRanDry = { id, played in Task { @MainActor in Deck.shared.documentEnded(id: id, anyPlayed: played) } }
        runtime.player.onEngineLost = { _, _ in Task { @MainActor in Deck.shared.engineLost() } }
        runtime.coordinator.remoteAudioActive = { onMainThread { Deck.shared.current?.kind == "audio" && Deck.shared.why == .playing } }
        // The lock screen's next and previous act on the deck at once (the held list), so they work with shared code not running; the deck writes them in its journal (`remote`, with where the item was) and shared code reconciles the sheet when it runs. Always handled here.
        runtime.player.onNextTrack = { Task { @MainActor in await Deck.shared.remoteNext() }; return true }
        runtime.player.onPreviousTrack = { Task { @MainActor in Deck.shared.remotePrevious() }; return true }
        wireRunSheet()
        warmVoice()
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await te in self.runtime.log.stream() {
                if case .error(let stage, let m) = te.event, Self.surfacedErrorStages.contains(stage) {
                    self.say(Self.plain(stage: stage, message: m))
                }
                if case .phase(_, let to) = te.event, to == "playing" { RadioBridge.shared.send("turnEnded") }
            }
        }
    }

    /// The deck's engine and the hooks that feed it. Called once from `init`. (The run sheet and its decisions are shared code; the deck plays what it is handed.)
    private func wireRunSheet() {
        let me = self
        Task { @MainActor in
            let engine = ChoirPlaybackEngine(app: me)
            Deck.shared.engine = engine
            Deck.shared.sessionReady = { [weak me] reason in await me?.ensureEngine(reason: reason) }
            let continuity = PlaybackContinuity(runtime: me.runtime)
            engine.continuity = continuity
            me.continuity = continuity
            me.runtime.audio.onVoiceHold = { [weak engine] held in Task { @MainActor in engine?.remote.setHeld(held) } }
            engine.remote.onSystemPause = { [weak continuity] in continuity?.remoteSystemPaused() }
            engine.remote.note = { [weak me] in me?.runtime.log.emit(.audioNotice(kind: "remote", detail: $0)) }
            engine.remote.onEnded = { _ in Task { @MainActor in Deck.shared.remoteEnded() } }
            Deck.shared.boot()                             // an item the kill left open is closed in the journal first
        }
        // The lock screen's and the headset's play, pause and toggle are the deck's.
        runtime.player.onTransport = { cmd in
            onMainThread {
                switch cmd {
                case "play": Task { await Deck.shared.resumed() }
                case "pause": Deck.shared.listenerPaused()
                default: if Deck.shared.why == .playing { Deck.shared.listenerPaused() } else { Task { await Deck.shared.resumed() } }
                }
                return true
            }
        }
        // The voice's yield (a barge-in pauses the radio, the end of the turn plays it again) acts on the deck at once, here; the deck's journal tells shared code (the `level` verb that also told it was ignored there, and woke its pump on every duck: removed).
        runtime.player.onLevel = { l in
            Task { @MainActor in
                switch l { case .pause: Deck.shared.listenerPaused(); case .play: await Deck.shared.resumed(); case .dip: break }
            }
        }
        BarTracker.shared.speedObservers.append { v in Task { @MainActor in Deck.shared.engine?.setRemoteRate(v) } }
    }

    /// Error stages that reach the player's message. "start" and "voice" are the Play path: a failed engine start and a voice
    /// that could not be made or could not speak.
    static let surfacedErrorStages: Set<String> = ["system1", "desk", "programme", "start", "voice", "listen"]

    func say(_ m: String) { lock.lock(); message = m; lock.unlock(); onChange?() }

    /// Plain words for the listener; the details stay in the Trace.
    static func plain(stage: String, message m: String) -> String {
        if m.contains("not signed in") { return "Sign in to talk to your computer." }
        if m.contains("401") { return "Your sign-in is no longer accepted. Please sign in again." }
        if m.contains("429") { return "Your computer is busy right now. Try again in a moment." }
        if m.contains("network") || m.contains("timed out") { return "I can't reach the paper right now. Check your connection." }
        switch stage {
        case "system1": return "Your computer was slow to understand that; carrying on."
        case "desk": return "Your computer could not act on that. Try asking again."
        case "start": return "The radio could not start its audio (\(m)). Try Play again."
        case "voice": return "The radio's voice did not work, so nothing could be read aloud."
        case "listen": return "Can\u{2019}t listen: \(m)"
        default: return "Something went wrong with the radio."
        }
    }

    /// After launch or sign-in: the computer's connection is warmed. Nothing plays here, and the paper's story list is shared code's (radio-bridge.ts).
    public func refresh() async {
        guard signIn.signedIn else { onChange?(); return }
        await ChoirEnvironment.shared.warm(baseURL: signIn.baseURL)
    }

    /// Load the configured programme stage into the player when it is a document of its own (the setup interview's, a fixture picked in the
    /// Lab). The default stage is the run sheet, which loads nothing here.
    public func loadProgramme() async {
        lock.lock(); loading = true; message = ""; lock.unlock(); onChange?()
        defer { lock.lock(); loading = false; lock.unlock(); onChange?() }
        do {
            let prog = try runtime.registry.make(.programme, as: Programme.self, runtime.configuration)
            if prog is RadioProgramme { return }
            let doc = try await prog.load()
            runtime.player.setDocument(doc)
            runtime.player.start(at: 0)
        } catch is CancellationError {
        } catch {
            runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
            runtime.log.emit(.error(stage: "programme", message: "\(error)"))
            if case ChoirError.empty = error { say("The paper has no stories to read yet.") }
        }
    }

    // MARK: What the engine does for the run sheet

    /// Speak words as a document of one segment, from the start. The segment is named by what it is, not by a run sheet entry (the deck's item).
    func speakSegment(documentID: String, entryID: String, title: String, kind: String, cites: [ProgrammeCite] = [], paragraphs: [String]) async -> Bool {
        let incoming = IncomingSegment(stretchID: "rs", segmentID: entryID, kind: kind.isEmpty ? "story" : kind, title: title, text: paragraphs.joined(separator: "\n\n"), cites: cites, blockCount: nil)
        guard let prepared = SegmentPrep.prepare(incoming) else { return false }
        var doc = ProgrammeDocument(id: documentID, title: title.isEmpty ? "Autoradio" : title, blocks: [])
        doc.append(prepared)
        runtime.player.setDocument(doc)
        runtime.player.begin(at: 0)
        onChange?()
        return true
    }

    /// The audio engine is running before anything is spoken. The host's start request is used when it has one (under a simulator agent
    /// the engine must be the virtual one).
    /// Already running: the audio session is made ready (`ensureReady`: claimed, active, the engine up) because it may be dead after the background.
    /// Nil when sound can come out; else the plain reason it cannot.
    @discardableResult
    func ensureEngine(reason: String = "play") async -> String? {
        if runtime.snapshot().running {
            let ok = await runtime.ensureSessionReady(reason: reason)
            LaunchTimings.shared.mark("engine")
            if ok { LaunchTimings.shared.mark("session") }
            return ok ? nil : "the audio session could not be taken"
        }
        if let go = runtime.startRequest { await go() } else { await runtime.start() }
        let s = runtime.snapshot()
        if s.running { LaunchTimings.shared.mark("engine"); LaunchTimings.shared.mark("session") }
        return s.running ? nil : (s.message.isEmpty ? "the audio engine did not start" : s.message)
    }

    /// The voice is made ready in parallel with the engine start: one short line through the real voice path, so its model is loaded and
    /// compiled before the first block asks (the time is logged; it is not the launch line's `voice_load_ms`, which is what the first block waited).
    public func warmVoice() {
        let p = runtime.player
        Task.detached {
            let t0 = Date()
            let ok = await p.warmVoice()
            print("AUTORADIO voice warm \(ok ? "ok" : "failed") in \(Int(Date().timeIntervalSince(t0) * 1000)) ms")
        }
    }

    /// Sign-out, in this order: the gate shuts and the voice closes (microphone, turn, speech); playback stops; the run sheet gets up to five seconds to post its pending ops with the old key still held (`RadioBridge.waitSignedOut`);
    /// what is not answered is dropped, never sent under a later account; then the key goes and the wipe runs; the server revoke is shared code's. A kill leaves: before the key, signed in with the microphone off (hands-free is never kept across a launch); after it, signed out and shared code's marker finishes the wipe.
    public func signOut() async {
        lock.lock(); let t = signOutTask ?? Task { await self.signOutOnce(); self.lock.lock(); self.signOutTask = nil; self.lock.unlock() }; signOutTask = t; lock.unlock(); await t.value      // a second call joins the first: its `stopAll` would reset the first's wait
    }
    private func signOutOnce() async {
        await runtime.closeVoiceForSignOut()                     // first, before the wait and the key: the gate shuts, the microphone closes, the turn is cancelled; the old key is still held for the post below
        lock.lock(); loading = false; lock.unlock(); runtime.player.pause(reason: "sign out")
        runtime.player.clearQueue()
        runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        await MainActor.run { Deck.shared.engine?.stopSpoken(); Deck.shared.engine?.stopRemote() }
        RadioBridge.shared.send("stopAll")                      // shared code's run sheet leaves the air (`left`) and posts, then the account wipe runs
        await RadioBridge.shared.waitSignedOut(seconds: 5)      // until shared code says it is done (its own wait for the route is 3 s), with the old key still held
        await signIn.signOut()
        if !signIn.signedIn { TurnGate.shared.keyDeleted() }    // only once the key is truly gone: a key present after this is a new sign-in's and opens the gate
        lock.lock(); message = ""; lock.unlock(); onChange?()
    }

    /// Every store of the account, wiped by one call at sign-out. The module adds the stores that live in the app target (mail).
    public let accountWipe = AccountWipe()
    private func registerAccountWipe() {
        let kept = runtime.store
        // Registered at once (not in a later task): a sign-out before the main actor ran a task would wipe nothing.
        onMainThread { [accountWipe] in accountWipe.registerStandard(keptTurns: kept) }
        signIn.onSignOut = { [accountWipe] in await accountWipe.run() }
    }

    public func playerView() -> ChoirPlayerView {
        let snap = runtime.snapshot()
        let p = runtime.player
        var v = ChoirPlayerView()
        v.signedIn = signIn.signedIn
        lock.lock(); v.loading = loading; v.message = message; lock.unlock()
        let onAir = onMainThread { Deck.shared.current != nil || RadioBridge.shared.hasOnAir() }
        v.title = p.document.title
        if let si = p.document.segmentIndex(containingBlock: p.blockIndex) { v.title = p.document.segments[si].title }   // the entry's title
        if p.document.blocks.indices.contains(p.blockIndex) { v.passage = p.document.blocks[p.blockIndex].spokenText ?? p.document.blocks[p.blockIndex].summary }
        v.playing = onAir && snap.running && !p.userPaused          // nothing on air is never "playing"
        if !onAir { v.title = ""; v.passage = "" }
        v.talking = snap.phase != "playing"
        v.heard = snap.phase == "playing" ? "" : snap.transcript
        return v
    }
}
