// Lane file: reader. Owned by the reader lane (mobile/README.md, "File ownership"). The reader's sources and sends:
//   readerDoc(address)    the document at a pub: or texture: address, as blocks of runs, citations, sources, versions
//   texturesList(at)      the listener's documents (reloaded each time the Textures page is the page in front)
//   readerFollow          the block and sentence the radio is reading (set by lane 1 through ReaderFollow.shared.set)
//   readerAim             what the bar's prompt is aimed at: a document or one block of it
// View: web-page (WebPageView), the source page in place. Sends: readerOpenExternal, readerAimSet, readerStop (stop the desk's revision), readerRestore (a version as latest), readerReload, readerFollowSet.
import Foundation
import ExactPackage
import UIKit
import WebKit

/// The source page shown in place: a WKWebView that Exact lays out as the `web-page` node (the contract sizes it to the page
/// region above the bar). It loads only http(s) pages (SourceLink.route says so again here), opens a link that asks for a
/// new window in the same view, and tells the contract "loading" and "done" (or "failed") through `message`.
final class WebPageView: NSObject, ExactPackageView, WKNavigationDelegate, WKUIDelegate {
    private let web: WKWebView
    private let events: ExactPackageEvents
    private var loaded = ""

    init(events: ExactPackageEvents) {
        self.events = events
        web = WKWebView(frame: .zero, configuration: WKWebViewConfiguration())
        super.init()
        web.navigationDelegate = self
        web.uiDelegate = self
        web.allowsBackForwardNavigationGestures = true
        web.isOpaque = false
        web.backgroundColor = .clear
    }

    var view: ExactPackagePlatformView { web }

    func setProps(_ props: [String: String]) throws {
        let page = props["page"] ?? ""
        guard page != loaded else { return }
        loaded = page
        guard case .inApp(let url) = SourceLink.route(page) else { events.message("failed"); return }
        web.load(URLRequest(url: url))
    }

    func destroy() {
        web.stopLoading()
        web.navigationDelegate = nil
        web.uiDelegate = nil
    }

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) { events.message("loading") }
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { events.message("done"); events.load() }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { events.message("failed") }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { events.message("failed") }

    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = action.request.url else { return decisionHandler(.cancel) }
        switch SourceLink.route(url.absoluteString) {
        case .inApp: decisionHandler(.allow)
        case .external(let u): DispatchQueue.main.async { UIApplication.shared.open(u) }; decisionHandler(.cancel)   // a mailto: or tel: inside the page
        case .refused: decisionHandler(url.scheme?.lowercased() == "about" || action.targetFrame?.isMainFrame == false ? .allow : .cancel)   // about:blank and the page's own frames
        }
    }

    // target=_blank and window.open: show it in this view rather than nowhere.
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for action: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if action.targetFrame == nil, let url = action.request.url, case .inApp = SourceLink.route(url.absoluteString) { webView.load(URLRequest(url: url)) }
        return nil
    }
}

extension AutoradioModule {
    /// The package's one native view: `web-page`, declared in interface/reader.json and listed in apple/exact.json "views".
    public func view(_ tag: String, id: String, props: [String: String], events: ExactPackageEvents) throws -> ExactPackageView {
        guard tag == "web-page" else { throw ExactPackageRefusal("no view \(tag)") }
        let v = WebPageView(events: events)
        try v.setProps(props)
        return v
    }

    func registerReader() {
        pageSources.register(ReaderPageSource())
        ReaderFollow.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["readerFollow"]) } }
        // Only ever read and written on the main thread: Exact runs every handler there, and the write below hops to it.
        var lastList = TextureList()

        handle("readerDoc") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            guard TextureAddress.parse(address).kind != .none else { return reply(json(ReaderDoc.loading(""))) }
            Task { [weak self] in
                // A document the desk is revising is read again each time it is asked for (and asked for again every few seconds).
                let again = ReaderStore.shared.cached(address)?.revising ?? false
                let d = await ReaderStore.shared.doc(address, force: again)
                DispatchQueue.main.async {
                    reply(self?.json(d) ?? [:])
                    if d.revising { DispatchQueue.main.asyncAfter(deadline: .now() + 5) { self?.touch(["readerDoc"]) } }
                }
            }
        }
        handle("texturesList") { [unowned self] args, reply in
            guard (args["at"] as? String ?? "") == "textures:" else { return reply(json(lastList)) }
            Task { [weak self] in
                let l = await TextureList.load()
                DispatchQueue.main.async { lastList = l; reply(self?.json(l) ?? [:]) }
            }
        }
        handle("readerFollow") { [unowned self] _, reply in reply(json(ReaderFollow.shared.info)) }
        handle("readerAim") { [unowned self] _, reply in reply(json(ReaderAim.shared.info)) }

        handle("readerAimSet") { [unowned self] args, reply in
            let info = ReaderAimInfo(address: args["address"] as? String ?? "", blockId: args["blockId"] as? String ?? "", label: args["label"] as? String ?? "")
            if info.address.isEmpty { ReaderAim.shared.clear() } else {
                ReaderAim.shared.set(info)
                signals.record(SignalEvent(kind: "aim", subject: info.address, detail: info.blockId))
            }
            touch(["readerAim"])
            reply(json(ReaderAim.shared.info))
        }
        handle("readerStop") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { [weak self] in
                await ReaderStore.shared.cancelRevision(document: address)
                DispatchQueue.main.async { self?.touch(["readerDoc", "texturesList"]); reply(["ok": true]) }
            }
        }
        handle("readerRestore") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", version = args["version"] as? String ?? ""
            Task { [weak self] in
                let ok = await ReaderStore.shared.restore(document: address, revisionAddress: version)
                DispatchQueue.main.async { self?.touch(["readerDoc", "texturesList"]); reply(["ok": ok]) }
            }
        }
        handle("readerReload") { [unowned self] args, reply in
            ReaderStore.shared.invalidate(args["address"] as? String ?? "")
            touch(["readerDoc"])
            reply(["ok": true])
        }
        // Leaving the app on purpose: "Open in Safari" for a web page, or a mail, phone or message link. Web pages are otherwise shown
        // in place by the contract (WebLayer); anything that is neither is refused.
        handle("readerOpenExternal") { args, reply in
            switch SourceLink.route(args["url"] as? String ?? "") {
            case .inApp(let u), .external(let u): DispatchQueue.main.async { UIApplication.shared.open(u) }; reply(["ok": true])
            case .refused: reply(["ok": false])
            }
        }
        handle("readerFollowSet") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { [weak self] in
                let known = await ReaderStore.shared.doc(address)       // makes sure the document is cached
                _ = known
                DispatchQueue.main.async {
                    guard let self else { return reply([:]) }
                    reply(self.json(ReaderFollow.shared.set(address: address, spoken: args["spoken"] as? String ?? "")))
                }
            }
        }
    }
}
