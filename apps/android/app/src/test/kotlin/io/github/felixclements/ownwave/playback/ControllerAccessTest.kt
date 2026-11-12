package io.github.felixclements.ownwave.playback

import androidx.media3.common.Player
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ControllerAccessTest {
    private val own = "io.github.felixclements.ownwave"

    private fun trusted(pkg: String, systemTrusted: Boolean = false, carHost: Boolean = false) =
        ControllerAccess.isTrusted(pkg, own, systemTrusted, carHost)

    @Test fun `own app and car hosts are trusted`() {
        assertTrue(trusted(own))
        assertTrue(trusted("com.google.android.projection.gearhead"))
        assertTrue(trusted("com.android.car.media"))
        assertTrue(trusted("com.example.anything", carHost = true))
    }

    @Test fun `system trusted controllers are trusted`() {
        assertTrue(trusted("com.android.systemui", systemTrusted = true))
        assertTrue(trusted("com.android.bluetooth", systemTrusted = true))
    }

    @Test fun `other apps are not trusted`() {
        assertFalse(trusted("com.evil.app"))
        assertFalse(trusted("$own.evil"))
        assertFalse(trusted("android.media.session.MediaController"))
    }

    @Test fun `untrusted controllers get transport only`() {
        val commands = ControllerAccess.TRANSPORT_COMMANDS
        for (allowed in listOf(Player.COMMAND_PLAY_PAUSE, Player.COMMAND_SEEK_TO_NEXT, Player.COMMAND_GET_METADATA)) {
            assertTrue("missing $allowed", allowed in commands)
        }
        for (denied in listOf(
            Player.COMMAND_GET_TIMELINE,
            Player.COMMAND_SET_MEDIA_ITEM,
            Player.COMMAND_CHANGE_MEDIA_ITEMS,
            Player.COMMAND_SET_PLAYLIST_METADATA,
        )) {
            assertFalse("should not allow $denied", denied in commands)
        }
    }
}
