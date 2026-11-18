import Foundation

/// Where a tapped link goes. A web page (http or https) is shown inside the app, in a browser sheet that closes back to the
/// card the reader was on. Mail, phone and message links hand off to the system app that owns them, so they still leave the app.
/// Anything else (javascript:, file:, data:, a malformed string) is refused rather than opened.
public enum SourceLink: Equatable, Sendable {
    case inApp(URL)
    case external(URL)
    case refused

    private static let handOff: Set<String> = ["mailto", "tel", "sms", "facetime"]

    public static func route(_ text: String) -> SourceLink {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty, let url = URL(string: trimmed), let scheme = url.scheme?.lowercased() else { return .refused }
        if scheme == "http" || scheme == "https" {
            guard let host = url.host, !host.isEmpty else { return .refused }
            return .inApp(url)
        }
        if handOff.contains(scheme) { return .external(url) }
        return .refused
    }
}
