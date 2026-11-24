// The voice turn's ask for passages, and the app module's native ops for the embedding model. Native because the turn engine and the model are
// (the microphone, the speech and llama.cpp); the index, the ranking and the vectors are shared code (AutoradioLab/retrieval-run.ts).
//   source retrievalAsk   the ask the turn raised (interface/retrieval.json); shared code reads it and answers through the op below.
//   ops retrievalAnswer (shared code's answer: {id, context}) and embedState, embedEnsure, embed (the on-device model, AutoradioKit/EmbedDevice.swift)
//   are answered in modules/apple/ModelsEntry.swift: they need nothing of this module.
// The facts only native holds go with the ask as one JSON string: the run sheet's entries on air and ahead (shared code's run sheet, pushed with `radioSet`: voice-facts.ts),
// the open pages, the edition topics, and the text of the page in front.
import Foundation

extension AutoradioModule {
    func registerRetrieval() {
        handle("retrievalAsk") { _, reply in
            let a = RetrievalBridge.shared.current
            reply(["id": a?.id ?? "", "words": a?.words ?? "", "pageTitle": a?.pageTitle ?? "", "pageAddress": a?.pageAddress ?? "", "now": a?.now ?? 0, "facts": a?.facts ?? ""])
        }
        RetrievalBridge.shared.raise = { [weak self] in self?.touch(["retrievalAsk"], now: true) }
        RetrievalBridge.shared.facts = { [weak self] address in
            guard let self else { return "{}" }
            let entries: [[String: Any]] = RadioBridge.shared.facts()      // shared code's own list of the entries on air and ahead (voice-facts.ts)
            let pages = OpenPageFacts.shared.list().map { ["address": $0.address, "title": $0.title, "kind": $0.kind] }
            let topics = EditionPicksMirror.shared.topics      // the picks are shared code's (edition-picks.ts); this is its pushed copy (EditionSetup.swift)
            let text = address.isEmpty ? "" : await self.runtime.pageTextProvider?(address) ?? ""
            let o: [String: Any] = ["runSheet": entries, "pages": pages, "topics": topics, "pageText": String(text.prefix(20_000))]
            return (try? JSONSerialization.data(withJSONObject: o)).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
        }
    }
}
