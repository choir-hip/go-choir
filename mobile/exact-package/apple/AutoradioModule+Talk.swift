// Lane file: talk (lane autoradio-talk). The conversation (what was asked and what came back), the bar's answer card actions, and the
// queue-by-voice adapter. Sources and sends are declared in interface/talk.json (and the card's fields in BarInfo, interface/spine.json).
//   conversationInfo           the conversation page: every question and its answer, newest first
//   answerAction(action, id)   the card's buttons: "retry" asks the same words again, "dismiss" hides the card, "cancel" stops the turn
import Foundation
import ExactPackage

extension AutoradioModule {
    func registerTalk() {
        Conversation.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["barState", "conversationInfo"], now: true) } }

        // The queue by voice (owner, 2026-11-15). The paper's list is what the DJ may add; the rest goes through the run sheet
        // (Kit/TurnQueueWire.swift): `turnQueueJSON()` goes out with every turn as the `queue` field, the `queue_ops` event comes back whole to
        // `applyTurnOps`, which translates the old words onto run sheet ops.
        var hooks = QueueVoice.hooks
        hooks.candidates = { [choir] in (choir?.storyRows() ?? []).prefix(40).map { (route: $0.route, title: $0.title) } }
        // Plain commands are run sheet ops by the listener: "skip" is `skip(on air)`, the same op as the next button, a swipe on the
        // card and the lock screen's next; "back" is `previous`; "play this next" is `place next`, "queue this" is `place queued`.
        hooks.perform = { [weak self] ops, front in
            guard let self, let op = ops.first else { return false }
            switch op {
            case .pause: self.runtime.player.pause(); return true
            case .resume: await self.choir.startPlaying(); return true
            case .skip: await self.choir.moveStory(.next); return true
            case .previous: await self.choir.moveStory(.previous); return true
            case .clear: await MainActor.run { RunSheetPlayer.shared.clearUpcoming() }; return true
            case .playNext(.front), .enqueue(.front):
                // Only a paper story in front can be queued by "this" tonight; a page, book or episode in front falls through to the desk.
                guard let f = front, f.address.hasPrefix("pub:") else { return false }
                let entry = RunSheetEntry.paperStory(route: String(f.address.dropFirst(4)), title: f.title)
                await MainActor.run { if case .playNext = op { RunSheetPlayer.shared.playNext(entry) } else { RunSheetPlayer.shared.queue(entry) } }
                return true
            }
        }
        hooks.snapshot = { await MainActor.run { RunSheetPlayer.shared.turnQueueJSON() } }       // the wire's `queue`
        hooks.applyServerOps = { json in _ = await MainActor.run { RunSheetPlayer.shared.applyTurnOps(json) } }  // dedupe by turn, decoding, rebase: the sheet's
        // "Play that", "open the second one": the items the last answer retrieved (Kit/RetrievedCommand.swift).
        hooks.actOnRetrieved = { [weak self] action, item in
            guard let self else { return false }
            switch action {
            case .open:
                await MainActor.run { _ = self.pages.open(item.address, title: item.title) }
                return true
            case .play, .queue:
                guard let entry = RetrievedCommand.entry(for: item) else {
                    if action == .play { await MainActor.run { _ = self.pages.open(item.address, title: item.title) }; return true }     // it cannot play: it opens
                    return false
                }
                await MainActor.run { if action == .play { RunSheetPlayer.shared.playNow(entry) } else { RunSheetPlayer.shared.queue(entry) } }
                return true
            }
        }
        QueueVoice.hooks = hooks
        SavedCorpus.shared.installRunSheet()

        // The reader's voice (lane autoradio-talk): the Core ML model is fetched quietly (about 8 MB) and, once loaded, the gate listens.
        // Learned from what the reader says to the radio; stored on this phone only (SpeakerStore); never uploaded.
        SpeakerGate.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["voiceProfile"]) } }
        Task { let e = WeSpeakerEmbedder.standard(); if await e.ensureReady() { SpeakerGate.shared.embedder = e } }
        handle("voiceProfile") { [unowned self] _, reply in reply(self.json(SpeakerGate.shared.info())) }
        handle("voiceLearn") { _, reply in SpeakerGate.shared.forget(); reply(["ok": true, "message": ""]) }
        handle("voiceForget") { _, reply in SpeakerGate.shared.forget(); reply(["ok": true, "message": ""]) }

        handle("conversationInfo") { [unowned self] _, reply in reply(self.json(ConversationInfo.make(Conversation.shared.entries()))) }
        handle("answerAction") { [unowned self] args, reply in
            let action = args["action"] as? String ?? "", id = args["id"] as? String ?? ""
            switch action {
            case "dismiss": Conversation.shared.dismiss()
            case "cancel": runtime.cancelTalk()
            case "retry":
                let q = (id.isEmpty ? Conversation.shared.entries().last : Conversation.shared.entry(id))?.question ?? ""
                if !q.isEmpty { Task { [weak self] in _ = await self?.runtime.ask(q, pageAddress: "") } }
            default: break
            }
            reply(["ok": true, "message": ""])
        }
    }
}
