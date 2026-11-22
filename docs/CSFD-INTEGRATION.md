# ČSFD metadata integration — technical record

**Record date:** 2026-11-22  
**Status:** implemented in the Windows desktop client; CI verification pending  
**Scope:** optional enrichment of the local movie and series catalogue. This is not an official ČSFD integration or partnership.

## 1. Product decision

ČSFD is an enrichment source, not a playback provider.

- Sourbase remains the owner of the stream URL, Xtream identifiers, source category and playback behaviour.
- ČSFD supplies a separate identity and descriptive metadata: title, year, aggregate rating, rating count, poster, genres, countries, description, director and cast.
- Matching is explicit and manual. A user opens a movie or series, chooses **Spárovat s ČSFD**, reviews search results and confirms the exact record.
- No automatic full-catalogue scan, sequential ID crawl, review harvesting, user-profile harvesting or bulk mirror is implemented.
- A match can be replaced or removed. Removing a match does not delete a Sourbase title or affect its stream.
- A saved match is reused locally; browsing the same matched title does not issue another ČSFD request.

The manual workflow is deliberate: title-only matching is ambiguous (remakes, same-title films, different cuts, series/seasons and regional releases). A false automatic match would be worse than asking for confirmation.

## 2. Upstream adapter and verified contract

The integration uses [bartholomej/node-csfd-api](https://github.com/bartholomej/node-csfd-api), an open-source, **unofficial scraper and REST wrapper**. The reviewed upstream package metadata reports version **5.16.1** and an MIT licence for the adapter source. That licence does not grant rights to republish ČSFD's database, descriptions, photographs or posters.

The upstream REST contract used by Sourbase is:

| Upstream endpoint | Purpose | Response fields used |
|---|---|---|
| `GET /` | Service health/version; no title lookup | `name`, `version`, `links` |
| `GET /search/:query?language=cs` | Search | `movies[]`, `tvSeries[]`; `id`, `title`, `year`, `type`, `url`, `poster`, `colorRating` |
| `GET /movie/:id?language=cs` | Full movie or series detail | `id`, `title`, `year`, `rating`, `ratingCount`, `poster`, `photo`, `genres[]`, `origins[]`, `descriptions[]`, `creators.directors[]`, `creators.actors[]` |

Search results also contain users; Sourbase deliberately ignores them. Search results can contain a transparent placeholder poster; the confirmed detail response is preferred for the final artwork.

Upstream reference: [node-csfd-api README and REST API reference](https://github.com/bartholomej/node-csfd-api#rest-api).

## 3. Data flow and trust boundaries

```text
Sourbase UI
  │ local app session token
  ▼
Go loopback host (127.0.0.1)
  │ /api/csfd/status
  │ /api/csfd/search?q=...
  │ /api/csfd/movie/{numeric-id}
  │ optional Authorization: Bearer + X-API-Key
  ▼
node-csfd-api container (127.0.0.1:3000 only)
  │
  ▼
ČSFD public pages
```

The browser does not contact ČSFD directly. It calls the existing authenticated Go loopback host. The Go host calls the local adapter, so browser CORS is not used and the upstream key is not exposed in browser storage.

### Sourbase-local routes

All three routes require the existing Sourbase session token and accept GET only.

| Sourbase route | Behaviour |
|---|---|
| `/api/csfd/status?token=…` | Checks the local adapter root endpoint. It does not run a ČSFD search. |
| `/api/csfd/search?q=…&token=…` | Validates a 2–100 character query and forwards it to the upstream search endpoint in Czech. |
| `/api/csfd/movie/{id}?token=…` | Accepts only a 1–12 digit numeric ČSFD ID and returns the full upstream detail. |

The local adapter URL is restricted to `localhost`, `127.0.0.1` or `::1`; credentials, query strings and non-root paths are rejected. Redirects must keep the exact local scheme, host and port. This prevents the setting from becoming a general network proxy.

## 4. Request and response safeguards

- One upstream request slot is reserved every **1.2 seconds** per running Sourbase process. Concurrent requests are queued rather than sent together.
- Upstream HTTP timeout: **25 seconds**.
- Maximum upstream JSON body: **4 MiB**.
- Redirect limit: **3**, with an exact-origin check.
- Only JSON responses are accepted.
- 401, 404, 429, 502 and 503 conditions are converted to readable local errors.
- The adapter API key is sent in both `Authorization: Bearer <key>` and `X-API-Key`, matching the upstream default configuration.
- The health check uses the adapter's `/` route; it does not scrape a title.
- The UI displays at most 20 numeric-ID search results per query.
- Search runs only after a user opens the match dialog. There is no background metadata sweep or mass catalogue refresh.

The 1.2-second spacing is a Sourbase-side safeguard, not a statement about ČSFD's own permitted request rate. If the upstream service returns 429/503 or its operator changes access rules, users must stop and retry later; no bypass, identity rotation or challenge circumvention is implemented.

## 5. Metadata mapping and precedence

Each confirmed match is stored under `state.csfdMetadata["<kind>:<source-id>"]`, where kind is `movie` or `series`.

| Stored field | Source |
|---|---|
| `csfdId`, `url` | Confirmed ČSFD search/detail result |
| `title`, `year` | ČSFD detail, with search result fallback |
| `rating`, `ratingCount` | ČSFD aggregate rating and number of ratings |
| `poster` | ČSFD detail poster; only HTTPS `image.pmgstatic.com` URLs or the tiny transparent GIF placeholder are accepted |
| `genres`, `origins` | ČSFD detail |
| `description` | First available detail description, capped at 5,000 characters |
| `directors`, `actors` | Names from the detail's creator groups, capped to 5 and 8 respectively |
| `matchedAt` | Local timestamp when the user confirmed the match |

Display precedence:

1. A confirmed ČSFD poster is preferred for that matched movie/series.
2. If no usable ČSFD poster is present, Sourbase falls back to the existing IPTV catalogue artwork.
3. A ČSFD description is preferred in the title detail; otherwise the existing IPTV description remains.
4. The original stream title, stream URL, provider ID, categories and player behaviour are not overwritten.
5. The title detail shows the ČSFD rating/link and compact genres, directors and cast. The link is restricted to HTTPS `csfd.cz` / `www.csfd.cz`.

Poster files are not copied into Sourbase or mirrored to a new server; only their validated URL is stored. The upstream content remains hosted by its source.

## 6. Local configuration and operation (Windows)

### Docker Desktop — recommended

1. Install and start Docker Desktop.
2. In PowerShell, from the SourBase repository folder, run:

   ```powershell
   powershell -ExecutionPolicy Bypass -File .\Start-Sourbase-CSFD.ps1
   ```

3. The script pulls `bartholomej/node-csfd-api`, creates a random 256-bit API key when one does not exist, and starts the container with the host port bound to **127.0.0.1:3000 only**.
4. The key is stored encrypted for the current Windows account using PowerShell DPAPI at `%LOCALAPPDATA%\Sourbase\csfd-api-key.dpapi`. The script prints the key for copying into Sourbase.
5. In Sourbase **Nastavení → ČSFD · Filmová metadata**, keep the API URL as `http://127.0.0.1:3000`, paste the key and select **Uložit**.
6. Select **Ověřit spojení**. This checks the local adapter root, not a ČSFD title search.
7. Open any movie or series, choose **Spárovat s ČSFD**, verify the correct title/year and confirm it.

Stop the service:

```powershell
powershell -ExecutionPolicy Bypass -File .\Stop-Sourbase-CSFD.ps1
```

The stop script removes the container but keeps the DPAPI-protected key for the next start. The adapter can also be inspected with `docker logs sourbase-csfd`.

The launcher uses the Docker image's current `latest` tag. For a reproducible deployment, pin a verified immutable image digest or a published version tag after checking the registry; do not assume that the moving `latest` tag is immutable.

## 7. Storage and privacy

- The adapter URL and API key are part of the existing Windows protected-credentials record (`credentials.dpapi`), encrypted with the existing DPAPI mechanism.
- Match records are part of the existing local state snapshot (`state.dpapi`), also protected with DPAPI on Windows.
- The GitHub Pages/static browser build does not have the authenticated Go loopback service and therefore cannot use this local connector. A future standalone web client needs a separately designed authenticated backend; it must not embed the ČSFD API key in static JavaScript.
- The connector is optional. If it is stopped, existing playback, playlists, IPTV, EPG, the local library and existing catalogue artwork continue to work.
- Clearing Sourbase's local data clears the CSFD match records and connector credentials as part of the existing clear-data action.

## 8. Legal, licensing and operating limits

This is **not an official ČSFD API**. The upstream package describes itself as an unofficial scraper. The MIT licence applies to the adapter source code, not to the third-party database, reviews, descriptions, photographs or poster images.

Use it only where you have a lawful basis and permission under the current ČSFD terms. This implementation is intentionally limited to individual, user-initiated lookups for a local personal catalogue. It does not circumvent login, anti-bot challenges, access controls or rate limits. Do not use it to build or distribute a competing copy of the ČSFD catalogue, mirror posters, harvest user data or resell metadata. If ČSFD blocks access, returns 429/503 or changes its terms, disable the connector and obtain permission or a licensed data source instead. This record is an engineering decision log, not legal advice.

## 9. Verification checklist

| Check | Expected |
|---|---|
| `go test ./...` | URL restrictions, authorization, credentials, status/search/detail proxy tests pass |
| `go vet ./...` | No Go static-analysis findings |
| Embedded JavaScript syntax | `new Function(script)` / CI JavaScript syntax step passes |
| Search mock | Czech path/query and API key headers are correct |
| Detail mock | Movie/series ID route returns full JSON fixture |
| Redirect mock | Redirect to another origin is rejected |
| Browser/manual | Start local Docker service, test connection, search, confirm a match, verify poster/rating/description, remove match |
| Windows runtime | Run the EXE and Docker launcher on a real Windows machine |

Mock tests prove the Sourbase-to-adapter contract. They do **not** prove ČSFD currently permits scraping, that a live scrape will always succeed, or that the local Docker image will work on every machine. A real Windows/manual smoke test remains a release gate.

## 10. Deliberately deferred

- Automated title matching across the entire imported catalogue.
- Background or scheduled scraping.
- Search-result and image binary caching.
- Bulk metadata refresh and chart ingestion.
- Creator biography, user ratings/reviews, profiles or other personal data.
- Standalone web / GitHub Pages / Android TV support.
- A licensed official data feed or commercial redistribution rights.

These are deferred to keep the integration low-volume, explicit, auditable and independent of Sourbase playback.
