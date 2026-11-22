package com.sourbase.tv.playback

import android.content.Context
import androidx.media3.common.MediaItem
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import com.sourbase.tv.data.PlaybackSource

/**
 * Device-local playback engine for user-provided HTTP(S) HLS, DASH and
 * progressive URLs. Provider authentication stays outside this class.
 *
 * Call playback methods on the player's application thread (normally Main).
 * The caller owns this engine and must call [release] when its screen/service ends.
 */
@OptIn(UnstableApi::class)
class PlayerEngine(context: Context) {
    private val appContext = context.applicationContext
    val player: ExoPlayer = ExoPlayer.Builder(appContext).build()

    /**
     * Plays a source with optional MIME hint and request headers.
     * The headers are scoped to the media source created for this request.
     */
    fun play(source: PlaybackSource) {
        val uri = validatePlaybackSource(source)

        val httpFactory = DefaultHttpDataSource.Factory()
            .setConnectTimeoutMs(DEFAULT_CONNECT_TIMEOUT_MS)
            .setReadTimeoutMs(DEFAULT_READ_TIMEOUT_MS)
            .setDefaultRequestProperties(source.headers.toMap())

        val dataSourceFactory = DefaultDataSource.Factory(appContext, httpFactory)
        val mediaSourceFactory = DefaultMediaSourceFactory(appContext)
            .setDataSourceFactory(dataSourceFactory)

        val mediaItemBuilder = MediaItem.Builder().setUri(uri.toString())
        source.mimeType
            ?.trim()
            ?.takeIf(String::isNotEmpty)
            ?.let(mediaItemBuilder::setMimeType)

        val mediaSource = mediaSourceFactory.createMediaSource(mediaItemBuilder.build())
        player.setMediaSource(mediaSource)
        player.prepare()
        player.playWhenReady = true
    }

    /** Backwards-compatible convenience overload for public HTTP(S) stream URLs. */
    fun play(streamUrl: String) {
        play(PlaybackSource(url = streamUrl))
    }

    fun pause() {
        player.pause()
    }

    fun stop() {
        player.stop()
        player.clearMediaItems()
    }

    fun release() {
        player.release()
    }

    private companion object {
        const val DEFAULT_CONNECT_TIMEOUT_MS = 15_000
        const val DEFAULT_READ_TIMEOUT_MS = 30_000
    }
}
