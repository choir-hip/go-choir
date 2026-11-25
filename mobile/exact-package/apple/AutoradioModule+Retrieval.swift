// The voice turn's ask for passages, and the app module's native ops for the embedding model. Native because the turn engine and the model are
// (the microphone, the speech and llama.cpp); the index, the ranking and the vectors are shared code (AutoradioLab/retrieval-run.ts).
//   source retrievalAsk   the ask the turn raised (interface/retrieval.json); shared code reads it and answers through the op below.
//   ops retrievalAnswer (shared code's answer: {id, context}) and embedState, embedEnsure, embed (the on-device model, AutoradioKit/EmbedDevice.swift)
//   are answered in modules/apple/ModelsEntry.swift: they need nothing of this module.
// The ask carries one fact as a JSON string: the run sheet's entries on air and ahead (shared code's run sheet, pushed with `radioSet`: voice-facts.ts), until the radio's
// shared code can be read for it there. The open pages, the edition topics and the page's text are shared code's own (retrieval-live.ts `sharedFacts`): they are not sent back.
import Foundation

extension AutoradioModule {
    func registerRetrieval() {
        handle("retrievalAsk") { _, reply in
            let a = RetrievalBridge.shared.current
            reply(["id": a?.id ?? "", "words": a?.words ?? "", "pageTitle": a?.pageTitle ?? "", "pageAddress": a?.pageAddress ?? "", "now": a?.now ?? 0, "facts": a?.facts ?? ""])
        }
        RetrievalBridge.shared.raise = { [weak self] in self?.touch(["retrievalAsk"], now: true) }
        RetrievalBridge.shared.facts = { _ in
            let entries: [[String: Any]] = RadioBridge.shared.facts()      // shared code's own list of the entries on air and ahead (voice-facts.ts)
            return (try? JSONSerialization.data(withJSONObject: ["runSheet": entries])).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
        }
    }
}
