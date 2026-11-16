import Foundation

// The run sheet's player (slice B): it makes the entry on air heard, and nothing else decides what is heard. It watches the store. The
// entry the store says is on air is the entry that plays: when `place now`, `aired` or an undo puts a different entry on air, this plays it;
// when the on-air entry is gone (skip, previous, clear), the sound stops and the next ahead entry is aired. Only this reports air state:
// `aired on_air` when it starts the first ahead entry, `aired heard` or `skipped` when the entry ends.
//
// The sound itself is behind `PlaybackEngine`, so this is tested with a fake one. The real engine is `ChoirPlaybackEngine`: spoken words
// through the programme player (one document per entry), audio through AVPlayer. The deck of pages is not an input here.
//
// Ways this can fail (written before the code; each has a test in RunSheetStoreTests):
//  1. Skip, next, a swipe on the card and the lock screen's next do different things: they all call `next()`, which is one `skip(on air)`.
//  2. The next after a story is the paper's neighbour: the player never reads the paper's list; only `ahead` decides.
//  3. An entry that cannot be loaded stops the radio: it ends as skipped and the next one plays. Two in a row that make no sound stop it.
//  4. A late end (a document that was replaced) closes the entry that replaced it: the end must name the document that is playing.
//  5. The listener spoke over the end of an entry: the next one waits for the turn to end (`turnEnded`).
//  6. A clip whose file is not the transcribed one plays: it is checked first and ends as skipped, without its lead-in.
//  7. A clip's skip leaves it playing: skipping stops remote audio, and the entry ends `skipped`.
//  8. Nothing ahead and something arrives later: the player starts it when the radio was playing and is not paused.

/// What plays the sound. Spoken things go through the programme player; remote audio through AVPlayer.
@MainActor public protocol PlaybackEngine: AnyObject {
    /// Speak `text` as the document `documentID` (a single segment named for `entry`), replacing what is playing. False: nothing started.
    func speak(documentID: String, entry: RunSheetEntry, paragraphs: [String]) async -> Bool
    /// Load a paper story in full and play it. Its document id; nil when it could not be loaded.
    func playStory(route: String, title: String) async -> String?
    /// The spoken entry from its start.
    func restartSpoken()
    /// Where the spoken entry is: the server block being spoken, the last server block it has, and the seconds into it.
    func spokenReport() -> (block: Int, final: Int, seconds: Double)
    /// The block of the spoken entry and the seconds into it.
    func spokenPosition() -> (block: Int, seconds: Double)
    /// Silence the spoken entry (something else is starting, or it was skipped).
    func stopSpoken()
    /// The spoken entry says no more than the block it is on (the rest is replaced): whole sentences, no word cut.
    func endSpokenAfterCurrentBlock()
    /// Play remote audio from `start` (seconds) and stop at `end` when there is one.
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool
    func pauseRemote()
    func resumeRemote()
    /// Silence remote audio.
    func stopRemote()
    func seekRemote(by seconds: Double)
    func seekRemote(to seconds: Double)
    /// The playing clip no longer ends: the episode carries on from where it is, with no gap or restart.
    func clearRemoteEnd()
    func remotePosition() -> (elapsed: Double, duration: Double?)?
    func setRemoteRate(_ r: Double)
    func setRemoteLevel(_ l: PlayLevel)
    /// The length of the audio at `url`, read without playing it; nil when unknown within `deadline` seconds.
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double?
}
extension PlaybackEngine {
    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { nil }
    public func restartSpoken() {}
    public func spokenReport() -> (block: Int, final: Int, seconds: Double) { (0, 0, 0) }
    public func spokenPosition() -> (block: Int, seconds: Double) { (0, 0) }
    public func stopSpoken() {}
    public func endSpokenAfterCurrentBlock() {}
}

@MainActor public final class RunSheetPlayer {
    public static let shared = RunSheetPlayer(store: .shared)

    public let store: RunSheetStore
    public var engine: PlaybackEngine?
    /// Told after anything the screens read changed (the entry on air, the pause, the position of remote audio).
    public var onChange: (() -> Void)?
    /// The reader lane's: the book and block being read, reported as reading moves.
    public var onDocumentPosition: ((String, Int) -> Void)?
    /// The writer is asked whether the time ahead is short; the host sets it.
    public var aheadChanged: (() -> Void)?
    /// A turn (the listener's words) is open: the end of an entry waits for it.
    public var turnOpen: () -> Bool = { false }
    /// The listener paused the spoken radio: nothing starts by itself.
    public var spokenPaused: () -> Bool = { false }
    /// Fetches a page's text; replaceable for tests.
    public var pageText: (URL, String) async -> (title: String, paragraphs: [String])? = { await PageText.fetch($0, title: $1) }
    /// The phone's opening of this listening, and what became of desk entries the sheet did not take (id, reason).
    public private(set) var currentOpeningId: String?
    public internal(set) var droppedFromDesk: [(id: String, reason: String)] = []
    /// Called before every start of playback this player makes (the opening, play now, next, previous, an audio entry, a restart after a pause):
    /// the host makes sure the audio session is claimed and active and the engine is up (`ensureReady`). The reason names the entry kind.
    public var sessionReady: ((String) async -> Void)?
    /// How long a clip's audio may take to start before it is given up.
    public var startDeadline: TimeInterval = 6

    /// The desk could not be reached or wrote nothing: a quiet line, never an entry. Set by the host from the writer.
    public var deskNotice: String? { didSet { if oldValue != deskNotice { onChange?() } } }
    /// The desk is writing what comes next.
    /// The last write returned nothing new.
    public var deskDry = false { didSet { if oldValue != deskDry { onChange?() } } }
    public var deskWriting = false { didSet { if oldValue != deskWriting { onChange?() } } }
    /// A plain line about the last thing that could not play ("" when nothing is wrong).
    public private(set) var message = ""
    /// The listener paused remote audio (spoken entries are paused through the programme player's own transport).
    public private(set) var paused = false
    /// Which start this is; remote audio reports its end for the start it belongs to.
    public private(set) var generation = 0
    /// The block of the book or page being read (absolute).
    public private(set) var documentBlock = 0
    /// The audio now playing: "keep listening" clears its end.
    public private(set) var audio: RunSheetAudio?
    /// The entry the player was told to play (it is the entry the store has on air).
    public private(set) var playingId: String?
    public private(set) var playingDocument: String?

    private var textSources: [String: @Sendable (String, Int) async -> [String]?] = [:]
    private var provided: [String: (title: String, paragraphs: [String])] = [:]
    private var startTask: Task<Void, Never>?
    /// The radio is on: an entry that ends is followed by the next one. Off at launch, after a pause that stopped it, and after sign-out.
    var wants = false
    /// The turn ids already applied (the turn route's replay rule).
    var turnLog: [String] = []
    private var silent = 0
    private var reconciling = false, again = false
    private var pendingEnd: (id: String, played: Bool)?
    private var leadInDone = Set<String>()

    public init(store: RunSheetStore) {
        self.store = store
        store.onChange = { [weak self] in self?.storeChanged() }
    }

    /// Text for a book: the reader lane registers `{ id, position in blocks }`; blocks start at `position`.
    public func registerTextSource(kind: String, _ source: @escaping @Sendable (String, Int) async -> [String]?) { textSources[kind] = source }

    /// Wait for the start in flight (tests, and the host that wants to know it began).
    public func settle() async { await startTask?.value }

    // MARK: The listener's verbs (each is ops on the store)

    /// The opening: placed by the phone, on air at once. The radio is wanted from now on.
    /// What the desk and the phone wrote for an earlier listening is not played now (it may be a day old): it is taken off. What the listener
    /// placed themselves stays.
    func setOpeningForTest(_ id: String) { currentOpeningId = id }
    public func open(_ entry: RunSheetEntry) {
        currentOpeningId = entry.id
        wants = false                                // nothing ahead is aired while the sheet is being cleaned: the opening goes first
        for e in store.ahead(limit: Int.max) where e.placedBy != .listener { store.remove(id: e.id, by: .phone, reason: "left from an earlier listening") }
        store.place(entry, at: .now, by: .phone)
        wants = true
    }

    /// Play this now ("Play", Listen, Read it to me, Read aloud). What was on air is cut; what was ahead stays ahead. `words` are what a
    /// page's own text said, held for this entry (the entry itself is a pointer).
    public func playNow(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        wants = true
        if let words { provided[e.id] = words }
        let cut = store.onAir() != nil ? report(final: false) : nil
        store.place(e, at: .now, cutting: cut)
    }
    /// After the entry on air, before everything else ahead.
    public func playNext(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        if let words { provided[e.id] = words }
        store.place(e, at: .next)                   // when the radio is on and idle, the store's change starts it
    }
    /// "Queue this": after what the listener placed before it.
    public func queue(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        if let words { provided[e.id] = words }
        store.place(e, at: .queued)
    }
    public func remove(at i: Int) { let a = store.ahead(limit: i + 1); if a.indices.contains(i) { store.remove(id: a[i].id) } }
    /// Tap an item in Up next: play it now. The entries before it stay ahead.
    public func playUpcoming(at i: Int) {
        let a = store.ahead(limit: i + 1)
        guard a.indices.contains(i) else { return }
        wants = true
        store.move(id: a[i].id, at: .now)
    }
    public func clearUpcoming() { store.clearAhead() }

    /// Skip, the next button, a swipe on the now-playing card, the lock screen's next, and "next" by voice: one op, `skip(on air)`. With
    /// nothing on air it starts what is next.
    public func next() {
        wants = true
        if store.onAir() != nil { store.skipOnAir(report: report(final: false)) } else { advanceIfIdle() }
    }
    /// Back: the start of this entry when it has gone on a little (or it is audio), else the entry before it, aired again.
    public func previous() {
        wants = true
        if audio != nil, store.onAir() != nil { engine?.seekRemote(to: 0); changed(); return }
        if store.onAir() != nil {
            let p: (block: Int, seconds: Double) = engine?.spokenPosition() ?? (block: 0, seconds: 0)
            if StoryCursor.previousRestartsCurrent(blockIndex: p.block, secondsIntoBlock: p.seconds) { engine?.restartSpoken(); changed(); return }
        }
        if !store.previous().applied, store.onAir() != nil { engine?.restartSpoken() }       // nothing before it: this one again
        else if store.onAir() == nil { advanceIfIdle() }
    }

    /// Sign-out: what was on air stops and nothing carries over to the next account.
    public func stopAll() {
        wants = false
        // Sign-out is not a judgement of the entry: it leaves the air as `left`, by the phone, where it was.
        if let e = store.onAir() { let r = report(final: false); store.airedEnded(e.id, as: .heard, outcome: .left, report: r) }
        store.clearAhead(by: .phone, reason: "signed out")
        engine?.stopRemote(); engine?.stopSpoken()
    }

    /// The radio is on and nothing is on air: start what is next.
    public func advanceIfIdle() {
        guard store.onAir() == nil, playingId == nil else { return }
        advance()
    }

    private func advance() {
        guard let first = nextToAir() else { aheadChanged?(); return }
        store.airedOnAir(first.id)                    // the store's change starts it
    }

    /// The first entry ahead that should be aired. A transition is written for the two entries it sits between: it plays only when the entry
    /// just before it was heard to its end (not skipped past) and the entry after it is the one it was written for; otherwise it is dropped.
    func nextToAir() -> RunSheetEntry? {
        while let first = store.ahead(limit: 1).first {
            guard first.kind == .transition else { return first }
            let ahead = store.ahead(limit: 2)
            let prev = store.heard(limit: 1).first
            let after = ahead.count > 1 ? ahead[1] : nil
            let ok = prev?.state == .heard && (first.between.map { prev?.id == $0.after && after?.id == $0.before } ?? true) && after != nil
            if ok { return first }
            droppedFromDesk.append((first.id, "transition_orphaned"))
            store.remove(id: first.id, by: .phone, reason: "transition_orphaned")
        }
        return nil
    }

    /// A desk op about to be applied. Where the phone's opening is the same job as the desk's: before it began, the desk's replaces it; while
    /// it is on air it finishes the block it is on and the desk's follows it; once it was heard (or skipped) the desk's is not a second greeting
    /// and is dropped. Only for the opening write (`cause: open`).
    public func admitDesk(_ op: RunSheetOp, cause: String) -> RunSheetOp? {
        // A desk that tells a story already on the sheet (heard, on air or ahead) is not heard twice: dropped, and the reason kept.
        if op.op == .place, let e = op.entry, e.kind == .story,
           store.sheet.entries.contains(where: { $0.kind == .story && $0.id != e.id && $0.title.lowercased() == e.title.lowercased() && !e.title.isEmpty }) {
            droppedFromDesk.append((e.id, "title_already_on_sheet"))
            return nil
        }
        guard cause == "open", op.op == .place, var e = op.entry, e.kind == .opening, e.placedBy == .desk,
              let pid = currentOpeningId, let phone = store.entry(pid) else { return op }
        var o = op
        switch phone.state {
        case .heard, .skipped:
            droppedFromDesk.append((e.id, "opening_already_heard"))
            return nil
        case .ahead:
            store.remove(id: pid, by: .phone, reason: "opening_replaced_by_desk")
            o.at = .next
        case .onAir:
            engine?.endSpokenAfterCurrentBlock()
            o.at = .next
        }
        e.title = "Opening"; o.entry = e
        return o
    }

    // MARK: Reacting to the store

    private func storeChanged() {
        if reconciling { again = true; return }
        reconciling = true
        defer { reconciling = false }
        repeat {
            again = false
            reconcile()
        } while again
        changed()
        aheadChanged?()
    }

    private func reconcile() {
        if let air = store.onAir() {
            guard air.id != playingId else { return }
            playingId = air.id; playingDocument = nil; audio = nil; pendingEnd = nil
            startTask = Task { @MainActor in await self.begin(air) }
            return
        }
        if playingId != nil {                         // what was on air was skipped, cut or taken off: its sound stops
            playingId = nil; playingDocument = nil; audio = nil; paused = false; generation += 1; pendingEnd = nil
            engine?.stopRemote(); engine?.stopSpoken()
        }
        if wants, !spokenPaused(), !turnOpen(), let first = nextToAir() { store.airedOnAir(first.id); again = true }
    }

    private func changed() { onChange?() }

    // MARK: Starting an entry

    private func begin(_ entry: RunSheetEntry) async {
        await sessionReady?(entry.kind.rawValue)
        guard playingId == entry.id else { return }            // a newer start took over while the session was made ready
        guard let engine else { fail(entry.id, why: nil); return }
        generation += 1
        let my = generation
        paused = false; message = ""
        documentBlock = entry.book?.at ?? 0
        var ok = false
        switch entry.kind {
        case .opening, .transition, .story, .answer, .result:
            engine.stopRemote()
            let doc = "rs:" + entry.id
            playingDocument = doc
            ok = await engine.speak(documentID: doc, entry: entry, paragraphs: [entry.text ?? ""])
        case .page, .mail:
            engine.stopRemote()
            if let w = provided[entry.id] {
                let doc = "rs:" + entry.id
                playingDocument = doc
                ok = await engine.speak(documentID: doc, entry: entry, paragraphs: w.paragraphs)
            } else if let route = entry.paperRoute {
                if let doc = await engine.playStory(route: route, title: entry.title), my == generation { playingDocument = doc; ok = true }
                else if my == generation { message = "Could not load \u{201C}\(entry.displayTitle)\u{201D}." }
            } else if let url = entry.webURL {
                if let p = await pageText(url, entry.title), my == generation {
                    let doc = "rs:" + entry.id
                    playingDocument = doc
                    ok = await engine.speak(documentID: doc, entry: entry, paragraphs: p.paragraphs)
                } else if my == generation { message = "Could not read \u{201C}\(entry.displayTitle)\u{201D}." }
            }
        case .document:
            engine.stopRemote()
            if let b = entry.book, let src = textSources["document"], let blocks = await src(b.id, b.at), !blocks.isEmpty, my == generation {
                let doc = "rs:" + entry.id
                playingDocument = doc
                ok = await engine.speak(documentID: doc, entry: entry, paragraphs: blocks)
            } else if my == generation { message = "Could not open \u{201C}\(entry.displayTitle)\u{201D} to read." }
        case .clip, .episode:
            guard let a = entry.audioSpec else { break }
            if entry.kind == .clip, let id = entry.source?.clipId, !(await preflight(id)) { if my == generation { finish(entry.id, played: false) }; return }
            guard my == generation else { return }
            audio = a
            if entry.kind == .clip, let lead = entry.text, !lead.isEmpty {
                let doc = "rs:" + entry.id
                playingDocument = doc
                ok = await engine.speak(documentID: doc, entry: entry, paragraphs: [lead])
            } else {
                ok = await startAudio(a, generation: my)
            }
        }
        guard my == generation, playingId == entry.id else { return }       // a newer start took over
        if !ok { if message.isEmpty { message = "Could not play \u{201C}\(entry.displayTitle)\u{201D}." }; fail(entry.id, why: message); return }
        silent = 0
        changed()
    }

    /// The remote half of an audio entry (after its lead-in, if it had one).
    private func startAudio(_ a: RunSheetAudio, generation my: Int) async -> Bool {
        guard let engine else { return false }
        let ok = await engine.playRemote(url: a.url, title: a.title, show: a.show, start: a.start, end: a.end)
        if !ok, my == generation { message = "Could not play \u{201C}\(a.title)\u{201D}." }
        return ok
    }

    /// Before a clip is spoken or played: is this the file the clip was cut from? Its length is read without playing it. False: unknown
    /// clip, no engine, no answer in time, or a length more than 3 s from the transcribed file's; the clip is marked so its "Hear it" row
    /// offers the episode only.
    public func preflight(_ id: String, store clips: ClipStore = .shared) async -> Bool {
        guard let clip = clips.clip(id), let engine else { return false }
        guard let d = await engine.remoteDuration(url: clip.audioURL, deadline: startDeadline) else { return false }
        if clip.duration > 0, abs(d - clip.duration) > PodcastClip.durationTolerance { clips.markMismatch(id); return false }
        return true
    }

    /// What cannot play is skipped, once, and the radio goes on. Two in a row that made no sound stop it.
    private func fail(_ id: String, why: String?) {
        silent += 1
        if silent >= 2, wants { wants = false; if message.isEmpty { message = "The radio could not play the next entries. Tap play to try again." } }
        finish(id, played: false)
    }

    /// The entry ended (or never made a sound): the phone reports it, and the store's change starts the next one.
    private func finish(_ id: String, played: Bool, report given: AirReport? = nil) {
        guard playingId == id else { return }
        let r = given ?? report(final: true)
        playingId = nil; playingDocument = nil; audio = nil; paused = false; pendingEnd = nil
        // A natural end carries the true final block and position; a failure is `failed` by the phone, never a skip.
        if played { store.airedEnded(id, as: .heard, outcome: .finished, report: r) }
        else { store.airedEnded(id, as: .skipped, outcome: .failed) }
        provided[id] = nil
    }

    /// Where the entry on air is now. Remote audio: its elapsed seconds. Spoken: the server block and the seconds into it (`final`: at its last block).
    func report(final: Bool) -> AirReport {
        if audio != nil, playingDocument == nil { return AirReport(block: 0, seconds: engine?.remotePosition()?.elapsed ?? 0) }
        guard let e = engine else { return AirReport() }
        let r = e.spokenReport()
        return AirReport(block: final ? r.final : r.block, seconds: r.seconds)
    }

    // MARK: Ends, reported by the engines

    /// The spoken document that was playing ran out of blocks. `anyPlayed`: a block made sound.
    public func documentEnded(id: String, anyPlayed: Bool) {
        guard let eid = playingId, id == playingDocument else { return }
        if turnOpen() { pendingEnd = (id, anyPlayed); return }
        // A clip's lead-in is over: the audio plays now.
        if let e = store.entry(eid), e.kind == .clip, let a = audio, !leadInDone.contains(eid) {
            leadInDone.insert(eid)
            playingDocument = nil
            let my = generation
            startTask = Task { @MainActor in
                let ok = await self.startAudio(a, generation: my)
                guard my == self.generation, self.playingId == eid else { return }
                if !ok { self.fail(eid, why: self.message) } else { self.changed() }
            }
            return
        }
        if anyPlayed { silent = 0 } else {
            silent += 1
            if silent >= 2 { wants = false; message = "The radio's voice did not work, so nothing could be read aloud." }
        }
        finish(eid, played: anyPlayed)
    }

    /// A turn ended: an entry that ended over the listener's words is closed now, and the next one starts.
    public func turnEnded() {
        if let p = pendingEnd { pendingEnd = nil; documentEnded(id: p.id, anyPlayed: p.played) }
        else if wants, store.onAir() == nil { advanceIfIdle() }
    }

    /// Remote audio reached its end, or the clip's end. `generation` is the start it belongs to.
    public func remoteEnded(generation g: Int) {
        guard g == generation, audio != nil, let id = playingId else { return }
        let r = report(final: true)                 // before the sound is stopped: the true position
        engine?.stopRemote()
        finish(id, played: true, report: r)
    }

    // MARK: Transport for remote audio

    public var isRemote: Bool { audio != nil && playingDocument == nil }
    public func pause() { guard isRemote else { return }; paused = true; engine?.pauseRemote(); store.save(); changed() }
    /// Resume paused remote audio (the engine makes the session ready first: `ChoirPlaybackEngine.resumeRemote`).
    public func resume() { guard isRemote else { return }; paused = false; engine?.resumeRemote(); changed() }
    public func seek(by seconds: Double) { guard isRemote else { return }; engine?.seekRemote(by: seconds); changed() }
    public func seek(to seconds: Double) { guard isRemote else { return }; engine?.seekRemote(to: seconds); changed() }
    public func setRate(_ r: Double) { engine?.setRemoteRate(PlaybackSpeed.clamp(r)) }
    /// The hands-free turn: the radio's own level (dip or pause for the reader's words, play after).
    public func setLevel(_ l: PlayLevel) { if isRemote { engine?.setRemoteLevel(l) } }

    /// "Keep listening": the clip stops being a clip. The episode continues past it, from where it is.
    public func keepListening() {
        guard var a = audio, a.end != nil else { return }
        a.end = nil
        audio = a
        engine?.clearRemoteEnd()
        store.save(); changed()
    }
    /// The current clip's title, and for a clip the show it is cut from (what "Keep listening" shows).
    public var keepListeningTitle: String? {
        guard let a = audio, a.end != nil else { return nil }
        return a.show.isEmpty ? a.title : "\(a.title) (\(a.show))"
    }
    public func remotePosition() -> (elapsed: Double, duration: Double?)? { isRemote ? engine?.remotePosition() : nil }

    /// Reading moved to block `i` of the book or page being read (counted from the block the reading began at).
    public func spokenBlock(_ i: Int) {
        let base = playingId.flatMap { store.entry($0)?.book?.at } ?? 0
        guard documentBlock != base + i else { return }
        documentBlock = base + i
        if let b = playingId.flatMap({ store.entry($0)?.book }) { onDocumentPosition?(b.id, documentBlock) }
    }
}
