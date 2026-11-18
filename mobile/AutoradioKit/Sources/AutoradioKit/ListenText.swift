import Foundation

/// What "Listen" speaks for one document: the headline first, as its own paragraph, then the body. It becomes audio through the one
/// existing path (`LabRuntime.speakText` -> `ProgrammePlayer.speakText`), which splits it into paragraphs and speaks them with the
/// player's own controls. The words are cleaned the way every spoken paragraph is (`ChoirText.strip`: links keep their text,
/// headings, quotes, bullets and emphasis lose their marks); images are dropped, and control characters never reach the voice.
/// Text with no body is not a listening: the result is empty and nothing starts.
public enum ListenText {
    public static func compose(title: String, body: String) -> String {
        let head = ChoirText.strip(clean(title))
        let ps = ChoirText.paragraphs(clean(body))
        guard !ps.isEmpty else { return "" }
        var out = ps
        if !head.isEmpty, ps[0].lowercased() != head.lowercased() { out.insert(head, at: 0) }
        return out.joined(separator: "\n\n")
    }

    /// Control characters out (newlines and tabs stay), images out, no more than one blank line between paragraphs.
    static func clean(_ s: String) -> String {
        var t = String(String.UnicodeScalarView(s.unicodeScalars.filter { $0 == "\n" || $0 == "\t" || $0 == "\r" || !CharacterSet.controlCharacters.contains($0) }))
        t = t.replacingOccurrences(of: "!\\[[^\\]]*\\]\\([^)]*\\)", with: "", options: .regularExpression)
        t = t.replacingOccurrences(of: "[ \\t]+\\n", with: "\n", options: .regularExpression)
        t = t.replacingOccurrences(of: "\\n{3,}", with: "\n\n", options: .regularExpression)
        return t
    }
}
