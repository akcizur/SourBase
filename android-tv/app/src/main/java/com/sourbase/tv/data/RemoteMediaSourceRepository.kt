package com.sourbase.tv.data

import android.content.Context
import android.net.Uri
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URI
import java.nio.charset.StandardCharsets

/**
 * Fetches user-provided remote playlist and guide files off the UI thread.
 *
 * The UI persists source URLs separately through SecureSourceStore. This
 * repository never logs URLs or response bodies because they may contain tokens.
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

    suspend fun loadM3u(context: Context, uri: Uri): M3uParseResult = withContext(Dispatchers.IO) {
        M3uPlaylistParser.parse(readDocumentText(context, uri, MAX_M3U_BYTES))
    }

    suspend fun loadXmltv(context: Context, uri: Uri): XmltvGuide = withContext(Dispatchers.IO) {
        XmltvGuideParser.parse(readDocumentText(context, uri, MAX_XMLTV_BYTES))
    }

    private fun readDocumentText(context: Context, uri: Uri, maxBytes: Int): String {
        require(uri.scheme == "content") {
            "Selected document must be provided through Android's document picker."
        }
        val input = context.contentResolver.openInputStream(uri)
            ?: throw IOException("The selected document could not be opened.")
        return readBoundedSourceText(input, maxBytes)
    }

    private fun fetchText(rawUrl: String, maxBytes: Int): String {
        var currentUri = validateSourceUrl(rawUrl)
        var redirectCount = 0

        while (true) {
            val connection = (currentUri.toURL().openConnection() as? HttpURLConnection)
                ?: throw IOException("Source URL is not an HTTP connection.")

            try {
                connection.connectTimeout = connectTimeoutMs
                connection.readTimeout = readTimeoutMs
                // Redirects are handled manually so HTTPS cannot silently downgrade to HTTP.
                connection.instanceFollowRedirects = false
                connection.requestMethod = "GET"
                connection.setRequestProperty("Accept", "*/*")
                connection.setRequestProperty("Accept-Encoding", "identity")
                connection.setRequestProperty("User-Agent", "Sourbase-AndroidTV/0.1")

                val responseCode = connection.responseCode
                if (responseCode in REDIRECT_CODES) {
                    if (redirectCount >= MAX_REDIRECTS) {
                        throw IOException("Source server exceeded the redirect limit.")
                    }
                    val location = connection.getHeaderField("Location")
                        ?: throw IOException("Source server returned a redirect without a Location header.")
                    currentUri = try {
                        resolveRemoteSourceRedirect(currentUri, location)
                    } catch (error: IllegalArgumentException) {
                        throw IOException("Source server returned an invalid or unsafe redirect.", error)
                    }
                    redirectCount++
                    continue
                }

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
    }

    private companion object {
        val REDIRECT_CODES = setOf(301, 302, 303, 307, 308)
        const val MAX_REDIRECTS = 5
        const val MAX_M3U_BYTES = 32 * 1024 * 1024
        const val MAX_XMLTV_BYTES = 64 * 1024 * 1024
    }
}

/**
 * Reads a selected source with a strict byte limit, decodes UTF-8 and strips a BOM.
 * The helper owns and closes the provided stream.
 */
internal fun readBoundedSourceText(input: InputStream, maxBytes: Int): String {
    require(maxBytes > 0) { "Maximum source size must be positive." }

    val output = ByteArrayOutputStream(8 * 1024)
    input.use { stream ->
        val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
        var totalBytes = 0
        while (true) {
            val read = stream.read(buffer)
            if (read < 0) break
            if (read == 0) continue
            totalBytes += read
            if (totalBytes > maxBytes) {
                throw IOException("Selected document exceeded the allowed $maxBytes byte limit.")
            }
            output.write(buffer, 0, read)
        }
    }
    return output.toByteArray().toString(StandardCharsets.UTF_8).removePrefix("\uFEFF")
}

/**
 * Resolves a redirect relative to the current source and validates its target.
 *
 * HTTPS may redirect to another HTTPS URL, but once a chain is HTTPS it cannot
 * downgrade to HTTP. This prevents a playlist token in the URL from being sent
 * over a weaker transport after an unexpected redirect.
 */
internal fun resolveRemoteSourceRedirect(currentUri: URI, location: String): URI {
    val value = location.trim()
    require(value.isNotEmpty()) { "Redirect Location must not be empty." }

    val locationUri = try {
        URI(value)
    } catch (error: Exception) {
        throw IllegalArgumentException("Redirect Location is malformed.", error)
    }

    val target = try {
        currentUri.resolve(locationUri)
    } catch (error: Exception) {
        throw IllegalArgumentException("Redirect target could not be resolved.", error)
    }

    val scheme = target.scheme?.lowercase()
    require(scheme == "http" || scheme == "https") {
        "Redirect target must use HTTP or HTTPS."
    }
    require(!target.host.isNullOrBlank()) { "Redirect target must contain a host." }
    require(target.rawUserInfo == null) {
        "Redirect target must not contain credentials in its URL authority."
    }
    require(
        !currentUri.scheme.equals("https", ignoreCase = true) ||
            scheme == "https"
    ) {
        "An HTTPS source cannot redirect to an insecure HTTP URL."
    }

    return target
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
