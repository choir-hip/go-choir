package choir.autoradio

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class NativeModelCatalogTest {
    @Test fun idsAndPinsMatchSharedIosCatalog() {
        assertEquals(
            listOf("qwen3-0.6b-q4_k_m", "qwen3-0.6b-q8_0", "embeddinggemma-2-q8_0", "silero-vad-v6.2.1", "tiny-voca"),
            NativeModelCatalog.models.map { it.id },
        )
        assertEquals(396_705_472L, NativeModelCatalog.require("qwen3-0.6b-q4_k_m").bytes)
        assertEquals(639_446_688L, NativeModelCatalog.require("qwen3-0.6b-q8_0").bytes)
        assertEquals(768, NativeModelCatalog.require("embeddinggemma-2-q8_0").embeddingDimension)
        assertEquals(
            "7ed98ddbad84ccac4cd0aeb3099049280713df825c610a8ed34543318f1b2c49",
            NativeModelCatalog.require("silero-vad-v6.2.1").files.single().sha256,
        )
        assertEquals(listOf("F2", "M2"), NativeModelCatalog.require("tiny-voca").supportedVoices)
    }

    @Test fun unknownOrPathLikeIdIsRefused() {
        for (id in listOf("missing", "../qwen3-0.6b-q8_0", "/qwen3-0.6b-q8_0", "qwen3-0.6b-q8_0/../../other")) {
            assertThrows(IllegalArgumentException::class.java) { NativeModelCatalog.require(id) }
        }
    }
}
