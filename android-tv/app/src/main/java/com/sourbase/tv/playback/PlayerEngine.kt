package com.sourbase.tv.playback

import android.content.Context
import android.net.Uri
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer

/**
 * Device-local playback engine for user-provided HLS, DASH and progressive URLs.
 * Provider authentication and source selection are intentionally kept outside
 * this class so the same playback contract can be used by the web client later.
 */
class PlayerEngine(context: Context) {
    val player: Player = ExoPlayer.Builder(context.applicationContext).build()

    fun play(streamUrl: String) {
        val uri = Uri.parse(streamUrl.trim())
        require(uri.scheme.equals("https", ignoreCase = true) ||
            uri.scheme.equals("http", ignoreCase = true)) {
            "Stream URL must use HTTP or HTTPS."
        }

        player.setMediaItem(MediaItem.fromUri(uri))
        player.prepare()
        player.playWhenReady = true
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
}
