// The Autoradio page (docs/design/autoradio-page-2026-11-17.md): one source, `radioPage`, which is the run sheet made readable, and one
// send, `radioPageAct`, for the gestures on it. The model is the Kit's (RadioPage.swift); this file only gathers its input.
//   radioPage      RadioPageInfo: the status line, the card on air, up next, suggestions, heard
//   radioPageAct   op reorder|remove|play|replay, arg the entry's id, to the id it now sits above ("" for the end)
// Declared in interface/radio.json. It reads the run sheet and sends the ops the bar and the old sheet sent; it keeps no state of its own.
import Foundation

extension AutoradioModule {
    func radioPageInfo() -> RadioPageInfo {
        let tracker = BarTracker.shared
        return onMainThread {
            let player = RunSheetPlayer.shared
            let route = ChoirEnvironment.shared.playing(runtime.player.document.id)?.route ?? ""
            let radio = player.overlay(tracker.radioInfo(player: choir.playerView(), snapshot: runtime.snapshot(), document: runtime.player.document, route: route,
                                                         nextTitle: player.store.ahead(limit: 1).first?.displayTitle ?? ""))
            let q = player.info()
            var i = RadioPageInput(sheet: player.store.sheet)
            // The page dims and says "Listening" while a turn is open: the engine's phase, read here (the bar's own states are bar.ts's).
            let snap = runtime.snapshot()
            if ["candidate", "addressed", "capturing", "ended"].contains(snap.phase) { i.barState = "listening"; i.heardWords = snap.transcript.isEmpty ? "Listening…" : snap.transcript }
            i.playing = radio.playing
            i.sentences = radio.sentences; i.sentenceIndex = radio.sentenceIndex; i.progress = radio.progress
            i.remoteAudio = q.audio; i.elapsedLabel = q.elapsedLabel; i.remainingLabel = q.remainingLabel
            i.wants = player.wantsToPlay; i.message = q.message; i.notice = q.notice; i.writing = q.writing; i.dry = q.dry
            if let r = player.store.onAir()?.paperRoute { i.published = PublishedTimes.shared.words(route: r) }
            i.versions = PublishedTimes.shared.versions()
            i.takeLookup = { TakeStore.shared.take($0) }
            i.stale = Set(player.store.sheet.ahead().filter { player.store.isStale($0) }.map(\.id))
            return RadioPage.make(i)
        }
    }

    func registerRadioPage() {
        handle("radioPage") { [unowned self] _, reply in reply(json(radioPageInfo())) }
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
                reply(self.json(self.radioPageInfo()))
            }
        }
    }
}
