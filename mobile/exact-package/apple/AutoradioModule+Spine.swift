// The spine's sources and sends: open pages, the page model, themes and the bar's state.
// Owned by the spine. A lane does not edit this file; it registers its own handlers in AutoradioModule+<Lane>.swift.
import Foundation

extension AutoradioModule {
    /// The simulator driven by the Exact agent opens no audio hardware.
    var virtualEngine: Bool {
        #if targetEnvironment(simulator)
        return context.agent
        #else
        return false
        #endif
    }

    /// A native module asks for a page to be opened (an episode from the playing bar). Native holds no set: the request is a source (`pageRequest`) that shared code
    /// answers by opening the page (pages.ts `pageRequested`), once per id.
    public func requestPage(_ address: String, title: String) {
        pageRequestState = (pageRequestState.id + 1, address, title)
        touch(["pageRequest"], now: true)
    }

    @MainActor public func holdPages(_ r: [String: Any]) async -> Bool {
        let rows = (r["pages"] as? [[String: Any]] ?? []).compactMap { o -> FrontPage? in
            guard let a = o["address"] as? String, !a.isEmpty else { return nil }
            return FrontPage(address: a, title: o["title"] as? String ?? "", kind: o["kind"] as? String ?? "")
        }
        let before = OpenPageFacts.shared.list().first?.address
        guard OpenPageFacts.shared.replace(gen: (r["gen"] as? NSNumber)?.intValue ?? 0, rows) else { return false }
        if before != rows.first?.address {
            runtime.warmResponder(address: rows.first?.address)
        }
        return true
    }

    func registerSpine() {
        // Sources
        handle("pageRequest") { [unowned self] _, reply in reply(["id": pageRequestState.id, "address": pageRequestState.address, "title": pageRequestState.title]) }

        // The radio's buttons start the engine on first use; under the simulator's agent that engine must be the virtual one
        // (no microphone: proof runs), as `labStart` already does. Start it virtually, then run the original action.
        // The acknowledgement names the page in front: title and kind of the page at an address (nil: the front one).
        runtime.frontPageResolver = { address in OpenPageFacts.shared.front(address) }
        // The text of that page, as far as the phone has it (the page the shared reader handed over): the local stream may say something relevant from it.
        runtime.pageTextProvider = { address in address.isEmpty ? "" : (HeldPages.shared.page(address)?.lines.joined(separator: " ") ?? "") }
        runtime.startRequest = { [weak self] in
            guard let self else { return }
            await self.runtime.start(virtual: self.virtualEngine)
        }
        // The bar's microphone: hands-free on or off, one state, off at every launch. Turning it on starts the engine if it is not running;
        // turning it off drops an open turn at once and the engine hears nothing more.
        handle("choirHandsFreeToggle") { [unowned self] _, reply in
            Task { [weak self] in
                guard let self else { return }
                let on = !HandsFreeSwitch.shared.isOn
                if on, !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                self.runtime.setHandsFree(on)
                DispatchQueue.main.async { self.touch(["barFacts"], now: true); reply(self.json(self.runtime.snapshot())) }
            }
        }

        // The bar's typed question: one turn, like a spoken one (the engine starts virtually under the agent, as for the buttons above).
        // `aim` carries the address of the page in front when the bar sent (absent before lane SHELL's interface lands: the page in front now).
        for name in ["choirAsk"] {
            handle(name) { [unowned self] args, reply in
                let text = args["text"] as? String ?? ""
                // A URL in the bar is not a question: it opens in place with Read aloud and Add to queue (lane autoradio-queue; the one early branch).
                if let url = BarURL.detect(text) {
                    self.barWebURL = url.absoluteString
                    return reply(self.json(self.runtime.snapshot()))
                }
                // `aim` (lane SHELL) is the front page's address; `address` is the older name. Either may be absent.
                let address = (args["aim"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? args["address"] as? String ?? ""
                Task { [weak self] in
                    guard let self else { return }
                    _ = await self.runtime.ask(text, pageAddress: address)
                    DispatchQueue.main.async { reply(self.json(self.runtime.snapshot())) }
                }
            }
        }
    }
}
