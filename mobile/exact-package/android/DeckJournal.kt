package choir.autoradio

import java.io.File
import java.io.IOException
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.time.Instant

/** Ordered, durable deck evidence. The lines file and the monotonic numbers file are separate so a lost lines file never repeats a number. */
class DeckJournal(val storage: File? = null) {
    var lines: List<DeckLine> = emptyList()
        private set
    var seq: Int = 0
        private set
    var acked: Int = 0
        private set
    var dropped: Int = 0
        private set
    var saveFailures: Int = 0
        private set
    var isLoaded: Boolean = true
        private set
    var log: (String) -> Unit = {}
    var now: () -> Long = System::currentTimeMillis

    private var floor = 0
    private var aside = false
    private var bootPending = false

    init {
        if (storage != null) {
            val loaded = readSaved()
            when (loaded) {
                Read.ABSENT -> Unit
                is Read.OK -> { val saved = loaded.saved; lines = saved.entries; seq = saved.seq; acked = saved.acked }
                Read.UNREADABLE -> isLoaded = false
                Read.CORRUPT -> setAside()
            }
            readNumbers()?.let { seq = maxOf(seq, it.seq); acked = maxOf(acked, it.acked) }
            floor = seq
            lines = lines.filter { it.seq > acked }
            if (aside) append("system", item = "", command = "journal-corrupt", reason = "the journal file could not be decoded and was moved aside")
        }
    }

    private val numbersFile: File?
        get() = storage?.let { File(it.parentFile, it.nameWithoutExtension + ".seq.json") }
    private val badFile: File?
        get() = storage?.let { File(it.parentFile, it.nameWithoutExtension + ".bad") }

    private sealed interface Read {
        data object ABSENT : Read
        data object UNREADABLE : Read
        data object CORRUPT : Read
        data class OK(val saved: Saved) : Read
    }
    private data class Saved(val seq: Int, val acked: Int, val entries: List<DeckLine>)
    private data class Numbers(val seq: Int, val acked: Int)

    private fun readSaved(): Read {
        val f = storage ?: return Read.ABSENT
        if (!f.exists()) return Read.ABSENT
        val bytes = try { f.readBytes() } catch (_: IOException) { return Read.UNREADABLE } catch (_: SecurityException) { return Read.UNREADABLE }
        val root = try { JsonParser(bytes.toString(Charsets.UTF_8)).parse() } catch (_: IllegalArgumentException) { return Read.CORRUPT }
        val o = root as? JObject ?: return Read.CORRUPT
        val s = o.number("seq")?.toInt() ?: return Read.CORRUPT
        val a = o.number("acked")?.toInt() ?: return Read.CORRUPT
        if (s < 0 || a < 0 || a > s) return Read.CORRUPT
        val rows = o["lines"] as? JArray ?: return Read.CORRUPT
        val decoded = ArrayList<DeckLine>(rows.items.size)
        for (row in rows.items) {
            val l = row as? JObject ?: return Read.CORRUPT
            val n = l.number("seq")?.toInt() ?: return Read.CORRUPT
            val type = l.string("type") ?: return Read.CORRUPT
            val item = l.string("item") ?: return Read.CORRUPT
            val at = l.string("at") ?: return Read.CORRUPT
            if (n <= 0 || n > s) return Read.CORRUPT
            decoded += DeckLine(
                n, type, item, at,
                block = l.number("block")?.toInt() ?: 0,
                seconds = l.number("seconds") ?: 0.0,
                final = l.number("final")?.toInt() ?: 0,
                rendered = l.boolean("rendered") ?: false,
                end = l.string("end") ?: "",
                command = l.string("command") ?: "",
                reason = l.string("reason") ?: "",
            )
        }
        // Against the FILE's acknowledged floor, not this instance's: a launch can append and ack before the file is readable.
        if (decoded.any { it.seq <= a } || decoded.map { it.seq }.distinct().size != decoded.size) return Read.CORRUPT
        return Read.OK(Saved(s, a, decoded))
    }

    private fun readNumbers(): Numbers? {
        val f = numbersFile ?: return null
        if (!f.exists()) return null
        return try {
            val o = JsonParser(f.readText()).parse() as? JObject ?: return null
            val s = o.number("seq")?.toInt() ?: return null
            val a = o.number("acked")?.toInt() ?: return null
            if (s < 0 || a < 0 || a > s) null else Numbers(s, a)
        } catch (_: Exception) { null }
    }

    private fun setAside() {
        val f = storage ?: return
        val bad = badFile ?: return
        runCatching { if (bad.exists()) bad.delete() }
        aside = runCatching { f.renameTo(bad) }.getOrDefault(false)
    }

    private fun ensureLoaded() {
        if (isLoaded) return
        val held = lines
        val loaded = readSaved()
        when (loaded) {
            Read.UNREADABLE -> return
            Read.ABSENT -> { lines = emptyList(); seq = floor }
            Read.CORRUPT -> { setAside(); lines = emptyList(); seq = floor }
            is Read.OK -> { val saved = loaded.saved; lines = saved.entries; seq = maxOf(floor, saved.seq); acked = maxOf(acked, saved.acked) }
        }
        lines = lines.filter { it.seq > acked }
        seq = maxOf(seq, lines.lastOrNull()?.seq ?: 0, acked)
        val merged = ArrayList(lines)
        for (old in held) { seq += 1; merged += old.copy(seq = seq) }
        lines = merged
        isLoaded = true
        if (aside) { aside = false; append("system", item = "", command = "journal-corrupt", reason = "the journal file could not be decoded and was moved aside") }
        if (bootPending) { bootPending = false; closeOpenAtBoot() }
    }

    fun append(
        type: String,
        item: String,
        block: Int = 0,
        seconds: Double = 0.0,
        final: Int = 0,
        rendered: Boolean = false,
        end: String = "",
        command: String = "",
        reason: String = "",
    ): DeckLine {
        ensureLoaded()
        seq += 1
        val line = DeckLine(seq, type, item, stamp(), block, DeckContract.finite(seconds), final, rendered, end, command, reason)
        val next = ArrayList(lines)
        next += line
        if (next.size > DeckRules.JOURNAL_MAX) {
            val cut = next.size - DeckRules.JOURNAL_MAX + DeckRules.JOURNAL_MAX / 10
            repeat(cut) { next.removeAt(0) }
            dropped += cut
            seq += 1
            next += DeckLine(seq, "system", "", stamp(), command = "dropped", reason = "dropped $cut unacknowledged lines")
        }
        lines = next
        save()
        return line
    }

    fun ack(through: Int) {
        ensureLoaded()
        if (through <= acked) return
        acked = minOf(through, seq)
        lines = lines.filter { it.seq > acked }
        save()
    }

    val openItem: String?
        get() {
            var open: String? = null
            for (line in lines) {
                if (line.type == "started") open = line.item
                else if (line.type == "ended" && line.item == open) open = null
            }
            return open
        }

    fun closeOpenAtBoot(): DeckLine? {
        ensureLoaded()
        if (!isLoaded) { bootPending = true; return null }
        val item = openItem ?: return null
        val start = lines.indexOfLast { it.type == "started" && it.item == item }
        val evidence = lines.drop(start + 1).lastOrNull { it.item == item && (it.type == "sounding" || it.type == "started") }
        val sounded = lines.drop(start + 1).any { it.item == item && it.type == "sounding" }
        return append("ended", item, evidence?.block ?: 0, evidence?.seconds ?: 0.0, evidence?.final ?: 0, sounded, DeckEnd.FAILED.wire, reason = "interrupted by a restart")
    }

    fun wipe() {
        lines = emptyList(); seq = 0; acked = 0; dropped = 0; floor = 0; aside = false; bootPending = false; isLoaded = true
        storage?.delete(); numbersFile?.delete()
    }

    private fun stamp(): String = Instant.ofEpochMilli(now()).toString()

    private fun save() {
        val f = storage ?: return
        if (!isLoaded) return
        try {
            val root = JObject(mapOf(
                "seq" to JNumber(seq.toDouble()), "acked" to JNumber(acked.toDouble()),
                "lines" to JArray(lines.map { line ->
                    JObject(mapOf(
                        "seq" to JNumber(line.seq.toDouble()), "type" to JString(line.type), "item" to JString(line.item), "at" to JString(line.at),
                        "block" to JNumber(line.block.toDouble()), "seconds" to JNumber(line.seconds), "final" to JNumber(line.final.toDouble()),
                        "rendered" to JBool(line.rendered), "end" to JString(line.end), "command" to JString(line.command), "reason" to JString(line.reason),
                    ))
                }),
            ))
            val nums = JObject(mapOf("seq" to JNumber(seq.toDouble()), "acked" to JNumber(acked.toDouble())))
            atomicWrite(numbersFile ?: return, Json.write(nums))
            atomicWrite(f, Json.write(root))
        } catch (_: Exception) {
            saveFailures += 1
            log("deck journal: save failed ($saveFailures failures, ${lines.size} lines held)")
        }
    }

    private fun atomicWrite(f: File, text: String) {
        val tmp = File(f.parentFile, f.name + ".tmp")
        try {
            tmp.writeText(text)
            try { Files.move(tmp.toPath(), f.toPath(), StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE) }
            catch (_: AtomicMoveNotSupportedException) { Files.move(tmp.toPath(), f.toPath(), StandardCopyOption.REPLACE_EXISTING) }
        } finally { if (tmp.exists()) tmp.delete() }
    }

    private sealed interface J
    private data class JObject(val fields: Map<String, J>) : J
    private data class JArray(val items: List<J>) : J
    private data class JString(val value: String) : J
    private data class JNumber(val value: Double) : J
    private data class JBool(val value: Boolean) : J
    private data object JNull : J

    private fun JObject.number(key: String): Double? = (this[key] as? JNumber)?.value
    private fun JObject.string(key: String): String? = (this[key] as? JString)?.value
    private fun JObject.boolean(key: String): Boolean? = (this[key] as? JBool)?.value
    private operator fun JObject.get(key: String): J? = fields[key]

    private object Json {
        fun write(value: J): String = when (value) {
            is JObject -> value.fields.entries.joinToString(",", "{", "}") { "${quote(it.key)}:${write(it.value)}" }
            is JArray -> value.items.joinToString(",", "[", "]") { write(it) }
            is JString -> quote(value.value)
            is JNumber -> if (value.value == value.value.toLong().toDouble()) value.value.toLong().toString() else value.value.toString()
            is JBool -> value.value.toString()
            JNull -> "null"
        }
        private fun quote(text: String): String = buildString(text.length + 2) {
            append('"')
            for (c in text) when (c) {
                '"' -> append("\\\""); '\\' -> append("\\\\"); '\b' -> append("\\b"); '\u000c' -> append("\\f"); '\n' -> append("\\n"); '\r' -> append("\\r"); '\t' -> append("\\t")
                else -> if (c.code < 0x20) append("\\u%04x".format(c.code)) else append(c)
            }
            append('"')
        }
    }

    private class JsonParser(private val text: String) {
        private var at = 0
        fun parse(): J { val value = value(); space(); if (at != text.length) fail("trailing"); return value }
        private fun value(): J {
            space(); if (at >= text.length) fail("value")
            return when (text[at]) {
                '{' -> objectValue(); '[' -> arrayValue(); '"' -> JString(stringValue())
                't' -> { literal("true"); JBool(true) }; 'f' -> { literal("false"); JBool(false) }; 'n' -> { literal("null"); JNull }
                else -> numberValue()
            }
        }
        private fun objectValue(): JObject {
            at++; val fields = LinkedHashMap<String, J>(); space()
            if (take('}')) return JObject(fields)
            while (true) {
                space(); if (at >= text.length || text[at] != '"') fail("key")
                val key = stringValue(); space(); if (!take(':')) fail("colon")
                if (fields.containsKey(key)) fail("duplicate")
                fields[key] = value(); space()
                if (take('}')) return JObject(fields)
                if (!take(',')) fail("comma")
            }
        }
        private fun arrayValue(): JArray {
            at++; val values = ArrayList<J>(); space()
            if (take(']')) return JArray(values)
            while (true) { values += value(); space(); if (take(']')) return JArray(values); if (!take(',')) fail("comma") }
        }
        private fun stringValue(): String {
            at++; val out = StringBuilder()
            while (at < text.length) {
                when (val c = text[at++]) {
                    '"' -> return out.toString()
                    '\\' -> if (at >= text.length) fail("escape") else when (val e = text[at++]) {
                        '"', '\\', '/' -> out.append(e); 'b' -> out.append('\b'); 'f' -> out.append('\u000c'); 'n' -> out.append('\n'); 'r' -> out.append('\r'); 't' -> out.append('\t')
                        'u' -> { if (at + 4 > text.length) fail("unicode"); val hex = text.substring(at, at + 4); at += 4; out.append(hex.toInt(16).toChar()) }
                        else -> fail("escape")
                    }
                    else -> if (c.code < 0x20) fail("control") else out.append(c)
                }
            }
            fail("string")
        }
        private fun numberValue(): J {
            val start = at
            if (take('-')) Unit
            if (take('0')) { if (at < text.length && text[at].isDigit()) fail("number") }
            else { if (at >= text.length || text[at] !in '1'..'9') fail("number"); while (at < text.length && text[at].isDigit()) at++ }
            if (take('.')) { val digits = at; while (at < text.length && text[at].isDigit()) at++; if (digits == at) fail("number") }
            if (take('e') || take('E')) { if (take('+') || take('-')) Unit; val digits = at; while (at < text.length && text[at].isDigit()) at++; if (digits == at) fail("number") }
            val n = text.substring(start, at).toDoubleOrNull() ?: fail("number")
            if (!n.isFinite()) fail("number")
            return JNumber(n)
        }
        private fun literal(word: String) { if (!text.startsWith(word, at)) fail("literal"); at += word.length }
        private fun take(c: Char): Boolean = if (at < text.length && text[at] == c) { at++; true } else false
        private fun space() { while (at < text.length && text[at].isWhitespace()) at++ }
        private fun fail(why: String): Nothing = throw IllegalArgumentException("invalid journal JSON: $why at $at")
    }
}
