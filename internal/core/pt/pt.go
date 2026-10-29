package pt

import (
	"github.com/rocboss/paopao-ce/internal/model/pt"
)

// ListTorrentsReq represents query parameters for listing torrents.
type ListTorrentsReq struct {
	Keyword    string `json:"keyword"`
	CategoryID int    `json:"category_id"`
	SpState    int    `json:"sp_state"`
	SortBy     string `json:"sort_by"`    // "id", "seeders", "size", "times_completed"
	SortOrder  string `json:"sort_order"` // "asc", "desc"
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// ListTorrentsResp represents paginated torrent results.
type ListTorrentsResp struct {
	Total int64         `json:"total"`
	List  []*pt.Torrent `json:"list"`
}

// UploadTorrentReq represents request payload for uploading a new torrent.
type UploadTorrentReq struct {
	Name        string `json:"name"`
	SmallDescr  string `json:"small_descr"`
	Descr       string `json:"descr"`
	CategoryID  int    `json:"category_id"`
	SpState     int    `json:"sp_state"`
	IsAnonymous bool   `json:"is_anonymous"`
	Nfo         string `json:"nfo"`
}

// AnnounceReq represents BitTorrent announce parameters.
type AnnounceReq struct {
	Passkey    string `json:"passkey"`
	InfoHash   []byte `json:"info_hash"` // 20 bytes binary
	PeerID     []byte `json:"peer_id"`   // 20 bytes binary
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Uploaded   uint64 `json:"uploaded"`
	Downloaded uint64 `json:"downloaded"`
	Left       uint64 `json:"left"`
	Event      string `json:"event"` // "started", "stopped", "completed", ""
	NumWant    int    `json:"numwant"`
	Compact    bool   `json:"compact"`
	NoPeerID   bool   `json:"no_peer_id"`
	Agent      string `json:"agent"`
}

// AnnouncePeer represents a peer returned to client.
type AnnouncePeer struct {
	PeerID string `json:"peer_id"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
}

// AnnounceResp represents response sent back to the BitTorrent client.
type AnnounceResp struct {
	Interval    int            `json:"interval"`
	MinInterval int            `json:"min_interval"`
	Complete    int            `json:"complete"`
	Incomplete  int            `json:"incomplete"`
	Peers       []AnnouncePeer `json:"peers,omitempty"`
	PeersBinary []byte         `json:"-"`
}

// ScrapeReq represents query for scrape info.
type ScrapeReq struct {
	InfoHashes [][]byte `json:"info_hashes"`
}

// ScrapeFile represents stats for a single info_hash.
type ScrapeFile struct {
	InfoHash   [20]byte
	Complete   int
	Downloaded int
	Incomplete int
}

// ScrapeResp represents scrape response.
type ScrapeResp struct {
	Files []ScrapeFile
}

// AdminPTUserItem represents user summary for the PT admin panel.
type AdminPTUserItem struct {
	UserID      int64   `json:"user_id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Avatar      string  `json:"avatar"`
	IsAdmin     bool    `json:"is_admin"`
	Class       int     `json:"class"`
	ClassName   string  `json:"class_name"`
	ClassColor  string  `json:"class_color"`
	CustomTitle string  `json:"custom_title"`
	VipUntil    int64   `json:"vip_until"`
	Uploaded    uint64  `json:"uploaded"`
	Downloaded  uint64  `json:"downloaded"`
	Ratio       float64 `json:"ratio"`
	SeedBonus   float64 `json:"seed_bonus"`
	CanLeech    bool    `json:"can_leech"`
	CanUpload   bool    `json:"can_upload"`
	CreatedOn   int64   `json:"created_on"`
}

// AdminUpdateUserReq represents parameters to modify a user's PT permissions and class.
type AdminUpdateUserReq struct {
	UserID      int64  `json:"user_id"`
	Class       int    `json:"class"`
	CustomTitle string `json:"custom_title"`
	VipDays     int    `json:"vip_days"`
	CanUpload   *bool  `json:"can_upload"`
	CanLeech    *bool  `json:"can_leech"`
}

// PTService provides full Private Tracker capabilities.
type PTService interface {
	// Torrents Management
	GetTorrent(id uint64) (*pt.Torrent, error)
	GetTorrentByInfoHash(infoHashHex string) (*pt.Torrent, error)
	ListTorrents(req *ListTorrentsReq) (*ListTorrentsResp, error)
	CreateTorrent(userID int64, req *UploadTorrentReq, rawTorrent []byte) (*pt.Torrent, error)
	UpdateTorrent(t *pt.Torrent) error
	DeleteTorrent(id uint64, userID int64, isAdmin bool) error
	GetTorrentFiles(torrentID uint64) ([]pt.TorrentFile, error)
	BindTorrentPost(torrentID uint64, postID int64) error
	GetTorrentsByPostIDs(postIDs []int64) (map[int64]*pt.Torrent, error)

	// Personalized Torrent Download
	GenerateUserTorrent(torrentID uint64, userID int64) ([]byte, string, error)

	// User PT Profile
	GetUserPTProfile(userID int64) (*pt.UserPTProfile, error)
	GetUserByPasskey(passkey string) (*pt.UserPTProfile, error)
	ResetPasskey(userID int64) (string, error)
	GetUserSnatches(userID int64, page, pageSize int) ([]pt.TorrentSnatched, int64, error)

	// Bonus / Karma Shop
	GetBonusOptions() []pt.PTBonusOption
	ExchangeBonus(userID int64, optionID int) error
	CalculateHourlyKarma() error

	// Authority & Promotion Engine
	RunPromotionAndDemotion() error
	UpdateUserClass(userID int64, targetClass int, customTitle string) error
	AdminListUsers(keyword string, classFilter int, page, pageSize int) ([]*AdminPTUserItem, int64, error)
	AdminUpdateUser(req *AdminUpdateUserReq) error
	GetAuthorityRules() *pt.PTAuthorityRules
	SaveAuthorityRules(rules *pt.PTAuthorityRules) error

	// Invites
	GetMyInvites(userID int64) ([]*pt.PTInvite, int, error)
	GenerateInvite(userID int64) (*pt.PTInvite, error)
	CheckInvite(code string) (*pt.PTInvite, error)
	ConsumeInvite(code string, invitedUID int64) error

	// Categories
	ListCategories() ([]pt.PTCategory, error)

	// Tracker Engine
	HandleAnnounce(req *AnnounceReq) (*AnnounceResp, error)
	HandleScrape(req *ScrapeReq) (*ScrapeResp, error)
	GetTorrentPeers(torrentID uint64, limit int) ([]pt.TorrentPeer, error)
}
