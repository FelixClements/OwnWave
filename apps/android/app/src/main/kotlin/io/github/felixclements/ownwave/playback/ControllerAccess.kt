package io.github.felixclements.ownwave.playback

import androidx.media3.common.Player

/**
 * Which media controllers get the full session. The service is exported (it has to be, for the
 * notification, Bluetooth, Android Auto and the system), so any app on the phone can connect.
 * Only trusted controllers may browse the library, change the queue, or send like and ban; every
 * other app gets transport controls and the current track, like a lock screen.
 */
internal object ControllerAccess {

    /** Car hosts that browse stations: Android Auto on the phone, and Android Automotive's media center. */
    val CAR_PACKAGES = setOf(
        "com.google.android.projection.gearhead",
        "com.android.car.media",
    )

    /**
     * @param systemTrusted Media3's `ControllerInfo.isTrusted`: the controller holds
     *   MEDIA_CONTENT_CONTROL or is an enabled notification listener (System UI, Bluetooth, Assistant).
     * @param carHost Media3 recognised the controller as Android Auto or Automotive.
     */
    fun isTrusted(packageName: String, ownPackage: String, systemTrusted: Boolean, carHost: Boolean): Boolean =
        systemTrusted || carHost || packageName == ownPackage || packageName in CAR_PACKAGES

    /** Play, pause, skip and seek, plus the current track's metadata. No queue, no library, no item changes. */
    val TRANSPORT_PLAYER_COMMANDS: Player.Commands = Player.Commands.Builder()
        .addAll(
            Player.COMMAND_PLAY_PAUSE,
            Player.COMMAND_PREPARE,
            Player.COMMAND_STOP,
            Player.COMMAND_SEEK_TO_DEFAULT_POSITION,
            Player.COMMAND_SEEK_IN_CURRENT_MEDIA_ITEM,
            Player.COMMAND_SEEK_TO_PREVIOUS_MEDIA_ITEM,
            Player.COMMAND_SEEK_TO_PREVIOUS,
            Player.COMMAND_SEEK_TO_NEXT_MEDIA_ITEM,
            Player.COMMAND_SEEK_TO_NEXT,
            Player.COMMAND_SEEK_BACK,
            Player.COMMAND_SEEK_FORWARD,
            Player.COMMAND_GET_CURRENT_MEDIA_ITEM,
            Player.COMMAND_GET_METADATA,
        )
        .build()
}
