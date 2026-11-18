// Every Markdown marker hidden while editing (choir patch 0001; mobile/exact-patches/README.md).
// The web editor hides syntax away from the caret; the Apple host dimmed it. This file holds the pure rules of hiding all of it, with no
// dependency on ExactKit so that they are unit-tested on their own (AutoradioKit's MarkupSyntaxHidingTests compiles this very file).
// @ref LLP 1045 D5: which runs are hidden is the styler's (`Styled.hidden`, asked with no selection to reveal); the source, its offsets,
// native undo and copy stay the Markdown's.
import Foundation

enum MarkupSyntaxHiding {
    /// `.markupHidden`'s value for a marker of syntax (a link's destination keeps its link index, below this).
    static let syntaxBase = 1_000_000
    /// `.markupHidden`'s value for a list bullet: hidden source drawn as a bullet glyph.
    static let bullet = -1
    static let joiner = "\u{2060}"

    /// What is drawn in place of a hidden run of `length` UTF-16 units: word joiners (no advance, no glyph), or for a bullet the glyph then
    /// joiners, so the run keeps its length and every offset stays a source offset.
    static func drawn(length: Int, bullet isBullet: Bool) -> String {
        guard length > 0 else { return "" }
        return isBullet ? "\u{2022}" + String(repeating: joiner, count: length - 1) : String(repeating: joiner, count: length)
    }

    /// The hidden runs that open a block: a heading's `#`s, a quote's `>`s and a list item's bullet, in the order they are in. A marker inside a
    /// line (`**`, `*`, a backtick, `~~`) is not one. `hidden` and `bullets` are sorted.
    static func blockPrefixes(of source: NSString, hidden: [NSRange], bullets: [NSRange]) -> [NSRange] {
        let candidates = (hidden.filter { r in
            guard r.length > 0, NSMaxRange(r) <= source.length else { return false }
            let text = source.substring(with: r)
            return text.unicodeScalars.allSatisfy { "#> \t".unicodeScalars.contains($0) } && text.contains(where: { $0 == "#" || $0 == ">" })
        } + bullets.filter { $0.length > 0 && NSMaxRange($0) <= source.length }).sorted { $0.location < $1.location }
        var accepted: [NSRange] = []
        for r in candidates {
            let lineStart = source.lineRange(for: NSRange(location: r.location, length: 0)).location
            var ok = true
            var u = lineStart
            while u < r.location {
                let c = source.character(at: u)
                let blank = c == 0x20 || c == 0x09
                if !blank && !accepted.contains(where: { NSLocationInRange(u, $0) }) { ok = false; break }
                u += 1
            }
            if ok { accepted.append(r) }
        }
        return accepted
    }

    /// Backspace with the caret just after a block's hidden prefix removes that prefix (innermost first), so the block loses its formatting
    /// and the text stays; nil when the caret is not at the end of one.
    static func backspaceRemoves(caret: Int, prefixes: [NSRange]) -> NSRange? {
        prefixes.last { NSMaxRange($0) == caret }
    }
}
