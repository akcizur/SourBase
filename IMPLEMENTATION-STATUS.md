# Research-to-code implementation status

## Applied in WAVE IPTV 1.6.0

- Local-first desktop architecture is retained: Go loopback host, embedded frontend and a single Windows x64 executable.
- The local HTTP listener remains bound to `127.0.0.1`.
- The complete local state snapshot is encrypted using Windows DPAPI current-user scope.
- Sensitive provider values (Xtream password and private SKTorrent feed URL) are stored in a separate DPAPI-protected file.
- Legacy 1.5.0 plaintext state is migrated automatically; the original file is deleted only after protected state and secrets have been written.
- The desktop build stops writing persistent app state into browser `localStorage` and removes legacy browser storage only after the protected write succeeds.
- qBittorrent credentials remain session-only; qBittorrent host validation remains loopback-only.
- Atomic state writes and migration/credential separation have regression tests.
- A Windows Actions build workflow now runs tests, vet and an x64 GUI build.

## Existing application capabilities retained

- M3U / M3U8 playlist import and switching.
- XMLTV EPG loading and channel matching.
- Xtream Live TV, VOD and series/episode catalog.
- Unified home, discover, library and player UI.
- Local media picker and authenticated HTTP Range endpoint.
- SKTorrent user-supplied RSS feed and qBittorrent Web UI integration.
- VLC / MPV external-player fallback for unsupported codecs and HLS.

## Not yet implemented from the research blueprint

- **SQLite:** current persistence is a DPAPI-encrypted JSON snapshot. There are no normalized SQL tables or indexed catalog queries yet. This is the next storage migration.
- **General plugin SDK:** SKTorrent and source adapters remain built-in modules; there is not yet a third-party plugin loader or signed plugin manifest.
- **Metadata enrichment service:** TMDB matching, poster caching and scheduled metadata refresh are not yet implemented as a complete provider service.
- **Embedded native playback engine:** WAVE currently uses the browser video element plus VLC/MPV fallback; libmpv/libVLC is not embedded.
- **Installer / signed releases:** the EXE is portable and unsigned. SmartScreen may warn until a trusted code-signing certificate is used.
- **Windows runtime validation:** the source includes a Windows-only DPAPI round-trip test, but this environment can only cross-compile it. A real Windows runner must execute it.

## Release verification

Performed locally:

- `go test ./...`
- `go vet ./...`
- JavaScript syntax check with `node --check`
- Windows x64 test-binary cross-compile
- Windows x64 GUI EXE cross-build
- PE header inspection of the generated EXE

Not performed here:

- opening the executable on a physical Windows desktop
- executing the DPAPI round-trip on Windows
- connecting a real IPTV/Xtream/SKTorrent account
- validating stream playback against a specific provider


## Multi-platform research follow-up — 2026-11-22

### Android TV — implemented

- Native Kotlin / Compose for TV navigation shell with TV Material 3, D-pad focus states and initial focus on Domů.
- Media3 / ExoPlayer playback host with HLS and DASH modules. `PlaybackSource` accepts validated HTTP(S) URL, optional MIME hint and request headers; the player uses 15-second connect and 30-second read timeouts.
- Remote M3U/M3U8 loader and parser: channel ID, name, logo, group, tvg-id and stream URL, stable IDs and warnings for invalid entries.
- Remote XMLTV loader and secure XMLTV parser: channel display names, programme title, description, categories, language and start/stop times with timezone offsets. Missing offsets are interpreted as UTC; invalid time ranges are skipped.
- XMLTV matching uses `tvg-id` / XMLTV channel ID. The Live TV view displays the current programme and includes a six-slot EPG timeline with 30-minute slots (a three-hour window). Selecting the currently airing programme starts its live channel; future programme selection does not claim archive playback.
- M3U and XMLTV URL settings are encrypted at rest using AES-256-GCM and a non-exportable Android Keystore key. App-private preferences contain only the IV/ciphertext record and Android backup is disabled. If the record cannot be decrypted, it is preserved until the user explicitly clears it.
- Unit tests cover playback URL validation, M3U parsing, XMLTV time offsets, matching and malformed/unsafe XML handling.

### CI verification — commit `4128ed981d5e3fa8de767945412c8751a13f8aed`

- **Android TV:** [workflow run 44975689815](https://github.com/akcizur/SourBase/actions/runs/44975689815) completed successfully. SDK setup, Gradle setup, Kotlin compile, `:app:assembleDebug`, `:app:testDebugUnitTest` and APK upload all succeeded. Artifact: `sourbase-android-tv-debug`, 15,054,218 bytes, SHA-256 `1bbff2b6c50484b7bc29a766f0f52da19b3d2c9ab4f9ca1c2402161852345734`.
- **Windows:** [workflow run 44975689835](https://github.com/akcizur/SourBase/actions/runs/44975689835) completed its Go unit tests, `go vet`, embedded UI JavaScript syntax check, Windows x64 EXE build and artifact upload successfully. Artifact: `sourbase-windows-x64`, 2,937,466 bytes, SHA-256 `e95ed1db966fd870f4034f1b0180e3a842a9df470b7ab3918c229f239642e4b2`.
- These CI results prove compilation, unit tests and artifact creation. They do not prove manual operation on a physical Android TV device or Windows desktop.

### Still not implemented

- Xtream login and Live/VOD/Series catalog integration in Android TV.
- Persistent full channel/VOD catalogue, favorites, watch history, resume sync and unified Live/VOD/EPG search.
- Local M3U/XMLTV file picker/import; current Android source import is from remote HTTP(S) URLs.
- EPG date navigation beyond the current three-hour window, station logos inside the EPG grid, recording, timeshift or catch-up.
- Advanced player overlay for audio/subtitle track selection, quality, channel zapping and error recovery.
- Android device/emulator UI tests for full D-pad focus traversal and physical TV playback smoke tests.
- Standalone Web client, Android phone, iOS/iPadOS and tvOS clients.
- Public authenticated backend, cloud sync, user accounts, subscriptions and real DRM licensing infrastructure.

### Security and compatibility boundaries

- Android source URLs are encrypted at rest; the current store does not yet persist Xtream passwords or a general credentials vault.
- HTTP is accepted for legacy IPTV providers, but HTTP traffic is not encrypted in transit; HTTPS is preferred. The current remote loader follows HTTP redirects using the platform connection behavior; redirect downgrade protection is a follow-up hardening item.
- Windows DPAPI files remain Windows-only and are never shared with Android or Apple clients.
- Catch-up/timeshift must remain unavailable unless the selected provider/channel reports a supported archive URL or API. DRM, JWT subscriptions and device limits remain architecture-only until real licensing and backend services exist.
- Windows CI remains green, but physical Windows launch, DPAPI round-trip and provider-specific playback still require testing on a Windows machine.
