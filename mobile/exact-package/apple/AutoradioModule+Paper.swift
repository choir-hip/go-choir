// Lane file: paper. Owned by the paper lane (mobile/README.md, "File ownership").
// The paper page is shared logic now: `paperFront` is answered by app.ts (AutoradioLab/paper.ts), not by this package. What stays here is the one thing
// only Swift knows until the edition setup moves: the reader's saved topics (`edition-topics.json`, written by the topic screens). `editionPicks`
// answers them, one per line, every time it is asked; the contract passes `focus` (paper, article or away) so it is asked again when the reader
// comes back from editing. Names are declared in interface/paper.json.
import Foundation

extension AutoradioModule {
    func registerPaper() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let topicsFile = support.appendingPathComponent("edition-topics.json")
        handle("editionPicks") { _, reply in
            let picks = (EditionSetupSession.load(topicsFile)?.topics ?? []).joined(separator: "\n")
            DispatchQueue.main.async { reply(["picks": picks]) }
        }
    }
}
