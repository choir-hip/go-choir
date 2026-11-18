import Foundation

// The tetramark tray's glyph rows (launcher.contract, KeyGlyph). The glyphs are drawn in rows of equal count so no row ends with one glyph alone:
// nine make three rows of three, eight two rows of four. The contract is given the column count (`cols`); this is the rule it was given.
public enum TrayGrid {
    /// Glyphs per row for `count` glyphs: three when three divides the count, else four, else two, each leaving at least two rows; a count none of
    /// them divides (5, 7, 11) takes four a row.
    public static func columns(count: Int) -> Int {
        for c in [3, 4, 2] where count % c == 0 && count / c >= 2 { return c }
        return 4
    }
    public static func rows(count: Int) -> Int { let c = columns(count: count); return (count + c - 1) / c }
    /// How many places are empty in the last row (0 when the rows are even).
    public static func emptyInLastRow(count: Int) -> Int { rows(count: count) * columns(count: count) - count }
}
