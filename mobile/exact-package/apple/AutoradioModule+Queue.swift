// Lane file: queue (lane autoradio-queue). The one playback queue's sources and sends, declared in interface/queue.json:
//   queueView         the queue as the screens read it (Up next, Keep listening, the scrubber, the page the bar opened)
//   queueAdd          mode now|next|add, kind story|page, ref, title: put a story or a web page in the queue
//   queueAct          op toggle|back15|fwd15|seek|keep|remove|playAt|skip|clear|openWeb|closeWeb, arg
// Everything plays through PlayQueue.shared (the Kit); other lanes call it directly from their own handlers.
import Foundation
import UIKit

extension AutoradioModule {
    func queueInfo() -> QueueInfo {
        var q = onMainThread { PlayQueue.shared.info() }
        if !barWebURL.isEmpty { q.webUrl = barWebURL; q.webHost = URL(string: barWebURL)?.host ?? barWebURL }
        return q
    }

    func registerQueue() {
        onMainThread {
            // lane autoradio-joins: the DJ's coming segments after the queued items (Up next); never put in the queue.
            PlayQueue.shared.programmeComing = { [weak self] in
                guard let self else { return [] }
                return DJUpcoming.titles(in: self.runtime.player.document, blockIndex: self.runtime.snapshot().blockIndex)
            }
            PlayQueue.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["queueView", "radioView", "barState"]) } }
            // The station starts by itself when Autoradio opens (after onboarding), and when the app comes back after it was closed.
            Station.shared.isReady = { OnboardingController.shared?.finished ?? false }
        }
        // While remote audio plays, its position moves; while a book or page is read, its block moves (the reader saves it).
        DispatchQueue.main.async { [weak self] in
            let t = Timer(timeInterval: 1.0, repeats: true) { [weak self] timer in
                guard let self else { timer.invalidate(); return }
                self.choir.syncQueuePosition()
                let moving = onMainThread { PlayQueue.shared.current?.isRemoteAudio == true && !PlayQueue.shared.paused }
                if moving { self.touch(["queueView", "radioView", "barState"]) }
            }
            RunLoop.main.add(t, forMode: .common)
        }

        Task { @MainActor in
            try? await Task.sleep(nanoseconds: 2_000_000_000)           // the sign-in and the first load get their moment
            await Station.shared.start(reason: .launch)
        }
        NotificationCenter.default.addObserver(forName: UIApplication.willEnterForegroundNotification, object: nil, queue: .main) { _ in
            Task { @MainActor in await Station.shared.start(reason: .foreground) }
        }

        handle("queueView") { [unowned self] _, reply in reply(json(queueInfo())) }

        handle("queueAdd") { [unowned self] args, reply in
            let mode = args["mode"] as? String ?? "add"
            let kind = args["kind"] as? String ?? ""
            let ref = args["ref"] as? String ?? ""
            let title = args["title"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                if let item = QueueRequest.item(kind: kind, ref: ref, title: title) {
                    switch mode {
                    case "now":
                        if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                        PlayQueue.shared.playNow(item)
                    case "next": PlayQueue.shared.playNext(item)
                    default: PlayQueue.shared.enqueue(item)
                    }
                    self.signals.record(SignalEvent(kind: "queue", subject: ref, detail: mode))
                }
                self.touch(["queueView", "radioView", "barState"], now: true)
                reply(self.json(self.queueInfo()))
            }
        }

        handle("queueAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? ""
            let arg = args["arg"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                let q = PlayQueue.shared
                switch op {
                case "toggle": if q.current?.isRemoteAudio == true { if q.paused { q.resume() } else { q.pause() } } else { await self.choir.playPause() }
                case "back15": q.seek(by: -15)
                case "fwd15": q.seek(by: 15)
                case "seek": if let f = Double(arg), let d = q.remotePosition()?.duration { q.seek(to: min(max(f, 0), 1) * d) }
                case "keep": q.keepListening()
                case "openEpisode": PodcastBridge.openEpisode(of: q.current)   // lane autoradio-joins: the page of the episode playing, at the clip's time
                case "remove": if let i = Int(arg) { q.remove(at: i) }
                case "playAt":
                    if let i = Int(arg) {
                        if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                        q.playUpcoming(at: i)
                    }
                case "skip": q.skip()
                case "clear": q.clearUpcoming()
                case "openWeb": self.barWebURL = arg; EmbeddedPage.shared.opened(arg)
                case "closeWeb": self.barWebURL = ""
                default: break
                }
                self.touch(["queueView", "radioView", "barState"], now: true)
                reply(self.json(self.queueInfo()))
            }
        }
    }
}
