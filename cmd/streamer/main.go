package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/sirupsen/logrus"
)

type StreamBridge struct {
	mu       sync.Mutex
	client   *torrent.Client
	torrents map[uint64]*torrent.Torrent
}

func newStreamBridge() (*StreamBridge, error) {
	cfg := torrent.NewDefaultClientConfig()
	cacheDir := filepath.Join(os.TempDir(), "paopao_torrent_stream_cache")
	_ = os.MkdirAll(cacheDir, 0755)
	cfg.DataDir = cacheDir
	cfg.NoDHT = true
	cfg.DisableUTP = false
	// Whitelisted BitTorrent client prefix for Private Trackers
	cfg.PeerID = "-qB5230-paopaostrm01"
	cfg.HTTPUserAgent = "qBittorrent/5.2.3"

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &StreamBridge{
		client:   client,
		torrents: make(map[uint64]*torrent.Torrent),
	}, nil
}

func (s *StreamBridge) getTorrent(torrentID uint64, torrentFilePath, announceURL string) (*torrent.Torrent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.torrents[torrentID]; ok {
		return t, nil
	}

	mi, err := metainfo.LoadFromFile(torrentFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load torrent file: %w", err)
	}

	t, err := s.client.AddTorrent(mi)
	if err != nil {
		return nil, fmt.Errorf("failed to add torrent: %w", err)
	}

	if announceURL != "" {
		t.AddTrackers([][]string{{announceURL}})
	}

	s.torrents[torrentID] = t
	return t, nil
}

func (s *StreamBridge) handleStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Accept-Ranges")
	w.Header().Set("Accept-Ranges", "bytes")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	idStr := r.URL.Query().Get("id")
	torrentID, _ := strconv.ParseUint(idStr, 10, 64)
	filePath := r.URL.Query().Get("file_path")
	announceURL := r.URL.Query().Get("announce")

	if torrentID == 0 || filePath == "" {
		http.Error(w, "Missing id or file_path parameter", http.StatusBadRequest)
		return
	}

	t, err := s.getTorrent(torrentID, filePath, announceURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for metadata
	select {
	case <-t.GotInfo():
	case <-time.After(15 * time.Second):
		http.Error(w, "Timeout waiting for torrent metadata", http.StatusGatewayTimeout)
		return
	}

	files := t.Files()
	if len(files) == 0 {
		http.Error(w, "No files found in torrent", http.StatusNotFound)
		return
	}

	var selectedFile *torrent.File
	var largestFile *torrent.File
	for _, f := range files {
		if largestFile == nil || f.Length() > largestFile.Length() {
			largestFile = f
		}
		lower := strings.ToLower(f.Path())
		if strings.HasSuffix(lower, ".mp4") ||
			strings.HasSuffix(lower, ".mkv") ||
			strings.HasSuffix(lower, ".webm") ||
			strings.HasSuffix(lower, ".mov") ||
			strings.HasSuffix(lower, ".mp3") {
			selectedFile = f
			break
		}
	}
	if selectedFile == nil {
		selectedFile = largestFile
	}

	selectedFile.Download()

	reader := selectedFile.NewReader()
	defer reader.Close()
	reader.SetReadahead(3 * 1024 * 1024)

	http.ServeContent(w, r, filepath.Base(selectedFile.Path()), time.Now(), reader)
}

func main() {
	bridge, err := newStreamBridge()
	if err != nil {
		logrus.Fatalf("Failed to initialize stream bridge: %v", err)
	}

	http.HandleFunc("/stream", bridge.handleStream)

	addr := "127.0.0.1:8009"
	logrus.Infof("PaoPao Torrent TCP-to-HTTP Stream Bridge listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logrus.Fatalf("Stream bridge server error: %v", err)
	}
}
