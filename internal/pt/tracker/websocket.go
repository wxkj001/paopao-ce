package tracker

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origin browser WebRTC clients
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// WSClientMessage represents an incoming message from a WebTorrent client.
type WSClientMessage struct {
	Action     string          `json:"action"` // "announce" or "scrape"
	InfoHash   string          `json:"info_hash"`
	PeerID     string          `json:"peer_id"`
	Uploaded   uint64          `json:"uploaded"`
	Downloaded uint64          `json:"downloaded"`
	Left       uint64          `json:"left"`
	Event      string          `json:"event"`
	NumWant    int             `json:"numwant"`
	Offers     []WSOfferItem   `json:"offers"`
	Answer     json.RawMessage `json:"answer"`
	ToPeerID   string          `json:"to_peer_id"`
	OfferID    string          `json:"offer_id"`
	Files      []string        `json:"files"`
}

type WSOfferItem struct {
	OfferID string          `json:"offer_id"`
	Offer   json.RawMessage `json:"offer"`
}

// WSServerMessage represents a message sent to a WebTorrent client.
type WSServerMessage struct {
	Action        string                  `json:"action"`
	InfoHash      string                  `json:"info_hash,omitempty"`
	PeerID        string                  `json:"peer_id,omitempty"`
	OfferID       string                  `json:"offer_id,omitempty"`
	Offer         json.RawMessage         `json:"offer,omitempty"`
	Answer        json.RawMessage         `json:"answer,omitempty"`
	Interval      int                     `json:"interval,omitempty"`
	Complete      int                     `json:"complete,omitempty"`
	Incomplete    int                     `json:"incomplete,omitempty"`
	FailureReason string                  `json:"failure reason,omitempty"`
	Files         map[string]WSScrapeFile `json:"files,omitempty"`
}

type WSScrapeFile struct {
	Complete   int `json:"complete"`
	Incomplete int `json:"incomplete"`
	Downloaded int `json:"downloaded"`
}

// WSPeerSession represents an active WebSocket connection from a browser peer.
type WSPeerSession struct {
	PeerID      string // raw string
	PeerIDHex   string
	InfoHash    string // raw string
	InfoHashHex string
	Passkey     string
	Conn        *websocket.Conn
	SendChan    chan []byte
	DoneChan    chan struct{}
	closeOnce   sync.Once
	IsSeeder    bool
}

func (s *WSPeerSession) Send(data []byte) bool {
	select {
	case <-s.DoneChan:
		return false
	case s.SendChan <- data:
		return true
	default:
		// Queue full, drop or close
		return false
	}
}

func (s *WSPeerSession) Close() {
	s.closeOnce.Do(func() {
		close(s.DoneChan)
		s.Conn.Close()
	})
}

// WSHub manages active WebTorrent WebSocket swarms and handles WebRTC SDP relaying.
type WSHub struct {
	sync.RWMutex
	ptSrv  corept.PTService
	swarms map[string]map[string]*WSPeerSession // infoHashHex -> peerIDHex -> session
}

func NewWSHub(ptSrv corept.PTService) *WSHub {
	return &WSHub{
		ptSrv:  ptSrv,
		swarms: make(map[string]map[string]*WSPeerSession),
	}
}

func (h *WSHub) RegisterPeer(infoHashHex string, session *WSPeerSession) {
	h.Lock()
	defer h.Unlock()

	swarm, ok := h.swarms[infoHashHex]
	if !ok {
		swarm = make(map[string]*WSPeerSession)
		h.swarms[infoHashHex] = swarm
	}
	swarm[session.PeerIDHex] = session
}

func (h *WSHub) UnregisterPeer(infoHashHex, peerIDHex string) {
	h.Lock()
	defer h.Unlock()

	if swarm, ok := h.swarms[infoHashHex]; ok {
		delete(swarm, peerIDHex)
		if len(swarm) == 0 {
			delete(h.swarms, infoHashHex)
		}
	}
}

func (h *WSHub) GetOtherPeers(infoHashHex, excludePeerIDHex string, limit int) []*WSPeerSession {
	h.RLock()
	defer h.RUnlock()

	swarm, ok := h.swarms[infoHashHex]
	if !ok {
		return nil
	}

	res := make([]*WSPeerSession, 0, limit)
	for peerIDHex, peer := range swarm {
		if peerIDHex == excludePeerIDHex {
			continue
		}
		res = append(res, peer)
		if len(res) >= limit {
			break
		}
	}
	return res
}

func (h *WSHub) FindPeer(infoHashHex, peerIDHex string) *WSPeerSession {
	h.RLock()
	defer h.RUnlock()

	if swarm, ok := h.swarms[infoHashHex]; ok {
		return swarm[peerIDHex]
	}
	return nil
}

func (h *WSHub) SwarmCounts(infoHashHex string) (complete int, incomplete int) {
	h.RLock()
	defer h.RUnlock()

	if swarm, ok := h.swarms[infoHashHex]; ok {
		for _, peer := range swarm {
			if peer.IsSeeder {
				complete++
			} else {
				incomplete++
			}
		}
	}
	return complete, incomplete
}

// HandleWebSocket handles incoming WebTorrent WebSocket connections.
func (t *Tracker) HandleWebSocket(c *gin.Context) {
	passkey := c.Query("passkey")
	if passkey == "" {
		passkey = c.Query("pasakey")
	}
	if passkey == "" {
		c.String(http.StatusBadRequest, "Missing passkey in tracker websocket url")
		return
	}

	// Validate user passkey
	profile, err := t.ptSrv.GetUserByPasskey(passkey)
	if err != nil {
		c.String(http.StatusForbidden, "Invalid passkey")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("WebTorrent tracker upgrade error: %v", err)
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()
	if userAgent == "" {
		userAgent = "WebTorrent"
	}

	t.serveWebSocketClient(conn, passkey, profile.UserID, clientIP, userAgent)
}

func (t *Tracker) serveWebSocketClient(conn *websocket.Conn, passkey string, userID int64, clientIP, userAgent string) {
	session := &WSPeerSession{
		Passkey:  passkey,
		Conn:     conn,
		SendChan: make(chan []byte, 128),
		DoneChan: make(chan struct{}),
	}

	defer func() {
		session.Close()
		if session.InfoHashHex != "" && session.PeerIDHex != "" {
			t.wsHub.UnregisterPeer(session.InfoHashHex, session.PeerIDHex)
		}
	}()

	// Write pump goroutine
	go func() {
		for {
			select {
			case <-session.DoneChan:
				return
			case msg, ok := <-session.SendChan:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}
	}()

	conn.SetReadLimit(65536)
	conn.SetReadDeadline(time.Now().Add(180 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(180 * time.Second))
		return nil
	})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(180 * time.Second))

		var msg WSClientMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			sendWSError(session, "Invalid JSON message")
			continue
		}

		switch msg.Action {
		case "announce":
			t.handleWSAnnounce(session, &msg, clientIP, userAgent)
		case "scrape":
			t.handleWSScrape(session, &msg)
		default:
			sendWSError(session, fmt.Sprintf("Unknown action: %s", msg.Action), msg.InfoHash)
		}
	}
}

func (t *Tracker) handleWSAnnounce(session *WSPeerSession, msg *WSClientMessage, clientIP, userAgent string) {
	infoHashBytes, infoHashHex, err := parseInfoHash(msg.InfoHash)
	if err != nil {
		sendWSError(session, "Invalid info_hash: "+err.Error(), msg.InfoHash)
		return
	}

	peerIDBytes, peerIDHex, err := parsePeerID(msg.PeerID)
	if err != nil {
		sendWSError(session, "Invalid peer_id: "+err.Error(), msg.InfoHash)
		return
	}

	session.InfoHash = msg.InfoHash
	session.InfoHashHex = infoHashHex
	session.PeerID = msg.PeerID
	session.PeerIDHex = peerIDHex
	session.IsSeeder = (msg.Left == 0)

	// Register peer in the in-memory WebTorrent swarm
	t.wsHub.RegisterPeer(infoHashHex, session)

	// 1. If this message is an Answer to a previous Offer
	if len(msg.Answer) > 0 && msg.ToPeerID != "" {
		_, toPeerIDHex, err := parsePeerID(msg.ToPeerID)
		if err == nil {
			targetPeer := t.wsHub.FindPeer(infoHashHex, toPeerIDHex)
			if targetPeer != nil {
				forwardAnswer := WSServerMessage{
					Action:   "announce",
					InfoHash: msg.InfoHash,
					PeerID:   msg.PeerID, // from
					OfferID:  msg.OfferID,
					Answer:   msg.Answer,
				}
				if data, err := json.Marshal(forwardAnswer); err == nil {
					targetPeer.Send(data)
				}
			}
		}
	}

	// 2. If this message contains WebRTC SDP Offers, forward them to peers in the swarm
	if len(msg.Offers) > 0 {
		numWant := msg.NumWant
		if numWant <= 0 || numWant > len(msg.Offers) {
			numWant = len(msg.Offers)
		}
		otherPeers := t.wsHub.GetOtherPeers(infoHashHex, peerIDHex, numWant)
		for i, otherPeer := range otherPeers {
			if i >= len(msg.Offers) {
				break
			}
			offer := msg.Offers[i]
			forwardOffer := WSServerMessage{
				Action:   "announce",
				InfoHash: msg.InfoHash,
				PeerID:   msg.PeerID, // from
				OfferID:  offer.OfferID,
				Offer:    offer.Offer,
			}
			if data, err := json.Marshal(forwardOffer); err == nil {
				otherPeer.Send(data)
			}
		}
	}

	// 3. Update database peer statistics via ptService
	req := &corept.AnnounceReq{
		InfoHash:   infoHashBytes,
		PeerID:     peerIDBytes,
		Passkey:    session.Passkey,
		Port:       0, // WebTorrent uses WebRTC DataChannel (port 0)
		Uploaded:   msg.Uploaded,
		Downloaded: msg.Downloaded,
		Left:       msg.Left,
		Event:      msg.Event,
		IP:         clientIP,
		Agent:      userAgent,
	}

	var dbResp *corept.AnnounceResp
	dbResp, err = t.ptSrv.HandleAnnounce(req)
	if err != nil {
		sendWSError(session, err.Error(), msg.InfoHash)
		return
	}

	complete, incomplete := t.wsHub.SwarmCounts(infoHashHex)
	if dbResp != nil {
		if dbResp.Complete > complete {
			complete = dbResp.Complete
		}
		if dbResp.Incomplete > incomplete {
			incomplete = dbResp.Incomplete
		}
	}

	// 4. Send standard announce response back to the client
	respMsg := WSServerMessage{
		Action:     "announce",
		InfoHash:   msg.InfoHash,
		Interval:   120,
		Complete:   complete,
		Incomplete: incomplete,
	}
	if data, err := json.Marshal(respMsg); err == nil {
		session.Send(data)
	}
}

func (t *Tracker) handleWSScrape(session *WSPeerSession, msg *WSClientMessage) {
	filesMap := make(map[string]WSScrapeFile)

	for _, rawHash := range msg.Files {
		_, infoHashHex, err := parseInfoHash(rawHash)
		if err != nil {
			continue
		}
		complete, incomplete := t.wsHub.SwarmCounts(infoHashHex)
		filesMap[rawHash] = WSScrapeFile{
			Complete:   complete,
			Incomplete: incomplete,
			Downloaded: 0,
		}
	}

	respMsg := WSServerMessage{
		Action: "scrape",
		Files:  filesMap,
	}
	if data, err := json.Marshal(respMsg); err == nil {
		session.Send(data)
	}
}

func sendWSError(session *WSPeerSession, reason string, infoHash ...string) {
	var hash string
	if len(infoHash) > 0 && infoHash[0] != "" {
		hash = infoHash[0]
	} else if session != nil {
		hash = session.InfoHash
	}
	msg := WSServerMessage{
		Action:        "announce",
		InfoHash:      hash,
		FailureReason: reason,
	}
	if data, err := json.Marshal(msg); err == nil {
		session.Send(data)
	}
}

// parseInfoHash parses a 20-character binary string (codepoints 0-255), 20-byte slice or 40-character hex string.
func parseInfoHash(s string) ([]byte, string, error) {
	if len(s) == 40 {
		decoded, err := hex.DecodeString(s)
		if err == nil && len(decoded) == 20 {
			return decoded, strings.ToLower(s), nil
		}
	}
	runes := []rune(s)
	if len(runes) == 20 {
		b := make([]byte, 20)
		isBinary := true
		for i, r := range runes {
			if r > 255 {
				isBinary = false
				break
			}
			b[i] = byte(r)
		}
		if isBinary {
			return b, hex.EncodeToString(b), nil
		}
	}
	b := []byte(s)
	if len(b) == 20 {
		return b, hex.EncodeToString(b), nil
	}
	if len(b) > 0 {
		h := sha1.Sum(b)
		return h[:], hex.EncodeToString(h[:]), nil
	}
	return nil, "", errors.New("empty info_hash")
}

// parsePeerID parses a 20-character binary string (codepoints 0-255), 20-byte slice or 40-character hex string.
func parsePeerID(s string) ([]byte, string, error) {
	if len(s) == 40 {
		decoded, err := hex.DecodeString(s)
		if err == nil && len(decoded) == 20 {
			return decoded, strings.ToLower(s), nil
		}
	}
	runes := []rune(s)
	if len(runes) == 20 {
		b := make([]byte, 20)
		isBinary := true
		for i, r := range runes {
			if r > 255 {
				isBinary = false
				break
			}
			b[i] = byte(r)
		}
		if isBinary {
			return b, hex.EncodeToString(b), nil
		}
	}
	b := []byte(s)
	if len(b) == 20 {
		return b, hex.EncodeToString(b), nil
	}
	if len(b) > 0 {
		h := sha1.Sum(b)
		return h[:], hex.EncodeToString(h[:]), nil
	}
	return nil, "", errors.New("empty peer_id")
}
