// Package torrent handles parsing, sanitizing and customizing .torrent files for Private Trackers.
package torrent

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/rocboss/paopao-ce/pkg/bencode"
)

var (
	ErrInvalidTorrent = errors.New("invalid torrent data")
	ErrMissingInfo    = errors.New("missing 'info' dictionary in torrent")
)

// FileInfo represents a file in a multi-file torrent.
type FileInfo struct {
	Length int64  `json:"length"`
	Path   string `json:"path"`
}

// MetaInfo represents parsed torrent metadata.
type MetaInfo struct {
	Announce     string     `json:"announce"`
	Comment      string     `json:"comment,omitempty"`
	CreatedBy    string     `json:"created_by,omitempty"`
	CreationDate int64      `json:"creation_date,omitempty"`
	Name         string     `json:"name"`
	PieceLength  int64      `json:"piece_length"`
	NumPieces    int        `json:"num_pieces"`
	TotalLength  int64      `json:"total_length"`
	Files        []FileInfo `json:"files"`
	InfoHash     [20]byte   `json:"info_hash"`
	InfoHashHex  string     `json:"info_hash_hex"`
	IsPrivate    bool       `json:"is_private"`
	RawDict      map[string]any `json:"-"`
}

// Parse parses a torrent from reader.
func Parse(r io.Reader) (*MetaInfo, error) {
	val, err := bencode.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTorrent, err)
	}

	dict, ok := val.(map[string]any)
	if !ok {
		return nil, ErrInvalidTorrent
	}

	infoRaw, ok := dict["info"]
	if !ok {
		return nil, ErrMissingInfo
	}
	infoDict, ok := infoRaw.(map[string]any)
	if !ok {
		return nil, ErrInvalidTorrent
	}

	// Calculate InfoHash
	infoEncoded, err := bencode.EncodeBytes(infoDict)
	if err != nil {
		return nil, fmt.Errorf("failed to encode info dict: %w", err)
	}
	infoHash := sha1.Sum(infoEncoded)

	meta := &MetaInfo{
		InfoHash:    infoHash,
		InfoHashHex: hex.EncodeToString(infoHash[:]),
		RawDict:     dict,
	}

	if ann, ok := dict["announce"].(string); ok {
		meta.Announce = ann
	}
	if comment, ok := dict["comment"].(string); ok {
		meta.Comment = comment
	}
	if createdBy, ok := dict["created by"].(string); ok {
		meta.CreatedBy = createdBy
	}
	if creationDate, ok := dict["creation date"].(int64); ok {
		meta.CreationDate = creationDate
	}

	if name, ok := infoDict["name"].(string); ok {
		meta.Name = name
	}
	if pieceLength, ok := infoDict["piece length"].(int64); ok {
		meta.PieceLength = pieceLength
	}
	if pieces, ok := infoDict["pieces"].(string); ok {
		meta.NumPieces = len(pieces) / 20
	}
	if priv, ok := infoDict["private"].(int64); ok && priv == 1 {
		meta.IsPrivate = true
	}

	// File structure: single file or multiple files
	if length, ok := infoDict["length"].(int64); ok {
		meta.TotalLength = length
		meta.Files = []FileInfo{
			{Length: length, Path: meta.Name},
		}
	} else if filesRaw, ok := infoDict["files"].([]any); ok {
		var total int64
		for _, f := range filesRaw {
			fDict, ok := f.(map[string]any)
			if !ok {
				continue
			}
			fLen, _ := fDict["length"].(int64)
			total += fLen

			var pathParts []string
			if pathRaw, ok := fDict["path"].([]any); ok {
				for _, p := range pathRaw {
					if pStr, ok := p.(string); ok {
						pathParts = append(pathParts, pStr)
					}
				}
			}
			meta.Files = append(meta.Files, FileInfo{
				Length: fLen,
				Path:   strings.Join(pathParts, "/"),
			})
		}
		meta.TotalLength = total
	}

	return meta, nil
}

// SanitizeForPT enforces Private Tracker constraints on a torrent:
// - sets info.private = 1
// - sets info.source = siteSource (e.g. "[paopao] PaoPao PT")
// - sets announce URL to default site announce URL
// - removes external DHT nodes and announce-list
// Returns sanitized torrent bytes and re-calculated info hash.
func SanitizeForPT(torrentData []byte, defaultAnnounceURL, siteSource string) ([]byte, [20]byte, error) {
	val, err := bencode.DecodeBytes(torrentData)
	if err != nil {
		return nil, [20]byte{}, err
	}
	dict, ok := val.(map[string]any)
	if !ok {
		return nil, [20]byte{}, ErrInvalidTorrent
	}

	infoDict, ok := dict["info"].(map[string]any)
	if !ok {
		return nil, [20]byte{}, ErrMissingInfo
	}

	// Enforce PT constraints
	infoDict["private"] = int64(1)
	if siteSource != "" {
		infoDict["source"] = siteSource
	}

	delete(dict, "announce-list")
	delete(dict, "nodes")
	dict["announce"] = defaultAnnounceURL

	// Recompute InfoHash
	infoEncoded, err := bencode.EncodeBytes(infoDict)
	if err != nil {
		return nil, [20]byte{}, err
	}
	infoHash := sha1.Sum(infoEncoded)

	out, err := bencode.EncodeBytes(dict)
	if err != nil {
		return nil, [20]byte{}, err
	}

	return out, infoHash, nil
}

// CustomizeForUser injects user's personal passkey into the announce URL and announce-list (HTTP + WebSocket) of the torrent.
func CustomizeForUser(torrentData []byte, userPasskey, announceBaseURL, wsAnnounceBaseURL string) ([]byte, error) {
	val, err := bencode.DecodeBytes(torrentData)
	if err != nil {
		return nil, err
	}
	dict, ok := val.(map[string]any)
	if !ok {
		return nil, ErrInvalidTorrent
	}

	httpAnnounce := formatAnnounceURL(announceBaseURL, userPasskey)
	dict["announce"] = httpAnnounce

	announceList := []any{
		[]any{httpAnnounce},
	}
	if wsAnnounceBaseURL != "" {
		wsAnnounce := formatAnnounceURL(wsAnnounceBaseURL, userPasskey)
		announceList = append(announceList, []any{wsAnnounce})
	}
	dict["announce-list"] = announceList

	delete(dict, "nodes")

	return bencode.EncodeBytes(dict)
}

func formatAnnounceURL(baseURL, passkey string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&passkey=%s", baseURL, passkey)
		}
		return fmt.Sprintf("%s?passkey=%s", baseURL, passkey)
	}
	q := u.Query()
	q.Set("passkey", passkey)
	u.RawQuery = q.Encode()
	return u.String()
}

// ParseBytes parses a torrent from a byte slice.
func ParseBytes(data []byte) (*MetaInfo, error) {
	return Parse(bytes.NewReader(data))
}
