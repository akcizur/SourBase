package com.sourbase.tv.data

/**
 * Client-side data contracts for the future shared catalog layer.
 * These models do not yet imply a remote account or cloud backend.
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
    val group: String? = null
)

data class EpgProgramme(
    val id: String,
    val channelId: String,
    val title: String,
    val description: String? = null,
    val startEpochMs: Long,
    val endEpochMs: Long
)
