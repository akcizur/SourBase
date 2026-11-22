package com.sourbase.tv.data

import java.io.ByteArrayInputStream
import java.io.IOException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class RemoteSourceDocumentTest {
    @Test
    fun decodesUtf8AndStripsByteOrderMark() {
        val content = "\uFEFF#EXTM3U\n#EXTINF:-1,Český kanál\nhttps://media.example.test/live.m3u8"
        val bytes = content.toByteArray(Charsets.UTF_8)

        val decoded = readBoundedSourceText(ByteArrayInputStream(bytes), bytes.size)

        assertEquals(content.removePrefix("\uFEFF"), decoded)
    }

    @Test
    fun acceptsContentAtTheExactByteLimit() {
        val bytes = "žluťoučký".toByteArray(Charsets.UTF_8)

        val decoded = readBoundedSourceText(ByteArrayInputStream(bytes), bytes.size)

        assertEquals("žluťoučký", decoded)
    }

    @Test
    fun rejectsContentOverTheByteLimit() {
        val bytes = "playlist-data".toByteArray(Charsets.UTF_8)

        assertThrows(IOException::class.java) {
            readBoundedSourceText(ByteArrayInputStream(bytes), bytes.size - 1)
        }
    }

    @Test
    fun rejectsNonPositiveLimits() {
        assertThrows(IllegalArgumentException::class.java) {
            readBoundedSourceText(ByteArrayInputStream(byteArrayOf(1)), 0)
        }
    }
}
