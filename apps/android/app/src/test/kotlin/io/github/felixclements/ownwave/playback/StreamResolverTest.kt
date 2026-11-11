package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.Quality
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class StreamResolverTest {
    private var server: HttpUrl? = "https://ownwave.example.com/".toHttpUrl()
    private var quality = Quality.HIGH
    private val resolver = StreamResolver(server = { server }, quality = { quality })

    @Test fun `track uri becomes a stream url with the current quality`() {
        assertEquals(
            "https://ownwave.example.com/api/stream/t1?quality=high",
            resolver.resolve("ownwave://track/t1").toString(),
        )
        quality = Quality.NORMAL
        assertEquals(
            "https://ownwave.example.com/api/stream/t1?quality=normal",
            resolver.resolve("ownwave://track/t1").toString(),
        )
    }

    @Test fun `keeps a non-default port`() {
        server = "http://10.0.2.2:8080/".toHttpUrl()
        assertEquals("http://10.0.2.2:8080/api/stream/t1?quality=high", resolver.resolve("ownwave://track/t1").toString())
    }

    @Test fun `nothing when signed out or for other uris`() {
        assertNull(resolver.resolve("https://evil.example/track/t1"))
        server = null
        assertNull(resolver.resolve("ownwave://track/t1"))
    }
}
