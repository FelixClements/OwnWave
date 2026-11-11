package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.Quality
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class QualityPolicyTest {
    @Test fun `defaults are high on wifi and normal on mobile`() {
        assertEquals(Quality.HIGH, QualityPolicy.choose(metered = false, QualitySettings()))
        assertEquals(Quality.NORMAL, QualityPolicy.choose(metered = true, QualitySettings()))
    }

    @Test fun `settings are used per network`() {
        val settings = QualitySettings(wifi = Quality.ORIGINAL, mobile = Quality.LOW)
        assertEquals(Quality.ORIGINAL, QualityPolicy.choose(metered = false, settings))
        assertEquals(Quality.LOW, QualityPolicy.choose(metered = true, settings))
    }

    @Test fun `original is never used on mobile data`() {
        assertFalse(Quality.ORIGINAL in QualityPolicy.mobileOptions)
        assertEquals(Quality.NORMAL, QualityPolicy.choose(metered = true, QualitySettings(mobile = Quality.ORIGINAL)))
    }
}
