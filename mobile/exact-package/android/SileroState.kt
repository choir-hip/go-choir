package choir.autoradio

/** Silero v6.2.1 op15's exact 16 kHz state shape, independent of ONNX Runtime for JVM tests. */
internal object SileroSpec {
    const val SAMPLE_RATE = 16_000
    const val WINDOW_SAMPLES = 512
    const val CONTEXT_SAMPLES = 64
    const val STATE_ROWS = 2
    const val STATE_BATCH = 1
    const val STATE_WIDTH = 128
    const val INPUT_SAMPLES = CONTEXT_SAMPLES + WINDOW_SAMPLES
}

/**
 * Upstream `OnnxWrapper` state: `input` is the previous 64 samples plus the new
 * 512, `state` is [2,1,128], and the next context is the input's last 64.
 */
internal class SileroState {
    private var context = FloatArray(SileroSpec.CONTEXT_SAMPLES)
    private var state = Array(SileroSpec.STATE_ROWS) { Array(SileroSpec.STATE_BATCH) { FloatArray(SileroSpec.STATE_WIDTH) } }
    private var pending: FloatArray? = null

    fun prepare(chunk: FloatArray): FloatArray {
        require(chunk.size == SileroSpec.WINDOW_SAMPLES) { "Silero needs exactly 512 samples at 16 kHz" }
        require(chunk.all { it.isFinite() }) { "Silero samples must be finite" }
        val input = FloatArray(SileroSpec.INPUT_SAMPLES)
        context.copyInto(input, 0)
        chunk.copyInto(input, SileroSpec.CONTEXT_SAMPLES)
        pending = chunk.copyOf()
        return input
    }

    fun accept(nextState: Array<Array<FloatArray>>) {
        val chunk = pending ?: throw IllegalStateException("prepare a window before accepting Silero state")
        require(nextState.size == SileroSpec.STATE_ROWS) { "Silero state must have two rows" }
        require(nextState.all { row -> row.size == SileroSpec.STATE_BATCH && row[0].size == SileroSpec.STATE_WIDTH }) {
            "Silero state must have shape [2,1,128]"
        }
        require(nextState.all { row -> row.all { values -> values.all { it.isFinite() } } }) { "Silero state must be finite" }
        state = Array(SileroSpec.STATE_ROWS) { row -> Array(SileroSpec.STATE_BATCH) { nextState[row][0].copyOf() } }
        chunk.copyInto(context, 0, SileroSpec.WINDOW_SAMPLES - SileroSpec.CONTEXT_SAMPLES, SileroSpec.WINDOW_SAMPLES)
        pending = null
    }

    fun snapshotState(): Array<Array<FloatArray>> =
        Array(SileroSpec.STATE_ROWS) { row -> Array(SileroSpec.STATE_BATCH) { state[row][0].copyOf() } }

    fun reset() {
        context = FloatArray(SileroSpec.CONTEXT_SAMPLES)
        state = Array(SileroSpec.STATE_ROWS) { Array(SileroSpec.STATE_BATCH) { FloatArray(SileroSpec.STATE_WIDTH) } }
        pending = null
    }
}
