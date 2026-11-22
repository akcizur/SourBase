package com.sourbase.tv.data

import java.net.URI
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class RemoteSourceRedirectTest {
    private val httpsSource = URI("https://media.example.test/feeds/live.m3u?token=secret")
    private val httpSource = URI("http://media.example.test/feeds/live.m3u")

    @Test
    fun resolvesRelativeRedirectsAgainstCurrentUrl() {
        val target = resolveRemoteSourceRedirect(httpsSource, "../epg/guide.xml")

        assertEquals("https", target.scheme)
        assertEquals("media.example.test", target.host)
        assertEquals("/epg/guide.xml", target.path)
    }

    @Test
    fun acceptsAbsoluteAndProtocolRelativeHttpsRedirects() {
        val absolute = resolveRemoteSourceRedirect(
            httpsSource,
            "https://cdn.example.test/playlist.m3u"
        )
        val protocolRelative = resolveRemoteSourceRedirect(
            httpsSource,
            "//cdn.example.test/guide.xml"
        )

        assertEquals("cdn.example.test", absolute.host)
        assertEquals("https", protocolRelative.scheme)
        assertEquals("cdn.example.test", protocolRelative.host)
    }

    @Test
    fun allowsHttpSourceToUpgradeToHttps() {
        val target = resolveRemoteSourceRedirect(
            httpSource,
            "https://media.example.test/secure/live.m3u"
        )

        assertEquals("https", target.scheme)
    }

    @Test
    fun rejectsHttpsDowngradeToHttp() {
        assertThrows(IllegalArgumentException::class.java) {
            resolveRemoteSourceRedirect(httpsSource, "http://media.example.test/live.m3u")
        }
    }

    @Test
    fun rejectsDowngradeAfterAnUpgradeInTheRedirectChain() {
        val upgraded = resolveRemoteSourceRedirect(
            httpSource,
            "https://media.example.test/secure/live.m3u"
        )

        assertThrows(IllegalArgumentException::class.java) {
            resolveRemoteSourceRedirect(upgraded, "http://media.example.test/fallback.m3u")
        }
    }

    @Test
    fun rejectsNonHttpSchemesMissingHostsAndEmbeddedCredentials() {
        listOf(
            "file:///sdcard/private.m3u",
            "https:///missing-host.m3u",
            "https://user:password@media.example.test/live.m3u"
        ).forEach { location ->
            assertThrows(IllegalArgumentException::class.java) {
                resolveRemoteSourceRedirect(httpSource, location)
            }
        }
    }

    @Test
    fun rejectsEmptyAndMalformedLocations() {
        listOf("", "   ", "https://[invalid-host").forEach { location ->
            assertThrows(IllegalArgumentException::class.java) {
                resolveRemoteSourceRedirect(httpsSource, location)
            }
        }
    }
}
