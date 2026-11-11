import Foundation

// Themes as data (plan section 5, schema version 3). A theme is JSON: colours (one value, or {light, dark}), type
// stacks and a named scale, radii, elevation (shadow, material, behind), motion (with reduced-motion values) and the
// mark. The loader reads the bundled themes (mobile/themes/*.json, embedded by scripts/embed-assets.py as
// BundledThemes), any *.json dropped into Application Support/Autoradio/themes at run time, and themes installed from
// a Texture (`GET /api/texture/theme?doc=`, saved there as texture-<id>.json). It validates each, gates contrast, and
// resolves one theme for one scheme into `Theme` (flat strings and numbers the contract binds directly) and
// `ThemeExtras` (the type scale and the rest of the mark and motion tokens). No component names a colour, a font or a
// radius. Lane 7 owns this file.
//
// Ways it can fail (written before the code; tests are in ThemeLoaderTests):
//  F1 A theme whose text is unreadable on its paper loads (the gate: fg and muted on paper and paper2 at 4.5, onAccent on
//     accent at 3, accent on paper at 3, on every scheme the theme claims; a user theme with a bad pair is refused with
//     the reason, the others still load).
//  F2 A theme names a face the phone does not have and the text falls back silently: stacks are reduced to the first
//     entry Exact resolves (ui-serif, system-ui, ui-rounded, ui-monospace; serif, sans-serif, monospace map to those); a
//     stack with none is refused.
//  F3 A value carries CSS that is not a value (url(, ;, braces, var(, calc(, @import, javascript:, expression): refused,
//     so a forked Texture cannot paint outside its roles.
//  F4 A scheme the theme does not define is shown (a dark-only theme in a light phone): the effective scheme is limited to
//     the sides the theme defines.
//  F5 A re-load of the user directory duplicates problems or the theme list: problems are kept per source.
//  F6 Reduced motion leaves springs and entrances running: with it on, enter is the theme's reduced value, the spring is
//     critically damped and quick, and the mark's drift is zero when its reduced value is "static".
//  F7 A theme installed from a Texture replaces a bundled theme by accident, or an invalid one is written to disk: the
//     install validates before it writes, and the file name is texture-<id>.json; the origin is shown on the card.
//  F8 A new colour role is missing in a partial theme: roles fall back to the base (paper2 to paper, accent2 to accent).
//  F9 A Texture or a file installs a theme under a bundled id (or the reserved id "default") and replaces the bundled one: refused.
//  F10 A chosen theme that has only one scheme (Chrome Pastel, Liquid Metal) meets a mode control that asks for the other: the
//     control shows the one scheme and refuses the other; the "default" id clears the choice and gives the person's mode back.

public struct Theme: Codable, Sendable, Equatable {
    public var id = "", name = "", scheme = "light", schemes = "light,dark"
    // colours for the resolved scheme (hex or rgba strings)
    public var paper = "#FFFFFF", paper2 = "#FFFFFF", fg = "#000000", muted = "#555555", accent = "#0066CC", accent2 = "#004499"
    public var success = "#0C6644", danger = "#A3142F", onAccent = "#FFFFFF", rule = "#999999"
    // the dimming scrim behind the pile's cards (a colour with its own alpha)
    public var scrim = "rgba(0,0,0,.7)", onScrim = "#FFFFFF"
    // type: CSS font stacks reduced to the one face Exact resolves; `fontRead` is the body face
    public var fontRead = "ui-serif", fontUi = "system-ui", fontMono = "ui-monospace", scale = "editorial"
    public var radiusCard = 14.0, radiusControl = 12.0, radiusPill = 999.0, radiusRules = 0.0
    // elevation 1..3: shadow (CSS), edge (CSS), material (glass | solid), press scale, what is behind a level-3 layer
    public var shadow1 = "none", shadow2 = "none", shadow3 = "none"
    public var edge1 = "none", edge2 = "none", edge3 = "none"
    public var material1 = "solid", material2 = "solid", material3 = "solid"
    public var press1 = 1.0
    public var behindScale3 = 1.0, behindDim3 = 0.0
    // the page-colour backing under the bar (level 2) and under a layer (level 3): how opaque, so text stays legible over the page
    public var barBacking = 0.94, layerBacking = 0.86
    // motion, in milliseconds, and the spring
    public var motionFastMs = 120.0, motionEnterMs = 320.0, motionSheetMs = 260.0, staggerMs = 40.0
    public var springStiffness = 240.0, springDamping = 26.0, springMass = 1.0
    public var reducedMotion = false
    // the mark
    public var markDriftS = 60.0, markListenScale = 1.15
    public var markTumble = "3.3s,5.9s"
    public init() {}
}

/// The rest of the tokens: the type scale and the mark's flip, the reduced-motion values, press tint, the contrast level.
/// Source `themeExtras` (interface/settings.json). Lane 9 reads the mark's from here; every page reads the scale.
public struct ThemeExtras: Codable, Sendable, Equatable {
    public var sizeNameplate = 36.0, sizeHeadline = 28.0, sizeH2 = 19.0, sizeBody = 18.0, sizeLabel = 15.0, sizeMeta = 13.0
    public var lineBody = 1.6, lineHeadline = 1.12
    public var weightHeadline = 700.0, weightUi = 600.0
    public var flipStiffness = 300.0, flipDamping = 22.0
    public var markTumbleAS = 3.3, markTumbleBS = 5.9
    public var markStatic = false
    public var pressTint = false
    public var reduceMotion = false
    public var contrast = "AA"
    public init() {}
}

public struct ThemeInfo: Codable, Sendable, Equatable {
    public var id: String, name: String, current: Bool
}

/// A theme as the preview card shows it: its own colours, type and depth in the scheme in force, not the app's.
public struct ThemeCard: Codable, Sendable, Equatable {
    public var id = "", name = "", origin = "bundled", current = false
    public var schemes = "light,dark", contrast = "AA", scale = "editorial"
    public var paper = "", paper2 = "", fg = "", muted = "", accent = "", accent2 = "", onAccent = "", rule = ""
    public var fontRead = "ui-serif", fontUi = "system-ui"
    public var radiusCard = 14.0, radiusControl = 12.0
    public var shadow = "none", material = "solid"
    public init() {}
}

public enum ThemeError: Error, CustomStringConvertible {
    case invalid(String)
    public var description: String { if case .invalid(let m) = self { return m }; return "theme" }
}

/// A colour that is one string or {light, dark}.
struct SchemeColor: Decodable {
    var light: String, dark: String
    init(from d: Decoder) throws {
        let c = try d.singleValueContainer()
        if let s = try? c.decode(String.self) { light = s; dark = s; return }
        let o = try c.decode([String: String].self)
        guard let l = o["light"] ?? o["dark"], let k = o["dark"] ?? o["light"] else { throw ThemeError.invalid("colour needs light or dark") }
        light = l; dark = k
    }
    func pick(_ scheme: String) -> String { scheme == "dark" ? dark : light }
}

struct ThemeDoc: Decodable {
    struct Level: Decodable { var shadow: String?; var edge: String?; var material: String?; var press: Double?; var pressTint: Bool?
        var backing: Double?
        var behind: Behind? ; struct Behind: Decodable { var scale: Double?; var dim: Double? } }
    struct TypeDoc: Decodable { var read: String?; var ui: String?; var mono: String?; var scale: String?; var sizes: [String: Double]? }
    struct Motion: Decodable {
        var fast: String?; var enter: String?; var sheet: String?; var stagger: String?
        struct Spring: Decodable { var stiffness: Double?; var damping: Double?; var mass: Double? }
        var spring: Spring?
        struct Reduced: Decodable { var enter: String?; var spring: String? }
        var reduced: Reduced?
    }
    struct Mark: Decodable {
        var drift: String?; var tumble: [String]?; var listenScale: Double?
        struct Flip: Decodable { var stiffness: Double?; var damping: Double? }
        var flip: Flip?
        var reduced: String?
    }
    var schema_version: Int
    var id: String
    var name: String
    var schemes: [String]?
    var colors: [String: SchemeColor]
    var type: TypeDoc?
    var radii: [String: String]?
    var elevation: [String: Level]?
    var motion: Motion?
    var mark: Mark?
}

/// Named type scales (the plan's type table; a theme names one and may override single steps with `type.sizes`).
public enum TypeScale {
    struct Steps { var nameplate, headline, h2, body, label, meta, lineBody: Double }
    static let named: [String: Steps] = [
        "editorial": Steps(nameplate: 36, headline: 28, h2: 19, body: 18, label: 15, meta: 13, lineBody: 1.6),
        "large": Steps(nameplate: 38, headline: 30, h2: 21, body: 20, label: 16, meta: 14, lineBody: 1.62),
        "compact": Steps(nameplate: 32, headline: 25, h2: 17, body: 16, label: 14, meta: 12, lineBody: 1.5),
    ]
    public static var names: [String] { named.keys.sorted() }
}

public final class ThemeStore: @unchecked Sendable {
    /// The defaults for a person who has not chosen a theme: the light one in a light scheme, the dark one in a dark scheme
    /// (the web's Chrome Pastel and Liquid Metal). A chosen theme (AutoradioThemeID) always wins and is never rewritten.
    public static let defaultLightID = "chrome-pastel"
    public static let defaultDarkID = "liquid-metal"
    public static let defaultID = defaultLightID
    /// The id Settings sends to go back to the default pair (it clears the choice). No theme may take it.
    public static let defaultChoice = "default"
    /// The faces Exact resolves on the phone, and the generics that map to them.
    static let faces: Set<String> = ["ui-serif", "system-ui", "ui-rounded", "ui-monospace"]
    static let genericMap: [String: String] = ["serif": "ui-serif", "sans-serif": "system-ui", "monospace": "ui-monospace", "ui-sans-serif": "system-ui"]

    private let lock = NSLock()
    private var docs: [String: ThemeDoc] = [:]
    private var origins: [String: String] = [:]        // id -> bundled | file | texture
    private var order: [String] = []
    private var sourceProblems: [String: String] = [:] // source name -> reason (F5)
    private var chosenID: String?                     // nil: no choice made, the default for the scheme shows
    private var mode: String                           // light | dark | system
    private var systemScheme: @Sendable () -> String
    /// The system's Reduce Motion; the host sets it (the module reads UIAccessibility).
    public var reduceMotion: @Sendable () -> Bool
    private let userDir: URL?
    private let defaults: UserDefaults
    public var problems: [String] { lock.lock(); defer { lock.unlock() }; return sourceProblems.keys.sorted().map { "\($0): \(sourceProblems[$0]!)" } }

    /// `bundled` are name -> JSON text; `userDir` holds extra *.json read at run time.
    public init(bundled: [String: String] = BundledThemes.json, userDir: URL?, defaults: UserDefaults = .standard,
                systemScheme: @escaping @Sendable () -> String = { "light" },
                reduceMotion: @escaping @Sendable () -> Bool = { false }) {
        self.userDir = userDir; self.defaults = defaults; self.systemScheme = systemScheme; self.reduceMotion = reduceMotion
        chosenID = defaults.string(forKey: "AutoradioThemeID")
        mode = defaults.string(forKey: "AutoradioThemeMode") ?? "system"
        for k in bundled.keys.sorted() { _ = add(Data(bundled[k]!.utf8), source: k, origin: "bundled") }
        reload()
    }

    /// Read the user's theme files again (a theme dropped in switches in without a rebuild).
    public func reload() {
        guard let userDir, let names = try? FileManager.default.contentsOfDirectory(atPath: userDir.path) else { return }
        for n in names.sorted() where n.hasSuffix(".json") {
            if let d = try? Data(contentsOf: userDir.appendingPathComponent(n)) {
                _ = add(d, source: n, origin: n.hasPrefix("texture-") ? "texture" : "file")
            }
        }
    }

    /// Validate and add. Returns the id, or throws the reason (and records it against `source`).
    @discardableResult
    private func add(_ data: Data, source: String, origin: String) -> String? {
        do {
            let doc = try JSONDecoder().decode(ThemeDoc.self, from: data)
            try Self.validate(doc)
            lock.lock()
            if origin != "bundled", origins[doc.id] == "bundled" {   // F9: nothing shadows a bundled theme
                sourceProblems[source] = "id \(doc.id) is a bundled theme and cannot be replaced"; lock.unlock(); return nil
            }
            if docs[doc.id] == nil { order.append(doc.id) }; docs[doc.id] = doc; origins[doc.id] = origin
            sourceProblems[source] = nil; lock.unlock()
            return doc.id
        } catch {
            lock.lock(); sourceProblems[source] = "\(error)"; lock.unlock()
            return nil
        }
    }

    /// Install a theme served by `GET /api/texture/theme` (the validated `theme` object, as JSON). It is validated again here
    /// (the phone does not trust the wire), written to the user directory as texture-<id>.json (so it survives a relaunch) and
    /// listed with origin "texture". Throws the reason when it is not a usable theme (F7).
    @discardableResult
    public func installFromTexture(_ themeJSON: Data) throws -> String {
        let doc = try JSONDecoder().decode(ThemeDoc.self, from: themeJSON)
        try Self.validate(doc)
        lock.lock(); let shadows = origins[doc.id] == "bundled"; lock.unlock()
        if shadows { throw ThemeError.invalid("id \(doc.id) is a bundled theme and cannot be replaced") }   // F9
        if let userDir {
            try? FileManager.default.createDirectory(at: userDir, withIntermediateDirectories: true)
            try themeJSON.write(to: userDir.appendingPathComponent("texture-\(doc.id).json"), options: .atomic)
        }
        lock.lock(); if docs[doc.id] == nil { order.append(doc.id) }; docs[doc.id] = doc; origins[doc.id] = "texture"
        sourceProblems["texture-\(doc.id).json"] = nil; lock.unlock()
        return doc.id
    }

    /// Remove a theme that came from a file or a Texture (a bundled theme stays).
    public func remove(_ id: String) {
        lock.lock()
        guard let o = origins[id], o != "bundled" else { lock.unlock(); return }
        docs[id] = nil; origins[id] = nil; order.removeAll { $0 == id }
        if chosenID == id { chosenID = nil; defaults.removeObject(forKey: "AutoradioThemeID") }
        lock.unlock()
        if let userDir, let names = try? FileManager.default.contentsOfDirectory(atPath: userDir.path) {
            for n in names where n == "texture-\(id).json" || n == "\(id).json" { try? FileManager.default.removeItem(at: userDir.appendingPathComponent(n)) }
        }
    }

    public func list() -> [ThemeInfo] {
        let cur = themeID
        lock.lock(); defer { lock.unlock() }
        return order.map { ThemeInfo(id: $0, name: docs[$0]?.name ?? $0, current: $0 == cur) }
    }

    /// The theme in force: the chosen one, else the default for the scheme wanted (the mode, or the system's).
    public var themeID: String {
        lock.lock(); let chosen = chosenID, m = mode; lock.unlock()
        let wanted = m == "system" ? systemScheme() : m
        lock.lock(); defer { lock.unlock() }
        if let chosen, docs[chosen] != nil { return chosen }
        let d = wanted == "dark" ? Self.defaultDarkID : Self.defaultLightID
        return docs[d] != nil ? d : (order.first ?? Self.defaultID)
    }
    private func activeDoc() -> ThemeDoc? {
        let id = themeID
        lock.lock(); defer { lock.unlock() }
        return docs[id] ?? order.first.flatMap { docs[$0] }
    }
    /// The one scheme the chosen theme can show (a theme that defines only light or only dark), else nil. While it is set the
    /// mode control shows that scheme and refuses the other (F10): the screen is never half one scheme and half the other.
    /// The mode the person set is kept, so choosing the Default or a two-scheme theme gives it back.
    public var lockedScheme: String? {
        lock.lock(); defer { lock.unlock() }
        guard let c = chosenID, let sides = docs[c]?.schemes, sides.count == 1 else { return nil }
        return sides[0]
    }
    public var schemeMode: String { let l = lockedScheme; lock.lock(); defer { lock.unlock() }; return l ?? mode }

    /// A theme id, or `defaultChoice` to clear the choice and show the default pair (Chrome Pastel in light, Liquid Metal in dark).
    public func setTheme(_ id: String) {
        lock.lock(); defer { lock.unlock() }
        if id == Self.defaultChoice { chosenID = nil; defaults.removeObject(forKey: "AutoradioThemeID") }
        else if docs[id] != nil { chosenID = id; defaults.set(id, forKey: "AutoradioThemeID") }
    }
    /// light, dark or system
    public func setMode(_ m: String) {
        guard ["light", "dark", "system"].contains(m) else { return }
        if let l = lockedScheme, m != l { return }
        lock.lock(); mode = m; defaults.set(m, forKey: "AutoradioThemeMode"); lock.unlock()
    }

    /// The scheme in force: the chosen one, else the system's, limited to the sides the theme defines.
    public func effectiveScheme() -> String {
        lock.lock(); let m = mode; lock.unlock()
        return Self.limited(m == "system" ? systemScheme() : m, activeDoc())
    }
    static func limited(_ wanted: String, _ doc: ThemeDoc?) -> String {
        let sides = doc?.schemes ?? ["light", "dark"]
        return sides.contains(wanted) ? wanted : (sides.first ?? "light")
    }

    public func current() -> Theme {
        let doc = activeDoc()
        guard let doc else { return Theme() }
        return Self.resolve(doc, scheme: effectiveScheme(), reduced: reduceMotion())
    }

    public func extras() -> ThemeExtras {
        let doc = activeDoc()
        guard let doc else { return ThemeExtras() }
        return Self.resolveExtras(doc, scheme: effectiveScheme(), reduced: reduceMotion())
    }

    /// Every theme as a preview card, in the scheme in force (or the theme's own side when it lacks that one).
    public func cards() -> [ThemeCard] {
        let cur = themeID
        lock.lock(); let ids = order, ds = docs, os = origins, chosen = chosenID; lock.unlock()
        let wanted = effectiveScheme()     // every card in the scheme in force (a locked theme leaves no mixed screen)
        let hasChoice = chosen.map { ds[$0] != nil } ?? false
        func card(_ id: String, _ d: ThemeDoc) -> ThemeCard {
            let t = Self.resolve(d, scheme: Self.limited(wanted, d), reduced: false)
            var c = ThemeCard()
            c.id = id; c.name = d.name; c.origin = os[d.id] ?? "bundled"; c.current = id == cur && hasChoice
            c.schemes = t.schemes; c.contrast = Self.contrastLevel(d); c.scale = t.scale
            c.paper = t.paper; c.paper2 = t.paper2; c.fg = t.fg; c.muted = t.muted; c.accent = t.accent; c.accent2 = t.accent2
            c.onAccent = t.onAccent; c.rule = t.rule; c.fontRead = t.fontRead; c.fontUi = t.fontUi
            c.radiusCard = t.radiusCard; c.radiusControl = t.radiusControl; c.shadow = t.shadow1; c.material = t.material2
            return c
        }
        // The Default entry comes first: the pair (Chrome Pastel in light, Liquid Metal in dark), painted as the one in force.
        var out: [ThemeCard] = []
        if let d = ds[cur] {
            var c = card(Self.defaultChoice, d)
            c.id = Self.defaultChoice; c.name = "Default"; c.origin = "bundled"; c.current = !hasChoice; c.schemes = "light,dark"
            out.append(c)
        }
        return out + ids.compactMap { id in
            guard let d = ds[id] else { return nil }
            return card(id, d)
        }
    }

    // MARK: Validation

    static let unsafe = ["url(", "expression", "@import", "javascript:", "var(", "calc(", ";", "{", "}", "<", ">", "\\"]
    static func safe(_ s: String) -> Bool { let l = s.lowercased(); return !unsafe.contains { l.contains($0) } && s.count <= 240 }

    static func validate(_ d: ThemeDoc) throws {
        guard d.schema_version == 3 else { throw ThemeError.invalid("schema_version \(d.schema_version) is not 3") }
        guard d.id.range(of: #"^[a-z0-9][a-z0-9-]{0,39}$"#, options: .regularExpression) != nil else { throw ThemeError.invalid("id must be lower case letters, digits and dashes") }
        guard d.id != ThemeStore.defaultChoice else { throw ThemeError.invalid("the id default is reserved for the default pair") }
        guard !d.name.isEmpty, d.name.count <= 40 else { throw ThemeError.invalid("name must be 1 to 40 characters") }
        let sides = d.schemes ?? ["light", "dark"]
        guard !sides.isEmpty, sides.allSatisfy({ $0 == "light" || $0 == "dark" }) else { throw ThemeError.invalid("schemes must be light and/or dark") }
        for (k, c) in d.colors {
            for s in [c.light, c.dark] where !safe(s) || rgb(s) == nil { throw ThemeError.invalid("colour \(k) is not hex or rgb()") }
        }
        for role in ["paper", "fg", "muted", "accent", "onAccent"] where d.colors[role] == nil { throw ThemeError.invalid("colour \(role) is missing") }
        if let t = d.type {
            for (k, v) in [("read", t.read), ("ui", t.ui), ("mono", t.mono)] {
                guard let v else { continue }
                guard safe(v), face(v) != nil else { throw ThemeError.invalid("type.\(k) names no face the phone has (ui-serif, system-ui, ui-rounded, ui-monospace)") }
            }
            if let s = t.scale, TypeScale.named[s] == nil { throw ThemeError.invalid("type.scale \(s) is not one of \(TypeScale.names.joined(separator: ", "))") }
        }
        for (k, v) in d.radii ?? [:] where !(safe(v) && Double(v.replacingOccurrences(of: "px", with: "")) != nil) { throw ThemeError.invalid("radii.\(k) is not a px length") }
        for (k, l) in d.elevation ?? [:] {
            for s in [l.shadow, l.edge].compactMap({ $0 }) where !safe(s) { throw ThemeError.invalid("elevation.\(k) has a value that is not a shadow") }
            if let m = l.material, m != "glass", m != "solid" { throw ThemeError.invalid("elevation.\(k).material is glass or solid") }
        }
        if let m = d.motion {
            for s in [m.fast, m.enter, m.sheet, m.stagger].compactMap({ $0 }) where !safe(s) || ms(s) == nil { throw ThemeError.invalid("motion value \(s.prefix(20)) is not a duration") }
            if let sp = m.spring {
                if let v = sp.stiffness, !(1...3000).contains(v) { throw ThemeError.invalid("spring stiffness out of range") }
                if let v = sp.damping, !(1...300).contains(v) { throw ThemeError.invalid("spring damping out of range") }
                if let v = sp.mass, !(0.1...10).contains(v) { throw ThemeError.invalid("spring mass out of range") }
            }
        }
        if let k = d.mark, let t = k.tumble, t.count != 2 || t.contains(where: { ms($0) == nil }) { throw ThemeError.invalid("mark.tumble is two durations") }
        for scheme in sides {
            let t = resolve(d, scheme: scheme, reduced: false)
            if let bad = contrastProblem(t) { throw ThemeError.invalid("\(bad) in \(scheme)") }
        }
    }

    /// The first face in a stack that Exact resolves; generics map to ui-serif / system-ui / ui-monospace. nil: none.
    static func face(_ stack: String) -> String? {
        for part in stack.split(separator: ",") {
            let n = part.trimmingCharacters(in: CharacterSet(charactersIn: " '\"")).lowercased()
            if faces.contains(n) { return n }
            if let m = genericMap[n] { return m }
        }
        return nil
    }

    static func ms(_ s: String?) -> Double? {
        guard let s else { return nil }
        let first = s.split(separator: " ").first.map(String.init) ?? s
        if first == "none" { return 0 }
        if first.hasSuffix("ms") { return Double(first.dropLast(2)) }
        if first.hasSuffix("s") { return Double(first.dropLast(1)).map { $0 * 1000 } }
        return nil
    }

    // MARK: Resolution

    static func resolve(_ d: ThemeDoc, scheme: String, reduced: Bool) -> Theme {
        var t = Theme()
        t.id = d.id; t.name = d.name; t.scheme = scheme; t.schemes = (d.schemes ?? ["light", "dark"]).joined(separator: ",")
        func c(_ k: String, _ fallback: String) -> String { d.colors[k]?.pick(scheme) ?? fallback }
        t.paper = c("paper", t.paper); t.paper2 = c("paper2", t.paper); t.fg = c("fg", t.fg); t.muted = c("muted", t.muted)
        t.accent = c("accent", t.accent); t.accent2 = c("accent2", t.accent); t.success = c("success", t.success)
        t.danger = c("danger", t.danger); t.onAccent = c("onAccent", t.onAccent); t.rule = c("rule", t.rule); t.scrim = c("scrim", t.scrim); t.onScrim = c("onScrim", t.onScrim)
        if let ty = d.type {
            t.fontRead = ty.read.flatMap(face) ?? t.fontRead; t.fontUi = ty.ui.flatMap(face) ?? t.fontUi
            t.fontMono = ty.mono.flatMap(face) ?? t.fontMono; t.scale = ty.scale ?? t.scale
        }
        func px(_ s: String?) -> Double? { s.flatMap { Double($0.replacingOccurrences(of: "px", with: "")) } }
        if let r = d.radii { t.radiusCard = px(r["card"]) ?? t.radiusCard; t.radiusControl = px(r["control"]) ?? t.radiusControl
            t.radiusPill = px(r["pill"]) ?? t.radiusPill; t.radiusRules = px(r["rules"]) ?? t.radiusRules }
        func edge(_ s: String?) -> String {
            // "0 1px 0 rule" names a colour token: map it to the resolved rule colour.
            (s ?? "none").replacingOccurrences(of: " rule", with: " " + t.rule)
        }
        if let e = d.elevation {
            if let l = e["1"] { t.shadow1 = l.shadow ?? "none"; t.edge1 = edge(l.edge); t.material1 = l.material ?? "solid"; t.press1 = l.press ?? 1 }
            if let l = e["2"] { t.shadow2 = l.shadow ?? "none"; t.edge2 = edge(l.edge); t.material2 = l.material ?? "solid"; t.barBacking = l.backing ?? t.barBacking }
            if let l = e["3"] { t.shadow3 = l.shadow ?? "none"; t.edge3 = edge(l.edge); t.material3 = l.material ?? "solid"
                t.behindScale3 = l.behind?.scale ?? 1; t.behindDim3 = l.behind?.dim ?? 0; t.layerBacking = l.backing ?? t.layerBacking }
        }
        func joined(_ shadow: String, _ edge: String) -> String {
            if shadow == "none" { return edge }
            return edge == "none" ? shadow : shadow + ", " + edge
        }
        t.shadow1 = joined(t.shadow1, t.edge1); if t.material2 != "glass" { t.shadow2 = joined(t.shadow2, t.edge2) }; t.shadow3 = joined(t.shadow3, t.edge3)
        if let m = d.motion {
            t.motionFastMs = ms(m.fast) ?? t.motionFastMs; t.motionEnterMs = ms(m.enter) ?? t.motionEnterMs
            t.motionSheetMs = ms(m.sheet) ?? t.motionSheetMs; t.staggerMs = ms(m.stagger) ?? t.staggerMs
            t.springStiffness = m.spring?.stiffness ?? t.springStiffness; t.springDamping = m.spring?.damping ?? t.springDamping
            t.springMass = m.spring?.mass ?? t.springMass
        }
        if let k = d.mark { t.markDriftS = (ms(k.drift) ?? 0) / 1000; t.markListenScale = k.listenScale ?? 1
            t.markTumble = (k.tumble ?? []).joined(separator: ",") }
        if reduced {
            // F6: the system's Reduce Motion wins. Entrances use the theme's reduced value (default instant), the spring
            // settles at once (critically damped and stiff), the mark stops drifting when its reduced form is static.
            t.reducedMotion = true
            t.motionEnterMs = ms(d.motion?.reduced?.enter) ?? 0
            t.motionSheetMs = min(t.motionSheetMs, 120); t.staggerMs = 0
            if (d.motion?.reduced?.spring ?? "none") == "none" { t.springStiffness = 900; t.springDamping = 60; t.springMass = 1 }
            if (d.mark?.reduced ?? "static") == "static" { t.markDriftS = 0; t.markListenScale = 1 }
            t.behindScale3 = 1
        }
        return t
    }

    static func resolveExtras(_ d: ThemeDoc, scheme: String, reduced: Bool) -> ThemeExtras {
        var x = ThemeExtras()
        let name = d.type?.scale ?? "editorial"
        if let s = TypeScale.named[name] {
            x.sizeNameplate = s.nameplate; x.sizeHeadline = s.headline; x.sizeH2 = s.h2; x.sizeBody = s.body
            x.sizeLabel = s.label; x.sizeMeta = s.meta; x.lineBody = s.lineBody
        }
        if let o = d.type?.sizes {
            x.sizeNameplate = o["nameplate"] ?? x.sizeNameplate; x.sizeHeadline = o["headline"] ?? x.sizeHeadline
            x.sizeH2 = o["h2"] ?? x.sizeH2; x.sizeBody = o["body"] ?? x.sizeBody; x.sizeLabel = o["label"] ?? x.sizeLabel
            x.sizeMeta = o["meta"] ?? x.sizeMeta
        }
        x.flipStiffness = d.mark?.flip?.stiffness ?? x.flipStiffness; x.flipDamping = d.mark?.flip?.damping ?? x.flipDamping
        let tum = (d.mark?.tumble ?? []).compactMap { ms($0) }.map { $0 / 1000 }
        if tum.count == 2 { x.markTumbleAS = tum[0]; x.markTumbleBS = tum[1] }
        x.markStatic = reduced && (d.mark?.reduced ?? "static") == "static"
        x.pressTint = d.elevation?["1"]?.pressTint ?? false
        x.reduceMotion = reduced
        x.contrast = contrastLevel(d)
        return x
    }

    // MARK: Contrast gate (the web's pairs: fg on paper 4.5, muted on paper 4.5, onAccent on accent 3; plus paper2 and accent)

    static func contrastProblem(_ t: Theme) -> String? {
        guard let paper = rgb(t.paper), let paper2 = rgb(t.paper2), let fg = rgb(t.fg), let muted = rgb(t.muted),
              let onA = rgb(t.onAccent), let acc = rgb(t.accent)
        else { return "a colour is not hex or rgb()" }
        if ratio(fg, paper) < 4.5 { return "fg on paper below 4.5" }
        if ratio(muted, paper) < 4.5 { return "muted on paper below 4.5" }
        if ratio(fg, paper2) < 4.5 { return "fg on paper2 below 4.5" }
        if ratio(muted, paper2) < 4.5 { return "muted on paper2 below 4.5" }
        if ratio(onA, acc) < 3 { return "onAccent on accent below 3" }
        if ratio(acc, paper) < 3 { return "accent on paper below 3" }
        return nil
    }
    /// "AAA" when every pair clears the enhanced level (7 for text, 4.5 for accent pairs) on every scheme, else "AA".
    static func contrastLevel(_ d: ThemeDoc) -> String {
        for scheme in d.schemes ?? ["light", "dark"] {
            let t = resolve(d, scheme: scheme, reduced: false)
            guard let paper = rgb(t.paper), let paper2 = rgb(t.paper2), let fg = rgb(t.fg), let muted = rgb(t.muted),
                  let onA = rgb(t.onAccent), let acc = rgb(t.accent) else { return "AA" }
            if min(ratio(fg, paper), ratio(muted, paper), ratio(fg, paper2), ratio(muted, paper2)) < 7 { return "AA" }
            if min(ratio(onA, acc), ratio(acc, paper)) < 4.5 { return "AA" }
        }
        return "AAA"
    }
    static func rgb(_ s: String) -> (Double, Double, Double)? {
        let v = s.trimmingCharacters(in: .whitespaces)
        if v.hasPrefix("#") {
            var h = String(v.dropFirst()); if h.count == 3 { h = h.map { "\($0)\($0)" }.joined() }
            guard h.count >= 6, let n = UInt32(h.prefix(6), radix: 16) else { return nil }
            return (Double((n >> 16) & 255), Double((n >> 8) & 255), Double(n & 255))
        }
        if v.hasPrefix("rgb") {
            let nums = v.drop { $0 != "(" }.dropFirst().prefix { $0 != ")" }.split(whereSeparator: { ",/ ".contains($0) }).prefix(3).compactMap { Double($0) }
            return nums.count == 3 ? (nums[0], nums[1], nums[2]) : nil
        }
        return nil
    }
    static func lum(_ c: (Double, Double, Double)) -> Double {
        func f(_ x: Double) -> Double { let s = x / 255; return s <= 0.03928 ? s / 12.92 : pow((s + 0.055) / 1.055, 2.4) }
        return 0.2126 * f(c.0) + 0.7152 * f(c.1) + 0.0722 * f(c.2)
    }
    static func ratio(_ a: (Double, Double, Double), _ b: (Double, Double, Double)) -> Double {
        let x = lum(a), y = lum(b); return (max(x, y) + 0.05) / (min(x, y) + 0.05)
    }
}

// MARK: Themes from a Texture

/// What `GET /api/texture/theme?doc=` answers: `{doc_id, revision_id, theme, ignored?}`.
public struct TextureTheme: Sendable, Equatable {
    public var docID: String, revisionID: String
    public var themeJSON: Data
    public var ignored: [String]
    public init(docID: String, revisionID: String, themeJSON: Data, ignored: [String] = []) {
        self.docID = docID; self.revisionID = revisionID; self.themeJSON = themeJSON; self.ignored = ignored
    }
    /// Parse the route's answer. Also accepts a bare theme object (a file) so a test fixture can be either.
    public static func parse(_ data: Data) throws -> TextureTheme {
        guard let o = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw ThemeError.invalid("the answer is not an object") }
        let themeObj = (o["theme"] as? [String: Any]) ?? o
        guard themeObj["schema_version"] != nil else { throw ThemeError.invalid("the answer holds no theme") }
        let json = try JSONSerialization.data(withJSONObject: themeObj, options: [.sortedKeys])
        return TextureTheme(docID: o["doc_id"] as? String ?? "", revisionID: o["revision_id"] as? String ?? "", themeJSON: json,
                            ignored: o["ignored"] as? [String] ?? [])
    }
}

public protocol ThemeSource: Sendable {
    func fetch(doc: String) async throws -> TextureTheme
}

/// The live route. 404 means no such theme document; 422 carries the server's reason (the document is not a valid theme).
public struct ChoirThemeSource: ThemeSource {
    public init() {}
    public func fetch(doc: String) async throws -> TextureTheme {
        let r = try await ChoirEnvironment.shared.client().send("GET", "/api/texture/theme", query: ["doc": doc], route: "/api/texture/theme")
        return try TextureTheme.parse(r.data)
    }
}

/// Demo mode: the bundled fixture answers for the one demo document and refuses any other id, as the route would.
public struct DemoThemeSource: ThemeSource {
    public init() {}
    public func fetch(doc: String) async throws -> TextureTheme {
        guard doc == "demo-theme", let d = Demo.fixture("texture-theme") else { throw ChoirError.server(status: 404, message: "no such theme document") }
        return try TextureTheme.parse(d)
    }
}
