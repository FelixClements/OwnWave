package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.auth.Session
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class StationRepositoryTest {
    private val server = MockWebServer()
    private lateinit var repo: StationRepository
    private lateinit var session: Session

    private val station = "11111111-1111-1111-1111-111111111111"

    @Before fun setUp() {
        server.start()
        repo = StationRepository(ApiFactory(OkHttpClient()))
        session = Session(server.url("/"), token = "t", userId = "u", username = "me", isAdmin = false)
    }

    @After fun tearDown() = server.close()

    private fun enqueue(code: Int, body: String) {
        server.enqueue(
            MockResponse.Builder().code(code).body(body).addHeader("Content-Type", "application/json").build(),
        )
    }

    @Test fun `browse asks for the node and parses items`() = runTest {
        enqueue(
            200,
            """{"id":"genre:techno","title":"Techno","items":[
              {"id":"station:$station","title":"Techno","browsable":false,"playable":true,
               "station_id":"$station","track_count":12,"artwork_url":"/api/stations/$station/cover"}
            ]}""",
        )
        val items = repo.browse(session, "genre:techno").getOrThrow()
        assertEquals(station, items.single().stationId)
        assertEquals("/api/stations/$station/cover", items.single().artworkUrl)
        val url = server.takeRequest().url
        assertEquals("/api/browse", url.encodedPath)
        assertEquals("genre:techno", url.queryParameter("node"))
    }

    @Test fun `browse failure is a failure and not retried`() = runTest {
        enqueue(404, """{"error":"unknown node"}""")
        assertTrue(repo.browse(session, "genre:gone").isFailure)
        assertEquals(1, server.requestCount)
    }

    @Test fun `search trims the query and parses hits`() = runTest {
        enqueue(
            200,
            """{"query":"techno","results":[
              {"id":"station:$station","title":"Techno","browsable":false,"playable":true,"reason":"genre","station_id":"$station"}
            ]}""",
        )
        val hits = repo.searchStations(session, "  techno ").getOrThrow()
        assertEquals(station, hits.single().stationId)
        val url = server.takeRequest().url
        assertEquals("/api/search/stations", url.encodedPath)
        assertEquals("techno", url.queryParameter("q"))
    }

    @Test fun `a blank search sends nothing`() = runTest {
        assertTrue(repo.searchStations(session, "   ").getOrThrow().isEmpty())
        assertEquals(0, server.requestCount)
    }
}
