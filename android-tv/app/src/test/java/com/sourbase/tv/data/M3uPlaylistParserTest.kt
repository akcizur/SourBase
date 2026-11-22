package com.sourbase.tv.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.assertThrows
import org.junit.Test

class M3uPlaylistParserTest {
    @Test
    fun parsesQuotedAttributesGroupsAndStableIds() {
        val source = """
            #EXTM3U
            #EXTINF:-1 tvg-id="news.one" tvg-name="News One HD" tvg-logo="https://img.example.test/news.png" group-title="News",News One
            https://media.example.test/live/news.m3u8?token=abc
            #EXTINF:-1 group-title="Sports",Sport 2
            http://media.example.test/live/sport2
        """.trimIndent()

        val result = M3uPlaylistParser.parse(source)

        assertEquals(2, result.channels.size)
        assertEquals("tvg:news.one", result.channels[0].id)
        assertEquals("news.one", result.channels[0].xmltvId)
        assertEquals("News One", result.channels[0].name)
        assertEquals("News", result.channels[0].group)
        assertEquals("https://img.example.test/news.png", result.channels[0].logoUrl)
        assertEquals("https://media.example.test/live/news.m3u8?token=abc", result.channels[0].stream.url)
        assertEquals("Sport 2", result.channels[1].name)
        assertEquals("Sports", result.channels[1].group)
    }

    @Test
    fun reportsAndSkipsInvalidStreamEntries() {
        val source = """
            #EXTM3U
            #EXTINF:-1,Relative URL
            /not/a/stream
            #EXTINF:-1,Valid channel
            https://media.example.test/live.m3u8
        """.trimIndent()

        val result = M3uPlaylistParser.parse(source)

        assertEquals(1, result.channels.size)
        assertTrue(result.warnings.isNotEmpty())
    }

    @Test
    fun rejectsCredentialsInsideStreamAuthority() {
        val source = """
            #EXTM3U
            #EXTINF:-1,Private channel
            https://user:pass@media.example.test/live.m3u8
        """.trimIndent()

        val error = assertThrows(IllegalArgumentException::class.java) {
            M3uPlaylistParser.parse(source)
        }

        assertTrue(error.message.orEmpty().contains("No valid"))
    }

    @Test
    fun generatesDeterministicFallbackIds() {
        val source = """
            #EXTM3U
            #EXTINF:-1,Channel
            https://media.example.test/live.m3u8
        """.trimIndent()

        val first = M3uPlaylistParser.parse(source).channels.single()
        val second = M3uPlaylistParser.parse(source).channels.single()

        assertEquals(first.id, second.id)
        assertFalse(first.id.isBlank())
    }

    @Test
    fun duplicateTvgIdsAndRepeatedStreamsReceiveDeterministicUniqueIds() {
        val source = """
            #EXTM3U
            #EXTINF:-1 tvg-id="news.one",News One
            https://media.example.test/live/news.m3u8
            #EXTINF:-1 tvg-id="news.one",News One (backup)
            https://media.example.test/live/news.m3u8
            #EXTINF:-1 tvg-id="news.one",News One (alternate)
            https://backup.example.test/live/news.m3u8
        """.trimIndent()

        val firstParse = M3uPlaylistParser.parse(source).channels
        val secondParse = M3uPlaylistParser.parse(source).channels

        assertEquals(3, firstParse.size)
        assertEquals(3, firstParse.map(TvChannel::id).toSet().size)
        assertEquals(firstParse.map(TvChannel::id), secondParse.map(TvChannel::id))
        assertEquals("tvg:news.one", firstParse.first().id)
        assertTrue(firstParse.drop(1).all { it.id.startsWith("tvg:news.one:") })
    }

    @Test
    fun ignoresHlsMediaSegmentsWithoutChannelMetadata() {
        val mediaPlaylist = """
            #EXTM3U
            #EXT-X-TARGETDURATION:6
            #EXTINF:6.0,
            segment-001.ts
        """.trimIndent()

        assertThrows(IllegalArgumentException::class.java) {
            M3uPlaylistParser.parse(mediaPlaylist)
        }
    }
}
