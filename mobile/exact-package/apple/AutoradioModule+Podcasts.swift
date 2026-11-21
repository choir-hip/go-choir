// The podcast app is shared code (AutoradioLab/podcast.ts, podcast-feed.ts, podcast-model.ts, clips.ts; pages podcasts.contract, clips.contract). What
// stays native here is only what the platform alone knows, and each name has its reason (also in scripts/autoradio-native-voice-list.txt):
//   podcastNow         what the player is playing and where: only the player knows the playhead and the clip's start.
//   podcastPlayAudio   hands one audio address to the player (play now, add to the queue, keep listening to a clip): the audio is native. When the deck's
//                      `deckPlay` and `deckEnqueue` land, the page calls those and this name goes (same fields).
//   the page titles    the deck's pile shows a title for a podcast page; the page's own header is the shared one, so this says only "Show" or "Episode".
//   the bridge         "Open episode" on the playing bar names an episode page (the queue's Swift still decides when it is offered).
import Foundation

struct PodcastPageSource: PageSource {
    func handles(_ address: String) -> Bool { address.hasPrefix("podcast:") || address.hasPrefix("podcasts:") }
    func load(_ address: String) async throws -> PageModel {
        let title = address == "podcasts:" ? "Podcasts" : address.hasPrefix("podcast:show/") ? "Show" : "Episode"
        return PageModel(address: address, kind: "podcast", title: title, status: "ready")
    }
}

extension AutoradioModule {
    func registerPodcasts() {
        pageSources.register(PodcastPageSource())
        // "Open episode" from a clip or the playing bar opens that episode's page at the clip's time (the address carries the audio and the start).
        Task { @MainActor in
            PodcastBridge.open = { [weak self] episodeId, feedURL, enclosureURL, at in
                guard let feedURL else { return }
                let enclosure = PodcastKey.webURL(enclosureURL.absoluteString)
                let address = PodcastKey.episodeAddress(feed: feedURL.absoluteString, id: episodeId ?? enclosure, enclosure: enclosure, at: at)   // no guid: the enclosure is the id (the feed's own rule)
                DispatchQueue.main.async { self?.requestPage(address, title: "Episode") }
            }
        }

        handle("podcastNow") { [unowned self] _, reply in reply(self.json(self.podcastNowView())) }
        // While a podcast plays its playhead moves: the pages' playhead argument is asked again every second, only then.
        DispatchQueue.main.async { [weak self] in
            let t = Timer(timeInterval: 1.0, repeats: true) { [weak self] timer in
                guard let self else { timer.invalidate(); return }
                if !self.podcastNowView().episodeId.isEmpty { self.touch(["podcastNow"]) }
            }
            RunLoop.main.add(t, forMode: .common)
        }

        handle("podcastPlayAudio") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", id = args["id"] as? String ?? "", urlString = args["url"] as? String ?? ""
            let title = args["title"] as? String ?? "", show = args["show"] as? String ?? ""
            let start = (args["start"] as? NSNumber)?.doubleValue ?? 0
            Task { @MainActor [weak self] in
                guard let self else { return }
                if op == "keep" { Deck.shared.engine?.clearRemoteEnd(); reply(self.json(PodDone())); return }
                guard let url = URL(string: urlString), PodcastKey.webURL(urlString) != "" else { reply(self.json(PodDone(status: "error", message: "This episode has no audio to play."))); return }
                let entry = RunSheetEntry.audio(RunSheetAudio(url: url, title: title.isEmpty ? "Episode" : title, show: show, start: start > 0 ? start : nil, end: nil, episodeId: id.isEmpty ? nil : id))
                if op == "queue" { RadioBridge.shared.place(entry, mode: "add") }
                else {
                    if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                    RadioBridge.shared.place(entry, mode: "now")
                }
                self.touch(["queueView", "radioView", "barFacts", "podcastNow"], now: true)
                reply(self.json(PodDone()))
            }
        }
    }

    /// What the queue's engine is playing, when it is a podcast (its episode id is a podcast key); empty otherwise.
    func podcastNowView() -> PodNowView {
        onMainThread {
            guard Deck.shared.current?.kind == "audio", let a = RadioBridge.shared.onAirEntry()?.audioSpec, let id = a.episodeId, PodcastKey.parseEpisode(id) != nil else { return PodNowView() }
            let p = Deck.shared.snapshot()
            return PodNowView(episodeId: id, clip: a.end != nil, start: a.start ?? 0, elapsed: p.seconds, duration: p.duration)
        }
    }
}

struct PodNowView: Codable { var episodeId = "", clip = false, start = 0.0, elapsed = 0.0, duration = 0.0 }
struct PodDone: Codable { var status = "ok", message = "" }
