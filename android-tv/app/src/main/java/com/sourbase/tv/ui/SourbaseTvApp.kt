package com.sourbase.tv.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusGroup
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private data class TvSection(
    val title: String,
    val eyebrow: String,
    val description: String
)

private val sections = listOf(
    TvSection("Domů", "SOURBASE", "Pokračujte ve sledování a objevujte nový obsah."),
    TvSection("Živá TV", "IPTV", "Kanály, skupiny a televizní program na jednom místě."),
    TvSection("Filmy", "VOD", "Filmová knihovna a přehledné žánrové kolekce."),
    TvSection("Seriály", "VOD", "Sezóny, epizody a navazování ve sledování."),
    TvSection("Knihovna", "MOJE SOURBASE", "Oblíbené položky a historie přehrávání."),
    TvSection("Hledat", "UNIVERZÁLNÍ", "Hledejte kanály, pořady, filmy a seriály.")
)

private val accent = Color(0xFFFF7043)
private val backgroundTop = Color(0xFF121821)
private val backgroundBottom = Color(0xFF080B10)
private val panel = Color(0xFF1B2430)
private val muted = Color(0xFF9BA8B8)

@Composable
fun SourbaseTvApp() {
    var selectedSection by remember { mutableStateOf("Domů") }

    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = accent,
            background = backgroundBottom,
            surface = panel,
            onPrimary = Color(0xFF17120F),
            onBackground = Color(0xFFF4F6FA),
            onSurface = Color(0xFFF4F6FA)
        )
    ) {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = backgroundBottom
        ) {
            Row(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            colors = listOf(backgroundTop, backgroundBottom)
                        )
                    )
                    .padding(horizontal = 44.dp, vertical = 32.dp),
                horizontalArrangement = Arrangement.spacedBy(38.dp)
            ) {
                NavigationRail(
                    selectedSection = selectedSection,
                    onSelect = { selectedSection = it },
                    modifier = Modifier
                        .width(190.dp)
                        .fillMaxHeight()
                )

                val section = sections.first { it.title == selectedSection }
                Column(
                    modifier = Modifier
                        .weight(1f)
                        .fillMaxHeight(),
                    verticalArrangement = Arrangement.spacedBy(20.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.SpaceBetween
                    ) {
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            Text(
                                text = section.eyebrow,
                                color = accent,
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Bold,
                                letterSpacing = 2.sp
                            )
                            Text(
                                text = section.title,
                                color = Color.White,
                                fontSize = 34.sp,
                                fontWeight = FontWeight.Bold
                            )
                        }
                        Text(
                            text = "NÁHLED  ·  CZ",
                            color = muted,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.SemiBold
                        )
                    }

                    HeroPanel(
                        selectedSection = selectedSection,
                        description = section.description
                    )

                    Text(
                        text = if (selectedSection == "Domů") "Prozkoumat Sourbase" else "Sekce",
                        color = Color.White,
                        fontSize = 21.sp,
                        fontWeight = FontWeight.SemiBold
                    )

                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(16.dp),
                        contentPadding = PaddingValues(end = 28.dp)
                    ) {
                        items(sections.filterNot { it.title == selectedSection }) { item ->
                            SectionCard(
                                section = item,
                                onClick = { selectedSection = item.title }
                            )
                        }
                    }

                    Spacer(modifier = Modifier.weight(1f))

                    Text(
                        text = "ANDROID TV PREVIEW  ·  ZÁKLADNÍ ROZHRANÍ",
                        color = Color(0xFF657386),
                        fontSize = 11.sp,
                        letterSpacing = 1.5.sp
                    )
                }
            }
        }
    }
}

@Composable
private fun NavigationRail(
    selectedSection: String,
    onSelect: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    val initialFocusRequester = remember { FocusRequester() }

    LaunchedEffect(Unit) {
        initialFocusRequester.requestFocus()
    }

    Column(
        modifier = modifier
            .padding(top = 10.dp)
            .focusGroup(),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "SOURBASE",
            color = Color.White,
            fontSize = 22.sp,
            fontWeight = FontWeight.Black,
            letterSpacing = 2.sp
        )
        Text(
            text = "MEDIA CENTER",
            color = muted,
            fontSize = 10.sp,
            letterSpacing = 2.sp
        )
        Spacer(modifier = Modifier.height(24.dp))

        sections.forEachIndexed { index, section ->
            var focused by remember { mutableStateOf(false) }
            val isSelected = selectedSection == section.title
            val shape = RoundedCornerShape(12.dp)

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(shape)
                    .onFocusChanged { focused = it.isFocused }
                    .background(
                        when {
                            focused -> Color(0xFF303B49)
                            isSelected -> Color(0xFF252F3C)
                            else -> Color.Transparent
                        }
                    )
                    .border(
                        width = if (focused) 2.dp else 1.dp,
                        color = if (focused) accent else Color.Transparent,
                        shape = shape
                    )
                    .then(
                        if (index == 0) Modifier.focusRequester(initialFocusRequester)
                        else Modifier
                    )
                    .clickable { onSelect(section.title) }
                    .focusable()
                    .padding(horizontal = 16.dp, vertical = 15.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Box(
                    modifier = Modifier
                        .width(4.dp)
                        .height(20.dp)
                        .clip(RoundedCornerShape(4.dp))
                        .background(if (isSelected) accent else Color.Transparent)
                )
                Text(
                    text = section.title,
                    modifier = Modifier.padding(start = 12.dp),
                    color = if (isSelected || focused) Color.White else muted,
                    fontSize = 16.sp,
                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium
                )
            }
        }

        Spacer(modifier = Modifier.weight(1f))
        Text(
            text = "Nastavení a zdroje
budou dostupné v menu.",
            color = Color(0xFF718094),
            fontSize = 11.sp,
            lineHeight = 17.sp
        )
    }
}

@Composable
private fun HeroPanel(
    selectedSection: String,
    description: String
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .height(205.dp)
            .clip(RoundedCornerShape(22.dp))
            .background(
                Brush.horizontalGradient(
                    colors = listOf(Color(0xFF273443), Color(0xFF19232F), Color(0xFF171D27))
                )
            )
            .padding(horizontal = 30.dp, vertical = 24.dp),
        verticalArrangement = Arrangement.SpaceBetween
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(9.dp)) {
            Text(
                text = if (selectedSection == "Domů") "VŠECHEN OBSAH. JEDNO MÍSTO." else "SOURBASE · $selectedSection",
                color = accent,
                fontSize = 12.sp,
                fontWeight = FontWeight.Bold,
                letterSpacing = 1.6.sp
            )
            Text(
                text = if (selectedSection == "Domů") "Vaše televize.
Po vašem." else selectedSection,
                color = Color.White,
                fontSize = 31.sp,
                lineHeight = 35.sp,
                fontWeight = FontWeight.Bold
            )
            Text(
                text = description,
                color = Color(0xFFC2CCD7),
                fontSize = 14.sp
            )
        }

        Text(
            text = "PŘIPOJENÍ ZDROJŮ A PŘEHRÁVÁNÍ BUDE NAVÁZÁNO V DALŠÍ FÁZI",
            color = Color(0xFF91A0B1),
            fontSize = 10.sp,
            letterSpacing = 1.sp
        )
    }
}

@Composable
private fun SectionCard(
    section: TvSection,
    onClick: () -> Unit
) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(16.dp)

    Column(
        modifier = Modifier
            .width(195.dp)
            .height(118.dp)
            .clip(shape)
            .onFocusChanged { focused = it.isFocused }
            .background(if (focused) Color(0xFF303E4D) else panel)
            .border(
                width = if (focused) 2.dp else 1.dp,
                color = if (focused) accent else Color(0xFF2D3947),
                shape = shape
            )
            .clickable(onClick = onClick)
            .focusable()
            .padding(16.dp),
        verticalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = section.eyebrow,
            color = accent,
            fontSize = 10.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 1.2.sp
        )
        Text(
            text = section.title,
            color = Color.White,
            fontSize = 20.sp,
            fontWeight = FontWeight.SemiBold
        )
    }
}
