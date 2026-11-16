// Lane file: talk (lane autoradio-talk). The conversation (what was asked and what came back), the bar's answer card actions, and the
// queue-by-voice adapter. Sources and sends are declared in interface/talk.json (and the card's fields in BarInfo, interface/spine.json).
//   conversationInfo           the conversation page: every question and its answer, newest first
//   answerAction(action, id)   the card's buttons: "retry" asks the same words again, "dismiss" hides the card, "cancel" stops the turn
import Foundation
import ExactPackage
import AutoradioKit

extension AutoradioModule {
    func registerTalk() {
        Conversation.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["barState", "conversationInfo"], now: true) } }

        // The queue by voice (owner, 2026-11-15). The paper's list is what the DJ may add; the rest of the queue goes through the queue
        // lane's PlayQueue (queue-wire.md): `snapshotJSON()` goes out with every turn, the `queue_ops` event comes back whole to `applyQueueOpsEvent`.
        var hooks = QueueVoice.hooks
        hooks.candidates = { [choir] in (choir?.storyRows() ?? []).prefix(40).map { (route: $0.route, title: $0.title) } }
        // Plain commands go through the queue's one mutation, `PlayQueue.apply(_:by:)`, as the reader. A count of 0 (nothing queued to
        // skip, no history to go back to) falls back to the paper's own list, which is what the lock screen's next and previous do.
        hooks.perform = { [weak self] ops, front in
            guard let self, let op = ops.first else { return false }
            switch op {
            case .pause: self.runtime.player.pause(); return true
            case .resume: await self.choir.startPlaying(); return true
            case .skip:
                let n = await MainActor.run { PlayQueue.shared.apply([.skip()], by: .reader) }
                if n == 0 { await self.choir.moveStory(.next) }
                return true
            case .previous:
                let n = await MainActor.run { PlayQueue.shared.apply([.previous], by: .reader) }
                if n == 0 { await self.choir.moveStory(.previous) }
                return true
            case .clear: _ = await MainActor.run { PlayQueue.shared.apply([.clear], by: .reader) }; return true
            case .playNext(.front), .enqueue(.front):
                // Only a paper story in front can be queued by "this" tonight; a page, book or episode in front falls through to the desk.
                guard let f = front, f.address.hasPrefix("pub:") else { return false }
                let item = PlayItem.story(route: String(f.address.dropFirst(4)), title: f.title)
                let queued: QueueOp = { if case .playNext = op { return .playNext(item) }; return .enqueue(item) }()
                _ = await MainActor.run { PlayQueue.shared.apply([queued], by: .reader) }
                return true
            }
        }
        hooks.snapshot = { await MainActor.run { PlayQueue.shared.snapshotJSON().data(using: .utf8) } }       // the wire's `queue`
        hooks.applyServerOps = { json in _ = await MainActor.run { PlayQueue.shared.applyQueueOpsEvent(json) } }  // rebase rule, dedupe, decoding: the queue's
        QueueVoice.hooks = hooks

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
