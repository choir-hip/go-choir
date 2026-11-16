import Foundation

extension StageRegistry {
    public static func registerChoirStages(_ r: StageRegistry) {
        r.register(.system1, ChoirSystem1.self) { ChoirSystem1($0) }
        r.register(.programme, ChoirFrontProgramme.self) { ChoirFrontProgramme($0) }
        r.register(.programme, RadioProgramme.self) { RadioProgramme($0) }
        r.register(.desk, ChoirDeskStage.self) { ChoirDeskStage($0) }
        r.register(.desk, AutoradioDeskStage.self) { AutoradioDeskStage($0) }
    }

    /// What the listener's app runs: the real programme, System 1 and desk on the listener's computer, and
    /// the best Apple stages for the rest. The voice is the saved choice, or on a fresh install the catalog's neural default (the speaker
    /// falls back to the system voice while that model is not on the phone, `makeSpeaker`). Stubs and fixtures stay registered and selectable in the Lab tab.
    public static func listenerConfiguration(voice: StageSetting = VoiceCatalog.defaultOption.setting) -> LabConfiguration {
        var c = LabConfiguration(name: "listener", stages: [
            .micSource: .init(id: "builtin"), .addressing: .init(id: "open"), .speechBounds: .init(id: "energy-vad", params: ["hangoverSeconds": .number(0.25)]),
            .speakerCheck: .init(id: "none"), .transcriber: .init(id: "apple-speech"), .yieldPolicy: .init(id: "dip-then-pause"),
            .system1: .init(id: ChoirSystem1.stageID), .holder: .init(id: "local-ack"), .voice: voice,
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
    public var position = ""        // "3 of 12"
    public var playing = false
    public var talking = false      // the listener's turn is in progress
    public var heard = ""           // what the radio heard, while talking
    public var message = ""         // plain words about anything wrong
}
public struct StoryRow: Codable, Sendable { public var title, dek, route, section: String; public var current: Bool }

/// The listener-facing controller: sign-in, the story list, loading the programme, plain-words status.
public final class ChoirApp: @unchecked Sendable {
    public let runtime: LabRuntime
    public let signIn: ChoirSignIn
    private let lock = NSLock()
    private var stories: [ChoirStory] = []
    private var message = ""
    private var loading = false
    private var currentRoute = ""
    private var eventTask: Task<Void, Never>?
    /// Reads the rest of the stretch into the player; cancelled by a reload and by sign-out, so a late segment never lands in a newer programme.
    private var followTask: Task<Void, Never>?
    private var loadToken = 0
    public var onChange: (@Sendable () -> Void)?

    // The list a story came from (player controls): next, previous and autoplay follow it. The host says what the paper's article
    // stack is; without it the front list is used.
    /// The paper's article stack in reading order, as the host last composed it. Empty or nil: not known.
    public var storyOrder: (@Sendable () -> [StoryRef])?
    private var listSnapshot: [StoryRef] = []
    /// The document that ran dry while a turn was open: autoplay waits for the turn to end, then asks again.
    private var pendingAutoplay: String?
    private var pendingQueueEnd: String?
    private var followRunning = false
    /// How many segments the programme had when the last `extend` was asked: a stretch that added nothing is not asked for again.
    private var extendedAt = -1

    public init(runtime: LabRuntime, keys: KeyStore, baseURL: String = ChoirEnvironment.defaultBaseURL, openURL: (@Sendable (URL) -> Void)?) {
        self.runtime = runtime
        ChoirEnvironment.shared.keys = keys
        ChoirEnvironment.shared.log = runtime.log
        signIn = ChoirSignIn(baseURL: baseURL)
        signIn.openURL = openURL
        signIn.onChange = { [weak self] in
            guard let self else { return }
            if self.signIn.current.status == "idle", self.signIn.current.signedIn, self.signIn.current.message == "signed in" { Task { await self.refresh() } }
            self.onChange?()
        }
        runtime.player.onRanDry = { [weak self] id, played in Task { await self?.ranDry(documentID: id, anyPlayed: played) } }
        runtime.player.onNextTrack = { [weak self] in self?.takeRemote(.next) ?? false }
        runtime.player.onPreviousTrack = { [weak self] in self?.takeRemote(.previous) ?? false }
        wireQueue()
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await te in self.runtime.log.stream() {
                if case .error(let stage, let m) = te.event, Self.surfacedErrorStages.contains(stage) {
                    self.say(Self.plain(stage: stage, message: m))
                }
                if case .phase(_, let to) = te.event, to == "playing" { await self.turnEnded() }
            }
        }
    }

    /// The queue's engine and the hooks that feed it. Called once from `init`.
    private func wireQueue() {
        let me = self
        Task { @MainActor in
            let q = PlayQueue.shared
            let engine = ChoirPlaybackEngine(app: me)
            q.engine = engine
            engine.remote.onEnded = { g in Task { @MainActor in await PlayQueue.shared.remoteEnded(generation: g) } }
            engine.remote.onTick = { PlayQueue.shared.save() }
            // Nothing queued behind a text, a page, a book or audio: the DJ's stretch carries on.
            q.defaultSource = { [weak me] in
                guard let me else { return false }
                await me.loadProgramme()
                await me.startPlaying()
                return me.runtime.player.document.blocks.count > 0
            }
            q.restore()
            Station.shared.startDJ = { [weak me] in await me?.startRadio() }       // the entry Play uses
        }
        runtime.player.onTransport = { cmd in
            onMainThread { () -> Bool in
                let q = PlayQueue.shared
                guard q.current?.isRemoteAudio == true else { return false }
                switch cmd {
                case "play": q.resume()
                case "pause": q.pause()
                default: if q.paused { q.resume() } else { q.pause() }
                }
                return true
            }
        }
        runtime.player.onLevel = { l in Task { @MainActor in PlayQueue.shared.setLevel(l) } }
        BarTracker.shared.speedObservers.append { v in Task { @MainActor in PlayQueue.shared.setRate(v) } }
    }

    /// Reading moved: tell the queue which block of the spoken document it is on (the reader saves it), and keep the queue file current.
    public func syncQueuePosition() {
        let id = runtime.player.document.id
        let idx = runtime.player.blockIndex
        Task { @MainActor in
            let q = PlayQueue.shared
            if id == "text:q\(q.generation)" { q.spokenBlock(idx) }
        }
    }

    /// Error stages that reach the player's message. "start" and "voice" are the Play path: a failed engine start and a voice
    /// that could not be made or could not speak.
    static let surfacedErrorStages: Set<String> = ["system1", "desk", "programme", "start", "voice"]

    private func say(_ m: String) { lock.lock(); message = m; lock.unlock(); onChange?() }

    /// Plain words for the listener; the details stay in the Trace.
    static func plain(stage: String, message m: String) -> String {
        if m.contains("not signed in") { return "Sign in to talk to your computer." }
        if m.contains("401") { return "Your sign-in is no longer accepted. Please sign in again." }
        if m.contains("429") { return "Your computer is busy right now. Try again in a moment." }
        if m.contains("network") || m.contains("timed out") { return "I can't reach the paper right now. Check your connection." }
        switch stage {
        case "system1": return "Your computer was slow to understand that; carrying on."
        case "desk": return "Your computer could not act on that. Try asking again."
        case "start": return "The radio could not start its audio. Try Play again."
        case "voice": return "The radio's voice did not work, so nothing could be read aloud."
        default: return "Something went wrong loading the radio."
        }
    }

    /// First run after launch or sign-in: load the story list and the programme.
    public func refresh() async {
        guard signIn.current.signedIn else { onChange?(); return }
        await ChoirEnvironment.shared.warm(baseURL: signIn.baseURL)
        await loadStories()
        await loadProgramme()
    }

    private func frontBaseURL() -> String {
        runtime.configuration.setting(.programme)?.params["baseURL"].map { "\($0)" } ?? signIn.baseURL
    }

    public func loadStories() async {
        let base = frontBaseURL()
        do {
            let list = try await ChoirFront.stories(client: ChoirEnvironment.shared.client(baseURL: base), limit: 30)
            lock.lock(); stories = list; lock.unlock()
            if list.isEmpty { say("The front page is empty right now.") }
        } catch { say(Self.plain(stage: "programme", message: "\(error)")) }
        onChange?()
    }

    /// Load the configured programme stage into the player (the real front page, or a fixture picked in the Lab).
    public func loadProgramme() async {
        // A load token: loads can overlap (the stretch is slow, the listener picks a story). Only the newest one may touch the player; an
        // older one that finishes late drops its result. A new load also ends the follower of the old stretch.
        let (my, old) = { () -> (Int, Task<Void, Never>?) in
            lock.lock(); defer { lock.unlock() }
            loadToken += 1; loading = true; message = ""
            let o = followTask; followTask = nil
            return (loadToken, o)
        }()
        old?.cancel(); onChange?()
        func current() -> Bool { lock.lock(); defer { lock.unlock() }; return my == loadToken }
        defer { lock.lock(); if my == loadToken { loading = false }; lock.unlock(); onChange?() }
        do {
            let prog = try runtime.registry.make(.programme, as: Programme.self, runtime.configuration)
            var doc: ProgrammeDocument
            var follow: (@Sendable (ProgrammePlayer) async -> Void)?
            if let radio = prog as? RadioProgramme { let o = try await radio.open(); doc = o.document; follow = o.follow }
            else { doc = try await prog.load() }
            guard current() else { return }
            runtime.player.setDocument(doc)
            runtime.player.start(at: 0)
            lock.lock(); currentRoute = ChoirEnvironment.shared.playing(doc.id)?.route ?? ""; lock.unlock()
            lock.lock(); extendedAt = -1; lock.unlock()
            if let follow { runFollow(follow, loadToken: my, documentID: doc.id) }
        } catch is CancellationError {
        } catch {
            guard current() else { return }
            runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
            runtime.log.emit(.error(stage: "programme", message: "\(error)"))
            if case ChoirError.empty = error { say("The paper has no stories to read yet.") }
        }
    }

    /// Play a story from the paper. The list it came from (the paper's stack) is taken now, so next, previous and autoplay follow it.
    public func play(route: String) async {
        let title = lock.withLock { stories.first { $0.route == route }?.title } ?? FrontOrder.shared.stories.first { $0.route == route }?.title ?? ""
        await MainActor.run { PlayQueue.shared.playNow(.story(route: route, title: title)) }       // the one queue decides; it comes back to `playStoryDirect`
    }

    /// The queue's story engine: what `play(route:)` was before the queue.
    @discardableResult
    func playStoryDirect(route: String) async -> Bool {
        beginList(at: route)
        if await startStory(StoryRef(route: route), report: true) { await startPlaying(); return true }
        return false
    }

    /// The reader's Listen on a story page: the text plays as its own document; the list is taken for it as for `play(route:)`.
    public func noteListening(address: String) {
        guard address.hasPrefix("pub:") else { return }
        beginList(at: String(address.dropFirst(4)))
    }

    // MARK: Next, previous, autoplay

    public enum StoryMove: Sendable { case next, previous }

    /// The route of the story in the player, when it is one a list can speak for: a `choir:` story or the reader's `text:pub:<route>`.
    private func currentStoryRoute() -> String? {
        let id = runtime.player.document.id
        if let r = ChoirEnvironment.shared.playing(id)?.route, !r.isEmpty { return r }
        return StoryCursor.route(ofTextDocument: id)
    }

    /// Take the list for a story that is starting now: the paper's stack when it holds it, else the front list, else nothing.
    private func beginList(at route: String) {
        let order = storyOrder?() ?? []
        lock.lock(); let front = stories.map { StoryRef(route: $0.route, title: $0.title) }; lock.unlock()
        let pick = order.contains { $0.route == route } ? order : (front.contains { $0.route == route } ? front : [])
        lock.lock(); listSnapshot = pick; lock.unlock()
    }
    /// The list that speaks for `route`: the one taken when it started, else the stack or the front list if they hold it. Never a guess.
    private func list(for route: String) -> [StoryRef] {
        lock.lock(); let snap = listSnapshot; lock.unlock()
        if snap.contains(where: { $0.route == route }) { return snap }
        beginList(at: route)
        lock.lock(); defer { lock.unlock() }
        return listSnapshot
    }

    /// Load one story into the player and start it from its beginning. `report`: say why in the log when it fails (the plain words reach
    /// the player); the autoplay path says its own words instead. A newer load wins, as in `loadProgramme`.
    @discardableResult
    private func startStory(_ ref: StoryRef, report: Bool) async -> Bool {
        let (my, old) = { () -> (Int, Task<Void, Never>?) in
            lock.lock(); defer { lock.unlock() }
            loadToken += 1; loading = true; message = ""; pendingAutoplay = nil
            let o = followTask; followTask = nil; followRunning = false
            return (loadToken, o)
        }()
        old?.cancel(); onChange?()
        func current() -> Bool { lock.lock(); defer { lock.unlock() }; return my == loadToken }
        defer { lock.lock(); if my == loadToken { loading = false }; lock.unlock(); onChange?() }
        do {
            let doc = try await ChoirFront.load(route: ref.route, client: ChoirEnvironment.shared.client(baseURL: frontBaseURL()))
            guard current() else { return true }                 // a newer load took over: nothing failed
            runtime.player.setDocument(doc)
            runtime.player.begin(at: 0)
            lock.lock(); currentRoute = ref.route; lock.unlock()
            await MainActor.run {
                if case .story(let r, _)? = PlayQueue.shared.current, r == ref.route { return }
                PlayQueue.shared.noteStarted(.story(route: ref.route, title: ref.title))      // the paper's list moved on: the queue shows it
            }
            return true
        } catch is CancellationError {
            return true
        } catch {
            guard current() else { return true }
            if report { runtime.log.emit(.error(stage: "programme", message: "\(error)")) }
            return false
        }
    }

    /// Next or previous story from the buttons. In a programme it is the next or previous segment; where no list speaks for the story it is
    /// the next or previous passage.
    public func moveStory(_ move: StoryMove) async {
        let p = runtime.player
        // Next with something queued by hand: that plays (the paper's list and the DJ's stretch come after it).
        if move == .next, await MainActor.run(body: { () -> Bool in let q = PlayQueue.shared; if q.hasUpcoming || q.current?.isRemoteAudio == true { q.skip(); return true }; return false }) { return }
        // The previous of remote audio is its start.
        if move == .previous, await MainActor.run(body: { () -> Bool in let q = PlayQueue.shared; guard q.current?.isRemoteAudio == true else { return false }; q.seek(to: 0); return true }) { return }
        if p.document.id.hasPrefix("radio:") || currentStoryRoute() == nil {
            if move == .next { p.next() } else { p.previous() }
            await startPlaying()
            return
        }
        guard let route = currentStoryRoute() else { return }
        let items = list(for: route)
        switch move {
        case .next:
            guard let n = StoryCursor.neighbour(of: route, in: items, step: 1) else { return }          // the last story: nothing, no wrap
            if await startStory(n, report: false) { await startPlaying() } else { say("Could not load \u{201C}\(n.title.isEmpty ? "that story" : n.title)\u{201D}.") }
        case .previous:
            let pos = p.storyPosition()
            let before = StoryCursor.neighbour(of: route, in: items, step: -1)
            if before == nil || StoryCursor.previousRestartsCurrent(blockIndex: pos.block, secondsIntoBlock: pos.seconds) {
                p.begin(at: 0); await startPlaying()                                                        // restart this one
            } else if let b = before {
                if await startStory(b, report: false) { await startPlaying() } else { say("Could not load \u{201C}\(b.title.isEmpty ? "that story" : b.title)\u{201D}.") }
            }
        }
    }

    /// The lock screen's next or previous: taken as a story move when a list speaks for the story, else left to the player's passage skip.
    private func takeRemote(_ move: StoryMove) -> Bool {
        if onMainThread({ PlayQueue.shared.hasUpcoming || PlayQueue.shared.current?.isRemoteAudio == true }) {
            Task { await moveStory(move) }; return true
        }
        guard !runtime.player.document.id.hasPrefix("radio:"), let route = currentStoryRoute(), !list(for: route).isEmpty else { return false }
        Task { await moveStory(move) }
        return true
    }

    /// The player ran out of blocks. A programme asks for more; a story starts the next one in its list, unless the reader paused, nothing
    /// was heard, or the list is over; and not over the reader's words (a turn is open): then it waits for the turn to end.
    func ranDry(documentID: String, anyPlayed: Bool) async {
        let p = runtime.player
        guard p.document.id == documentID else { return }                 // a newer document replaced it
        // The queue first: what the reader queued by hand plays before the paper's list and before the DJ's stretch.
        let queueText = documentID.hasPrefix("text:q")
        let turnOpenQ = runtime.snapshot().phase != "playing"
        if anyPlayed, !p.userPaused, (queueText || currentStoryRoute() != nil || documentID.hasPrefix("radio:")) {
            if turnOpenQ { lock.lock(); pendingQueueEnd = documentID; lock.unlock(); return }
            if await queueAdvance(documentID: documentID) { return }
        }
        if queueText { return }                                            // a queued text with nothing behind it: the queue's default source decides
        if documentID.hasPrefix("radio:") { await extendIfDry(documentID: documentID); return }
        guard let route = currentStoryRoute() else { return }
        let next = StoryCursor.neighbour(of: route, in: list(for: route), step: 1)
        let turnOpen = runtime.snapshot().phase != "playing"
        switch Autoplay.decide(userPaused: p.userPaused, turnOpen: turnOpen, anyPlayed: anyPlayed, hasNext: next != nil) {
        case .advance: await autoAdvance(from: route)
        case .waitForTurn: lock.lock(); pendingAutoplay = documentID; lock.unlock()
        case .stop: break
        }
    }

    /// The document that ended hands over to the queue. True: the queue started its own next item.
    private func queueAdvance(documentID: String) async -> Bool {
        let route = currentStoryRoute()
        return await PlayQueue.shared.advanceFromEnd(documentID: documentID, route: route)
    }

    /// A turn ended (the engine is back to playing): if autoplay was waiting for it, ask again.
    func turnEnded() async {
        lock.lock(); let qid = pendingQueueEnd; pendingQueueEnd = nil; lock.unlock()
        if let qid, runtime.player.document.id == qid, !runtime.player.loopIsActive, !runtime.player.userPaused, await queueAdvance(documentID: qid) { return }
        lock.lock(); let id = pendingAutoplay; pendingAutoplay = nil; lock.unlock()
        let p = runtime.player
        guard let id, p.document.id == id, !p.loopIsActive, !p.userPaused, let route = currentStoryRoute() else { return }
        await autoAdvance(from: route)
    }

    /// Start the story after `route`. One that fails to load is skipped, once, and the reader is told; a second failure stops.
    private func autoAdvance(from route: String) async {
        let items = list(for: route)
        guard let a = StoryCursor.neighbour(of: route, in: items, step: 1) else { return }
        if await startStory(a, report: false) { await startPlaying(); return }
        let name = a.title.isEmpty ? "A story" : "\u{201C}\(a.title)\u{201D}"
        guard let b = StoryCursor.neighbour(of: a.route, in: items, step: 1) else { say("\(name) could not be loaded. That was the last story."); return }
        if await startStory(b, report: false) { await startPlaying(); say("\(name) could not be loaded; skipped it.") }   // said after the load: a load clears the line
        else { say("The next two stories could not be loaded. Stopping here.") }
    }

    private func runFollow(_ follow: @escaping @Sendable (ProgrammePlayer) async -> Void, loadToken my: Int, documentID: String) {
        let player = runtime.player
        let t = Task { [weak self] in
            await follow(player)
            await self?.followEnded(token: my, documentID: documentID)
        }
        lock.lock(); if my == loadToken { followTask = t; followRunning = true } else { t.cancel() }; lock.unlock()
    }
    private func followEnded(token my: Int, documentID: String) async {
        lock.lock(); let same = my == loadToken; if same { followRunning = false }; lock.unlock()
        guard same else { return }
        await extendIfDry(documentID: documentID)                           // the stream ended after the loop did
    }

    /// The stretch is told and the player has nothing left to say: one more request, leaving out the stories already told. Not while a stretch
    /// is still arriving, not for a reader who paused, and not twice for a programme that did not grow.
    private func extendIfDry(documentID: String) async {
        let p = runtime.player
        guard p.document.id == documentID, !p.loopIsActive, !p.userPaused else { return }
        lock.lock(); let running = followRunning; let at = extendedAt; lock.unlock()
        let doc = p.document
        guard !running, doc.segments.count != at else { return }
        guard let radio = (try? runtime.registry.make(.programme, as: Programme.self, runtime.configuration)) as? RadioProgramme else { return }
        lock.lock(); extendedAt = doc.segments.count; let my = loadToken; lock.unlock()
        guard let follow = await radio.more(excludingRoutes: StretchCandidates.routes(citedIn: doc), documentID: documentID) else { return }
        runFollow(follow, loadToken: my, documentID: documentID)
    }

    /// A programme a listener can hear: a stretch, a story or a text. The player starts with the Lab's fixture edition ("written only to test the
    /// radio") and holds an empty document after sign-out; neither is what Play should speak.
    static func isRealProgramme(_ d: ProgrammeDocument) -> Bool {
        !d.blocks.isEmpty && (d.id.hasPrefix("radio:") || d.id.hasPrefix("choir:") || d.id.hasPrefix("text:"))
    }

    /// Before the radio plays: make sure the paper's programme is what is loaded. The launch load may still be running (wait for it, up to 20
    /// seconds, rather than ask the computer twice); when none has run or it found nothing, load now. Never leaves the fixture edition to play.
    func ensureProgramme() async {
        func loadingNow() -> Bool { lock.lock(); defer { lock.unlock() }; return loading }
        var waited = 0
        while loadingNow(), waited < 400 { try? await Task.sleep(nanoseconds: 50_000_000); waited += 1 }
        if Self.isRealProgramme(runtime.player.document) { return }
        await loadProgramme()
    }

    /// Play/pause from the big button: starts the audio engine on first use.
    public func playPause() async {
        if await MainActor.run(body: { () -> Bool in
            let q = PlayQueue.shared
            guard q.current?.isRemoteAudio == true else { return false }
            if q.paused { q.resume() } else { q.pause() }
            return true
        }) { onChange?(); return }
        await ensureProgramme()
        if !runtime.snapshot().running { await runtime.start() }
        if runtime.player.userPaused { runtime.player.play() } else { runtime.player.pause() }
        onChange?()
    }
    /// Start the radio and leave it playing, whatever it was doing (a toggle would pause it if the interview had left it
    /// running): the setup's "Start the radio". A programme is loaded first when none is.
    public func startRadio() async {
        await ensureProgramme()
        if !runtime.snapshot().running { await runtime.start() }
        runtime.player.play()
        onChange?()
    }
    public func startPlaying() async {
        if !runtime.snapshot().running { await runtime.start() }
        runtime.player.play()
        onChange?()
    }
    /// The lab's talk button: start the engine if it is not running and turn hands-free on.
    public func talkDown() async { if !runtime.snapshot().running { await runtime.start() }; runtime.setHandsFree(true) }

    public func signOut() async {
        await signIn.signOut()
        lock.lock(); loadToken += 1; loading = false; followTask?.cancel(); followTask = nil; lock.unlock()   // a load still in flight is stale now, and will not clear this itself
        runtime.player.pause()
        runtime.player.clearQueue()
        runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        lock.lock(); stories = []; message = ""; lock.unlock()
        onChange?()
    }

    public func playerView() -> ChoirPlayerView {
        let snap = runtime.snapshot()
        let p = runtime.player
        var v = ChoirPlayerView()
        v.signedIn = signIn.current.signedIn
        lock.lock(); v.loading = loading; v.message = message; lock.unlock()
        v.title = p.document.title
        if let si = p.document.segmentIndex(containingBlock: p.blockIndex) { v.title = p.document.segments[si].title }   // a programme: the segment's title
        if p.document.blocks.indices.contains(p.blockIndex) { v.passage = p.document.blocks[p.blockIndex].spokenText ?? p.document.blocks[p.blockIndex].summary }
        v.position = p.document.blocks.isEmpty ? "" : "\(p.blockIndex + 1) of \(p.document.blocks.count)"
        if let si = p.document.segmentIndex(containingBlock: p.blockIndex) { v.position = "\(si + 1) of \(p.document.segments.count)" }   // a programme counts segments, as the bar does
        v.playing = snap.running && !p.userPaused
        v.talking = snap.phase != "playing"
        v.heard = snap.phase == "playing" ? "" : snap.transcript
        if v.message.isEmpty, v.signedIn, !v.loading, p.document.blocks.isEmpty { v.message = "Pick a story below." }
        return v
    }
    public func storyRows() -> [StoryRow] {
        lock.lock(); let s = stories, r = currentRoute; lock.unlock()
        return s.map { StoryRow(title: $0.title, dek: $0.dek, route: $0.route, section: $0.section, current: $0.route == r) }
    }
    public func authView() -> ChoirAuthState { signIn.current }
}
