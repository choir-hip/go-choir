import Foundation

// Pieces the radio shares that used to live beside the old queue: a URL typed into the bar, a book imported from the bar, a web page read
// as paragraphs, and running on the main actor from any thread. Moved unchanged out of PlayQueue.swift.

// MARK: A URL in the prompt bar

public enum BarURL {
    /// A book or a PDF typed into the bar goes to the Reader's import, not to a web page. The reader lane sets this to `ReaderShelf.importFrom(url:)`;
    /// until it is set the address opens as a page.
    nonisolated(unsafe) public static var importDocument: (@Sendable (URL) -> Void)?
    public static func isDocument(_ url: URL) -> Bool { ["pdf", "epub"].contains(url.pathExtension.lowercased()) }
    /// The page a submitted bar text means, or nil when it is an ordinary sentence. http and https; or one bare word with a dot, a
    /// known-looking host ("choir.news", "example.com/path"). A question ("what is choir.news?") has spaces, so it is not a URL.
    public static func detect(_ text: String) -> URL? {
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty, t.rangeOfCharacter(from: .whitespacesAndNewlines) == nil, !t.contains("@") || t.lowercased().hasPrefix("http") else { return nil }
        let lower = t.lowercased()
        if lower.hasPrefix("http://") || lower.hasPrefix("https://") {
            guard let u = URL(string: t), let h = u.host, !h.isEmpty else { return nil }
            return u
        }
        if t.contains("://") || t.hasSuffix("?") || t.hasSuffix("!") || t.hasSuffix(",") { return nil }
        let host = t.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? t
        let hostOnly = host.split(separator: ":").first.map(String.init) ?? host
        let labels = hostOnly.split(separator: ".", omittingEmptySubsequences: false)
        guard labels.count >= 2, labels.allSatisfy({ !$0.isEmpty }) else { return nil }
        guard let tld = labels.last, tld.count >= 2, tld.allSatisfy({ $0.isLetter && $0.isLowercase }) else { return nil }
        guard labels.allSatisfy({ $0.allSatisfy { $0.isLetter || $0.isNumber || $0 == "-" } }) else { return nil }
        return URL(string: "https://" + t)
    }
}

/// A .pdf or .epub typed into the bar: import it into the Reader, then open its page; if it fails say why in the bar's own line.
public enum BarDocumentImport {
    public static func name(of url: URL) -> String { url.lastPathComponent.removingPercentEncoding ?? url.lastPathComponent }
    public static func failureLine(_ error: Error, url: URL) -> String {
        let why = (error as? ShelfError)?.description ?? ShelfError.unreadable("").description
        return "\(name(of: url)) was not added. \(why)"
    }
    /// `importer` returns the new document's id. Nothing opens on a failure.
    public static func run(url: URL, importer: (URL) async throws -> String, open: @MainActor (String) -> Void, say: (String) -> Void) async {
        do { let id = try await importer(url); await open(ShelfAddress.address(id)) }
        catch { say(failureLine(error, url: url)) }
    }
}

// MARK: Reading a page

public enum PageText {
    /// A web page as paragraphs to read: its main content (an article, a main, else the body), with the page title first.
    public static func paragraphs(html: String, fallbackTitle: String) -> (title: String, paragraphs: [String]) {
        let title = firstMatch(html, "<title[^>]*>(.*?)</title>").map { MailText.decodeEntities($0).trimmingCharacters(in: .whitespacesAndNewlines) } ?? ""
        var body = html
        for tag in ["article", "main"] {
            if let inner = firstMatch(body, "<\(tag)\\b[^>]*>(.*)</\(tag)\\s*>"), inner.count > 400 { body = inner; break }
        }
        for junk in ["nav", "header", "footer", "aside", "form"] {
            body = body.replacingOccurrences(of: "<\(junk)\\b.*?</\(junk)\\s*>", with: "", options: [.regularExpression, .caseInsensitive])
        }
        let text = MailText.htmlToText(body)
        let t = (title.isEmpty ? fallbackTitle : title)
        let composed = ListenText.compose(title: t, body: text)
        return (t, ChoirText.paragraphs(composed, maxChars: ChoirText.speechChars))
    }

    static func firstMatch(_ s: String, _ pattern: String) -> String? {
        guard let re = try? NSRegularExpression(pattern: pattern, options: [.caseInsensitive, .dotMatchesLineSeparators]),
              let m = re.firstMatch(in: s, range: NSRange(s.startIndex..., in: s)), m.numberOfRanges > 1, let r = Range(m.range(at: 1), in: s) else { return nil }
        return String(s[r])
    }

    /// Fetch and extract. nil when the page cannot be fetched or has nothing to read.
    public static func fetch(_ url: URL, title: String) async -> (title: String, paragraphs: [String])? {
        var req = URLRequest(url: url, timeoutInterval: 20)
        req.setValue("text/html,application/xhtml+xml", forHTTPHeaderField: "Accept")
        guard let (data, resp) = try? await URLSession.shared.data(for: req),
              (resp as? HTTPURLResponse).map({ (200..<300).contains($0.statusCode) }) ?? true else { return nil }
        let html = String(decoding: data.prefix(3_000_000), as: UTF8.self)
        let r = paragraphs(html: html, fallbackTitle: title)
        return r.paragraphs.count > (r.title.isEmpty ? 0 : 1) ? r : nil
    }
}

/// Run on the main actor from any thread (the lock screen's commands arrive on whichever thread the system chooses).
func onMainThread<T>(_ f: @MainActor () -> T) -> T {
    if Thread.isMainThread { return MainActor.assumeIsolated(f) }
    return DispatchQueue.main.sync { MainActor.assumeIsolated(f) }
}

/// How a clip block of a programme document ended (`ProgrammePlayer.clipHandler`). Run sheet clips are entries, not blocks, so nothing sets that handler any more.
public enum ClipOutcome: Sendable, Equatable { case played, unknown, failed }
