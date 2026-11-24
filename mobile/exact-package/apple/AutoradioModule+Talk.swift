// Lane file: talk (lane autoradio-talk). The conversation (what was asked and what came back), the bar's answer card actions, and the
// queue-by-voice adapter. Sources and sends are declared in interface/talk.json (the bar's fields are bar.ts: AutoradioLab/bar-types.contract).
//   conversationEntries       the voice turn engine's entries, oldest first, whole (the page's rows are AutoradioLab/conversation.ts: shared code)
//   answerAction(action, id)   the card's buttons: "retry" asks the same words again, "cancel" stops the turn
//                              (Close, "dismiss", is shared code: AutoradioLab/bar.ts dismissAnswer; native keeps no dismissal)
import Foundation
import ExactPackage

extension AutoradioModule {
    func registerTalk() {
        Conversation.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["conversationEntries"], now: true) } }

        // The queue by voice (owner, 2026-11-15). The paper's list is what the DJ may add; the rest goes through the run sheet
        // (Kit/TurnQueueWire.swift): `turnQueueJSON()` goes out with every turn as the `queue` field, the `queue_ops` event comes back whole to
        // `applyTurnOps`, which translates the old words onto run sheet ops.
        var hooks = QueueVoice.hooks
        hooks.candidates = { Array(RadioBridge.shared.candidates().prefix(40)) }                                                 // the paper's list, as shared code last said (radio-bridge.ts: the front list it holds)
        hooks.snapshot = { RadioBridge.shared.snapshot }                                                                   // the wire's `queue`, as shared code last made it (voice-queue.ts)
        hooks.applyServerOps = { json in RadioBridge.shared.send("turnOps", ["json": String(data: json, encoding: .utf8) ?? ""]) }   // the `queue_ops` event, whole, to shared code: it applies it (dedupe by turn, the sheet's rules)
        QueueVoice.hooks = hooks

        // The reader's voice (lane autoradio-talk): the Core ML model is fetched quietly (about 8 MB) and, once loaded, the gate listens.
        // Learned from what the reader says to the radio; stored on this phone only (SpeakerStore); never uploaded.
        SpeakerGate.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["voiceProfile"]) } }
        // The model is fetched only after the person agreed on the download step (OnboardingController.fetchSpeakerModel); a phone that already has it loads it here, which fetches nothing.
        Task { let e = WeSpeakerEmbedder.standard(); if e.onDisk, await e.ensureReady() { SpeakerGate.shared.embedder = e } }
        handle("voiceProfile") { [unowned self] _, reply in reply(self.json(SpeakerGate.shared.info())) }
        handle("voiceLearn") { _, reply in OnboardingController.shared?.discardEnrolment(); reply(["ok": true, "message": ""]) }       // "Learn again" opens the page (shared code starts it over); the stored voice stays until a new one is confirmed
        handle("voiceForget") { _, reply in OnboardingController.shared?.discardEnrolment(); SpeakerGate.shared.forget(); reply(["ok": true, "message": ""]) }

        // Native because the voice turn engine writes the entries; it hands them over as they are (and the id the card was closed on) and decides nothing. The bar's answer card is bar.ts.
        handle("conversationEntries") { [unowned self] _, reply in reply(["entries": Conversation.shared.entries().map { self.json($0) }]) }
        handle("answerAction") { [unowned self] args, reply in
            let action = args["action"] as? String ?? "", id = args["id"] as? String ?? ""
            switch action {
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
