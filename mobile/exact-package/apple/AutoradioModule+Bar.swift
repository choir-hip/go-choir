// Lane 1 (bar, radio, talk): the bar's state (replacing the spine's pure one with the timed tracker), the radio view that
// feeds the expanded player, jump and speed. Registered by registerBar(), after registerSpine(), so `barState` here wins.
import Foundation

extension AutoradioModule {
    func registerBar() {
        let tracker = BarTracker.shared

        // Timed states (error 4 s, revised 6 s) end by themselves: ask once more when the window closes.
        func scheduleExpiry() {
            guard let r = tracker.expiry() else { return }
            DispatchQueue.main.asyncAfter(deadline: .now() + r + 0.1) { [weak self] in self?.touch(["barState", "radioView"]) }
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
                    DispatchQueue.main.async { self.touch(["barState", "radioView"]); scheduleExpiry() }
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
                if s.running && !s.paused { self.touch(["radioView", "barState"]) }
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

        handle("barState") { [unowned self] _, reply in
            let info = MainActor.assumeIsolated { RunSheetPlayer.shared.overlay(tracker.barInfo(player: choir.playerView(), snapshot: runtime.snapshot())) }   // remote audio shows as the radio (lane autoradio-queue)
            scheduleExpiry()
            reply(json(info))
        }

        handle("radioView") { [unowned self] _, reply in
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
            touch(["radioView", "barState"])
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
                self.touch(["radioView", "barState"])
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
