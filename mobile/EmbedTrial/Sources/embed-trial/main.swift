import Foundation
import AutoradioKit
import EmbeddingGemma2CoreAI

// embed-trial: EmbeddingGemma 2 on the Mac, both routes. See docs/reference/EMBEDDINGGEMMA.md.
//   embed-trial bench --route a|b [--gpu 0|1] [--threads 4] [--compute ane|gpu|cpu] [--tag NAME] [--corpus DIR] [--book FILE]
//   embed-trial quality TAG [TAG ...]       recall@5 and MRR for BM25, vectors, fusion (needs bench output for each TAG)
//   embed-trial compare TAG TAG             cosine of the same texts across two routes
//   embed-trial scan [TAG]                  vector store timings at the corpus size and at 50,000 synthetic vectors
var args = Array(CommandLine.arguments.dropFirst())
let cmd = args.isEmpty ? "help" : args.removeFirst()
func opt(_ k: String, _ d: String) -> String { if let i = args.firstIndex(of: "--\(k)"), i + 1 < args.count { return args[i + 1] }; return d }
let here = FileManager.default.currentDirectoryPath
let corpusDir = URL(fileURLWithPath: opt("corpus", "\(here)/corpus"))
let bookURL = URL(fileURLWithPath: opt("book", cacheDir.appendingPathComponent("alice.txt").path))
let outDir = cacheDir.appendingPathComponent("out")
try? FileManager.default.createDirectory(at: outDir, withIntermediateDirectories: true)

func line(_ s: String) { print(s); fflush(stdout) }
func readVecs(_ tag: String, dim: Int = 768) throws -> [[Float]] {
    let d = try Data(contentsOf: outDir.appendingPathComponent("vec-\(tag).f32"))
    let a = d.withUnsafeBytes { Array($0.bindMemory(to: Float.self)) }
    return stride(from: 0, to: a.count, by: dim).map { Array(a[$0..<$0 + dim]) }
}

func bench() async throws {
    let route = opt("route", "a"), tag = opt("tag", route)
    var (docs, qs, _) = try Corpus.load(dir: corpusDir, book: bookURL)
    if let lim = Int(opt("limit", "0")), lim > 0, lim < docs.count { docs = Array(docs.prefix(lim)) }   // a bounded timing run; the vectors file then holds only these
    line("[\(tag)] corpus: \(docs.count) documents (\(docs.filter { $0.id.hasPrefix("alice") }.count) book passages), \(qs.count) questions")
    let e: TrialEmbedder
    var loadMs = 0.0, loadWarmMs: Double? = nil, prewarmS: Double? = nil
    let before = currentFootprintMB()
    if route == "a" {
        let path = cacheDir.appendingPathComponent("gguf/embeddinggemma-2-Q8_0.gguf").path
        let gpu = opt("gpu", "1") == "1", th = Int32(opt("threads", "4"))!
        let l = try LlamaEmbedder(path: path, gpu: gpu, threads: th)
        loadMs = l.loadMs; e = l
        line("[\(tag)] loaded GGUF in \(f1(loadMs)) ms (gpu \(gpu), threads \(th)); n_embd_out \(l.dim); pooling \(l.poolingName)")
    } else {
        let c: ComputePreference = ["ane": .neuralEngine, "gpu": .gpu, "cpu": .cpu][opt("compute", "ane")] ?? .neuralEngine
        let bundle = cacheDir.appendingPathComponent("coreai/model")
        let l = try await CoreAIEmbedder(bundle: bundle, compute: c)
        loadMs = l.loadMs; e = l
        line("[\(tag)] loaded Core AI (\(c.rawValue)) in \(f1(loadMs)) ms (this includes specialisation when the cache is cold)")
        let t0 = nowMs(); _ = try await l.prewarm(); prewarmS = (nowMs() - t0) / 1000
        line("[\(tag)] prewarm \(f1(prewarmS! * 1000)) ms")
    }
    let afterLoad = currentFootprintMB()
    // smoke: dimension, unit length, paraphrase pair above an unrelated pair (sentence-similarity prompt, from the card)
    let sp = "task: sentence similarity | query: "
    let s1 = try await e.embed(sp + "A man is playing a guitar on stage in front of a large crowd.").vec
    let s2 = try await e.embed(sp + "Someone performs music with a stringed instrument for an audience.").vec
    let s3 = try await e.embed(sp + "The quarterly tax filing deadline has been moved to the end of March.").vec
    line("[\(tag)] smoke: dim \(s1.count), norm \(String(format: "%.5f", norm(s1))), cos(paraphrase) \(String(format: "%.4f", dot(s1, s2))), cos(unrelated) \(String(format: "%.4f", dot(s1, s3))) -> \(dot(s1, s2) > dot(s1, s3) ? "PASS" : "FAIL")")
    // batch: every document, one at a time (the route's own pace), in corpus order
    var vecs: [[Float]] = []; var tokens = 0
    var perDoc: [Double] = []
    let t0 = nowMs()
    for d in docs {
        let t = nowMs()
        let r = try await e.embed(documentText(title: d.title, body: d.body))
        perDoc.append(nowMs() - t); tokens += r.tokens; vecs.append(r.vec)
    }
    let batchMs = nowMs() - t0
    var qv: [[Float]] = []; var qms: [Double] = []
    for q in qs { let t = nowMs(); qv.append(try await e.embed(queryPrefix + q.q).vec); qms.append(nowMs() - t) }
    // single query again, after the batch (warm), 30 repeats of the same question
    var rep: [Double] = []
    for _ in 0..<30 { let t = nowMs(); _ = try await e.embed(queryPrefix + qs[0].q); rep.append(nowMs() - t) }
    let all = vecs + qv
    var bytes = Data()
    for v in all { v.withUnsafeBytes { bytes.append(contentsOf: $0) } }
    try bytes.write(to: outDir.appendingPathComponent("vec-\(tag).f32"))
    let norms = all.map { norm($0) }
    let mem = peakMemoryMB()
    if route == "a", let l = e as? LlamaEmbedder {   // a second load in this process: the warm-cache load
        let g = opt("gpu", "1") == "1", th = Int32(opt("threads", "4"))!
        let t = nowMs(); let l2 = try LlamaEmbedder(path: cacheDir.appendingPathComponent("gguf/embeddinggemma-2-Q8_0.gguf").path, gpu: g, threads: th)
        loadWarmMs = nowMs() - t; _ = l; _ = l2
    }
    let n = Double(docs.count)
    let res: [String: Any] = [
        "tag": tag, "route": e.name, "docs": docs.count, "tokens": tokens, "batchSeconds": batchMs / 1000,
        "passagesPerSecond": n / (batchMs / 1000), "tokensPerSecond": Double(tokens) / (batchMs / 1000),
        "docMsP50": percentile(perDoc, 0.5), "docMsP95": percentile(perDoc, 0.95),
        "queryMsP50": percentile(rep, 0.5), "queryMsP95": percentile(rep, 0.95), "queryMsFirst": qms.first ?? 0,
        "loadMs": loadMs, "loadWarmMsSameProcess": loadWarmMs as Any, "prewarmSeconds": prewarmS as Any,
        "footprintBeforeLoadMB": before, "footprintAfterLoadMB": afterLoad, "peakFootprintMB": mem.footprint, "peakResidentMB": mem.resident,
        "normMin": norms.min() ?? 0, "normMax": norms.max() ?? 0, "rawNormMin": (e as? LlamaEmbedder)?.rawNormMin as Any, "rawNormMax": (e as? LlamaEmbedder)?.rawNormMax as Any, "dim": all.first?.count ?? 0,
        "os": ProcessInfo.processInfo.operatingSystemVersionString, "date": ISO8601DateFormatter().string(from: Date()),
    ]
    let j = try JSONSerialization.data(withJSONObject: res, options: [.prettyPrinted, .sortedKeys])
    try j.write(to: outDir.appendingPathComponent("bench-\(tag).json"))
    line(String(data: j, encoding: .utf8)!)
}

func quality() async throws {
    let (docs, qs, rel) = try Corpus.load(dir: corpusDir, book: bookURL)
    let idx = Corpus.index(docs)
    let ids = docs.map { $0.id }
    let baseN = 30
    let answerable = (0..<qs.count).filter { qs[$0].kind != "unanswerable" }
    let floorArg = Float(opt("floor", "0")) ?? 0
    func group(_ i: Int) -> String { i < baseN ? "base" : "extra" }
    var rows: [(String, [Int: [String]])] = []
    func record(_ name: String, _ rankings: [Int: [String]]) { rows.append((name, rankings)) }
    func query(_ i: Int) -> RetrievalQuery { RetrievalQuery(words: qs[i].q) }
    var bmFloor: [Int: [String]] = [:], bmRaw: [Int: [String]] = [:], exact: [Int: [String]] = [:]
    for i in 0..<qs.count {
        bmFloor[i] = idx.candidates(query(i), limit: 50).map(\.id)
        bmRaw[i] = idx.bm25Ranking(qs[i].q, limit: 50).map { $0.id }
        exact[i] = idx.exactTermHits(query(i), limit: 20)
    }
    record("BM25 as shipped (relevance floor)", bmFloor)
    record("BM25 no floor", bmRaw)
    var top1: [String: [Int: Float]] = [:]
    for tag in args where !tag.hasPrefix("--") && Float(tag) == nil {
        let v = try readVecs(tag); let dv = Array(v.prefix(docs.count)), qv = Array(v.suffix(qs.count))
        for dim in [768, 256] {
            let store = FlatStore(dim: dim); for x in dv { store.add(x) }
            var vr: [Int: [String]] = [:], t1: [Int: Float] = [:]
            for i in 0..<qs.count {
                let hits = store.search(normalized(qv[i], dim: dim), k: 50)
                t1[i] = hits.first?.score ?? 0
                vr[i] = hits.filter { $0.score >= floorArg }.map { ids[$0.row] }
            }
            if dim == 768 { top1[tag] = t1 }
            for s in FusionStrategy.allCases {
                record("\(s.rawValue) \(tag) \(dim)d", Dictionary(uniqueKeysWithValues: (0..<qs.count).map { ($0, HybridRank.fuse(s, vectors: vr[$0]!, bm25: bmFloor[$0]!, exact: exact[$0]!, limit: 20)) }))
            }
        }
    }
    func metrics(_ r: [Int: [String]], _ sel: [Int]) -> Metrics { var m = Metrics(); for i in sel { m.add(r[i]!, rel[i]) }; return m }
    let base = answerable.filter { $0 < baseN }, extra = answerable.filter { $0 >= baseN }
    line("method | base 30 | extra 20 | all 50 | exact+mixed names | paraphrase")
    let names = answerable.filter { qs[$0].kind == "exact" || qs[$0].kind == "mixed" }, para = answerable.filter { qs[$0].kind == "para" }
    for (n, r) in rows { line("\(n) | \(metrics(r, base).line) | \(metrics(r, extra).line) | \(metrics(r, answerable).line) | \(metrics(r, names).line) | \(metrics(r, para).line)") }
    // the floor: the best cosine of each question, answerable against not
    for (tag, t1) in top1 {
        let a = answerable.map { Double(t1[$0]!) }.sorted(), u = (0..<qs.count).filter { qs[$0].kind == "unanswerable" }.map { Double(t1[$0]!) }.sorted()
        func f(_ x: [Double]) -> String { x.isEmpty ? "-" : String(format: "min %.3f p5 %.3f median %.3f max %.3f", x.first!, percentile(x, 0.05), percentile(x, 0.5), x.last!) }
        line("\n[\(tag)] best cosine per question: answerable (\(a.count)) \(f(a)); unanswerable (\(u.count)) \(f(u))")
        for fl in [0.0, 0.6, 0.65, 0.7, 0.72, 0.74, 0.76, 0.78, 0.8] {
            let keepA = a.filter { $0 >= fl }.count, dropU = u.filter { $0 < fl }.count
            line(String(format: "   floor %.2f: answerable kept %d/%d, unanswerable refused %d/%d", fl, keepA, a.count, dropU, u.count))
        }
    }
    let un = (0..<qs.count).filter { qs[$0].kind == "unanswerable" }
    for (n, r) in rows where n.hasPrefix("BM25") || n.hasSuffix("768d") { line("\(n): unanswerable questions that still return something: \(un.filter { !r[$0]!.isEmpty }.count)/\(un.count)") }
    line("\nper question (first right rank; bm25 floored, then vectors-only 768 / rrf / exact-terms for the first vector tag):")
    let first = args.first { !$0.hasPrefix("--") && Float($0) == nil }
    for i in answerable {
        func rk(_ l: [String]?) -> String { l?.firstIndex(where: { rel[i].contains($0) }).map { "\($0 + 1)" } ?? "-" }
        var s = "\(i + 1). [\(qs[i].kind)] bm25 \(rk(bmFloor[i]))"
        if let t = first { for st in FusionStrategy.allCases { s += " \(st.rawValue.prefix(4)) \(rk(rows.first { $0.0 == "\(st.rawValue) \(t) 768d" }?.1[i]))" } }
        line(s + " | " + qs[i].q)
    }
}

func compare() throws {
    let a = try readVecs(args[0]), b = try readVecs(args[1])
    let c = zip(a, b).map { Double(dot($0, $1) / (norm($0) * norm($1))) }
    var c256: [Double] = zip(a, b).map { Double(dot(normalized($0, dim: 256), normalized($1, dim: 256))) }
    line("same texts, \(args[0]) vs \(args[1]): \(c.count) vectors, cosine min \(String(format: "%.5f", c.min()!)) mean \(String(format: "%.5f", c.reduce(0, +) / Double(c.count))) p5 \(String(format: "%.5f", percentile(c, 0.05)))")
    line("   at 256d: min \(String(format: "%.5f", c256.min()!)) mean \(String(format: "%.5f", c256.reduce(0, +) / Double(c256.count)))")
    c256.removeAll()
}

func scan() throws {
    let dimList = [768, 256]
    func random(_ n: Int, _ dim: Int) -> [[Float]] {
        var g = SystemRandomNumberGenerator()
        return (0..<n).map { _ in normalized((0..<768).map { _ in Float.random(in: -1...1, using: &g) }, dim: dim) }
    }
    for dim in dimList {
        for n in [330, 50_000] {
            let store = FlatStore(dim: dim)
            let vs = random(n, dim)
            for v in vs { store.add(v) }
            let file = outDir.appendingPathComponent("scan-\(dim)-\(n).f16"); try store.write(to: file)
            let mapped = try FlatStore.open(file, dim: dim)
            let q = vs[n / 2]
            var t16: [Double] = [], t32: [Double] = []
            _ = mapped.search(q, k: 50); _ = mapped.searchF32(q, k: 50)
            for _ in 0..<30 { var t = nowMs(); _ = mapped.search(q, k: 50); t16.append(nowMs() - t); t = nowMs(); _ = mapped.searchF32(q, k: 50); t32.append(nowMs() - t) }
            let top = mapped.search(q, k: 1).first
            line("dim \(dim), \(n) vectors, \(mapped.bytes / 1024) KiB f16 file: scan f16->f32 blocks p50 \(String(format: "%.3f", percentile(t16, 0.5))) ms p95 \(String(format: "%.3f", percentile(t16, 0.95))) ms | f32 in memory p50 \(String(format: "%.3f", percentile(t32, 0.5))) ms | self-match row \(top?.row ?? -1) (expect \(n / 2)) score \(String(format: "%.4f", top?.score ?? 0))")
        }
    }
}

switch cmd {
case "bench": try await bench()
case "quality": try await quality()
case "compare": try compare()
case "scan": try scan()
default: line("usage: embed-trial bench|quality|compare|scan (see the header of main.swift)")
}
