import Foundation
#if canImport(CoreText)
import CoreText
#endif

/// How many lines a text takes at a size and width. The engine asks; the host answers with Core Text, tests with a
/// deterministic estimate.
public protocol TextMeasurer: Sendable {
    func lines(_ text: String, size: Double, weight: Double, width: Double) -> Int
}

/// Greedy word wrap with an average glyph width: deterministic, platform independent (tests and fallback).
public struct ApproxMeasurer: TextMeasurer {
    public var em: Double
    public init(em: Double = 0.5) { self.em = em }
    public func lines(_ text: String, size: Double, weight: Double, width: Double) -> Int {
        let w = max(width, 1)
        let glyph = size * em * (weight >= 600 ? 1.07 : 1.0)
        var lines = 1, x = 0.0
        for word in text.split(separator: " ", omittingEmptySubsequences: true) {
            let len = Double(word.count) * glyph
            let space = x == 0 ? 0 : glyph
            if x + space + len <= w { x += space + len; continue }
            // A word longer than the line breaks across lines.
            let own = max(1, Int((len / w).rounded(.up)))
            lines += own - (x == 0 ? 1 : 0)
            x = own == 1 ? len : len - Double(own - 1) * w
        }
        return text.isEmpty ? 0 : lines
    }
}

#if canImport(CoreText)
/// Core Text framesetter with a serif face close to the theme's reading face. The theme's face is a CSS stack the
/// phone resolves itself, so this is an estimate with a safety margin (3 percent of the width), not an oracle.
public struct CoreTextMeasurer: TextMeasurer {
    public init() {}
    public func lines(_ text: String, size: Double, weight: Double, width: Double) -> Int {
        guard !text.isEmpty else { return 0 }
        let name = weight >= 600 ? "Georgia-Bold" : "Georgia"
        let font = CTFontCreateWithName(name as CFString, CGFloat(size), nil)
        let attr = NSAttributedString(string: text, attributes: [NSAttributedString.Key(kCTFontAttributeName as String): font])
        let setter = CTFramesetterCreateWithAttributedString(attr)
        let w = max(width * 0.97, 1)
        let path = CGPath(rect: CGRect(x: 0, y: 0, width: w, height: 10_000), transform: nil)
        let frame = CTFramesetterCreateFrame(setter, CFRange(location: 0, length: 0), path, nil)
        return max(1, CFArrayGetCount(CTFrameGetLines(frame)))
    }
}
#endif

public enum DefaultMeasurer {
    public static func make() -> TextMeasurer {
        #if canImport(CoreText)
        return CoreTextMeasurer()
        #else
        return ApproxMeasurer()
        #endif
    }
}
