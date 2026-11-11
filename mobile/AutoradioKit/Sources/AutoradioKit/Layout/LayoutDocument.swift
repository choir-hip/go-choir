import Foundation

// The layout document (plan section 3): what goes where and at what size. Separate from the theme (faces, colours,
// depth). Source of truth: mobile/layouts/default.json; DefaultLayout.swift carries the same text so the app needs no
// resource bundle (LayoutTests checks the two are equal). A layout may later arrive from the desk as a document.
//
// Ways this can fail (written first):
//  1. Malformed JSON or a wrong schema: `LayoutDocument.load` returns the built-in default, never throws to the page.
//  2. A rule names a module the document does not define, or a condition the evaluator does not know: that rule is
//     skipped (a condition it cannot parse is false), the page still composes.
//  3. A step named in a slot is missing from `steps`: the slot falls back to the first step that exists.
//  4. Missing optional keys (dekLines, meta, degrade): defaults below.

public struct LayoutDocument: Codable, Sendable, Equatable {
    public struct Grid: Codable, Sendable, Equatable { public var units: Int; public var gutter: Double; public var margin: Double }
    public struct Step: Codable, Sendable, Equatable { public var size: Double; public var weight: Double; public var lineHeight: Double }
    public struct Slot: Codable, Sendable, Equatable {
        public var span: Int
        public var headline: [String]
        public var maxLines: Int
        public var dek: Bool?
        public var dekLines: Int?
        public var meta: String?
    }
    public struct Follow: Codable, Sendable, Equatable { public var max: Int; public var headline: [String]; public var label: String }
    public struct ListSpec: Codable, Sendable, Equatable { public var max: Int; public var headline: [String]; public var label: String? }
    public struct Module: Codable, Sendable, Equatable {
        public var slots: [Slot]?
        public var rule: String?
        public var degrade: String?
        public var stackHeadline: [String]?
        public var stackMaxLines: Int?
        public var follow: Follow?
        public var list: ListSpec?
        public var bySection: Bool?
        public var perSection: Int?
        public var maxSections: Int?
        public var headline: [String]?
    }
    public struct Rule: Codable, Sendable, Equatable {
        public var group: String?
        public var cond: String?
        public var use: String?
        public var `else`: String?
        public var then: String?
        enum CodingKeys: String, CodingKey { case group, cond = "if", use, `else`, then }
        /// The module this rule places, whichever key named it.
        public var module: String? { use ?? `else` ?? then }
        /// `else` and `then` are unconditional.
        public var unconditional: Bool { cond == nil }
    }

    public var schema: String
    public var name: String
    public var grid: Grid
    public var steps: [String: Step]
    public var freshHours: Double?
    public var modules: [String: Module]
    public var rules: [Rule]

    public static func load(json: String) -> LayoutDocument {
        guard let d = json.data(using: .utf8), let doc = try? JSONDecoder().decode(LayoutDocument.self, from: d),
              doc.schema == "choir.layout.v1", doc.grid.units > 0 else { return builtIn }
        return doc
    }

    public static var builtIn: LayoutDocument {
        // The embedded text is checked by the tests; a decode failure here is a build defect, not a runtime path.
        try! JSONDecoder().decode(LayoutDocument.self, from: Data(DefaultLayout.json.utf8))
    }
}
