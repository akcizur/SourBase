package com.sourbase.tv.ui

import android.content.Intent
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import com.sourbase.tv.data.EpgProgramme
import com.sourbase.tv.data.RemoteMediaSourceRepository
import com.sourbase.tv.data.SecureSourceStore
import com.sourbase.tv.data.TvChannel
import com.sourbase.tv.data.XmltvGuide
import com.sourbase.tv.playback.PlayerEngine
import com.sourbase.tv.playback.SourbasePlayerView
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private val liveAccent = Color(0xFFFF7043)
private val livePanel = Color(0xFF1B2430)
private val liveMuted = Color(0xFF9BA8B8)

@Composable
fun LiveTvSection(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val repository = remember { RemoteMediaSourceRepository() }
    val sourceStore = remember(context) { SecureSourceStore(context) }
    val playerEngine = remember(context) { PlayerEngine(context) }
    val scope = rememberCoroutineScope()

    var playlistUrl by remember { mutableStateOf("") }
    var xmltvUrl by remember { mutableStateOf("") }
    var playlistDocumentUri by remember { mutableStateOf<String?>(null) }
    var xmltvDocumentUri by remember { mutableStateOf<String?>(null) }
    var channels by remember { mutableStateOf(emptyList<TvChannel>()) }
    var guide by remember { mutableStateOf<XmltvGuide?>(null) }
    var selectedChannel by remember { mutableStateOf<TvChannel?>(null) }
    var status by remember { mutableStateOf("Načítám uložené zdroje…") }
    var sourcesReady by remember { mutableStateOf(false) }
    var sourceStorageError by remember { mutableStateOf(false) }
    var sourceReadError by remember { mutableStateOf(false) }
    var showEpgGrid by remember { mutableStateOf(false) }
    var loadingPlaylist by remember { mutableStateOf(false) }
    var loadingGuide by remember { mutableStateOf(false) }
    var playingError by remember { mutableStateOf<String?>(null) }
    val currentGuide = guide

    DisposableEffect(playerEngine) {
        onDispose { playerEngine.release() }
    }

    LaunchedEffect(sourceStore) {
        try {
            val saved = withContext(Dispatchers.IO) { sourceStore.load() }
            playlistUrl = saved.playlistUrl.orEmpty()
            xmltvUrl = saved.xmltvUrl.orEmpty()
            playlistDocumentUri = saved.playlistUri
            xmltvDocumentUri = saved.xmltvUri
            status = if (
                playlistUrl.isNotBlank() || xmltvUrl.isNotBlank() ||
                playlistDocumentUri != null || xmltvDocumentUri != null
            ) {
                "Uložené URL a dokumenty byly bezpečně načteny."
            } else {
                "Zadejte M3U/XMLTV URL nebo vyberte soubor ze zařízení."
            }
        } catch (_: Exception) {
            sourceStorageError = true
            sourceReadError = true
            status = "Uložená URL nelze dešifrovat. Nebyla smazána ani přepsána."
        } finally {
            sourcesReady = true
        }
    }

    fun loadPlaylistDocument(uri: Uri) {
        val oldUri = playlistDocumentUri
        val persistedUri = if (takePersistableDocumentReadPermission(context, uri)) {
            uri.toString()
        } else {
            null
        }
        if (oldUri != persistedUri) releasePersistedDocumentReadPermission(context, oldUri)
        playlistDocumentUri = persistedUri
        playlistUrl = ""

        scope.launch {
            loadingPlaylist = true
            status = "Načítám místní M3U soubor…"
            val result = runCatching { repository.loadM3u(context, uri) }
            result.onSuccess { parsed ->
                channels = parsed.channels
                selectedChannel = null
                playingError = null
                persistSourceUrls(
                    store = sourceStore,
                    playlistUrl = "",
                    xmltvUrl = xmltvUrl,
                    playlistUri = persistedUri,
                    xmltvUri = xmltvDocumentUri,
                    canPersist = !sourceReadError,
                    onSaved = { sourceStorageError = false },
                    onFailure = {
                        sourceStorageError = true
                        status = "Soubor načten, ale jeho URI se nepodařilo bezpečně uložit."
                    }
                )
                status = buildString {
                    append("Ze souboru načteno ")
                    append(parsed.channels.size)
                    append(" kanálů.")
                    if (persistedUri == null) append(" Přístup k souboru se nemusí zachovat po restartu.")
                    if (parsed.warnings.isNotEmpty()) {
                        append(" Upozornění: ")
                        append(parsed.warnings.size)
                    }
                }
            }.onFailure { error ->
                status = error.message ?: "M3U soubor se nepodařilo načíst."
            }
            loadingPlaylist = false
        }
    }

    fun loadXmltvDocument(uri: Uri) {
        val oldUri = xmltvDocumentUri
        val persistedUri = if (takePersistableDocumentReadPermission(context, uri)) {
            uri.toString()
        } else {
            null
        }
        if (oldUri != persistedUri) releasePersistedDocumentReadPermission(context, oldUri)
        xmltvDocumentUri = persistedUri
        xmltvUrl = ""

        scope.launch {
            loadingGuide = true
            status = "Načítám místní XMLTV soubor…"
            val result = runCatching { repository.loadXmltv(context, uri) }
            result.onSuccess { parsed ->
                guide = parsed
                persistSourceUrls(
                    store = sourceStore,
                    playlistUrl = playlistUrl,
                    xmltvUrl = "",
                    playlistUri = playlistDocumentUri,
                    xmltvUri = persistedUri,
                    canPersist = !sourceReadError,
                    onSaved = { sourceStorageError = false },
                    onFailure = {
                        sourceStorageError = true
                        status = "EPG soubor načten, ale jeho URI se nepodařilo bezpečně uložit."
                    }
                )
                status = buildString {
                    append("Ze souboru načteno EPG: ")
                    append(parsed.channels.size)
                    append(" stanic, ")
                    append(parsed.programmes.size)
                    append(" pořadů.")
                    if (persistedUri == null) append(" Přístup k souboru se nemusí zachovat po restartu.")
                }
            }.onFailure { error ->
                status = error.message ?: "XMLTV soubor se nepodařilo načíst."
            }
            loadingGuide = false
        }
    }

    val playlistDocumentPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) loadPlaylistDocument(uri)
    }

    val xmltvDocumentPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) loadXmltvDocument(uri)
    }

    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            SourceUrlField(
                value = playlistUrl,
                onValueChange = { playlistUrl = it },
                label = "M3U / M3U8 URL",
                modifier = Modifier.weight(1f)
            )
            SourceUrlField(
                value = xmltvUrl,
                onValueChange = { xmltvUrl = it },
                label = "XMLTV EPG URL",
                modifier = Modifier.weight(1f)
            )
        }

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Button(
                onClick = { playlistDocumentPicker.launch(arrayOf("*/*")) },
                enabled = sourcesReady && !loadingPlaylist
            ) {
                Text("Vybrat soubor M3U")
            }
            Button(
                onClick = { xmltvDocumentPicker.launch(arrayOf("*/*")) },
                enabled = sourcesReady && !loadingGuide
            ) {
                Text("Vybrat soubor XMLTV")
            }
        }

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Button(
                onClick = {
                    val requestUrl = playlistUrl
                    val requestDocumentUri = playlistDocumentUri
                    if (requestUrl.isBlank() && requestDocumentUri.isNullOrBlank()) {
                        status = "Nejdříve zadejte URL nebo vyberte M3U/M3U8 soubor."
                    } else {
                        scope.launch {
                            loadingPlaylist = true
                            status = "Načítám playlist…"
                            val result = runCatching {
                                if (requestUrl.isNotBlank()) {
                                    repository.loadM3u(requestUrl)
                                } else {
                                    repository.loadM3u(context, Uri.parse(requireNotNull(requestDocumentUri)))
                                }
                            }
                            result.onSuccess { parsed ->
                                channels = parsed.channels
                                selectedChannel = null
                                playingError = null
                                if (requestUrl.isNotBlank() && playlistDocumentUri != null) {
                                    releasePersistedDocumentReadPermission(context, playlistDocumentUri)
                                    playlistDocumentUri = null
                                }
                                persistSourceUrls(
                                    store = sourceStore,
                                    playlistUrl = requestUrl,
                                    xmltvUrl = xmltvUrl,
                                    playlistUri = playlistDocumentUri,
                                    xmltvUri = xmltvDocumentUri,
                                    canPersist = !sourceReadError,
                                    onSaved = { sourceStorageError = false },
                                    onFailure = {
                                        sourceStorageError = true
                                        status = if (sourceReadError) {
                                            "Playlist načten. Starý šifrovaný záznam nebyl přepsán; nejdříve jej ručně vymažte."
                                        } else {
                                            "Playlist načten, ale jeho URL se nepodařilo bezpečně uložit."
                                        }
                                    }
                                )
                                status = buildString {
                                    append("Načteno ")
                                    append(parsed.channels.size)
                                    append(" kanálů.")
                                    if (parsed.warnings.isNotEmpty()) {
                                        append(" Přeskočeno položek s upozorněním: ")
                                        append(parsed.warnings.size)
                                        append('.')
                                    }
                                }
                            }.onFailure { error ->
                                status = error.message ?: "Playlist se nepodařilo načíst."
                            }
                            loadingPlaylist = false
                        }
                    }
                },
                enabled = sourcesReady && !loadingPlaylist
            ) {
                Text(if (loadingPlaylist) "Načítám…" else "Načíst playlist")
            }

            Button(
                onClick = {
                    val requestUrl = xmltvUrl
                    val requestDocumentUri = xmltvDocumentUri
                    if (requestUrl.isBlank() && requestDocumentUri.isNullOrBlank()) {
                        status = "Nejdříve zadejte URL nebo vyberte XMLTV soubor."
                    } else {
                        scope.launch {
                            loadingGuide = true
                            status = "Načítám EPG…"
                            val result = runCatching {
                                if (requestUrl.isNotBlank()) {
                                    repository.loadXmltv(requestUrl)
                                } else {
                                    repository.loadXmltv(context, Uri.parse(requireNotNull(requestDocumentUri)))
                                }
                            }
                            result.onSuccess { parsed ->
                                guide = parsed
                                if (requestUrl.isNotBlank() && xmltvDocumentUri != null) {
                                    releasePersistedDocumentReadPermission(context, xmltvDocumentUri)
                                    xmltvDocumentUri = null
                                }
                                persistSourceUrls(
                                    store = sourceStore,
                                    playlistUrl = playlistUrl,
                                    xmltvUrl = requestUrl,
                                    playlistUri = playlistDocumentUri,
                                    xmltvUri = xmltvDocumentUri,
                                    canPersist = !sourceReadError,
                                    onSaved = { sourceStorageError = false },
                                    onFailure = {
                                        sourceStorageError = true
                                        status = if (sourceReadError) {
                                            "EPG načteno. Starý šifrovaný záznam nebyl přepsán; nejdříve jej ručně vymažte."
                                        } else {
                                            "EPG načteno, ale URL zdrojů se nepodařilo bezpečně uložit."
                                        }
                                    }
                                )
                                status = "EPG načteno: ${parsed.channels.size} stanic, ${parsed.programmes.size} pořadů."
                            }.onFailure { error ->
                                status = error.message ?: "XMLTV průvodce se nepodařilo načíst."
                            }
                            loadingGuide = false
                        }
                    }
                },
                enabled = sourcesReady && !loadingGuide
            ) {
                Text(if (loadingGuide) "Načítám EPG…" else "Načíst EPG")
            }

            if (guide != null && channels.isNotEmpty()) {
                Button(onClick = { showEpgGrid = !showEpgGrid }) {
                    Text(if (showEpgGrid) "Zpět na kanály" else "TV program")
                }
            }

            if (
                (playlistUrl.isNotBlank() || xmltvUrl.isNotBlank() ||
                    playlistDocumentUri != null || xmltvDocumentUri != null) &&
                !sourceReadError
            ) {
                Button(
                    onClick = {
                        scope.launch {
                            val cleared = runCatching {
                                withContext(Dispatchers.IO) { sourceStore.clear() }
                            }
                            cleared.onSuccess {
                                playerEngine.stop()
                                releasePersistedDocumentReadPermission(context, playlistDocumentUri)
                                releasePersistedDocumentReadPermission(context, xmltvDocumentUri)
                                playlistUrl = ""
                                xmltvUrl = ""
                                playlistDocumentUri = null
                                xmltvDocumentUri = null
                                channels = emptyList()
                                guide = null
                                selectedChannel = null
                                playingError = null
                                showEpgGrid = false
                                sourceStorageError = false
                                status = "Uložené zdroje, kanály a EPG byly odebrány."
                            }.onFailure {
                                sourceStorageError = true
                                status = "Uložené zdroje se nepodařilo bezpečně odebrat."
                            }
                        }
                    },
                    enabled = sourcesReady && !loadingPlaylist && !loadingGuide
                ) {
                    Text("Odebrat zdroje")
                }
            }

            Spacer(Modifier.weight(1f))
            Text(
                text = "${channels.size} kanálů",
                color = liveMuted,
                fontSize = 13.sp
            )
        }

        Text(
            text = status,
            color = liveMuted,
            fontSize = 12.sp,
            maxLines = 2
        )

        if (sourceReadError) {
            Button(
                onClick = {
                    scope.launch {
                        val cleared = runCatching {
                            withContext(Dispatchers.IO) { sourceStore.clear() }
                        }
                        cleared.onSuccess {
                            sourceReadError = false
                            sourceStorageError = false
                            status = "Nedostupný uložený záznam byl vymazán. Zadejte zdroje znovu."
                        }.onFailure {
                            status = "Uložený záznam se nepodařilo bezpečně vymazat."
                        }
                    }
                },
                enabled = sourcesReady && !loadingPlaylist && !loadingGuide
            ) {
                Text("Vymazat nedostupné uložené URL")
            }
        }

        if (showEpgGrid && currentGuide != null && channels.isNotEmpty()) {
            EpgTimelineGrid(
                channels = channels,
                guide = currentGuide,
                onProgrammeSelected = { channel, programme ->
                    val now = System.currentTimeMillis()
                    if (now >= programme.startEpochMs && now < programme.endEpochMs) {
                        selectedChannel = channel
                        playingError = null
                        runCatching { playerEngine.play(channel.stream) }
                            .onSuccess {
                                status = "Přehrávám živě: ${channel.name} · ${programme.title}"
                                showEpgGrid = false
                            }
                            .onFailure { error ->
                                playingError = error.message ?: "Přehrávání se nepodařilo spustit."
                                status = playingError.orEmpty()
                            }
                    } else {
                        val description = programme.description?.takeIf(String::isNotBlank)
                        status = buildString {
                            append(programme.title)
                            append(" · ")
                            append(formatTime(programme.startEpochMs))
                            append("–")
                            append(formatTime(programme.endEpochMs))
                            description?.let {
                                append(" · ")
                                append(it)
                            }
                            append(" · Catch-up není dostupný; živý kanál lze spustit v nabídce Kanály.")
                        }
                    }
                },
                modifier = Modifier.weight(1f)
            )
        } else if (channels.isEmpty()) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f)
                    .clip(RoundedCornerShape(16.dp))
                    .background(livePanel)
                    .padding(22.dp),
                verticalArrangement = Arrangement.Center
            ) {
                Text(
                    text = "Živá TV",
                    color = Color.White,
                    fontSize = 24.sp,
                    fontWeight = FontWeight.Bold
                )
                Text(
                    text = "Po načtení playlistu se zde zobrazí kanály a jejich aktuální pořady. Vyberte kanál tlačítkem OK a spusťte stream.",
                    color = liveMuted,
                    fontSize = 14.sp
                )
            }
        } else {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .weight(1f),
                horizontalArrangement = Arrangement.spacedBy(14.dp)
            ) {
                LazyColumn(
                    modifier = Modifier
                        .weight(0.43f)
                        .fillMaxHeight(),
                    verticalArrangement = Arrangement.spacedBy(6.dp)
                ) {
                    items(channels, key = TvChannel::id) { channel ->
                        ChannelRow(
                            channel = channel,
                            selected = selectedChannel?.id == channel.id,
                            programme = guide?.programmesFor(channel)?.firstCurrentProgramme(),
                            onClick = {
                                selectedChannel = channel
                                playingError = null
                                runCatching { playerEngine.play(channel.stream) }
                                    .onFailure { error ->
                                        playingError = error.message ?: "Přehrávání se nepodařilo spustit."
                                    }
                            }
                        )
                    }
                }

                Column(
                    modifier = Modifier
                        .weight(0.57f)
                        .fillMaxHeight(),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    selectedChannel?.let { channel ->
                        Text(
                            text = channel.name,
                            color = Color.White,
                            fontSize = 19.sp,
                            fontWeight = FontWeight.Bold
                        )
                        channel.group?.let {
                            Text(text = it, color = liveAccent, fontSize = 12.sp)
                        }
                        guide?.programmesFor(channel)?.firstCurrentProgramme()?.let { programme ->
                            Text(
                                text = "${formatTime(programme.startEpochMs)}–${formatTime(programme.endEpochMs)}  ·  ${programme.title}",
                                color = liveMuted,
                                fontSize = 13.sp,
                                maxLines = 2
                            )
                        } ?: Text(
                            text = "Aktuální pořad není v EPG dostupný.",
                            color = liveMuted,
                            fontSize = 12.sp
                        )
                        playingError?.let {
                            Text(text = it, color = Color(0xFFFF8A80), fontSize = 12.sp)
                        }
                        SourbasePlayerView(
                            player = playerEngine.player,
                            modifier = Modifier
                                .fillMaxWidth()
                                .weight(1f)
                                .clip(RoundedCornerShape(14.dp))
                        )
                    } ?: Column(
                        modifier = Modifier
                            .fillMaxSize()
                            .clip(RoundedCornerShape(16.dp))
                            .background(livePanel)
                            .padding(22.dp),
                        verticalArrangement = Arrangement.Center
                    ) {
                        Text(
                            text = "Vyberte kanál",
                            color = Color.White,
                            fontSize = 21.sp,
                            fontWeight = FontWeight.Bold
                        )
                        Text(
                            text = "Stiskněte OK na kanálu v seznamu. Přehrávač se zobrazí zde.",
                            color = liveMuted,
                            fontSize = 14.sp
                        )
                    }
                }
            }
        }

        Text(
            text = if (sourceStorageError) "ÚLOŽIŠTĚ ZDROJŮ VYŽADUJE KONTROLU · ULOŽENÁ DATA ZŮSTALA ZACHOVÁNA" else "ZDROJE JSOU ŠIFROVÁNY POMOCÍ ANDROID KEYSTORE · POUŽÍVEJTE VLASTNÍ OPRÁVNĚNÉ PLAYLISTY",
            color = Color(0xFF657386),
            fontSize = 9.sp,
            letterSpacing = 1.sp
        )
    }
}


private fun kotlinx.coroutines.CoroutineScope.persistSourceUrls(
    store: SecureSourceStore,
    playlistUrl: String,
    xmltvUrl: String,
    playlistUri: String?,
    xmltvUri: String?,
    canPersist: Boolean,
    onSaved: () -> Unit,
    onFailure: () -> Unit
) {
    launch {
        if (!canPersist) {
            onFailure()
            return@launch
        }

        try {
            withContext(Dispatchers.IO) {
                store.save(playlistUrl, xmltvUrl, playlistUri, xmltvUri)
            }
            onSaved()
        } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (_: Exception) {
            onFailure()
        }
    }
}


private fun takePersistableDocumentReadPermission(context: android.content.Context, uri: Uri): Boolean =
    runCatching {
        context.contentResolver.takePersistableUriPermission(
            uri,
            Intent.FLAG_GRANT_READ_URI_PERMISSION
        )
    }.isSuccess

private fun releasePersistedDocumentReadPermission(context: android.content.Context, rawUri: String?) {
    val value = rawUri?.takeIf(String::isNotBlank) ?: return
    runCatching {
        context.contentResolver.releasePersistableUriPermission(
            Uri.parse(value),
            Intent.FLAG_GRANT_READ_URI_PERMISSION
        )
    }
}

private const val EPG_SLOT_DURATION_MS = 30L * 60L * 1000L
private const val EPG_SLOT_COUNT = 6
private val EPG_SLOT_WIDTH = 92.dp
private val EPG_CHANNEL_COLUMN_WIDTH = 164.dp

@Composable
private fun EpgTimelineGrid(
    channels: List<TvChannel>,
    guide: XmltvGuide,
    onProgrammeSelected: (TvChannel, EpgProgramme) -> Unit,
    modifier: Modifier = Modifier
) {
    var windowOffsetSlots by remember(guide) { mutableStateOf(0) }
    val timelineStart = remember(guide, windowOffsetSlots) {
        val now = System.currentTimeMillis()
        val currentSlotStart = now - (now % EPG_SLOT_DURATION_MS)
        currentSlotStart + windowOffsetSlots.toLong() * EPG_SLOT_DURATION_MS
    }
    val timelineEnd = timelineStart + EPG_SLOT_COUNT.toLong() * EPG_SLOT_DURATION_MS

    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Button(onClick = { windowOffsetSlots -= EPG_SLOT_COUNT }) {
                Text("← 3 h")
            }
            Button(onClick = { windowOffsetSlots = 0 }) {
                Text("Nyní")
            }
            Button(onClick = { windowOffsetSlots += EPG_SLOT_COUNT }) {
                Text("3 h →")
            }
            Text(
                text = "${formatTime(timelineStart)} – ${formatTime(timelineEnd)}",
                color = liveMuted,
                fontSize = 11.sp
            )
        }

        Row(
            modifier = Modifier
                .fillMaxWidth()
                .background(livePanel)
                .padding(vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = "STANICE",
                modifier = Modifier.width(EPG_CHANNEL_COLUMN_WIDTH).padding(start = 12.dp),
                color = liveAccent,
                fontSize = 10.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 1.sp
            )
            repeat(EPG_SLOT_COUNT) { slot ->
                Text(
                    text = formatTime(timelineStart + slot.toLong() * EPG_SLOT_DURATION_MS),
                    modifier = Modifier.width(EPG_SLOT_WIDTH),
                    color = liveMuted,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold
                )
            }
        }

        LazyColumn(
            modifier = Modifier
                .fillMaxWidth()
                .weight(1f),
            verticalArrangement = Arrangement.spacedBy(5.dp)
        ) {
            items(channels, key = TvChannel::id) { channel ->
                val visibleProgrammes = guide.programmesFor(channel)
                    .filter { it.endEpochMs > timelineStart && it.startEpochMs < timelineEnd }

                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(72.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column(
                        modifier = Modifier
                            .width(EPG_CHANNEL_COLUMN_WIDTH)
                            .padding(end = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(3.dp)
                    ) {
                        Text(
                            text = channel.name,
                            color = Color.White,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.SemiBold,
                            maxLines = 1
                        )
                        Text(
                            text = channel.group ?: "TV",
                            color = liveMuted,
                            fontSize = 10.sp,
                            maxLines = 1
                        )
                    }

                    Row(
                        modifier = Modifier.height(68.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        var cursor = timelineStart
                        visibleProgrammes.forEach { programme ->
                            val visibleStart = maxOf(programme.startEpochMs, cursor, timelineStart)
                            val visibleEnd = minOf(programme.endEpochMs, timelineEnd)
                            if (visibleEnd > visibleStart) {
                                if (visibleStart > cursor) {
                                    Spacer(Modifier.width(epgWidthFor(visibleStart - cursor)))
                                }
                                val cardWidth = epgWidthFor(visibleEnd - visibleStart)
                                Button(
                                    onClick = { onProgrammeSelected(channel, programme) },
                                    modifier = Modifier
                                        .width(cardWidth)
                                        .height(64.dp)
                                        .padding(horizontal = 2.dp)
                                ) {
                                    Column(
                                        modifier = Modifier.fillMaxWidth(),
                                        verticalArrangement = Arrangement.spacedBy(3.dp)
                                    ) {
                                        Text(
                                            text = programme.title,
                                            color = Color.White,
                                            fontSize = 11.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            maxLines = 2
                                        )
                                        Text(
                                            text = "${formatTime(programme.startEpochMs)}–${formatTime(programme.endEpochMs)}",
                                            color = Color(0xFFD5DDE6),
                                            fontSize = 9.sp,
                                            maxLines = 1
                                        )
                                    }
                                }
                                cursor = visibleEnd
                            }
                        }
                        if (cursor < timelineEnd) {
                            Spacer(Modifier.width(epgWidthFor(timelineEnd - cursor)))
                        }
                    }
                }
            }
        }

        Text(
            text = "OK na právě vysílaném pořadu spustí kanál. Výběr budoucího pořadu zobrazí detail; catch-up není zatím napojen.",
            color = liveMuted,
            fontSize = 10.sp
        )
    }
}

private fun epgWidthFor(durationMs: Long): Dp =
    (EPG_SLOT_WIDTH.value * durationMs.toFloat() / EPG_SLOT_DURATION_MS.toFloat()).dp

@Composable
private fun SourceUrlField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    modifier: Modifier = Modifier
) {
    Column(modifier = modifier, verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(text = label, color = liveMuted, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            singleLine = true,
            textStyle = TextStyle(color = Color.White, fontSize = 13.sp),
            cursorBrush = SolidColor(liveAccent),
            modifier = Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(10.dp))
                .background(livePanel)
                .border(1.dp, Color(0xFF344252), RoundedCornerShape(10.dp))
                .padding(horizontal = 12.dp, vertical = 11.dp),
            decorationBox = { innerTextField ->
                androidx.compose.foundation.layout.Box {
                    if (value.isEmpty()) {
                        Text(
                            text = "https://…",
                            color = Color(0xFF758396),
                            fontSize = 13.sp
                        )
                    }
                    innerTextField()
                }
            }
        )
    }
}

@Composable
private fun ChannelRow(
    channel: TvChannel,
    selected: Boolean,
    programme: EpgProgramme?,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(10.dp)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(shape)
            .onFocusChanged { focused = it.isFocused }
            .background(
                when {
                    focused -> Color(0xFF344252)
                    selected -> Color(0xFF293545)
                    else -> livePanel
                }
            )
            .border(
                width = if (focused || selected) 2.dp else 1.dp,
                color = if (focused) liveAccent else if (selected) Color(0xFF536477) else Color.Transparent,
                shape = shape
            )
            .clickable(onClick = onClick)
            .focusable()
            .padding(horizontal = 12.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Text(
            text = if (selected) "▶" else "TV",
            color = liveAccent,
            fontSize = 12.sp,
            fontWeight = FontWeight.Bold
        )
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                text = channel.name,
                color = Color.White,
                fontSize = 14.sp,
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium,
                maxLines = 1
            )
            Text(
                text = programme?.title ?: channel.group ?: "Bez programu",
                color = liveMuted,
                fontSize = 11.sp,
                maxLines = 1
            )
        }
    }
}

private fun List<EpgProgramme>.firstCurrentProgramme(): EpgProgramme? {
    val now = System.currentTimeMillis()
    return firstOrNull { it.startEpochMs <= now && it.endEpochMs > now }
}

private fun formatTime(epochMs: Long): String =
    DateTimeFormatter.ofPattern("HH:mm")
        .withZone(ZoneId.systemDefault())
        .format(Instant.ofEpochMilli(epochMs))
