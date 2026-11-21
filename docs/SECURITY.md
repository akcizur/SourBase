# Security model

- Bind only to `127.0.0.1`.
- Generate a cryptographically random bearer token per process.
- Require the token for library reads and all write operations.
- Bound imported playlist size and parser line length.
- Do not fetch stream URLs while importing M3U; playback is an explicit user action.
- Windows state is DPAPI protected for the current user. Other operating systems currently store state with restrictive permissions but without encryption.
- Stream URLs can embed provider credentials. Treat the local data file as sensitive.
- Do not bypass tracker authentication, DRM, or provider access controls.
- Before adding remote URL imports, implement SSRF protections, timeouts, TLS verification and explicit host permissions.
