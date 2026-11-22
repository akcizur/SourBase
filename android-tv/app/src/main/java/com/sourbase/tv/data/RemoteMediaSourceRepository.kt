package com.sourbase.tv.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URI
import java.nio.charset.StandardCharsets

/**
 * Fetches user-provided remote playlist and guide files off the UI thread.
 *
 * Source URLs are deliberately kept in memory by the current TV screen only.
 * They are not persisted until encrypted Android Keystore-backed source storage
 * is implemented.
 */
class RemoteMediaSourceRepository(
    private val connectTimeoutMs: Int = 15_000,
    private val readTimeoutMs: Int = 30_000
) {
    suspend fun loadM3u(url: String): M3uParseResult = withContext(Dispatchers.IO) {
        M3uPlaylistParser.parse(fetchText(url, MAX_M3U_BYTES))
    }

    suspend fun loadXmltv(url: String): XmltvGuide = withContext(Dispatchers.IO) {
        XmltvGuideParser.parse(fetchText(url, MAX_XMLTV_BYTES))
    }

    private fun fetchText(rawUrl: String, maxBytes: Int): String {
        val uri = validateSourceUrl(rawUrl)
        val connection = (uri.toURL().openConnection() as? HttpURLConnection)
            ?: throw IOException("Source URL is not an HTTP connection.")

        try {
            connection.connectTimeout = connectTimeoutMs
            connection.readTimeout = readTimeoutMs
            connection.instanceFollowRedirects = true
            connection.requestMethod = "GET"
            connection.setRequestProperty("Accept", "*/*")
            connection.setRequestProperty("Accept-Encoding", "identity")
            connection.setRequestProperty("User-Agent", "Sourbase-AndroidTV/0.1")

            val responseCode = connection.responseCode
            if (responseCode !in 200..299) {
                throw IOException("Source server returned HTTP $responseCode.")
            }

            val declaredLength = connection.contentLengthLong
            if (declaredLength > maxBytes) {
                throw IOException("Source is larger than the allowed $maxBytes byte limit.")
            }

            val output = ByteArrayOutputStream(
                if (declaredLength in 1..maxBytes.toLong()) declaredLength.toInt() else 8 * 1024
            )
            connection.inputStream.use { input ->
                val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                var totalBytes = 0
                while (true) {
                    val read = input.read(buffer)
                    if (read < 0) break
                    totalBytes += read
                    if (totalBytes > maxBytes) {
                        throw IOException("Source exceeded the allowed $maxBytes byte limit.")
                    }
                    output.write(buffer, 0, read)
                }
            }

            return output.toByteArray().toString(StandardCharsets.UTF_8)
                .removePrefix("\uFEFF")
        } finally {
            connection.disconnect()
        }
    }

    private fun validateSourceUrl(rawUrl: String): URI {
        val value = rawUrl.trim()
        require(value.isNotEmpty()) { "Source URL must not be empty." }
        val uri = try {
            URI(value)
        } catch (error: Exception) {
            throw IllegalArgumentException("Source URL is malformed.", error)
        }
        val scheme = uri.scheme?.lowercase()
        require(scheme == "http" || scheme == "https") {
            "Source URL must use HTTP or HTTPS."
        }
        require(!uri.host.isNullOrBlank()) { "Source URL must contain a host." }
        require(uri.rawUserInfo == null) {
            "Do not put credentials in the URL authority. Use provider request headers instead."
        }
        return uri
    }

    private companion object {
        const val MAX_M3U_BYTES = 32 * 1024 * 1024
        const val MAX_XMLTV_BYTES = 64 * 1024 * 1024
    }
}
