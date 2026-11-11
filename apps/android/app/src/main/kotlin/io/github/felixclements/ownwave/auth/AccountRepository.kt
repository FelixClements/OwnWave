package io.github.felixclements.ownwave.auth

import io.github.felixclements.ownwave.api.models.LoginDeviceRequest
import io.github.felixclements.ownwave.data.ApiFactory
import kotlinx.serialization.SerializationException
import java.io.IOException

sealed interface LoginResult {
    data class Success(val session: Session) : LoginResult
    data object InvalidUrl : LoginResult
    data object InsecureUrl : LoginResult
    data object BadCredentials : LoginResult
    data object RateLimited : LoginResult

    /** A redirect or a non-OwnWave page, e.g. a login gate in front of the server. */
    data object NotOwnWave : LoginResult
    data class ServerError(val code: Int) : LoginResult
    data class Network(val message: String?) : LoginResult
}

/** Signs this device in and out. Saving the session is the caller's job. */
class AccountRepository(
    private val apis: ApiFactory,
    private val allowHttp: Boolean,
) {
    suspend fun login(serverInput: String, username: String, password: String, deviceName: String): LoginResult {
        val server = when (val parsed = ServerUrl.parse(serverInput, allowHttp)) {
            is ServerUrl.Result.Ok -> parsed.url
            ServerUrl.Result.Invalid -> return LoginResult.InvalidUrl
            ServerUrl.Result.InsecureNotAllowed -> return LoginResult.InsecureUrl
        }
        val response = try {
            apis.forServer(server).auth.loginDevice(
                LoginDeviceRequest(username = username.trim(), password = password, deviceName = deviceName.take(80)),
            )
        } catch (e: SerializationException) {
            return LoginResult.NotOwnWave
        } catch (e: IOException) {
            return LoginResult.Network(e.message)
        }
        val body = response.body()
        return when {
            response.isSuccessful && body != null -> LoginResult.Success(
                Session(
                    server = server,
                    token = body.token,
                    userId = body.user.id,
                    username = body.user.username,
                    isAdmin = body.user.isAdmin,
                ),
            )
            response.isSuccessful -> LoginResult.NotOwnWave
            response.code() in 300..399 -> LoginResult.NotOwnWave
            response.code() == 401 -> LoginResult.BadCredentials
            response.code() == 429 -> LoginResult.RateLimited
            else -> LoginResult.ServerError(response.code())
        }
    }

    /** Revokes the device token on the server when it can; signing out locally never fails. */
    suspend fun logout(session: Session) {
        runCatching { apis.forServer(session.server).auth.logout() }
    }
}
