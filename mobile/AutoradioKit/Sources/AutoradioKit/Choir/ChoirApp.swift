import Foundation

extension StageRegistry {
    public static func registerChoirStages(_ r: StageRegistry) {
        r.register(.system1, ChoirSystem1.self) { ChoirSystem1($0) }
        r.register(.programme, RadioProgramme.self) { RadioProgramme($0) }       // the run sheet decides what plays; this stage loads nothing
        r.register(.desk, ChoirDeskStage.self) { ChoirDeskStage($0) }
        r.register(.holder, SavedAnswerHolder.self) { SavedAnswerHolder($0) }
        r.register(.desk, AutoradioDeskStage.self) { AutoradioDeskStage($0) }
    }

    /// What the listener's app runs: the real programme, System 1 and desk on the listener's computer, and
    /// the best Apple stages for the rest. The voice is the saved choice, or on a fresh install the catalog's neural default (the speaker
    /// falls back to the system voice while that model is not on the phone, `makeSpeaker`). Stubs and fixtures stay registered and selectable in the Lab tab.
    public static func listenerConfiguration(voice: StageSetting = VoiceCatalog.defaultOption.setting) -> LabConfiguration {
        var c = LabConfiguration(name: "listener", stages: [
            .micSource: .init(id: "builtin"), .addressing: .init(id: "open"), .speechBounds: .init(id: "energy-vad", params: ["hangoverSeconds": .number(0.25)]),
            .speakerCheck: .init(id: "none"), .transcriber: .init(id: "apple-speech"), .yieldPolicy: .init(id: "dip-then-pause"),
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
public struct StoryRow: Codable, Sendable { public var title, dek, route, section: String; public var current: Bool }

/// The phone's own opening: what it says first, every time, built from the front page it already holds (titles and deks). It needs no model, no
/// network and no deadline; it is the way the radio starts, not a fallback.
public enum PhoneOpening {
    public struct Line: Codable, Sendable, Equatable { public var title: String; public var dek: String
        public init(title: String, dek: String = "") { self.title = title; self.dek = dek } }

    static func sentence(_ s: String) -> String {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let last = t.last else { return "" }
        return ".!?".contains(last) ? t : t + "."
    }
    /// The app's one day-part word: morning from 05:00, afternoon from 12:00, evening from 18:00, and plain "Hello" from midnight to 05:00
    /// (evening ends at midnight). The opening and the Autoradio page both use it.
    public static func hello(_ now: Date, calendar: Calendar = .current) -> String {
        let h = calendar.component(.hour, from: now)
        return h < 5 ? "Hello" : h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"
    }
    public static func greeting(_ now: Date, calendar: Calendar = .current) -> String { hello(now, calendar: calendar) + "." }
    /// The first sentence of a dek, with no closing mark; "" when it only says the headline again.
    static func lead(dek: String, title: String) -> String {
        guard let first = BarTracker.sentences(dek.trimmingCharacters(in: .whitespacesAndNewlines)).first else { return "" }
        let d = first.trimmingCharacters(in: CharacterSet(charactersIn: ".!? \n"))
        let t = title.trimmingCharacters(in: CharacterSet(charactersIn: ".!? \n")).lowercased()
        if d.isEmpty || d.lowercased().hasPrefix(t) || t.hasPrefix(d.lowercased()) { return "" }
        return d
    }
    /// A signpost: the greeting, the first story with its dek once, then at most two more headlines by name. One story, two, many: each reads as
    /// ordinary sentences ("A. Dek. Also: B; and C.") and no dek is read twice or in full.
    public static func entry(lines: [Line], now: Date = Date(), calendar: Calendar = .current) -> RunSheetEntry {
        var seen = Set<String>()
        let named = lines.filter { l in
            let t = l.title.trimmingCharacters(in: .whitespacesAndNewlines)
            return !t.isEmpty && seen.insert(t.lowercased()).inserted
        }
        var text = greeting(now, calendar: calendar) + " This is Autoradio."
        if let first = named.first {
            text += " From the paper's front page: " + sentence(first.title)
            let dek = lead(dek: first.dek, title: first.title)
            if !dek.isEmpty { text += " " + sentence(dek) }
            let rest = named.dropFirst().prefix(2).map { $0.title.trimmingCharacters(in: CharacterSet(charactersIn: ".!? \n")) }
            if !rest.isEmpty { text += " Also: " + rest.joined(separator: "; and ") + "." }
        } else {
            text += " I don't have the front page yet."
        }
        text = StretchCandidates.cut(text, bytes: RunSheetBounds.maxTextBytes)
        return RunSheetEntry(id: RunSheetEntry.mintId("p-"), kind: .opening, placedBy: .phone, title: "Opening", text: text, estSeconds: RunSheetEntry.estimateSeconds(words: text))
    }
}

/// The listener-facing controller: sign-in, the story list, the run sheet's opening, plain-words status.
public final class ChoirApp: @unchecked Sendable {
    public let runtime: LabRuntime
    public let signIn: ChoirSignIn
    private let lock = NSLock()
    private var stories: [ChoirStory] = []
    private var message = ""
    private var loading = false
    private var eventTask: Task<Void, Never>?
    public var onChange: (@Sendable () -> Void)?
    nonisolated(unsafe) var continuity: PlaybackContinuity?

    /// The stretch's source; tests replace it. Default: the programme route on the paper's computer.
    var stretchSource: (@Sendable () -> StretchSource)?
    /// Where the front page's titles are kept between launches, so the opening can be built with no network.
    var frontCache: URL?
    /// The writer of the desk's entries (made on the main actor with the run sheet's player).
    @MainActor var writer: RunSheetWriter?
    /// The opening written and spoken ahead of the launch (kept in Caches), and what writes it.
    let openingCache = OpeningCache(dir: OpeningCache.defaultDirectory() ?? FileManager.default.temporaryDirectory.appendingPathComponent("AutoradioOpening"))
    @MainActor var writeAhead: OpeningWriteAhead?
    @MainActor private var askedAfterOpening = false

    public init(runtime: LabRuntime, keys: KeyStore, baseURL: String = ChoirEnvironment.defaultBaseURL, openURL: (@Sendable (URL) -> Void)?) {
        self.runtime = runtime
        LaunchTimings.shared.begin()
        ChoirEnvironment.shared.keys = keys
        ChoirEnvironment.shared.log = runtime.log
        signIn = ChoirSignIn(baseURL: baseURL)
        signIn.openURL = openURL
        frontCache = Self.defaultFrontCache()
        registerAccountWipe()
        signIn.onChange = { [weak self] in
            guard let self else { return }
            if self.signIn.current.status == "idle", self.signIn.current.signedIn, self.signIn.current.message == "signed in" {
                // The run sheet's poster was dropped at sign-out: it is rebuilt for this sign-in.
                Task { @MainActor in
                    if RunSheetStore.shared.poster == nil { RunSheetStore.shared.poster = RouteSheetPoster(baseURL: self.signIn.baseURL) }
                    RunSheetStore.shared.flush()
                }
                Task { await self.refresh() }
            }
            self.onChange?()
        }
        runtime.player.onRanDry = { id, played in Task { @MainActor in RunSheetPlayer.shared.documentEnded(id: id, anyPlayed: played) } }
        runtime.player.onEngineLost = { id, n in Task { @MainActor in RunSheetPlayer.shared.engineLost(documentID: id, block: n) } }
        runtime.coordinator.remoteAudioActive = { onMainThread { RunSheetPlayer.shared.isRemote && !RunSheetPlayer.shared.paused } }
        // The lock screen's next and previous are the run sheet's: `skip(on air)` and `previous` (ChoirApp.moveStory). Always handled here.
        runtime.player.onNextTrack = { [weak self] in self?.takeRemote(.next) ?? false }
        runtime.player.onPreviousTrack = { [weak self] in self?.takeRemote(.previous) ?? false }
        wireRunSheet()
        warmVoice()
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await te in self.runtime.log.stream() {
                if case .error(let stage, let m) = te.event, Self.surfacedErrorStages.contains(stage) {
                    self.say(Self.plain(stage: stage, message: m))
                }
                if case .phase(_, let to) = te.event, to == "playing" { await MainActor.run { RunSheetPlayer.shared.turnEnded() } }
            }
        }
    }

    /// The run sheet's engine and the hooks that feed it. Called once from `init`.
    private func wireRunSheet() {
        let me = self
        Task { @MainActor in
            let p = RunSheetPlayer.shared
            let engine = ChoirPlaybackEngine(app: me)
            p.engine = engine
            Deck.shared.engine = engine
            Deck.shared.sessionReady = { [weak me] reason in await me?.ensureEngine(reason: reason) }
            p.sessionReady = { [weak me] reason in await me?.ensureEngine(reason: reason) }       // before every start of playback
            let continuity = PlaybackContinuity(runtime: me.runtime)
            engine.continuity = continuity
            me.continuity = continuity
            me.runtime.audio.onVoiceHold = { [weak engine] held in Task { @MainActor in engine?.remote.setHeld(held) } }
            engine.remote.onSystemPause = { [weak continuity] in continuity?.remoteSystemPaused() }
            engine.remote.note = { [weak me] in me?.runtime.log.emit(.audioNotice(kind: "remote", detail: $0)) }
            engine.remote.onEnded = { g in Task { @MainActor in RunSheetPlayer.shared.remoteEnded(generation: g) } }
            engine.remote.onTick = { RunSheetStore.shared.save() }
            p.turnOpen = { [weak me] in me.map { $0.runtime.snapshot().phase != "playing" } ?? false }
            p.spokenPaused = { [weak me] in me?.runtime.player.userPaused ?? false }
            p.spokenPausedByListener = { [weak me] in me?.runtime.player.pausedByListener ?? false }
            let w = RunSheetWriter(store: .shared) { [weak me] in
                guard let me else { return nil }
                return ProgrammeRouteFeed(source: me.stretchSource?() ?? RouteStretchSource(baseURL: me.frontBaseURL()))
            }
            me.writer = w
            me.accountWipe.register("writer") { await MainActor.run { w.stop() } }
            w.offlineFill = { LocalStories.entries(from: .shared, sheet: .shared) }       // the desk cannot be reached: stories held on the phone
            let wa = OpeningWriteAhead(cache: me.openingCache, store: .shared,
                                       feed: { [weak me] in me.map { ProgrammeRouteFeed(source: $0.stretchSource?() ?? RouteStretchSource(baseURL: $0.frontBaseURL())) } },
                                       voice: { [weak me] in me?.runtime.player.voiceKey ?? "" },
                                       synth: { [weak me] t in await me?.runtime.player.synthesizeForCache(t) },
                                       busy: { [weak w] in w?.inFlight ?? false })
            me.writeAhead = wa
            me.runtime.player.preparedSpeech = { [cache = me.openingCache] t in cache.prepared(text: t) }
            p.onEntryEnded = { [weak me] e, played in
                guard let me, e.kind == .opening, played, !me.askedAfterOpening else { return }
                me.askedAfterOpening = true
                Task { @MainActor in await me.writeAhead?.refresh(reason: "opening aired") }
            }
            w.admit = { op, cause in RunSheetPlayer.shared.admitDesk(op, cause: cause) }
            w.onNotice = { n in RunSheetPlayer.shared.deskNotice = n }
            w.onDry = { d in RunSheetPlayer.shared.deskDry = d }
            w.onWriting = { on in RunSheetPlayer.shared.deskWriting = on }
            // An ask ended (never sign-out's stop): a refused write-ahead runs now; a missing kept first entry is kept.
            w.onIdle = { [weak me] in Task { @MainActor in await me?.writeAhead?.writerIdle() } }
            p.aheadChanged = { [weak w] in w?.checkAhead() }
            RunSheetStore.shared.poster = RouteSheetPoster(baseURL: me.signIn.baseURL)
            Deck.shared.boot()                             // an item the kill left open is closed in the journal first
            RunSheetStore.shared.restore(journal: Deck.shared.journal)       // the pending ops of the last launch; nothing plays
            Station.shared.startDJ = { [weak me] in await me?.startRadio() }       // the entry Play uses
        }
        runtime.player.onTransport = { cmd in
            onMainThread { RunSheetPlayer.shared.remoteTransport(cmd) }
        }
        runtime.player.onLevel = { l in Task { @MainActor in RunSheetPlayer.shared.setLevel(l) } }
        BarTracker.shared.speedObservers.append { v in Task { @MainActor in RunSheetPlayer.shared.setRate(v) } }
    }

    /// Reading moved: tell the run sheet which block of the book or page it is on (the reader saves it).
    public func syncQueuePosition() {
        let id = runtime.player.document.id
        let idx = runtime.player.blockIndex
        Task { @MainActor in
            let p = RunSheetPlayer.shared
            if let doc = p.playingDocument, doc == id { p.spokenBlock(idx) }
            p.checkSound()                                  // the deck's evidence that sound is being rendered
        }
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

    /// First run after launch or sign-in: load the story list (the opening is built from it). Nothing plays here.
    public func refresh() async {
        guard signIn.current.signedIn else { onChange?(); return }
        await ChoirEnvironment.shared.warm(baseURL: signIn.baseURL)
        await loadStories()
    }

    private func frontBaseURL() -> String {
        runtime.configuration.setting(.programme)?.params["baseURL"].map { "\($0)" } ?? signIn.baseURL
    }

    public func loadStories() async {
        let base = frontBaseURL()
        lock.lock(); loading = true; lock.unlock(); onChange?()
        defer { lock.lock(); loading = false; lock.unlock(); onChange?() }
        do {
            let list = try await ChoirFront.stories(client: ChoirEnvironment.shared.client(baseURL: base), limit: 30)
            lock.lock(); stories = list; lock.unlock()
            saveFrontCache(list)
            // The headlines and deks the phone now holds are part of what it can answer from first.
            for s in list { StoryDates.shared.note("pub:" + s.route, PaperStory.date(s.updated)) }
            SavedCorpus.shared.setSource("front") { list.filter { !$0.dek.isEmpty }.map { SavedCorpus.Document(id: "front:" + $0.route, title: $0.title, address: "pub:" + $0.route, text: $0.dek, kind: "story", source: "Autopaper", date: PaperStory.date($0.updated)) } }
            ContentSync.kick()          // stories, mail, documents, files into the phone's own store, in the background
            ClipStore.shared.markStale()                          // the front is fresh: the next write asks for the clips again, once
            if list.isEmpty { say("The front page is empty right now.") }
        } catch { say(Self.plain(stage: "programme", message: "\(error)")) }
    }

    // MARK: The front page, kept for the opening

    static func defaultFrontCache() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("front-titles.json")
    }
    private func saveFrontCache(_ list: [ChoirStory]) {
        guard let f = frontCache, !list.isEmpty else { return }
        let lines = list.prefix(10).map { PhoneOpening.Line(title: $0.title, dek: $0.dek) }
        if let d = try? JSONEncoder().encode(lines) { try? d.write(to: f, options: .atomic) }
    }
    /// The front page the phone holds: this launch's, else the last one kept on the phone.
    func frontLines() -> [PhoneOpening.Line] {
        lock.lock(); let held = stories; lock.unlock()
        if !held.isEmpty { return held.prefix(10).map { PhoneOpening.Line(title: $0.title, dek: $0.dek) } }
        guard let f = frontCache, let d = try? Data(contentsOf: f), let l = try? JSONDecoder().decode([PhoneOpening.Line].self, from: d) else { return [] }
        return l
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

    /// Speak an entry's words as a document of one segment, and play it from the start.
    func speakEntry(documentID: String, entry: RunSheetEntry, paragraphs: [String]) async -> Bool {
        let kind: String
        switch entry.kind {
        case .opening: kind = "opening"
        case .transition: kind = "transition"
        case .answer: kind = "answer"
        case .result: kind = "result"
        case .mail: kind = "mail"
        default: kind = "story"
        }
        let cites = (entry.cites ?? []).map { ProgrammeCite(ref: $0.ref, role: $0.role.rawValue, label: $0.label) }
        return await speakSegment(documentID: documentID, entryID: entry.id, title: entry.title, kind: kind, cites: cites, paragraphs: paragraphs)
    }

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

    /// Load a paper story in full and play it. The document's id; nil when the story could not be loaded.
    func playPaperStory(route: String) async -> String? {
        do {
            let doc = try await ChoirFront.load(route: route, client: ChoirEnvironment.shared.client(baseURL: frontBaseURL()))
            runtime.player.setDocument(doc)
            runtime.player.begin(at: 0)
            onChange?()
            return doc.id
        } catch is CancellationError { return nil }
        catch {
            runtime.log.emit(.error(stage: "programme", message: "\(error)"))
            return nil
        }
    }

    // MARK: Play, next, previous

    /// Play this story now (a row, a page's Play): `place now` on the run sheet. What was ahead stays ahead.
    public func play(route: String, title: String = "") async {
        let heading = title.isEmpty ? (lock.withLock { stories.first { $0.route == route }?.title } ?? "") : title      // the page names the story; the radio's list is the fallback
        await MainActor.run { RunSheetPlayer.shared.playNow(.paperStory(route: route, title: heading)) }
        await RunSheetPlayer.shared.settle()
    }

    public enum StoryMove: Sendable { case next, previous }

    /// The next and previous buttons, the card's swipe, the lock screen and "next" by voice: `skip(on air)` and `previous` on the run sheet.
    public func moveStory(_ move: StoryMove) async {
        await MainActor.run { if move == .next { RunSheetPlayer.shared.next() } else { RunSheetPlayer.shared.previous() } }
    }

    /// The lock screen's next or previous (the remote-command routing: nothing else about the lock screen is touched here).
    private func takeRemote(_ move: StoryMove) -> Bool {
        Task { await moveStory(move) }
        return true
    }

    /// Before the radio plays: when nothing is on air, the phone opens (what is ahead plays; with nothing ahead, the opening).
    func ensureOnAir() async {
        let (onAir, ahead) = await MainActor.run { (RunSheetStore.shared.onAir() != nil, !RunSheetStore.shared.ahead(limit: 1).isEmpty) }
        if onAir { return }
        if ahead { await MainActor.run { RunSheetPlayer.shared.advanceIfIdle() } } else { await openRadio() }
    }

    /// The radio opens: the phone's opening is placed and plays at once (from the front page it holds, with no network and no model), and
    /// the desk is asked for its own opening in the same moment.
    func openRadio() async {
        let lines = frontLines()
        let plan = openingCache.take(voice: runtime.player.voiceKey)
        await MainActor.run {
            Self.placeLaunch(plan: plan, lines: lines, player: RunSheetPlayer.shared, store: RunSheetStore.shared, writer: writer)
            LaunchTimings.shared.mark("sheet")
        }
        onChange?()                                   // the entry's own start begins the player (speakEntry); nothing here plays a document
    }

    /// What the radio puts on air at launch and what it asks the desk for. The phone's own opening needs no model; the desk's `open` answer is
    /// exactly one opening line, which the player drops when the phone's is on air ("one per start"), so when the phone has an opening of its
    /// own the desk is asked to `extend` (the stretch is written while the opening plays). `open` is asked only when a kept first entry
    /// starts the radio: the desk's opening then follows it as a transition.
    @MainActor static func placeLaunch(plan: OpeningPlan, lines: [PhoneOpening.Line], player p: RunSheetPlayer, store: RunSheetStore, writer: RunSheetWriter?) {
        // A restored sheet still holds the id of a kept entry. Ahead: the copy is stale and `open` removes it, the kept one plays. On air, heard or
        // skipped: the listener has had it, never replay it (the phone's opening plays; the reason is logged).
        @MainActor func fresh(_ e: RunSheetEntry) -> RunSheetEntry? {
            var x = e; x.state = .ahead; x.ended = nil
            guard let have = store.entry(x.id) else { return x }
            if have.state == .ahead { return x }
            print("AUTORADIO kept first entry not played: its copy on the sheet is \(have.state.rawValue)")
            return nil
        }
        switch plan {
        case .opening(let o, let next):
            var o = o; o.placedBy = .phone; o.state = .ahead; o.ended = nil
            p.open(o)
            if let n = next.flatMap(fresh) { store.place(n, at: .next, by: .desk, reason: "written ahead and kept for this launch") }
            writer?.openAhead()
            print("AUTORADIO opening from the written-ahead cache \(o.id)")
        case .entry(let n):
            if let n = fresh(n) {
                p.open(n, asOpening: false)
                print("AUTORADIO first entry from the written-ahead cache \(n.id); the live opening follows as a transition")
                writer?.open()
            } else { p.open(PhoneOpening.entry(lines: lines)); writer?.openAhead() }
        case .none:
            p.open(PhoneOpening.entry(lines: lines))
            writer?.openAhead()
        }
    }

    /// A fresh opening is kept: the launch need not wait for the sign-in or the first load.
    public func openingReady() -> Bool { openingCache.isReady(voice: runtime.player.voiceKey) }
    /// What the launch waits for: a written-ahead opening, a front page held or kept, or no sign-in (nothing will come).
    public func launchConditionHolds() -> Bool { openingReady() || !frontLines().isEmpty || !signIn.current.signedIn }
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
    /// The app went to the background: write the next opening ahead (one request, none when a fresh one is kept).
    @MainActor public func enteredBackground() async { await writeAhead?.refresh(reason: "background") }

    /// Play/pause from the big button: starts the audio engine on first use.
    public func playPause() async {
        LaunchTimings.shared.mark("play")
        // Try again, when the session was the trouble: that press is a retry and nothing else (the bar says paused while an entry is held).
        if await MainActor.run(body: { self.continuity?.readerTapped(); return RunSheetPlayer.shared.playPressed() }) { onChange?(); return }
        if await MainActor.run(body: { () -> Bool in
            let p = RunSheetPlayer.shared
            guard p.isRemote else { return false }
            if p.paused { p.resume() } else { p.pause() }
            return true
        }) { onChange?(); return }
        // Whether something was on air BEFORE this press decides what the press means: with nothing on air, Play opens the radio, and
        // opening it starts the player (the entry's own start), so reading `userPaused` afterwards would take that start for "it was
        // playing" and pause it at once (the Mac's first Play spoke one block and fell silent).
        let wasOnAir = await MainActor.run { RunSheetStore.shared.onAir() != nil }
        await ensureOnAir()
        await RunSheetPlayer.shared.settle()
        guard await MainActor.run(body: { RunSheetStore.shared.onAir() != nil }) else { onChange?(); return }      // nothing is on air: never play a document the player happens to hold
        await ensureEngine()
        if runtime.player.userPaused { runtime.player.play() }
        else if wasOnAir { runtime.player.pause(reason: "play button toggle") }
        onChange?()
    }
    /// Start the radio and leave it playing, whatever it was doing: the setup's "Start the radio" and the station when Autoradio opens.
    public func startRadio() async {
        LaunchTimings.shared.mark("play")
        await openRadio()
        await RunSheetPlayer.shared.settle()
        onChange?()
    }
    /// Sign-out, in this order: playback stops (the player reports the entry on air as `left`); the run sheet gets three seconds to post its
    /// pending ops with the old key still held, and what is not answered then is dropped, never sent under a later account; then the local key
    /// goes and the account wipe runs (ChoirSignIn.signOut); the revoke follows in the background.
    public func signOut() async {
        await MainActor.run { TakeSession.live?.discard() }      // a take in hand is the old account's: dropped first (its give-back is a queued task that may land after the pause; the queue is cleared, so it plays nothing)
        lock.lock(); loading = false; lock.unlock()
        runtime.player.pause(reason: "sign out")
        runtime.player.clearQueue()
        runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        await MainActor.run { RunSheetPlayer.shared.stopAll(); self.writer?.stop() }
        await RunSheetStore.shared.settlePending(timeout: 3)
        await signIn.signOut()
        lock.lock(); stories = []; message = ""; lock.unlock()
        onChange?()
    }

    /// Every store of the account, wiped by one call at sign-out. The module adds the stores that live in the app target (mail).
    public let accountWipe = AccountWipe()
    private func registerAccountWipe() {
        let front = frontCache, opening = openingCache, kept = runtime.store
        // Registered at once (not in a later task): a sign-out before the main actor ran a task would wipe nothing.
        onMainThread { [accountWipe] in accountWipe.registerStandard(openingCache: opening, keptTurns: kept) }
        accountWipe.register("front-titles") { if let f = front { try? FileManager.default.removeItem(at: f) } }
        signIn.onSignOut = { [accountWipe] in await accountWipe.run() }
    }

    public func playerView() -> ChoirPlayerView {
        let snap = runtime.snapshot()
        let p = runtime.player
        var v = ChoirPlayerView()
        v.signedIn = signIn.current.signedIn
        lock.lock(); v.loading = loading; v.message = message; lock.unlock()
        let onAir = onMainThread { RunSheetStore.shared.onAir() != nil || RunSheetPlayer.shared.isRemote }
        v.title = p.document.title
        if let si = p.document.segmentIndex(containingBlock: p.blockIndex) { v.title = p.document.segments[si].title }   // the entry's title
        if p.document.blocks.indices.contains(p.blockIndex) { v.passage = p.document.blocks[p.blockIndex].spokenText ?? p.document.blocks[p.blockIndex].summary }
        v.playing = onAir && snap.running && !p.userPaused          // nothing on air is never "playing"
        if !onAir { v.title = ""; v.passage = "" }
        v.talking = snap.phase != "playing"
        v.heard = snap.phase == "playing" ? "" : snap.transcript
        if v.message.isEmpty, v.signedIn, !v.loading, p.document.blocks.isEmpty { v.message = "Pick a story below." }
        return v
    }
    public func storyRows() -> [StoryRow] {
        lock.lock(); let s = stories; lock.unlock()
        let r = onMainThread { RunSheetStore.shared.onAir()?.paperRoute } ?? ""
        return s.map { StoryRow(title: $0.title, dek: $0.dek, route: $0.route, section: $0.section, current: $0.route == r) }
    }
    public func authView() -> ChoirAuthState { signIn.current }
}
