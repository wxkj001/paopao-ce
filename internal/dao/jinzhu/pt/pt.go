package pt

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/rocboss/paopao-ce/internal/model/pt"
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

	// Check if already exists
	var count int64
	s.db.Model(&pt.Torrent{}).Where("info_hash = ?", infoHashHex).Count(&count)
	if count > 0 {
		return nil, errors.New("this torrent is already registered with this tracker")
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
	return s.db.Model(&pt.Torrent{}).Where("id = ?", t.ID).Updates(map[string]any{
		"name":             t.Name,
		"small_descr":      t.SmallDescr,
		"descr":            t.Descr,
		"category_id":      t.CategoryID,
		"sp_state":         t.SpState,
		"promotion_until":  t.PromotionUntil,
		"is_anonymous":     t.IsAnonymous,
		"status":           t.Status,
		"nfo":              t.Nfo,
	}).Error
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
		if err := tx.Delete(&t).Error; err != nil {
			return err
		}
		_ = os.Remove(t.FilePath)
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

	for uid, points := range userBonus {
		s.db.Model(&pt.UserPTProfile{}).Where("user_id = ?", uid).
			UpdateColumn("seed_bonus", gorm.Expr("seed_bonus + ?", points))
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

	// 2. Identify torrent
	infoHashHex := hex.EncodeToString(req.InfoHash)
	var t pt.Torrent
	if err := s.db.Where("info_hash = ? AND status = ?", infoHashHex, pt.TorrentStatusNormal).First(&t).Error; err != nil {
		return nil, errors.New("torrent not registered with this tracker")
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
