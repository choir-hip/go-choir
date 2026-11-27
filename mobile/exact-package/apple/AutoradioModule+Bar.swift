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
                if s.running && !s.paused { self.touch(["radioView", "barFacts"], pump: false) }
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
                    DispatchQueue.main.async { self.touch(["radioView"], pump: false) }
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
            let route = RadioBridge.shared.airAddress()
            let info = MainActor.assumeIsolated { (tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document)) }   // lane autoradio-queue
            ReaderFeed.shared.feed(info, address: route)
            reply(json(info))
        }

        // Reading speed, 0.5 to 3 in steps of 0.05. The tracker clamps and snaps it, hands it to the player (time-pitch in the audio
        // engine, pitch kept, for every voice), and it is kept for the next launch. The saved speed is applied now.
        if let saved = SpeedStore.standard.load() { tracker.setSpeed(saved) }
        handle("radioSpeed") { [unowned self] args, reply in
            if let v = Double(args["speed"] as? String ?? "") { tracker.setSpeed(v); SpeedStore.standard.save(tracker.speed) }
            touch(["radioView"])
            reply(json(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document)))
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
    var running = false, playing = false
    var title = "", message = ""
    var handsFree = false
    var holding = ""
    var revisedAt = 0.0, errorAt = 0.0
    var errorStage = "", errorMessage = ""
}

extension AutoradioModule {
    @MainActor func barFacts() -> BarFacts {
        let s = runtime.snapshot(), p = choir.playerView(), raw = BarTracker.shared.raw()
        var f = BarFacts()
        f.at = Date().timeIntervalSince1970
        f.phase = s.phase; f.transcript = s.transcript
        f.blockIndex = s.blockIndex; f.blockCount = s.blockCount; f.blockFraction = BarTracker.shared.blockFraction(s)
        f.running = s.running; f.playing = p.playing; f.title = p.title; f.message = p.message
        f.handsFree = HandsFreeSwitch.shared.isOn
        f.holding = raw.holding; f.revisedAt = raw.revisedAt; f.errorAt = raw.errorAt; f.errorStage = raw.errorStage; f.errorMessage = raw.errorMessage
        return f
    }

    /// What only the audio code knows, for the radio's pump (shared code: radio-bridge.ts `readFacts`), and the inbox of what native callers and the voice turn handed over (RadioBridge). It decides nothing.
    /// `at` is the device's time in milliseconds (a data source may not read the clock); `ready`: onboarding is complete and no other audio holds the phone, so the radio may start by itself.
    /// `foreground`: the app is in front. Only native knows it on a locked iPhone, because Exact does not establish that the page turns `hidden` there (docs/reference/exact/ui-model.md "Page facts"); shared code stops re-asking the screens
    /// each second while it is false (spine.contract `posKey`). A host that does not give it is taken as in front.
    @MainActor public func radioFacts(peek: Bool) -> [String: Any] {
        let s = runtime.snapshot(), tracker = BarTracker.shared, deck = Deck.shared
        let info = tracker.radioInfo(player: choir.playerView(), snapshot: s, document: runtime.player.document)
        let pos = runtime.player.storyPosition()
        var o: [String: Any] = ["at": Date().timeIntervalSince1970 * 1000, "turnOpen": s.phase != "playing", "remainder": runtime.player.remainingText() ?? "",
                                "position": ["block": pos.block, "seconds": pos.seconds], "wants": !runtime.player.userPaused, "foreground": !runtime.coordinator.isBackgrounded, "ready": OnboardingController.shared?.finished ?? false,
                                "sentences": info.sentences, "sentenceIndex": info.sentenceIndex, "progress": info.progress, "inbox": peek ? [] : RadioBridge.shared.drain()]
        if let c = deck.current, c.kind == "audio", let r = deck.engine?.remotePosition() {
            o["audio"] = ["url": c.url, "title": c.title, "show": c.show, "start": c.start, "elapsed": r.elapsed] as [String: Any]
            o["remote"] = ["elapsed": r.elapsed, "duration": r.duration ?? 0] as [String: Any]
        }
        return o
    }
}
