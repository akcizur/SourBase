package com.sourbase.tv.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URI
import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import java.util.Locale

/**
 * Xtream Codes-compatible API client for user-owned provider accounts.
 *
 * Credentials remain in memory for the request and are never logged. Source
 * credentials must be stored through SecureSourceStore by the UI layer.
 *
 * API requests deliberately do not follow redirects: provider credentials are
 * included in the query and must not be forwarded to another endpoint.
 */
class XtreamApiRepository(
    private val connectTimeoutMs: Int = 15_000,
    private val readTimeoutMs: Int = 30_000
) {
    suspend fun authenticate(credentials: XtreamCredentials): XtreamAccountInfo =
        withContext(Dispatchers.IO) {
            val response = requestObject(buildXtreamApiUri(credentials, action = null))
            val userInfo = response.optJSONObject("user_info")
                ?: throw IOException("Poskytovatel nevrátil informace o účtu.")

            val authenticated = userInfo.opt("auth")?.toString()?.let { it != "0" && !it.equals("false", true) }
                ?: true
            val status = userInfo.optString("status").ifBlank { "Unknown" }
            if (!authenticated || status.equals("Expired", true) ||
                status.equals("Disabled", true) || status.equals("Banned", true)
            ) {
                throw IOException("Přístup k účtu poskytovatele není aktivní.")
            }

            XtreamAccountInfo(
                username = userInfo.optString("username").ifBlank { credentials.username },
                status = status,
                expirationEpochSeconds = userInfo.optString("exp_date")
                    .takeIf(String::isNotBlank)
                    ?.toLongOrNull()
                    ?.takeIf { it > 0L },
                isTrial = userInfo.optString("is_trial").let { it == "1" || it.equals("true", true) },
                activeConnections = userInfo.optString("active_cons").toIntOrNull(),
                maximumConnections = userInfo.optString("max_connections").toIntOrNull()
            )
        }

    suspend fun loadLiveCategories(credentials: XtreamCredentials): List<XtreamCategory> =
        withContext(Dispatchers.IO) {
            requestArray(buildXtreamApiUri(credentials, "get_live_categories")).toCategories()
        }

    suspend fun loadLiveChannels(
        credentials: XtreamCredentials,
        categoryId: String? = null
    ): List<XtreamLiveChannel> = withContext(Dispatchers.IO) {
        requestArray(buildXtreamApiUri(credentials, "get_live_streams", categoryId))
            .mapNotNull { item ->
                val streamId = item.optString("stream_id").takeIf(::isNumericId) ?: return@mapNotNull null
                val name = item.optString("name").trim().takeIf(String::isNotEmpty) ?: return@mapNotNull null
                val extension = safeExtension(item.optString("container_extension"), "m3u8")
                XtreamLiveChannel(
                    id = streamId,
                    name = name,
                    categoryId = item.optNullableString("category_id"),
                    categoryName = item.optNullableString("category_name"),
                    epgChannelId = item.optNullableString("epg_channel_id"),
                    logoUrl = item.optHttpUrl("stream_icon"),
                    stream = PlaybackSource(
                        url = buildXtreamStreamUri(credentials, "live", streamId, extension),
                        mimeType = if (extension == "m3u8") HLS_MIME_TYPE else null
                    )
                )
            }
    }

    suspend fun loadVodCategories(credentials: XtreamCredentials): List<XtreamCategory> =
        withContext(Dispatchers.IO) {
            requestArray(buildXtreamApiUri(credentials, "get_vod_categories")).toCategories()
        }

    suspend fun loadMovies(
        credentials: XtreamCredentials,
        categoryId: String? = null
    ): List<XtreamVodItem> = withContext(Dispatchers.IO) {
        requestArray(buildXtreamApiUri(credentials, "get_vod_streams", categoryId))
            .mapNotNull { item ->
                val streamId = item.optString("stream_id").takeIf(::isNumericId) ?: return@mapNotNull null
                val name = item.optString("name").trim().takeIf(String::isNotEmpty) ?: return@mapNotNull null
                val extension = safeExtension(item.optString("container_extension"), "mp4")
                XtreamVodItem(
                    id = streamId,
                    name = name,
                    categoryId = item.optNullableString("category_id"),
                    categoryName = item.optNullableString("category_name"),
                    posterUrl = item.optHttpUrl("stream_icon"),
                    plot = item.optNullableString("plot"),
                    year = item.optNullableString("year"),
                    rating = item.optNullableString("rating"),
                    stream = PlaybackSource(
                        url = buildXtreamStreamUri(credentials, "movie", streamId, extension),
                        mimeType = mimeTypeForExtension(extension)
                    )
                )
            }
    }

    suspend fun loadSeriesCategories(credentials: XtreamCredentials): List<XtreamCategory> =
        withContext(Dispatchers.IO) {
            requestArray(buildXtreamApiUri(credentials, "get_series_categories")).toCategories()
        }

    suspend fun loadSeries(
        credentials: XtreamCredentials,
        categoryId: String? = null
    ): List<XtreamSeriesItem> = withContext(Dispatchers.IO) {
        requestArray(buildXtreamApiUri(credentials, "get_series", categoryId))
            .mapNotNull { item ->
                val seriesId = item.optString("series_id").takeIf(::isNumericId)
                    ?: item.optString("id").takeIf(::isNumericId)
                    ?: return@mapNotNull null
                val name = item.optString("name").trim().takeIf(String::isNotEmpty) ?: return@mapNotNull null
                XtreamSeriesItem(
                    id = seriesId,
                    name = name,
                    categoryId = item.optNullableString("category_id"),
                    categoryName = item.optNullableString("category_name"),
                    coverUrl = item.optHttpUrl("cover"),
                    plot = item.optNullableString("plot"),
                    year = item.optNullableString("year"),
                    rating = item.optNullableString("rating")
                )
            }
    }

    suspend fun loadSeriesDetail(
        credentials: XtreamCredentials,
        series: XtreamSeriesItem
    ): XtreamSeriesDetail = withContext(Dispatchers.IO) {
        val response = requestObject(
            buildXtreamApiUri(credentials, action = "get_series_info", extra = listOf("series_id" to series.id))
        )
        val info = response.optJSONObject("info")
        val episodesObject = response.optJSONObject("episodes")
            ?: throw IOException("Poskytovatel nevrátil epizody seriálu.")

        val seasons = mutableListOf<XtreamSeason>()
        val keys = episodesObject.keys()
        while (keys.hasNext()) {
            val seasonKey = keys.next()
            val episodeArray = episodesObject.optJSONArray(seasonKey) ?: continue
            val seasonNumber = seasonKey.toIntOrNull()
            val episodes = episodeArray.mapObjects().mapIndexedNotNull { index, episode ->
                val episodeId = episode.optString("id").takeIf(::isNumericId) ?: return@mapIndexedNotNull null
                val title = episode.optString("title").ifBlank {
                    episode.optString("name").ifBlank { "Epizoda ${index + 1}" }
                }
                val extension = safeExtension(episode.optString("container_extension"), "mp4")
                val episodeNumber = episode.optString("episode_num").toIntOrNull()
                XtreamEpisode(
                    id = episodeId,
                    seasonNumber = seasonNumber,
                    episodeNumber = episodeNumber,
                    title = title,
                    plot = episode.optJSONObject("info")?.optNullableString("plot"),
                    containerExtension = extension,
                    stream = PlaybackSource(
                        url = buildXtreamStreamUri(credentials, "series", episodeId, extension),
                        mimeType = mimeTypeForExtension(extension)
                    )
                )
            }
            seasons += XtreamSeason(number = seasonNumber, episodes = episodes)
        }

        XtreamSeriesDetail(
            seriesId = series.id,
            name = series.name,
            coverUrl = info?.optHttpUrl("cover_big") ?: series.coverUrl,
            plot = info?.optNullableString("plot") ?: series.plot,
            seasons = seasons.sortedWith(compareBy(nullsFirst()) { it.number })
        )
    }

    private fun requestObject(uri: URI): JSONObject =
        try {
            JSONObject(requestText(uri))
        } catch (error: IOException) {
            throw error
        } catch (error: Exception) {
            throw IOException("Odpověď poskytovatele není platný JSON objekt.", error)
        }

    private fun requestArray(uri: URI): JSONArray =
        try {
            JSONArray(requestText(uri))
        } catch (error: IOException) {
            throw error
        } catch (error: Exception) {
            throw IOException("Odpověď poskytovatele není platné JSON pole.", error)
        }

    private fun requestText(uri: URI): String {
        val connection = (uri.toURL().openConnection() as? HttpURLConnection)
            ?: throw IOException("Xtream URL is not an HTTP connection.")
        try {
            connection.connectTimeout = connectTimeoutMs
            connection.readTimeout = readTimeoutMs
            connection.instanceFollowRedirects = false
            connection.requestMethod = "GET"
            connection.setRequestProperty("Accept", "application/json")
            connection.setRequestProperty("Accept-Encoding", "identity")
            connection.setRequestProperty("User-Agent", USER_AGENT)

            val responseCode = connection.responseCode
            if (responseCode in REDIRECT_CODES) {
                throw IOException("Xtream API přesměrovalo požadavek. Zkontrolujte základní URL poskytovatele.")
            }
            if (responseCode !in 200..299) {
                throw IOException("Xtream API vrátilo HTTP $responseCode.")
            }

            val declaredLength = connection.contentLengthLong
            if (declaredLength > MAX_RESPONSE_BYTES) {
                throw IOException("Odpověď Xtream API překračuje povolenou velikost.")
            }

            val output = ByteArrayOutputStream(
                if (declaredLength in 1..MAX_RESPONSE_BYTES.toLong()) declaredLength.toInt() else 8 * 1024
            )
            connection.inputStream.use { input ->
                val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
                var total = 0
                while (true) {
                    val read = input.read(buffer)
                    if (read < 0) break
                    total += read
                    if (total > MAX_RESPONSE_BYTES) {
                        throw IOException("Odpověď Xtream API překročila povolenou velikost.")
                    }
                    output.write(buffer, 0, read)
                }
            }
            return output.toByteArray().toString(StandardCharsets.UTF_8).removePrefix("\uFEFF")
        } finally {
            connection.disconnect()
        }
    }

    private companion object {
        const val MAX_RESPONSE_BYTES = 16 * 1024 * 1024
        const val HLS_MIME_TYPE = "application/vnd.apple.mpegurl"
        const val USER_AGENT = "Sourbase-AndroidTV/0.1"
        val REDIRECT_CODES = setOf(301, 302, 303, 307, 308)
    }
}

internal fun buildXtreamApiUri(
    credentials: XtreamCredentials,
    action: String?,
    categoryId: String? = null,
    extra: List<Pair<String, String>> = emptyList()
): URI {
    val base = validateXtreamBaseUrl(credentials.baseUrl)
    require(credentials.username.isNotBlank()) { "Xtream username must not be empty." }
    require(credentials.password.isNotBlank()) { "Xtream password must not be empty." }
    require(action == null || action.matches(Regex("[A-Za-z0-9_]+"))) { "Xtream action is invalid." }

    val query = buildList {
        add("username" to credentials.username)
        add("password" to credentials.password)
        action?.let { add("action" to it) }
        categoryId?.let { add("category_id" to it) }
        addAll(extra)
    }.joinToString("&") { (key, value) ->
        "${encodeQueryComponent(key)}=${encodeQueryComponent(value)}"
    }

    return URI("${base.toASCIIString()}/player_api.php?$query")
}

internal fun buildXtreamStreamUri(
    credentials: XtreamCredentials,
    contentType: String,
    streamId: String,
    extension: String
): String {
    val base = validateXtreamBaseUrl(credentials.baseUrl)
    require(credentials.username.isNotBlank()) { "Xtream username must not be empty." }
    require(credentials.password.isNotBlank()) { "Xtream password must not be empty." }
    require(contentType in setOf("live", "movie", "series")) { "Xtream content type is invalid." }
    require(isNumericId(streamId)) { "Xtream stream id is invalid." }

    val safeExtension = safeExtension(extension, "mp4")
    val username = encodePathComponent(credentials.username)
    val password = encodePathComponent(credentials.password)
    return "${base.toASCIIString()}/$contentType/$username/$password/$streamId.$safeExtension"
}

private fun validateXtreamBaseUrl(rawUrl: String): URI {
    val value = rawUrl.trim().trimEnd('/')
    require(value.isNotEmpty()) { "Xtream base URL must not be empty." }
    val uri = try {
        URI(value)
    } catch (error: Exception) {
        throw IllegalArgumentException("Xtream base URL is malformed.", error)
    }
    val scheme = uri.scheme?.lowercase(Locale.ROOT)
    require(scheme == "http" || scheme == "https") {
        "Xtream base URL must use HTTP or HTTPS."
    }
    require(!uri.host.isNullOrBlank() && uri.rawUserInfo == null) {
        "Xtream base URL must contain a host and no embedded credentials."
    }
    require(uri.rawQuery == null && uri.rawFragment == null) {
        "Xtream base URL must not contain a query or fragment."
    }
    return uri
}

private fun encodeQueryComponent(value: String): String =
    URLEncoder.encode(value, StandardCharsets.UTF_8.name())

private fun encodePathComponent(value: String): String =
    encodeQueryComponent(value).replace("+", "%20")

private fun isNumericId(value: String): Boolean =
    value.isNotBlank() && value.all(Char::isDigit)

private fun safeExtension(value: String, fallback: String): String {
    val normalized = value.trim().removePrefix(".").lowercase(Locale.ROOT)
    return normalized.takeIf { it.matches(Regex("[a-z0-9]{1,10}")) } ?: fallback
}

private fun mimeTypeForExtension(extension: String): String? = when (extension.lowercase(Locale.ROOT)) {
    "mp4", "m4v" -> "video/mp4"
    "mkv" -> "video/x-matroska"
    "mp3" -> "audio/mpeg"
    "m3u8" -> "application/vnd.apple.mpegurl"
    else -> null
}

private fun JSONObject.optNullableString(key: String): String? =
    optString(key).trim().takeIf(String::isNotEmpty)

private fun JSONObject.optHttpUrl(key: String): String? {
    val value = optNullableString(key) ?: return null
    return try {
        val uri = URI(value)
        if ((uri.scheme.equals("http", true) || uri.scheme.equals("https", true)) &&
            !uri.host.isNullOrBlank() && uri.rawUserInfo == null
        ) value else null
    } catch (_: Exception) {
        null
    }
}

private fun JSONArray.mapObjects(): List<JSONObject> =
    (0 until length()).mapNotNull { index -> optJSONObject(index) }

private fun JSONArray.toCategories(): List<XtreamCategory> =
    mapObjects().mapNotNull { item ->
        val id = item.optString("category_id").takeIf(::isNumericId) ?: return@mapNotNull null
        val name = item.optString("category_name").trim().takeIf(String::isNotEmpty) ?: return@mapNotNull null
        XtreamCategory(id, name, item.optNullableString("parent_id"))
    }
