package io.github.felixclements.ownwave.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class MediaIdsTest {
    @Test fun `station ids round trip`() {
        assertEquals("s1", MediaIds.parseStation(MediaIds.station("s1")))
        assertNull(MediaIds.parseStation("track:t1|station:s1"))
        assertNull(MediaIds.parseStation("station:"))
    }

    @Test fun `track ids round trip with and without a station`() {
        assertEquals(MediaIds.TrackRef("t1", "s1"), MediaIds.parseTrack(MediaIds.track("t1", "s1")))
        assertEquals(MediaIds.TrackRef("t1", null), MediaIds.parseTrack(MediaIds.track("t1", null)))
        assertNull(MediaIds.parseTrack("station:s1"))
        assertNull(MediaIds.parseTrack("track:"))
    }

    @Test fun `track uri gives the track id back and nothing else does`() {
        assertEquals("t1", MediaIds.trackIdFromUri(MediaIds.trackUri("t1")))
        assertNull(MediaIds.trackIdFromUri("https://example.com/track/t1"))
        assertNull(MediaIds.trackIdFromUri("ownwave://track/t1/x"))
        assertNull(MediaIds.trackIdFromUri("ownwave://track/"))
    }
}
