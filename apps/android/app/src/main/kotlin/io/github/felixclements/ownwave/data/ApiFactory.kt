package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.api.apis.AuthApi
import io.github.felixclements.ownwave.api.apis.PlaybackApi
import io.github.felixclements.ownwave.api.apis.StationsApi
import io.github.felixclements.ownwave.api.infrastructure.Serializer
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import retrofit2.converter.scalars.ScalarsConverterFactory

/** Builds the generated Retrofit APIs for one server, sharing the app's OkHttp client. */
class ApiFactory(private val client: OkHttpClient) {

    class Apis(val auth: AuthApi, val stations: StationsApi, val playback: PlaybackApi)

    @Volatile private var cached: Pair<HttpUrl, Apis>? = null

    fun forServer(server: HttpUrl): Apis {
        cached?.let { (url, apis) -> if (url == server) return apis }
        val retrofit = Retrofit.Builder()
            .baseUrl(server)
            .client(client)
            .addConverterFactory(ScalarsConverterFactory.create())
            .addConverterFactory(Serializer.kotlinxSerializationJson.asConverterFactory("application/json".toMediaType()))
            .build()
        return Apis(
            auth = retrofit.create(AuthApi::class.java),
            stations = retrofit.create(StationsApi::class.java),
            playback = retrofit.create(PlaybackApi::class.java),
        ).also { cached = server to it }
    }
}
