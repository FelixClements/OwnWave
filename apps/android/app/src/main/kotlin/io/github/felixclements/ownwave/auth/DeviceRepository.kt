package io.github.felixclements.ownwave.auth

import io.github.felixclements.ownwave.api.models.Device
import io.github.felixclements.ownwave.data.ApiFactory
import kotlinx.serialization.SerializationException
import java.io.IOException
import java.util.UUID

/**
 * The account's signed-in devices (`GET/DELETE /api/me/devices`). Failures are returned, never retried.
 *
 * This device is never revoked here: signing it out goes through [AccountRepository.logout],
 * which also forgets the local session.
 */
class DeviceRepository(private val apis: ApiFactory) {

    /** This device first, then the others in server order (most recently used first). */
    suspend fun devices(session: Session): Result<List<Device>> = try {
        val response = apis.forServer(session.server).auth.listDevices()
        val body = response.body()
        if (response.isSuccessful && body != null) {
            Result.success(body.devices.sortedByDescending { it.current })
        } else {
            Result.failure(IOException("HTTP ${response.code()}"))
        }
    } catch (e: IOException) {
        Result.failure(e)
    } catch (e: SerializationException) {
        Result.failure(e)
    }

    /** Signs another device out. A 404 means it is already gone, which counts as success. */
    suspend fun revoke(session: Session, device: Device): Result<Unit> {
        if (device.current) return Result.failure(IllegalArgumentException("use sign out for this device"))
        val id = runCatching { UUID.fromString(device.id) }.getOrElse { return Result.failure(it) }
        return try {
            val response = apis.forServer(session.server).auth.revokeDevice(id)
            if (response.isSuccessful || response.code() == 404) {
                Result.success(Unit)
            } else {
                Result.failure(IOException("HTTP ${response.code()}"))
            }
        } catch (e: IOException) {
            Result.failure(e)
        }
    }
}
