package io.github.felixclements.ownwave.auth

import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull

/** Turns what the user typed into a server origin such as `https://ownwave.example.com`. */
object ServerUrl {
    sealed interface Result {
        data class Ok(val url: HttpUrl) : Result
        data object Invalid : Result
        data object InsecureNotAllowed : Result
    }

    /**
     * Adds `https://` when no scheme is given, drops any path, query and trailing slash.
     * Plain `http` is accepted only when [allowHttp] is true (debug builds, for a local dev server).
     */
    fun parse(input: String, allowHttp: Boolean): Result {
        val trimmed = input.trim()
        if (trimmed.isEmpty() || trimmed.any { it.isWhitespace() }) return Result.Invalid
        val withScheme = if ("://" in trimmed) trimmed else "https://$trimmed"
        val url = withScheme.toHttpUrlOrNull() ?: return Result.Invalid
        if (url.scheme == "http" && !allowHttp) return Result.InsecureNotAllowed
        return Result.Ok(url.newBuilder().encodedPath("/").query(null).fragment(null).build())
    }

    /** True when [url] points at the same scheme, host and port as [server]. */
    fun sameOrigin(server: HttpUrl, url: HttpUrl): Boolean =
        server.scheme == url.scheme && server.host == url.host && server.port == url.port
}
