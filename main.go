package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/akcizur/studious-octo-engine/internal/playlist"
	"github.com/akcizur/studious-octo-engine/internal/state"
)

//go:embed web/*
var webFiles embed.FS

const maxImportBytes = 24 << 20

type application struct {
	store *state.Store
	token string
}
type importRequest struct {
	Name string `json:"name"`
	Content string `json:"content"`
}
type favoriteRequest struct {
	ChannelID string `json:"channelId"`
}
type activeRequest struct {
	PlaylistID string `json:"playlistId"`
}

func main() {
	base, err := os.UserConfigDir()
	if err != nil { log.Fatal(err) }
	appDir := filepath.Join(base, "WAVE IPTV")
	if err := os.MkdirAll(appDir, 0700); err != nil { log.Fatal(err) }
	store, err := state.Open(filepath.Join(appDir, "library.dat"))
	if err != nil { log.Fatalf("open local library: %v", err) }
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil { log.Fatal(err) }
	app := &application{store: store, token: hex.EncodeToString(tokenBytes)}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil { log.Fatal(err) }
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", app.handleHealth)
	mux.HandleFunc("/api/state", app.handleState)
	mux.HandleFunc("/api/playlists/import", app.handleImportPlaylist)
	mux.HandleFunc("/api/playlists/", app.handlePlaylist)
	mux.HandleFunc("/api/favorites", app.handleFavorites)
	mux.HandleFunc("/api/active", app.handleActivePlaylist)
	mux.HandleFunc("/", app.handleWeb)
	server := &http.Server{Handler: securityHeaders(mux), ReadHeaderTimeout: 5*time.Second, ReadTimeout: 30*time.Second, WriteTimeout: 60*time.Second, IdleTimeout: 60*time.Second}
	address := "http://" + listener.Addr().String()
	if os.Getenv("WAVE_NO_BROWSER") != "1" {
		go openBrowser(address)
	}
	log.Printf("WAVE local UI: %s", address)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) { log.Fatal(err) }
}

func (a *application) authorized(r *http.Request) bool {
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if len(provided) != len(a.token) || provided == "" { return false }
	return subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) == 1
}
func (a *application) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w, http.MethodGet); return }
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": "WAVE IPTV", "version": "0.1.0"})
}
func (a *application) handleState(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) { writeError(w, 401, "Nepovolený požadavek."); return }
	if r.Method != http.MethodGet { methodNotAllowed(w, http.MethodGet); return }
	writeJSON(w, 200, a.store.Snapshot())
}
func (a *application) handleImportPlaylist(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) { writeError(w, 401, "Nepovolený požadavek."); return }
	if r.Method != http.MethodPost { methodNotAllowed(w, http.MethodPost); return }
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxImportBytes+1))
	if err != nil || len(body) > maxImportBytes { writeError(w, 413, "Playlist je příliš velký (limit 24 MB)."); return }
	var input importRequest
	if err := json.Unmarshal(body, &input); err != nil { writeError(w, 400, "Neplatný požadavek."); return }
	channels, err := playlist.Parse(strings.NewReader(input.Content))
	if err != nil { writeError(w, 400, err.Error()); return }
	name := strings.TrimSpace(input.Name)
	if name == "" { name = "Můj playlist" }
	if len(name) > 120 { name = name[:120] }
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil { writeError(w, 500, "Nepodařilo se vytvořit ID playlistu."); return }
	entry := state.Playlist{ID: hex.EncodeToString(idBytes), Name: name, ImportedAt: time.Now().UTC(), Channels: channels}
	if err := a.store.AddPlaylist(entry); err != nil { writeError(w, 500, "Playlist se nepodařilo uložit."); return }
	writeJSON(w, 201, map[string]any{"playlist": entry, "channelCount": len(channels)})
}
func (a *application) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) { writeError(w, 401, "Nepovolený požadavek."); return }
	if r.Method != http.MethodDelete { methodNotAllowed(w, http.MethodDelete); return }
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/playlists/"), "/")
	if id == "" || strings.Contains(id, "/") { writeError(w, 400, "Neplatné ID playlistu."); return }
	if err := a.store.RemovePlaylist(id); err != nil { writeError(w, 404, "Playlist nebyl nalezen."); return }
	writeJSON(w, 200, map[string]bool{"deleted": true})
}
func (a *application) handleFavorites(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) { writeError(w, 401, "Nepovolený požadavek."); return }
	if r.Method != http.MethodPost { methodNotAllowed(w, http.MethodPost); return }
	defer r.Body.Close()
	var input favoriteRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&input); err != nil || strings.TrimSpace(input.ChannelID) == "" { writeError(w, 400, "Chybí ID kanálu."); return }
	if err := a.store.ToggleFavorite(input.ChannelID); err != nil { writeError(w, 500, "Oblíbené položky se nepodařilo uložit."); return }
	writeJSON(w, 200, a.store.Snapshot())
}
func (a *application) handleActivePlaylist(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) { writeError(w, 401, "Nepovolený požadavek."); return }
	if r.Method != http.MethodPost { methodNotAllowed(w, http.MethodPost); return }
	defer r.Body.Close()
	var input activeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&input); err != nil { writeError(w, 400, "Neplatný požadavek."); return }
	if err := a.store.SetActivePlaylist(input.PlaylistID); err != nil { writeError(w, 404, "Playlist nebyl nalezen."); return }
	writeJSON(w, 200, a.store.Snapshot())
}
func (a *application) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w, http.MethodGet); return }
	if r.URL.Path == "/" {
		content, err := webFiles.ReadFile("web/index.html")
		if err != nil { http.Error(w, "UI is unavailable", 500); return }
		page := strings.ReplaceAll(string(content), "__WAVE_TOKEN__", a.token)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, page)
		return
	}
	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil { http.NotFound(w, r); return }
	http.FileServer(http.FS(webRoot)).ServeHTTP(w, r)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) { writeJSON(w, status, map[string]string{"error": message}) }
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, 405, "Metoda není povolena.")
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self' blob: data:; img-src 'self' https: data: blob:; media-src 'self' https: http: blob:; style-src 'self'; script-src 'self'; connect-src 'self' https: http:")
		next.ServeHTTP(w, r)
	})
}
func openBrowser(address string) {
	var command string; var args []string
	switch runtime.GOOS {
	case "windows": command, args = "rundll32.exe", []string{"url.dll,FileProtocolHandler", address}
	case "darwin": command, args = "open", []string{address}
	default: command, args = "xdg-open", []string{address}
	}
	if err := exec.Command(command, args...).Start(); err != nil { log.Printf("open browser manually at %s: %v", address, err) }
}
