package io.github.felixclements.ownwave.auth

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

class AccountRepositoryTest {
    private val server = MockWebServer()
    private var session: Session? = null
    private val unauthorized = mutableListOf<Session>()
    private lateinit var repo: AccountRepository

    @Before fun setUp() {
        server.start()
        // Same setup as AppContainer: no redirects, auth interceptor.
        val client = OkHttpClient.Builder()
            .followRedirects(false)
            .followSslRedirects(false)
            .addInterceptor(AuthInterceptor({ session }) { unauthorized += it })
            .build()
        repo = AccountRepository(ApiFactory(client), allowHttp = true)
    }

    @After fun tearDown() = server.close()

    private fun base() = server.url("/").toString()

    private fun enqueue(code: Int, body: String = "", contentType: String = "application/json", location: String? = null) {
        val builder = MockResponse.Builder().code(code).body(body).addHeader("Content-Type", contentType)
        if (location != null) builder.addHeader("Location", location)
        server.enqueue(builder.build())
    }

    @Test fun successReturnsSessionAndSendsDeviceName() = runTest {
        enqueue(
            200,
            """
            {"token":"tok-123","user":{"id":"u1","username":"felix","is_admin":true},
             "device":{"id":"d1","name":"Pixel","created_at":"2026-11-10T10:00:00Z",
                       "expires_at":"2027-11-10T10:00:00Z","current":true}}
            """.trimIndent(),
        )
        val result = repo.login(base(), " felix ", "pw", "Pixel 8")

        val s = (result as LoginResult.Success).session
        assertEquals("tok-123", s.token)
        assertEquals("felix", s.username)
        assertTrue(s.isAdmin)
        val request = server.takeRequest()
        assertEquals("/api/auth/device", request.url.encodedPath)
        val body = request.body!!.utf8()
        assertTrue(body, body.contains("\"device_name\":\"Pixel 8\"") && body.contains("\"username\":\"felix\""))
        assertNull("no token on sign-in", request.headers["Authorization"])
    }

    @Test fun wrongPassword() = runTest {
        enqueue(401, "invalid credentials", "text/plain")
        assertEquals(LoginResult.BadCredentials, repo.login(base(), "felix", "bad", "Pixel"))
    }

    @Test fun rateLimited() = runTest {
        enqueue(429, "slow down", "text/plain")
        assertEquals(LoginResult.RateLimited, repo.login(base(), "felix", "pw", "Pixel"))
    }

    @Test fun redirectToLoginGateIsNotOwnWave() = runTest {
        enqueue(302, location = "https://auth.example.com/application/o/authorize/")
        assertEquals(LoginResult.NotOwnWave, repo.login(base(), "felix", "pw", "Pixel"))
    }

    @Test fun htmlPageIsNotOwnWave() = runTest {
        enqueue(200, "<html><body>Sign in</body></html>", "text/html")
        assertEquals(LoginResult.NotOwnWave, repo.login(base(), "felix", "pw", "Pixel"))
    }

    @Test fun serverError() = runTest {
        enqueue(503, "busy", "text/plain")
        assertEquals(LoginResult.ServerError(503), repo.login(base(), "felix", "pw", "Pixel"))
    }

    @Test fun bearerOnlyForSignedInServerAndSignsOutOn401() = runTest {
        val s = Session(server.url("/"), "tok-9", "u1", "felix", false)
        session = s
        enqueue(401, "expired", "text/plain")
        repo.logout(s)

        val request = server.takeRequest()
        assertEquals("Bearer tok-9", request.headers["Authorization"])
        assertEquals(listOf(s), unauthorized)
    }
}
