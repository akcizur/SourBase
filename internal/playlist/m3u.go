package playlist

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const maxLineBytes = 2 << 20
const maxChannels = 50000
var attributePattern = regexp.MustCompile(`([A-Za-z0-9_-]+)="([^"]*)"`)

type Channel struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Group string `json:"group"`
	Logo string `json:"logo"`
	TVGID string `json:"tvgId"`
	URL string `json:"url"`
	Duration float64 `json:"duration"`
}
type pendingEntry struct { attributes map[string]string; name string; duration float64 }

// Parse reads Extended M3U without fetching referenced streams.
func Parse(source io.Reader) ([]Channel, error) {
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	var channels []Channel
	var pending *pendingEntry
	lineNumber := 0
	validHeader := false
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" { continue }
		if strings.HasPrefix(line, "#EXTM3U") { validHeader = true; continue }
		if strings.HasPrefix(line, "#EXTINF:") { pending = parseExtInf(strings.TrimPrefix(line, "#EXTINF:")); continue }
		if strings.HasPrefix(line, "#") { continue }
		if pending == nil { pending = &pendingEntry{attributes: map[string]string{}, name: titleFromURL(line)} }
		if !validStreamURL(line) { pending = nil; continue }
		name := strings.TrimSpace(pending.name)
		if name == "" { name = titleFromURL(line) }
		attrs := pending.attributes
		id := attrs["tvg-id"]
		if id == "" { id = stableID(name, line) }
		channels = append(channels, Channel{ID:id, Name:name, Group:firstNonEmpty(attrs["group-title"],"Ostatní"), Logo:attrs["tvg-logo"], TVGID:attrs["tvg-id"], URL:line, Duration:pending.duration})
		pending = nil
		if len(channels) > maxChannels { return nil, fmt.Errorf("playlist překračuje limit %d kanálů", maxChannels) }
	}
	if err := scanner.Err(); err != nil { return nil, fmt.Errorf("chyba při čtení playlistu na řádku %d: %w", lineNumber, err) }
	if !validHeader { return nil, fmt.Errorf("soubor neobsahuje hlavičku #EXTM3U") }
	if len(channels) == 0 { return nil, fmt.Errorf("playlist neobsahuje žádné platné streamy") }
	return channels, nil
}
func parseExtInf(value string) *pendingEntry {
	attributes := make(map[string]string)
	for _, match := range attributePattern.FindAllStringSubmatch(value,-1) { attributes[strings.ToLower(match[1])] = strings.TrimSpace(match[2]) }
	name := ""
	if comma := strings.Index(value,","); comma >= 0 && comma+1 < len(value) { name = strings.TrimSpace(value[comma+1:]) }
	if name == "" { name = firstNonEmpty(attributes["tvg-name"],attributes["tvg-id"]) }
	fields := strings.Fields(value)
	var duration float64
	if len(fields)>0 { duration,_ = strconv.ParseFloat(strings.TrimSuffix(fields[0],","),64) }
	return &pendingEntry{attributes:attributes,name:name,duration:duration}
}
func validStreamURL(raw string) bool {
	u,err := url.Parse(strings.TrimSpace(raw)); if err != nil { return false }
	switch strings.ToLower(u.Scheme) { case "http","https","rtsp","rtmp","udp": return u.Host != ""; default: return false }
}
func stableID(name, streamURL string) string {
	sum:=sha256.Sum256([]byte(strings.ToLower(name)+"\x00"+streamURL))
	return hex.EncodeToString(sum[:12])
}
func titleFromURL(raw string) string {
	u,err:=url.Parse(raw); if err==nil { base:=path.Base(u.Path); if base!="."&&base!="/"&&base!="" { return strings.TrimSuffix(base,path.Ext(base)) } }
	return "TV kanál"
}
func firstNonEmpty(values ...string) string { for _,v:=range values { if strings.TrimSpace(v)!="" { return strings.TrimSpace(v) } }; return "" }
