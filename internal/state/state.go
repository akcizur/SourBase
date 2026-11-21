package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/akcizur/studious-octo-engine/internal/playlist"
)

type Playlist struct {
	ID string `json:"id"`
	Name string `json:"name"`
	ImportedAt time.Time `json:"importedAt"`
	Channels []playlist.Channel `json:"channels"`
}
type Snapshot struct {
	SchemaVersion int `json:"schemaVersion"`
	Playlists []Playlist `json:"playlists"`
	ActivePlaylist string `json:"activePlaylist"`
	Favorites []string `json:"favorites"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Store struct { mu sync.RWMutex; path string; data Snapshot }

func Open(path string) (*Store,error) {
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
	s:=&Store{path:path,data:Snapshot{SchemaVersion:1,Playlists:[]Playlist{},Favorites:[]string{}}}
	blob,err:=os.ReadFile(path)
	if err==nil {
		plain,legacy,decErr:=decodeState(blob);if decErr!=nil{return nil,decErr}
		if err:=json.Unmarshal(plain,&s.data);err!=nil{return nil,err}
		if s.data.Playlists==nil{s.data.Playlists=[]Playlist{}}
		if s.data.Favorites==nil{s.data.Favorites=[]string{}}
		if s.data.SchemaVersion==0{s.data.SchemaVersion=1}
		if legacy{if err:=s.persistLocked();err!=nil{return nil,err}}
	} else if !errors.Is(err,os.ErrNotExist){return nil,err}
	return s,nil
}
func(s *Store) Snapshot() Snapshot {
	s.mu.RLock();defer s.mu.RUnlock()
	copyData:=s.data
	copyData.Playlists=append([]Playlist(nil),s.data.Playlists...)
	copyData.Favorites=append([]string(nil),s.data.Favorites...)
	for i:=range copyData.Playlists{copyData.Playlists[i].Channels=append([]playlist.Channel(nil),s.data.Playlists[i].Channels...)}
	return copyData
}
func(s *Store) AddPlaylist(p Playlist)error{
	s.mu.Lock();defer s.mu.Unlock()
	for _,existing:=range s.data.Playlists{if existing.ID==p.ID{return errors.New("playlist already exists")}}
	s.data.Playlists=append([]Playlist{p},s.data.Playlists...)
	if s.data.ActivePlaylist==""{s.data.ActivePlaylist=p.ID}
	return s.persistLocked()
}
func(s *Store) RemovePlaylist(id string)error{
	s.mu.Lock();defer s.mu.Unlock()
	found:=false;next:=make([]Playlist,0,len(s.data.Playlists))
	for _,p:=range s.data.Playlists{if p.ID==id{found=true;continue};next=append(next,p)}
	if !found{return os.ErrNotExist}
	s.data.Playlists=next
	if s.data.ActivePlaylist==id{s.data.ActivePlaylist="";if len(next)>0{s.data.ActivePlaylist=next[0].ID}}
	channelIDs:=map[string]bool{}
	for _,p:=range next{for _,c:=range p.Channels{channelIDs[c.ID]=true}}
	favs:=make([]string,0,len(s.data.Favorites));for _,f:=range s.data.Favorites{if channelIDs[f]{favs=append(favs,f)}};s.data.Favorites=favs
	return s.persistLocked()
}
func(s *Store) ToggleFavorite(id string)error{
	s.mu.Lock();defer s.mu.Unlock()
	for i,v:=range s.data.Favorites{if v==id{s.data.Favorites=append(s.data.Favorites[:i],s.data.Favorites[i+1:]...);return s.persistLocked()}}
	s.data.Favorites=append(s.data.Favorites,id);return s.persistLocked()
}
func(s *Store) SetActivePlaylist(id string)error{
	s.mu.Lock();defer s.mu.Unlock()
	if id!=""{found:=false;for _,p:=range s.data.Playlists{if p.ID==id{found=true;break}};if !found{return os.ErrNotExist}}
	s.data.ActivePlaylist=id;return s.persistLocked()
}
func(s *Store) persistLocked()error{
	s.data.UpdatedAt=time.Now().UTC()
	b,err:=json.MarshalIndent(s.data,"","  ");if err!=nil{return err}
	encoded,err:=encodeState(b);if err!=nil{return err}
	tmp:=s.path+".tmp"
	f,err:=os.OpenFile(tmp,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return err}
	if _,err=f.Write(encoded);err!=nil{f.Close();os.Remove(tmp);return err}
	if err=f.Sync();err!=nil{f.Close();os.Remove(tmp);return err}
	if err=f.Close();err!=nil{os.Remove(tmp);return err}
	if err=os.Chmod(tmp,0600);err!=nil&&os.PathSeparator=='/'{os.Remove(tmp);return err}
	if err=replaceFile(tmp,s.path);err!=nil{os.Remove(tmp);return err}
	return nil
}
