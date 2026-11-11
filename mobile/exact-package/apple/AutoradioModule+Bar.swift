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
            let t = Timer(timeInterval: 0.4, repeats: true) { [weak self] timer in
                guard let self else { timer.invalidate(); return }
                let s = self.runtime.snapshot()
                if s.running && !s.paused { self.touch(["radioView", "barState"]) }
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
            let info = tracker.barInfo(player: choir.playerView(), snapshot: runtime.snapshot())
            scheduleExpiry()
            reply(json(info))
        }

        handle("radioView") { [unowned self] _, reply in
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            let info = tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route)
            ReaderFeed.shared.feed(info, route: route)
            reply(json(info))
        }

        // Jump to a block of the programme (the player's segments and the up-next rows).
        handle("radioJump") { [unowned self] args, reply in
            if let i = Int(args["index"] as? String ?? ""), runtime.player.document.blocks.indices.contains(i) {
                runtime.player.start(at: i)
            }
            touch(["radioView", "barState"])
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            reply(json(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route)))
        }

        // Reading speed. Stored and used for the sentence estimate; the audio side applies it once the engine takes a rate
        // (see "Requests to the spine" in the lane report).
        handle("radioSpeed") { [unowned self] args, reply in
            if let v = Double(args["speed"] as? String ?? "") { tracker.setSpeed(v) }
            touch(["radioView"])
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            reply(json(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route)))
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
