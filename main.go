package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed web/*
var assets embed.FS

const (
	appName         = "WAVE IPTV"
	defaultPort     = 38477
	maxFetchBytes   = 300 << 20 // 300 MiB
	maxStateBytes   = 120 << 20 // 120 MiB
	maxTorrentBytes = 10 << 20  // 10 MiB
	maxCSFDBytes    = 4 << 20   // 4 MiB
)

type appServer struct {
	token           string
	statePath       string
	legacyStatePath string
	credentialsPath string
	protectData     func([]byte) ([]byte, error)
	unprotectData   func([]byte) ([]byte, error)
	server          *http.Server
	lastSeen        atomic.Int64
	shutting        atomic.Bool
	logPath         string
	qbtMu           sync.Mutex
	qbtBase         string
	qbtClient       *http.Client
	qbtVersion      string
	mediaMu         sync.RWMutex
	mediaFiles      map[string]string
	csfdMu           sync.Mutex
	csfdNextRequest  time.Time
}

func main() {
	if err := run(); err != nil {
		writeLog("startup error: " + err.Error())
		if runtime.GOOS != "windows" {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func run() error {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		configDir = os.TempDir()
	}
	appDir := filepath.Join(configDir, appName)
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return fmt.Errorf("cannot create local data folder: %w", err)
	}
	app := &appServer{statePath: filepath.Join(appDir, "state.dpapi"), legacyStatePath: filepath.Join(appDir, "state.json"), credentialsPath: filepath.Join(appDir, "credentials.dpapi"), logPath: filepath.Join(appDir, "app.log"), mediaFiles: make(map[string]string)}
	if err := app.migrateLegacyState(); err != nil {
		app.log("legacy state migration deferred: " + err.Error())
	}
	if err := app.newToken(); err != nil {
		return err
	}
	app.lastSeen.Store(time.Now().Unix())

	listener, port, err := listenLocal()
	if err != nil {
		return err
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleRoot)
	mux.HandleFunc("/api/fetch", app.handleFetch)
	mux.HandleFunc("/api/state", app.handleState)
	mux.HandleFunc("/api/credentials", app.handleCredentials)
	mux.HandleFunc("/api/csfd/status", app.handleCSFDStatus)
	mux.HandleFunc("/api/csfd/search", app.handleCSFDSearch)
	mux.HandleFunc("/api/csfd/movie/", app.handleCSFDMovie)
	mux.HandleFunc("/api/open-player", app.handleOpenPlayer)
	mux.HandleFunc("/api/open-link", app.handleOpenLink)
	mux.HandleFunc("/api/open-torrent", app.handleOpenTorrent)
	mux.HandleFunc("/api/qbt/connect", app.handleQBTConnect)
	mux.HandleFunc("/api/qbt/disconnect", app.handleQBTDisconnect)
	mux.HandleFunc("/api/qbt/status", app.handleQBTStatus)
	mux.HandleFunc("/api/qbt/torrents", app.handleQBTTorrents)
	mux.HandleFunc("/api/qbt/add", app.handleQBTAdd)
	mux.HandleFunc("/api/qbt/action", app.handleQBTAction)
	mux.HandleFunc("/api/qbt/play", app.handleQBTPlay)
	mux.HandleFunc("/api/pick-local", app.handlePickLocal)
	mux.HandleFunc("/api/media/register", app.handleMediaRegister)
	mux.HandleFunc("/api/media/", app.handleMediaServe)
	mux.HandleFunc("/api/ping", app.handlePing)
	mux.HandleFunc("/api/quit", app.handleQuit)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "wave-iptv-local")
	})

	app.server = &http.Server{
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		if err := app.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.log("server error: " + err.Error())
		}
	}()
	go app.watchdog()

	app.log("started at " + baseURL)
	if os.Getenv("WAVE_NO_BROWSER") != "1" {
		if err := openAppWindow(baseURL); err != nil {
			app.log("browser launch failed: " + err.Error())
			return fmt.Errorf("server started at %s, but the browser could not be opened: %w", baseURL, err)
		}
	}
	// Keep the lightweight local host alive until the app window is closed or
	// the user selects Exit in Settings.
	select {}
}

func (a *appServer) newToken() error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("cannot generate local session token: %w", err)
	}
	a.token = hex.EncodeToString(b)
	return nil
}

func listenLocal() (net.Listener, int, error) {
	for port := defaultPort; port < defaultPort+12; port++ {
		ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, port, nil
		}
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("cannot start local web host: %w", err)
	}
	_, p, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	fmt.Sscan(p, &port)
	return ln, port, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The server listens only on loopback. Also reject unexpected Host headers.
		host, _, err := net.SplitHostPort(r.Host)
		if err == nil && host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self' data: blob: http: https:; img-src 'self' data: blob: http: https:; media-src * blob:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'self' http: https:; font-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func (a *appServer) authorized(r *http.Request) bool {
	return r.URL.Query().Get("token") == a.token
}

func (a *appServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		f, err := assets.Open("web/" + strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		if strings.HasSuffix(r.URL.Path, ".m3u") || strings.HasSuffix(r.URL.Path, ".m3u8") {
			w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
		} else if strings.HasSuffix(r.URL.Path, ".xml") || strings.HasSuffix(r.URL.Path, ".rss") {
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		}
		io.Copy(w, f)
		return
	}
	content, err := assets.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "application files are missing", http.StatusInternalServerError)
		return
	}
	html := strings.ReplaceAll(string(content), "__WAVE_TOKEN__", a.token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	io.WriteString(w, html)
}

func (a *appServer) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	target, err := url.Parse(r.URL.Query().Get("url"))
	if err != nil || target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		http.Error(w, "URL must use HTTP or HTTPS", http.StatusBadRequest)
		return
	}
	requestCtx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	request.Header.Set("User-Agent", "WAVE-IPTV/1.0")
	request.Header.Set("Accept", "*/*")
	client := &http.Client{
		Timeout: 3 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect protocol not allowed")
			}
			return nil
		},
	}
	resp, err := client.Do(request)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxFetchBytes {
		http.Error(w, "upstream file exceeds 300 MiB limit", http.StatusRequestEntityTooLarge)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		http.Error(w, fmt.Sprintf("Upstream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))), resp.StatusCode)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	limited := io.LimitReader(resp.Body, maxFetchBytes+1)
	n, copyErr := io.Copy(w, limited)
	if copyErr != nil {
		a.log("proxy response write error: " + copyErr.Error())
	}
	if n > maxFetchBytes {
		a.log("upstream response exceeded maximum size")
	}
}

func (a *appServer) handleState(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		b, err := a.readState()
		if errors.Is(err, os.ErrNotExist) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			io.WriteString(w, "{}")
			return
		}
		if err != nil {
			writeJSONError(w, "Lokální databázi WAVE nelze dešifrovat. Spusťte aplikaci pod stejným účtem Windows, pod kterým byla vytvořena.", http.StatusServiceUnavailable)
			return
		}
		// Older releases wrote secrets into the plaintext state snapshot. Never return those values
		// to the UI once the new secure credential store is available.
		var state map[string]any
		if json.Unmarshal(b, &state) == nil && state != nil {
			delete(state, "xtreamPass")
			delete(state, "sktorrentFeedUrl")
			state["waveSchemaVersion"] = 2
			b, _ = json.Marshal(state)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(b)
	case http.MethodDelete:
		if err := os.Remove(a.statePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			http.Error(w, "cannot delete local state", http.StatusInternalServerError)
			return
		}
		if a.legacyStatePath != "" {
			if err := os.Remove(a.legacyStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				http.Error(w, "cannot delete legacy local state", http.StatusInternalServerError)
				return
			}
		}
		if a.credentialsPath != "" {
			if err := os.Remove(a.credentialsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				http.Error(w, "cannot delete local credentials", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"deleted":true}`)
	case http.MethodPost:
		defer r.Body.Close()
		b, err := io.ReadAll(io.LimitReader(r.Body, maxStateBytes+1))
		if err != nil || len(b) > maxStateBytes {
			http.Error(w, "state file is too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		var value map[string]any
		if err := json.Unmarshal(b, &value); err != nil || value == nil {
			http.Error(w, "invalid state JSON", http.StatusBadRequest)
			return
		}
		// Compatibility migration for older clients that still include secrets
		// in the state payload. New clients use /api/credentials directly.
		creds, credsErr := a.readCredentials()
		if credsErr != nil {
			writeJSONError(w, "Uložené přihlašovací údaje nelze dešifrovat; stávající hodnoty nebyly změněny.", http.StatusServiceUnavailable)
			return
		}
		secretChanged := false
		if v, ok := value["xtreamPass"]; ok {
			if text, ok := v.(string); ok {
				creds.XtreamPass = text
				secretChanged = true
			}
			delete(value, "xtreamPass")
		}
		if v, ok := value["sktorrentFeedUrl"]; ok {
			if text, ok := v.(string); ok {
				creds.SKTorrentFeedURL = text
				secretChanged = true
			}
			delete(value, "sktorrentFeedUrl")
		}
		value["waveSchemaVersion"] = 2
		if secretChanged {
			if err := a.writeCredentials(creds); err != nil {
				writeJSONError(w, "Nelze bezpečně uložit přihlašovací údaje. Na Windows je vyžadováno šifrování DPAPI.", http.StatusNotImplemented)
				return
			}
		}
		clean, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "cannot encode local state", http.StatusInternalServerError)
			return
		}
		if err := a.writeState(clean); err != nil {
			writeJSONError(w, "Nelze bezpečně uložit lokální knihovnu. Ve Windows je vyžadováno šifrování DPAPI.", http.StatusNotImplemented)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"saved":true,"schemaVersion":2}`)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type privateCredentials struct {
	XtreamPass       string `json:"xtreamPass"`
	SKTorrentFeedURL string `json:"sktorrentFeedUrl"`
	CSFDBaseURL      string `json:"csfdBaseUrl"`
	CSFDAPIKey       string `json:"csfdApiKey"`
}


var csfdNumericID = regexp.MustCompile("^[0-9]{1,12}$")

func validateCSFDServiceURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, errors.New("ČSFD API adresa musí být platná HTTP(S) URL")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil, errors.New("ČSFD API musí běžet lokálně na tomto zařízení")
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("ČSFD API adresa nesmí obsahovat přihlašovací údaje, cestu ani parametry")
	}
	u.Path = ""
	u.RawPath = ""
	return u, nil
}

func (a *appServer) waitForCSFDSlot(ctx context.Context) error {
	a.csfdMu.Lock()
	now := time.Now()
	slot := now
	if a.csfdNextRequest.After(slot) {
		slot = a.csfdNextRequest
	}
	a.csfdNextRequest = slot.Add(1200 * time.Millisecond)
	wait := time.Until(slot)
	a.csfdMu.Unlock()
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *appServer) fetchCSFDJSON(r *http.Request, creds privateCredentials, endpoint string) ([]byte, int, error) {
	base, err := validateCSFDServiceURL(creds.CSFDBaseURL)
	if err != nil {
		return nil, http.StatusServiceUnavailable, errors.New("ČSFD konektor není nastaven. Spusťte lokální node-csfd-api a uložte jeho adresu v Nastavení.")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	if err := a.waitForCSFDSlot(ctx); err != nil {
		return nil, http.StatusGatewayTimeout, errors.New("Požadavek na ČSFD byl zrušen nebo vypršel časový limit.")
	}

	target, err := url.Parse(strings.TrimRight(base.String(), "/") + endpoint)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("Neplatný požadavek na ČSFD.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("Neplatný požadavek na ČSFD.")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Sourbase-Media-Center/1.0")
	if creds.CSFDAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+creds.CSFDAPIKey)
		req.Header.Set("X-API-Key", creds.CSFDAPIKey)
	}
	client := &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= 3 || next.URL.Scheme != base.Scheme || !strings.EqualFold(next.URL.Host, base.Host) {
				return errors.New("ČSFD API přesměrovalo mimo lokální službu")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, errors.New("Lokální ČSFD API není dostupné. Zkontrolujte, že běží na uložené adrese.")
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxCSFDBytes+1))
	if readErr != nil {
		return nil, http.StatusBadGateway, errors.New("Odpověď ČSFD API se nepodařilo přečíst.")
	}
	if len(body) > maxCSFDBytes {
		return nil, http.StatusBadGateway, errors.New("Odpověď ČSFD API je příliš velká.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, http.StatusUnauthorized, errors.New("ČSFD API odmítlo API klíč. Zkontrolujte jej v Nastavení.")
		case http.StatusTooManyRequests:
			return nil, http.StatusTooManyRequests, errors.New("ČSFD nebo jeho API omezilo počet požadavků. Zkuste to později.")
		case http.StatusNotFound:
			return nil, http.StatusNotFound, errors.New("Na ČSFD nebyl nalezen odpovídající záznam.")
		case http.StatusServiceUnavailable:
			return nil, http.StatusServiceUnavailable, errors.New("ČSFD dočasně odmítlo požadavek.")
		default:
			return nil, http.StatusBadGateway, fmt.Errorf("Lokální ČSFD API vrátilo HTTP %d.", resp.StatusCode)
		}
	}
	if !json.Valid(body) {
		return nil, http.StatusBadGateway, errors.New("Lokální ČSFD API nevrátilo platný JSON.")
	}
	return body, http.StatusOK, nil
}

func (a *appServer) csfdCredentials(w http.ResponseWriter, r *http.Request) (privateCredentials, bool) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return privateCredentials{}, false
	}
	creds, err := a.readCredentials()
	if err != nil {
		writeJSONError(w, "Uložená konfigurace ČSFD není dostupná.", http.StatusServiceUnavailable)
		return privateCredentials{}, false
	}
	if _, err := validateCSFDServiceURL(creds.CSFDBaseURL); err != nil {
		writeJSONError(w, "ČSFD konektor není nastaven. Spusťte lokální node-csfd-api.", http.StatusServiceUnavailable)
		return privateCredentials{}, false
	}
	return creds, true
}

func (a *appServer) handleCSFDStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	creds, ok := a.csfdCredentials(w, r)
	if !ok {
		return
	}
	body, status, err := a.fetchCSFDJSON(r, creds, "/")
	if err != nil {
		writeJSONError(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func (a *appServer) handleCSFDSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) < 2 || len([]rune(query)) > 100 {
		writeJSONError(w, "Zadejte název dlouhý 2 až 100 znaků.", http.StatusBadRequest)
		return
	}
	creds, ok := a.csfdCredentials(w, r)
	if !ok {
		return
	}
	body, status, err := a.fetchCSFDJSON(r, creds, "/search/"+url.PathEscape(query)+"?language=cs")
	if err != nil {
		writeJSONError(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func (a *appServer) handleCSFDMovie(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/csfd/movie/")
	if !csfdNumericID.MatchString(id) {
		writeJSONError(w, "Neplatné ČSFD ID.", http.StatusBadRequest)
		return
	}
	creds, ok := a.csfdCredentials(w, r)
	if !ok {
		return
	}
	body, status, err := a.fetchCSFDJSON(r, creds, "/movie/"+id+"?language=cs")
	if err != nil {
		writeJSONError(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}


func (a *appServer) handleCredentials(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		creds, err := a.readCredentials()
		if err != nil {
			writeJSONError(w, "Uložené přihlašovací údaje nelze dešifrovat. Zkontrolujte účet Windows, pod kterým byla aplikace spuštěna.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(creds)
	case http.MethodPost:
		defer r.Body.Close()
		var creds privateCredentials
		if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&creds); err != nil {
			writeJSONError(w, "Neplatné přihlašovací údaje.", http.StatusBadRequest)
			return
		}
		if err := a.writeCredentials(creds); err != nil {
			writeJSONError(w, "Bezpečné ukládání přihlašovacích údajů je dostupné pouze ve Windows s DPAPI.", http.StatusNotImplemented)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"saved":true,"encrypted":true}`)
	case http.MethodDelete:
		if a.credentialsPath != "" {
			if err := os.Remove(a.credentialsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				writeJSONError(w, "Nelze odstranit uložené přihlašovací údaje.", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"deleted":true}`)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *appServer) readCredentials() (privateCredentials, error) {
	if a.credentialsPath == "" {
		return privateCredentials{}, nil
	}
	ciphertext, err := os.ReadFile(a.credentialsPath)
	if errors.Is(err, os.ErrNotExist) {
		return privateCredentials{}, nil
	}
	if err != nil {
		return privateCredentials{}, err
	}
	plain, err := a.unprotect(ciphertext)
	if err != nil {
		return privateCredentials{}, err
	}
	defer wipeBytes(plain)
	var creds privateCredentials
	if err := json.Unmarshal(plain, &creds); err != nil {
		return privateCredentials{}, err
	}
	return creds, nil
}

func (a *appServer) writeCredentials(creds privateCredentials) error {
	if a.credentialsPath == "" {
		return errors.New("credential path is not configured")
	}
	plain, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	defer wipeBytes(plain)
	ciphertext, err := a.protect(plain)
	if err != nil {
		return err
	}
	defer wipeBytes(ciphertext)
	return atomicWriteFile(a.credentialsPath, ciphertext, 0600)
}

func (a *appServer) protect(plain []byte) ([]byte, error) {
	if a.protectData != nil {
		return a.protectData(plain)
	}
	return protectSecret(plain)
}

func (a *appServer) unprotect(ciphertext []byte) ([]byte, error) {
	if a.unprotectData != nil {
		return a.unprotectData(ciphertext)
	}
	return unprotectSecret(ciphertext)
}

func (a *appServer) readState() ([]byte, error) {
	ciphertext, err := os.ReadFile(a.statePath)
	if err != nil {
		return nil, err
	}
	plain, err := a.unprotect(ciphertext)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

func (a *appServer) writeState(plain []byte) error {
	ciphertext, err := a.protect(plain)
	if err != nil {
		return err
	}
	defer wipeBytes(ciphertext)
	return atomicWriteFile(a.statePath, ciphertext, 0600)
}

// migrateLegacyState moves v1.5's plaintext state.json to DPAPI-protected
// state.dpapi and moves credentials into their own DPAPI-protected store.
func (a *appServer) migrateLegacyState() error {
	if a.legacyStatePath == "" || a.statePath == "" {
		return nil
	}
	if _, err := os.Stat(a.statePath); err == nil {
		// Keep a legacy copy if the encrypted store cannot be opened. A readable
		// encrypted store supersedes it and allows safe removal of plaintext.
		if _, readErr := a.readState(); readErr != nil {
			return fmt.Errorf("existing encrypted state is unreadable; legacy state preserved: %w", readErr)
		}
		if _, legacyErr := os.Stat(a.legacyStatePath); legacyErr == nil {
			if err := os.Remove(a.legacyStatePath); err != nil {
				return err
			}
		}
		return nil
	}
	b, err := os.ReadFile(a.legacyStatePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var value map[string]any
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	pass, _ := value["xtreamPass"].(string)
	feed, _ := value["sktorrentFeedUrl"].(string)
	if pass != "" || feed != "" {
		creds, readErr := a.readCredentials()
		if readErr != nil {
			return fmt.Errorf("cannot read existing protected credentials: %w", readErr)
		}
		if pass != "" {
			creds.XtreamPass = pass
		}
		if feed != "" {
			creds.SKTorrentFeedURL = feed
		}
		if err := a.writeCredentials(creds); err != nil {
			return fmt.Errorf("cannot protect legacy credentials: %w", err)
		}
	}
	delete(value, "xtreamPass")
	delete(value, "sktorrentFeedUrl")
	value["waveSchemaVersion"] = 2
	clean, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := a.writeState(clean); err != nil {
		return fmt.Errorf("cannot encrypt legacy state: %w", err)
	}
	// Delete plaintext only after the encrypted state and secrets are safely
	// written. This is a migration, not a backup copy of unencrypted data.
	if err := os.Remove(a.legacyStatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil && runtime.GOOS != "windows" {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

type openPlayerRequest struct {
	URL string `json:"url"`
}

func (a *appServer) handleOpenPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var input openPlayerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	u, err := url.Parse(input.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSONError(w, "Only HTTP/HTTPS stream URLs are allowed", http.StatusBadRequest)
		return
	}
	player, args := findExternalPlayer(input.URL)
	if player == "" {
		writeJSONError(w, "VLC ani MPV nebyly nalezeny. Nainstalujte VLC nebo MPV a zkuste to znovu.", http.StatusNotFound)
		return
	}
	cmd := exec.Command(player, args...)
	if err := cmd.Start(); err != nil {
		writeJSONError(w, "Nepodařilo se spustit externí přehrávač: "+err.Error(), http.StatusInternalServerError)
		return
	}
	go cmd.Wait()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "player": filepath.Base(player)})
}

func findExternalPlayer(streamURL string) (string, []string) {
	candidates := []string{"vlc", "vlc.exe", "mpv", "mpv.exe"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			if strings.Contains(strings.ToLower(filepath.Base(path)), "mpv") {
				return path, []string{"--force-window=yes", streamURL}
			}
			return path, []string{"--no-video-title-show", streamURL}
		}
	}
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			root := os.Getenv(env)
			if root == "" {
				continue
			}
			path := filepath.Join(root, "VideoLAN", "VLC", "vlc.exe")
			if _, err := os.Stat(path); err == nil {
				return path, []string{"--no-video-title-show", streamURL}
			}
		}
	}
	return "", nil
}

type openLinkRequest struct {
	URL string `json:"url"`
}

// validateExternalLink deliberately supports only ordinary web links and
// BitTorrent magnet URIs. No shell command is assembled from user input.
func validateExternalLink(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("neplatný odkaz")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Hostname() == "" {
			return nil, fmt.Errorf("webový odkaz nemá hostitele")
		}
	case "magnet":
		xt := strings.ToLower(u.Query().Get("xt"))
		if !strings.HasPrefix(xt, "urn:btih:") && !strings.HasPrefix(xt, "urn:btmh:") {
			return nil, fmt.Errorf("magnet odkaz neobsahuje platný BitTorrent identifikátor")
		}
	default:
		return nil, fmt.Errorf("povolené jsou pouze HTTP, HTTPS a BitTorrent magnet odkazy")
	}
	return u, nil
}

func openWithSystemHandler(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("systémový prohlížeč nebo torrent klient se nepodařilo otevřít: %w", err)
	}
	go cmd.Wait()
	return nil
}

func (a *appServer) handleOpenLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input openLinkRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatný požadavek.", http.StatusBadRequest)
		return
	}
	u, err := validateExternalLink(input.URL)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := openWithSystemHandler(u.String()); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "scheme": u.Scheme})
}

// downloadTorrentToCache downloads a user-supplied direct .torrent URL and
// verifies that the response looks like a bencoded torrent before handing it
// to the operating system. Magnet links are handled without downloading.
func downloadTorrentToCache(rawURL, cacheDir string) (string, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Hostname() == "" {
		return "", fmt.Errorf("torrent URL musí používat HTTP nebo HTTPS")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", fmt.Errorf("neplatný torrent odkaz")
	}
	req.Header.Set("User-Agent", "WAVE-IPTV/1.6.0")
	req.Header.Set("Accept", "application/x-bittorrent, application/octet-stream, */*")
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("příliš mnoho přesměrování")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("nepovolený protokol přesměrování")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("torrent se nepodařilo stáhnout: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tracker vrátil HTTP %d; odkaz může vyžadovat přihlášení", resp.StatusCode)
	}
	if resp.ContentLength > maxTorrentBytes {
		return "", fmt.Errorf("torrent soubor překračuje limit 10 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTorrentBytes+1))
	if err != nil {
		return "", fmt.Errorf("torrent se nepodařilo načíst: %w", err)
	}
	if len(data) > maxTorrentBytes {
		return "", fmt.Errorf("torrent soubor překračuje limit 10 MiB")
	}
	if len(data) < 8 || data[0] != 'd' || !bytes.Contains(data, []byte("4:info")) || data[len(data)-1] != 'e' {
		return "", fmt.Errorf("tracker nevrátil platný .torrent soubor; zkontrolujte nastavení Download link / Alternative RSS")
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("nelze vytvořit lokální cache torrentů: %w", err)
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(cacheDir, fmt.Sprintf("wave-%x.torrent", hash[:8]))
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("torrent soubor nelze uložit: %w", err)
	}
	return path, nil
}

func (a *appServer) handleOpenTorrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input openLinkRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatný požadavek.", http.StatusBadRequest)
		return
	}
	u, err := validateExternalLink(input.URL)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.EqualFold(u.Scheme, "magnet") {
		if err := openWithSystemHandler(u.String()); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		io.WriteString(w, `{"ok":true,"magnet":true}`)
		return
	}
	cacheDir := filepath.Join(filepath.Dir(a.statePath), "torrent-cache")
	path, err := downloadTorrentToCache(u.String(), cacheDir)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := openWithSystemHandler(path); err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "magnet": false, "file": filepath.Base(path)})
}

func (a *appServer) handlePickLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path, err := pickLocalMediaFile()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if path == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "cancelled": true})
		return
	}
	id, err := a.registerMediaFile(path)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id, "file": filepath.Base(path), "path": path, "mediaUrl": a.mediaURL(id)})
}

type mediaRegisterRequest struct {
	Path string `json:"path"`
}

var playableLocalExtensions = map[string]bool{
	".mp4": true, ".m4v": true, ".webm": true, ".ogv": true, ".ogg": true,
	".mp3": true, ".m4a": true, ".aac": true, ".wav": true, ".flac": true,
	".mov": true, ".mkv": true, ".avi": true, ".wmv": true, ".ts": true,
	".mpg": true, ".mpeg": true,
}

func (a *appServer) registerMediaFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("cesta k videosouboru je prázdná")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("neplatná cesta k souboru")
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("vybraný soubor není dostupný")
	}
	if !playableLocalExtensions[strings.ToLower(filepath.Ext(abs))] {
		return "", fmt.Errorf("nepodporovaný typ souboru: %s", filepath.Ext(abs))
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("nelze vytvořit identifikátor média")
	}
	id := hex.EncodeToString(b[:])
	a.mediaMu.Lock()
	if a.mediaFiles == nil {
		a.mediaFiles = make(map[string]string)
	}
	a.mediaFiles[id] = abs
	a.mediaMu.Unlock()
	return id, nil
}

func (a *appServer) mediaURL(id string) string {
	return "/api/media/" + id + "?token=" + url.QueryEscape(a.token)
}

func (a *appServer) handleMediaRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input mediaRegisterRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatný požadavek pro registraci média.", http.StatusBadRequest)
		return
	}
	id, err := a.registerMediaFile(input.Path)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id, "file": filepath.Base(input.Path), "mediaUrl": a.mediaURL(id)})
}

// handleMediaServe exposes only files explicitly registered by the native
// picker or the qBittorrent completed-file workflow. ServeContent supports
// byte ranges, which enables seeking in the in-app video player.
func (a *appServer) handleMediaServe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/media/")
	if id == "" || strings.ContainsAny(id, "/\\") || len(id) != 32 {
		http.NotFound(w, r)
		return
	}
	a.mediaMu.RLock()
	path := a.mediaFiles[id]
	a.mediaMu.RUnlock()
	if path == "" {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(filepath.Base(path), "\"", "")+"\"")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func pickLocalMediaFile() (string, error) {
	if runtime.GOOS == "windows" {
		script := `$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.OpenFileDialog; $d.Title='Vyberte stažené video'; $d.Filter='Video soubory|*.mkv;*.mp4;*.avi;*.mov;*.webm;*.m4v;*.ts;*.mpg;*.mpeg|Všechny soubory|*.*'; $d.Multiselect=$false; if($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){[Console]::Write($d.FileName)}`
		out, err := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script).Output()
		if err != nil {
			return "", fmt.Errorf("dialog pro výběr souboru se nepodařilo otevřít: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	}
	for _, picker := range []string{"zenity", "kdialog"} {
		if bin, err := exec.LookPath(picker); err == nil {
			var cmd *exec.Cmd
			if picker == "zenity" {
				cmd = exec.Command(bin, "--file-selection", "--title=Vyberte stažené video")
			} else {
				cmd = exec.Command(bin, "--getopenfilename", ".", "Video files (*.mkv *.mp4 *.avi *.mov *.webm *.m4v *.ts)")
			}
			out, err := cmd.Output()
			if err != nil {
				return "", nil // User cancelled the picker.
			}
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", fmt.Errorf("nativní dialog pro výběr videa není dostupný; otevřete stažený soubor přímo ve VLC nebo MPV")
}

func openLocalMedia(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("vybraný soubor není dostupný: %w", err)
	}
	if player, args := findExternalPlayer(path); player != "" {
		cmd := exec.Command(player, args...)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("nepodařilo se spustit přehrávač: %w", err)
		}
		go cmd.Wait()
		return nil
	}
	return openWithSystemHandler(path)
}

type qbtConnectRequest struct {
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type qbtAddRequest struct {
	URL string `json:"url"`
}

type qbtActionRequest struct {
	Hash   string `json:"hash"`
	Action string `json:"action"`
}

type qbtTorrentInfo struct {
	Hash       string  `json:"hash"`
	Name       string  `json:"name"`
	Progress   float64 `json:"progress"`
	DLSpeed    int64   `json:"dlspeed"`
	UPSpeed    int64   `json:"upspeed"`
	Size       int64   `json:"size"`
	Downloaded int64   `json:"downloaded"`
	ETA        int64   `json:"eta"`
	State      string  `json:"state"`
	Category   string  `json:"category"`
	AmountLeft int64   `json:"amount_left"`
	Ratio      float64 `json:"ratio"`
	NumSeeds   int     `json:"num_seeds"`
	NumLeechs  int     `json:"num_leechs"`
}

func validateQBTBase(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, fmt.Errorf("Adresa qBittorrent musí být platná HTTP/HTTPS URL.")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("Adresa qBittorrent nesmí obsahovat přihlašovací údaje, query ani fragment.")
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("Z bezpečnostních důvodů lze připojit pouze qBittorrent na tomto počítači (localhost / 127.0.0.1).")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func qbtRequest(client *http.Client, base *url.URL, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	target := *base
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	target.Path = strings.TrimRight(base.Path, "/") + parsedEndpoint.Path
	target.RawQuery = parsedEndpoint.RawQuery
	req, err := http.NewRequest(method, target.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", strings.TrimRight(base.String(), "/")+"/")
	req.Header.Set("Origin", strings.TrimRight(base.String(), "/"))
	req.Header.Set("User-Agent", "WAVE-IPTV/1.6.0")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return client.Do(req)
}

func qbtErrorBody(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	return strings.TrimSpace(string(b))
}

func (a *appServer) handleQBTConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input qbtConnectRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatné nastavení qBittorrent.", http.StatusBadRequest)
		return
	}
	base, err := validateQBTBase(input.BaseURL)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		writeJSONError(w, "Vyplňte uživatelské jméno a heslo qBittorrent Web UI.", http.StatusBadRequest)
		return
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 12 * time.Second}
	form := url.Values{"username": {input.Username}, "password": {input.Password}}
	resp, err := qbtRequest(client, base, http.MethodPost, "/api/v2/auth/login", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		writeJSONError(w, "qBittorrent není dostupný. Zkontrolujte, zda běží a má zapnuté Web UI.", http.StatusBadGateway)
		return
	}
	loginBody := qbtErrorBody(resp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || (loginBody != "Ok." && loginBody != "") {
		writeJSONError(w, "Přihlášení k qBittorrent se nezdařilo. Zkontrolujte údaje a Web UI.", http.StatusUnauthorized)
		return
	}
	resp, err = qbtRequest(client, base, http.MethodGet, "/api/v2/app/version", nil, "")
	if err != nil {
		writeJSONError(w, "Přihlášení proběhlo, ale qBittorrent API neodpovídá.", http.StatusBadGateway)
		return
	}
	version := qbtErrorBody(resp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSONError(w, "qBittorrent API odmítlo požadavek. Zkontrolujte Web UI a oprávnění.", http.StatusBadGateway)
		return
	}
	a.qbtMu.Lock()
	a.qbtBase = strings.TrimRight(base.String(), "/")
	a.qbtClient = client
	a.qbtVersion = version
	a.qbtMu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "version": version})
}

func (a *appServer) qbtSession() (string, *http.Client, string, error) {
	a.qbtMu.Lock()
	defer a.qbtMu.Unlock()
	if a.qbtClient == nil {
		return "", nil, "", fmt.Errorf("qBittorrent není připojen.")
	}
	return a.qbtBase, a.qbtClient, a.qbtVersion, nil
}

func (a *appServer) handleQBTDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a.qbtMu.Lock()
	a.qbtBase = ""
	a.qbtClient = nil
	a.qbtVersion = ""
	a.qbtMu.Unlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, `{"ok":true}`)
}

func (a *appServer) handleQBTStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	base, client, version, err := a.qbtSession()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	u, _ := url.Parse(base)
	resp, err := qbtRequest(client, u, http.MethodGet, "/api/v2/transfer/info", nil, "")
	if err != nil {
		writeJSONError(w, "Spojení s qBittorrent se přerušilo.", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSONError(w, "qBittorrent odmítl stavový dotaz. Znovu se připojte.", http.StatusBadGateway)
		return
	}
	var transfer map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&transfer); err != nil {
		writeJSONError(w, "qBittorrent vrátil neplatný stav.", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "connected": true, "version": version, "transfer": transfer})
}

func (a *appServer) handleQBTTorrents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	base, client, _, err := a.qbtSession()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	u, _ := url.Parse(base)
	resp, err := qbtRequest(client, u, http.MethodGet, "/api/v2/torrents/info?filter=all", nil, "")
	if err != nil {
		writeJSONError(w, "Nepodařilo se načíst stahování z qBittorrent.", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSONError(w, "qBittorrent odmítl seznam stahování.", http.StatusBadGateway)
		return
	}
	var torrents []qbtTorrentInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&torrents); err != nil {
		writeJSONError(w, "qBittorrent vrátil neplatný seznam stahování.", http.StatusBadGateway)
		return
	}
	if len(torrents) > 200 {
		torrents = torrents[:200]
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "torrents": torrents})
}

func (a *appServer) handleQBTAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input qbtAddRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatný torrent odkaz.", http.StatusBadRequest)
		return
	}
	target, err := validateExternalLink(input.URL)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	base, client, _, err := a.qbtSession()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	u, _ := url.Parse(base)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if err := writer.WriteField("urls", target.String()); err != nil {
		writeJSONError(w, "Nepodařilo se připravit torrent.", http.StatusInternalServerError)
		return
	}
	if err := writer.Close(); err != nil {
		writeJSONError(w, "Nepodařilo se připravit torrent.", http.StatusInternalServerError)
		return
	}
	body := &buffer
	contentType := writer.FormDataContentType()
	resp, err := qbtRequest(client, u, http.MethodPost, "/api/v2/torrents/add", body, contentType)
	if err != nil {
		writeJSONError(w, "qBittorrent není dostupný.", http.StatusBadGateway)
		return
	}
	result := qbtErrorBody(resp)
	status := resp.StatusCode
	resp.Body.Close()
	if status < 200 || status >= 300 || (result != "Ok." && result != "" && !strings.Contains(strings.ToLower(result), "success")) {
		writeJSONError(w, "qBittorrent nepřidal torrent: "+result, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "message": "Torrent byl předán qBittorrent."})
}

func (a *appServer) handleQBTAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input qbtActionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatná akce.", http.StatusBadRequest)
		return
	}
	if !regexp.MustCompile(`(?i)^[a-f0-9]{40,64}$`).MatchString(input.Hash) {
		writeJSONError(w, "Neplatný identifikátor torrentu.", http.StatusBadRequest)
		return
	}
	endpoint := ""
	switch input.Action {
	case "pause":
		endpoint = "/api/v2/torrents/stop"
	case "resume":
		endpoint = "/api/v2/torrents/start"
	default:
		writeJSONError(w, "Nepodporovaná akce.", http.StatusBadRequest)
		return
	}
	base, client, _, err := a.qbtSession()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	u, _ := url.Parse(base)
	form := url.Values{"hashes": {input.Hash}}
	resp, err := qbtRequest(client, u, http.MethodPost, endpoint, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		writeJSONError(w, "qBittorrent není dostupný.", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSONError(w, "qBittorrent akci odmítl.", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, `{"ok":true}`)
}

type qbtPlayRequest struct {
	Hash string `json:"hash"`
}

type qbtTorrentPathInfo struct {
	SavePath    string  `json:"save_path"`
	ContentPath string  `json:"content_path"`
	Progress    float64 `json:"progress"`
}

type qbtFileInfo struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
}

func completedVideoPath(contentPath, savePath string, files []qbtFileInfo) (string, error) {
	allowed := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true, ".webm": true, ".m4v": true, ".ts": true, ".mpg": true, ".mpeg": true, ".wmv": true}
	// A single-file torrent exposes content_path as the full file path.
	if contentPath != "" && allowed[strings.ToLower(filepath.Ext(contentPath))] {
		if info, err := os.Stat(contentPath); err == nil && info.Mode().IsRegular() {
			return contentPath, nil
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Size > files[j].Size })
	roots := []string{}
	for _, root := range []string{contentPath, savePath} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		clean := filepath.Clean(root)
		if clean == "." {
			continue
		}
		if !slices.Contains(roots, clean) {
			roots = append(roots, clean)
		}
	}
	for _, file := range files {
		if file.Progress < 0.999 || file.Size <= 0 || !allowed[strings.ToLower(filepath.Ext(file.Name))] {
			continue
		}
		name := filepath.Clean(filepath.FromSlash(file.Name))
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			continue
		}
		for _, root := range roots {
			candidate := filepath.Clean(filepath.Join(root, name))
			rel, err := filepath.Rel(root, candidate)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				continue
			}
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() {
				return candidate, nil
			}
			// Some clients report the torrent root folder in the relative filename.
			if base := filepath.Base(name); base != name {
				fallback := filepath.Join(root, base)
				if info, err := os.Stat(fallback); err == nil && info.Mode().IsRegular() {
					return fallback, nil
				}
			}
		}
	}
	return "", fmt.Errorf("V dokončeném torrentu nebyl nalezen dostupný videosoubor. Zkontrolujte cílovou složku a dokončení stahování.")
}

func (a *appServer) handleQBTPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var input qbtPlayRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&input); err != nil {
		writeJSONError(w, "Neplatný požadavek pro přehrávání.", http.StatusBadRequest)
		return
	}
	if !regexp.MustCompile(`(?i)^[a-f0-9]{40,64}$`).MatchString(input.Hash) {
		writeJSONError(w, "Neplatný identifikátor torrentu.", http.StatusBadRequest)
		return
	}
	base, client, _, err := a.qbtSession()
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	u, _ := url.Parse(base)
	infoURL := "/api/v2/torrents/info?hash=" + url.QueryEscape(input.Hash)
	resp, err := qbtRequest(client, u, http.MethodGet, infoURL, nil, "")
	if err != nil {
		writeJSONError(w, "Nepodařilo se načíst detail torrentu.", http.StatusBadGateway)
		return
	}
	var torrents []qbtTorrentPathInfo
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&torrents)
	status := resp.StatusCode
	resp.Body.Close()
	if status != http.StatusOK || decodeErr != nil || len(torrents) == 0 {
		writeJSONError(w, "Torrent nebyl nalezen.", http.StatusNotFound)
		return
	}
	if torrents[0].Progress < 0.999 {
		writeJSONError(w, "Titul ještě není kompletně stažený.", http.StatusConflict)
		return
	}
	filesURL := "/api/v2/torrents/files?hash=" + url.QueryEscape(input.Hash)
	resp, err = qbtRequest(client, u, http.MethodGet, filesURL, nil, "")
	if err != nil {
		writeJSONError(w, "Nepodařilo se načíst soubory torrentu.", http.StatusBadGateway)
		return
	}
	var files []qbtFileInfo
	decodeErr = json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&files)
	status = resp.StatusCode
	resp.Body.Close()
	if status != http.StatusOK || decodeErr != nil {
		writeJSONError(w, "qBittorrent vrátil neplatný seznam souborů.", http.StatusBadGateway)
		return
	}
	path, err := completedVideoPath(torrents[0].ContentPath, torrents[0].SavePath, files)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusNotFound)
		return
	}
	id, err := a.registerMediaFile(path)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "id": id, "file": filepath.Base(path), "path": path, "mediaUrl": a.mediaURL(id)})
}

func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (a *appServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a.lastSeen.Store(time.Now().Unix())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, "ok")
}

func (a *appServer) handleQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"closing":true}`)
	go func() {
		time.Sleep(250 * time.Millisecond)
		if a.server != nil {
			a.shutting.Store(true)
			a.server.Close()
			os.Exit(0)
		}
	}()
}

func (a *appServer) watchdog() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if a.shutting.Load() {
			return
		}
		if time.Since(time.Unix(a.lastSeen.Load(), 0)) > 4*time.Minute {
			a.log("no active browser session; shutting down local host")
			os.Exit(0)
		}
	}
}

func (a *appServer) log(message string) {
	line := time.Now().Format(time.RFC3339) + " " + message + "\n"
	f, err := os.OpenFile(a.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		f.WriteString(line)
		f.Close()
	}
}

func writeLog(message string) {
	configDir, _ := os.UserConfigDir()
	if configDir == "" {
		configDir = os.TempDir()
	}
	appDir := filepath.Join(configDir, appName)
	os.MkdirAll(appDir, 0700)
	f, err := os.OpenFile(filepath.Join(appDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), message)
		f.Close()
	}
}

func openAppWindow(address string) error {
	if runtime.GOOS == "windows" {
		candidates := []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		}
		for _, path := range candidates {
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				return exec.Command(path, "--app="+address, "--new-window").Start()
			}
		}
		if browser, err := exec.LookPath("msedge.exe"); err == nil {
			return exec.Command(browser, "--app="+address, "--new-window").Start()
		}
		if browser, err := exec.LookPath("chrome.exe"); err == nil {
			return exec.Command(browser, "--app="+address, "--new-window").Start()
		}
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", address).Start()
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", address).Start()
	}
	return exec.Command("xdg-open", address).Start()
}
