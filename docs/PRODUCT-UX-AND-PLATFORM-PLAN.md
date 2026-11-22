# Sourbase — platformní UX a technický plán

> Produktový cíl: hybridní mediální centrum VOD + IPTV. Zásada: obsah na prvním místě, nastavení na vyžádání. Obsah má být spuštěný v jednom až dvou krocích.

## 1. Platformy a jejich role

| Platforma | Cílová role | Doporučená technologie | Stav v repozitáři |
|---|---|---|---|
| Windows | Zachovat současný desktopový klient | Go host + vložené HTML UI + DPAPI | Existující aplikace; Windows workflow na commitu před aktuálními Android změnami prošel |
| Android TV | Primární nová platforma | Kotlin, Compose for TV, AndroidX Media3 | TV shell, remote M3U/XMLTV import, channel matching, six-slot EPG and Media3 live playback; source URLs encrypted with Keystore; CI APK passes |
| Web / desktop browser | Druhá klientská plocha | Samostatný web + autentizované API + Shaka nebo hls.js | Samostatný browser klient a síťové API chybí; web/index.html patří Windows aplikaci |
| Android phone | Mobilní klient | Nativní Android nebo později KMP | Chybí |
| iPhone / iPad | Mobilní klient | SwiftUI + AVPlayer | Chybí |
| Apple TV / tvOS | Velká obrazovka v Apple ekosystému | SwiftUI/tvOS focus + AVPlayer | Chybí |

**Důležitá hranice:** vložené webové rozhraní ve Windows aplikaci není samostatná webová aplikace. Go server nyní naslouchá na 127.0.0.1; není to veřejné API pro mobilní telefony, Apple TV ani jiné počítače.

## 2. Co je implementováno a co je pouze základ

### Windows

Současná Windows větev zachovává Go host, lokální webové UI, DPAPI úložiště, M3U/M3U8, XMLTV EPG, Xtream Live/VOD/series, lokální média, RSS a volitelné qBittorrent napojení. Windows workflow na kontrolovaném commitu bb3b9e48 úspěšně provedl Go testy, go vet, kontrolu JavaScript syntaxe a Windows x64 build. Fyzické spuštění, DPAPI round-trip a streamy konkrétního poskytovatele vyžadují samostatný test na Windows zařízení.

### Android TV

V android-tv je Kotlin/Compose TV shell, datové modely, Media3 player, vzdálený M3U/M3U8 import, XMLTV parser a matching podle `tvg-id`/XMLTV ID. Obrazovka Live TV vypisuje kanály, aktuální pořad a 2D EPG timeline o šesti půlhodinových slotech (tři hodiny). Výběr právě vysílaného pořadu spouští živý stream. M3U/XMLTV URL se ukládají šifrovaně pomocí AES-GCM a Android Keystore. Home mimo Live TV zůstává preview; Xtream autentizace, VOD/series katalog, full-text search, favourites, watch history, local file import a pokročilý player overlay chybí.

Toolchain používá AGP 9.2.0, Gradle 9.4.1, compileSdk 37 (Android 17 preview balíček `android-37.0`, SDK channel 3), JDK 17 a AGP built-in Kotlin. Android workflow na commitu `4128ed981d5e3fa8de767945412c8751a13f8aed` úspěšně sestavil APK, spustil unit testy a nahrál artefakt. Fyzické ověření na Android TV zařízení však zatím neproběhlo.

### Chybějící platformy a služby

Samostatný web, mobilní Android, iOS/iPadOS a tvOS nejsou implementované. Chybí také veřejné/síťové API, cloudová synchronizace, účty, předplatné a licenční infrastruktura. Tyto položky nesmí být popisovány jako hotové funkce.

## 3. Sdílené obrazovky a core user flows

1. **Výběr profilu / přihlášení:** lokální profily nejprve; dětský profil a rodičovský PIN až po doplnění odpovídajících pravidel. Online účet a předplatné až po vybudování skutečného backendu.
2. **Domů:** Pokračovat ve sledování, oblíbené TV kanály, Právě v TV, doporučené filmy a seriály.
3. **Detail VOD:** hero/poster, název, rok, délka, popis, dostupné hodnocení, Přehrát/Pokračovat, u seriálu výběr řady a epizody.
4. **Detail pořadu / EPG:** stanice, pořad, začátek/konec, popis a stav vysílání; Přehrát živě, Přehrát od začátku jen s potvrzenou podporou catch-up.
5. **Univerzální hledání:** jeden dotaz přes názvy kanálů, aktuální/budoucí pořady, filmy, seriály a epizody.
6. **Přehrávací relace:** společný aplikační kontrakt pro spuštění obsahu, ale platformně specifický player, kodeky a ovládání.
7. **Profil / nastavení:** zdroje, jazyk, vzhled, rodičovská kontrola a správa zařízení. Online správa účtu se zobrazí až po implementaci backendu.

## 4. Wireframe mapa podle platformy

### Android TV

- Levý vysouvací panel pro Domů, Živá TV, Filmy, Seriály, Knihovna a Hledat; focus D-padu musí být vždy viditelný.
- Horní přibližně třetina Home je Hero. Pod ní řady Pokračovat, Právě v TV, Oblíbené kanály, Filmy a Seriály.
- Live TV karta má poměr 16:9, logo stanice, název pořadu a jemný progress. VOD poster má poměr 2:3 a nejvýše jeden badge.
- EPG je 2D mřížka: loga a názvy kanálů ve fixním levém sloupci, časová osa vodorovně. Šipky mění focus mezi pořady a posouvají časovou osu; vybraný pořad se zobrazí v informační oblasti.
- OK na aktuálním kanálu/pořadu spouští živý stream bez mezilehlého detailu. Popis je vedlejší akce Info nebo dlouhé podržení.
- tvOS má vlastní focus a ovládání ovladačem; nesdílet Android TV UI pouhým změněním velikosti. Sdílet doménové kontrakty a use-case, ne platformní komponenty.

### Web — desktopový prohlížeč

- Horní navigace Domů, Živá TV, Filmy, Seriály, Knihovna a univerzální hledání.
- Hero banner s hlavní akcí Přehrát/Pokračovat a sekundární akcí Více informací.
- Karusely s pěti až sedmi kartami v řadě; podpora myši, klávesnice, focus i hover stavu.
- EPG jako široká časová tabulka s filtrem žánru a volbou data.
- Player má časovou osu, kvalitu a klávesové zkratky; seek náhledy jen tehdy, pokud je lze ze zdroje získat.
- Samostatný browser klient potřebuje definované API, autentizaci, CORS a ochranu proti CSRF. Lokální Windows loopback server není vzdálený backend.

### Mobil — iOS a Android telefon

- Spodní navigace Domů, Živá TV, Filmy, Seriály, Hledat.
- Horní řádek Právě v TV s 16:9 kartami, logem stanice, názvem pořadu a progress barem; pod ním poster grid ve dvou až třech sloupcích.
- Kompaktní horizontální filtry žánru a řazení; detail titulu s výraznou akcí Přehrát/Pokračovat.
- Player má dotykové ovládání, výběr dostupného audia/titulků, kvalitu, rychlé přepnutí kanálu a PiP pouze tehdy, když jej povoluje operační systém a zdroj.

## 5. Progressive disclosure — nastavení na vyžádání

| Volba | Umístění | Pravidlo |
|---|---|---|
| Přidat / spravovat zdroj | Profil nebo ozubené kolo | M3U URL/soubor, Xtream, XMLTV, lokální média; zobrazit stav importu |
| CZ / EN | Profil → Nastavení | Zachovat stávající Windows lokalizaci; nové platformy ověřovat samostatně |
| Light / Dark | Profil → Vzhled | TV může mít Dark jako výchozí, ale uživatel si volí |
| Audio / titulky | Player overlay | Zobrazit jen skutečně dostupné tracky |
| Kvalita | Player overlay | Auto a jen manifestem/playerem dostupné varianty |
| Rodičovský zámek | Profil → Nastavení | PIN a oddělený dětský profil; zatím návrh, nikoli funkce |
| qBittorrent | Windows nastavení | Zachovat v Windows klientovi, dokud nebude bezpečně navržen serverový konektor |

Hlavní navigace zůstává obsahová, bez samostatné položky Nastavení. Overlay přehrávače se po nečinnosti skryje, ale musí jít snadno znovu vyvolat. Cíl je spuštění obsahu v jednom až dvou krocích.

## 6. Přehrávání, EPG a Catch-up

| Platforma | Přehrávač | Podmínky |
|---|---|---|
| Android / Android TV | AndroidX Media3 ExoPlayer | HLS/DASH moduly; ABR podle manifestu a zařízení |
| iOS / iPadOS / tvOS | AVPlayer / AVFoundation | HLS, kodeky a systémové funkce závisí na OS a obsahu |
| Web | Shaka Player nebo hls.js | Shaka pro HLS/DASH přes MSE/EME tam, kde je podpora; hls.js pro HLS v prohlížečích s MSE |
| Windows současná verze | Browser video element + VLC/MPV fallback | Zachovat současné chování, dokud nebude ověřen embedded player |

HLS/DASH a ABR se řídí skutečným manifestem. LL-HLS není přepínač v UI: funguje jen při podpoře serveru, playlistu i klienta. Audio, titulky a kvalita se zobrazují pouze podle tracků a schopností zdroje.

### XMLTV a channel matching

- Parser musí zpracovat channel, programme, jazyk, popis, kategorii, začátek a konec včetně časového offsetu.
- Párování: nejprve stabilní tvg-id/XMLTV channel ID, pak explicitní aliasy, nakonec pouze návrh podle názvu. Nepárovat automaticky nevratně jen podle podobného názvu.
- Interně ukládat jednoznačný okamžik (UTC epoch); v UI vykreslit místní čas včetně DST. Program s end menším nebo rovným start je validační chyba.
- EPG grid musí virtualizovat řádky a časová okna; rozsáhlý XMLTV feed se nesmí vykreslovat celý v jednom UI stromu.

### Catch-up / timeshift

Catch-up a timeshift jsou capability konkrétního poskytovatele nebo kanálu. Akci Přehrát od začátku zobrazit jen tehdy, když poskytovatel pro konkrétní program nabízí platný archivní odkaz nebo podporované API. Nevytvářet univerzální URL šablonu pro všechny poskytovatele.

## 7. Sdílené doménové kontrakty bez riskantního přepisu Windows

Go Windows, Kotlin Android a HTML/JavaScript dnes nemají společný doménový modul. První krok proto není přesun Windows do Kotlinu ani velký KMP refaktor:

1. Definovat verzovaná JSON Schema / OpenAPI schémata pro MediaEntry, PlaybackSource, TvChannel, EpgProgramme, SourceDefinition, PlaybackCapabilities a WatchProgress.
2. Přidat fixture JSON. Každý klient musí ze stejných fixtures vytvořit stejné ID, timestampy a výsledek párování.
3. Go, Kotlin a TypeScript použijí generované nebo kontrolované typy; Swift typy doplnit s Apple klientem.
4. Kotlin Multiplatform zvážit později jen pro nové Android+iOS parsery/use-case, pokud přínos převáží cenu multiplatformního build systému. KMP automaticky nesjednotí Go Windows host ani browser UI.

Windows DPAPI data se nesmějí číst z Androidu/iOS. Budoucí Android citlivé údaje patří do Android Keystore; Apple do Keychain. Web vyžaduje vlastní autentizaci a bezpečnostní vrstvu. Go listener na 127.0.0.1 se nemá vystavovat do LAN bez nové bezpečnostní vrstvy.

## 8. DRM, účty a předplatné — pouze budoucí architektura

Budoucí návrh může zvažovat Widevine, FairPlay a PlayReady, ale reálné nasazení vyžaduje licenční vztah, kompatibilní balení obsahu, licenční server, klíče a testovací obsah. Dokud tato infrastruktura neexistuje, Sourbase nesmí tvrdit, že nabízí DRM chráněný obsah, předplatné, ověřená zařízení, JWT autorizaci nebo limity souběžných relací. Pro osobní playlisty je cílem bezpečně uchovávat přístupové údaje a respektovat práva poskytovatele.

## 9. Prioritizovaný backlog a akceptační kritéria

### P0 — stabilní Android TV základ

1. **Build — hotovo v CI:** Gradle dependency resolution, Kotlin compilation, unit tests, assembleDebug a APK upload. Akceptace CI splněna v run 44975689815; fyzické TV ověření stále čeká.
2. **PlaybackSource — hotovo v CI:** HTTP(S), MIME hint, hlavičky, chybějící host, userinfo a CR/LF validace s JVM testy.
3. **M3U — hotovo v CI pro remote URL:** parser channel ID, název, group-title, logo, tvg-id a stream URL; fixture tests. Lokální file picker zatím chybí.
4. **XMLTV — hotovo v CI:** parser timezone offsetů, UTC fallback, validace intervalů a channel matching přes ID; fixture tests.
5. **Basic Live TV/EPG — implementováno:** remote source form, list kanálů, current programme, šestipolohová timeline. Manuální D-pad/Player test na Android TV hardware čeká.
6. **1–2 kroky:** aktuální pořad spouští live stream; budoucí pořad zobrazí informaci, ale catch-up není implementován. Home/VOD toky čekají na katalog.
7. **Windows regrese — CI prošla:** Go tests, go vet, embedded JavaScript check a Windows x64 build na commitu 4128ed; DPAPI/device runtime stále vyžaduje fyzický test.

### P1 — katalog a další klienti

1. Xtream import Live/VOD/Series, normalizace kategorií a epizod.
2. EPG mřížka, hledání napříč Live/VOD/EPG a profilová nastavení.
3. Player overlay pro audio/titulky/kvalitu pouze podle dostupných tracků a capabilities.
4. Samostatný webový klient s explicitním backend API.
5. iOS + tvOS klient s AVPlayer a Android mobilní UI.
6. CZ/EN a Light/Dark samostatně ověřit na každé skutečně implementované platformě.

### P2 — funkce závislé na poskytovateli a produkční službě

1. Catch-up/timeshift a nahrávání pouze pro zdroje s archivním API.
2. DRM/licence, účty, subscription a limity zařízení až po skutečné obchodní a technické integraci.
3. Synchronizace knihovny a watch progress mezi zařízeními až po implementaci backendu.
4. Metadata enrichment, postery, doporučování a pokročilá analytika.
5. Přístupnost, VoiceOver/TalkBack a další výkonové optimalizace.

## 10. Testovací strategie a release gate

- **Unit:** M3U parser, XMLTV parser, UTC/timezone převody, channel matching, URL a header validator, serializace.
- **Contract:** stejná fixture JSON pro Go/Kotlin/TypeScript/Swift musí dát stejné stabilní identifikátory a časové hodnoty.
- **Android:** JVM unit testy, Compose UI testy pro D-pad, instrumentovaný playback test na emulátoru/zařízení, HLS/DASH test na povoleném testovacím streamu.
- **Apple:** XCTest pro modely a parsery, UI focus testy a HLS test na simulátoru/zařízení.
- **Web:** browser compatibility, HLS/DASH podle MSE/EME a ověřené browser matrix.
- **Windows:** Go test/vet, JS syntax check a Windows runner; fyzický smoke test pro DPAPI i přehrávání.
- Platforma se nesmí označit za hotovou jen proto, že projde kompilace: vyžaduje doložený build, test navigace a reálný playback test.

## 11. Ověřené technologické reference

- [Compose for TV release notes](https://developer.android.com/jetpack/androidx/releases/tv): tv-material 1.1.0 a tv-foundation 1.0.0 jsou uvedeny jako stable.
- [Compose BOM a kompatibilita](https://developer.android.com/develop/ui/compose/setup-compose-dependencies-and-compiler): Compose 1.12.0+ vyžaduje compileSdk 37 a AGP 9.0+; používáme BOM 2026.09.00.
- [Android Gradle Plugin 9.2.0](https://developer.android.com/build/releases/agp-9-2-0-release-notes): podporuje API 37, vyžaduje Gradle 9.4.1 a JDK 17.
- [Android 17 SDK setup](https://developer.android.com/about/versions/17/setup-sdk): API 37 je v Android Studio uvedeno jako Android Cinnamon Bun Preview; SDK manager musí instalovat preview platformu.
- [AGP built-in Kotlin migration](https://developer.android.com/build/migrate-to-built-in-kotlin): AGP 9+ nahrazuje plugin org.jetbrains.kotlin.android vestavěnou podporou Kotlin.
- [AndroidX Media3 releases](https://developer.android.com/jetpack/androidx/releases/media3): Media3 1.11.1 bylo publikováno 10. září 2026.
- [Apple AVPlayer](https://developer.apple.com/documentation/avfoundation/avplayer): systémový přehrávač pro lokální/vzdálená média včetně HLS.
- [Shaka Player](https://github.com/shaka-project/shaka-player): browserový adaptivní HLS/DASH přehrávač přes MSE/EME.
- [Google Cloud Maven Central mirror](https://docs.cloud.google.com/docs/buildpacks/java): dostupná mirror URL; samotná mirror služba uvádí, že nejde o oficiálně podporovaný produkt Google, proto zůstává fallbackem a Maven Central zůstává v konfiguraci jako další zdroj.