package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akcizur/studious-octo-engine/internal/state"
)

func TestImportPlaylistAPIRequiresTokenAndPersists(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "library.dat"))
	if err != nil { t.Fatal(err) }
	app := &application{store: store, token: "test-token"}
	payload := `{"name":"Local test","content":"#EXTM3U\n#EXTINF:-1,Demo TV\nhttps://media.example/live/demo.m3u8\n"}`

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/playlists/import", strings.NewReader(payload))
	unauthorizedRec := httptest.NewRecorder()
	app.handleImportPlaylist(unauthorizedRec, unauthorized)
	if unauthorizedRec.Code != http.StatusUnauthorized { t.Fatalf("expected 401, got %d", unauthorizedRec.Code) }

	req := httptest.NewRequest(http.MethodPost, "/api/playlists/import", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	app.handleImportPlaylist(rec, req)
	if rec.Code != http.StatusCreated { t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String()) }
	var result struct { ChannelCount int `json:"channelCount"` }
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil { t.Fatal(err) }
	if result.ChannelCount != 1 { t.Fatalf("expected one imported channel, got %d", result.ChannelCount) }
	if got := len(store.Snapshot().Playlists); got != 1 { t.Fatalf("playlist was not persisted: %d", got) }
}

func TestWebHandlerInjectsPerRunToken(t *testing.T) {
	store, err := state.Open(filepath.Join(t.TempDir(), "library.dat"))
	if err != nil { t.Fatal(err) }
	app := &application{store: store, token: "test-token-123"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.handleWeb(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d", rec.Code) }
	if !strings.Contains(rec.Body.String(), `content="test-token-123"`) { t.Fatal("token was not injected into local UI") }
	if strings.Contains(rec.Body.String(), "__WAVE_TOKEN__") { t.Fatal("token placeholder remains in UI") }
}
