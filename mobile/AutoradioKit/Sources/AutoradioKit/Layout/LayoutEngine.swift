import Foundation

// The layout engine (plan section 3): a pure function from ranked stories, a layout document, a width and a text measurer
// to placed bands. Rules walk the ranked pool band by band; each band's stories leave the pool.
//
// Ways this can fail (written before the code; LayoutTests covers each):
//  1. No stories, one story, two stories: no crash, no empty band; one story is a lead, two a pair or colead.
//  2. Equal weights: the order is total (weight, newer, route), so a refresh does not reshuffle.
//  3. A headline too long for its slot at the smallest step: the smallest step is used, lines are clamped to maxLines.
//  4. A pair whose slots do not fit: it degrades to a stack (the plan's 118-character case at 353 pt); wide: side by side.
//  5. A story appears twice in the input (same route): kept once.
//  6. A rule names an unknown module or an unparseable condition: skipped; the page still composes.
//  7. Every input story appears in at most one band, and none is invented. (A river's tail beyond perSection or
//     maxSections is not shown on the front; the section page shows it.)
//  8. Width tiny or zero: widths are floored so measuring never divides by zero.
//  9. Stories without importance, sources, a date, a dek, a section or entities: defaults, the dek is dropped, section "News".
// 10. Opinion stories never take the lead; they live in the opinion band only.

public struct LayoutEngine: Sendable {
    public var doc: LayoutDocument
    public var measurer: TextMeasurer
    public init(doc: LayoutDocument = .builtIn, measurer: TextMeasurer = DefaultMeasurer.make()) { self.doc = doc; self.measurer = measurer }

    // MARK: ranking

    public func rank(_ stories: [PaperStory], now: Date) -> [PaperStory] {
        var seen = Set<String>()
        let unique = stories.filter { seen.insert($0.route).inserted }
        return unique.sorted { a, b in
            let wa = a.weight(now: now), wb = b.weight(now: now)
            if wa != wb { return wa > wb }
            if a.updated != b.updated { return (a.updated ?? .distantPast) > (b.updated ?? .distantPast) }
            return a.route < b.route
        }
    }

    // MARK: composing

    public func compose(_ stories: [PaperStory], width: Double, now: Date) -> [PaperBand] {
        let all = rank(stories, now: now)
        var pool = all.filter { !$0.isOpinion }
        var opinions = all.filter { $0.isOpinion }
        guard !pool.isEmpty || !opinions.isEmpty else { return [] }
        let content = max(width - 2 * doc.grid.margin, 120)
        let freshCut = now.addingTimeInterval(-(doc.freshHours ?? 12) * 3600)
        let fresh = pool.filter { ($0.updated ?? .distantPast) >= freshCut }.count
        var placed = Set<String>()
        var anchor: PaperStory?
        var bands: [PaperBand] = []
        var rank = 0
        var resolved = Set<String>()

        func nextRank() -> Int { rank += 1; return rank }
        func related() -> Int {
            guard let a = anchor else { return 0 }
            let ids = Set(a.entities.map(\.id))
            return pool.filter { s in s.entities.contains { ids.contains($0.id) } }.count
        }
        func weightOf(_ i: Int) -> Double { i < pool.count ? pool[i].weight(now: now) : 0 }

        func cond(_ text: String) -> Bool {
            for term in text.components(separatedBy: " and ") {
                let t = term.trimmingCharacters(in: .whitespaces)
                if !term1(t) { return false }
            }
            return true
        }
        func compare(_ a: Double, _ op: String, _ b: Double) -> Bool {
            switch op { case ">=": return a >= b; case "<=": return a <= b; case ">": return a > b; case "<": return a < b; case "==": return a == b; default: return false }
        }
        func term1(_ t: String) -> Bool {
            if let m = t.firstMatch(#"^top\.weight\s*(>=|<=|>|<)\s*([0-9.]+)\s*\*\s*second\.weight$"#) {
                guard !pool.isEmpty, let k = Double(m[1]) else { return false }
                return compare(weightOf(0), m[0], k * weightOf(1))
            }
            if let m = t.firstMatch(#"^(fresh|remaining|related\(lead\))\s*(>=|<=|>|<|==)\s*([0-9.]+)$"#) {
                guard let n = Double(m[2]) else { return false }
                let v: Double = m[0] == "fresh" ? Double(fresh) : m[0] == "remaining" ? Double(pool.count) : Double(related())
                return compare(v, m[1], n)
            }
            if let m = t.firstMatch(#"^has\((\w+)\)$"#) {
                let name = m[0]
                return bands.contains { $0.module == name || (name == "lead" && $0.module == "colead") }
            }
            return false
        }

        func take(_ n: Int) -> [PaperStory] {
            let got = Array(pool.prefix(n)); pool.removeFirst(got.count); got.forEach { placed.insert($0.route) }; return got
        }

        for r in doc.rules {
            guard let name = r.module, let mod = doc.modules[name] else { continue }
            if let g = r.group, resolved.contains(g) { continue }
            if let c = r.cond, !cond(c) { continue }
            if let g = r.group { resolved.insert(g) }
            switch name {
            case "lead", "colead":
                guard let slots = mod.slots, !slots.isEmpty, !pool.isEmpty else { continue }
                let n = (name == "colead" && pool.count >= 2) ? 2 : 1
                let ss = take(n)
                if anchor == nil { anchor = ss.first }
                let items = ss.enumerated().map { i, s in item(s, slot: slots[min(i, slots.count - 1)], width: slotWidth(span: 6, content: content), now: now, rank: nextRank(), withEntities: i == 0 && name == "lead") }
                bands.append(PaperBand(id: "\(name)-\(bands.count)", module: n == 2 ? "colead" : "lead", label: "", side: false, rule: n == 2 && mod.rule == "between", items: items))
            case "pair":
                guard let slots = mod.slots, slots.count >= 2, pool.count >= 2 else { continue }
                let ss = take(2)
                let half = slotWidth(span: slots[0].span, content: content)
                let items = ss.enumerated().map { i, s in itemFit(s, slot: slots[i], width: half, now: now, rank: nextRank()) }
                let fits = items.allSatisfy { $0.fitted }
                if fits || mod.degrade != "stack" {
                    bands.append(PaperBand(id: "pair-\(bands.count)", module: "pair", label: "", side: true, rule: false, items: items.map { $0.item }))
                } else {
                    var sl = slots[0]; sl.headline = mod.stackHeadline ?? sl.headline; sl.maxLines = mod.stackMaxLines ?? sl.maxLines; sl.span = doc.grid.units
                    let stacked = ss.enumerated().map { i, s in item(s, slot: sl, width: slotWidth(span: doc.grid.units, content: content), now: now, rank: items[i].item.rank, withEntities: false) }
                    bands.append(PaperBand(id: "pair-\(bands.count)", module: "pair", label: "", side: false, rule: true, items: stacked))
                }
            case "cluster":
                guard let f = mod.follow, let a = anchor else { continue }
                let ids = Set(a.entities.map(\.id))
                let rel = pool.filter { s in s.entities.contains { ids.contains($0.id) } }.prefix(f.max)
                guard !rel.isEmpty else { continue }
                pool.removeAll { s in rel.contains { $0.route == s.route } }
                rel.forEach { placed.insert($0.route) }
                var sl = LayoutDocument.Slot(span: doc.grid.units, headline: f.headline, maxLines: 3, dek: false, dekLines: 0, meta: nil)
                sl.dek = false
                let items = rel.map { s -> PaperItem in var it = item(s, slot: sl, width: slotWidth(span: doc.grid.units, content: content) - 16, now: now, rank: nextRank(), withEntities: false); it.indent = true; return it }
                bands.append(PaperBand(id: "cluster-\(bands.count)", module: "cluster", label: f.label, side: false, rule: false, items: items))
            case "briefs":
                guard let l = mod.list, !pool.isEmpty else { continue }
                let ss = take(l.max)
                let sl = LayoutDocument.Slot(span: doc.grid.units, headline: l.headline, maxLines: 3, dek: false, dekLines: 0, meta: nil)
                bands.append(PaperBand(id: "briefs-\(bands.count)", module: "briefs", label: l.label ?? "", side: false, rule: true,
                                       items: ss.map { item($0, slot: sl, width: slotWidth(span: doc.grid.units, content: content), now: now, rank: nextRank(), withEntities: false) }))
            case "river":
                guard mod.bySection == true, !pool.isEmpty else { continue }
                var order: [String] = []
                for s in pool { let k = s.section.isEmpty ? "News" : s.section; if !order.contains(k) { order.append(k) } }
                let sl = LayoutDocument.Slot(span: doc.grid.units, headline: mod.headline ?? ["s"], maxLines: 3, dek: false, dekLines: 0, meta: nil)
                for sec in order.prefix(mod.maxSections ?? 5) {
                    let ss = pool.filter { ($0.section.isEmpty ? "News" : $0.section) == sec }.prefix(mod.perSection ?? 4)
                    pool.removeAll { s in ss.contains { $0.route == s.route } }
                    ss.forEach { placed.insert($0.route) }
                    bands.append(PaperBand(id: "river-\(PaperText.slug(sec))", module: "river", label: sec, side: false, rule: false,
                                           items: ss.map { item($0, slot: sl, width: slotWidth(span: doc.grid.units, content: content), now: now, rank: nextRank(), withEntities: false) }))
                }
            case "opinion":
                guard let l = mod.list, !opinions.isEmpty else { continue }
                let ss = Array(opinions.prefix(l.max)); opinions.removeFirst(ss.count)
                let sl = LayoutDocument.Slot(span: doc.grid.units, headline: l.headline, maxLines: 3, dek: false, dekLines: 0, meta: nil)
                bands.append(PaperBand(id: "opinion", module: "opinion", label: l.label ?? "Opinion", side: false, rule: true,
                                       items: ss.map { item($0, slot: sl, width: slotWidth(span: doc.grid.units, content: content), now: now, rank: nextRank(), withEntities: false) }))
            default: continue
            }
        }
        return bands
    }

    /// The composition string, for proofs and tests: the modules in order, a stacked pair marked.
    public static func composition(_ bands: [PaperBand]) -> String {
        var out: [String] = []
        for b in bands {
            let m = b.module == "pair" && !b.side ? "pair(stack)" : b.module
            if m == "river" && out.last == "river" { continue }
            out.append(m)
        }
        return out.joined(separator: "+")
    }

    // MARK: fit

    func slotWidth(span: Int, content: Double) -> Double {
        let u = Double(max(doc.grid.units, 1)), g = doc.grid.gutter
        let col = (content - g * (u - 1)) / u
        return max(col * Double(span) + g * Double(max(span - 1, 0)), 60)
    }

    struct Fit { var step: LayoutDocument.Step; var lines: Int; var fitted: Bool }

    func fit(_ title: String, steps: [String], maxLines: Int, width: Double) -> Fit {
        let candidates = steps.compactMap { doc.steps[$0] }
        let list = candidates.isEmpty ? [doc.steps.values.min { $0.size < $1.size } ?? .init(size: 17, weight: 600, lineHeight: 1.25)] : candidates
        for st in list {
            let n = measurer.lines(title, size: st.size, weight: st.weight, width: width)
            if n <= maxLines { return Fit(step: st, lines: n, fitted: true) }
        }
        let last = list[list.count - 1]
        return Fit(step: last, lines: maxLines, fitted: false)
    }

    func item(_ s: PaperStory, slot: LayoutDocument.Slot, width: Double, now: Date, rank: Int, withEntities: Bool) -> PaperItem {
        itemFit(s, slot: slot, width: width, now: now, rank: rank, withEntities: withEntities).item
    }

    func itemFit(_ s: PaperStory, slot: LayoutDocument.Slot, width: Double, now: Date, rank: Int, withEntities: Bool = false) -> (item: PaperItem, fitted: Bool) {
        let f = fit(s.title, steps: slot.headline, maxLines: slot.maxLines, width: width)
        let dekStep = doc.steps["dek"] ?? .init(size: 16, weight: 400, lineHeight: 1.4)
        var show = false, dl = 0
        if slot.dek == true, !s.dek.isEmpty {
            let n = measurer.lines(s.dek, size: dekStep.size, weight: dekStep.weight, width: width)
            if n <= (slot.dekLines ?? 3) { show = true; dl = n }
        }
        var parts: [String] = []
        if slot.meta == "full" {
            if s.sources > 0 { parts.append("\(s.sources) sources") }
            if s.versions > 1 { parts.append("\(s.versions) versions") }
        } else if slot.meta == "short", s.sources > 0 { parts.append("\(s.sources) sources") }
        let rel = PaperText.relative(s.updated, now: now)
        if slot.meta != nil, !rel.isEmpty { parts.append(rel) }
        let ents = s.entities.prefix(3)
        let it = PaperItem(route: s.route, title: s.title, dek: show ? s.dek : "", section: s.section.isEmpty ? "News" : s.section,
                           meta: parts.joined(separator: " · "), byline: s.author,
                           entityLabels: withEntities || slot.meta == "full" ? ents.map(\.label).joined(separator: " · ") : "",
                           entityIds: s.entities.map(\.id).joined(separator: ","), size: f.step.size, weight: f.step.weight,
                           lineHeight: f.step.lineHeight, lines: f.lines, showDek: show, dekSize: dekStep.size, dekLines: dl, indent: false, rank: rank)
        return (it, f.fitted)
    }
}

extension String {
    /// Capture groups of the first match, or nil.
    func firstMatch(_ pattern: String) -> [String]? {
        guard let re = try? NSRegularExpression(pattern: pattern), let m = re.firstMatch(in: self, range: NSRange(startIndex..., in: self)) else { return nil }
        return (1..<m.numberOfRanges).map { i in Range(m.range(at: i), in: self).map { String(self[$0]) } ?? "" }
    }
}

// MARK: the whole page

public enum PaperComposer {
    /// Edition → the front page value the Contract reads. `section` is "" for the front, or a section slug.
    public static func front(stories: [PaperStory], section: String = "", width: Double, now: Date? = nil, edition: String = "",
                             engine: LayoutEngine = LayoutEngine(), status: String = "ready", message: String = "") -> PaperFront {
        let at = now ?? stories.compactMap(\.updated).max() ?? Date()
        let rankedAll = engine.rank(stories, now: at)
        var counts: [String: (label: String, n: Int)] = [:]
        var order: [String] = []
        for s in rankedAll {
            let label = s.section.isEmpty ? "News" : s.section
            let id = PaperText.slug(label)
            if counts[id] == nil { counts[id] = (label, 0); order.append(id) }
            counts[id]!.n += 1
        }
        let sections = order.map { PaperSection(id: $0, label: counts[$0]!.label, count: counts[$0]!.n, current: $0 == section) }
        let shown = section.isEmpty ? stories : stories.filter { PaperText.slug($0.section.isEmpty ? "News" : $0.section) == section }
        let bands = engine.compose(shown, width: width, now: at)
        let newest = shown.compactMap(\.updated).max()
        return PaperFront(status: shown.isEmpty ? "empty" : status, message: shown.isEmpty && message.isEmpty ? "Nothing on the front page yet." : message,
                          edition: edition, layout: engine.doc.name, composition: LayoutEngine.composition(bands),
                          dateLine: PaperText.dateLine(at), updated: newest.map { "Updated \(PaperText.relative($0, now: at))" } ?? "",
                          sectionId: section, sectionLabel: counts[section]?.label ?? "", count: shown.count, sections: sections, bands: bands)
    }
}
