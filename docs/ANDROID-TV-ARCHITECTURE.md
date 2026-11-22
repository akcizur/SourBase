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

The new `android-tv/` project is a native Android TV application:

- Kotlin + Jetpack Compose UI.
- TV-first navigation with visible D-pad focus states.
- AndroidX Media3 / ExoPlayer with HLS and DASH modules.
- App-private persistence and Android Keystore for any future secrets.
- User-provided HTTP/HTTPS IPTV and VOD sources; cleartext HTTP is permitted for legacy provider compatibility.

The first commit is a buildable UI and playback foundation. The home sections are navigation scaffolding, not yet connected to playlist or EPG data. No cloud account, subscription backend, DRM licensing service, or cross-device synchronization is claimed.

## Shared domain boundary

Keep source and catalog concepts independent from platform UI:

- `MediaEntry`: title, kind, description, artwork and optional playback source.
- `TvChannel`: stable channel ID, display name, stream, logo and group.
- `EpgProgramme`: channel ID, title, synopsis and start/end timestamps.
- `PlaybackSource`: URL, optional MIME type and request headers.

The Android models are an initial client-side contract. When the domain layer is extracted, preserve identifiers and field meaning across Windows, Web and Android instead of sharing platform-specific storage or UI code.

## Delivery phases

1. **Android TV foundation:** app shell, D-pad navigation, Media3 playback host, CI APK build.
2. **Source ingestion:** M3U/M3U8 import, source editing and app-private encrypted storage.
3. **EPG and VOD:** XMLTV parsing, channel matching, Xtream-compatible catalog integration and metadata shelves.
4. **Player UX:** live channel switching, audio/subtitle selection, catch-up only where the provider exposes a supported archive URL.
5. **Web companion:** responsive source/library management against an explicitly designed and authenticated API.
6. **Mobile companion:** reuse domain contracts and account/sync APIs; do not copy Windows DPAPI behavior.

## Build locally

Requirements: JDK 17, Android SDK platform 36, and Gradle 8.13.

```bash
cd android-tv
gradle :app:assembleDebug
```

The debug APK is written to `android-tv/app/build/outputs/apk/debug/app-debug.apk`.
