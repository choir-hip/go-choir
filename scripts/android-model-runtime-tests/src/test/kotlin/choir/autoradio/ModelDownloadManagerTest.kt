package choir.autoradio

import com.sun.net.httpserver.HttpServer
import java.io.File
import java.net.InetSocketAddress
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Files
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class ModelDownloadManagerTest {
    @get:Rule val temporary = TemporaryFolder()

    @Test fun wifiRefusalMakesNoRequestAndLeavesNoFile() {
        val root = temporary.newFolder("models")
        var opened = 0
        val manager = ModelDownloadManager(root, { false }, { opened++; error("must not open") })
        val error = assertThrows(ModelDownloadException::class.java) {
            manager.download("silero-vad-v6.2.1")
        }
        assertEquals("WIFI_REQUIRED", error.code)
        assertEquals(0, opened)
        assertFalse(File(root, "silero-vad-v6.2.1").exists())
    }

    @Test fun verifiedDownloadIsAtomicAndReadinessRechecksThePin() {
        val root = temporary.newFolder("models")
        val payload = "not the pinned model".toByteArray()
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/") { exchange ->
            exchange.sendResponseHeaders(200, payload.size.toLong())
            exchange.responseBody.use { it.write(payload) }
        }
        server.start()
        try {
            val local = URL("http://127.0.0.1:${server.address.port}/model")
            val manager = ModelDownloadManager(root, { true }, { local.openConnection() as HttpURLConnection })
            val error = assertThrows(ModelDownloadException::class.java) {
                manager.download("silero-vad-v6.2.1")
            }
            assertEquals("SIZE_MISMATCH", error.code)
            val directory = File(root, "silero-vad-v6.2.1")
            assertFalse(directory.listFiles()?.any { it.name.endsWith(".part") } == true)
            assertFalse(directory.listFiles()?.any { it.name == "silero_vad_16k_op15.onnx" } == true)
        } finally {
            server.stop(0)
        }
    }

    @Test fun hashMismatchIsNotReady() {
        val root = temporary.newFolder("models")
        val file = File(root, "model.gguf").apply { writeText("wrong") }
        assertFalse(ModelDownloadManager.verifyFile(file, "0".repeat(64), 5))
        assertTrue(ModelDownloadManager.verifyFile(file, ModelDownloadManager.sha256(file), 5))
    }

    @Test fun modelPathCannotEscapeThroughIdOrSymlink() {
        val root = temporary.newFolder("models").canonicalFile
        for (id in listOf("../outside", "/outside", "a/b", "a\\b", ".", "..")) {
            assertThrows(IllegalArgumentException::class.java) { safeModelDirectory(root, id) }
        }
        val outside = temporary.newFolder("outside").canonicalFile
        Files.createSymbolicLink(File(root, "escape").toPath(), outside.toPath())
        assertThrows(IllegalArgumentException::class.java) { safeModelDirectory(root, "escape") }
    }

    @Test fun removeDeletesOnlyTheNamedModelDirectory() {
        val root = temporary.newFolder("models")
        val selected = safeModelDirectory(root, "silero-vad-v6.2.1")
        selected.mkdirs()
        File(selected, "model.onnx").writeText("model")
        val other = safeModelDirectory(root, "tiny-voca")
        other.mkdirs()
        File(other, "keep").writeText("keep")
        ModelDownloadManager(root, { true }).remove("silero-vad-v6.2.1")
        assertFalse(selected.exists())
        assertTrue(File(other, "keep").exists())
    }

    @Test fun stateWindowKeepsExactlyTheLast64SamplesAndCopiesReturnedState() {
        val state = SileroState()
        val first = FloatArray(512) { it.toFloat() }
        assertEquals(576, state.prepare(first).size)
        assertTrue(state.prepare(first).copyOfRange(0, 64).all { it == 0f })
        val returned = Array(2) { arrayOf(FloatArray(128) { 1f }) }
        state.accept(returned)
        returned[0][0][0] = 99f
        assertEquals(1f, state.snapshotState()[0][0][0])
        val second = FloatArray(512) { (it + 1000).toFloat() }
        val input = state.prepare(second)
        assertTrue(input.copyOfRange(0, 64).contentEquals(first.copyOfRange(448, 512)))
        state.reset()
        assertTrue(state.snapshotState().all { row -> row.all { it.all { value -> value == 0f } } })
        assertTrue(state.prepare(first).copyOfRange(0, 64).all { it == 0f })
    }

    @Test fun stateRejectsWrongWindowsAndShapesWithoutChangingContext() {
        val state = SileroState()
        for (size in listOf(0, 511, 513)) {
            assertThrows(IllegalArgumentException::class.java) { state.prepare(FloatArray(size)) }
        }
        assertThrows(IllegalArgumentException::class.java) { state.prepare(FloatArray(512) { Float.NaN }) }
        state.prepare(FloatArray(512) { 3f })
        assertThrows(IllegalArgumentException::class.java) { state.accept(Array(2) { arrayOf(FloatArray(127)) }) }
        assertEquals(0f, state.prepare(FloatArray(512))[0])
    }
}
