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
    private var followRunning = false
    /// How many segments the programme had when the last `extend` was asked: a stretch that added nothing is not asked for again.
    private var extendedAt = -1

    // The DJ (lane autoradio-dj-clips): Play hands the list to the DJ's stretch.
    /// The stretch's source; tests replace it. Default: the programme route on the paper's computer.
    var stretchSource: (@Sendable () -> StretchSource)?
    /// The last request for a stretch found none (route missing, nothing to tell): stories play plain and autoplay goes on as it did.
    private var djUnavailable = false
    /// The list the DJ was handed (the stories it may tell, in the reader's order); an `extend` asks for the same list.
    private var djScope: [String]?
    /// Stories the DJ has already told in this listening.
    private var toldRoutes = Set<String>()
    /// A stretch that arrived after the plain read began: it joins at the next boundary.
    private var pendingStretch: OpenedProgramme?
    private var openTask: Task<OpenedProgramme?, Never>?
    /// How long the reader waits for the stretch's first segment before the story plays plain at once.
    var djFirstSegmentWait: TimeInterval = 8

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
        runtime.player.clipHandler = { id in await DJClipRunner.shared.playClip(id) }
        runtime.player.boundaryHandler = { [weak self] _ in
            self?.noteTold()
            await DJClipRunner.shared.yieldToQueue()
        }
        Task { @MainActor [weak self] in
            DJClipRunner.shared.install()
            DJClipRunner.shared.whenIdle = { [weak self] in await self?.continueIntoDJ() ?? false }
        }
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
            ClipStore.shared.markStale()                          // the front is fresh: the next stretch asks for the clips again, once
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
            if let radio = prog as? RadioProgramme {
                // Play with no story chosen: the current list from the top (the front list when the paper has not composed one).
                let scope = handedRoutes(from: nil)
                lock.lock(); djScope = scope; lock.unlock()
                if let f = stretchSource { let r = RadioProgramme(source: f(), fallback: { try await ChoirFrontProgramme(StageParams()).load() }); let o = try await r.open(routes: scope); doc = o.document; follow = o.follow; lock.lock(); djUnavailable = o.usedFallback; lock.unlock() }
                else { let o = try await radio.open(routes: scope); doc = o.document; follow = o.follow; lock.lock(); djUnavailable = o.usedFallback; lock.unlock() }
                lock.lock(); toldRoutes = []; lock.unlock()
            }
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
        beginList(at: route)
        lock.lock(); toldRoutes = []; lock.unlock()
        await playStack(from: route, reportFailure: true)
    }

    // MARK: Play hands the stack to the DJ

    /// The stories the DJ is handed: the current list from `route` onward (the whole list when nil), without what it already told.
    private func handedRoutes(from route: String?) -> [String]? {
        let order = storyOrder?() ?? []
        lock.lock(); let front = stories.map { StoryRef(route: $0.route, title: $0.title) }; let told = toldRoutes; lock.unlock()
        var list = order.isEmpty ? front : order
        if let route {
            if let i = list.firstIndex(where: { $0.route == route }) { list = Array(list[i...]) }
            else { list = [StoryRef(route: route)] }
        }
        let routes = list.map(\.route).filter { !told.contains($0) || $0 == route }
        return routes.isEmpty ? nil : routes
    }

    private func noteTold() {
        let doc = runtime.player.document
        guard doc.id.hasPrefix("radio:") else { return }
        let r = StretchCandidates.routes(citedIn: doc)
        lock.lock(); toldRoutes.formUnion(r); lock.unlock()
    }

    private func radioForStack(fallbackRoute: String?) -> RadioProgramme {
        let base = frontBaseURL()
        let source: StretchSource = stretchSource?() ?? RouteStretchSource(baseURL: base)
        return RadioProgramme(source: source, fallback: { [weak self] in
            guard let route = fallbackRoute else { return try await ChoirFrontProgramme(StageParams()).load() }
            _ = self
            return try await ChoirFront.load(route: route, client: ChoirEnvironment.shared.client(baseURL: base))
        }, firstSegmentDeadline: 120)
    }

    /// Start the DJ's stretch over the current list from `route` (or its top). The reader hears the stretch's first segment when it comes
    /// within `djFirstSegmentWait`; slower than that, the story plays plain at once and the stretch joins at the next boundary.
    func playStack(from route: String?, reportFailure: Bool = false) async {
        let scope = handedRoutes(from: route)
        let fallbackRoute = route ?? scope?.first
        let (my, old) = { () -> (Int, Task<Void, Never>?) in
            lock.lock(); defer { lock.unlock() }
            loadToken += 1; loading = true; message = ""; pendingAutoplay = nil; pendingStretch = nil
            openTask?.cancel()
            let o = followTask; followTask = nil; followRunning = false
            djScope = scope
            return (loadToken, o)
        }()
        old?.cancel(); onChange?()
        func current() -> Bool { lock.lock(); defer { lock.unlock() }; return my == loadToken }
        let radio = radioForStack(fallbackRoute: fallbackRoute)
        let task = Task { try? await radio.open(cause: "play", routes: scope) }
        lock.lock(); openTask = task; lock.unlock()

        enum Raced { case opened(OpenedProgramme?), slow }
        let shot = OneShot<Raced>()
        Task { shot.fire(.opened(await task.value)) }
        let wait = djFirstSegmentWait
        let timer = Task { try? await Task.sleep(nanoseconds: UInt64(max(0, wait) * 1_000_000_000)); shot.fire(.slow) }
        let raced = await shot.wait()
        timer.cancel()
        guard current() else { return }

        switch raced {
        case .opened(let o?):
            lock.lock(); loading = false; djUnavailable = o.usedFallback; lock.unlock()
            install(o)
            await startPlaying()
            onChange?()
        case .opened(nil), .slow:
            lock.lock(); loading = false; lock.unlock()
            // Plain read at once; the stretch joins later. Nothing to read at all: the failure is said once.
            if let r = fallbackRoute {
                let ok = await startStory(StoryRef(route: r), report: reportFailure)
                if ok { await startPlaying() }
                lock.lock(); let token = loadToken; lock.unlock()
                if case .slow = raced {
                    Task { [weak self] in
                        guard let o = await task.value, !o.usedFallback else { self?.setUnavailable(); return }
                        await self?.stretchArrived(o, token: token)
                    }
                } else { setUnavailable() }
            } else { say("The paper has no stories to read yet."); setUnavailable() }
        }
    }
    private func setUnavailable() { lock.lock(); djUnavailable = true; lock.unlock() }

    /// Make an opened programme the one in the player and follow its stream.
    private func install(_ o: OpenedProgramme) {
        runtime.player.setDocument(o.document)
        runtime.player.begin(at: 0)
        lock.lock(); let my = loadToken; currentRoute = ChoirEnvironment.shared.playing(o.document.id)?.route ?? ""; extendedAt = -1; lock.unlock()
        if let f = o.follow { runFollow(f, loadToken: my, documentID: o.document.id) }
    }

    /// The stretch came after a plain read began. If the story is over (or nothing plays) it starts now; else at the story's end.
    private func stretchArrived(_ o: OpenedProgramme, token: Int) async {
        lock.lock(); guard token == loadToken else { lock.unlock(); return }; pendingStretch = o; lock.unlock()
        if !runtime.player.loopIsActive, !runtime.player.userPaused { _ = await joinPendingStretch() }
    }
    /// True when a waiting stretch took over from the plain read.
    @discardableResult
    private func joinPendingStretch() async -> Bool {
        lock.lock(); let o = pendingStretch; pendingStretch = nil; lock.unlock()
        guard let o else { return false }
        install(o); await startPlaying(); onChange?()
        return true
    }

    /// The end of what the reader played by hand or queued: the DJ goes on over the rest of the list.
    func continueIntoDJ() async -> Bool {
        guard signIn.current.signedIn || stretchSource != nil else { return false }
        await playStack(from: nil)
        return true
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
        if move == .next, await MainActor.run(body: { let r = DJClipRunner.shared; let parked = r.isParked; if parked { r.skipCurrent() }; return parked }) { return }
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
        guard !runtime.player.document.id.hasPrefix("radio:"), let route = currentStoryRoute(), !list(for: route).isEmpty else { return false }
        Task { await moveStory(move) }
        return true
    }

    /// The player ran out of blocks. A programme asks for more; a story starts the next one in its list, unless the reader paused, nothing
    /// was heard, or the list is over; and not over the reader's words (a turn is open): then it waits for the turn to end.
    func ranDry(documentID: String, anyPlayed: Bool) async {
        let p = runtime.player
        guard p.document.id == documentID else { return }                 // a newer document replaced it
        if documentID.hasPrefix("radio:") { await extendIfDry(documentID: documentID); return }
        if !p.userPaused, await joinPendingStretch() { return }          // a stretch that came late joins now
        guard let route = currentStoryRoute() else { return }
        let next = StoryCursor.neighbour(of: route, in: list(for: route), step: 1)
        let turnOpen = runtime.snapshot().phase != "playing"
        switch Autoplay.decide(userPaused: p.userPaused, turnOpen: turnOpen, anyPlayed: anyPlayed, hasNext: next != nil) {
        case .advance: await continueAfter(route)
        case .waitForTurn: lock.lock(); pendingAutoplay = documentID; lock.unlock()
        case .stop: break
        }
    }

    /// A turn ended (the engine is back to playing): if autoplay was waiting for it, ask again.
    func turnEnded() async {
        lock.lock(); let id = pendingAutoplay; pendingAutoplay = nil; lock.unlock()
        let p = runtime.player
        guard let id, p.document.id == id, !p.loopIsActive, !p.userPaused, let route = currentStoryRoute() else { return }
        await continueAfter(route)
    }

    /// A hand-played story ended: the DJ's stretch over the rest of the list; plain autoplay when the paper has no stretch to give.
    private func continueAfter(_ route: String) async {
        lock.lock(); let dj = !djUnavailable; lock.unlock()
        if dj, let n = StoryCursor.neighbour(of: route, in: list(for: route), step: 1) { await playStack(from: n.route); return }
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
        let radio: RadioProgramme
        if let f = stretchSource { radio = RadioProgramme(source: f(), fallback: { throw ChoirError.empty("none") }) }
        else if let r = (try? runtime.registry.make(.programme, as: Programme.self, runtime.configuration)) as? RadioProgramme { radio = r }
        else { return }
        lock.lock(); extendedAt = doc.segments.count; let my = loadToken; lock.unlock()
        lock.lock(); let scope = djScope; lock.unlock()
        guard let follow = await radio.more(excludingRoutes: StretchCandidates.routes(citedIn: doc), documentID: documentID, routes: scope) else { return }
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
        await ensureProgramme()
        if !runtime.snapshot().running { await runtime.start() }
        // A clip is playing through the queue: the button pauses and resumes it, not the (waiting) programme.
        let clip = await MainActor.run { () -> Bool in
            guard DJClipRunner.shared.isParked, DJClipRunner.shared.queue.current?.isRemoteAudio == true else { return false }
            let q = DJClipRunner.shared.queue
            if q.paused { q.resume() } else { q.pause() }
            return true
        }
        if clip { onChange?(); return }
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
