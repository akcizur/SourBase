package com.sourbase.tv.data

import java.net.URI
import java.nio.charset.StandardCharsets
import java.util.UUID

/**
 * Parses an IPTV extended M3U channel playlist.
 *
 * Only absolute HTTP(S) stream URLs are accepted. HLS media playlists and
 * local file paths are not treated as channel lists.
 */
object M3uPlaylistParser {
    private const val MAX_PLAYLIST_CHARS = 32 * 1024 * 1024

    private val attributePattern = Regex(
        """([A-Za-z0-9_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^,\s]*))"""
    )

    private data class PendingChannel(
        val attributes: Map<String, String>,
        val displayName: String
    )

    fun parse(content: String): M3uParseResult {
        require(content.length <= MAX_PLAYLIST_CHARS) {
            "Playlist is too large (maximum 32 MiB of text)."
        }

        val channels = mutableListOf<TvChannel>()
        val usedChannelIds = mutableSetOf<String>()
        val warnings = mutableListOf<String>()
        var pending: PendingChannel? = null

        content.lineSequence().forEachIndexed { index, rawLine ->
            val line = rawLine.trim().removePrefix("\uFEFF")
            when {
                line.startsWith("#EXTINF:", ignoreCase = true) -> {
                    if (pending != null) {
                        warnings += "Line ${index + 1}: previous EXTINF entry had no stream URL."
                    }
                    pending = parseExtInf(line)
                    if (pending == null) {
                        warnings += "Line ${index + 1}: EXTINF entry has no display-name separator."
                    }
                }

                line.isBlank() || line.startsWith("#") -> Unit

                pending != null -> {
                    val entry = pending
                    pending = null
                    val streamUri = parseHttpUri(line)
                    if (streamUri == null) {
                        warnings += "Line ${index + 1}: unsupported or invalid stream URL; entry skipped."
                    } else {
                        val attributes = entry.attributes
                        val tvgId = attributes["tvg-id"]?.trim()?.takeIf(String::isNotEmpty)
                        val tvgName = attributes["tvg-name"]?.trim()?.takeIf(String::isNotEmpty)
                        val channelName = entry.displayName
                            .ifBlank { tvgName.orEmpty() }
                            .ifBlank { tvgId.orEmpty() }
                            .ifBlank { "Channel ${channels.size + 1}" }
                        val stableId = tvgId?.let { "tvg:$it" } ?: stableStreamId(streamUri.toString())
                        val streamSuffix = stableStreamId(streamUri.toString()).removePrefix("stream:")
                        var uniqueId = stableId
                        if (!usedChannelIds.add(uniqueId)) {
                            uniqueId = "$stableId:$streamSuffix"
                            var duplicateOrdinal = 2
                            while (!usedChannelIds.add(uniqueId)) {
                                uniqueId = "$stableId:$streamSuffix:$duplicateOrdinal"
                                duplicateOrdinal++
                            }
                        }

                        channels += TvChannel(
                            id = uniqueId,
                            name = channelName,
                            stream = PlaybackSource(url = streamUri.toString()),
                            logoUrl = attributes["tvg-logo"]?.trim()?.takeIf(::isHttpUrl),
                            group = attributes["group-title"]?.trim()?.takeIf(String::isNotEmpty)
                                ?: attributes["group"]?.trim()?.takeIf(String::isNotEmpty),
                            xmltvId = tvgId
                        )
                    }
                }
            }
        }

        if (pending != null) {
            warnings += "Final EXTINF entry has no stream URL."
        }
        if (channels.isEmpty()) {
            throw IllegalArgumentException("No valid HTTP(S) TV channels were found in the M3U playlist.")
        }

        return M3uParseResult(channels = channels, warnings = warnings)
    }

    private fun parseExtInf(line: String): PendingChannel? {
        val comma = line.indexOf(',')
        if (comma < 0) return null

        val metadata = line.substring(0, comma)
        val attributes = attributePattern.findAll(metadata)
            .associate { match ->
                val value = match.groups[2]?.value
                    ?: match.groups[3]?.value
                    ?: match.groups[4]?.value
                    .orEmpty()
                match.groupValues[1].lowercase() to value
            }

        return PendingChannel(
            attributes = attributes,
            displayName = line.substring(comma + 1).trim()
        )
    }

    private fun parseHttpUri(value: String): URI? = try {
        val uri = URI(value.trim())
        val scheme = uri.scheme?.lowercase()
        if ((scheme == "http" || scheme == "https") &&
            !uri.host.isNullOrBlank() &&
            uri.rawUserInfo == null
        ) uri else null
    } catch (_: Exception) {
        null
    }

    private fun isHttpUrl(value: String): Boolean = parseHttpUri(value) != null

    private fun stableStreamId(url: String): String =
        "stream:${UUID.nameUUIDFromBytes(url.toByteArray(StandardCharsets.UTF_8))}"
}
