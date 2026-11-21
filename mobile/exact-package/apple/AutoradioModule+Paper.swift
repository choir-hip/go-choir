// Lane file: paper. Owned by the paper lane (mobile/README.md, "File ownership").
// The paper page is shared logic: `paperFront` and `editionPicks` are answered by app.ts (AutoradioLab/paper.ts, edition-picks.ts), not by this package. What stays here is the native
// retrieval corpus's source for the listener's topics: the picks are shared code's (the file `app:/data/edition-topics.json`), and this reads their pushed copy in memory
// (`EditionPicksMirror`, EditionSetup.swift), because the on-device retrieval (voice) is native and cannot read shared code's file.
import Foundation

extension AutoradioModule {
    func registerPaper() {
        SavedCorpus.shared.setSource("edition-topics") { RetrievalSources.editionTopics(EditionPicksMirror.shared.topics) }
    }
}
