package com.sourbase.tv.playback

import com.sourbase.tv.data.PlaybackSource
import java.net.URI
import java.util.Locale

/**
 * Validates user-supplied playback metadata before creating a Media3 data source.
 * This deliberately accepts only absolute HTTP(S) URLs; local file paths and
 * implicit/relative URLs must use a separate, explicit source implementation.
 */
internal fun validatePlaybackSource(source: PlaybackSource): URI {
    val rawUrl = source.url.trim()
    require(rawUrl.isNotEmpty()) { "Playback URL must not be empty." }

    val uri = try {
        URI(rawUrl)
    } catch (error: Exception) {
        throw IllegalArgumentException("Playback URL is malformed.", error)
    }

    val scheme = uri.scheme?.lowercase(Locale.ROOT)
    require(scheme == "http" || scheme == "https") {
        "Playback URL must use HTTP or HTTPS."
    }
    require(!uri.host.isNullOrBlank()) { "Playback URL must contain a host." }
    require(uri.rawUserInfo == null) {
        "Credentials must not be embedded in the URL authority; use source credentials or request headers."
    }

    validateMimeType(source.mimeType)
    validateRequestHeaders(source.headers)

    return uri
}

private fun validateMimeType(mimeType: String?) {
    val value = mimeType?.trim()?.takeIf(String::isNotEmpty) ?: return
    require('/' in value && value.length <= MAX_MIME_TYPE_LENGTH) {
        "MIME type must be a short type/subtype value."
    }
    require('\r' !in value && '\n' !in value) {
        "MIME type must not contain line breaks."
    }
}

private fun validateRequestHeaders(headers: Map<String, String>) {
    headers.forEach { (name, value) ->
        require(name.isNotBlank() && name.all(::isHeaderNameCharacter)) {
            "Request header name is invalid."
        }
        require('\r' !in value && '\n' !in value) {
            "Request header values must not contain line breaks."
        }
    }
}

private fun isHeaderNameCharacter(char: Char): Boolean =
    char.isLetterOrDigit() || char in HEADER_NAME_EXTRA_CHARACTERS

private const val MAX_MIME_TYPE_LENGTH = 127
private const val HEADER_NAME_EXTRA_CHARACTERS = "!#$%&'*+-.^_|~"
