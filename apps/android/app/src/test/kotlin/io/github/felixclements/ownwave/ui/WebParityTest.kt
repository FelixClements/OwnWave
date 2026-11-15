package io.github.felixclements.ownwave.ui

import io.github.felixclements.ownwave.ui.admin.AdminJson
import io.github.felixclements.ownwave.ui.admin.phase
import io.github.felixclements.ownwave.ui.common.Colors
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.Rgb
import java.time.Instant
import java.time.LocalTime
import java.time.OffsetDateTime
import java.time.ZoneOffset
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** The Compose UI repeats helpers of apps/web/src/lib; these pin them to the web's results. */
class WebParityTest {
    @Test fun `name colours match the web`() {
        // Values from apps/web/src/lib/color.ts nameColor.
        assertEquals(Rgb(138, 104, 40), Colors.nameColor("Techno"))
        assertEquals(Rgb(138, 71, 40), Colors.nameColor("Drum & Bass"))
        assertEquals(Rgb(109, 138, 40), Colors.nameColor("Songs of Summer"))
    }

    @Test fun `dominant colour prefers colourful cells and falls back when transparent`() {
        val red = 0xFFC83232.toInt()
        val grey = 0xFF808080.toInt()
        val pixels = IntArray(16) { if (it == 0) red else grey }
        val c = Colors.dominantColor(pixels, 4, 4)
        assertTrue(c.r > c.g && c.r > c.b)
        assertEquals(Colors.FALLBACK, Colors.dominantColor(IntArray(16), 4, 4))
    }

    @Test fun `greeting follows the hour`() {
        assertEquals("Good morning", Format.greeting(LocalTime.of(8, 0)))
        assertEquals("Good afternoon", Format.greeting(LocalTime.of(13, 0)))
        assertEquals("Good evening", Format.greeting(LocalTime.of(21, 0)))
        assertEquals("Good evening", Format.greeting(LocalTime.of(2, 0)))
    }

    @Test fun `durations and plurals`() {
        assertEquals("3:07", Format.duration(187.9))
        assertEquals("1:02:03", Format.duration(3723.0))
        assertEquals("–:––", Format.duration(null))
        assertEquals("1 track", Format.plural(1, "track"))
        assertEquals("0 tracks", Format.plural(0, "track"))
    }

    @Test fun `time ago`() {
        val now = Instant.parse("2026-11-13T12:00:00Z")
        fun ago(seconds: Long) = Format.timeAgo(OffsetDateTime.ofInstant(now.minusSeconds(seconds), ZoneOffset.UTC), now)
        assertEquals("just now", ago(10))
        assertEquals("5 min ago", ago(300))
        assertEquals("3 h ago", ago(3 * 3600))
        assertEquals("yesterday", ago(86400 + 60))
        assertEquals("3 days ago", ago(3 * 86400))
    }

    @Test fun `initials`() {
        assertEquals("DB", Format.initials("drum  bass & more"))
        assertEquals("T", Format.initials("Techno"))
    }

    @Test fun `routes survive encoding`() {
        val routes = listOf(Route.Home, Route.Search, Route.Library, Route.Playlists, Route.Admin, Route.Stations, Route.Settings, Route.Station("a"), Route.Playlist("b"), Route.Genre("techno"))
        routes.forEach { assertEquals(it, Route.decode(it.encode())) }
    }

    @Test fun `scan progress follows the web`() {
        val json = Json.parseToJsonElement(
            """
            {"jobs": [
              {"id": "1", "status": "running", "started_at": "2026-11-13T10:00:00Z", "folders": {"total": 10, "done": 3, "failed": 1}, "stats": {"imported": 40, "model_success": 12, "failed_files": 2}},
              {"id": "2", "status": "pending", "folders": {"total": 0, "done": 0, "failed": 0}, "stats": {}},
              {"id": "3", "status": "failed", "error": "disk gone", "folders": {"total": 0, "done": 0, "failed": 0}, "stats": {}}
            ]}
            """,
        )
        val scans = AdminJson.scans(json)
        assertEquals(3, scans.size)
        assertEquals("Importing folders: 4 of 10", scans[0].phase().label)
        assertEquals(0.4, scans[0].phase().progress!!, 1e-9)
        assertEquals(40, scans[0].imported)
        assertEquals("Waiting for a worker…", scans[1].phase().label)
        assertNull(scans[1].phase().progress)
        assertEquals("disk gone", scans[2].phase().label)
        assertTrue(scans[2].finished)
    }

    @Test fun `health leaves out the version`() {
        val h = AdminJson.health(Json.parseToJsonElement("""{"database": "ok", "worker": "down", "version": "1.2"}"""))
        assertEquals(listOf("database" to "ok", "worker" to "down"), h)
    }
}
