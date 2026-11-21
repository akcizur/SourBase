package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/akcizur/studious-octo-engine/internal/playlist"
)

func TestStorePersistsPlaylistsAndFavorites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.dat")
	store, err := Open(path)
	if err != nil { t.Fatal(err) }
	entry := Playlist{ID:"pl-1", Name:"Test playlist", ImportedAt:time.Now().UTC(), Channels:[]playlist.Channel{{ID:"ch-1",Name:"ČT1",Group:"CZ",URL:"https://example.test/live"}}}
	if err := store.AddPlaylist(entry); err != nil { t.Fatal(err) }
	if err := store.ToggleFavorite("ch-1"); err != nil { t.Fatal(err) }
	if err := store.SetActivePlaylist("pl-1"); err != nil { t.Fatal(err) }
	reloaded, err := Open(path)
	if err != nil { t.Fatal(err) }
	snapshot := reloaded.Snapshot()
	if len(snapshot.Playlists) != 1 || snapshot.Playlists[0].Name != "Test playlist" { t.Fatalf("playlist snapshot mismatch: %#v", snapshot.Playlists) }
	if snapshot.ActivePlaylist != "pl-1" { t.Fatalf("active playlist mismatch: %q", snapshot.ActivePlaylist) }
	if len(snapshot.Favorites) != 1 || snapshot.Favorites[0] != "ch-1" { t.Fatalf("favorites mismatch: %#v", snapshot.Favorites) }
}

func TestRemovePlaylistCleansOrphanFavorites(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "library.dat"))
	if err != nil { t.Fatal(err) }
	if err := store.AddPlaylist(Playlist{ID:"a",Name:"A",Channels:[]playlist.Channel{{ID:"one",Name:"One"}}}); err != nil { t.Fatal(err) }
	if err := store.AddPlaylist(Playlist{ID:"b",Name:"B",Channels:[]playlist.Channel{{ID:"two",Name:"Two"}}}); err != nil { t.Fatal(err) }
	if err := store.ToggleFavorite("one"); err != nil { t.Fatal(err) }
	if err := store.ToggleFavorite("two"); err != nil { t.Fatal(err) }
	if err := store.RemovePlaylist("a"); err != nil { t.Fatal(err) }
	got := store.Snapshot()
	if len(got.Playlists) != 1 || got.Playlists[0].ID != "b" { t.Fatalf("unexpected playlists: %#v", got.Playlists) }
	if len(got.Favorites) != 1 || got.Favorites[0] != "two" { t.Fatalf("orphan favorite was not removed: %#v", got.Favorites) }
}
