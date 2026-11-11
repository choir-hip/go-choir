// Lane file: paper. Owned by the paper lane (mobile/README.md, "File ownership").
// `paperFront` answers the composed front page: the stories (fixtures in demo mode, the public front list otherwise),
// laid out by the Kit's layout engine for the width the page reports. Names are declared in interface/paper.json.
import Foundation

extension AutoradioModule {
    func registerPaper() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let source = PaperSource(cacheFile: support.appendingPathComponent("paper-front.json"))
        let engine = LayoutEngine()
        handle("paperFront") { [weak self] args, reply in
            let section = args["section"] as? String ?? ""
            let edition = args["edition"] as? String ?? ""
            let w = (args["width"] as? Double) ?? Double(args["width"] as? Int ?? 0)
            let width = w >= 200 ? w : 393
            Task {
                let r: PaperSource.Result
                if Demo.isOn { r = source.demo(edition: edition) } else { r = await source.live(client: ChoirEnvironment.shared.client()) }
                // A demo edition is dated by its own newest story; a live one by the clock.
                let front = PaperComposer.front(stories: r.stories, section: section, width: width, now: Demo.isOn ? nil : Date(),
                                                edition: edition, engine: engine, status: r.status, message: r.message)
                DispatchQueue.main.async { reply(self?.json(front) ?? [:]) }
            }
        }
    }
}
