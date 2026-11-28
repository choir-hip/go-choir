package choir.autoradio

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** The place the deck reports (docs/design/sentence-follow-in-shared-code-2026-11-28.md): the digest, each unit's range in the received text, and the place of the chunk sounding. No player: the pure parts. */
class DeckPlaceTest {
    private val story = "First sentence here. Second one follows it.\n\n  A second paragraph\nwraps over two lines. It ends here.  \n\n\n\n" + "A long paragraph sentence that goes on and on. ".repeat(12).trim()

    @Test fun digestMatchesTheSharedCodeVectors() {
        assertEquals("0:811c9dc5", DeckDigest.of(""))
        assertEquals("1:e40c292c", DeckDigest.of("a"))
        assertEquals("6:bf9cf968", DeckDigest.of("foobar"))
        assertEquals("7:f1b38d91", DeckDigest.of("café 😀"))     // an emoji is two UTF-16 code units, which is what a Kotlin String holds
    }

    @Test fun everyPartsRangeIsItsTextInTheSource() {
        val parts = SpeechRanges.parts(story, 0, 100)
        assertTrue(parts.size > 4)
        var last = 0
        for (p in parts) {
            assertEquals(p.text, story.substring(p.start, p.start + p.length))
            assertTrue("in order", p.start >= last); last = p.start + p.length
            assertTrue(p.text.length <= 100)
            assertEquals(p.text, p.text.trim())
        }
        assertEquals(setOf(0, 1, 2), parts.map { it.block }.toSet())
    }

    @Test fun theSameWordsAsTheItemsParagraphListAreSpoken() {
        val item = DeckItem("e", "speech", paragraphs = story)
        assertEquals(item.paragraphList, SpeechRanges.parts(story, 0, 100000).map { it.text })
    }

    @Test fun startingAtABlockSkipsEarlierParagraphsButKeepsAbsoluteRanges() {
        val all = SpeechRanges.parts(story, 0, 100000)
        assertEquals(all.drop(1), SpeechRanges.parts(story, 1, 100000))
    }

    @Test fun aRepeatedParagraphIsRangedAtItsOwnPlace() {
        val src = "Same words here.\n\nOther words entirely.\n\nSame words here."
        assertEquals(listOf(0, 18, 41), SpeechRanges.parts(src, 0, 100).map { it.start })
    }

    @Test fun wideCharactersCountInUtf16() {
        val src = "Café 😀 first.\n\nSecond one."
        val r = SpeechRanges.parts(src, 0, 100)
        assertEquals("Café 😀 first.".length, r[0].length)
        assertEquals(src.length - "Second one.".length, r[1].start)
    }

    @Test fun anEmptyOrBlankTextHasNoParts() {
        assertTrue(SpeechRanges.parts("", 0, 100).isEmpty())
        assertTrue(SpeechRanges.parts("  \n\n \n\n", 0, 100).isEmpty())
    }

    private val parts = listOf(SpeechPart(0, "One.", 0, 4), SpeechPart(1, "Two words.", 6, 10))

    @Test fun f22ANegativeDurationIsZeroAndTheRestIsReported() {
        assertEquals(DeckPlace("e1", "d", 6, 10, 2.5, 0.0), placeOf("e1", "d", parts, 1, 2500, -9223372036854775807L))       // C.TIME_UNSET
        assertEquals(0.0, placeOf("e1", "d", parts, 0, 100, 0).total, 0.0)
        assertEquals(1.5, placeOf("e1", "d", parts, 0, 100, 1500).total, 0.0)
    }

    @Test fun f22NoChunksYetIsNoPlaceAndAnIndexPastTheEndIsBounded() {
        assertEquals(DeckPlace(), placeOf("e1", "d", emptyList(), 0, 100, 1000))
        assertEquals(6, placeOf("e1", "d", parts, 9, 0, 1000).start)
        assertEquals(0, placeOf("e1", "d", parts, -3, 0, 1000).start)
        assertEquals(DeckPlace(), placeOf("", "d", parts, 0, 0, 0))
        assertEquals(0.0, placeOf("e1", "d", parts, 0, -50, 1000).seconds, 0.0)
    }

    @Test fun theSnapshotCarriesTheEnginesPlaceOnlyForTheItemOnAir() {
        val engine = PlaceEngine()
        val scope = kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.SupervisorJob() + kotlinx.coroutines.Dispatchers.Unconfined)
        val deck = DeckController(DeckJournal(), engine, object : DeckScheduler { override fun schedule(delayMs: Long, action: () -> Unit): () -> Unit = {} }, scope)
        kotlinx.coroutines.runBlocking { deck.play(DeckItem("a", "speech", paragraphs = "One.\n\nTwo.")) }
        engine.place = DeckPlace("a", "d", 0, 4, 1.0, 2.0)
        assertEquals(engine.place, deck.snapshot().place)
        engine.place = DeckPlace("other", "d", 0, 4, 1.0, 2.0)
        assertEquals(DeckPlace(), deck.snapshot().place)
        scope.coroutineContext[kotlinx.coroutines.Job]?.cancel()
    }
}

private class PlaceEngine : DeckEngine {
    override var host: DeckEngineHost? = null
    var place = DeckPlace()
    override suspend fun prepareSession() = true
    override suspend fun play(item: DeckItem, generation: Long) = true
    override fun pause() {}
    override fun resume() {}
    override fun stop() {}
    override fun seekTo(seconds: Double) {}
    override fun seekBy(seconds: Double) {}
    override fun clearEnd() {}
    override fun restart() {}
    override fun position() = DeckPosition()
    override fun place() = place
    override fun release() {}
    override fun wipeCache() {}
}
