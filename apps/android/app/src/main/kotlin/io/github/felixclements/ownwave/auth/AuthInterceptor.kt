package io.github.felixclements.ownwave.auth

import okhttp3.Interceptor
import okhttp3.Response

/**
 * Adds `Authorization: Bearer` to requests for the signed-in server only, never to other hosts.
 * A 401 from that server means the token was revoked or expired: [onUnauthorized] signs the app
 * out, so no further requests go out with a dead token (bursts of 4xx get IPs banned by CrowdSec).
 */
class AuthInterceptor(
    private val session: () -> Session?,
    private val onUnauthorized: (Session) -> Unit,
) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val current = session()
        if (current == null || !ServerUrl.sameOrigin(current.server, request.url)) {
            return chain.proceed(request)
        }
        val response = chain.proceed(
            request.newBuilder().header("Authorization", "Bearer ${current.token}").build(),
        )
        if (response.code == 401) onUnauthorized(current)
        return response
    }
}
