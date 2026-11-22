# Sourbase Android TV direction

## Product target

Sourbase is moving toward an **Android TV-first** product for IPTV and VOD. The Web client is the second surface for source and library management. A mobile companion is a later phase. The existing Windows executable remains supported during this transition.

## Existing implementation (preserve)

- Windows desktop host is a Go executable with an embedded HTML UI.
- The host binds only to `127.0.0.1`; it is not a LAN or cloud API.
- Windows state and credentials are protected with DPAPI and remain in the legacy `%AppData%\\WAVE IPTV\\` directory for upgrade compatibility.
- The current HTML UI contains working IPTV, XMLTV/EPG, Xtream VOD, local media, RSS and qBittorrent flows.

Do not expose the loopback API to the network and do not attempt to read Windows DPAPI files from Android.

## Android TV client

The `android-tv/` project is a native Android TV application built with Kotlin (AGP built-in Kotlin), Jetpack Compose for TV, TV Material 3 and AndroidX Media3 / ExoPlayer with HLS and DASH modules. Its current implementation includes:

- TV-first navigation with visible D-pad focus states, initial focus on Domů, and grouped focus traversal.
- Remote M3U/M3U8 URL import and parsing of channel ID, display name, logo, group and stream URL.
- Remote XMLTV import and parsing of channel display names, programme title/description/category/language and start/stop timestamps with timezone offsets.
- Channel matching through the playlist `tvg-id` / XMLTV channel ID rather than fuzzy display-name matching.
- A six-slot, 30-minute-per-slot EPG timeline (three hours) with a fixed channel column. The current programme can launch its channel; selecting a future programme does not claim catch-up playback.
- Validated HTTP(S) playback sources with optional MIME hints and per-source request headers.
- AES-GCM encryption of M3U/XMLTV source URLs using a non-exportable Android Keystore key. Only IV/ciphertext are held in app-private preferences; Android backup is disabled. Unreadable encrypted data is preserved until explicit user reset.

The Android Home sections outside Live TV remain navigation scaffolding. Xtream login and catalog, VOD/series, persistent channel catalogue, favourites, watch history, unified search, local playlist file import, advanced player overlay and catch-up are not yet implemented. HTTP is accepted for legacy IPTV providers, but it is not encrypted in transit; HTTPS is preferred. No cloud account, subscription backend, DRM licensing service or cross-device synchronization is claimed.

## Shared domain boundary

Keep source and catalog concepts independent from platform UI:

- `MediaEntry`: title, kind, description, artwork and optional playback source.
- `TvChannel`: stable channel ID, display name, stream, logo and group.
- `EpgProgramme`: channel ID, title, synopsis and start/end timestamps.
- `PlaybackSource`: URL, optional MIME type and request headers.

The Android models are an initial client-side contract. M3U and XMLTV parser logic lives in the data layer, while remote downloads run on `Dispatchers.IO` with response-size limits and HTTP(S)-only URL validation. Source URLs are encrypted at rest; this does not currently encrypt transport for legacy `http://` URLs, nor store provider passwords. When the domain layer is extracted, preserve identifiers and field meaning across Windows, Web and Android instead of sharing platform-specific storage or UI code.

## Delivery phases

1. **Android TV foundation — implemented and CI-verified:** app shell, D-pad navigation, Media3 playback host and debug APK artifact.
2. **M3U/XMLTV source ingestion — implemented and CI-verified:** remote URL import, parser tests, XMLTV timezone parsing and channel ID matching; source URLs encrypted at rest with Android Keystore.
3. **Basic Live TV/EPG — implemented and CI-verified:** channel list, current programme label and six-slot three-hour EPG timeline; current programme selection starts its live channel.
4. **VOD/player UX — next:** Xtream-compatible catalog, movies/series/episodes, audio/subtitle/quality overlay and catch-up only where the provider exposes a supported archive URL.
5. **Web companion:** responsive source/library management against an explicitly designed and authenticated API.
6. **Mobile companion:** reuse domain contracts and account/sync APIs; do not copy Windows DPAPI behavior.

## Build locally

Requirements: JDK 17, Android 17 preview SDK platform 37.0 (SDK channel 3), Android Gradle Plugin 9.2.0 and Gradle 9.4.1.

```bash
cd android-tv
gradle :app:assembleDebug
```

The debug APK is written to `android-tv/app/build/outputs/apk/debug/app-debug.apk`. The latest verified artifact is `sourbase-android-tv-debug` from [workflow run 44975689815](https://github.com/akcizur/SourBase/actions/runs/44975689815), built from commit `4128ed981d5e3fa8de767945412c8751a13f8aed`. CI ran `assembleDebug`, `testDebugUnitTest` and uploaded the APK. Manual remote-control and real-stream checks on Android TV hardware are still required.
