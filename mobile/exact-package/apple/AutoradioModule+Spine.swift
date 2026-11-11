// The spine's sources and sends: open pages, the page model, themes, the bar's state, signals and demo mode.
// Owned by the spine. A lane does not edit this file; it registers its own handlers in AutoradioModule+<Lane>.swift.
import Foundation
import UIKit

extension AutoradioModule {
    /// The simulator driven by the Exact agent opens no audio hardware.
    var virtualEngine: Bool {
        #if targetEnvironment(simulator)
        return context.agent
        #else
        return false
        #endif
    }

    func pageList() -> [String: Any] {
        let list = pages.list()
        let top = list.first
        return ["rows": list.map { json($0) }, "top": top?.address ?? "", "topKind": top?.kind ?? "", "topTitle": top?.title ?? "", "count": list.count]
    }

    func registerSpine() {
        // Sources
        handle("openPages") { [unowned self] _, reply in reply(pageList()) }
        handle("pageModel") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            if address.isEmpty { return reply(json(PageModel(address: "", kind: "", title: "", status: "loading"))) }
            Task { [pageSources, weak self] in
                let m = await pageSources.model(for: address)
                DispatchQueue.main.async { reply(self?.json(m) ?? [:]) }
            }
        }
        handle("theme") { [unowned self] _, reply in reply(json(themes.current())) }
        handle("themeList") { [unowned self] _, reply in
            reply(["rows": themes.list().map { json($0) }, "mode": themes.schemeMode, "themeID": themes.themeID,
                   "scheme": themes.effectiveScheme(), "problems": themes.problems])
        }
        handle("barState") { [unowned self] _, reply in reply(json(BarState.info(player: choir.playerView(), snapshot: runtime.snapshot()))) }
        handle("demoInfo") { _, reply in reply(["on": Demo.isOn]) }
        handle("signalList") { [unowned self] _, reply in reply(["rows": signals.recent(limit: 30).map { json($0) }]) }

        // The radio's buttons start the engine on first use; under the simulator's agent that engine must be the virtual one
        // (no microphone: demo and proof runs), as `labStart` already does. Start it virtually, then run the original action.
        for name in ["choirPlayPause", "choirStartRadio", "choirTalkDown", "choirPlay"] {
            handle(name) { [unowned self] args, reply in
                Task { [weak self] in
                    guard let self else { return }
                    if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                    switch name {
                    case "choirPlayPause": await self.choir.playPause()
                    case "choirStartRadio": await self.choir.startRadio()
                    case "choirTalkDown": await self.choir.talkDown()
                    default: await self.choir.play(route: args["route"] as? String ?? "")
                    }
                    DispatchQueue.main.async { reply(self.json(self.runtime.snapshot())) }
                }
            }
        }

        // Sends. Every one answers the open-page list, the shape the page host reads.
        handle("pageOpen") { [unowned self] args, reply in
            pages.open(args["address"] as? String ?? "", title: args["title"] as? String); touch(["openPages"]); reply(pageList())
        }
        handle("pageClose") { [unowned self] args, reply in pages.close(args["address"] as? String ?? ""); touch(["openPages"]); reply(pageList()) }
        handle("pageFocus") { [unowned self] args, reply in pages.focus(args["address"] as? String ?? ""); touch(["openPages"]); reply(pageList()) }
        handle("pagePin") { [unowned self] args, reply in pages.togglePin(args["address"] as? String ?? ""); touch(["openPages"]); reply(pageList()) }
        handle("pageReopen") { [unowned self] _, reply in pages.reopenLastClosed(); touch(["openPages"]); reply(pageList()) }
        handle("themeSet") { [unowned self] args, reply in
            themes.setTheme(args["id"] as? String ?? ""); touch(["theme", "themeList"]); reply(pageList())
        }
        handle("schemeSet") { [unowned self] args, reply in
            themes.setMode(args["mode"] as? String ?? ""); touch(["theme", "themeList"]); reply(pageList())
        }
        handle("signalRecord") { [unowned self] args, reply in
            signals.record(SignalEvent(kind: args["kind"] as? String ?? "", subject: args["subject"] as? String ?? "", detail: args["detail"] as? String ?? ""))
            touch(["signalList"]); reply(pageList())
        }
        handle("demoSet") { [unowned self] args, reply in
            Demo.setOn((args["on"] as? String ?? "") == "true"); touch(["demoInfo"]); reply(pageList())
        }
    }
}
