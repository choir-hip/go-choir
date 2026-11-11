import Foundation

// Themes as data (plan section 5, schema version 3). A theme is JSON: colours (one value, or {light, dark}), type
// stacks, radii, elevation (shadow, material, behind), motion and the mark. The loader reads the bundled themes
// (mobile/themes/*.json, embedded by scripts/embed-assets.py as BundledThemes) and any *.json dropped into
// Application Support/Autoradio/themes at run time, validates the contrast gate, and resolves one theme for one
// scheme into `Theme`, a flat record of strings and numbers the contract binds directly: no component names a
// colour, a font or a radius.

public struct Theme: Codable, Sendable, Equatable {
    public var id = "", name = "", scheme = "light", schemes = "light,dark"
    // colours for the resolved scheme (hex or rgba strings)
    public var paper = "#FFFFFF", paper2 = "#FFFFFF", fg = "#000000", muted = "#555555", accent = "#0066CC", accent2 = "#004499"
    public var success = "#0C6644", danger = "#A3142F", onAccent = "#FFFFFF", rule = "#999999"
    // type: CSS font stacks; `fontRead` is the body face
    public var fontRead = "ui-serif", fontUi = "system-ui", fontMono = "ui-monospace", scale = "editorial"
    public var radiusCard = 14.0, radiusControl = 12.0, radiusPill = 999.0, radiusRules = 0.0
    // elevation 1..3: shadow (CSS), edge (CSS), material (glass | solid), press scale, what is behind a level-3 layer
    public var shadow1 = "none", shadow2 = "none", shadow3 = "none"
    public var edge1 = "none", edge2 = "none", edge3 = "none"
    public var material1 = "solid", material2 = "solid", material3 = "solid"
    public var press1 = 1.0
    public var behindScale3 = 1.0, behindDim3 = 0.0
    // motion, in milliseconds, and the spring
    public var motionFastMs = 120.0, motionEnterMs = 320.0, motionSheetMs = 260.0, staggerMs = 40.0
    public var springStiffness = 240.0, springDamping = 26.0, springMass = 1.0
    public var reducedMotion = false
    // the mark
    public var markDriftS = 60.0, markListenScale = 1.15
    public var markTumble = "3.3s,5.9s"
    public init() {}
}

public struct ThemeInfo: Codable, Sendable, Equatable {
    public var id: String, name: String, current: Bool
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
    struct Level: Decodable { var shadow: String?; var edge: String?; var material: String?; var press: Double?
        var behind: Behind? ; struct Behind: Decodable { var scale: Double?; var dim: Double? } }
    struct TypeDoc: Decodable { var read: String?; var ui: String?; var mono: String?; var scale: String? }
    struct Motion: Decodable {
        var fast: String?; var enter: String?; var sheet: String?; var stagger: String?
        struct Spring: Decodable { var stiffness: Double?; var damping: Double?; var mass: Double? }
        var spring: Spring?
    }
    struct Mark: Decodable { var drift: String?; var tumble: [String]?; var listenScale: Double? }
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

public final class ThemeStore: @unchecked Sendable {
    public static let defaultID = "depth"
    private let lock = NSLock()
    private var docs: [String: ThemeDoc] = [:]
    private var order: [String] = []
    private var currentID: String
    private var mode: String                   // light | dark | system
    private var systemScheme: @Sendable () -> String
    private let userDir: URL?
    private let defaults: UserDefaults
    public private(set) var problems: [String] = []

    /// `bundled` are name -> JSON text; `userDir` holds extra *.json read at run time.
    public init(bundled: [String: String] = BundledThemes.json, userDir: URL?, defaults: UserDefaults = .standard,
                systemScheme: @escaping @Sendable () -> String = { "light" }) {
        self.userDir = userDir; self.defaults = defaults; self.systemScheme = systemScheme
        currentID = defaults.string(forKey: "AutoradioThemeID") ?? Self.defaultID
        mode = defaults.string(forKey: "AutoradioThemeMode") ?? "system"
        for k in bundled.keys.sorted() { add(Data(bundled[k]!.utf8), source: k) }
        reload()
    }

    /// Read the user's theme files again (a theme dropped in switches in without a rebuild).
    public func reload() {
        guard let userDir, let names = try? FileManager.default.contentsOfDirectory(atPath: userDir.path) else { return }
        for n in names.sorted() where n.hasSuffix(".json") {
            if let d = try? Data(contentsOf: userDir.appendingPathComponent(n)) { add(d, source: n) }
        }
    }

    private func add(_ data: Data, source: String) {
        do {
            let doc = try JSONDecoder().decode(ThemeDoc.self, from: data)
            guard doc.schema_version == 3 else { throw ThemeError.invalid("schema_version \(doc.schema_version) is not 3") }
            for scheme in doc.schemes ?? ["light", "dark"] {
                let t = Self.resolve(doc, scheme: scheme)
                if let bad = Self.contrastProblem(t) { throw ThemeError.invalid("\(bad) in \(scheme)") }
            }
            lock.lock(); if docs[doc.id] == nil { order.append(doc.id) }; docs[doc.id] = doc; lock.unlock()
        } catch { lock.lock(); problems.append("\(source): \(error)"); lock.unlock() }
    }

    public func list() -> [ThemeInfo] {
        lock.lock(); defer { lock.unlock() }
        return order.map { ThemeInfo(id: $0, name: docs[$0]?.name ?? $0, current: $0 == currentID) }
    }

    public var themeID: String { lock.lock(); defer { lock.unlock() }; return docs[currentID] != nil ? currentID : (order.first ?? Self.defaultID) }
    public var schemeMode: String { lock.lock(); defer { lock.unlock() }; return mode }

    public func setTheme(_ id: String) {
        lock.lock(); if docs[id] != nil { currentID = id; defaults.set(id, forKey: "AutoradioThemeID") }; lock.unlock()
    }
    /// light, dark or system
    public func setMode(_ m: String) {
        guard ["light", "dark", "system"].contains(m) else { return }
        lock.lock(); mode = m; defaults.set(m, forKey: "AutoradioThemeMode"); lock.unlock()
    }

    /// The scheme in force: the chosen one, else the system's, limited to the sides the theme defines.
    public func effectiveScheme() -> String {
        lock.lock(); let m = mode, id = currentID, doc = docs[currentID] ?? order.first.flatMap { docs[$0] }; lock.unlock()
        _ = id
        var s = m == "system" ? systemScheme() : m
        let sides = doc?.schemes ?? ["light", "dark"]
        if !sides.contains(s) { s = sides.first ?? "light" }
        return s
    }

    public func current() -> Theme {
        lock.lock(); let doc = docs[currentID] ?? order.first.flatMap { docs[$0] }; lock.unlock()
        guard let doc else { return Theme() }
        return Self.resolve(doc, scheme: effectiveScheme())
    }

    // MARK: Resolution

    static func resolve(_ d: ThemeDoc, scheme: String) -> Theme {
        var t = Theme()
        t.id = d.id; t.name = d.name; t.scheme = scheme; t.schemes = (d.schemes ?? ["light", "dark"]).joined(separator: ",")
        func c(_ k: String, _ fallback: String) -> String { d.colors[k]?.pick(scheme) ?? fallback }
        t.paper = c("paper", t.paper); t.paper2 = c("paper2", t.paper); t.fg = c("fg", t.fg); t.muted = c("muted", t.muted)
        t.accent = c("accent", t.accent); t.accent2 = c("accent2", t.accent); t.success = c("success", t.success)
        t.danger = c("danger", t.danger); t.onAccent = c("onAccent", t.onAccent); t.rule = c("rule", t.rule)
        if let ty = d.type { t.fontRead = ty.read ?? t.fontRead; t.fontUi = ty.ui ?? t.fontUi; t.fontMono = ty.mono ?? t.fontMono; t.scale = ty.scale ?? t.scale }
        func px(_ s: String?) -> Double? { s.flatMap { Double($0.replacingOccurrences(of: "px", with: "")) } }
        if let r = d.radii { t.radiusCard = px(r["card"]) ?? t.radiusCard; t.radiusControl = px(r["control"]) ?? t.radiusControl
            t.radiusPill = px(r["pill"]) ?? t.radiusPill; t.radiusRules = px(r["rules"]) ?? t.radiusRules }
        func edge(_ s: String?) -> String {
            // "0 1px 0 rule" names a colour token: map it to the resolved rule colour.
            (s ?? "none").replacingOccurrences(of: " rule", with: " " + t.rule)
        }
        if let e = d.elevation {
            if let l = e["1"] { t.shadow1 = l.shadow ?? "none"; t.edge1 = edge(l.edge); t.material1 = l.material ?? "solid"; t.press1 = l.press ?? 1 }
            if let l = e["2"] { t.shadow2 = l.shadow ?? "none"; t.edge2 = edge(l.edge); t.material2 = l.material ?? "solid" }
            if let l = e["3"] { t.shadow3 = l.shadow ?? "none"; t.edge3 = edge(l.edge); t.material3 = l.material ?? "solid"
                t.behindScale3 = l.behind?.scale ?? 1; t.behindDim3 = l.behind?.dim ?? 0 }
        }
        func joined(_ shadow: String, _ edge: String) -> String {
            if shadow == "none" { return edge }
            return edge == "none" ? shadow : shadow + ", " + edge
        }
        t.shadow1 = joined(t.shadow1, t.edge1); if t.material2 != "glass" { t.shadow2 = joined(t.shadow2, t.edge2) }; t.shadow3 = joined(t.shadow3, t.edge3)
        func ms(_ s: String?) -> Double? {
            guard let s else { return nil }
            let first = s.split(separator: " ").first.map(String.init) ?? s
            if first.hasSuffix("ms") { return Double(first.dropLast(2)) }
            if first.hasSuffix("s") { return Double(first.dropLast(1)).map { $0 * 1000 } }
            return nil
        }
        if let m = d.motion {
            t.motionFastMs = ms(m.fast) ?? t.motionFastMs; t.motionEnterMs = ms(m.enter) ?? t.motionEnterMs
            t.motionSheetMs = ms(m.sheet) ?? t.motionSheetMs; t.staggerMs = ms(m.stagger) ?? t.staggerMs
            t.springStiffness = m.spring?.stiffness ?? t.springStiffness; t.springDamping = m.spring?.damping ?? t.springDamping
            t.springMass = m.spring?.mass ?? t.springMass
        }
        if let k = d.mark { t.markDriftS = (ms(k.drift) ?? 0) / 1000; t.markListenScale = k.listenScale ?? 1
            t.markTumble = (k.tumble ?? []).joined(separator: ",") }
        return t
    }

    // MARK: Contrast gate (the web's pairs: fg on paper 4.5, muted on paper 4.5, onAccent on accent 3)

    static func contrastProblem(_ t: Theme) -> String? {
        guard let paper = rgb(t.paper), let fg = rgb(t.fg), let muted = rgb(t.muted), let onA = rgb(t.onAccent), let acc = rgb(t.accent)
        else { return "a colour is not hex or rgb()" }
        if ratio(fg, paper) < 4.5 { return "fg on paper below 4.5" }
        if ratio(muted, paper) < 4.5 { return "muted on paper below 4.5" }
        if ratio(onA, acc) < 3 { return "onAccent on accent below 3" }
        return nil
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
