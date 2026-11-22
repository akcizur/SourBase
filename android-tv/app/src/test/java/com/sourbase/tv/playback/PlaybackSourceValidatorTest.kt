package com.sourbase.tv.playback

import com.sourbase.tv.data.PlaybackSource
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class PlaybackSourceValidatorTest {
    @Test
    fun acceptsHttpsPlaylistWithQuery() {
        val result = validatePlaybackSource(
            PlaybackSource(" https://media.example.test/live/index.m3u8?token=abc ")
        )

        assertEquals("https", result.scheme)
        assertEquals("media.example.test", result.host)
        assertEquals("/live/index.m3u8", result.path)
    }

    @Test
    fun acceptsHttpProviderEndpoint() {
        val result = validatePlaybackSource(
            PlaybackSource("http://192.0.2.10:8080/movie/42.mp4")
        )

        assertEquals("http", result.scheme)
        assertEquals(8080, result.port)
    }

    @Test
    fun rejectsBlankUrl() {
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(PlaybackSource("  "))
        }
    }

    @Test
    fun rejectsRelativeAndNonHttpUrls() {
        listOf("/live.m3u8", "file:///storage/video.mp4", "javascript:alert(1)").forEach { url ->
            assertThrows(IllegalArgumentException::class.java) {
                validatePlaybackSource(PlaybackSource(url))
            }
        }
    }

    @Test
    fun rejectsUrlWithoutHost() {
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(PlaybackSource("https:///missing-host.m3u8"))
        }
    }

    @Test
    fun rejectsCredentialsInUrlAuthority() {
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(PlaybackSource("https://user:secret@example.test/live.m3u8"))
        }
    }

    @Test
    fun rejectsInvalidMimeTypeAndHeaderInjection() {
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(
                PlaybackSource("https://media.example.test/live.m3u8", mimeType = "m3u8")
            )
        }
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(
                PlaybackSource(
                    "https://media.example.test/live.m3u8",
                    headers = mapOf("X-Test\r\nInjected" to "value")
                )
            )
        }
        assertThrows(IllegalArgumentException::class.java) {
            validatePlaybackSource(
                PlaybackSource(
                    "https://media.example.test/live.m3u8",
                    headers = mapOf("Authorization" to "Bearer token\r\nInjected: yes")
                )
            )
        }
    }
}
