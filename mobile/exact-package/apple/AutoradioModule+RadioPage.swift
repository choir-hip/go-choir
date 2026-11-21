// The Autoradio page (docs/design/autoradio-page-2026-11-17.md): the page is shared code (AutoradioLab/radio-page.ts, over radio-model.ts). This file answers its FACTS and carries out its gestures.
//   radioPageFacts   RadioPageFacts: the run sheet as the store holds it, and what only native code knows (the player, the desk's flags, the audio's place, the takes)
//   radioPageAct     op reorder|remove|play|replay, arg the entry's id, to the id it now sits above ("" for the end). Native until the queue and its posted ops move: it changes the sheet,
//                    so it posts ops. It answers {ok}; the page is read again.
// Declared in interface/radio.json.
import Foundation

extension AutoradioModule {
    /// Native because only native code holds these: the sheet (until the queue moves), the player's sentence and the audio's place in it, the desk's state, the takes. It decides nothing.
    @MainActor func radioPageFacts() -> RadioPageFacts {
        let tracker = BarTracker.shared, player = RunSheetPlayer.shared
        let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
        let radio = player.overlay(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route,
                                                     nextTitle: player.store.ahead(limit: 1).first?.displayTitle ?? ""))
        let q = player.info()
        var f = RadioPageFacts()
        f.at = Date().timeIntervalSince1970
        var sheet = player.store.sheet; sheet.appliedOps = []
        f.sheet = (try? JSONEncoder().encode(sheet)).flatMap { String(data: $0, encoding: .utf8) } ?? ""
        f.playing = radio.playing; f.airTitle = radio.title
        f.sentences = radio.sentences; f.sentenceIndex = radio.sentenceIndex; f.progress = radio.progress
        f.remoteAudio = q.audio; f.remoteElapsed = q.elapsed; f.remoteDuration = q.duration
        f.wants = player.wantsToPlay; f.message = q.message; f.notice = q.notice; f.writing = q.writing; f.dry = q.dry
        if let r = player.store.onAir()?.paperRoute { f.publishedAt = PublishedTimes.shared.updated(route: r) }
        let versions = PublishedTimes.shared.versions().sorted { $0.key < $1.key }
        f.versionRoutes = versions.map(\.key); f.versionNumbers = versions.map(\.value)
        f.stale = player.store.sheet.ahead().filter { player.store.isStale($0) }.map(\.id)
        if let id = player.store.onAir()?.takeID, let t = TakeStore.shared.take(id) { f.takeTranscript = t.transcript; f.takeRecord = (t.record ?? TakeRecordSummary()).words }
        return f
    }

    func registerRadioPage() {
        handle("radioPageFacts") { [unowned self] _, reply in reply(json(MainActor.assumeIsolated { self.radioPageFacts() })) }
        handle("radioPageAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? ""
            let id = args["arg"] as? String ?? ""
            let to = args["to"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                let p = RunSheetPlayer.shared
                switch op {
                case "reorder": p.perform(.reorder(id: id, before: to.isEmpty ? nil : to))
                case "remove": p.perform(.remove(id: id))
                case "play":
                    if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                    p.perform(.playNow(id: id))
                case "replay":
                    if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                    p.perform(.replay(id: id))
                default: break
                }
                self.touch(["queueView", "radioView", "barFacts"], now: true)
                reply(["ok": true, "message": ""])
            }
        }
    }
}
