package io.github.felixclements.ownwave.auth

import io.github.felixclements.ownwave.api.models.Device
import io.github.felixclements.ownwave.data.ApiFactory
import kotlinx.coroutines.test.runTest
import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.time.OffsetDateTime

class DeviceRepositoryTest {
    private val server = MockWebServer()
    private lateinit var session: Session
    private lateinit var repo: DeviceRepository

    @Before fun setUp() {
        server.start()
        session = Session(server.url("/"), "tok-1", "u1", "felix", false)
        val client = OkHttpClient.Builder()
            .followRedirects(false)
            .addInterceptor(AuthInterceptor({ session }) {})
            .build()
        repo = DeviceRepository(ApiFactory(client))
    }

    @After fun tearDown() = server.close()

    private fun enqueue(code: Int, body: String = "", contentType: String = "application/json") {
        server.enqueue(MockResponse.Builder().code(code).body(body).addHeader("Content-Type", contentType).build())
    }

    private fun device(id: String, current: Boolean) = Device(
        id = id,
        name = "Pixel",
        createdAt = OffsetDateTime.parse("2026-11-10T10:00:00Z"),
        expiresAt = OffsetDateTime.parse("2027-11-10T10:00:00Z"),
        current = current,
    )

    @Test fun listPutsThisDeviceFirstAndKeepsServerOrder() = runTest {
        enqueue(
            200,
            """
            {"devices":[
              {"id":"11111111-1111-1111-1111-111111111111","name":"Tablet","created_at":"2026-11-01T10:00:00Z",
               "expires_at":"2027-11-01T10:00:00Z","current":false,"last_used_at":"2026-11-11T09:00:00Z"},
              {"id":"22222222-2222-2222-2222-222222222222","name":"Car phone","created_at":"2026-11-02T10:00:00Z",
               "expires_at":"2027-11-02T10:00:00Z","current":false,"last_used_at":null},
              {"id":"33333333-3333-3333-3333-333333333333","name":"Pixel 8","created_at":"2026-11-10T10:00:00Z",
               "expires_at":"2027-11-10T10:00:00Z","current":true,"last_used_at":"2026-11-10T11:00:00Z"}
            ]}
            """.trimIndent(),
        )
        val devices = repo.devices(session).getOrThrow()

        assertEquals(listOf("Pixel 8", "Tablet", "Car phone"), devices.map { it.name })
        assertNull(devices[2].lastUsedAt)
        val request = server.takeRequest()
        assertEquals("GET", request.method)
        assertEquals("/api/me/devices", request.url.encodedPath)
        assertEquals("Bearer tok-1", request.headers["Authorization"])
    }

    @Test fun listFailureIsReturnedNotRetried() = runTest {
        enqueue(503, "busy", "text/plain")
        assertTrue(repo.devices(session).isFailure)
        assertEquals(1, server.requestCount)
    }

    @Test fun listHtmlPageIsAFailure() = runTest {
        enqueue(200, "<html>gate</html>", "text/html")
        assertTrue(repo.devices(session).isFailure)
    }

    @Test fun revokeSendsDeleteForThatDevice() = runTest {
        enqueue(204)
        val id = "11111111-1111-1111-1111-111111111111"
        assertTrue(repo.revoke(session, device(id, current = false)).isSuccess)

        val request = server.takeRequest()
        assertEquals("DELETE", request.method)
        assertEquals("/api/me/devices/$id", request.url.encodedPath)
    }

    @Test fun revokeAlreadyGoneCountsAsDone() = runTest {
        enqueue(404, """{"error":"not found"}""")
        assertTrue(repo.revoke(session, device("11111111-1111-1111-1111-111111111111", current = false)).isSuccess)
    }

    @Test fun revokeServerErrorIsAFailureNotRetried() = runTest {
        enqueue(500, "boom", "text/plain")
        assertTrue(repo.revoke(session, device("11111111-1111-1111-1111-111111111111", current = false)).isFailure)
        assertEquals(1, server.requestCount)
    }

    @Test fun neverRevokesThisDeviceOrNonUuidIds() = runTest {
        assertTrue(repo.revoke(session, device("33333333-3333-3333-3333-333333333333", current = true)).isFailure)
        assertTrue(repo.revoke(session, device("../me", current = false)).isFailure)
        assertEquals(0, server.requestCount)
    }
}
