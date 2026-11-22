package com.sourbase.tv.data

/**
 * Client-side data contracts for the shared catalogue layer.
 * These models do not imply a remote account or cloud backend.
 */
enum class MediaKind {
    LIVE_CHANNEL,
    MOVIE,
    SERIES,
    EPISODE,
    LOCAL_VIDEO
}

data class PlaybackSource(
    val url: String,
    val mimeType: String? = null,
    val headers: Map<String, String> = emptyMap()
)

data class MediaEntry(
    val id: String,
    val title: String,
    val kind: MediaKind,
    val description: String? = null,
    val artworkUrl: String? = null,
    val playback: PlaybackSource? = null,
    val durationMs: Long? = null
)

data class TvChannel(
    val id: String,
    val name: String,
    val stream: PlaybackSource,
    val logoUrl: String? = null,
    val group: String? = null,
    /** The XMLTV channel id from tvg-id / XMLTV channel id, when available. */
    val xmltvId: String? = null
)

data class EpgProgramme(
    val id: String,
    val channelId: String,
    val title: String,
    val description: String? = null,
    val startEpochMs: Long,
    val endEpochMs: Long,
    val categories: List<String> = emptyList(),
    val language: String? = null
)

data class M3uParseResult(
    val channels: List<TvChannel>,
    val warnings: List<String>
)

data class XmltvChannel(
    val id: String,
    val displayNames: List<String>,
    val logoUrl: String? = null
) {
    val displayName: String
        get() = displayNames.firstOrNull().orEmpty().ifBlank { id }
}

data class XmltvGuide(
    val channels: List<XmltvChannel>,
    val programmes: List<EpgProgramme>,
    val warnings: List<String>
) {
    /** XMLTV ids are matched through the playlist's tvg-id first, then its stable channel id. */
    fun programmesFor(channel: TvChannel): List<EpgProgramme> {
        val xmltvChannelId = channel.xmltvId?.takeIf(String::isNotBlank) ?: channel.id
        return programmes
            .asSequence()
            .filter { it.channelId == xmltvChannelId }
            .sortedBy(EpgProgramme::startEpochMs)
            .toList()
    }
}
