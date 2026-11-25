// MLMultiArray helpers: raw buffers in and out, no NSNumber per element.
// Outputs are read through their strides: the Neural Engine pads rows of its outputs.

import CoreML

enum Arrays {
    /// A zeroed dense array; `fill` writes it row-major.
    static func make<T>(_ shape: [Int], _ type: MLMultiArrayDataType, as _: T.Type,
                        _ fill: (UnsafeMutableBufferPointer<T>) -> Void) throws -> MLMultiArray {
        let array = try MLMultiArray(shape: shape.map { NSNumber(value: $0) }, dataType: type)
        array.withUnsafeMutableBytes { raw, strides in
            precondition(strides == dense(shape), "MLMultiArray is not dense")
            raw.initializeMemory(as: UInt8.self, repeating: 0)
            fill(raw.bindMemory(to: T.self))
        }
        return array
    }

    static func half(_ shape: [Int], _ fill: (UnsafeMutableBufferPointer<Float16>) -> Void = { _ in }) throws -> MLMultiArray {
        try make(shape, .float16, as: Float16.self, fill)
    }

    static func float(_ shape: [Int], _ values: [Float]) throws -> MLMultiArray {
        try make(shape, .float32, as: Float.self) { buffer in
            values.withUnsafeBufferPointer { buffer.baseAddress!.update(from: $0.baseAddress!, count: values.count) }
        }
    }

    static func dense(_ shape: [Int]) -> [Int] {
        var strides = [Int](repeating: 1, count: shape.count)
        for i in stride(from: shape.count - 2, through: 0, by: -1) { strides[i] = strides[i + 1] * shape[i + 1] }
        return strides
    }

    /// The array viewed as rows (all axes but the last) x columns, the first `length` columns of
    /// every row, row-major as Float.
    static func rows(_ array: MLMultiArray, length: Int? = nil) -> [Float] {
        let shape = array.shape.map(\.intValue), strides = array.strides.map(\.intValue)
        let columns = length ?? shape.last!, step = strides.last!
        precondition(columns <= shape.last!)
        var offsets = [0]
        for axis in 0..<(shape.count - 1) {
            offsets = offsets.flatMap { base in (0..<shape[axis]).map { base + $0 * strides[axis] } }
        }
        var output = [Float](repeating: 0, count: offsets.count * columns)
        output.withUnsafeMutableBufferPointer { out in
            func copy<T: MLShapedArrayScalar>(_: T.Type, _ convert: (T) -> Float) {
                array.withUnsafeBufferPointer(ofType: T.self) { input in
                    for (row, offset) in offsets.enumerated() {
                        let target = out.baseAddress! + row * columns
                        for j in 0..<columns { target[j] = convert(input[offset + j * step]) }
                    }
                }
            }
            switch array.dataType {
            case .float16: copy(Float16.self) { Float($0) }
            case .float32: copy(Float.self) { $0 }
            case .double: copy(Double.self) { Float($0) }
            default: preconditionFailure("unexpected output type \(array.dataType)")
            }
        }
        return output
    }
}
