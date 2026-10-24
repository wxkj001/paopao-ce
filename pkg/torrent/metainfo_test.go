package torrent

import (
	"strings"
	"testing"

	"github.com/rocboss/paopao-ce/pkg/bencode"
)

func TestParseAndSanitizeTorrent(t *testing.T) {
	// Build a mock torrent dictionary
	samplePieces := strings.Repeat("12345678901234567890", 5) // 5 pieces of 20 bytes
	info := map[string]any{
		"name":         "test_movie.mkv",
		"piece length": int64(1048576),
		"pieces":       samplePieces,
		"length":       int64(5242880),
	}
	torrentDict := map[string]any{
		"announce": "http://public.tracker.com/announce",
		"comment":  "Test Torrent",
		"info":     info,
	}

	rawBytes, err := bencode.EncodeBytes(torrentDict)
	if err != nil {
		t.Fatalf("Failed to encode mock torrent: %v", err)
	}

	// 1. Test Parse
	meta, err := ParseBytes(rawBytes)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if meta.Name != "test_movie.mkv" {
		t.Fatalf("Expected name 'test_movie.mkv', got %s", meta.Name)
	}
	if meta.TotalLength != 5242880 {
		t.Fatalf("Expected length 5242880, got %d", meta.TotalLength)
	}
	if meta.NumPieces != 5 {
		t.Fatalf("Expected 5 pieces, got %d", meta.NumPieces)
	}
	if meta.IsPrivate {
		t.Fatalf("Expected public torrent initially")
	}

	// 2. Test Sanitize for PT
	defaultAnnounce := "http://127.0.0.1:8008/tracker/announce"
	sanitizedBytes, _, err := SanitizeForPT(rawBytes, defaultAnnounce, "[PaoPao] PaoPao PT")
	if err != nil {
		t.Fatalf("SanitizeForPT failed: %v", err)
	}

	sanitizedMeta, err := ParseBytes(sanitizedBytes)
	if err != nil {
		t.Fatalf("Failed to parse sanitized torrent: %v", err)
	}
	if !sanitizedMeta.IsPrivate {
		t.Fatalf("Expected sanitized torrent to have private=1")
	}
	if sanitizedMeta.Announce != defaultAnnounce {
		t.Fatalf("Expected announce to be %s, got %s", defaultAnnounce, sanitizedMeta.Announce)
	}

	// 3. Test Customize for User
	userPasskey := "1234567890abcdef1234567890abcdef"
	wsAnnounce := "ws://127.0.0.1:8008/tracker/ws"
	customBytes, err := CustomizeForUser(sanitizedBytes, userPasskey, defaultAnnounce, wsAnnounce)
	if err != nil {
		t.Fatalf("CustomizeForUser failed: %v", err)
	}

	customMeta, err := ParseBytes(customBytes)
	if err != nil {
		t.Fatalf("Failed to parse custom torrent: %v", err)
	}
	expectedAnnounce := defaultAnnounce + "?passkey=" + userPasskey
	if customMeta.Announce != expectedAnnounce {
		t.Fatalf("Expected customized announce %s, got %s", expectedAnnounce, customMeta.Announce)
	}
}
