// Lane 1 (bar, radio, talk): the bar's FACTS (the bar's own page model is shared code: AutoradioLab/bar.ts), the radio view that
// feeds the expanded player, jump and speed. Registered by registerBar().
import Foundation

extension AutoradioModule {
    func registerBar() {
        let tracker = BarTracker.shared

        // The radio view's revision line ends by itself: ask once more when the window closes. (The bar's timed states are bar.ts's.)
        func scheduleExpiry() {
            guard let r = tracker.expiry() else { return }
            DispatchQueue.main.asyncAfter(deadline: .now() + r + 0.1) { [weak self] in self?.touch(["radioView"]) }
        }

        // The engine's events feed the tracker: holding lines, revisions, errors.
        // The module lives as long as the app (Exact makes it once per session), so these listeners are app-lifetime by design; they
        // also end on their own once the module is gone (`guard let self`), and the timer invalidates itself.
        let stream = runtime.log.stream()
        Task { [weak self] in
            for await te in stream {
                guard let self else { break }
                tracker.observe(te.event)
                switch te.event {
                case .holdingSaid, .revisionArrived, .error, .phase, .yielded:
                    DispatchQueue.main.async { self.touch(["barFacts", "radioView"]); scheduleExpiry() }
                default: break
                }
            }
        }

        // While the programme plays, the position inside the block moves: one gentle refresh, ~2.5 per second.
        DispatchQueue.main.async { [weak self] in
            var lastBlock = -1
            let t = Timer(timeInterval: 0.4, repeats: true) { [weak self] timer in
                guard let self else { timer.invalidate(); return }
                let s = self.runtime.snapshot()
                if s.running && !s.paused { self.touch(["radioView", "barFacts"]) }
                if s.blockIndex != lastBlock { lastBlock = s.blockIndex; self.touch(["queueView"]) }   // lane autoradio-joins: the DJ's coming segments moved on
            }
            RunLoop.main.add(t, forMode: .common)
        }

        // The microphone's level feeds the mark (MarkAt level=): a gentle redraw only when it moved.
        let micFrames = runtime.bus.mic()
        Task { [weak self] in
            var lastTouch = Date.distantPast
            for await f in micFrames {
                guard let self else { break }
                if tracker.feedMic(db: f.rmsDb), Date().timeIntervalSince(lastTouch) > 0.09 {
                    lastTouch = Date()
                    DispatchQueue.main.async { self.touch(["radioView"]) }
                }
            }
        }

        // Native because only native code knows these: the engine's phase and words, its events (stamped with the device's clock), the player and the run sheet player.
        // It decides nothing: bar.ts makes the state, the words and the timed windows from them. `at` is the clock bar.ts uses (a data source may not read time).
        handle("barFacts") { [unowned self] _, reply in
            let facts = MainActor.assumeIsolated { self.barFacts() }
            reply(json(facts))
        }

        handle("radioView") { [unowned self] _, reply in
            scheduleExpiry()
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            let info = MainActor.assumeIsolated { RunSheetPlayer.shared.overlay(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route, nextTitle: RunSheetStore.shared.ahead(limit: 1).first?.displayTitle ?? "")) }   // lane autoradio-queue
            ReaderFeed.shared.feed(info, route: route)
            reply(json(info))
        }

        // Jump to a block of the programme (the player's segments and the up-next rows); in a programme of segments, to a segment.
        handle("radioJump") { [unowned self] args, reply in
            if let i = Int(args["index"] as? String ?? "") {
                if !runtime.player.document.segments.isEmpty { runtime.player.jumpToSegment(i) }
                else if runtime.player.document.blocks.indices.contains(i) { runtime.player.start(at: i) }
            }
            touch(["radioView", "barFacts"])
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            reply(json(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route, nextTitle: onMainThread { RunSheetStore.shared.ahead(limit: 1).first?.displayTitle ?? "" })))
        }

        // Next or previous (move "next" or "previous"): the run sheet's `skip(on air)`, and its `previous` (the start of this entry when it has
        // gone on a little, else the entry before it). The paper's list is never asked.
        handle("radioStory") { [unowned self] args, reply in
            let move: ChoirApp.StoryMove = (args["move"] as? String) == "previous" ? .previous : .next
            Task { [weak self] in
                guard let self else { return }
                await self.choir.moveStory(move)
                self.touch(["radioView", "barFacts"])
                let route = ChoirEnvironment.shared.playing(self.runtime.player.document.id)?.route ?? ""
                DispatchQueue.main.async { reply(self.json(tracker.radioInfo(player: self.choir.playerView(), snapshot: self.runtime.snapshot(), document: self.runtime.player.document, route: route, nextTitle: onMainThread { RunSheetStore.shared.ahead(limit: 1).first?.displayTitle ?? "" }))) }
            }
        }

        // Reading speed, 0.5 to 3 in steps of 0.05. The tracker clamps and snaps it, hands it to the player (time-pitch in the audio
        // engine, pitch kept, for every voice), and it is kept for the next launch. The saved speed is applied now.
        if let saved = SpeedStore.standard.load() { tracker.setSpeed(saved) }
        handle("radioSpeed") { [unowned self] args, reply in
            if let v = Double(args["speed"] as? String ?? "") { tracker.setSpeed(v); SpeedStore.standard.save(tracker.speed) }
            touch(["radioView"])
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            reply(json(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route, nextTitle: onMainThread { RunSheetStore.shared.ahead(limit: 1).first?.displayTitle ?? "" })))
        }
    }
}

/// Lane 1 feeds the reader: the sentence the radio is saying, by address, so the reader lights it (ReaderFollow, lane 3).
/// Only a change is passed on; when nothing plays the follow is cleared.
/// The bar's facts as the shared bar page model reads them (interface/bar.json `BarFacts`).
struct BarFacts: Codable {
    var at = 0.0
    var phase = "playing", transcript = ""
    var blockIndex = 0, blockCount = 0
    var blockFraction = 0.0
    var running = false, signedIn = false, playing = false
    var title = "", message = ""
    var handsFree = false
    var holding = ""
    var revisedAt = 0.0, errorAt = 0.0
    var errorStage = "", errorMessage = ""
    /// The run sheet player's: something is on air, it is held for the audio session, remote audio is the current item (its play state, title and place).
    var onAir = false, waiting = false, remote = false, remotePlaying = false
    var remoteTitle = ""
    var remoteFraction = 0.0
}

extension AutoradioModule {
    @MainActor func barFacts() -> BarFacts {
        let s = runtime.snapshot(), p = choir.playerView(), raw = BarTracker.shared.raw(), player = RunSheetPlayer.shared, q = player.info()
        var f = BarFacts()
        f.at = Date().timeIntervalSince1970
        f.phase = s.phase; f.transcript = s.transcript
        f.blockIndex = s.blockIndex; f.blockCount = s.blockCount; f.blockFraction = BarTracker.shared.blockFraction(s)
        f.running = s.running; f.signedIn = p.signedIn; f.playing = p.playing; f.title = p.title; f.message = p.message
        f.handsFree = HandsFreeSwitch.shared.isOn
        f.holding = raw.holding; f.revisedAt = raw.revisedAt; f.errorAt = raw.errorAt; f.errorStage = raw.errorStage; f.errorMessage = raw.errorMessage
        f.onAir = player.store.onAir() != nil; f.waiting = player.isWaitingForSession
        f.remote = player.isRemote; f.remotePlaying = q.playing; f.remoteTitle = q.current; f.remoteFraction = q.fraction
        return f
    }
}

final class ReaderFeed: @unchecked Sendable {
    static let shared = ReaderFeed()
    private let lock = NSLock()
    private var last = ""
    func feed(_ r: RadioInfo, route: String) {
        let spoken = r.playing && !route.isEmpty && r.sentences.indices.contains(r.sentenceIndex) ? r.sentences[r.sentenceIndex] : ""
        let key = spoken.isEmpty ? "" : "pub:\(route)|\(spoken)"
        lock.lock(); let changed = key != last; last = key; lock.unlock()
        guard changed else { return }
        if key.isEmpty { ReaderFollow.shared.clear() } else { ReaderFollow.shared.set(address: "pub:\(route)", spoken: spoken) }
    }
}
