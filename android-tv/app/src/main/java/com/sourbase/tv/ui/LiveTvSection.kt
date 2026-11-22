package com.sourbase.tv.ui

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.Text
import com.sourbase.tv.data.EpgProgramme
import com.sourbase.tv.data.RemoteMediaSourceRepository
import com.sourbase.tv.data.TvChannel
import com.sourbase.tv.data.XmltvGuide
import com.sourbase.tv.playback.PlayerEngine
import com.sourbase.tv.playback.SourbasePlayerView
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlinx.coroutines.launch

private val liveAccent = Color(0xFFFF7043)
private val livePanel = Color(0xFF1B2430)
private val liveMuted = Color(0xFF9BA8B8)

@Composable
fun LiveTvSection(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val repository = remember { RemoteMediaSourceRepository() }
    val playerEngine = remember(context) { PlayerEngine(context) }
    val scope = rememberCoroutineScope()

    var playlistUrl by remember { mutableStateOf("") }
    var xmltvUrl by remember { mutableStateOf("") }
    var channels by remember { mutableStateOf(emptyList<TvChannel>()) }
    var guide by remember { mutableStateOf<XmltvGuide?>(null) }
    var selectedChannel by remember { mutableStateOf<TvChannel?>(null) }
    var status by remember { mutableStateOf("Přidejte M3U/M3U8 playlist. Zdroj se zatím ukládá pouze po dobu této obrazovky.") }
    var loadingPlaylist by remember { mutableStateOf(false) }
    var loadingGuide by remember { mutableStateOf(false) }
    var playingError by remember { mutableStateOf<String?>(null) }

    DisposableEffect(playerEngine) {
        onDispose { playerEngine.release() }
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
                onClick = {
                    if (playlistUrl.isBlank()) {
                        status = "Nejdříve zadejte URL M3U/M3U8 playlistu."
                    } else {
                        scope.launch {
                            loadingPlaylist = true
                            status = "Načítám playlist…"
                            val result = runCatching { repository.loadM3u(playlistUrl) }
                            result.onSuccess { parsed ->
                                channels = parsed.channels
                                selectedChannel = null
                                playingError = null
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
                enabled = !loadingPlaylist
            ) {
                Text(if (loadingPlaylist) "Načítám…" else "Načíst playlist")
            }

            Button(
                onClick = {
                    if (xmltvUrl.isBlank()) {
                        status = "Nejdříve zadejte URL XMLTV programu."
                    } else {
                        scope.launch {
                            loadingGuide = true
                            status = "Načítám EPG…"
                            val result = runCatching { repository.loadXmltv(xmltvUrl) }
                            result.onSuccess { parsed ->
                                guide = parsed
                                status = "EPG načteno: ${parsed.channels.size} stanic, ${parsed.programmes.size} pořadů."
                            }.onFailure { error ->
                                status = error.message ?: "XMLTV průvodce se nepodařilo načíst."
                            }
                            loadingGuide = false
                        }
                    }
                },
                enabled = !loadingGuide
            ) {
                Text(if (loadingGuide) "Načítám EPG…" else "Načíst EPG")
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

        if (channels.isEmpty()) {
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
            text = "ZDROJE SE ZATÍM NEUKLÁDAJÍ NA ZAŘÍZENÍ · POUŽÍVEJTE VLASTNÍ OPRÁVNĚNÉ PLAYLISTY",
            color = Color(0xFF657386),
            fontSize = 9.sp,
            letterSpacing = 1.sp
        )
    }
}

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
