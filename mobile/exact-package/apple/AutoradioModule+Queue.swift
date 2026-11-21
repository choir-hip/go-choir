// Lane file: queue (lane autoradio-queue). The run sheet's sources and sends, declared in interface/queue.json:
//   queueView         the run sheet as the screens read it (what is on air, Up next, Keep listening, the scrubber, the page the bar opened)
//   queueAdd          mode now|next|add, kind story|page, ref, title: `place now`, `place next` or `place queued` on the run sheet
//   queueAct          op toggle|back15|fwd15|seek|keep|remove|playAt|skip|clear|openWeb|closeWeb, arg
// Everything plays through RunSheetPlayer.shared and RunSheetStore.shared (the Kit); other lanes call them directly from their own handlers.
import Foundation

extension AutoradioModule {
    /// The file behind an `app:/cache/files/<hash>/<name>` path from the Files page (exactly two plain parts); nil for any other shape or a missing file.
    func filesAudioURL(local: String) -> URL? {
        let parts = local.dropFirst("app:/cache/files/".count).split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        guard local.hasPrefix("app:/cache/files/"), parts.count == 2, !parts.contains(where: { $0.isEmpty || $0 == "." || $0 == ".." }) else { return nil }
        let url = filesCacheDisk.appendingPathComponent(parts[0], isDirectory: true).appendingPathComponent(parts[1])
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    func queueInfo() -> QueueInfo {
        var q = onMainThread { RunSheetPlayer.shared.info() }
        if !barWebURL.isEmpty { q.webUrl = barWebURL; q.webHost = URL(string: barWebURL)?.host ?? barWebURL }
        return q
    }

    func registerQueue() {
        onMainThread {
            RunSheetPlayer.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["queueView", "radioView", "barFacts"]) } }
            // The station starts by itself when Autoradio opens (after onboarding), and when the app comes back after it was closed.
            Station.shared.isReady = { OnboardingController.shared?.finished ?? false }
        }
        // While remote audio plays, its position moves; while a book or page is read, its block moves (the reader saves it).
        DispatchQueue.main.async { [weak self] in
            let t = Timer(timeInterval: 1.0, repeats: true) { [weak self] timer in
                guard let self else { timer.invalidate(); return }
                self.choir.syncQueuePosition()
                let moving = onMainThread { RunSheetPlayer.shared.isRemote && !RunSheetPlayer.shared.paused }
                if moving { self.touch(["queueView", "radioView", "barFacts"]) }
            }
            RunLoop.main.add(t, forMode: .common)
        }

        Task { @MainActor in
            // The start waits for the front page it speaks from (held, or kept from the last launch, or nothing to wait for), at most 2 s.
            Station.shared.launchCondition = { [weak self] in self?.choir.launchConditionHolds() ?? true }
            await Station.shared.start(reason: .launch)
        }
        // Going to the background: write the next opening ahead (one request; the system gives the app a little time to finish it).
        NotificationCenter.default.addObserver(forName: Platform.wentToBackground, object: nil, queue: .main) { [weak self] _ in
            guard let self else { return }
            Platform.finishInBackground { await self.choir.enteredBackground() }
        }
        NotificationCenter.default.addObserver(forName: Platform.becameActive, object: nil, queue: .main) { _ in
            Task { @MainActor in await Station.shared.start(reason: .foreground) }
        }

        handle("queueView") { [unowned self] _, reply in reply(json(queueInfo())) }

        handle("queueAdd") { [unowned self] args, reply in
            let mode = args["mode"] as? String ?? "add"
            let kind = args["kind"] as? String ?? ""
            let ref = args["ref"] as? String ?? ""
            let title = args["title"] as? String ?? ""
            let local = args["local"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                // An audio file from Files (kind "file", local the app file the page brought): native because only the Swift run sheet player plays an entry, from a URL on disk.
                var found = QueueRequest.entry(kind: kind, ref: ref, title: title)
                if found == nil, kind == "file", ref.hasPrefix("file:"), let url = self.filesAudioURL(local: local) {
                    let e = RunSheetEntry.filesAudio(address: ref, title: title.isEmpty ? (((ref.removingPercentEncoding ?? ref) as NSString).lastPathComponent as NSString).deletingPathExtension : title)
                    RunSheetPlayer.shared.localFiles[e.id] = url; found = e
                }
                if let entry = found {
                    switch mode {
                    case "now":
                        if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                        RunSheetPlayer.shared.playNow(entry)
                    case "next": RunSheetPlayer.shared.playNext(entry)
                    default: RunSheetPlayer.shared.queue(entry)
                    }
                }
                self.touch(["queueView", "radioView", "barFacts"], now: true)
                reply(self.json(self.queueInfo()))
            }
        }

        handle("queueAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? ""
            let arg = args["arg"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                let q = RunSheetPlayer.shared
                switch op {
                case "toggle": if q.isRemote { if q.paused { q.resume() } else { q.pause() } } else { await self.choir.playPause() }
                case "back15": q.seek(by: -15)
                case "fwd15": q.seek(by: 15)
                case "seek": if let f = Double(arg), let d = q.remotePosition()?.duration { q.seek(to: min(max(f, 0), 1) * d) }
                case "keep": q.keepListening()
                case "openEpisode": PodcastBridge.openEpisode(of: q.audio)   // lane autoradio-joins: the page of the episode playing, at the clip's time
                case "remove": if let i = Int(arg) { q.remove(at: i) }
                case "playAt":
                    if let i = Int(arg) {
                        if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                        q.playUpcoming(at: i)
                    }
                case "skip": q.next()                      // the one op: skip(on air)
                case "clear": q.clearUpcoming()
                case "openWeb": self.barWebURL = arg; EmbeddedPage.shared.opened(arg)
                case "closeWeb": self.barWebURL = ""
                default: break
                }
                self.touch(["queueView", "radioView", "barFacts"], now: true)
                reply(self.json(self.queueInfo()))
            }
        }
    }
}
