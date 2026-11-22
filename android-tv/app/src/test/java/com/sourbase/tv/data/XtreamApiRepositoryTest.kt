package com.sourbase.tv.data

import java.net.URI
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class XtreamApiRepositoryTest {
    private val credentials = XtreamCredentials(
        baseUrl = "https://provider.example.test:8443/portal/",
        username = "user name",
        password = "p@ss&word"
    )

    @Test
    fun buildsPlayerApiUrlWithEncodedCredentialsAndActions() {
        val uri = buildXtreamApiUri(
            credentials,
            action = "get_vod_streams",
            categoryId = "42"
        )

        assertEquals("https", uri.scheme)
        assertEquals("provider.example.test", uri.host)
        assertEquals(8443, uri.port)
        assertEquals("/portal/player_api.php", uri.path)
        assertEquals(
            "username=user+name&password=p%40ss%26word&action=get_vod_streams&category_id=42",
            uri.rawQuery
        )
    }

    @Test
    fun buildsLiveAndMovieStreamPathsWithoutLeakingCredentialsIntoAuthority() {
        val live = URI(buildXtreamStreamUri(credentials, "live", "123", "m3u8"))
        val movie = URI(buildXtreamStreamUri(credentials, "movie", "456", ".mp4"))

        assertEquals("provider.example.test", live.host)
        assertEquals("/portal/live/user%20name/p%40ss%26word/123.m3u8", live.rawPath)
        assertEquals("/portal/movie/user%20name/p%40ss%26word/456.mp4", movie.rawPath)
        assertEquals(null, live.rawUserInfo)
        assertEquals(null, live.rawQuery)
    }

    @Test
    fun normalizesTrailingSlashAndUsesProviderSubpath() {
        val uri = buildXtreamApiUri(
            credentials.copy(baseUrl = "https://provider.example.test/panel/"),
            action = "get_series"
        )

        assertEquals("/panel/player_api.php", uri.path)
    }

    @Test
    fun rejectsInvalidBaseUrlsAndActionValues() {
        listOf(
            "",
            "ftp://provider.example.test",
            "https:///missing-host",
            "https://user:password@provider.example.test",
            "https://provider.example.test?token=secret",
            "https://provider.example.test#section"
        ).forEach { value ->
            assertThrows(IllegalArgumentException::class.java) {
                buildXtreamApiUri(credentials.copy(baseUrl = value), action = "get_live_streams")
            }
        }

        assertThrows(IllegalArgumentException::class.java) {
            buildXtreamApiUri(credentials, action = "get_live_streams&password=bad")
        }
    }

    @Test
    fun rejectsInvalidStreamIdsAndExtensions() {
        assertThrows(IllegalArgumentException::class.java) {
            buildXtreamStreamUri(credentials, "movie", "../123", "mp4")
        }

        val uri = URI(buildXtreamStreamUri(credentials, "movie", "123", "../mp4"))
        assertEquals("/portal/movie/user%20name/p%40ss%26word/123.mp4", uri.rawPath)
    }
}
