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
    /// The same, named by what it is and not by a run sheet entry (the deck's item): `kind` is the segment (opening, transition, answer, result, mail; "" a story).
    func speak(documentID: String, entryID: String, title: String, kind: String, paragraphs: [String]) async -> Bool
    /// Load a paper story in full and play it. Its document id; nil when it could not be loaded.
    func playStory(route: String, title: String) async -> String?
    /// The spoken entry from the block that did not play (the engine was lost mid-entry and is back).
    func resumeSpoken()
    /// The spoken entry from its start.
    func restartSpoken()
    /// Where the spoken entry is: the server block being spoken, the last server block it has, and the seconds into it.
    func spokenReport() -> (block: Int, final: Int, seconds: Double)
    /// The block of the spoken entry and the seconds into it.
    func spokenPosition() -> (block: Int, seconds: Double)
    /// Silence the spoken entry (something else is starting, or it was skipped).
    func stopSpoken()
    /// Why the last spoken block could not be made (nil: it was).
    func speakFailure() -> String?
    /// What of the spoken entry is still to be said, from the start of the sentence it is in; nil when unknown.
    func spokenRemainder() -> String?
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
    /// Pause the spoken entry (the listener's pause, or the system's).
    func pauseSpoken()
    /// Sound is reaching the output for what is playing now (spoken: rendered seconds past zero; remote: the position is moving). The deck's
    /// evidence that an item was heard; a deck that cannot tell says false and nothing is ever journalled as rendered by it.
    func rendering() -> Bool
}
extension PlaybackEngine {
    /// An engine that knows only the entry form is handed one built from the item's own fields.
    public func speak(documentID: String, entryID: String, title: String, kind: String, paragraphs: [String]) async -> Bool {
        let k: RunSheetKind = ["opening": .opening, "transition": .transition, "answer": .answer, "result": .result, "mail": .mail][kind] ?? .story
        let text = paragraphs.joined(separator: "\n\n")
        var e = RunSheetEntry(id: entryID, kind: k, placedBy: .phone, title: title, text: text)
        e.estSeconds = RunSheetEntry.estimateSeconds(words: text)
        return await speak(documentID: documentID, entry: e, paragraphs: paragraphs)
    }
    public func pauseSpoken() {}
    public func rendering() -> Bool { false }
    public func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { nil }
    public func restartSpoken() {}
    public func resumeSpoken() {}
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
    /// The listener paused (or the route was lost): not the system's interruption pause. A held entry resumes by itself over the system's pause only.
    public var spokenPausedByListener: () -> Bool = { false }
    /// Fetches a page's text; replaceable for tests.
    public var pageText: (URL, String) async -> (title: String, paragraphs: [String])? = { await PageText.fetch($0, title: $1) }
    /// Where the phone keeps its own copy of an audio file from Files, by entry id: never posted. After a relaunch it is empty and `fileResolver` finds the file again.
    public var localFiles: [String: URL] = [:]
    /// Finds (downloading when the copy is gone) the phone's copy of the file at a Files address.
    public var fileResolver: ((String) async -> URL?)?
    /// The phone's opening of this listening, and what became of desk entries the sheet did not take (id, reason).
    public private(set) var currentOpeningId: String?
    public internal(set) var droppedFromDesk: [(id: String, reason: String)] = []
    /// Called before every start of playback this player makes (the opening, play now, next, previous, an audio entry, a restart after a pause):
    /// the host makes sure the audio session is claimed and active and the engine is up (`ensureReady`). The reason names the entry kind.
    public var sessionReady: ((String) async -> String?)?   // nil: ready; else why sound cannot come out
    /// How long a clip's audio may take to start before it is given up.
    public var startDeadline: TimeInterval = 6
    /// Finds a podcast clip in the file the host serves now, from its fingerprint, and keeps the bytes it measured (ClipLocator). A clip is
    /// played from those bytes, never from the host again.
    public var locator: ClipLocating = ClipLocator.shared
    /// The clips the phone holds (the located ones are looked up here by the entry's clip id).
    public var clips: ClipStore = .shared
    /// Fetches a take's audio (the publisher's voice, signed in) into a file on the phone, by take id; nil when it could not be had (deleted,
    /// yanked, offline). Set by the host. The file is deleted when the entry ends or leaves the sheet.
    public var takeAudio: ((String) async -> URL?)?
    var takeFiles: [String: URL] = [:]
    /// Locating in flight or done, by entry id: it starts when the entry is next in line, so the clip begins with no gap.
    private var locating: [String: Task<ClipLocation, Never>] = [:]
    /// The clip files kept for entries (deleted when the entry ends or leaves the sheet).
    private var located: [String: LocatedClip] = [:]

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
    /// The deck under this player: it keeps the journal of what started and ended, with the device's time and whether sound was rendered.
    public var deck: Deck

    private var textSources: [String: @Sendable (String, Int) async -> [String]?] = [:]
    private var provided: [String: (title: String, paragraphs: [String])] = [:]
    private var startTask: Task<Void, Never>?
    /// The radio is on: an entry that ends is followed by the next one. Off at launch, after a pause that stopped it, and after sign-out.
    var wants = false
    /// The radio is wanted (started and not stopped): the next entry starts by itself when one arrives.
    public var wantsToPlay: Bool { wants }
    /// The turn ids already applied (the turn route's replay rule).
    var turnLog: [String] = []
    private var silent = 0
    private var reconciling = false, again = false
    private var pendingEnd: (id: String, played: Bool)?
    private var leadInDone = Set<String>()
    private var keptSeen = Set<String>()
    /// An answer went on air over this entry: where it was, as a continuation entry (fresh id, same source, the words still to say, or the same
    /// audio from where it was). It is placed `next` when the answers are over, unless the listener asked for something else meanwhile.
    var remembered: RunSheetEntry?

    public init(store: RunSheetStore) {
        self.store = store
        deck = store === RunSheetStore.shared ? Deck.shared : Deck(journal: DeckJournal())       // a test's player never writes the app's journal
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
    /// The launch started with an entry that is not an opening (the first entry written ahead): the desk's live opening, when it comes, is a
    /// transition placed next, never a second greeting over a story.
    public var openingIsTransition = false
    /// An entry ended (heard or skipped): the host writes ahead after the session's opening.
    public var onEntryEnded: ((RunSheetEntry, Bool) -> Void)?

    /// THE ONE OPENING RULE (a listener never hears two openings in a row): an opening may be placed only when none is on air and none was heard
    /// (finished or part-heard) in the last `openingWindow`. The phone's `open` (its built opening and the cached one alike: both are an entry of
    /// kind opening) and the desk's `admitDesk` both ask it. A heard opening with no time on it counts as recent. This is admission on the phone,
    /// before an op is made; the reducer and the server are not involved.
    public static let openingWindow: TimeInterval = 30 * 60
    public func openingAllowed() -> Bool {
        if store.sheet.entries.contains(where: { $0.kind == .opening && $0.state == .onAir }) { return false }
        let now = store.now()
        for e in store.sheet.entries where e.kind == .opening && e.state.isTail && e.ended?.outcome != .failed {
            guard let at = e.ended?.at, !at.isEmpty, let d = ISO8601DateFormatter().date(from: at) else { return false }
            if now.timeIntervalSince(d) <= Self.openingWindow { return false }
        }
        return true
    }

    public func open(_ entry: RunSheetEntry, asOpening: Bool = true) {
        openingIsTransition = !asOpening
        wants = false                                // nothing ahead is aired while the sheet is being cleaned: the opening goes first
        let allowed = entry.kind != .opening || openingAllowed()
        // A fresh opening goes first and what an earlier listening left is taken off. When the one-opening rule refuses the opening, the earlier
        // listening was minutes ago: what it left ahead is still good, and it plays at once (nothing waits for the desk).
        if allowed { for e in store.ahead(limit: Int.max) where e.placedBy != .listener { store.remove(id: e.id, by: .phone, reason: "left from an earlier listening") } }
        if allowed {
            currentOpeningId = asOpening ? entry.id : nil
            store.place(entry, at: .now, by: .phone)
        } else {
            currentOpeningId = nil
            print("AUTORADIO opening dropped reason=one per start; \(store.ahead(limit: Int.max).count) entries ahead play at once")
        }
        wants = true
        if !allowed { advanceIfIdle() }                // nothing changed on the sheet, so nothing would start by itself
    }

    /// Play this now ("Play", Listen, Read it to me, Read aloud). What was on air is cut; what was ahead stays ahead. `words` are what a
    /// page's own text said, held for this entry (the entry itself is a pointer).
    public func playNow(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        dropRemembered()
        wants = true
        pressPending = true; defer { pressPending = false }       // an explicit placement is a press: it may take the session back
        if let words { provided[e.id] = words }
        let cut = store.onAir() != nil ? report(final: false) : nil
        placed(store.place(e, at: .now, cutting: cut))
    }
    /// After the entry on air, before everything else ahead.
    public func playNext(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        if let words { provided[e.id] = words }
        placed(store.place(e, at: .next))                   // when the radio is on and idle, the store's change starts it
    }
    /// "Queue this": after what the listener placed before it.
    public func queue(_ e: RunSheetEntry, words: (title: String, paragraphs: [String])? = nil) {
        if let words { provided[e.id] = words }
        placed(store.place(e, at: .queued))
    }
    /// An address too long for the server is not placed, and the listener is told so.
    private func placed(_ r: RunSheetResult) {
        if r.dropped == "ref_too_long" { message = "That address is too long to add."; changed() }
    }
    public func remove(at i: Int) { let a = store.ahead(limit: i + 1); if a.indices.contains(i) { store.remove(id: a[i].id) } }
    /// Tap an item in Up next: play it now. The entries before it stay ahead.
    public func playUpcoming(at i: Int) {
        let a = store.ahead(limit: i + 1)
        guard a.indices.contains(i) else { return }
        dropRemembered()
        wants = true
        pressPending = true; defer { pressPending = false }
        store.move(id: a[i].id, at: .now)
    }
    public func clearUpcoming() { dropRemembered(); store.clearAhead() }

    /// Skip, the next button, a swipe on the now-playing card, the lock screen's next, and "next" by voice: one op, `skip(on air)`. With
    /// nothing on air it starts what is next.
    public func next() {
        dropRemembered()
        wants = true
        if store.onAir() != nil { store.skipOnAir(report: report(final: false)) } else { advanceIfIdle() }
    }
    /// Back: the start of this entry when it has gone on a little (or it is audio), else the entry before it, aired again.
    public func previous() {
        dropRemembered()
        wants = true
        if audio != nil, playingDocument == nil, store.onAir() != nil { engine?.seekRemote(to: 0); changed(); return }
        if store.onAir() != nil {
            let p: (block: Int, seconds: Double) = engine?.spokenPosition() ?? (block: 0, seconds: 0)
            if StoryCursor.previousRestartsCurrent(blockIndex: p.block, secondsIntoBlock: p.seconds) { engine?.restartSpoken(); changed(); return }
        }
        if !store.previous().applied, store.onAir() != nil { engine?.restartSpoken() }       // nothing before it: this one again
        else if store.onAir() == nil { advanceIfIdle() }
    }

    /// Sign-out: what was on air stops and nothing carries over to the next account.
    public func stopAll() {
        dropRemembered()
        wants = false
        // Sign-out is not a judgement of the entry: it leaves the air as `left`, by the phone, where it was.
        if let e = store.onAir() {
            let r = report(final: false)
            // An entry that never sounded (held at its start, or lost at its first block) was not heard: `skipped` with `failed`, the phone's
            // "could not play it", which the server settles as no verdict. One that sounded leaves as it did: `heard`, `left`.
            let silent = (sessionHeldFor == e.id && !heldMidEntry) || (heldMidEntry && lostAtFirstBlock)
            if silent { store.airedEnded(e.id, as: .skipped, outcome: .failed, report: r, reason: "signed out before it made a sound") }
            else { store.airedEnded(e.id, as: .heard, outcome: .left, report: r) }
        }
        store.clearAhead(by: .phone, reason: "signed out")
        engine?.stopRemote(); engine?.stopSpoken()
    }

    /// The radio is on and nothing is on air: start what is next.
    public func advanceIfIdle() {
        guard store.onAir() == nil, playingId == nil else { return }
        wants = true                                  // pressing Play (or the station) is the want: what ends is followed by the next, with no press
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
        if openingIsTransition, cause == "open", op.op == .place, var e = op.entry, e.kind == .opening {
            openingIsTransition = false
            e.kind = .transition; e.title = ""; e.between = nil
            var o = op; o.at = .next; o.entry = e
            return o
        }
        guard cause == "open", op.op == .place, var e = op.entry, e.kind == .opening, e.placedBy == .desk else { return op }
        // A phone opening ahead and not yet on air is replaced by the desk's.
        if let pid = currentOpeningId, let phone = store.entry(pid), phone.state == .ahead {
            store.remove(id: pid, by: .phone, reason: "opening_replaced_by_desk")
            var o = op
            o.at = .next
            e.title = "Opening"; o.entry = e
            return o
        }
        // Otherwise one opening per start: none on air, none heard lately. The line is dropped; what follows it is admitted as usual.
        guard openingAllowed() else {
            droppedFromDesk.append((e.id, "opening_one_per_start"))
            print("AUTORADIO opening dropped reason=one per start")
            return nil
        }
        var o = op
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
        prefetchClips()
        // A kept row goes once its entry was on the sheet and is no longer ahead or on air (an entry made but not yet placed keeps its row).
        keptSeen.formUnion(store.sheet.entries.map(\.id))
        let live = Set(store.sheet.entries.filter { $0.state == .ahead || $0.state == .onAir }.map(\.id) + (remembered.map { [$0.id] } ?? []))
        let gone = keptSeen.subtracting(live)
        if !gone.isEmpty { HeldRest.shared.remove(ids: gone); keptSeen.subtract(gone) }
        changed()
        aheadChanged?()
    }

    /// The clip that is next in line (or the one after a transition) is located now, while the entry before it plays. Entries that left the
    /// sheet lose their file.
    private func prefetchClips() {
        let live = Set(store.sheet.entries.filter { $0.state == .ahead || $0.state == .onAir }.map(\.id))
        for id in Array(locating.keys) where !live.contains(id) {
            locating[id]?.cancel(); locating[id] = nil
            if let l = located.removeValue(forKey: id) { try? FileManager.default.removeItem(at: l.file) }
        }
        for e in store.ahead(limit: 2) where e.kind == .clip { _ = locating(e) }
    }

    /// The located clip for an entry: started once, shared by the prefetch and the start. Nil when the entry is not a clip the phone can find.
    private func locating(_ entry: RunSheetEntry) -> Task<ClipLocation, Never>? {
        guard entry.kind == .clip, let id = entry.source?.clipId else { return nil }
        if let t = locating[entry.id] { return t }
        guard let clip = clips.clip(id), clip.isRelocatable else { return nil }
        let loc = locator, eid = entry.id
        let t = Task { @MainActor [weak self] () -> ClipLocation in
            let r = await Task.detached { await loc.locate(clip) }.value
            if case .located(let l) = r { self?.located[eid] = l }
            return r
        }
        locating[entry.id] = t
        return t
    }

    private func dropLocated(_ id: String) {
        if let f = takeFiles.removeValue(forKey: id) { try? FileManager.default.removeItem(at: f) }
        locating[id]?.cancel(); locating[id] = nil
        if let l = located.removeValue(forKey: id) { try? FileManager.default.removeItem(at: l.file) }
    }

    /// An answer is about to take the air from an entry that was cut: keep that entry's place. (The reducer's `place now` cuts it; the player
    /// is what remembers where.) Read before the engine is stopped.
    private func rememberPlace(of oldId: String?, nextKind: RunSheetKind?) {
        guard nextKind == .answer, remembered == nil, let oldId, let old = store.entry(oldId),
              old.kind != .answer, old.kind != .transition, old.ended?.outcome == .cut else { return }
        if audio != nil, playingDocument == nil, let a = audio, located[old.id] == nil {
            var c = a
            c.start = max(a.start ?? 0, engine?.remotePosition()?.elapsed ?? 0)
            remembered = .audio(c, by: .phone)
            print("AUTORADIO kept place \(old.id) at \(Int(c.start ?? 0)) s while an answer speaks")
        } else if old.wordsComeFromThePhone, let rest = engine?.spokenRemainder(), !rest.isEmpty {
            // A message, book or file: the rest of the passage is held here. The continuation is an entry that names the same address with no
            // words (as the original did); the phone reads the words from its own store.
            let id = "c-" + UUID().uuidString.prefix(8).lowercased()
            var c = RunSheetEntry(id: id, kind: old.kind == .mail ? .mail : .page, placedBy: .phone, title: old.title, estSeconds: RunSheetEntry.estimateSeconds(words: rest))
            c.source = old.source
            provided[id] = (old.title, [rest])
            // The words are also held on disk, so a relaunch before it finishes still has them.
            HeldRest.shared.put(id: id, rest: rest)
            remembered = c
            print("AUTORADIO kept place \(old.id) as \(c.id) on the phone while an answer speaks")
        } else if let rest = engine?.spokenRemainder(), !rest.isEmpty {
            var c = RunSheetEntry(id: "c-" + UUID().uuidString.prefix(8).lowercased(), kind: .story, placedBy: .phone, title: old.title, text: rest,
                                  estSeconds: RunSheetEntry.estimateSeconds(words: rest))
            c.source = old.source; c.cites = old.cites
            remembered = c
            print("AUTORADIO kept place \(old.id) as \(c.id) while an answer speaks")
        }
    }
    /// The listener asked for something else: what was kept is not wanted back.
    private func dropRemembered() { if remembered != nil { print("AUTORADIO kept place dropped: the listener changed what plays") }; remembered = nil }

    private func reconcile() {
        if let air = store.onAir() {
            guard air.id != playingId else { return }
            deckEnd(playingId)                          // what was on air was replaced: the deck's journal says how it left
            rememberPlace(of: playingId, nextKind: air.kind)
            sessionHeldFor = nil; sessionAttempts = 0; retryTask?.cancel(); retryTask = nil; heldMidEntry = false
            playingId = air.id; playingDocument = nil; audio = nil; pendingEnd = nil
            let press = pressPending
            startTask = Task { @MainActor in await self.begin(air, reason: press ? "play" : nil) }
            return
        }
        if playingId != nil {                         // what was on air was skipped, cut or taken off: its sound stops
            deckEnd(playingId)
            if let p = playingId, let f = takeFiles.removeValue(forKey: p) { try? FileManager.default.removeItem(at: f) }
            rememberPlace(of: playingId, nextKind: nextToAir()?.kind)
            sessionHeldFor = nil; sessionAttempts = 0; retryTask?.cancel(); retryTask = nil; heldMidEntry = false
            playingId = nil; playingDocument = nil; audio = nil; paused = false; generation += 1; pendingEnd = nil
            engine?.stopRemote(); engine?.stopSpoken()
        }
        // The answers are over (nothing on air, the next is not an answer): what they cut goes back on air, from the start of its sentence.
        if let r = remembered, playingId == nil, nextToAir()?.kind != .answer {
            remembered = nil
            store.place(r, at: .next, by: .phone, reason: "back to where it was before the answer")
            again = true
            return
        }
        if wants, !spokenPaused(), !turnOpen(), let first = nextToAir() { store.airedOnAir(first.id); again = true }
    }

    private func changed() { onChange?() }

    // MARK: Starting an entry

    /// Seconds between tries when the session cannot be taken (the last repeats).
    nonisolated(unsafe) public static var sessionRetryDelays: [TimeInterval] = [1, 2, 4, 8, 15]
    private var sessionHeldFor: String?
    private var sessionAttempts = 0
    /// The entry is held for the session in the middle of its speech (a block could not sound): when the session is back it goes on from the
    /// block that did not play, and it is never finished, heard or failed, meanwhile.
    private var heldMidEntry = false
    /// The engine was lost at the entry's first block: the entry has made no sound at all.
    private var lostAtFirstBlock = false
    /// The spoken document `documentID` lost the engine at a block: the entry on air is held like one whose start could not take the session.
    public func engineLost(documentID: String, block: Int? = nil) {
        if deck.commanded { deck.engineLost(); return }
        guard let eid = playingId, documentID == playingDocument, let e = store.entry(eid) else { return }
        heldMidEntry = true
        lostAtFirstBlock = block == 0
        Task { @MainActor in await self.hold(e, why: "the audio session could not be taken") }
    }
    private var retryTask: Task<Void, Never>?
    /// The entry on air is waiting for the audio session.
    public var isWaitingForSession: Bool { sessionHeldFor != nil }
    /// The big Play button was pressed. The held entry is retried at once. True when the press was only that: while an entry is held for the
    /// session (at its start or in the middle) the bar says paused, so the press means Play, and it must never be taken for a pause.
    public func playPressed() -> Bool {
        let held = isWaitingForSession
        sessionMayBeBack(reason: "play")
        return held
    }
    /// The lock screen's play, pause or toggle. A press is a press wherever it comes from: while an entry is held for the session, play and toggle
    /// are the in-app Play (`playPressed`). Otherwise remote audio takes the command; false when it is not remote (the programme player does).
    public func remoteTransport(_ cmd: String) -> Bool {
        if isWaitingForSession, cmd == "play" || cmd == "toggle" { _ = playPressed(); return true }
        guard isRemote else { return false }
        switch cmd {
        case "play": resume()
        case "pause": pause()
        default: if paused { resume() } else { pause() }
        }
        return true
    }
    /// The app is back in front, an interruption ended, a route came back, or the listener tapped Play: try the held entry now.
    public func sessionMayBeBack(reason: String = "resume") {
        guard let id = sessionHeldFor, playingId == id, let e = store.entry(id) else { return }
        retryTask?.cancel()
        retryTask = Task { @MainActor in await self.begin(e, reason: reason) }
    }

    /// The session or the engine is not ready: that is not this entry's failure. It stays on air, held, with no outcome recorded; the
    /// claim is tried again with backoff, and at once when the app returns, an interruption ends or the listener taps Play.
    private func hold(_ entry: RunSheetEntry, why: String) async {
        guard playingId == entry.id else { return }
        message = "Can\u{2019}t speak: \(why)"
        sessionHeldFor = entry.id
        sessionAttempts += 1
        let wait = Self.sessionRetryDelays[min(sessionAttempts - 1, Self.sessionRetryDelays.count - 1)]
        print("AUTORADIO on air \(entry.id) HELD \(why); attempt \(sessionAttempts), again in \(wait) s")
        changed()
        retryTask?.cancel()
        retryTask = Task { @MainActor in
            try? await Task.sleep(nanoseconds: UInt64(wait * 1_000_000_000))
            guard !Task.isCancelled, self.sessionHeldFor == entry.id, self.playingId == entry.id else { return }
            await self.begin(entry, reason: "retry")      // gated while another app has the session
        }
    }

    private var pressPending = false
    private var beginSeq = 0
    private func begin(_ entry: RunSheetEntry, reason: String? = nil) async {
        beginSeq += 1
        let seq = beginSeq
        let notReady = await sessionReady?(reason ?? entry.kind.rawValue)
        guard playingId == entry.id else { return }            // a newer start took over while the session was made ready
        guard seq == beginSeq else { return }                  // a later begin of this entry (a retry, Play, the foreground) is the one that goes on
        if let why = notReady { await hold(entry, why: why); return }
        if reason != "play", sessionHeldFor == entry.id, spokenPausedByListener() {
            // The listener paused (or the route was lost) while the entry was held, at its start or in the middle: it stays held, and only their Play
            // goes on. Neither the timer nor the foreground undoes a pause.
            print("AUTORADIO on air \(entry.id) stays held: the listener paused")
            changed(); return
        }
        if heldMidEntry {
            // The session is back: the entry goes on from the block that did not play (not from the start, and not finished).
            heldMidEntry = false
            sessionHeldFor = nil; sessionAttempts = 0; retryTask?.cancel(); retryTask = nil
            message = ""
            paused = false
            print("AUTORADIO on air \(entry.id) resumes at the block that did not play")
            engine?.resumeSpoken()
            changed()
            return
        }
        sessionHeldFor = nil; sessionAttempts = 0; retryTask?.cancel(); retryTask = nil
        guard let engine else { fail(entry.id, why: nil); return }
        generation += 1
        let my = generation
        leadInDone.remove(entry.id)                    // a clip or take played again plays its lead-in and then its audio
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
            if let w = provided[entry.id] ?? HeldRest.shared.row(entry.id).map({ (entry.title, [$0.rest]) }) {
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
        case .take:
            guard let tid = entry.takeID, let fetch = takeAudio else { if my == generation { finish(entry.id, played: false, why: "the take has no id or no way to be fetched") }; return }
            guard let file = await fetch(tid) else { if my == generation, playingId == entry.id { finish(entry.id, played: false, why: "the take\u{2019}s audio could not be had") }; return }
            guard my == generation, playingId == entry.id else { try? FileManager.default.removeItem(at: file); return }
            takeFiles[entry.id] = file
            let a = RunSheetAudio(url: file, title: entry.title, show: entry.source?.label ?? "")
            audio = a
            if let lead = entry.text, !lead.isEmpty {
                let doc = "rs:" + entry.id
                playingDocument = doc
                ok = await engine.speak(documentID: doc, entry: entry, paragraphs: [lead])
            } else {
                ok = await startAudio(a, generation: my)
            }
        case .clip, .episode:
            var spec = entry.audioSpec
            if spec == nil, let address = entry.filesAddress {
                let url: URL?
                if let known = localFiles[entry.id] { url = known } else { url = await fileResolver?(address) }
                if let url { localFiles[entry.id] = url; spec = RunSheetAudio(url: url, title: entry.displayTitle, show: entry.source?.label ?? "Files") }
            }
            guard let a = spec else { break }
            if entry.kind == .clip, entry.source?.clipId != nil {
                // The clip is found in the file the host serves now, in a window fetched once, and plays from those bytes.
                guard let task = locating(entry) else { if my == generation { finish(entry.id, played: false, why: "the clip has no fingerprint, so it is not played as a clip") }; return }
                let r = await task.value
                guard my == generation, playingId == entry.id else { return }
                if case .refused(let why) = r { finish(entry.id, played: false, why: "the clip was not found in the file (\(why))"); return }
            }
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
        deck.began(id: entry.id, kind: audio != nil && playingDocument == nil ? "audio" : "speech")
        changed()
    }

    /// The store says the entry `id` left the air by a cut, a skip or a sign-out: the deck's journal gets the matching end (once).
    private func deckEnd(_ id: String?) {
        guard let id, deck.current?.id == id else { return }
        let r = report(final: false)
        let how: Deck.How
        switch store.entry(id)?.ended?.outcome {
        case .cut?: how = .replaced
        case .left?: how = .stopped
        case .failed?: how = .failed
        default: how = .skipped
        }
        deck.ended(how, block: r.block, seconds: r.seconds, final: engine?.spokenReport().final ?? 0)
        deck.ack(through: deck.journal.seq)             // the store holds the op and keeps it until the server answers
    }

    /// Called about once a second by the host: if sound is now reaching the output for the entry on air, the deck says so (once per entry).
    public func checkSound() {
        guard playingId != nil, engine?.rendering() == true else { return }
        let r = report(final: false)
        deck.soundBegan(block: r.block, seconds: r.seconds, final: engine?.spokenReport().final ?? 0)
    }

    /// The remote half of an audio entry (after its lead-in, if it had one).
    private func startAudio(_ a: RunSheetAudio, generation my: Int) async -> Bool {
        guard let engine else { return false }
        let ok: Bool
        if let id = playingId, let l = located[id] {
            ok = await engine.playRemote(url: l.file, title: a.title, show: a.show, start: nil, end: nil)      // the bytes that were measured; the clip is the whole file
        } else {
            ok = await engine.playRemote(url: a.url, title: a.title, show: a.show, start: a.start, end: a.end)
        }
        if !ok, my == generation { message = "Could not play \u{201C}\(a.title)\u{201D}." }
        return ok
    }

    /// What cannot play is skipped, once, and the radio goes on. Two in a row that made no sound stop it.
    private func fail(_ id: String, why: String?) {
        silent += 1
        if silent >= 2, wants { wants = false; if message.isEmpty { message = "The radio could not play the next entries. Tap play to try again." } }
        finish(id, played: false, why: "failed to start: \(why ?? message)")
    }

    /// The entry ended (or never made a sound): the phone reports it, and the store's change starts the next one.
    private func finish(_ id: String, played: Bool, report given: AirReport? = nil, why: String = "") {
        guard playingId == id else { return }
        print("AUTORADIO entry finished id=\(id) played=\(played)\(why.isEmpty ? "" : " why=\(why)")")
        let r = given ?? report(final: true)
        playingId = nil; playingDocument = nil; audio = nil; paused = false; pendingEnd = nil
        // The deck's journal first (durable, with the device's time), then the store: a kill between the two is closed from the journal at the
        // next launch (RunSheetStore.restore). Heard only if sound was rendered.
        if deck.current?.id != id { deck.began(id: id) }         // an entry that never started is still one line: began and failed
        let line = deck.ended(played ? .finished : .failed, played: played, block: r.block, seconds: r.seconds, final: r.block, reason: played ? "" : "failed to start", id: id)
        defer { if let line { deck.ack(through: line.seq) } }
        // A natural end carries the true final block and position; a failure is `failed` by the phone, never a skip.
        if played { store.airedEnded(id, as: .heard, outcome: .finished, report: r) }
        else { store.airedEnded(id, as: .skipped, outcome: .failed) }
        provided[id] = nil
        HeldRest.shared.remove(id)
        leadInDone.remove(id)
        dropLocated(id)
        if let e = store.entry(id) { onEntryEnded?(e, played) }
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
        if deck.commanded { deck.documentEnded(id: id, anyPlayed: anyPlayed); return }      // shared code commanded this item: the deck plays on
        guard let eid = playingId, id == playingDocument else { return }
        if turnOpen() { pendingEnd = (id, anyPlayed); return }
        // A clip's lead-in is over: the audio plays now.
        if let e = store.entry(eid), e.kind == .clip || e.kind == .take, let a = audio, !leadInDone.contains(eid) {
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
            message = "Can\u{2019}t speak: \(engine?.speakFailure() ?? "the voice made no sound")"
            if silent >= 2 { wants = false }
        }
        finish(eid, played: anyPlayed, why: anyPlayed ? "" : "its document ended with no block heard (\(engine?.speakFailure() ?? "no failure reported"))")
    }

    /// A turn ended: an entry that ended over the listener's words is closed now, and the next one starts.
    public func turnEnded() {
        if let p = pendingEnd { pendingEnd = nil; documentEnded(id: p.id, anyPlayed: p.played) }
        else if wants, store.onAir() == nil { advanceIfIdle() }
    }

    /// Remote audio reached its end, or the clip's end. `generation` is the start it belongs to.
    public func remoteEnded(generation g: Int) {
        if deck.commanded { deck.remoteEnded(); return }
        guard g == generation, audio != nil, let id = playingId else { return }
        let r = report(final: true)                 // before the sound is stopped: the true position
        engine?.stopRemote()
        finish(id, played: true, report: r)
    }

    // MARK: Transport for remote audio

    public var isRemote: Bool { audio != nil && playingDocument == nil }
    public func pause() { guard isRemote else { return }; print("AUTORADIO paused reason=remote entry"); paused = true; engine?.pauseRemote(); store.save(); changed() }
    /// Resume paused remote audio (the engine makes the session ready first: `ChoirPlaybackEngine.resumeRemote`).
    public func resume() { guard isRemote else { return }; paused = false; engine?.resumeRemote(); changed() }
    public func seek(by seconds: Double) { guard isRemote else { return }; engine?.seekRemote(by: seconds); changed() }
    public func seek(to seconds: Double) { guard isRemote else { return }; engine?.seekRemote(to: seconds); changed() }
    public func setRate(_ r: Double) { engine?.setRemoteRate(PlaybackSpeed.clamp(r)) }
    /// The hands-free turn: the radio's own level (dip or pause for the reader's words, play after).
    public func setLevel(_ l: PlayLevel) { if isRemote { engine?.setRemoteLevel(l) } }

    /// "Keep listening": the clip stops being a clip. The episode continues past it, from where it is.
    public func keepListening() {
        guard var a = audio, a.end != nil, playingId.map({ located[$0] == nil }) ?? true else { return }
        a.end = nil
        audio = a
        engine?.clearRemoteEnd()
        store.save(); changed()
    }
    /// The current clip's title, and for a clip the show it is cut from (what "Keep listening" shows).
    public var keepListeningTitle: String? {
        guard let a = audio, a.end != nil, playingId.map({ located[$0] == nil }) ?? true else { return nil }
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

public extension PlaybackEngine { func speakFailure() -> String? { nil }; func spokenRemainder() -> String? { nil } }
