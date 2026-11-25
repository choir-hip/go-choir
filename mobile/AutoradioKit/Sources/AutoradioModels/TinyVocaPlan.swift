// Where Core ML runs each op of a model function (MLComputePlan, macOS 14.4+).

import CoreML
import Foundation

public struct PlanSummary: Sendable {
    public var function: String
    /// Compute ops (const ops are not counted) and those whose preferred device is the Neural Engine.
    public var ops = 0, neuralEngineOps = 0
    /// Estimated cost of all ops and of those on the Neural Engine.
    public var cost = 0.0, neuralEngineCost = 0.0
    /// Ops elsewhere, by "operator@device".
    public var elsewhere: [String: Int] = [:]
}

extension TinyVoca {
    /// The compute plan of one function of a compiled model for `units`.
    public static func computePlan(_ url: URL, function: String?, units: MLComputeUnits) async throws -> PlanSummary {
        let configuration = MLModelConfiguration()
        configuration.computeUnits = units
        configuration.functionName = function
        let plan = try await MLComputePlan.load(contentsOf: url, configuration: configuration)
        guard case .program(let program) = plan.modelStructure else { throw TinyVocaError.badInput("\(url.lastPathComponent) is not an ML program") }
        let name = function ?? "main"
        guard let body = program.functions[name] ?? (program.functions.count == 1 ? program.functions.values.first : nil) else {
            throw TinyVocaError.badInput("no function \(name) in \(url.lastPathComponent)")
        }
        var summary = PlanSummary(function: name)
        func walk(_ block: MLModelStructure.Program.Block) {
            for operation in block.operations {
                operation.blocks.forEach(walk)
                if operation.operatorName == "const" { continue }
                let weight = plan.estimatedCost(of: operation)?.weight ?? 0
                summary.ops += 1
                summary.cost += weight
                switch plan.deviceUsage(for: operation)?.preferred {
                case .neuralEngine?:
                    summary.neuralEngineOps += 1
                    summary.neuralEngineCost += weight
                case .cpu?: summary.elsewhere["\(operation.operatorName)@CPU", default: 0] += 1
                case .gpu?: summary.elsewhere["\(operation.operatorName)@GPU", default: 0] += 1
                default: summary.elsewhere["\(operation.operatorName)@unplanned", default: 0] += 1
                }
            }
        }
        walk(body.block)
        return summary
    }
}
