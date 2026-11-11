package io.github.felixclements.ownwave.auth

import okhttp3.HttpUrl

/** A signed-in device: the server it belongs to and its Bearer token. */
data class Session(
    val server: HttpUrl,
    val token: String,
    val userId: String,
    val username: String,
    val isAdmin: Boolean,
) {
    override fun toString(): String = "Session(server=$server, user=$username, token=<hidden>)"
}

sealed interface SessionState {
    data object Loading : SessionState
    data object SignedOut : SessionState
    data class SignedIn(val session: Session) : SessionState
}
