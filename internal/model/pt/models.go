// Package pt defines data models, constants and transfer types for the PT module.
package pt

import (
	"gorm.io/gorm"
)

// SpState (Special Promotion State) constants aligned with NexusPHP
const (
	SpStateNormal        = 1 // Normal: 1x Upload, 1x Download
	SpStateFree          = 2 // Free: 1x Upload, 0x Download
	SpState2X            = 3 // 2X: 2x Upload, 1x Download
	SpState2XFree        = 4 // 2X Free: 2x Upload, 0x Download
	SpStateHalfDown      = 5 // 50%: 1x Upload, 0.5x Download
	SpState2XHalfDown    = 6 // 2X 50%: 2x Upload, 0.5x Download
	SpStateThirtyPercent = 7 // 30%: 1x Upload, 0.3x Download
)

// User Class constants aligned with NexusPHP
const (
	ClassPeasant       = 0  // UC_PEASANT: Warning / Restricted
	ClassUser          = 1  // UC_USER: Default Member
	ClassPowerUser     = 2  // UC_POWER_USER: Advanced User
	ClassEliteUser     = 3  // UC_ELITE_USER: Elite Member
	ClassCrazyUser     = 4  // UC_CRAZY_USER: Crazy Seeder
	ClassInsaneUser    = 5  // UC_INSANE_USER: Insane Seeder
	ClassVeteranUser   = 6  // UC_VETERAN_USER: Veteran Member
	ClassExtremeUser   = 7  // UC_EXTREME_USER: Extreme Contributor
	ClassUltimateUser  = 8  // UC_ULTIMATE_USER: Ultimate Member
	ClassNexusMaster   = 9  // UC_NEXUS_MASTER: Nexus Master (Top Member)
	ClassVIP           = 10 // UC_VIP: VIP (Immune to Demotion & Free Down)
	ClassRetiree       = 11 // UC_RETIREE: Retired Staff / Senior
	ClassUploader      = 12 // UC_UPLOADER: Official Uploader
	ClassModerator     = 13 // UC_MODERATOR: Forum & Torrent Moderator
	ClassAdministrator = 14 // UC_ADMINISTRATOR: Site Administrator
	ClassSysOp         = 15 // UC_SYSOP: System Operator / Technical Director
	ClassStaffLeader   = 16 // UC_STAFFLEADER: Site Owner / Staff Leader
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
	UserID       int64   `gorm:"primaryKey" json:"user_id"`
	Passkey      string  `gorm:"size:32;uniqueIndex;not null" json:"passkey"`
	Uploaded     uint64  `gorm:"not null;default:0" json:"uploaded"`
	Downloaded   uint64  `gorm:"not null;default:0" json:"downloaded"`
	SeedBonus    float64 `gorm:"not null;default:0.0" json:"seed_bonus"` // Karma points
	SeedTime     int64   `gorm:"not null;default:0" json:"seed_time"`
	LeechTime    int64   `gorm:"not null;default:0" json:"leech_time"`
	CanLeech     bool    `gorm:"not null;default:true" json:"can_leech"`
	CanUpload    bool    `gorm:"not null;default:true" json:"can_upload"`
	Invites      int     `gorm:"not null;default:0" json:"invites"`
	Class        int     `gorm:"not null;default:1" json:"class"`
	MaxClassOnce int     `gorm:"not null;default:1" json:"max_class_once"`
	VipUntil     int64   `gorm:"not null;default:0" json:"vip_until"`
	OldClass     int     `gorm:"not null;default:1" json:"old_class"`
	CustomTitle  string  `gorm:"size:50;not null;default:''" json:"custom_title"`
	CreatedAt    int64   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    int64   `gorm:"autoUpdateTime" json:"updated_at"`

	// Transient computed fields
	Ratio          float64            `gorm:"-" json:"ratio"`
	SeedingCount   int                `gorm:"-" json:"seeding_count"`
	LeechCount     int                `gorm:"-" json:"leech_count"`
	HttpTrackerURL string             `gorm:"-" json:"http_tracker_url"`
	WsTrackerURL   string             `gorm:"-" json:"ws_tracker_url"`
	ClassName       string             `gorm:"-" json:"class_name"`
	ClassColor      string             `gorm:"-" json:"class_color"`
	NextClass       *NextClassProgress `gorm:"-" json:"next_class,omitempty"`
	Privileges         []string           `gorm:"-" json:"privileges,omitempty"`
	KarmaMultiplier    float64            `gorm:"-" json:"karma_multiplier"`
	CanCustomPromotion bool               `gorm:"-" json:"can_custom_promotion"`
	CanAnonymous       bool               `gorm:"-" json:"can_anonymous"`
	CanDownload        bool               `gorm:"-" json:"can_download"`
}

// NextClassProgress details requirements and current metrics for upgrading to the next user class.
type NextClassProgress struct {
	NextClass     int     `json:"next_class"`
	NextClassName string  `json:"next_class_name"`
	ReqWeeks      int     `json:"req_weeks"`
	CurWeeks      int     `json:"cur_weeks"`
	ReqUploadGB   float64 `json:"req_upload_gb"`
	CurUploadGB   float64 `json:"cur_upload_gb"`
	ReqDownloadGB float64 `json:"req_download_gb"`
	CurDownloadGB float64 `json:"cur_download_gb"`
	ReqRatio      float64 `json:"req_ratio"`
	CurRatio      float64 `json:"cur_ratio"`
	ReqBonus      float64 `json:"req_bonus"`
	CurBonus      float64 `json:"cur_bonus"`
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

// PTSettingRecord represents persistent key-value configuration in database.
type PTSettingRecord struct {
	Key       string `gorm:"primaryKey;size:100" json:"key"`
	Value     string `gorm:"type:text" json:"value"`
	UpdatedAt int64  `gorm:"autoUpdateTime" json:"updated_at"`
}

const (
	InviteStatusUnused  = 0
	InviteStatusUsed    = 1
	InviteStatusExpired = 2
)

// PTInvite represents an invitation code issued by a user.
type PTInvite struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	InviterID   int64  `gorm:"index;not null" json:"inviter_id"`
	InviterName string `gorm:"<-:false;-:migration" json:"inviter_name,omitempty"`
	Code        string `gorm:"uniqueIndex;size:32;not null" json:"code"`
	Status      int    `gorm:"not null;default:0" json:"status"` // 0: 未使用, 1: 已使用, 2: 已过期
	InvitedUID  int64  `gorm:"index;default:0" json:"invited_uid"`
	InvitedName string `gorm:"<-:false;-:migration" json:"invited_name,omitempty"`
	CreatedAt   int64  `gorm:"autoCreateTime" json:"created_at"`
	ExpireAt    int64  `gorm:"not null" json:"expire_at"`
	UsedAt      int64  `gorm:"default:0" json:"used_at"`
}

// PromotionRule defines requirement thresholds and invite reward for automatic class promotion.
type PromotionRule struct {
	Class         int     `json:"class"`
	ClassName     string  `json:"class_name,omitempty"`
	MinWeeks      int     `json:"min_weeks"`       // 最低注册周数
	MinDownloadGB float64 `json:"min_download_gb"` // 最低下载量 (GB)
	MinRatio      float64 `json:"min_ratio"`       // 晋升要求最低分享率
	DemoteRatio   float64 `json:"demote_ratio"`    // 降级触发分享率 (低于此值将降级)
	MinBonus      float64 `json:"min_bonus"`       // 最低做种魔力值 (Karma) 要求
	AwardInvites  int     `json:"award_invites"`   // 首次晋升赠送邀请名额
}

// PTAuthorityRules defines dynamic behavioral gates and special group privilege options.
type PTAuthorityRules struct {
	MinClassDownload    int             `json:"min_class_download"`     // 最低下载种子等级 (default: 1)
	MinClassUpload      int             `json:"min_class_upload"`       // 最低发种等级 (default: 1)
	MinClassPromotion   int             `json:"min_class_promotion"`    // 最低自定义促销等级 (default: 12 Uploader)
	MinClassAnonymous   int             `json:"min_class_anonymous"`    // 最低匿名发种等级 (default: 4 Crazy User)
	MinClassViewPeers   int             `json:"min_class_view_peers"`   // 最低查看 Swarm Peers 详细名单等级 (default: 2)
	RetireeFreeUpload   bool            `json:"retiree_free_upload"`    // 养老退役免上传 (0x下载计费) (default: true)
	RetireeExemptDemote bool            `json:"retiree_exempt_demote"`  // 养老退役终生免考核降级 (default: true)
	UploaderKarmaMulti  float64         `json:"uploader_karma_multi"`   // 发布员魔力加成倍率 (default: 1.5)
	VipKarmaMulti       float64         `json:"vip_karma_multi"`        // VIP 魔力加成倍率 (default: 1.2)
	VeteranKarmaMulti   float64         `json:"veteran_karma_multi"`    // 骨灰用户魔力加成 (default: 1.1)
	UltimateKarmaMulti  float64         `json:"ultimate_karma_multi"`   // 巅峰用户魔力加成 (default: 1.2)
	MasterKarmaMulti    float64         `json:"master_karma_multi"`     // 大师用户魔力加成 (default: 1.3)
	PromotionRules      []PromotionRule `json:"promotion_rules"`        // 各等级晋升与降级指标阶梯
}
