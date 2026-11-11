package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.auth.Session
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class PlaybackRepositoryTest {
    private val server = MockWebServer()
    private lateinit var repo: PlaybackRepository
    private lateinit var session: Session

    private val station = "11111111-1111-1111-1111-111111111111"
    private val track = "22222222-2222-2222-2222-222222222222"

    @Before fun setUp() {
        server.start()
        repo = PlaybackRepository(ApiFactory(OkHttpClient()))
        session = Session(server.url("/"), token = "t", userId = "u", username = "me", isAdmin = false)
    }

    @After fun tearDown() = server.close()

    private fun enqueue(code: Int, body: String = "") {
        server.enqueue(
            MockResponse.Builder().code(code).body(body).addHeader("Content-Type", "application/json").build(),
        )
    }

    @Test fun `queue parses tracks in order`() = runTest {
        enqueue(
            200,
            """{"queue":[
              {"id":"$track","title":"One","position":0,"liked":true,"artist":"A","duration_seconds":200.5},
              {"id":"33333333-3333-3333-3333-333333333333","title":"Two","position":1,"liked":false}
            ]}""",
        )
        val tracks = repo.queue(session, station).getOrThrow()
        assertEquals(listOf("One", "Two"), tracks.map { it.title })
        assertTrue(tracks[0].liked)
        assertEquals("/api/stations/$station/queue", server.takeRequest().url.encodedPath)
    }

    @Test fun `queue failure is a failure, not an empty list`() = runTest {
        enqueue(404, """{"error":"not found"}""")
        assertTrue(repo.queue(session, station).isFailure)
        assertEquals(1, server.requestCount)
    }

    @Test fun `bad ids are never sent`() = runTest {
        assertTrue(repo.queue(session, "../admin").isFailure)
        assertFalse(repo.ban(session, "not-a-uuid"))
        assertEquals(0, server.requestCount)
    }

    @Test fun `record play posts the station`() = runTest {
        enqueue(204)
        assertTrue(repo.recordPlay(session, track, station))
        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertEquals("/api/tracks/$track/played", request.url.encodedPath)
        assertEquals("""{"station_id":"$station"}""", request.body!!.utf8())
    }

    @Test fun `like posts and unlike deletes`() = runTest {
        enqueue(204)
        enqueue(204)
        assertTrue(repo.setLiked(session, track, liked = true))
        assertTrue(repo.setLiked(session, track, liked = false))
        val like = server.takeRequest()
        assertEquals("POST", like.method)
        assertEquals("/api/tracks/$track/feedback", like.url.encodedPath)
        assertEquals("""{"feedback":"like"}""", like.body!!.utf8())
        val unlike = server.takeRequest()
        assertEquals("DELETE", unlike.method)
        assertEquals("/api/tracks/$track/feedback", unlike.url.encodedPath)
        assertEquals("""{"feedback":"like"}""", unlike.body!!.utf8())
    }

    @Test fun `ban posts ban and reports failure without retrying`() = runTest {
        enqueue(500)
        assertFalse(repo.ban(session, track))
        val request = server.takeRequest()
        assertEquals("""{"feedback":"ban"}""", request.body!!.utf8())
        assertEquals(1, server.requestCount)
    }
}
