package com.sourbase.tv.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class XmltvGuideParserTest {
    @Test
    fun parsesChannelsProgrammesAndTimeOffsets() {
        val xml = """
            <?xml version="1.0" encoding="UTF-8"?>
            <tv>
              <channel id="news.one">
                <display-name lang="en">News One</display-name>
                <display-name>News One HD</display-name>
                <icon src="https://img.example.test/news.png"/>
              </channel>
              <programme start="20261122100000 +0100" stop="20261122103000 +0100" channel="news.one">
                <title lang="en">Morning Briefing</title>
                <desc>Top stories of the day.</desc>
                <category>News</category>
                <category>Magazine</category>
              </programme>
            </tv>
        """.trimIndent()

        val guide = XmltvGuideParser.parse(xml)
        val programme = guide.programmes.single()

        assertEquals("news.one", guide.channels.single().id)
        assertEquals("News One", guide.channels.single().displayName)
        assertEquals("https://img.example.test/news.png", guide.channels.single().logoUrl)
        assertEquals("Morning Briefing", programme.title)
        assertEquals("en", programme.language)
        assertEquals("Top stories of the day.", programme.description)
        assertEquals(listOf("News", "Magazine"), programme.categories)
        assertEquals(1_795_333_200_000L, programme.startEpochMs)
        assertEquals(1_795_335_000_000L, programme.endEpochMs)
    }

    @Test
    fun interpretsTimezoneMissingTimestampAsUtcAndSkipsInvalidProgramme() {
        val xml = """
            <tv>
              <channel id="ch1"><display-name>Channel One</display-name></channel>
              <programme start="20261122090000" stop="20261122100000" channel="ch1">
                <title>UTC programme</title>
              </programme>
              <programme start="20261122100000 +0100" stop="20261122090000 +0100" channel="ch1">
                <title>Invalid range</title>
              </programme>
            </tv>
        """.trimIndent()

        val guide = XmltvGuideParser.parse(xml)

        assertEquals(1, guide.programmes.size)
        assertEquals(1_795_329_200_000L, guide.programmes.single().startEpochMs)
        assertTrue(guide.warnings.isNotEmpty())
    }

    @Test
    fun matchesGuideByTvgIdRatherThanDisplayName() {
        val xml = """
            <tv>
              <channel id="news.one"><display-name>Totally Different Label</display-name></channel>
              <programme start="20261122090000 +0000" stop="20261122100000 +0000" channel="news.one">
                <title>Live News</title>
              </programme>
              <programme start="20261122090000 +0000" stop="20261122100000 +0000" channel="other">
                <title>Other Channel</title>
              </programme>
            </tv>
        """.trimIndent()
        val guide = XmltvGuideParser.parse(xml)
        val channel = TvChannel(
            id = "stream:channel-1",
            name = "News One",
            stream = PlaybackSource("https://media.example.test/news.m3u8"),
            xmltvId = "news.one"
        )

        assertEquals("Live News", guide.programmesFor(channel).single().title)
    }

    @Test
    fun rejectsMalformedXmlAndExternalEntityExpansion() {
        assertTrue(runCatching { XmltvGuideParser.parse("<tv><programme>") }.isFailure)

        val externalEntity = """
            <!DOCTYPE tv [ <!ENTITY xxe SYSTEM "file:///etc/passwd"> ]>
            <tv>
              <channel id="ch1"><display-name>&xxe;</display-name></channel>
            </tv>
        """.trimIndent()
        assertTrue(runCatching { XmltvGuideParser.parse(externalEntity) }.isFailure)
    }

    @Test
    fun rejectsWrongRootElement() {
        assertTrue(runCatching { XmltvGuideParser.parse("<playlist/>") }.isFailure)
    }
}
