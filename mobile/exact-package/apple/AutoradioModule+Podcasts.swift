// Lane file: podcasts (autoradio-podcasts). Sources and sends are declared in exact-package/interface/podcasts.json; the pages are
// AutoradioLab/podcasts.contract; the data and the logic are Kit/PodcastService.swift and Kit/PodcastFeed.swift (tested there).
//
// This file is also THE ADAPTER to the shared playback queue: the only place the podcast app names the run sheet.
// If the run sheet's API changes, change `registerPodcastQueueAdapter` and nothing else.
import Foundation

/// Page model for podcast pages (titles in the deck and the pile).
struct PodcastPageSource: PageSource {
    let service: PodcastService
    func handles(_ address: String) -> Bool { address.hasPrefix("podcast:") || address.hasPrefix("podcasts:") }
    func load(_ address: String) async throws -> PageModel {
        if address == PodcastService.homeAddress { return PageModel(address: address, kind: "podcast", title: "Podcasts", status: "ready") }
        if PodcastService.parseShow(address) != nil {
            let v = service.show(address: address)
            return PageModel(address: address, kind: "podcast", title: v.title.isEmpty ? "Show" : v.title, status: v.status == "loading" ? "loading" : "ready", message: v.message)
        }
        let v = service.episode(address: address)
        return PageModel(address: address, kind: "podcast", title: v.title.isEmpty ? "Episode" : v.title, status: v.status == "loading" ? "loading" : "ready", message: v.message)
    }
}

extension AutoradioModule {
    func registerPodcasts() {
        let service = PodcastService.shared
        SavedCorpus.shared.setSource("podcasts") { PodcastService.shared.retrievalDocuments() }   // shows, cached episodes and fetched transcripts
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        service.configure(directory: support)
        if !Demo.isOn { service.syncHooks = PodcastSyncCodec.hooks(); service.platform = PodcastPlatformCodec.hooks() }
        service.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["podcastHome", "podcastShow", "podcastEpisode", "launcherData"]) } }
        service.opener = { [weak self] address, title in DispatchQueue.main.async { self?.pages.open(address, title: title) } }
        pageSources.register(PodcastPageSource(service: service))
        registerPodcastQueueAdapter(service)
        // lane autoradio-joins: "Open episode" from a clip or the player opens that episode's page at the clip's time.
        Task { @MainActor in
            PodcastBridge.open = { episodeId, feedURL, enclosureURL, at in
                guard let feedURL else { return }
                let enclosure = PodcastText.webURL(enclosureURL.absoluteString)
                PodcastEpisodes.open(episodeId: episodeId ?? enclosure, feedURL: feedURL.absoluteString, enclosureURL: enclosure, at: at)   // no guid: the enclosure is the id (the feed's own rule)
            }
        }
        // One line: podcasts as a group in the universal search (the launcher merges `SearchProviders` into its hits).
        SearchProviders.register(name: "podcasts") { PodcastService.shared.searchHits($0) }

        handle("podcastHome") { args, reply in
            let address = args["address"] as? String ?? ""
            guard address == PodcastService.homeAddress else { var v = PodcastService.HomeView(); v.status = "idle"; reply(self.json(v)); return }
            reply(self.json(service.home(query: args["q"] as? String ?? "")))
        }
        handle("podcastShow") { args, reply in
            let a = args["address"] as? String ?? ""
            guard PodcastService.parseShow(a) != nil else { reply(self.json(PodcastService.ShowView())); return }
            reply(self.json(service.show(address: a)))
        }
        handle("podcastEpisode") { args, reply in
            let a = args["address"] as? String ?? ""
            guard PodcastService.parseEpisode(a) != nil else { reply(self.json(PodcastService.EpisodeView())); return }
            reply(self.json(service.episode(address: a, find: args["find"] as? String ?? "")))
        }
        handle("podcastSubscribe") { [weak self] args, reply in
            let feed = args["feedURL"] as? String ?? ""
            let did = service.subscribe(feedURL: feed)
            self?.touch(["podcastHome", "podcastShow", "podcastEpisode"])
            reply(self?.json(PodcastService.Ack(status: "ok", message: did ? "Subscribed. New episodes will show here." : "You already subscribe to this show.")) ?? [:])
        }
        handle("podcastUnsubscribe") { [weak self] args, reply in
            let did = service.unsubscribe(args["feedURL"] as? String ?? "")
            self?.touch(["podcastHome", "podcastShow", "podcastEpisode"])
            reply(self?.json(PodcastService.Ack(status: "ok", message: did ? "Unsubscribed on this phone." : "")) ?? [:])
        }
        handle("podcastPlay") { [weak self] args, reply in
            let a = service.play(address: args["address"] as? String ?? "", mode: args["mode"] as? String ?? "resume")
            self?.touch(["podcastShow", "podcastEpisode"])
            reply(self?.json(a) ?? [:])
        }
        handle("podcastQueue") { [weak self] args, reply in reply(self?.json(service.enqueue(address: args["address"] as? String ?? "")) ?? [:]) }
        handle("podcastRefresh") { [weak self] args, reply in
            if let feed = PodcastService.parseShow(args["address"] as? String ?? "") ?? PodcastService.parseEpisode(args["address"] as? String ?? "")?.feed { service.refreshIfNeeded(feed, force: true) }
            reply(self?.json(PodcastService.Ack()) ?? [:])
        }

        // While a podcast plays, keep its playhead every 5 s (the resume position), and when the pages are read.
        Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { _ in service.sample() }
    }

    /// The adapter: podcast plays go to the one queue and nothing else. Positions are read from the queue's engine.
    private func registerPodcastQueueAdapter(_ service: PodcastService) {
        func onMain<T>(_ f: @MainActor () -> T) -> T {
            if Thread.isMainThread { return MainActor.assumeIsolated { f() } }
            return DispatchQueue.main.sync { MainActor.assumeIsolated { f() } }
        }
        service.playHook = { req, enqueue in
            DispatchQueue.main.async {
                MainActor.assumeIsolated {
                    let entry = RunSheetEntry.audio(RunSheetAudio(url: req.url, title: req.title, show: req.show, start: req.start, end: req.end, episodeId: req.episodeId))
                    if enqueue { RunSheetPlayer.shared.queue(entry) } else { RunSheetPlayer.shared.playNow(entry) }
                }
            }
        }
        service.nowHook = {
            onMain {
                guard let a = RunSheetPlayer.shared.audio, RunSheetPlayer.shared.isRemote, let id = a.episodeId, PodcastKey.parseEpisode(id) != nil else { return nil }
                let p = RunSheetPlayer.shared.remotePosition()
                return PodcastNow(episodeId: id, isClip: a.end != nil, start: a.start, elapsed: p?.elapsed, duration: p?.duration)
            }
        }
        service.keepHook = { DispatchQueue.main.async { MainActor.assumeIsolated { RunSheetPlayer.shared.keepListening() } } }
    }
}
