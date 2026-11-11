package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.Quality

/** Stream quality per network type, as chosen in Settings. */
data class QualitySettings(
    val wifi: Quality = Quality.HIGH,
    val mobile: Quality = Quality.NORMAL,
)

object QualityPolicy {
    /** Qualities offered on Wi-Fi. `aac` is left out: Opus plays everywhere on Android. */
    val wifiOptions = listOf(Quality.ORIGINAL, Quality.HIGH, Quality.NORMAL, Quality.LOW)

    /** Qualities offered on mobile data. `original` can be a large lossless file, so it is Wi-Fi only. */
    val mobileOptions = listOf(Quality.HIGH, Quality.NORMAL, Quality.LOW)

    /** The quality for the network in use. A metered network (mobile data, hotspot) uses the mobile setting. */
    fun choose(metered: Boolean, settings: QualitySettings): Quality {
        val wanted = if (metered) settings.mobile else settings.wifi
        return if (metered && wanted !in mobileOptions) QualitySettings().mobile else wanted
    }
}
