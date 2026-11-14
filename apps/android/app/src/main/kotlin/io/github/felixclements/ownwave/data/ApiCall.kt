package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.api.infrastructure.Serializer
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import retrofit2.Response
import java.io.IOException

/** A failed call, with the server's own message when it sent one (`{"error": "..."}`). */
class ApiException(message: String, val code: Int? = null) : IOException(message)

/**
 * Runs one request and turns it into a [Result]. A 2xx without a body (204) succeeds with null.
 * Nothing retries: bursts of 4xx responses get the phone's IP banned by CrowdSec.
 */
suspend fun <T> apiCall(request: suspend () -> Response<T>): Result<T?> = try {
    val response = request()
    if (response.isSuccessful) {
        Result.success(response.body())
    } else {
        Result.failure(ApiException(errorMessage(response), response.code()))
    }
} catch (e: IOException) {
    Result.failure(ApiException(e.message ?: "Could not reach the server."))
} catch (e: SerializationException) {
    Result.failure(ApiException("The server sent something unexpected."))
} catch (e: IllegalArgumentException) {
    // A malformed id (not a UUID): never sent.
    Result.failure(ApiException(e.message ?: "Invalid request."))
}

/** Like [apiCall], but a missing body is an error. */
suspend fun <T : Any> apiBody(request: suspend () -> Response<T>): Result<T> =
    apiCall(request).mapCatching { it ?: throw ApiException("The server sent an empty answer.") }

private fun errorMessage(response: Response<*>): String {
    val text = runCatching { response.errorBody()?.string() }.getOrNull().orEmpty()
    val fromJson = runCatching {
        (Serializer.kotlinxSerializationJson.parseToJsonElement(text) as? JsonObject)
            ?.get("error")
            ?.let { (it as? JsonPrimitive)?.content }
    }.getOrNull()
    return fromJson?.takeIf { it.isNotBlank() } ?: when (response.code()) {
        401 -> "Signed out. Sign in again."
        403 -> "You are not allowed to do that."
        404 -> "Not found."
        429 -> "Too many requests. Wait a minute and try again."
        in 500..599 -> "The server had a problem (HTTP ${response.code()})."
        else -> "HTTP ${response.code()}"
    }
}
