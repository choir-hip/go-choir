package choir.autoradio

import ai.onnxruntime.OrtEnvironment
import ai.onnxruntime.OrtException
import ai.onnxruntime.OrtSession
import ai.onnxruntime.OnnxTensor
import java.io.File
import java.nio.FloatBuffer
import java.nio.LongBuffer

/** Exact Silero v6.2.1 op15 ONNX execution: CPU, 16 kHz, 512-sample windows. */
class SileroVad private constructor(
    private val environment: OrtEnvironment,
    private val session: OrtSession,
) : AutoCloseable {
    private val state = SileroState()
    private val lock = Any()
    private var closed = false

    fun probability(chunk: FloatArray): Float = synchronized(lock) {
        if (closed) throw IllegalStateException("Silero session is closed")
        val input = state.prepare(chunk)
        val recurrent = state.snapshotState()
        val flat = FloatArray(SileroSpec.STATE_ROWS * SileroSpec.STATE_BATCH * SileroSpec.STATE_WIDTH)
        for (row in 0 until SileroSpec.STATE_ROWS) for (batch in 0 until SileroSpec.STATE_BATCH) {
            recurrent[row][batch].copyInto(flat, (row * SileroSpec.STATE_BATCH + batch) * SileroSpec.STATE_WIDTH)
        }
        // op15's `sr` is a scalar (rank 0); the input and state keep their declared ranks. Each
        // tensor is closed even when a later one cannot be made.
        var inputTensor: OnnxTensor? = null
        var stateTensor: OnnxTensor? = null
        var rateTensor: OnnxTensor? = null
        try {
            inputTensor = OnnxTensor.createTensor(environment, FloatBuffer.wrap(input), longArrayOf(1, SileroSpec.INPUT_SAMPLES.toLong()))
            stateTensor = OnnxTensor.createTensor(environment, FloatBuffer.wrap(flat), longArrayOf(2, 1, SileroSpec.STATE_WIDTH.toLong()))
            rateTensor = OnnxTensor.createTensor(environment, LongBuffer.wrap(longArrayOf(SileroSpec.SAMPLE_RATE.toLong())), longArrayOf())
            session.run(mapOf("input" to inputTensor, "state" to stateTensor, "sr" to rateTensor)).use { result ->
                val output = result[0].value as Array<FloatArray>
                val nextState = result[1].value as Array<Array<FloatArray>>
                require(output.size == 1 && output[0].size == 1) { "Silero output must have shape [1,1]" }
                val probability = output[0][0]
                require(probability.isFinite() && probability in 0f..1f) { "Silero returned an invalid probability" }
                state.accept(nextState)
                probability
            }
        } catch (error: OrtException) {
            throw IllegalStateException("Silero ONNX inference failed", error)
        } finally {
            inputTensor?.close()
            stateTensor?.close()
            rateTensor?.close()
        }
    }

    fun reset() = synchronized(lock) { state.reset() }

    override fun close() = synchronized(lock) {
        if (!closed) {
            closed = true
            session.close()
        }
    }

    companion object {
        fun load(path: File): SileroVad {
            if (!path.isFile) throw IllegalArgumentException("Silero model file is missing")
            val environment = OrtEnvironment.getEnvironment()
            val options = OrtSession.SessionOptions()
            return try {
                options.setIntraOpNumThreads(1)
                options.setInterOpNumThreads(1)
                SileroVad(environment, environment.createSession(path.absolutePath, options))
            } catch (error: OrtException) {
                throw IllegalStateException("Silero ONNX session could not be created", error)
            } finally {
                options.close()
            }
        }
    }
}

data class VadEvent(val kind: Kind, val sample: Long) {
    enum class Kind { START, END }
}

/** Upstream VADIterator's hysteresis and sample coordinates, without resampling. */
class SileroVadIterator(
    private val vad: SileroVad,
    private val threshold: Float = 0.5f,
    minSilenceMs: Int = 100,
    speechPadMs: Int = 30,
) {
    init {
        require(threshold.isFinite() && threshold in 0f..1f) { "Silero threshold must be between zero and one" }
        require(minSilenceMs >= 0 && minSilenceMs <= 600_000 && speechPadMs >= 0 && speechPadMs <= 600_000) {
            "Silero timing must be between zero and ten minutes"
        }
    }

    private val minSilenceSamples = (SileroSpec.SAMPLE_RATE.toLong() * minSilenceMs / 1000).toInt()
    private val speechPadSamples = (SileroSpec.SAMPLE_RATE.toLong() * speechPadMs / 1000).toInt()
    private val lock = Any()
    private var triggered = false
    private var tempEnd = 0L
    private var currentSample = 0L

    fun next(chunk: FloatArray): VadEvent? = synchronized(lock) {
        require(chunk.size == SileroSpec.WINDOW_SAMPLES) { "Silero iterator needs exactly 512 samples at 16 kHz" }
        val probability = vad.probability(chunk)
        currentSample += SileroSpec.WINDOW_SAMPLES
        if (probability >= threshold && tempEnd != 0L) tempEnd = 0
        if (probability >= threshold && !triggered) {
            triggered = true
            return@synchronized VadEvent(VadEvent.Kind.START, maxOf(0, currentSample - speechPadSamples - SileroSpec.WINDOW_SAMPLES))
        }
        if (probability < threshold - 0.15f && triggered) {
            if (tempEnd == 0L) tempEnd = currentSample
            if (currentSample - tempEnd < minSilenceSamples) return@synchronized null
            val end = tempEnd + speechPadSamples - SileroSpec.WINDOW_SAMPLES
            tempEnd = 0
            triggered = false
            return@synchronized VadEvent(VadEvent.Kind.END, end)
        }
        null
    }

    fun reset() = synchronized(lock) {
        vad.reset()
        triggered = false
        tempEnd = 0
        currentSample = 0
    }
}
