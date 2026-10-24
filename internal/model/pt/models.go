// Package pt defines data models, constants and transfer types for the PT module.
package pt

import (
	"gorm.io/gorm"
)

// SpState (Special Promotion State) constants aligned with NexusPHP
const (
	SpStateNormal         = 1 // Normal: 1x Upload, 1x Download
	SpStateFree           = 2 // Free: 1x Upload, 0x Download
	SpState2X             = 3 // 2X: 2x Upload, 1x Download
	SpState2XFree         = 4 // 2X Free: 2x Upload, 0x Download
	SpStateHalfDown       = 5 // 50%: 1x Upload, 0.5x Download
	SpState2XHalfDown     = 6 // 2X 50%: 2x Upload, 0.5x Download
	SpStateThirtyPercent  = 7 // 30%: 1x Upload, 0.3x Download
)

// Torrent status constants
const (
	TorrentStatusNormal = 0
	TorrentStatusBanned = 1
	TorrentStatusAudit  = 2
)

// Torrent represents a registered BitTorrent item in the PT system.
type Torrent struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	PostID         int64          `gorm:"index;not null;default:0" json:"post_id"`
	InfoHash       string         `gorm:"size:40;uniqueIndex;not null" json:"info_hash"` // 40-character hex of 20-byte SHA1
	Name           string         `gorm:"size:255;not null;index" json:"name"`
	CleanName      string         `gorm:"size:255;not null" json:"clean_name"`
	SmallDescr     string         `gorm:"size:255" json:"small_descr"`
	Descr          string         `gorm:"type:text" json:"descr"`
	CategoryID     int            `gorm:"not null;default:1;index" json:"category_id"`
	Size           int64          `gorm:"not null;default:0" json:"size"`
	NumFiles       int            `gorm:"not null;default:1" json:"num_files"`
	UserID         int64          `gorm:"not null;index" json:"user_id"`
	SpState        int            `gorm:"not null;default:1" json:"sp_state"`
	PromotionUntil int64          `gorm:"not null;default:0" json:"promotion_until"` // 0 for permanent
	Seeders        int            `gorm:"not null;default:0" json:"seeders"`
	Leechers       int            `gorm:"not null;default:0" json:"leechers"`
	TimesCompleted int            `gorm:"not null;default:0" json:"times_completed"`
	Hits           int            `gorm:"not null;default:0" json:"hits"`
	IsAnonymous    bool           `gorm:"not null;default:false" json:"is_anonymous"`
	Status         int            `gorm:"not null;default:0" json:"status"`
	Nfo            string         `gorm:"type:text" json:"nfo,omitempty"`
	FilePath       string         `gorm:"size:255" json:"file_path"`
	CreatedAt      int64          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      int64          `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations & transient formatted fields
	Category *PTCategory   `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Files    []TorrentFile `gorm:"foreignKey:TorrentID" json:"files,omitempty"`
	Username string        `gorm:"-" json:"username,omitempty"`
	Nickname string        `gorm:"-" json:"nickname,omitempty"`
}

// TorrentFile stores the file breakdown within a torrent.
type TorrentFile struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	TorrentID uint64 `gorm:"not null;index" json:"torrent_id"`
	FilePath  string `gorm:"size:512;not null" json:"file_path"`
	Size      int64  `gorm:"not null;default:0" json:"size"`
}

// TorrentPeer records an active BitTorrent peer currently connected to the swarm.
type TorrentPeer struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	TorrentID  uint64 `gorm:"not null;index:idx_torrent_peer,unique" json:"torrent_id"`
	UserID     int64  `gorm:"not null;index" json:"user_id"`
	PeerID     string `gorm:"size:40;not null;index:idx_torrent_peer,unique" json:"peer_id"` // hex or raw string
	IP         string `gorm:"size:64;not null" json:"ip"`
	Port       int    `gorm:"not null;default:0" json:"port"`
	Uploaded   uint64 `gorm:"not null;default:0" json:"uploaded"`
	Downloaded uint64 `gorm:"not null;default:0" json:"downloaded"`
	ToGo       uint64 `gorm:"not null;default:0" json:"to_go"`
	IsSeeder   bool   `gorm:"not null;default:false;index" json:"is_seeder"`
	Agent      string `gorm:"size:100" json:"agent"`
	LastAction int64  `gorm:"not null;default:0;index" json:"last_action"`
	Started    int64  `gorm:"not null;default:0" json:"started"`
}

// TorrentSnatched records historical downloads and seed times for user accounting.
type TorrentSnatched struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	TorrentID   uint64 `gorm:"not null;uniqueIndex:idx_user_torrent" json:"torrent_id"`
	UserID      int64  `gorm:"not null;uniqueIndex:idx_user_torrent;index" json:"user_id"`
	Uploaded    uint64 `gorm:"not null;default:0" json:"uploaded"`
	Downloaded  uint64 `gorm:"not null;default:0" json:"downloaded"`
	ToGo        uint64 `gorm:"not null;default:0" json:"to_go"`
	SeedTime    int64  `gorm:"not null;default:0" json:"seed_time"` // in seconds
	LeechTime   int64  `gorm:"not null;default:0" json:"leech_time"`
	IsFinished  bool   `gorm:"not null;default:false" json:"is_finished"`
	CompletedAt int64  `gorm:"not null;default:0" json:"completed_at"`
	LastAction  int64  `gorm:"not null;default:0" json:"last_action"`
}

// UserPTProfile holds user's PT statistics, passkey and karma balance.
type UserPTProfile struct {
	UserID     int64   `gorm:"primaryKey" json:"user_id"`
	Passkey    string  `gorm:"size:32;uniqueIndex;not null" json:"passkey"`
	Uploaded   uint64  `gorm:"not null;default:0" json:"uploaded"`
	Downloaded uint64  `gorm:"not null;default:0" json:"downloaded"`
	SeedBonus  float64 `gorm:"not null;default:0.0" json:"seed_bonus"` // Karma points
	SeedTime   int64   `gorm:"not null;default:0" json:"seed_time"`
	LeechTime  int64   `gorm:"not null;default:0" json:"leech_time"`
	CanLeech   bool    `gorm:"not null;default:true" json:"can_leech"`
	CanUpload  bool    `gorm:"not null;default:true" json:"can_upload"`
	Invites    int     `gorm:"not null;default:0" json:"invites"`
	CreatedAt  int64   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  int64   `gorm:"autoUpdateTime" json:"updated_at"`

	// Transient computed fields
	Ratio          float64 `gorm:"-" json:"ratio"`
	SeedingCount   int     `gorm:"-" json:"seeding_count"`
	LeechCount     int     `gorm:"-" json:"leech_count"`
	HttpTrackerURL string  `gorm:"-" json:"http_tracker_url"`
	WsTrackerURL   string  `gorm:"-" json:"ws_tracker_url"`
}

// PTCategory represents a content category for torrents.
type PTCategory struct {
	ID   int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"size:50;not null" json:"name"`
	Icon string `gorm:"size:50" json:"icon"`
	Sort int    `gorm:"not null;default:0" json:"sort"`
}

// PTBonusLog records transactions in the Karma / Bonus shop.
type PTBonusLog struct {
	ID          uint64  `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64   `gorm:"not null;index" json:"user_id"`
	ActionType  string  `gorm:"size:50;not null" json:"action_type"`
	CostBonus   float64 `gorm:"not null;default:0.0" json:"cost_bonus"`
	Description string  `gorm:"size:255" json:"description"`
	CreatedAt   int64   `gorm:"autoCreateTime" json:"created_at"`
}

// PTBonusOption defines an item exchangeable in the Bonus shop.
type PTBonusOption struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Points      float64 `json:"points"`
	UploadBytes uint64  `json:"upload_bytes,omitempty"`
	Invites     int     `json:"invites,omitempty"`
	Description string  `json:"description"`
}
