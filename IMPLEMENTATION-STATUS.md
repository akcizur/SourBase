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
