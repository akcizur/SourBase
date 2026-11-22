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

### Android TV changes added

- Gradle repository configuration now tries a Google Cloud-hosted Maven Central mirror before Maven Central to reduce the observed HTTP 429 impact. The mirror is a fallback only; Maven Central remains configured.
- Media3 player now uses the PlaybackSource MIME hint and request headers, with 15-second connect and 30-second read timeouts.
- Added HTTP(S) playback URL validation: empty/relative/non-HTTP URLs, missing hosts, embedded user-info credentials, malformed MIME values and CR/LF header injection are rejected.
- Added JVM unit tests for accepted HTTP(S) sources and rejected invalid URLs/headers.
- Added the platform UX/technical plan at docs/PRODUCT-UX-AND-PLATFORM-PLAN.md.
- Updated the Android toolchain for Compose 1.12.1: AGP 9.2.0, Gradle 9.4.1, compileSdk 37 (Android 17 preview package android-37.0 via SDK channel 3) and AGP built-in Kotlin; targetSdk remains 35 to avoid opting into new runtime behavior in this tooling-only change.

### CI verification boundary

The dependency mirror reduced the Maven Central HTTP 429 blockage. The next observed run reached :app:checkDebugAarMetadata and exposed a real version alignment issue: Compose 1.12.1 requires compileSdk 37 and AGP 9+. The subsequent CI run showed that Android 17's SDK package is a preview package, not the stable `platforms;android-37` path. The project now uses AGP 9.2.0, Gradle 9.4.1, AGP built-in Kotlin and installs `platforms;android-37.0` from SDK channel 3. No APK is confirmed until the updated CI build and artifact list are checked.

The Windows workflow had succeeded on commit bb3b9e48, running Go tests, go vet, embedded JavaScript syntax validation and the Windows x64 build. This does not replace a physical Windows launch, DPAPI round-trip or provider-specific playback test.

### Explicitly still missing

- M3U/M3U8 and Xtream import in Android TV.
- XMLTV parser, reliable channel matching and TV EPG 2D grid in Android TV.
- Persistent Android catalog, encrypted provider credential storage, watch history, favourites and unified Live/VOD/EPG search.
- Production player overlay for available audio/subtitle tracks, quality and channel switching.
- Standalone web client, Android phone app, iOS/iPadOS and tvOS clients.
- Public authenticated backend, cloud sync, account/subscription and real DRM licensing infrastructure.

Catch-up/timeshift must remain hidden unless the selected provider/channel reports a supported archive URL or API. DRM, JWT subscriptions and device limits remain architecture-only until real licensing and backend services exist. Windows DPAPI files remain Windows-only; they must not be shared with Android or Apple clients.
