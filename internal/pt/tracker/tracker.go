package tracker

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/rocboss/paopao-ce/pkg/bencode"
)

// Tracker handles HTTP Announce, Scrape, and WebTorrent WebSocket requests.
type Tracker struct {
	ptSrv corept.PTService
	wsHub *WSHub
}

// NewTracker creates a new Tracker controller.
func NewTracker(ptSrv corept.PTService) *Tracker {
	return &Tracker{
		ptSrv: ptSrv,
		wsHub: NewWSHub(ptSrv),
	}
}

// HandleAnnounce handles GET /tracker/announce.
func (t *Tracker) HandleAnnounce(c *gin.Context) {
	rawQuery := c.Request.URL.RawQuery
	params := parseRawQuery(rawQuery)

	passkey := string(params.getFirst("passkey"))
	if passkey == "" {
		// Tolerant fallback for common typo
		passkey = string(params.getFirst("pasakey"))
	}
	if passkey == "" {
		sendFailure(c, "Missing passkey in announce URL")
		return
	}

	infoHash := params.getFirst("info_hash")
	if len(infoHash) != 20 {
		sendFailure(c, fmt.Sprintf("Invalid info_hash (length: %d)", len(infoHash)))
		return
	}

	peerID := params.getFirst("peer_id")
	if len(peerID) != 20 {
		sendFailure(c, fmt.Sprintf("Invalid peer_id (length: %d)", len(peerID)))
		return
	}

	agent := c.Request.UserAgent()
	if checkErr := CheckClient(peerID, agent); checkErr != "" {
		sendFailure(c, checkErr)
		return
	}

	port, _ := strconv.Atoi(string(params.getFirst("port")))
	if port <= 0 || port > 65535 {
		sendFailure(c, "Invalid port")
		return
	}

	uploaded, _ := strconv.ParseUint(string(params.getFirst("uploaded")), 10, 64)
	downloaded, _ := strconv.ParseUint(string(params.getFirst("downloaded")), 10, 64)
	left, _ := strconv.ParseUint(string(params.getFirst("left")), 10, 64)
	event := string(params.getFirst("event"))
	numWant, _ := strconv.Atoi(string(params.getFirst("numwant")))
	compact := string(params.getFirst("compact")) == "1"
	noPeerID := string(params.getFirst("no_peer_id")) == "1"

	// Resolve client IP
	clientIP := getClientIP(c)

	req := &corept.AnnounceReq{
		Passkey:    passkey,
		InfoHash:   infoHash,
		PeerID:     peerID,
		IP:         clientIP,
		Port:       port,
		Uploaded:   uploaded,
		Downloaded: downloaded,
		Left:       left,
		Event:      event,
		NumWant:    numWant,
		Compact:    compact,
		NoPeerID:   noPeerID,
		Agent:      agent,
	}

	resp, err := t.ptSrv.HandleAnnounce(req)
	if err != nil {
		sendFailure(c, err.Error())
		return
	}

	// Build bencoded response
	respDict := map[string]any{
		"interval":     int64(resp.Interval),
		"min interval": int64(resp.MinInterval),
		"complete":     int64(resp.Complete),
		"incomplete":   int64(resp.Incomplete),
	}

	if compact || len(resp.PeersBinary) > 0 {
		respDict["peers"] = resp.PeersBinary
	} else {
		var peerList []any
		for _, p := range resp.Peers {
			pDict := map[string]any{
				"ip":   p.IP,
				"port": int64(p.Port),
			}
			if !noPeerID {
				pDict["peer id"] = p.PeerID
			}
			peerList = append(peerList, pDict)
		}
		respDict["peers"] = peerList
	}

	bencodeBytes, err := bencode.EncodeBytes(respDict)
	if err != nil {
		sendFailure(c, "Internal tracker encoding error")
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", bencodeBytes)
}

// HandleScrape handles GET /tracker/scrape.
func (t *Tracker) HandleScrape(c *gin.Context) {
	rawQuery := c.Request.URL.RawQuery
	params := parseRawQuery(rawQuery)

	infoHashes := params.getAll("info_hash")

	req := &corept.ScrapeReq{
		InfoHashes: infoHashes,
	}

	resp, err := t.ptSrv.HandleScrape(req)
	if err != nil {
		sendFailure(c, err.Error())
		return
	}

	filesDict := make(map[string]any)
	for _, f := range resp.Files {
		filesDict[string(f.InfoHash[:])] = map[string]any{
			"complete":   int64(f.Complete),
			"downloaded": int64(f.Downloaded),
			"incomplete": int64(f.Incomplete),
		}
	}

	scrapeDict := map[string]any{
		"files": filesDict,
	}

	bencodeBytes, err := bencode.EncodeBytes(scrapeDict)
	if err != nil {
		sendFailure(c, "Internal tracker encoding error")
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", bencodeBytes)
}

func sendFailure(c *gin.Context, reason string) {
	failDict := map[string]any{
		"failure reason": reason,
	}
	bytes, _ := bencode.EncodeBytes(failDict)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", bytes)
}

func getClientIP(c *gin.Context) string {
	// Gin's ClientIP takes X-Forwarded-For and X-Real-IP into account
	ip := c.ClientIP()
	if ip == "" || ip == "::1" {
		ip = "127.0.0.1"
	}
	// Strip port if present
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}

// queryParams stores binary raw values from URL query string
type queryParams struct {
	values map[string][][]byte
}

func (q *queryParams) getFirst(key string) []byte {
	if vals, ok := q.values[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return nil
}

func (q *queryParams) getAll(key string) [][]byte {
	return q.values[key]
}

// parseRawQuery correctly decodes raw percent-encoded binary bytes without UTF-8 corruption
func parseRawQuery(query string) *queryParams {
	res := &queryParams{values: make(map[string][][]byte)}
	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, "&")
		if key == "" {
			continue
		}
		key, val, _ := strings.Cut(key, "=")
		k := string(unescapeBinary(key))
		v := unescapeBinary(val)
		res.values[k] = append(res.values[k], v)
	}
	return res
}

func unescapeBinary(s string) []byte {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b := hexToByte(s[i+1])<<4 | hexToByte(s[i+2])
			buf.WriteByte(b)
			i += 2
		} else if s[i] == '+' {
			buf.WriteByte(' ')
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.Bytes()
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func hexToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
