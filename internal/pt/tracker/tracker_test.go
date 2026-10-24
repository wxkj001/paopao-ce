package tracker

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/rocboss/paopao-ce/pkg/bencode"
)

func TestParseRawQueryBinary(t *testing.T) {
	// Query string containing non-ascii and binary bytes
	raw := "passkey=abcdef123456&info_hash=%00%ff%01%ab&port=6881"
	params := parseRawQuery(raw)

	pk := string(params.getFirst("passkey"))
	if pk != "abcdef123456" {
		t.Fatalf("Expected passkey abcdef123456, got %s", pk)
	}

	ih := params.getFirst("info_hash")
	expectedIH := []byte{0x00, 0xff, 0x01, 0xab}
	if !bytes.Equal(ih, expectedIH) {
		t.Fatalf("Expected info_hash %v, got %v", expectedIH, ih)
	}
}

func TestCheckClient(t *testing.T) {
	// 1. Browser should be blocked
	browserAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	peerID := []byte("-qB4390-123456789012")
	if CheckClient(peerID, browserAgent) == "" {
		t.Fatalf("Expected browser to be blocked")
	}

	// 2. Allowed qBittorrent client
	qbAgent := "qBittorrent/4.3.9"
	if errStr := CheckClient(peerID, qbAgent); errStr != "" {
		t.Fatalf("Expected qBittorrent to be allowed, got: %s", errStr)
	}

	// 3. Transmission
	trPeerID := []byte("-TR3000-123456789012")
	trAgent := "Transmission/3.00"
	if errStr := CheckClient(trPeerID, trAgent); errStr != "" {
		t.Fatalf("Expected Transmission to be allowed, got: %s", errStr)
	}
}

// MockPTService for Tracker tests
type mockPTService struct {
	corept.PTService
}

func (m *mockPTService) HandleAnnounce(req *corept.AnnounceReq) (*corept.AnnounceResp, error) {
	return &corept.AnnounceResp{
		Interval:    1800,
		MinInterval: 300,
		Complete:    5,
		Incomplete:  2,
		Peers: []corept.AnnouncePeer{
			{IP: "1.2.3.4", Port: 6881, PeerID: "peer1"},
		},
		PeersBinary: []byte{1, 2, 3, 4, 0x1a, 0xe1}, // 1.2.3.4:6881
	}, nil
}

func (m *mockPTService) HandleScrape(req *corept.ScrapeReq) (*corept.ScrapeResp, error) {
	var h [20]byte
	copy(h[:], req.InfoHashes[0])
	return &corept.ScrapeResp{
		Files: []corept.ScrapeFile{
			{InfoHash: h, Complete: 10, Downloaded: 50, Incomplete: 1},
		},
	}, nil
}

func TestTrackerHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tracker := NewTracker(&mockPTService{})

	r := gin.New()
	r.GET("/tracker/announce", tracker.HandleAnnounce)
	r.GET("/tracker/scrape", tracker.HandleScrape)

	// Test Announce
	peerID20 := "-qB4390-123456789012"
	infoHash20 := strings.Repeat("a", 20)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tracker/announce?passkey=testpasskey&info_hash="+infoHash20+"&peer_id="+peerID20+"&port=6881&compact=1", nil)
	req.Header.Set("User-Agent", "qBittorrent/4.3.9")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	decoded, err := bencode.DecodeBytes(w.Body.Bytes())
	if err != nil {
		t.Fatalf("Failed to decode announce response: %v", err)
	}
	dict := decoded.(map[string]any)
	if dict["complete"] != int64(5) {
		t.Fatalf("Expected complete 5, got %v", dict["complete"])
	}

	// Test Scrape
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/tracker/scrape?info_hash="+infoHash20, nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected 200 for scrape, got %d: %s", w2.Code, w2.Body.String())
	}
	scrapeDecoded, err := bencode.DecodeBytes(w2.Body.Bytes())
	if err != nil {
		t.Fatalf("Failed to decode scrape response: %v", err)
	}
	scrapeDict := scrapeDecoded.(map[string]any)
	files := scrapeDict["files"].(map[string]any)
	if files[infoHash20] == nil {
		t.Fatalf("Expected infoHash in files, got %v", files)
	}
}
