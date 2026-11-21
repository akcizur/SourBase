package playlist

import (
	"strings"
	"testing"
)

func TestParseExtendedM3U(t *testing.T) {
	input := `#EXTM3U
#EXTINF:-1 tvg-id="ct1" tvg-logo="https://img.example/ct.png" group-title="CZ | General",ČT1 HD
https://media.example/live/ct1.m3u8
#EXTINF:-1 group-title="SK | News",TA3
https://media.example/live/ta3
`
	got, err := Parse(strings.NewReader(input))
	if err != nil { t.Fatal(err) }
	if len(got) != 2 { t.Fatalf("got %d channels", len(got)) }
	if got[0].Name != "ČT1 HD" || got[0].TVGID != "ct1" || got[0].Group != "CZ | General" { t.Fatalf("unexpected first channel: %#v", got[0]) }
	if got[1].Name != "TA3" || got[1].Group != "SK | News" { t.Fatalf("unexpected second channel: %#v", got[1]) }
}

func TestParseRejectsInvalidHeaderAndProtocols(t *testing.T) {
	if _, err := Parse(strings.NewReader("https://example.com/live\n")); err == nil { t.Fatal("expected header error") }
	input := "#EXTM3U\n#EXTINF:-1,Bad\nfile:///etc/passwd\n#EXTINF:-1,Good\nhttps://example.com/live\n"
	got, err := Parse(strings.NewReader(input))
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Name != "Good" { t.Fatalf("unexpected parsed channels: %#v", got) }
}
