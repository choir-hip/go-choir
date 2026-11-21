// Lane file: talk (lane autoradio-talk). The conversation (what was asked and what came back), the bar's answer card actions, and the
// queue-by-voice adapter. Sources and sends are declared in interface/talk.json (the bar's fields are bar.ts: AutoradioLab/bar-types.contract).
//   conversationEntries       the voice turn engine's entries, oldest first, whole (the page's rows are AutoradioLab/conversation.ts: shared code)
//   answerAction(action, id)   the card's buttons: "retry" asks the same words again, "dismiss" hides the card, "cancel" stops the turn
import Foundation
import ExactPackage

extension AutoradioModule {
    func registerTalk() {
        Conversation.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["conversationEntries"], now: true) } }

        // The queue by voice (owner, 2026-11-15). The paper's list is what the DJ may add; the rest goes through the run sheet
        // (Kit/TurnQueueWire.swift): `turnQueueJSON()` goes out with every turn as the `queue` field, the `queue_ops` event comes back whole to
        // `applyTurnOps`, which translates the old words onto run sheet ops.
        var hooks = QueueVoice.hooks
        hooks.candidates = { [choir] in (choir?.storyRows() ?? []).prefix(40).map { (route: $0.route, title: $0.title) } }
        hooks.snapshot = { RadioBridge.shared.snapshot }                                                                   // the wire's `queue`, as shared code last made it (voice-queue.ts)
        hooks.applyServerOps = { json in RadioBridge.shared.send("turnOps", ["json": String(data: json, encoding: .utf8) ?? ""]) }   // the `queue_ops` event, whole, to shared code: it applies it (dedupe by turn, the sheet's rules)
        QueueVoice.hooks = hooks

        // The reader's voice (lane autoradio-talk): the Core ML model is fetched quietly (about 8 MB) and, once loaded, the gate listens.
        // Learned from what the reader says to the radio; stored on this phone only (SpeakerStore); never uploaded.
        SpeakerGate.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["voiceProfile"]) } }
        Task { let e = WeSpeakerEmbedder.standard(); if await e.ensureReady() { SpeakerGate.shared.embedder = e } }
        handle("voiceProfile") { [unowned self] _, reply in reply(self.json(SpeakerGate.shared.info())) }
        handle("voiceLearn") { _, reply in SpeakerGate.shared.forget(); reply(["ok": true, "message": ""]) }
        handle("voiceForget") { _, reply in SpeakerGate.shared.forget(); reply(["ok": true, "message": ""]) }

        // Native because the voice turn engine writes the entries; it hands them over as they are (and the id the card was closed on) and decides nothing. The bar's answer card is bar.ts.
        handle("conversationEntries") { [unowned self] _, reply in reply(["entries": Conversation.shared.entries().map { self.json($0) }, "dismissed": Conversation.shared.dismissedID()]) }
        handle("answerAction") { [unowned self] args, reply in
            let action = args["action"] as? String ?? "", id = args["id"] as? String ?? ""
            switch action {
            case "dismiss": Conversation.shared.dismiss(id: id)
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
