package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFetchProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "#EXTM3U\n#EXTINF:-1,Sample\nhttps://example.invalid/live.m3u8\n")
	}))
	defer upstream.Close()

	app := &appServer{token: "secret"}
	q := url.Values{}
	q.Set("token", "secret")
	q.Set("url", upstream.URL+"/playlist.m3u")
	req := httptest.NewRequest(http.MethodGet, "/api/fetch?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	app.handleFetch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "#EXTM3U") {
		t.Fatalf("unexpected proxy body: %s", rec.Body.String())
	}
}

func TestLocalStateRoundTripAndDelete(t *testing.T) {
	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{token: "secret", statePath: filepath.Join(t.TempDir(), "state.dpapi"), protectData: identity, unprotectData: identity}
	post := httptest.NewRequest(http.MethodPost, "/api/state?token=secret", strings.NewReader(`{"channels":[{"id":"one"}],"savedAt":123}`))
	postRec := httptest.NewRecorder()
	app.handleState(postRec, post)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST status %d: %s", postRec.Code, postRec.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/state?token=secret", nil)
	getRec := httptest.NewRecorder()
	app.handleState(getRec, get)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), `"savedAt":123`) {
		t.Fatalf("GET did not return saved state: %d %s", getRec.Code, getRec.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/state?token=secret", nil)
	delRec := httptest.NewRecorder()
	app.handleState(delRec, del)
	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE status %d: %s", delRec.Code, delRec.Body.String())
	}
	if _, err := os.Stat(app.statePath); !os.IsNotExist(err) {
		t.Fatalf("state file still exists: %v", err)
	}
}

func TestFetchRejectsNonHTTPProtocols(t *testing.T) {
	app := &appServer{token: "secret"}
	q := url.Values{}
	q.Set("token", "secret")
	q.Set("url", "file:///etc/passwd")
	req := httptest.NewRequest(http.MethodGet, "/api/fetch?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	app.handleFetch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for file URL, got %d", rec.Code)
	}
}

func TestValidateExternalLink(t *testing.T) {
	valid := []string{
		"https://sktorrent.org/torrents.php?id=1",
		"http://example.invalid/path",
		"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
		"magnet:?xt=urn:btmh:1220abcdef",
	}
	for _, value := range valid {
		if _, err := validateExternalLink(value); err != nil {
			t.Errorf("expected %q to be allowed: %v", value, err)
		}
	}
	invalid := []string{"file:///etc/passwd", "javascript:alert(1)", "magnet:?dn=missing-hash", "https:///missing-host"}
	for _, value := range invalid {
		if _, err := validateExternalLink(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestOpenLinkRequiresSessionAndPost(t *testing.T) {
	app := &appServer{token: "secret"}
	req := httptest.NewRequest(http.MethodPost, "/api/open-link?token=wrong", strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	app.handleOpenLink(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rec.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/open-link?token=secret", nil)
	getRec := httptest.NewRecorder()
	app.handleOpenLink(getRec, get)
	if getRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for GET, got %d", getRec.Code)
	}
}

func TestDownloadTorrentToCache(t *testing.T) {
	payload := []byte("d4:infod4:name4:testee")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-bittorrent")
		w.Write(payload)
	}))
	defer server.Close()

	cache := t.TempDir()
	path, err := downloadTorrentToCache(server.URL+"/demo.torrent", cache)
	if err != nil {
		t.Fatalf("downloadTorrentToCache returned error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read cached torrent: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("cached data mismatch: %q", got)
	}
	if filepath.Ext(path) != ".torrent" {
		t.Fatalf("expected .torrent suffix, got %q", path)
	}
}

func TestDownloadTorrentRejectsHTMLAndOversizedFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{name: "html login page", body: []byte("<html>Login required</html>")},
		{name: "not bencoded", body: []byte("This is not a torrent")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(tc.body) }))
			defer server.Close()
			if _, err := downloadTorrentToCache(server.URL, t.TempDir()); err == nil {
				t.Fatal("expected invalid torrent to be rejected")
			}
		})
	}
}

func TestDownloadTorrentRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-bittorrent")
		w.Header().Set("Content-Length", fmt.Sprint(maxTorrentBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if _, err := downloadTorrentToCache(server.URL, t.TempDir()); err == nil {
		t.Fatal("expected oversized torrent to be rejected")
	}
}

func TestOpenTorrentRequiresLocalSession(t *testing.T) {
	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{token: "secret", statePath: filepath.Join(t.TempDir(), "state.dpapi"), protectData: identity, unprotectData: identity}
	req := httptest.NewRequest(http.MethodPost, "/api/open-torrent?token=wrong", strings.NewReader(`{"url":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}`))
	rec := httptest.NewRecorder()
	app.handleOpenTorrent(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rec.Code)
	}
}

func TestEmbeddedSktorrentDemoFeedIsServedLocally(t *testing.T) {
	app := &appServer{}
	req := httptest.NewRequest(http.MethodGet, "/sample-sktorrent.rss", nil)
	rec := httptest.NewRecorder()
	app.handleRoot(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("unexpected content type: %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "WAVE IPTV - Offline demo catalogue") {
		t.Fatal("demo feed was not served from the embedded application assets")
	}
}

func TestValidateQBTBaseRequiresLocalHost(t *testing.T) {
	for _, raw := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if _, err := validateQBTBase(raw); err != nil {
			t.Errorf("expected local URL %q to be accepted: %v", raw, err)
		}
	}
	for _, raw := range []string{"https://example.com:8080", "http://192.168.1.5:8080", "file:///tmp/qbt", "http://user:pass@localhost:8080"} {
		if _, err := validateQBTBase(raw); err == nil {
			t.Errorf("expected non-local URL %q to be rejected", raw)
		}
	}
}

func TestQBTConnectListAndAddMagnet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Errorf("unexpected login request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("username") != "wave" || r.Form.Get("password") != "secret" {
			t.Errorf("unexpected login credentials")
		}
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "wave-session", Path: "/"})
		io.WriteString(w, "Ok.")
	})
	mux.HandleFunc("/api/v2/app/version", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("SID"); err != nil || c.Value != "wave-session" {
			t.Errorf("qBittorrent session cookie missing")
		}
		io.WriteString(w, "v5.0.0")
	})
	mux.HandleFunc("/api/v2/torrents/info", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter") != "all" {
			t.Errorf("expected filter=all query, got %q", r.URL.RawQuery)
		}
		if c, err := r.Cookie("SID"); err != nil || c.Value != "wave-session" {
			t.Errorf("torrent list request missing session cookie")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"hash":"0123456789abcdef0123456789abcdef01234567","name":"Legal Demo","progress":0.5,"dlspeed":1024,"size":2048,"state":"downloading"}]`)
	})
	mux.HandleFunc("/api/v2/transfer/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dl_info_speed":1024,"up_info_speed":0}`)
	})
	mux.HandleFunc("/api/v2/torrents/add", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil || !strings.HasPrefix(r.FormValue("urls"), "magnet:") {
			t.Errorf("magnet wasn't passed to qBittorrent: %v", err)
		}
		io.WriteString(w, "Ok.")
	})
	mux.HandleFunc("/api/v2/torrents/stop", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "Ok.") })
	server := httptest.NewServer(mux)
	defer server.Close()

	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{token: "secret", statePath: filepath.Join(t.TempDir(), "state.dpapi"), protectData: identity, unprotectData: identity}
	connectReq := httptest.NewRequest(http.MethodPost, "/api/qbt/connect?token=secret", strings.NewReader(`{"baseUrl":"`+server.URL+`","username":"wave","password":"secret"}`))
	connectRec := httptest.NewRecorder()
	app.handleQBTConnect(connectRec, connectReq)
	if connectRec.Code != http.StatusOK || !strings.Contains(connectRec.Body.String(), "v5.0.0") {
		t.Fatalf("qBittorrent connection failed: %d %s", connectRec.Code, connectRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/qbt/status?token=secret", nil)
	statusRec := httptest.NewRecorder()
	app.handleQBTStatus(statusRec, statusReq)
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), "dl_info_speed") {
		t.Fatalf("qBittorrent status failed: %d %s", statusRec.Code, statusRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/qbt/torrents?token=secret", nil)
	listRec := httptest.NewRecorder()
	app.handleQBTTorrents(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "Legal Demo") {
		t.Fatalf("qBittorrent list failed: %d %s", listRec.Code, listRec.Body.String())
	}

	addReq := httptest.NewRequest(http.MethodPost, "/api/qbt/add?token=secret", strings.NewReader(`{"url":"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"}`))
	addRec := httptest.NewRecorder()
	app.handleQBTAdd(addRec, addReq)
	if addRec.Code != http.StatusOK {
		t.Fatalf("qBittorrent add failed: %d %s", addRec.Code, addRec.Body.String())
	}

	actionReq := httptest.NewRequest(http.MethodPost, "/api/qbt/action?token=secret", strings.NewReader(`{"hash":"0123456789abcdef0123456789abcdef01234567","action":"pause"}`))
	actionRec := httptest.NewRecorder()
	app.handleQBTAction(actionRec, actionReq)
	if actionRec.Code != http.StatusOK {
		t.Fatalf("qBittorrent pause failed: %d %s", actionRec.Code, actionRec.Body.String())
	}
}

func TestCompletedVideoPathSelectsLargestCompleteVideoAndRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	large := filepath.Join(root, "movie.mkv")
	if err := os.WriteFile(large, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	files := []qbtFileInfo{
		{Name: "movie.mkv", Size: 500, Progress: 1},
		{Name: "sample.mp4", Size: 100, Progress: 1},
		{Name: "incomplete.mkv", Size: 900, Progress: 0.4},
		{Name: "../outside.mkv", Size: 1000, Progress: 1},
		{Name: "readme.txt", Size: 2000, Progress: 1},
	}
	got, err := completedVideoPath("", root, files)
	if err != nil {
		t.Fatalf("completedVideoPath failed: %v", err)
	}
	if got != large {
		t.Fatalf("expected largest completed video %q, got %q", large, got)
	}
}

func TestCompletedVideoPathRequiresExistingFile(t *testing.T) {
	_, err := completedVideoPath("", t.TempDir(), []qbtFileInfo{{Name: "missing.mkv", Size: 10, Progress: 1}})
	if err == nil {
		t.Fatal("expected missing local file to be rejected")
	}
}

func TestLocalMediaRegisterAndRangePlayback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.mp4")
	payload := []byte("WAVE_MEDIA_RANGE_TEST")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	app := &appServer{token: "secret", mediaFiles: make(map[string]string)}
	id, err := app.registerMediaFile(path)
	if err != nil {
		t.Fatalf("registerMediaFile: %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("unexpected media id length: %d", len(id))
	}

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/media/"+id+"?token=wrong", nil)
	unauthorizedRec := httptest.NewRecorder()
	app.handleMediaServe(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized media request, got %d", unauthorizedRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media/"+id+"?token=secret", nil)
	req.Header.Set("Range", "bytes=5-9")
	rec := httptest.NewRecorder()
	app.handleMediaServe(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 for byte range, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != string(payload[5:10]) {
		t.Fatalf("range payload mismatch: got %q want %q", got, string(payload[5:10]))
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("expected byte range support, headers: %#v", rec.Header())
	}
}

func TestRegisterMediaRejectsUnsupportedAndMissingFiles(t *testing.T) {
	app := &appServer{mediaFiles: make(map[string]string)}
	textPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(textPath, []byte("not video"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.registerMediaFile(textPath); err == nil {
		t.Fatal("expected unsupported extension to be rejected")
	}
	if _, err := app.registerMediaFile(filepath.Join(t.TempDir(), "missing.mp4")); err == nil {
		t.Fatal("expected missing media file to be rejected")
	}
}

func TestStateAndSecretsAreSeparated(t *testing.T) {
	encode := func(b []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
		base64.StdEncoding.Encode(out, b)
		return out, nil
	}
	decode := func(b []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
		n, err := base64.StdEncoding.Decode(out, b)
		return out[:n], err
	}
	dir := t.TempDir()
	app := &appServer{
		token:           "secret",
		statePath:       filepath.Join(dir, "state.dpapi"),
		credentialsPath: filepath.Join(dir, "credentials.dpapi"),
		protectData:     encode,
		unprotectData:   decode,
	}
	payload := `{"channels":[{"id":"test","url":"https://provider.invalid/live/user/password/1.m3u8"}],"xtreamPass":"password-secret","sktorrentFeedUrl":"https://sktorrent.invalid/rss?token=private-token","savedAt":456}`
	post := httptest.NewRequest(http.MethodPost, "/api/state?token=secret", strings.NewReader(payload))
	postRec := httptest.NewRecorder()
	app.handleState(postRec, post)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST status %d: %s", postRec.Code, postRec.Body.String())
	}
	stateBytes, err := os.ReadFile(app.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateBytes), "password-secret") || strings.Contains(string(stateBytes), "private-token") {
		t.Fatal("secrets leaked into the state file")
	}
	getState := httptest.NewRequest(http.MethodGet, "/api/state?token=secret", nil)
	getStateRec := httptest.NewRecorder()
	app.handleState(getStateRec, getState)
	if strings.Contains(getStateRec.Body.String(), "password-secret") || strings.Contains(getStateRec.Body.String(), "private-token") {
		t.Fatal("state API returned secrets")
	}
	getCreds := httptest.NewRequest(http.MethodGet, "/api/credentials?token=secret", nil)
	getCredsRec := httptest.NewRecorder()
	app.handleCredentials(getCredsRec, getCreds)
	if getCredsRec.Code != http.StatusOK || !strings.Contains(getCredsRec.Body.String(), "password-secret") || !strings.Contains(getCredsRec.Body.String(), "private-token") {
		t.Fatalf("credential API did not round-trip secrets: %d %s", getCredsRec.Code, getCredsRec.Body.String())
	}
	ciphertext, err := os.ReadFile(app.credentialsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ciphertext), "password-secret") || strings.Contains(string(ciphertext), "private-token") {
		t.Fatal("secrets leaked into the credentials file")
	}
}

func TestLegacyPlaintextStateMigratesAndIsRemoved(t *testing.T) {
	encode := func(b []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
		base64.StdEncoding.Encode(out, b)
		return out, nil
	}
	decode := func(b []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
		n, err := base64.StdEncoding.Decode(out, b)
		return out[:n], err
	}
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(legacyPath, []byte(`{"channels":[{"id":"legacy"}],"xtreamPass":"legacy-pass","sktorrentFeedUrl":"legacy-feed","savedAt":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		statePath:       filepath.Join(dir, "state.dpapi"),
		legacyStatePath: legacyPath,
		credentialsPath: filepath.Join(dir, "credentials.dpapi"),
		protectData:     encode,
		unprotectData:   decode,
	}
	if err := app.migrateLegacyState(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext state was not removed: %v", err)
	}
	state, err := app.readState()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(state), "legacy-pass") || strings.Contains(string(state), "legacy-feed") {
		t.Fatal("migrated state still contains secrets")
	}
	creds, err := app.readCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds.XtreamPass != "legacy-pass" || creds.SKTorrentFeedURL != "legacy-feed" {
		t.Fatalf("legacy credentials did not migrate: %+v", creds)
	}
}

func TestMigrationPreservesLegacyFileWhenDPAPIUnavailable(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(legacyPath, []byte(`{"channels":[{"id":"legacy"}],"savedAt":10}`), 0600); err != nil {
		t.Fatal(err)
	}
	app := &appServer{
		statePath:       filepath.Join(dir, "state.dpapi"),
		legacyStatePath: legacyPath,
		credentialsPath: filepath.Join(dir, "credentials.dpapi"),
		protectData: func([]byte) ([]byte, error) {
			return nil, fmt.Errorf("DPAPI unavailable")
		},
		unprotectData: func(b []byte) ([]byte, error) { return b, nil },
	}
	if err := app.migrateLegacyState(); err == nil {
		t.Fatal("expected migration to fail without a protector")
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy state must be preserved on migration failure: %v", err)
	}
}

func TestStateRejectsNullJSON(t *testing.T) {
	app := &appServer{token: "secret", statePath: filepath.Join(t.TempDir(), "state.dpapi"), protectData: func(b []byte) ([]byte, error) { return b, nil }}
	req := httptest.NewRequest(http.MethodPost, "/api/state?token=secret", strings.NewReader("null"))
	rec := httptest.NewRecorder()
	app.handleState(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for null state, got %d: %s", rec.Code, rec.Body.String())
	}
}


func TestCSFDServiceURLRequiresLoopback(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:3000",
		"http://localhost:3000",
		"https://[::1]:3000",
	} {
		if _, err := validateCSFDServiceURL(raw); err != nil {
			t.Errorf("expected local CSFD URL %q to be accepted: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"https://csfd.cz",
		"http://192.168.1.10:3000",
		"http://127.0.0.1:3000/private",
		"http://user:pass@127.0.0.1:3000",
		"file:///tmp/csfd",
	} {
		if _, err := validateCSFDServiceURL(raw); err == nil {
			t.Errorf("expected unsafe CSFD URL %q to be rejected", raw)
		}
	}
}

func TestCSFDSearchUsesLocalAPIAndProtectedCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/Matrix" {
			t.Errorf("unexpected CSFD endpoint: %s", r.URL.String())
		}
		if r.URL.Query().Get("language") != "cs" {
			t.Errorf("expected Czech language, got %q", r.URL.Query().Get("language"))
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected bearer API key, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("expected X-API-Key header, got %q", r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"movies":[{"id":535121,"title":"Na špatné straně","year":2018}],"tvSeries":[]}`)
	}))
	defer server.Close()

	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{
		token:           "secret",
		credentialsPath: filepath.Join(t.TempDir(), "credentials.dpapi"),
		protectData:     identity,
		unprotectData:   identity,
	}
	if err := app.writeCredentials(privateCredentials{CSFDBaseURL: server.URL, CSFDAPIKey: "test-api-key"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/csfd/search?q=Matrix&token=secret", nil)
	rec := httptest.NewRecorder()
	app.handleCSFDSearch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Na špatné straně") {
		t.Fatalf("unexpected CSFD response: %s", rec.Body.String())
	}
}

func TestCSFDSearchRejectsUnauthorizedAndShortQueries(t *testing.T) {
	app := &appServer{token: "secret"}
	unauthorized := httptest.NewRequest(http.MethodGet, "/api/csfd/search?q=Matrix&token=wrong", nil)
	unauthorizedRec := httptest.NewRecorder()
	app.handleCSFDSearch(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", unauthorizedRec.Code)
	}

	short := httptest.NewRequest(http.MethodGet, "/api/csfd/search?q=x&token=secret", nil)
	shortRec := httptest.NewRecorder()
	app.handleCSFDSearch(shortRec, short)
	if shortRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short query, got %d: %s", shortRec.Code, shortRec.Body.String())
	}
}

func TestCSFDMovieRejectsInvalidID(t *testing.T) {
	app := &appServer{token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/api/csfd/movie/../../etc/passwd?token=secret", nil)
	rec := httptest.NewRecorder()
	app.handleCSFDMovie(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid ČSFD ID, got %d: %s", rec.Code, rec.Body.String())
	}
}


func TestCSFDStatusUsesLocalServiceRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("expected service root, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"node-csfd-api","version":"5.16.1","links":["/search/:query","/movie/:id"]}`)
	}))
	defer server.Close()

	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{
		token:           "secret",
		credentialsPath: filepath.Join(t.TempDir(), "credentials.dpapi"),
		protectData:     identity,
		unprotectData:   identity,
	}
	if err := app.writeCredentials(privateCredentials{CSFDBaseURL: server.URL}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/csfd/status?token=secret", nil)
	rec := httptest.NewRecorder()
	app.handleCSFDStatus(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "node-csfd-api") {
		t.Fatalf("unexpected status response %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSFDMovieUsesExpectedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/535121" || r.URL.Query().Get("language") != "cs" {
			t.Errorf("unexpected movie endpoint: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":535121,"title":"Na špatné straně","year":"2018","rating":73,"ratingCount":6654,"poster":"https://image.pmgstatic.com/poster.jpg","genres":["Krimi","Drama"],"descriptions":["Popis filmu"],"creators":{"directors":[{"name":"S. Craig Zahler"}],"actors":[{"name":"Mel Gibson"}]}}`)
	}))
	defer server.Close()

	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{
		token:           "secret",
		credentialsPath: filepath.Join(t.TempDir(), "credentials.dpapi"),
		protectData:     identity,
		unprotectData:   identity,
	}
	if err := app.writeCredentials(privateCredentials{CSFDBaseURL: server.URL}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/csfd/movie/535121?token=secret", nil)
	rec := httptest.NewRecorder()
	app.handleCSFDMovie(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ratingCount") {
		t.Fatalf("unexpected detail response %d: %s", rec.Code, rec.Body.String())
	}
}


func TestCSFDRedirectCannotLeaveLocalService(t *testing.T) {
	destinationHit := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationHit = true
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"unexpected":true}`)
	}))
	defer destination.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"/", http.StatusFound)
	}))
	defer redirector.Close()

	identity := func(b []byte) ([]byte, error) { return append([]byte(nil), b...), nil }
	app := &appServer{
		token:           "secret",
		credentialsPath: filepath.Join(t.TempDir(), "credentials.dpapi"),
		protectData:     identity,
		unprotectData:   identity,
	}
	if err := app.writeCredentials(privateCredentials{CSFDBaseURL: redirector.URL}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/csfd/status?token=secret", nil)
	rec := httptest.NewRecorder()
	app.handleCSFDStatus(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected redirect to be rejected as 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if destinationHit {
		t.Fatal("ČSFD adapter followed a redirect to a different local service")
	}
}
