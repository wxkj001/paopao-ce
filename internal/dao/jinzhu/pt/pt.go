package pt

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/rocboss/paopao-ce/internal/dao/jinzhu/dbr"
	"github.com/rocboss/paopao-ce/internal/model/pt"
	"github.com/rocboss/paopao-ce/internal/pt/authority"
	"github.com/rocboss/paopao-ce/pkg/torrent"
	"gorm.io/gorm"
)

var (
	ErrTorrentNotFound   = errors.New("torrent not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPasskey    = errors.New("invalid or missing passkey")
	ErrUserDisabled      = errors.New("user is not permitted to download or leech")
	ErrInsufficientBonus = errors.New("insufficient bonus points")
	ErrInvalidBonusItem  = errors.New("invalid bonus option")
	ErrPeasantRestricted = errors.New("peasant class is restricted from downloading non-free torrents")
	ErrDownloadPermissionDenied = errors.New("user class has no permission to download torrents")
)

type ptService struct {
	db                *gorm.DB
	storagePath       string
	announceBaseURL   string
	wsAnnounceBaseURL string
	siteSource        string
	announceInterval  int
	minInterval       int
}

// NewPTService creates a new PTService backed by GORM.
func NewPTService(db *gorm.DB, storagePath, announceBaseURL, wsAnnounceBaseURL, siteSource string, announceInterval, minInterval int) corept.PTService {
	if storagePath == "" {
		storagePath = "custom/data/paopao-ce/torrents"
	}
	if announceBaseURL == "" {
		announceBaseURL = "http://127.0.0.1:8008/tracker/announce"
	}
	if wsAnnounceBaseURL == "" {
		wsAnnounceBaseURL = "ws://127.0.0.1:8008/tracker/ws"
	}
	if siteSource == "" {
		siteSource = "[PaoPao] PaoPao PT"
	}
	if announceInterval <= 0 {
		announceInterval = 1800 // 30 minutes
	}
	if minInterval <= 0 {
		minInterval = 300 // 5 minutes
	}

	_ = os.MkdirAll(storagePath, 0755)

	// Ensure all PT tables and newly added columns exist
	_ = db.AutoMigrate(
		&pt.Torrent{},
		&pt.TorrentFile{},
		&pt.TorrentPeer{},
		&pt.TorrentSnatched{},
		&pt.UserPTProfile{},
		&pt.PTCategory{},
		&pt.PTBonusLog{},
		&pt.PTSettingRecord{},
		&pt.PTInvite{},
	)

	// Load persisted authority rules if present
	var setting pt.PTSettingRecord
	if err := db.Where("key = ?", "authority_rules").First(&setting).Error; err == nil && setting.Value != "" {
		var rules pt.PTAuthorityRules
		if err := json.Unmarshal([]byte(setting.Value), &rules); err == nil {
			authority.SetAuthorityRules(&rules)
		}
	}

	return &ptService{
		db:                db,
		storagePath:       storagePath,
		announceBaseURL:   announceBaseURL,
		wsAnnounceBaseURL: wsAnnounceBaseURL,
		siteSource:        siteSource,
		announceInterval:  announceInterval,
		minInterval:       minInterval,
	}
}

func (s *ptService) GetTorrent(id uint64) (*pt.Torrent, error) {
	var t pt.Torrent
	err := s.db.Preload("Category").Where("id = ?", id).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTorrentNotFound
		}
		return nil, err
	}
	// Fetch author nickname/username
	var user struct {
		Username string
		Nickname string
	}
	if err := s.db.Table("p_user").Select("username, nickname").Where("id = ?", t.UserID).Scan(&user).Error; err == nil {
		t.Username = user.Username
		t.Nickname = user.Nickname
	}
	return &t, nil
}

func (s *ptService) GetTorrentByInfoHash(infoHashHex string) (*pt.Torrent, error) {
	var t pt.Torrent
	err := s.db.Where("info_hash = ?", strings.ToLower(infoHashHex)).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTorrentNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (s *ptService) ListTorrents(req *corept.ListTorrentsReq) (*corept.ListTorrentsResp, error) {
	query := s.db.Model(&pt.Torrent{}).Where("status = ?", pt.TorrentStatusNormal)

	if req.Keyword != "" {
		kw := "%" + strings.TrimSpace(req.Keyword) + "%"
		query = query.Where("name LIKE ? OR small_descr LIKE ?", kw, kw)
	}
	if req.CategoryID > 0 {
		query = query.Where("category_id = ?", req.CategoryID)
	}
	if req.SpState > 0 {
		query = query.Where("sp_state = ?", req.SpState)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	sortField := "id"
	switch req.SortBy {
	case "seeders", "size", "times_completed", "hits", "created_at":
		sortField = req.SortBy
	}
	sortOrder := "DESC"
	if strings.ToLower(req.SortOrder) == "asc" {
		sortOrder = "ASC"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortField, sortOrder))

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var list []*pt.Torrent
	if err := query.Preload("Category").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}

	// Attach author names
	if len(list) > 0 {
		var userIDs []int64
		for _, item := range list {
			userIDs = append(userIDs, item.UserID)
		}
		var users []struct {
			ID       int64
			Username string
			Nickname string
		}
		s.db.Table("p_user").Select("id, username, nickname").Where("id IN ?", userIDs).Scan(&users)
		userMap := make(map[int64]struct{ Username, Nickname string })
		for _, u := range users {
			userMap[u.ID] = struct{ Username, Nickname string }{u.Username, u.Nickname}
		}
		for _, item := range list {
			if u, ok := userMap[item.UserID]; ok {
				item.Username = u.Username
				item.Nickname = u.Nickname
			}
		}
	}

	return &corept.ListTorrentsResp{
		Total: total,
		List:  list,
	}, nil
}

func (s *ptService) CreateTorrent(userID int64, req *corept.UploadTorrentReq, rawTorrent []byte) (*pt.Torrent, error) {
	// 1. Sanitize torrent to enforce PT requirements (private=1, clean announce list, source tag)
	sanitizedBytes, infoHashBytes, err := torrent.SanitizeForPT(rawTorrent, s.announceBaseURL, s.siteSource)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize torrent: %w", err)
	}

	meta, err := torrent.ParseBytes(sanitizedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sanitized torrent: %w", err)
	}

	infoHashHex := hex.EncodeToString(infoHashBytes[:])

	// Check if already exists on tracker
	var existing pt.Torrent
	if err := s.db.Unscoped().Where("info_hash = ?", infoHashHex).First(&existing).Error; err == nil {
		canReplace := false
		if existing.DeletedAt.Valid {
			canReplace = true
		} else if existing.PostID > 0 {
			var postCount int64
			s.db.Table("p_post").Where("id = ? AND is_del = 0", existing.PostID).Count(&postCount)
			if postCount == 0 {
				// The post it was bound to is deleted or gone
				canReplace = true
			}
		} else if existing.PostID == 0 {
			// Unattached torrent (draft from compose dialog that was not posted yet)
			if existing.UserID == userID {
				canReplace = true
			}
		}

		if canReplace {
			_ = s.db.Unscoped().Where("torrent_id = ?", existing.ID).Delete(&pt.TorrentFile{})
			_ = s.db.Unscoped().Where("torrent_id = ?", existing.ID).Delete(&pt.TorrentPeer{})
			_ = s.db.Unscoped().Delete(&existing)
			if existing.FilePath != "" {
				_ = os.Remove(existing.FilePath)
			}
		} else {
			return nil, errors.New("this torrent is already registered with this tracker")
		}
	}

	// 2. Save torrent file to disk
	fileName := fmt.Sprintf("%s.torrent", infoHashHex)
	filePath := filepath.Join(s.storagePath, fileName)
	if err := os.WriteFile(filePath, sanitizedBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write torrent file: %w", err)
	}

	torrentName := req.Name
	if torrentName == "" {
		torrentName = meta.Name
	}
	spState := req.SpState
	if spState <= 0 {
		spState = pt.SpStateNormal
	}

	t := &pt.Torrent{
		InfoHash:    infoHashHex,
		Name:        torrentName,
		CleanName:   meta.Name,
		SmallDescr:  req.SmallDescr,
		Descr:       req.Descr,
		CategoryID:  req.CategoryID,
		Size:        meta.TotalLength,
		NumFiles:    len(meta.Files),
		UserID:      userID,
		SpState:     spState,
		IsAnonymous: req.IsAnonymous,
		Status:      pt.TorrentStatusNormal,
		Nfo:         req.Nfo,
		FilePath:    filePath,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}

		if len(meta.Files) > 0 {
			var files []pt.TorrentFile
			for _, f := range meta.Files {
				files = append(files, pt.TorrentFile{
					TorrentID: t.ID,
					FilePath:  f.Path,
					Size:      f.Length,
				})
			}
			if err := tx.Create(&files).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = os.Remove(filePath)
		return nil, err
	}

	return t, nil
}

func (s *ptService) UpdateTorrent(t *pt.Torrent) error {
	updates := map[string]any{
		"name":            t.Name,
		"small_descr":     t.SmallDescr,
		"descr":           t.Descr,
		"sp_state":        t.SpState,
		"promotion_until": t.PromotionUntil,
		"is_anonymous":    t.IsAnonymous,
		"status":          t.Status,
		"nfo":             t.Nfo,
	}
	if t.CategoryID > 0 {
		updates["category_id"] = t.CategoryID
	}
	return s.db.Model(&pt.Torrent{}).Where("id = ?", t.ID).Updates(updates).Error
}

func (s *ptService) DeleteTorrent(id uint64, userID int64, isAdmin bool) error {
	var t pt.Torrent
	if err := s.db.First(&t, id).Error; err != nil {
		return err
	}
	if !isAdmin && t.UserID != userID {
		return errors.New("permission denied")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		_ = tx.Unscoped().Where("torrent_id = ?", t.ID).Delete(&pt.TorrentFile{})
		_ = tx.Unscoped().Where("torrent_id = ?", t.ID).Delete(&pt.TorrentPeer{})
		if err := tx.Unscoped().Delete(&t).Error; err != nil {
			return err
		}
		if t.FilePath != "" {
			_ = os.Remove(t.FilePath)
		}
		return nil
	})
}

func (s *ptService) GetTorrentFiles(torrentID uint64) ([]pt.TorrentFile, error) {
	var files []pt.TorrentFile
	err := s.db.Where("torrent_id = ?", torrentID).Find(&files).Error
	return files, err
}

func (s *ptService) BindTorrentPost(torrentID uint64, postID int64) error {
	return s.db.Model(&pt.Torrent{}).Where("id = ?", torrentID).Update("post_id", postID).Error
}

func (s *ptService) GetTorrentsByPostIDs(postIDs []int64) (map[int64]*pt.Torrent, error) {
	if len(postIDs) == 0 {
		return map[int64]*pt.Torrent{}, nil
	}
	var torrents []pt.Torrent
	err := s.db.Where("post_id IN ? AND status = ?", postIDs, pt.TorrentStatusNormal).Find(&torrents).Error
	if err != nil {
		return nil, err
	}
	res := make(map[int64]*pt.Torrent, len(torrents))
	for i := range torrents {
		res[torrents[i].PostID] = &torrents[i]
	}
	return res, nil
}

func (s *ptService) GenerateUserTorrent(torrentID uint64, userID int64) ([]byte, string, error) {
	t, err := s.GetTorrent(torrentID)
	if err != nil {
		return nil, "", err
	}

	profile, err := s.GetUserPTProfile(userID)
	if err != nil {
		return nil, "", err
	}
	if !profile.CanLeech {
		return nil, "", ErrUserDisabled
	}
	if !authority.CanDownloadTorrent(profile.Class, false) {
		return nil, "", ErrDownloadPermissionDenied
	}
	if profile.Class == pt.ClassPeasant && t.SpState != pt.SpStateFree && t.SpState != pt.SpState2XFree {
		return nil, "", ErrPeasantRestricted
	}

	// Read base sanitized torrent file
	data, err := os.ReadFile(t.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read stored torrent: %w", err)
	}

	// Inject user's unique passkey
	customBytes, err := torrent.CustomizeForUser(data, profile.Passkey, s.announceBaseURL, s.wsAnnounceBaseURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to customize torrent: %w", err)
	}

	// Update download hits
	s.db.Model(&pt.Torrent{}).Where("id = ?", t.ID).UpdateColumn("hits", gorm.Expr("hits + ?", 1))

	downloadFilename := fmt.Sprintf("[%s].%s.torrent", s.siteSource, t.CleanName)
	return customBytes, downloadFilename, nil
}

func (s *ptService) GetUserPTProfile(userID int64) (*pt.UserPTProfile, error) {
	var profile pt.UserPTProfile
	err := s.db.Where("user_id = ?", userID).First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Auto create PT profile for this user
			passkey := generatePasskey()
			profile = pt.UserPTProfile{
				UserID:    userID,
				Passkey:   passkey,
				CanLeech:  true,
				CanUpload: true,
			}
			if err := s.db.Create(&profile).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Calculate ratio
	if profile.Downloaded > 0 {
		profile.Ratio = math.Round((float64(profile.Uploaded)/float64(profile.Downloaded))*100) / 100
	} else if profile.Uploaded > 0 {
		profile.Ratio = float64(profile.Uploaded) / (1024 * 1024 * 1024) // Show upload volume if 0 dl
	} else {
		profile.Ratio = 0
	}

	// Active peer counts
	var seedCount, leechCount int64
	now := time.Now().Unix()
	activeThreshold := now - int64(s.announceInterval*2)
	s.db.Model(&pt.TorrentPeer{}).Where("user_id = ? AND is_seeder = 1 AND last_action >= ?", userID, activeThreshold).Count(&seedCount)
	s.db.Model(&pt.TorrentPeer{}).Where("user_id = ? AND is_seeder = 0 AND last_action >= ?", userID, activeThreshold).Count(&leechCount)
	profile.SeedingCount = int(seedCount)
	profile.LeechCount = int(leechCount)

	// Check VIP expiration
	if profile.Class == pt.ClassVIP && profile.VipUntil > 0 && profile.VipUntil <= now {
		targetClass := profile.OldClass
		if targetClass == 0 || targetClass == pt.ClassVIP {
			targetClass = pt.ClassUser
		}
		profile.Class = targetClass
		profile.VipUntil = 0
		s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
			"class":     targetClass,
			"vip_until": 0,
		})
	}

	// Format class label and badge color
	profile.ClassName = authority.GetClassName(profile.Class)
	profile.ClassColor = authority.GetClassColor(profile.Class)
	profile.Privileges = authority.GetPrivilegeList(profile.Class)
	profile.KarmaMultiplier = authority.GetKarmaMultiplier(profile.Class)
	profile.CanCustomPromotion = authority.CanCustomPromotion(profile.Class, false)
	profile.CanAnonymous = authority.CanBeAnonymous(profile.Class, false)
	profile.CanDownload = profile.CanLeech && authority.CanDownloadTorrent(profile.Class, false)

	// Fetch user registration time to compute next class progress
	var u dbr.User
	if err := s.db.Select("created_on").Where("id = ?", userID).First(&u).Error; err == nil && u.Model != nil {
		profile.NextClass = authority.GetNextClassProgress(&profile, u.CreatedOn)
	}

	return &profile, nil
}

func (s *ptService) GetUserByPasskey(passkey string) (*pt.UserPTProfile, error) {
	var profile pt.UserPTProfile
	err := s.db.Where("passkey = ?", passkey).First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidPasskey
		}
		return nil, err
	}
	return &profile, nil
}

func (s *ptService) ResetPasskey(userID int64) (string, error) {
	newKey := generatePasskey()
	err := s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", userID).Update("passkey", newKey).Error
	if err != nil {
		return "", err
	}
	return newKey, nil
}

func (s *ptService) GetUserSnatches(userID int64, page, pageSize int) ([]pt.TorrentSnatched, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	s.db.Model(&pt.TorrentSnatched{}).Where("user_id = ?", userID).Count(&total)

	var list []pt.TorrentSnatched
	err := s.db.Where("user_id = ?", userID).Order("last_action DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (s *ptService) GetBonusOptions() []pt.PTBonusOption {
	return []pt.PTBonusOption{
		{ID: 1, Name: "1.0 GB 上传量", Points: 1000.0, UploadBytes: 1024 * 1024 * 1024, Description: "增加 1.0 GB 账户上传量流量"},
		{ID: 2, Name: "5.0 GB 上传量", Points: 4000.0, UploadBytes: 5 * 1024 * 1024 * 1024, Description: "增加 5.0 GB 账户上传量流量 (优惠)"},
		{ID: 3, Name: "10.0 GB 上传量", Points: 7000.0, UploadBytes: 10 * 1024 * 1024 * 1024, Description: "增加 10.0 GB 账户上传量流量 (特惠)"},
		{ID: 4, Name: "1 枚 注册邀请码", Points: 10000.0, Invites: 1, Description: "获得 1 个邀请新用户注册的宝贵名额"},
		{ID: 5, Name: "30 天 VIP 资格", Points: 15000.0, Description: "获得 30 天 VIP 贵宾特权 (免除分享率考核、免下载计费、专属贵宾标识)"},
	}
}

func (s *ptService) ExchangeBonus(userID int64, optionID int) error {
	options := s.GetBonusOptions()
	var selected *pt.PTBonusOption
	for _, opt := range options {
		if opt.ID == optionID {
			selected = &opt
			break
		}
	}
	if selected == nil {
		return ErrInvalidBonusItem
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var profile pt.UserPTProfile
		if err := tx.Where("user_id = ?", userID).First(&profile).Error; err != nil {
			return err
		}
		if profile.SeedBonus < selected.Points {
			return ErrInsufficientBonus
		}

		// Deduct bonus
		profile.SeedBonus -= selected.Points
		if selected.UploadBytes > 0 {
			profile.Uploaded += selected.UploadBytes
		}
		if selected.Invites > 0 {
			profile.Invites += selected.Invites
		}
		if selected.ID == 5 {
			now := time.Now().Unix()
			if profile.Class != pt.ClassVIP {
				profile.OldClass = profile.Class
				profile.Class = pt.ClassVIP
			}
			baseTime := now
			if profile.VipUntil > now {
				baseTime = profile.VipUntil
			}
			profile.VipUntil = baseTime + 30*86400
		}

		if err := tx.Save(&profile).Error; err != nil {
			return err
		}

		// Log transaction
		log := pt.PTBonusLog{
			UserID:      userID,
			ActionType:  fmt.Sprintf("buy_option_%d", selected.ID),
			CostBonus:   selected.Points,
			Description: fmt.Sprintf("兑换了: %s", selected.Name),
		}
		return tx.Create(&log).Error
	})
}

func (s *ptService) CalculateHourlyKarma() error {
	// Simple & robust NexusPHP Karma algorithm:
	// For each active seeder in the swarm during the past hour,
	// reward base points = 1.0 + (size in GB * 0.1) per torrent.
	now := time.Now().Unix()
	oneHourAgo := now - 3600

	type ActiveSeed struct {
		UserID int64
		Size   int64
	}

	var seeds []ActiveSeed
	err := s.db.Table("p_torrent_peer AS p").
		Select("p.user_id, t.size").
		Joins("JOIN p_torrent AS t ON p.torrent_id = t.id").
		Where("p.is_seeder = 1 AND p.last_action >= ?", oneHourAgo).
		Scan(&seeds).Error
	if err != nil {
		return err
	}

	userBonus := make(map[int64]float64)
	for _, s := range seeds {
		sizeGB := float64(s.Size) / (1024 * 1024 * 1024)
		points := 1.0 + (sizeGB * 0.05)
		userBonus[s.UserID] += points
	}

	var userClasses []struct {
		UserID int64 `gorm:"column:user_id"`
		Class  int   `gorm:"column:class"`
	}
	s.db.Table("p_user_pt_profile").Select("user_id, class").Find(&userClasses)
	classMap := make(map[int64]int)
	for _, uc := range userClasses {
		classMap[uc.UserID] = uc.Class
	}

	for uid, points := range userBonus {
		multi := authority.GetKarmaMultiplier(classMap[uid])
		finalPoints := points * multi
		s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", uid).
			UpdateColumn("seed_bonus", gorm.Expr("seed_bonus + ?", finalPoints))
	}
	return nil
}

func (s *ptService) ListCategories() ([]pt.PTCategory, error) {
	var cats []pt.PTCategory
	err := s.db.Order("sort ASC, id ASC").Find(&cats).Error
	return cats, err
}

func (s *ptService) GetTorrentPeers(torrentID uint64, limit int) ([]pt.TorrentPeer, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var peers []pt.TorrentPeer
	now := time.Now().Unix()
	activeThreshold := now - int64(s.announceInterval*2)
	err := s.db.Where("torrent_id = ? AND last_action >= ?", torrentID, activeThreshold).
		Order("is_seeder DESC, last_action DESC").Limit(limit).Find(&peers).Error
	return peers, err
}

func (s *ptService) HandleAnnounce(req *corept.AnnounceReq) (*corept.AnnounceResp, error) {
	now := time.Now().Unix()

	// 1. Authenticate user by passkey
	profile, err := s.GetUserByPasskey(req.Passkey)
	if err != nil {
		return nil, ErrInvalidPasskey
	}
	if !profile.CanLeech && req.Left > 0 {
		return nil, ErrUserDisabled
	}
	if !authority.CanDownloadTorrent(profile.Class, false) && req.Left > 0 {
		return nil, ErrDownloadPermissionDenied
	}

	// 2. Identify torrent
	infoHashHex := hex.EncodeToString(req.InfoHash)
	var t pt.Torrent
	if err := s.db.Where("info_hash = ? AND status = ?", infoHashHex, pt.TorrentStatusNormal).First(&t).Error; err != nil {
		return nil, errors.New("torrent not registered with this tracker")
	}

	// Peasant restriction: cannot leech non-free torrents
	if profile.Class == pt.ClassPeasant && req.Left > 0 && t.SpState != pt.SpStateFree && t.SpState != pt.SpState2XFree {
		return nil, ErrPeasantRestricted
	}

	isSeeder := (req.Left == 0)
	peerIDHex := hex.EncodeToString(req.PeerID)

	// 3. Handle Peer Swarm tracking & Accounting
	var prevPeer pt.TorrentPeer
	hasPrev := (s.db.Where("torrent_id = ? AND peer_id = ?", t.ID, peerIDHex).First(&prevPeer).Error == nil)

	var deltaUp, deltaDown uint64
	if hasPrev {
		if req.Uploaded > prevPeer.Uploaded {
			deltaUp = req.Uploaded - prevPeer.Uploaded
		}
		if req.Downloaded > prevPeer.Downloaded {
			deltaDown = req.Downloaded - prevPeer.Downloaded
		}
	}

	// Calculate promotion multiplier
	var creditedUp, creditedDown uint64
	switch t.SpState {
	case pt.SpStateFree:
		creditedUp = deltaUp
		creditedDown = 0
	case pt.SpState2X:
		creditedUp = deltaUp * 2
		creditedDown = deltaDown
	case pt.SpState2XFree:
		creditedUp = deltaUp * 2
		creditedDown = 0
	case pt.SpStateHalfDown:
		creditedUp = deltaUp
		creditedDown = deltaDown / 2
	case pt.SpState2XHalfDown:
		creditedUp = deltaUp * 2
		creditedDown = deltaDown / 2
	case pt.SpStateThirtyPercent:
		creditedUp = deltaUp
		creditedDown = uint64(float64(deltaDown) * 0.3)
	default: // Normal
		creditedUp = deltaUp
		creditedDown = deltaDown
	}

	// VIP & Retiree privileges: 0x Download count (no upload contribution required)
	if profile.Class == pt.ClassVIP || authority.IsRetireeFreeUpload(profile.Class) {
		creditedDown = 0
	}

	// Update user traffic
	if creditedUp > 0 || creditedDown > 0 {
		s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", profile.UserID).Updates(map[string]any{
			"uploaded":   gorm.Expr("uploaded + ?", creditedUp),
			"downloaded": gorm.Expr("downloaded + ?", creditedDown),
		})
	}

	// Process BitTorrent event
	switch req.Event {
	case "stopped":
		s.db.Where("torrent_id = ? AND peer_id = ?", t.ID, peerIDHex).Delete(&pt.TorrentPeer{})
	case "completed":
		s.db.Model(&pt.Torrent{}).Where("id = ?", t.ID).UpdateColumn("times_completed", gorm.Expr("times_completed + ?", 1))
		// Upsert snatch record
		snatch := pt.TorrentSnatched{
			TorrentID:   t.ID,
			UserID:      profile.UserID,
			Uploaded:    req.Uploaded,
			Downloaded:  req.Downloaded,
			ToGo:        0,
			IsFinished:  true,
			CompletedAt: now,
			LastAction:  now,
		}
		s.db.Where("torrent_id = ? AND user_id = ?", t.ID, profile.UserID).
			Assign(map[string]any{
				"is_finished":  true,
				"completed_at": now,
				"last_action":  now,
				"to_go":        0,
			}).FirstOrCreate(&snatch)
		fallthrough
	default:
		// Regular or started announce
		peer := pt.TorrentPeer{
			TorrentID:  t.ID,
			UserID:     profile.UserID,
			PeerID:     peerIDHex,
			IP:         req.IP,
			Port:       req.Port,
			Uploaded:   req.Uploaded,
			Downloaded: req.Downloaded,
			ToGo:       req.Left,
			IsSeeder:   isSeeder,
			Agent:      req.Agent,
			LastAction: now,
			Started:    now,
		}
		if hasPrev {
			s.db.Model(&pt.TorrentPeer{}).Where("id = ?", prevPeer.ID).Updates(map[string]any{
				"ip":          req.IP,
				"port":        req.Port,
				"uploaded":    req.Uploaded,
				"downloaded":  req.Downloaded,
				"to_go":       req.Left,
				"is_seeder":   isSeeder,
				"agent":       req.Agent,
				"last_action": now,
			})
		} else {
			s.db.Create(&peer)
		}
	}

	// 4. Update Swarm seeders and leechers counts
	activeThreshold := now - int64(s.announceInterval*2)
	var seederCount, leecherCount int64
	s.db.Model(&pt.TorrentPeer{}).Where("torrent_id = ? AND is_seeder = 1 AND last_action >= ?", t.ID, activeThreshold).Count(&seederCount)
	s.db.Model(&pt.TorrentPeer{}).Where("torrent_id = ? AND is_seeder = 0 AND last_action >= ?", t.ID, activeThreshold).Count(&leecherCount)
	s.db.Model(&pt.Torrent{}).Where("id = ?", t.ID).Updates(map[string]any{
		"seeders":  seederCount,
		"leechers": leecherCount,
	})

	// 5. Query candidate peers for announcer
	numWant := req.NumWant
	if numWant <= 0 || numWant > 50 {
		numWant = 50
	}

	peerQuery := s.db.Model(&pt.TorrentPeer{}).
		Where("torrent_id = ? AND peer_id != ? AND last_action >= ?", t.ID, peerIDHex, activeThreshold)
	if isSeeder {
		// Seeders only need leechers
		peerQuery = peerQuery.Where("is_seeder = 0")
	}

	var swarmPeers []pt.TorrentPeer
	peerQuery.Limit(numWant).Find(&swarmPeers)

	// Build compact binary IPv4/port format (6 bytes per peer)
	var compactBuf []byte
	var dictPeers []corept.AnnouncePeer

	for _, p := range swarmPeers {
		ip := net.ParseIP(p.IP).To4()
		if ip != nil {
			buf := make([]byte, 6)
			copy(buf[0:4], ip)
			binary.BigEndian.PutUint16(buf[4:6], uint16(p.Port))
			compactBuf = append(compactBuf, buf...)
		}
		dictPeers = append(dictPeers, corept.AnnouncePeer{
			PeerID: p.PeerID,
			IP:     p.IP,
			Port:   p.Port,
		})
	}

	return &corept.AnnounceResp{
		Interval:    s.announceInterval,
		MinInterval: s.minInterval,
		Complete:    int(seederCount),
		Incomplete:  int(leecherCount),
		Peers:       dictPeers,
		PeersBinary: compactBuf,
	}, nil
}

func (s *ptService) HandleScrape(req *corept.ScrapeReq) (*corept.ScrapeResp, error) {
	resp := &corept.ScrapeResp{}
	if len(req.InfoHashes) == 0 {
		var list []pt.Torrent
		s.db.Where("status = ?", pt.TorrentStatusNormal).Limit(100).Find(&list)
		for _, t := range list {
			hashBytes, err := hex.DecodeString(t.InfoHash)
			if err == nil && len(hashBytes) == 20 {
				var h [20]byte
				copy(h[:], hashBytes)
				resp.Files = append(resp.Files, corept.ScrapeFile{
					InfoHash:   h,
					Complete:   t.Seeders,
					Downloaded: t.TimesCompleted,
					Incomplete: t.Leechers,
				})
			}
		}
		return resp, nil
	}

	for _, ih := range req.InfoHashes {
		infoHashHex := hex.EncodeToString(ih)
		var t pt.Torrent
		if err := s.db.Where("info_hash = ?", infoHashHex).First(&t).Error; err == nil {
			var h [20]byte
			copy(h[:], ih)
			resp.Files = append(resp.Files, corept.ScrapeFile{
				InfoHash:   h,
				Complete:   t.Seeders,
				Downloaded: t.TimesCompleted,
				Incomplete: t.Leechers,
			})
		}
	}

	return resp, nil
}

func generatePasskey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *ptService) UpdateUserClass(userID int64, targetClass int, customTitle string) error {
	updates := map[string]interface{}{
		"class": targetClass,
	}
	if customTitle != "" {
		updates["custom_title"] = customTitle
	}
	return s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", userID).Updates(updates).Error
}

func (s *ptService) RunPromotionAndDemotion() error {
	now := time.Now().Unix()

	// 1. Revert expired VIPs back to old_class
	var expiredVIPs []pt.UserPTProfile
	if err := s.db.Where("class = ? AND vip_until > 0 AND vip_until <= ?", pt.ClassVIP, now).Find(&expiredVIPs).Error; err == nil {
		for _, u := range expiredVIPs {
			targetClass := u.OldClass
			if targetClass <= 0 || targetClass == pt.ClassVIP {
				targetClass = pt.ClassUser
			}
			s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", u.UserID).Updates(map[string]interface{}{
				"class":     targetClass,
				"vip_until": 0,
			})
			className := authority.GetClassName(targetClass)
			s.sendSystemMessage(u.UserID, "VIP 特权到期通知", fmt.Sprintf("尊敬的用户，您的 VIP 贵宾特权已到期，系统已为您恢复为「%s」等级。感谢您的支持！", className))
		}
	}

	// 2. Fetch all user PT profiles with user registration timestamp
	type UserRow struct {
		UserID       int64
		CreatedOn    int64
		Class        int
		MaxClassOnce int
		Uploaded     uint64
		Downloaded   uint64
		SeedBonus    float64
		CanLeech     bool
		CanUpload    bool
	}
	var users []UserRow
	err := s.db.Table("p_user_pt_profile AS pt").
		Select("pt.user_id, pt.class, pt.max_class_once, pt.uploaded, pt.downloaded, pt.seed_bonus, pt.can_leech, pt.can_upload, u.created_on").
		Joins("JOIN p_user AS u ON pt.user_id = u.id").
		Scan(&users).Error
	if err != nil {
		return err
	}

	for _, u := range users {
		prof := pt.UserPTProfile{
			UserID:       u.UserID,
			Class:        u.Class,
			MaxClassOnce: u.MaxClassOnce,
			Uploaded:     u.Uploaded,
			Downloaded:   u.Downloaded,
			SeedBonus:    u.SeedBonus,
		}

		// A. Check Peasant recovery
		if prof.Class == pt.ClassPeasant && authority.CheckPeasantRecovery(&prof) {
			s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", prof.UserID).Updates(map[string]interface{}{
				"class":      pt.ClassUser,
				"can_leech":  true,
				"can_upload": true,
			})
			s.sendSystemMessage(prof.UserID, "解除贫民警告通知", "恭喜您！通过积极保种与上传流量，您的分享率已恢复达标，已解除贫民 (Peasant) 限制并恢复为普通会员！")
			continue
		}

		// B. Check Demotion
		if newClass, demoted := authority.CheckDemotion(&prof); demoted {
			updates := map[string]interface{}{
				"class": newClass,
			}
			if newClass == pt.ClassPeasant {
				updates["can_upload"] = false
			}
			s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", prof.UserID).Updates(updates)
			className := authority.GetClassName(newClass)
			s.sendSystemMessage(prof.UserID, "等级降级通知", fmt.Sprintf("很遗憾，由于您的分享率已跌破维持标准，您的等级已自动降为「%s」。请保持做种并关注分享率！", className))
			continue
		}

		// C. Check Promotion
		if newClass, awardInvites, promoted := authority.CheckPromotion(&prof, u.CreatedOn); promoted {
			updates := map[string]interface{}{
				"class": newClass,
			}
			if newClass > prof.MaxClassOnce {
				updates["max_class_once"] = newClass
				if awardInvites > 0 {
					updates["invites"] = gorm.Expr("invites + ?", awardInvites)
				}
			}
			s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", prof.UserID).Updates(updates)
			className := authority.GetClassName(newClass)
			msg := fmt.Sprintf("恭喜您！由于您积极做种与贡献流量，您的等级已晋升为「%s」！", className)
			if awardInvites > 0 {
				msg += fmt.Sprintf(" 系统额外奖励您 %d 枚注册邀请码！", awardInvites)
			}
			s.sendSystemMessage(prof.UserID, "恭喜晋升等级通知", msg)
		}
	}

	return nil
}

func (s *ptService) sendSystemMessage(receiverUserID int64, brief, content string) {
	msg := dbr.Message{
		Model: &dbr.Model{
			CreatedOn: time.Now().Unix(),
		},
		SenderUserID:   0,
		ReceiverUserID: receiverUserID,
		Type:           dbr.MsgTypeSystem,
		Brief:          brief,
		Content:        content,
	}
	_ = s.db.Create(&msg).Error
}

func (s *ptService) AdminListUsers(keyword string, classFilter int, page, pageSize int) ([]*corept.AdminPTUserItem, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := s.db.Table("p_user AS u").
		Select("u.id AS user_id, u.username, u.nickname, u.avatar, u.is_admin, u.created_on, " +
			"COALESCE(pt.class, 1) AS class, COALESCE(pt.custom_title, '') AS custom_title, " +
			"COALESCE(pt.vip_until, 0) AS vip_until, COALESCE(pt.uploaded, 0) AS uploaded, " +
			"COALESCE(pt.downloaded, 0) AS downloaded, COALESCE(pt.seed_bonus, 0) AS seed_bonus, " +
			"COALESCE(pt.can_leech, 1) AS can_leech, COALESCE(pt.can_upload, 1) AS can_upload").
		Joins("LEFT JOIN p_user_pt_profile AS pt ON u.id = pt.user_id").
		Where("u.is_del = 0")

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("u.username LIKE ? OR u.nickname LIKE ? OR u.id = ?", like, like, keyword)
	}

	if classFilter >= 0 {
		query = query.Where("COALESCE(pt.class, 1) = ?", classFilter)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type ResultRow struct {
		UserID      int64
		Username    string
		Nickname    string
		Avatar      string
		IsAdmin     bool
		CreatedOn   int64
		Class       int
		CustomTitle string
		VipUntil    int64
		Uploaded    uint64
		Downloaded  uint64
		SeedBonus   float64
		CanLeech    bool
		CanUpload   bool
	}
	var rows []ResultRow
	if err := query.Order("u.id ASC").Offset(offset).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	var items []*corept.AdminPTUserItem
	for _, r := range rows {
		var ratio float64
		if r.Downloaded > 0 {
			ratio = math.Round((float64(r.Uploaded)/float64(r.Downloaded))*100) / 100
		} else if r.Uploaded > 0 {
			ratio = 999.0
		}

		item := &corept.AdminPTUserItem{
			UserID:      r.UserID,
			Username:    r.Username,
			Nickname:    r.Nickname,
			Avatar:      r.Avatar,
			IsAdmin:     r.IsAdmin,
			Class:       r.Class,
			ClassName:   authority.GetClassName(r.Class),
			ClassColor:  authority.GetClassColor(r.Class),
			CustomTitle: r.CustomTitle,
			VipUntil:    r.VipUntil,
			Uploaded:    r.Uploaded,
			Downloaded:  r.Downloaded,
			Ratio:       ratio,
			SeedBonus:   r.SeedBonus,
			CanLeech:    r.CanLeech,
			CanUpload:   r.CanUpload,
			CreatedOn:   r.CreatedOn,
		}
		items = append(items, item)
	}

	return items, total, nil
}

func (s *ptService) AdminUpdateUser(req *corept.AdminUpdateUserReq) error {
	if req == nil || req.UserID <= 0 {
		return errors.New("invalid user_id")
	}

	// Ensure profile exists
	_, err := s.GetUserPTProfile(req.UserID)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Class >= 0 && req.Class <= 16 {
		updates["class"] = req.Class
	}
	updates["custom_title"] = req.CustomTitle

	if req.CanUpload != nil {
		updates["can_upload"] = *req.CanUpload
	}
	if req.CanLeech != nil {
		updates["can_leech"] = *req.CanLeech
	}

	now := time.Now().Unix()
	if req.VipDays > 0 {
		updates["vip_until"] = now + int64(req.VipDays*86400)
		updates["class"] = pt.ClassVIP
	} else if req.VipDays == -1 {
		updates["vip_until"] = 0
		if req.Class == pt.ClassVIP {
			updates["class"] = pt.ClassUser
		}
	}

	return s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", req.UserID).Updates(updates).Error
}

func (s *ptService) GetAuthorityRules() *pt.PTAuthorityRules {
	return authority.GetAuthorityRules()
}

func (s *ptService) SaveAuthorityRules(rules *pt.PTAuthorityRules) error {
	if rules == nil {
		return errors.New("nil authority rules")
	}
	bytes, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	record := pt.PTSettingRecord{
		Key:       "authority_rules",
		Value:     string(bytes),
		UpdatedAt: time.Now().Unix(),
	}
	if err := s.db.Save(&record).Error; err != nil {
		return err
	}
	authority.SetAuthorityRules(rules)
	return nil
}

func (s *ptService) GetMyInvites(userID int64) ([]*pt.PTInvite, int, error) {
	profile, err := s.GetUserPTProfile(userID)
	if err != nil {
		return nil, 0, err
	}

	var list []*pt.PTInvite
	now := time.Now().Unix()
	if err := s.db.Where("inviter_id = ?", userID).Order("id DESC").Find(&list).Error; err != nil {
		return nil, profile.Invites, err
	}

	var invitedUIDs []int64
	for _, inv := range list {
		if inv.Status == pt.InviteStatusUnused && inv.ExpireAt < now {
			inv.Status = pt.InviteStatusExpired
			s.db.Model(&pt.PTInvite{}).Where("id = ? AND status = ?", inv.ID, pt.InviteStatusUnused).Update("status", pt.InviteStatusExpired)
		}
		if inv.InvitedUID > 0 {
			invitedUIDs = append(invitedUIDs, inv.InvitedUID)
		}
	}

	if len(invitedUIDs) > 0 {
		type userBrief struct {
			ID       int64  `gorm:"column:id"`
			Username string `gorm:"column:username"`
			Nickname string `gorm:"column:nickname"`
		}
		var users []userBrief
		s.db.Table("p_user").Where("id IN ?", invitedUIDs).Find(&users)
		userMap := make(map[int64]string)
		for _, u := range users {
			name := u.Nickname
			if name == "" {
				name = u.Username
			}
			userMap[u.ID] = name
		}
		for _, inv := range list {
			if name, ok := userMap[inv.InvitedUID]; ok {
				inv.InvitedName = name
			}
		}
	}

	return list, profile.Invites, nil
}

func (s *ptService) GenerateInvite(userID int64) (*pt.PTInvite, error) {
	profile, err := s.GetUserPTProfile(userID)
	if err != nil {
		return nil, err
	}
	if profile.Invites <= 0 {
		return nil, errors.New("暂无可用邀请名额，可通过魔力值商城兑换或提升等级获取")
	}

	res := s.db.Model(&pt.UserPTProfile{}).
		Where("user_id = ? AND invites > 0", userID).
		Update("invites", gorm.Expr("invites - ?", 1))
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, errors.New("邀请名额不足")
	}

	b := make([]byte, 16)
	_, _ = rand.Read(b)
	code := hex.EncodeToString(b)

	now := time.Now().Unix()
	invite := &pt.PTInvite{
		InviterID: userID,
		Code:      code,
		Status:    pt.InviteStatusUnused,
		CreatedAt: now,
		ExpireAt:  now + 7*86400,
	}

	if err := s.db.Create(invite).Error; err != nil {
		s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", userID).Update("invites", gorm.Expr("invites + ?", 1))
		return nil, err
	}

	return invite, nil
}

func (s *ptService) CheckInvite(code string) (*pt.PTInvite, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("邀请码不能为空")
	}
	var invite pt.PTInvite
	if err := s.db.Where("code = ?", code).First(&invite).Error; err != nil {
		return nil, errors.New("邀请码无效或不存在")
	}

	now := time.Now().Unix()
	if invite.Status == pt.InviteStatusUsed {
		return nil, errors.New("该邀请码已被使用")
	}
	if invite.Status == pt.InviteStatusExpired || invite.ExpireAt < now {
		if invite.Status != pt.InviteStatusExpired {
			s.db.Model(&pt.PTInvite{}).Where("id = ?", invite.ID).Update("status", pt.InviteStatusExpired)
			invite.Status = pt.InviteStatusExpired
		}
		return nil, errors.New("该邀请码已过期")
	}

	var inviter struct {
		Username string `gorm:"column:username"`
		Nickname string `gorm:"column:nickname"`
	}
	if s.db.Table("p_user").Where("id = ?", invite.InviterID).First(&inviter).Error == nil {
		invite.InviterName = inviter.Nickname
		if invite.InviterName == "" {
			invite.InviterName = inviter.Username
		}
	}

	return &invite, nil
}

func (s *ptService) ConsumeInvite(code string, invitedUID int64) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	now := time.Now().Unix()
	res := s.db.Model(&pt.PTInvite{}).
		Where("code = ? AND status = ? AND expire_at > ?", code, pt.InviteStatusUnused, now).
		Updates(map[string]any{
			"status":      pt.InviteStatusUsed,
			"invited_uid": invitedUID,
			"used_at":     now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("邀请码无效、已使用或已过期")
	}
	return nil
}


