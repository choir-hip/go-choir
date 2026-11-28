// Lane file: reader. The document itself (a story or a Texture document: blocks, citations, versions, related, "Hear it", stop, restore, reload, prefetch) is read by shared code now
// (AutoradioLab/reader.ts, reader-parse.ts) and is not here. What stays native, and why:
//   holdPage (the `pageText` op)     the page in front, handed over by shared code (reader.ts `hand`): the voice turn reads its text
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

    /// The shared reader hands over the page in front (AutoradioModule's app-module op `pageText`, ModelsEntry.swift): held for the voice turn's page text and the corpus.
    /// The reason it is native: the voice engine is native and cannot ask the shared module for a page's text.
    /// True only when the page was held (the hand-over names an address and is not from an account that has since signed out).
    @discardableResult
    public func holdPage(_ request: [String: Any]) -> Bool {
        guard let p = HeldPage.parse(request) else { return false }
        guard HeldPages.shared.hold(gen: HeldPage.generation(of: request), p) else { return false }      // read for an account that has since signed out: neither held nor handed to the voice
        let text = p.lines.joined(separator: " ")
        if !text.isEmpty { runtime.pageTextReady(address: p.address, text: text) }   // the text may arrive after the page is in front
        return true
    }

    func registerReader() {
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
