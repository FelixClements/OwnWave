package io.github.felixclements.ownwave.playback

import android.os.Bundle
import androidx.media3.session.SessionCommand

/** Custom session commands for like, ban and hide, shared by the service, the phone UI and (later) Auto. */
object Commands {
    const val ACTION_LIKE = "io.github.felixclements.ownwave.LIKE"
    const val ACTION_BAN = "io.github.felixclements.ownwave.BAN"
    const val ACTION_HIDE = "io.github.felixclements.ownwave.HIDE"

    val LIKE = SessionCommand(ACTION_LIKE, Bundle.EMPTY)
    val BAN = SessionCommand(ACTION_BAN, Bundle.EMPTY)

    /** Takes the current track off the station it plays from (not other stations), and skips it. */
    val HIDE = SessionCommand(ACTION_HIDE, Bundle.EMPTY)

    /** Session extras: the current track id and whether it is liked. */
    const val EXTRA_TRACK_ID = "track_id"
    const val EXTRA_LIKED = "liked"
}
