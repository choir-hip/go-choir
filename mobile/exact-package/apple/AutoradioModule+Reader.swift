// Lane file: reader. The document itself (a story or a Texture document: blocks, citations, versions, related, "Hear it", stop, restore, reload, prefetch) is read by shared code now
// (AutoradioLab/reader.ts, reader-parse.ts) and is not here. What stays native, and why:
//   readerFollow                     the block and sentence the radio is reading: set by the bar's voice turn, matched against the page the shared reader handed over (HeldPages)
//   holdPage (the `pageText` op)     the page in front, handed over by shared code (reader.ts `hand`): the voice turn reads its text and the follow-along matches against it
//   readerAim / readerAimSet         what the bar's prompt is aimed at; the voice turn reads it
//   readerOpenExternal               Platform.open (leaving the app: Safari, mail, phone)
// View: web-page (WebPageView), the source page in place: a WKWebView, a platform view only native code can draw.
import Foundation
import ExactPackage
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
        #if os(iOS)
        web.isOpaque = false
        web.backgroundColor = .clear
        #else
        web.setValue(false, forKey: "drawsBackground")
        #endif
    }

    var view: ExactPackagePlatformView { web }

    func setProps(_ props: [String: String]) throws {
        let page = props["page"] ?? ""
        guard page != loaded else { return }
        loaded = page
        guard case .inApp(let url) = SourceLink.route(page) else { events.message("failed"); return }
        EmbeddedPage.shared.opened(page)        // lane autoradio-queue: "Read aloud" reads the page the view is on
        web.load(URLRequest(url: url))
    }

    func destroy() {
        web.stopLoading()
        web.navigationDelegate = nil
        web.uiDelegate = nil
    }

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) { events.message("loading") }
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { EmbeddedPage.shared.loaded(url: webView.url, title: webView.title); events.message("done"); events.load() }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { events.message("failed") }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { events.message("failed") }

    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = action.request.url else { return decisionHandler(.cancel) }
        switch SourceLink.route(url.absoluteString) {
        case .inApp: decisionHandler(.allow)
        case .external(let u): Platform.open(u); decisionHandler(.cancel)   // a mailto: or tel: inside the page
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

    /// The shared reader hands over the page in front (AutoradioModule's app-module op `pageText`, ModelsEntry.swift): held for the voice turn's page text, the corpus and the follow-along.
    /// The reason it is native: the voice engine is native and cannot ask the shared module for a page's text.
    /// True only when the page was held (the hand-over names an address).
    @discardableResult
    public func holdPage(_ request: [String: Any]) -> Bool {
        guard let p = HeldPage.parse(request) else { return false }
        HeldPages.shared.hold(p)
        let text = p.lines.joined(separator: " ")
        if !text.isEmpty { runtime.pageTextReady(address: p.address, text: text) }   // the text may arrive after the page is in front
        DispatchQueue.main.async { [weak self] in self?.touch(["pageModel"]) }
        return true
    }

    func registerReader() {
        pageSources.register(HeldPageSource())
        ReaderFollow.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["readerFollow"]) } }

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
        // Leaving the app on purpose: "Open in Safari" for a web page, or a mail, phone or message link. Web pages are otherwise shown
        // in place by the contract (WebLayer); anything that is neither is refused.
        handle("readerOpenExternal") { args, reply in
            switch SourceLink.route(args["url"] as? String ?? "") {
            case .inApp(let u), .external(let u): Platform.open(u); reply(["ok": true])
            case .refused: reply(["ok": false])
            }
        }
    }
}
