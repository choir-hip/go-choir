import Foundation
import AutoradioKit
#if canImport(Darwin)
import Darwin
#endif
#if canImport(os)
import os
#endif

/// The device measurement of EmbeddingGemma 2 (docs/reference/EMBEDDINGGEMMA.md), run once at the lead's request and never otherwise:
/// launch the app with `-EmbedTrial 1` (devicectl `--arguments`, like `-ModelProofStages`); it writes Documents/proof/embed-trial.json and
/// logs `autoradio embed-trial: ...` lines to stderr. It downloads the 310 MB GGUF to the model store first when it is not there.
/// Route A (llama.cpp, the official GGUF) runs on any iOS the app supports. Route B (Core AI) is not built into the app (the framework is
/// iOS 27 only); the file records what the OS has so the verdict row can be filled.
public enum EmbedDeviceTrial {
    struct Report: Codable {
        var date = "", device = "", os = "", lowPowerMode = false
        var thermalBefore = "", thermalAfter = ""
        var routeA = RouteA(), routeB = RouteB()
        var error: String?
    }
    struct RouteA: Codable {
        var ran = false, modelLoaded = false
        var error: String?
        var engine = "", modelBytes: Int64 = 0, downloadSeconds: Double = 0
        var loadMsFirst = 0.0, loadMsWarm = 0.0
        var availableMBBefore = 0.0, availableMBAfterLoad = 0.0, availableMBAfterBatch = 0.0
        var footprintMBAfterLoad = 0.0, peakFootprintMB = 0.0
        var dimension = 0, rawNormMin = 0.0, rawNormMax = 0.0
        var cosParaphrase = 0.0, cosUnrelated = 0.0, smokePass = false
        var passages = 0, tokens = 0, passagesPerSecond = 0.0, tokensPerSecond = 0.0
        var queryMsP50 = 0.0, queryMsP95 = 0.0
        var recallAt5 = ["bm25": 0.0, "vector768": 0.0, "vector256": 0.0, "fused768": 0.0], mrr = ["bm25": 0.0, "vector768": 0.0, "vector256": 0.0, "fused768": 0.0]
        var questions = 0
        var scan50kMsP50_768 = 0.0, scan50kMsP95_768 = 0.0, scan50kMsP50_256 = 0.0
        var jobResume = ""
    }
    struct RouteB: Codable { var builtIn = false, osHasCoreAIFramework = false, note = "" }

    #if os(iOS)
    static func availableMB() -> Double { Double(os_proc_available_memory()) / 1_048_576 }
    #else
    static func availableMB() -> Double { 0 }
    #endif
    static func footprint() -> (now: Double, peak: Double) {
        var info = task_vm_info_data_t()
        var count = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
        let kr = withUnsafeMutablePointer(to: &info) { $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &count) } }
        guard kr == KERN_SUCCESS else { return (0, 0) }
        return (Double(info.phys_footprint) / 1_048_576, Double(info.ledger_phys_footprint_peak) / 1_048_576)
    }
    static func thermal() -> String {
        switch ProcessInfo.processInfo.thermalState { case .nominal: return "nominal"; case .fair: return "fair"; case .serious: return "serious"; case .critical: return "critical"; @unknown default: return "unknown" }
    }
    static func pct(_ xs: [Double], _ p: Double) -> Double { let s = xs.sorted(); return s.isEmpty ? 0 : s[min(s.count - 1, Int(Double(s.count) * p))] }

    /// `-EmbedTrial 1` on the launch line. Does nothing otherwise.
    static func runFromLaunchArguments() {
        guard UserDefaults.standard.bool(forKey: "EmbedTrial") else { return }
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("proof", isDirectory: true)
        Task.detached {
            func log(_ s: String) { fputs("autoradio embed-trial: \(s)\n", stderr); fflush(stderr) }
            var r = Report()
            r.date = ISO8601DateFormatter().string(from: Date())
            r.device = ModelProof.machine
            r.os = ProcessInfo.processInfo.operatingSystemVersionString
            r.lowPowerMode = ProcessInfo.processInfo.isLowPowerModeEnabled
            r.thermalBefore = thermal()
            log("start on \(r.device), \(r.os), thermal \(r.thermalBefore), low power \(r.lowPowerMode)")
            r.routeB.osHasCoreAIFramework = dlopen("/System/Library/Frameworks/CoreAI.framework/CoreAI", RTLD_LAZY) != nil
            r.routeB.note = "Core AI needs iOS 27; this app does not link it. The OS here \(r.routeB.osHasCoreAIFramework ? "has" : "does not have") the framework."
            #if canImport(llama)
            do { r.routeA = try await runRouteA(log: log) } catch { r.routeA.error = "\(error)"; log("route A failed: \(error)") }
            #else
            r.routeA.error = "this build has no llama.cpp framework"
            #endif
            r.thermalAfter = thermal()
            do {
                try FileManager.default.createDirectory(at: docs, withIntermediateDirectories: true)
                let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
                try e.encode(r).write(to: docs.appendingPathComponent("embed-trial.json"), options: .atomic)
                log("done: wrote embed-trial.json (route A \(r.routeA.modelLoaded ? "loaded" : "did not load"))")
            } catch { log("could not write embed-trial.json: \(error)") }
        }
    }

    #if canImport(llama)
    static func runRouteA(log: (String) -> Void) async throws -> RouteA {
        var a = RouteA(); a.ran = true
        let spec = KnownModels.embeddingGemma2
        a.engine = KnownModels.embeddingEngine; a.modelBytes = spec.bytes
        a.availableMBBefore = availableMB()
        let d0 = nowMs()
        let have = await ModelStore.shared.localFileIfPresent(spec.id) != nil
        _ = try await ModelStore.shared.ensure(spec.id)
        a.downloadSeconds = have ? 0 : (nowMs() - d0) / 1000
        log("model file \(have ? "already on the phone" : "downloaded in \(Int(a.downloadSeconds)) s"), available \(Int(a.availableMBBefore)) MB")
        let l0 = nowMs()
        let e = try await LlamaEmbedder.shared(download: false)
        a.loadMsFirst = nowMs() - l0; a.modelLoaded = true
        a.availableMBAfterLoad = availableMB(); a.footprintMBAfterLoad = footprint().now
        log("loaded in \(Int(a.loadMsFirst)) ms, footprint \(Int(a.footprintMBAfterLoad)) MB, available \(Int(a.availableMBAfterLoad)) MB")
        // a second load of the same file (weights cached by the system, contexts new)
        do { let t = nowMs(); let w = try LlamaWeights.shared(path: try await LlamaFiles.url(spec.id, download: false).path, gpu: true); _ = try LlamaEmbedder(weights: w, threads: 4); a.loadMsWarm = nowMs() - t }
        // smoke
        let sp = "task: sentence similarity | query: "
        let v = try await e.embed([sp + "A man is playing a guitar on stage in front of a large crowd.", sp + "Someone performs music with a stringed instrument for an audience.", sp + "The quarterly tax filing deadline has been moved to the end of March."])
        func dot(_ x: [Float], _ y: [Float]) -> Double { Double(zip(x, y).reduce(0) { $0 + $1.0 * $1.1 }) }
        a.dimension = v[0].count; a.cosParaphrase = dot(v[0], v[1]); a.cosUnrelated = dot(v[0], v[2])
        a.smokePass = a.dimension == 768 && abs(dot(v[0], v[0]) - 1) < 1e-3 && a.cosParaphrase > a.cosUnrelated
        a.rawNormMin = Double(e.rawNormRange.min); a.rawNormMax = Double(e.rawNormRange.max)
        log("smoke dim \(a.dimension), paraphrase \(String(format: "%.3f", a.cosParaphrase)) vs unrelated \(String(format: "%.3f", a.cosUnrelated)) \(a.smokePass ? "PASS" : "FAIL")")
        // batch: the fixture stories, one passage at a time, as the app's embedder is called. (Retrieval quality and the vector scan are measured in
        // AutoradioLab/retrieval-run.test.ts and search.test.ts now: the index, the vectors and the job are shared code.)
        let rows = EmbedTrialFixture.rows
        var tokens = 0
        let b0 = nowMs()
        for r in rows { tokens += try e.embedOne(EmbedPrompt.document(title: r.title, text: r.body)).tokens }
        let batchMs = nowMs() - b0
        a.passages = rows.count; a.tokens = tokens
        a.passagesPerSecond = Double(rows.count) / (batchMs / 1000); a.tokensPerSecond = Double(tokens) / (batchMs / 1000)
        a.availableMBAfterBatch = availableMB(); a.peakFootprintMB = footprint().peak
        var qms: [Double] = []
        for r in rows { let t = nowMs(); _ = try e.embedOne(EmbedPrompt.query(r.question)); qms.append(nowMs() - t) }
        a.questions = rows.count
        a.queryMsP50 = pct(qms, 0.5); a.queryMsP95 = pct(qms, 0.95)
        log("batch \(rows.count) passages, \(String(format: "%.1f", a.passagesPerSecond))/s, \(Int(a.tokensPerSecond)) tokens/s; query p50 \(Int(a.queryMsP50)) ms")
        return a
    }
    #endif
}
