package io.github.felixclements.ownwave.auth

import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ServerUrlTest {
    private fun ok(input: String, allowHttp: Boolean = false): String =
        (ServerUrl.parse(input, allowHttp) as ServerUrl.Result.Ok).url.toString()

    @Test fun addsHttpsWhenSchemeMissing() = assertEquals("https://ownwave.example.com/", ok("ownwave.example.com"))

    @Test fun trimsAndDropsPathQueryAndSlash() =
        assertEquals("https://ownwave.example.com/", ok("  https://ownwave.example.com/library?x=1#y  "))

    @Test fun keepsPort() = assertEquals("https://host:8443/", ok("host:8443"))

    @Test fun rejectsHttpUnlessAllowed() {
        assertEquals(ServerUrl.Result.InsecureNotAllowed, ServerUrl.parse("http://10.0.2.2:8080", allowHttp = false))
        assertEquals("http://10.0.2.2:8080/", ok("http://10.0.2.2:8080", allowHttp = true))
    }

    @Test fun rejectsGarbage() {
        assertEquals(ServerUrl.Result.Invalid, ServerUrl.parse("", allowHttp = false))
        assertEquals(ServerUrl.Result.Invalid, ServerUrl.parse("not a url", allowHttp = false))
        assertEquals(ServerUrl.Result.Invalid, ServerUrl.parse("ftp://host", allowHttp = false))
    }

    @Test fun sameOriginComparesSchemeHostPort() {
        val server = "https://ownwave.example.com/".toHttpUrl()
        assertTrue(ServerUrl.sameOrigin(server, "https://ownwave.example.com/api/me".toHttpUrl()))
        assertFalse(ServerUrl.sameOrigin(server, "http://ownwave.example.com/api/me".toHttpUrl()))
        assertFalse(ServerUrl.sameOrigin(server, "https://ownwave.example.com:8443/api/me".toHttpUrl()))
        assertFalse(ServerUrl.sameOrigin(server, "https://evil.example.com/api/me".toHttpUrl()))
    }
}
