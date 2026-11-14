package io.github.felixclements.ownwave.ui.common

import java.text.NumberFormat
import java.time.Duration
import java.time.Instant
import java.time.LocalTime
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import kotlin.math.abs
import kotlin.math.floor
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

/** Text helpers shared with the web UI (apps/web/src/lib/format.ts). */
object Format {
    /** "Good morning" / "Good afternoon" / "Good evening". */
    fun greeting(time: LocalTime = LocalTime.now()): String {
        val h = time.hour
        return if (h < 5 || h >= 18) "Good evening" else if (h < 12) "Good morning" else "Good afternoon"
    }

    /** "1 track", "1,234 tracks". */
    fun plural(n: Int, word: String): String = "${NumberFormat.getIntegerInstance().format(n)} $word${if (n == 1) "" else "s"}"

    /** 3:07, or 1:02:03 past an hour. */
    fun duration(seconds: Double?): String {
        if (seconds == null || seconds.isNaN() || seconds.isInfinite() || seconds < 0) return "–:––"
        val s = floor(seconds).toLong()
        val h = s / 3600
        val m = (s % 3600) / 60
        val sec = (s % 60).toString().padStart(2, '0')
        return if (h > 0) "$h:${m.toString().padStart(2, '0')}:$sec" else "$m:$sec"
    }

    /** "just now", "5 min ago", "3 h ago", "2 days ago", or a date. */
    fun timeAgo(at: OffsetDateTime, now: Instant = Instant.now()): String {
        val secs = Duration.between(at.toInstant(), now).seconds
        return when {
            secs < 60 -> "just now"
            secs < 3600 -> "${secs / 60} min ago"
            secs < 86400 -> "${secs / 3600} h ago"
            secs < 7 * 86400 -> (secs / 86400).let { if (it == 1L) "yesterday" else "$it days ago" }
            else -> at.atZoneSameInstant(ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.SHORT))
        }
    }

    fun date(at: OffsetDateTime): String =
        at.atZoneSameInstant(ZoneId.systemDefault()).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM))

    /** Up to two initials, for artwork without an image. */
    fun initials(name: String): String =
        name.split(Regex("\\s+")).filter { it.isNotEmpty() }.take(2).joinToString("") { it.first().uppercase() }
}

/** An RGB colour as 0..255 channels (like the web's `Rgb`). */
data class Rgb(val r: Int, val g: Int, val b: Int) {
    fun scaled(k: Double) = Rgb((r * k).roundToInt(), (g * k).roundToInt(), (b * k).roundToInt())
}

/** Colours from names and artwork (apps/web/src/lib/color.ts). */
object Colors {
    val FALLBACK = Rgb(45, 212, 191)

    /** Stable colour for things without artwork, from their name: HSL(hue, 55%, 35%). */
    fun nameColor(name: String): Rgb {
        var h = 0L
        // Same hash as the web (UTF-16 code units, unsigned 32-bit).
        for (ch in name) h = (h * 31 + ch.code) and 0xFFFFFFFFL
        val hue = (h % 360).toInt()
        val s = 0.55
        val l = 0.35
        val a = s * min(l, 1 - l)
        fun k(n: Int) = (n + hue / 30.0) % 12
        fun f(n: Int) = (255 * (l - a * max(-1.0, min(k(n) - 3, min(9 - k(n), 1.0))))).roundToInt()
        return Rgb(f(0), f(8), f(4))
    }

    /** Scales the colour so its brightest channel is at most [maxChannel]. */
    fun darken(c: Rgb, maxChannel: Int = 150): Rgb {
        val m = max(max(c.r, c.g), max(c.b, 1))
        return c.scaled(min(1.0, maxChannel.toDouble() / m))
    }

    /**
     * Picks the most saturated of the image's averaged colours: averages each cell of a 4x4 grid,
     * then prefers colourful, mid-bright cells over grey ones. [pixels] are ARGB ints.
     */
    fun dominantColor(pixels: IntArray, width: Int, height: Int): Rgb {
        val grid = 4
        var best: Rgb? = null
        var bestScore = -1.0
        for (gy in 0 until grid) for (gx in 0 until grid) {
            var r = 0L
            var g = 0L
            var b = 0L
            var n = 0
            for (y in (gy * height) / grid until ((gy + 1) * height) / grid) {
                for (x in (gx * width) / grid until ((gx + 1) * width) / grid) {
                    val p = pixels[y * width + x]
                    if ((p ushr 24) < 128) continue
                    r += (p shr 16) and 0xFF
                    g += (p shr 8) and 0xFF
                    b += p and 0xFF
                    n++
                }
            }
            if (n == 0) continue
            val c = Rgb((r / n).toInt(), (g / n).toInt(), (b / n).toInt())
            val hi = max(c.r, max(c.g, c.b))
            val lo = min(c.r, min(c.g, c.b))
            val score = (hi - lo) * (1 - abs(hi - 140) / 255.0)
            if (score > bestScore) {
                best = c
                bestScore = score
            }
        }
        return best?.let { darken(it) } ?: FALLBACK
    }
}
