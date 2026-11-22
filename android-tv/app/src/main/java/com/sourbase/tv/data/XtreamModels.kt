package com.sourbase.tv.data

data class XtreamCredentials(
    val baseUrl: String,
    val username: String,
    val password: String
)

data class XtreamAccountInfo(
    val username: String,
    val status: String,
    val expirationEpochSeconds: Long?,
    val isTrial: Boolean,
    val activeConnections: Int?,
    val maximumConnections: Int?
)

data class XtreamCategory(
    val id: String,
    val name: String,
    val parentId: String? = null
)

data class XtreamLiveChannel(
    val id: String,
    val name: String,
    val categoryId: String?,
    val categoryName: String?,
    val epgChannelId: String?,
    val logoUrl: String?,
    val stream: PlaybackSource
)

data class XtreamVodItem(
    val id: String,
    val name: String,
    val categoryId: String?,
    val categoryName: String?,
    val posterUrl: String?,
    val plot: String?,
    val year: String?,
    val rating: String?,
    val stream: PlaybackSource
)

data class XtreamSeriesItem(
    val id: String,
    val name: String,
    val categoryId: String?,
    val categoryName: String?,
    val coverUrl: String?,
    val plot: String?,
    val year: String?,
    val rating: String?
)

data class XtreamEpisode(
    val id: String,
    val seasonNumber: Int?,
    val episodeNumber: Int?,
    val title: String,
    val plot: String?,
    val containerExtension: String?,
    val stream: PlaybackSource
)

data class XtreamSeriesDetail(
    val seriesId: String,
    val name: String,
    val coverUrl: String?,
    val plot: String?,
    val seasons: List<XtreamSeason>
)

data class XtreamSeason(
    val number: Int?,
    val episodes: List<XtreamEpisode>
)
