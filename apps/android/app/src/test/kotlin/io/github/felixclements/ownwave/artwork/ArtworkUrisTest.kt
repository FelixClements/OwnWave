package io.github.felixclements.ownwave.artwork

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ArtworkUrisTest {
    private val id = "11111111-1111-1111-1111-111111111111"
    private val base = "content://${ArtworkUris.AUTHORITY}"

    @Test fun `server artwork paths become content uris`() {
        assertEquals("$base/stations/$id", ArtworkUris.fromServerPath("/api/stations/$id/cover"))
        assertEquals("$base/playlists/$id", ArtworkUris.fromServerPath("/api/playlists/$id/cover"))
        assertEquals("$base/tracks/$id", ArtworkUris.track(id))
        assertEquals("$base/stations/$id", ArtworkUris.station(id))
    }

    @Test fun `content uri segments map back to the server path`() {
        assertEquals("api/stations/$id/cover", ArtworkUris.serverPath(listOf("stations", id)))
        assertEquals("api/tracks/$id/cover", ArtworkUris.serverPath(listOf("tracks", id)))
    }

    @Test fun `anything else is refused`() {
        assertNull(ArtworkUris.fromServerPath(null))
        assertNull(ArtworkUris.fromServerPath("/api/users/$id/cover"))
        assertNull(ArtworkUris.fromServerPath("https://evil.example/api/stations/$id/cover"))
        assertNull(ArtworkUris.serverPath(listOf("stations")))
        assertNull(ArtworkUris.serverPath(listOf("stations", id, "x")))
        assertNull(ArtworkUris.serverPath(listOf("me", "devices")))
        assertNull(ArtworkUris.serverPath(listOf("stations", "..")))
        assertNull(ArtworkUris.serverPath(listOf("stations", "a%2F..")))
        assertNull(ArtworkUris.track("../../admin"))
    }
}
