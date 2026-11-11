import Foundation

// Demo mode: the app signed in as a demo listener, served from bundled fixtures, with no network and no microphone
// (the engine is already virtual under the Exact agent). Every lane builds and proves its screens on the simulator
// against this. Fixtures live in mobile/fixtures/demo/*.json and are embedded by scripts/embed-assets.py.
//
//   Turn it on (simulator):  xcrun simctl spawn <udid> defaults write news.choir.autoradio.lab AutoradioDemo -bool YES
//   Turn it off:             ... defaults delete news.choir.autoradio.lab AutoradioDemo
//   or at run time:          the `demoSet` send (Components page, "Demo mode"), then relaunch.
//
// What is stubbed: GET /api/platform/publications/front, /resolve and /api/compute/status (from fixtures); every other
// route answers 404 {"error":"demo mode serves fixtures only"} so a lane that needs a route adds a fixture here
// (add the file under mobile/fixtures/demo/ and one `case` in DemoURLProtocol.route, nowhere else).

public enum Demo {
    public static let defaultsKey = "AutoradioDemo"

    public static var isOn: Bool {
        UserDefaults.standard.bool(forKey: defaultsKey) || ProcessInfo.processInfo.environment["AUTORADIO_DEMO"] == "1"
    }
    public static func setOn(_ on: Bool) {
        if on { UserDefaults.standard.set(true, forKey: defaultsKey) } else { UserDefaults.standard.removeObject(forKey: defaultsKey) }
    }

    /// A key store already signed in as the demo listener (the key is a placeholder; nothing leaves the phone).
    public static func keys() -> KeyStore {
        InMemoryKeyStore([KeyAccount.apiKey: "choir_sk_demo", KeyAccount.computerID: "demo-computer", KeyAccount.keyID: "demo-key"])
    }

    /// A URLSession whose every request is answered from the fixtures.
    public static func session() -> URLSession {
        let c = URLSessionConfiguration.ephemeral
        c.protocolClasses = [DemoURLProtocol.self]
        return URLSession(configuration: c)
    }

    /// Wire the shared Choir environment to the fixtures. Call before ChoirApp is made.
    public static func install(into env: ChoirEnvironment = .shared) {
        env.session = session()
        env.keys = keys()
    }

    public static func fixture(_ name: String) -> Data? { DemoFixtureData.files[name].map { Data($0.utf8) } }

    /// The demo front page as stories, for pages that list them without a request.
    public static func stories() -> [ChoirStory] {
        guard let d = fixture("front"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any],
              let list = o["stories"] as? [[String: Any]] else { return [] }
        return list.compactMap { s in
            guard let title = s["title"] as? String, let route = (s["route"] as? [String: Any])?["bare"] as? String else { return nil }
            return ChoirStory(title: title, dek: s["dek"] as? String ?? "", route: route, section: s["section"] as? String ?? "",
                              words: s["words"] as? Int ?? 0, updated: s["updated_at"] as? String ?? "")
        }
    }
}

final class DemoURLProtocol: URLProtocol {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func stopLoading() {}

    override func startLoading() {
        let url = request.url ?? URL(string: "https://demo.invalid/")!
        let items = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems ?? []
        var sent = request.httpBody ?? Data()
        if let st = request.httpBodyStream { st.open(); var buf = [UInt8](repeating: 0, count: 4096)
            while st.hasBytesAvailable { let n = st.read(&buf, maxLength: buf.count); if n <= 0 { break }; sent.append(buf, count: n) }; st.close() }
        // Mail (lane 6) needs the method and the body, so it answers first; every other route is the one switch below.
        let (status, body) = MailDemo.answer(method: request.httpMethod ?? "GET", path: url.path, query: items,
                                             body: (try? JSONSerialization.jsonObject(with: sent)) as? [String: Any] ?? [:]) ?? Self.route(path: url.path, query: items)
        let resp = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
    }

    /// One case per stubbed route. A lane adds its fixture file and a case here.
    static func route(path: String, query: [URLQueryItem]) -> (Int, Data) {
        func q(_ n: String) -> String { query.first { $0.name == n }?.value ?? "" }
        switch path {
        case "/api/platform/publications/front": if let d = Demo.fixture("front") { return (200, d) }
        case "/api/compute/status": if let d = Demo.fixture("status") { return (200, d) }
        case "/api/platform/publications/resolve": if let d = Demo.fixture("resolve-" + TextureDemo.bareRoute(q("route"))) { return (200, d) }
        case "/api/platform/publications/related": if let d = Demo.fixture("related-" + TextureDemo.bareRoute(q("route"))) { return (200, d) }
        default: if let d = TextureDemo.route(path) { return (200, d) }
        }
        return (404, Data(#"{"error":"demo mode serves fixtures only"}"#.utf8))
    }
}

/// `pub:<route>` as plain paragraphs, from the network, or the fixtures in demo mode (the session is the stub).
/// The spine's text renderer shows it; lane 3 replaces it with the real reader and its own source.
public struct PublicationTextSource: PageSource {
    public init() {}
    public func handles(_ address: String) -> Bool { address.hasPrefix("pub:") }
    public func load(_ address: String) async throws -> PageModel {
        let route = PageAddress.rest(of: address)
        let r = try await ChoirEnvironment.shared.client().send("GET", "/api/platform/publications/resolve", query: ["route": route],
                                                                  authenticated: false, route: "/api/platform/publications/resolve")
        let o = try r.object()
        let title = ((o["publication"] as? [String: Any])?["title"] as? String) ?? "Story"
        let text = ((o["payload"] as? [String: Any])?["text"] as? String) ?? ""
        return PageModel(address: address, kind: "reader", title: title, status: "ready", lines: ChoirText.paragraphs(text))
    }
}
