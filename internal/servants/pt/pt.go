package pt

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/alimy/tryst/cfg"
	"github.com/robfig/cron/v3"
	"github.com/rocboss/paopao-ce/internal/conf"
	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	jinzhupt "github.com/rocboss/paopao-ce/internal/dao/jinzhu/pt"
	"github.com/rocboss/paopao-ce/internal/infra/events"
	modelpt "github.com/rocboss/paopao-ce/internal/model/pt"
	"github.com/rocboss/paopao-ce/internal/pt/authority"
	"github.com/rocboss/paopao-ce/internal/servants/base"
	"github.com/rocboss/paopao-ce/internal/servants/chain"
	"github.com/rocboss/paopao-ce/pkg/app"
	"github.com/rocboss/paopao-ce/pkg/xerror"
	"github.com/sirupsen/logrus"
)

// InitPT initializes the PT service, registers web routes and tracker endpoints, and schedules jobs.
func InitPT(e *gin.Engine) {
	var announceURL, wsAnnounceURL, siteSource, storagePath string
	var announceInterval, minInterval int
	if conf.PTSetting != nil {
		announceURL = conf.PTSetting.AnnounceURL
		wsAnnounceURL = conf.PTSetting.WsAnnounceURL
		announceInterval = conf.PTSetting.AnnounceInterval
		minInterval = conf.PTSetting.MinInterval
		siteSource = conf.PTSetting.SiteSource
		storagePath = conf.PTSetting.StoragePath
	}
	db := conf.MustGormDB()
	ptSrv := jinzhupt.NewPTService(db, storagePath, announceURL, wsAnnounceURL, siteSource, announceInterval, minInterval)
	RegisterPTServants(e, ptSrv)
	RegisterTrackerRoutes(e, ptSrv)

	cfg.Not("DisableJobManager", func() {
		schedule, err := cron.ParseStandard("@every 1h")
		if err == nil {
			events.OnTask(schedule, func() {
				if err := ptSrv.CalculateHourlyKarma(); err != nil {
					logrus.Warnf("calculateHourlyKarma occurs error: %s", err)
				}
				if err := ptSrv.RunPromotionAndDemotion(); err != nil {
					logrus.Warnf("runPromotionAndDemotion occurs error: %s", err)
				}
			})
		}
	})
}

// RegisterPTServants registers all PT web endpoints to gin.Engine.
func RegisterPTServants(e *gin.Engine, ptSrv corept.PTService) {
	s := &ptServant{ptSrv: ptSrv}

	v1 := e.Group("v1/pt")
	{
		// Public or read endpoints
		v1.GET("/categories", s.listCategories)
		v1.GET("/torrents", s.listTorrents)
		v1.GET("/torrent", s.getTorrent)
		v1.GET("/torrent/download", s.downloadTorrent)
		v1.HEAD("/torrent/download", s.downloadTorrent)
		v1.GET("/torrent/stream", s.streamTorrent)
		v1.HEAD("/torrent/stream", s.streamTorrent)
		v1.GET("/bonus/options", s.getBonusOptions)
		v1.GET("/invite/check", s.checkInvite)

		// Authenticated endpoints
		authGroup := v1.Group("", chain.JWT())
		{
			authGroup.POST("/torrent/upload", s.uploadTorrent)
			authGroup.POST("/torrent/edit", s.editTorrent)
			authGroup.POST("/torrent/delete", s.deleteTorrent)
			authGroup.GET("/user/profile", s.getUserProfile)
			authGroup.POST("/user/passkey/reset", s.resetPasskey)
			authGroup.GET("/user/snatches", s.getUserSnatches)
			authGroup.POST("/bonus/exchange", s.exchangeBonus)
			authGroup.GET("/invites", s.getMyInvites)
			authGroup.POST("/invite/generate", s.generateInvite)

			adminGroup := authGroup.Group("/admin")
			{
				adminGroup.GET("/classes", s.adminListClasses)
				adminGroup.GET("/users", s.adminListUsers)
				adminGroup.POST("/user/class", s.adminUpdateUserClass)
				adminGroup.GET("/authority/settings", s.adminGetAuthoritySettings)
				adminGroup.POST("/authority/settings", s.adminSaveAuthoritySettings)
			}
		}
	}
}

type ptServant struct {
	ptSrv corept.PTService
}

func (s *ptServant) listCategories(c *gin.Context) {
	resp := app.NewResponse(c)
	cats, err := s.ptSrv.ListCategories()
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(cats)
}

func (s *ptServant) listTorrents(c *gin.Context) {
	resp := app.NewResponse(c)

	categoryID, _ := strconv.Atoi(c.Query("category_id"))
	spState, _ := strconv.Atoi(c.Query("sp_state"))
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	req := &corept.ListTorrentsReq{
		Keyword:    c.Query("keyword"),
		CategoryID: categoryID,
		SpState:    spState,
		SortBy:     c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
		Page:       page,
		PageSize:   pageSize,
	}

	result, err := s.ptSrv.ListTorrents(req)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(result)
}

func (s *ptServant) getTorrent(c *gin.Context) {
	resp := app.NewResponse(c)
	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		resp.ToErrorResponse(xerror.InvalidParams.WithDetails("invalid torrent id"))
		return
	}

	torrent, err := s.ptSrv.GetTorrent(id)
	if err != nil {
		resp.ToErrorResponse(xerror.NotFound)
		return
	}

	files, _ := s.ptSrv.GetTorrentFiles(id)
	peers, _ := s.ptSrv.GetTorrentPeers(id, 50)

	resp.ToResponse(gin.H{
		"torrent": torrent,
		"files":   files,
		"peers":   peers,
	})
}

func (s *ptServant) uploadTorrent(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	userProf, err := s.ptSrv.GetUserPTProfile(user.ID)
	userClass := 0
	if err == nil && userProf != nil {
		userClass = userProf.Class
		if !authority.CanUploadTorrent(userProf.Class, user.IsAdmin) || !userProf.CanUpload {
			resp.ToErrorResponse(xerror.AccessDenied.WithDetails("您当前权限不足或处于受限期，禁止发布种子，请先保种提升分享率"))
			return
		}
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		resp.ToErrorResponse(xerror.InvalidParams.WithDetails("missing torrent file"))
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".torrent") {
		resp.ToErrorResponse(xerror.InvalidParams.WithDetails("file must be a .torrent file"))
		return
	}

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails("failed to read uploaded file"))
		return
	}

	categoryID, _ := strconv.Atoi(c.PostForm("category_id"))
	spState, _ := strconv.Atoi(c.PostForm("sp_state"))
	isAnonymous := c.PostForm("is_anonymous") == "true" || c.PostForm("is_anonymous") == "1"

	// Only designated classes (e.g. Uploader/Moderator/Admin) can assign custom promotion states upon upload
	if spState != modelpt.SpStateNormal && !authority.CanCustomPromotion(userClass, user.IsAdmin) {
		spState = modelpt.SpStateNormal
	}

	// Only designated classes can upload anonymously
	if isAnonymous && !authority.CanBeAnonymous(userClass, user.IsAdmin) {
		isAnonymous = false
	}

	req := &corept.UploadTorrentReq{
		Name:        c.PostForm("name"),
		SmallDescr:  c.PostForm("small_descr"),
		Descr:       c.PostForm("descr"),
		CategoryID:  categoryID,
		SpState:     spState,
		IsAnonymous: isAnonymous,
		Nfo:         c.PostForm("nfo"),
	}

	created, err := s.ptSrv.CreateTorrent(user.ID, req, rawBytes)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	resp.ToResponse(created)
}

func (s *ptServant) downloadTorrent(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "Invalid torrent ID")
		return
	}

	var userID int64
	passkey := c.Query("passkey")
	if passkey != "" {
		// Download using passkey parameter
		profile, err := s.ptSrv.GetUserByPasskey(passkey)
		if err != nil {
			c.String(http.StatusForbidden, "Invalid passkey")
			return
		}
		userID = profile.UserID
	} else {
		// Try bearer token or query token
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = authHeader[7:]
			}
		}
		if token != "" {
			if claims, err := app.ParseToken(token); err == nil {
				userID = claims.UID
			}
		}
	}

	if userID == 0 {
		c.String(http.StatusUnauthorized, "Authentication required to download torrent")
		return
	}

	data, filename, err := s.ptSrv.GenerateUserTorrent(id, userID)
	if err != nil {
		if errors.Is(err, jinzhupt.ErrPeasantRestricted) {
			c.String(http.StatusForbidden, "您当前处于贫民 (Peasant) 受限期，禁止下载非免费种子，请先保种或兑换上传量提升分享率！")
			return
		}
		if errors.Is(err, jinzhupt.ErrDownloadPermissionDenied) {
			c.String(http.StatusForbidden, "您当前的用户等级暂无下载种子权限")
			return
		}
		if errors.Is(err, jinzhupt.ErrUserDisabled) {
			c.String(http.StatusForbidden, "您的账号下载权限已被禁用")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Type", "application/x-bittorrent")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", encodedFilename, encodedFilename))
	c.Data(http.StatusOK, "application/x-bittorrent", data)
}

var streamerProxy = httputil.NewSingleHostReverseProxy(&url.URL{
	Scheme: "http",
	Host:   "127.0.0.1:8009",
})

func (s *ptServant) streamTorrent(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "Invalid torrent ID")
		return
	}

	var userID int64
	passkey := c.Query("passkey")
	if passkey != "" {
		profile, err := s.ptSrv.GetUserByPasskey(passkey)
		if err == nil && profile != nil {
			userID = profile.UserID
		}
	}

	if userID == 0 {
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = authHeader[7:]
			}
		}
		if token != "" {
			if claims, err := app.ParseToken(token); err == nil {
				userID = claims.UID
			}
		}
	}

	if userID == 0 {
		c.String(http.StatusUnauthorized, "Authentication required to stream torrent")
		return
	}

	t, err := s.ptSrv.GetTorrent(id)
	if err != nil {
		c.String(http.StatusNotFound, "Torrent not found")
		return
	}

	profile, err := s.ptSrv.GetUserPTProfile(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if !profile.CanLeech {
		c.String(http.StatusForbidden, "您的账号下载与播放权限已被禁用")
		return
	}
	if !authority.CanDownloadTorrent(profile.Class, false) {
		c.String(http.StatusForbidden, "您当前的用户等级暂无播放与下载种子权限")
		return
	}
	if profile.Class == modelpt.ClassPeasant && t.SpState != modelpt.SpStateFree && t.SpState != modelpt.SpState2XFree {
		c.String(http.StatusForbidden, "您当前处于贫民 (Peasant) 受限期，禁止播放非免费种子")
		return
	}

	// Forward request to local TCP-to-HTTP torrent streaming bridge
	req := c.Request.Clone(c.Request.Context())
	q := req.URL.Query()
	q.Set("file_path", t.FilePath)
	if conf.PTSetting != nil {
		q.Set("announce", fmt.Sprintf("%s?passkey=%s", conf.PTSetting.AnnounceURL, profile.Passkey))
	}
	req.URL.RawQuery = q.Encode()
	req.URL.Path = "/stream"

	c.Header("Access-Control-Allow-Origin", "*")
	streamerProxy.ServeHTTP(c.Writer, req)
}

func (s *ptServant) editTorrent(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	var form struct {
		ID             uint64 `json:"id"`
		Name           string `json:"name"`
		SmallDescr     string `json:"small_descr"`
		Descr          string `json:"descr"`
		CategoryID     int    `json:"category_id"`
		SpState        int    `json:"sp_state"`
		PromotionUntil int64  `json:"promotion_until"`
		IsAnonymous    bool   `json:"is_anonymous"`
		Nfo            string `json:"nfo"`
	}
	if err := c.ShouldBindJSON(&form); err != nil {
		resp.ToErrorResponse(xerror.InvalidParams)
		return
	}

	existing, err := s.ptSrv.GetTorrent(form.ID)
	if err != nil {
		resp.ToErrorResponse(xerror.NotFound)
		return
	}
	if existing.UserID != user.ID && !user.IsAdmin {
		resp.ToErrorResponse(xerror.AccessDenied.WithDetails("权限不足"))
		return
	}

	if form.Name != "" {
		existing.Name = form.Name
	}
	if form.SmallDescr != "" {
		existing.SmallDescr = form.SmallDescr
	}
	if form.Descr != "" {
		existing.Descr = form.Descr
	}
	if form.CategoryID > 0 {
		existing.CategoryID = form.CategoryID
	}
	if form.SpState > 0 {
		userProf, _ := s.ptSrv.GetUserPTProfile(user.ID)
		userClass := 0
		if userProf != nil {
			userClass = userProf.Class
		}
		if form.SpState == modelpt.SpStateNormal || authority.CanCustomPromotion(userClass, user.IsAdmin) {
			existing.SpState = form.SpState
		} else {
			resp.ToErrorResponse(xerror.AccessDenied.WithDetails("您当前等级无权自定义种子促销状态"))
			return
		}
	}
	existing.PromotionUntil = form.PromotionUntil
	if form.IsAnonymous {
		userProf, _ := s.ptSrv.GetUserPTProfile(user.ID)
		userClass := 0
		if userProf != nil {
			userClass = userProf.Class
		}
		if authority.CanBeAnonymous(userClass, user.IsAdmin) {
			existing.IsAnonymous = form.IsAnonymous
		}
	} else {
		existing.IsAnonymous = false
	}
	if form.Nfo != "" {
		existing.Nfo = form.Nfo
	}

	if err := s.ptSrv.UpdateTorrent(existing); err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(existing)
}

func (s *ptServant) deleteTorrent(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	var form struct {
		ID uint64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&form); err != nil || form.ID == 0 {
		resp.ToErrorResponse(xerror.InvalidParams)
		return
	}

	if err := s.ptSrv.DeleteTorrent(form.ID, user.ID, user.IsAdmin); err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(nil)
}

func (s *ptServant) getUserProfile(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	targetUID := user.ID
	if uidStr := c.Query("user_id"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil && uid > 0 {
			targetUID = uid
		}
	}

	profile, err := s.ptSrv.GetUserPTProfile(targetUID)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	if targetUID != user.ID {
		// Do not leak other user's passkey or upgrade progress
		profile.Passkey = ""
		profile.NextClass = nil
	}

	if conf.PTSetting != nil {
		if profile.Passkey != "" {
			profile.HttpTrackerURL = fmt.Sprintf("%s?passkey=%s", conf.PTSetting.AnnounceURL, profile.Passkey)
			if conf.PTSetting.WsAnnounceURL != "" {
				profile.WsTrackerURL = fmt.Sprintf("%s?passkey=%s", conf.PTSetting.WsAnnounceURL, profile.Passkey)
			}
		} else {
			profile.HttpTrackerURL = conf.PTSetting.AnnounceURL
			profile.WsTrackerURL = conf.PTSetting.WsAnnounceURL
		}
	}

	resp.ToResponse(profile)
}

func (s *ptServant) resetPasskey(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	newKey, err := s.ptSrv.ResetPasskey(user.ID)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(gin.H{"passkey": newKey})
}

func (s *ptServant) getUserSnatches(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	list, total, err := s.ptSrv.GetUserSnatches(user.ID, page, pageSize)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(gin.H{
		"total": total,
		"list":  list,
	})
}

func (s *ptServant) getBonusOptions(c *gin.Context) {
	resp := app.NewResponse(c)
	options := s.ptSrv.GetBonusOptions()
	resp.ToResponse(options)
}

func (s *ptServant) exchangeBonus(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	var form struct {
		OptionID int `json:"option_id"`
	}
	if err := c.ShouldBindJSON(&form); err != nil || form.OptionID <= 0 {
		resp.ToErrorResponse(xerror.InvalidParams)
		return
	}

	if err := s.ptSrv.ExchangeBonus(user.ID, form.OptionID); err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(nil)
}

func (s *ptServant) adminListClasses(c *gin.Context) {
	resp := app.NewResponse(c)
	type ClassOption struct {
		Class int    `json:"class"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	var list []ClassOption
	for c := 0; c <= 16; c++ {
		list = append(list, ClassOption{
			Class: c,
			Name:  authority.GetClassName(c),
			Color: authority.GetClassColor(c),
		})
	}
	resp.ToResponse(list)
}

func (s *ptServant) adminListUsers(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil || !user.IsAdmin {
		resp.ToErrorResponse(xerror.AccessDenied.WithDetails("管理员权限不足"))
		return
	}

	keyword := c.Query("keyword")
	classFilter, _ := strconv.Atoi(c.DefaultQuery("class", "-1"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	items, total, err := s.ptSrv.AdminListUsers(keyword, classFilter, page, pageSize)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(gin.H{
		"total": total,
		"list":  items,
	})
}

func (s *ptServant) adminUpdateUserClass(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil || !user.IsAdmin {
		resp.ToErrorResponse(xerror.AccessDenied.WithDetails("管理员权限不足"))
		return
	}

	var req corept.AdminUpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		resp.ToErrorResponse(xerror.InvalidParams)
		return
	}

	if err := s.ptSrv.AdminUpdateUser(&req); err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}
	resp.ToResponse(nil)
}

func (s *ptServant) adminGetAuthoritySettings(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil || !user.IsAdmin {
		resp.ToErrorResponse(xerror.AccessDenied.WithDetails("管理员权限不足"))
		return
	}

	rules := s.ptSrv.GetAuthorityRules()
	resp.ToResponse(rules)
}

func (s *ptServant) adminSaveAuthoritySettings(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil || !user.IsAdmin {
		resp.ToErrorResponse(xerror.AccessDenied.WithDetails("管理员权限不足"))
		return
	}

	var rules modelpt.PTAuthorityRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		resp.ToErrorResponse(xerror.InvalidParams.WithDetails(err.Error()))
		return
	}

	if err := s.ptSrv.SaveAuthorityRules(&rules); err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	resp.ToResponse(rules)
}

func (s *ptServant) getMyInvites(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	list, invites, err := s.ptSrv.GetMyInvites(user.ID)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	resp.ToResponse(gin.H{
		"invites": invites,
		"list":    list,
	})
}

func (s *ptServant) generateInvite(c *gin.Context) {
	resp := app.NewResponse(c)
	user, ok := base.UserFrom(c)
	if !ok || user == nil {
		resp.ToErrorResponse(xerror.UnauthorizedAuthNotExist)
		return
	}

	invite, err := s.ptSrv.GenerateInvite(user.ID)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	resp.ToResponse(invite)
}

func (s *ptServant) checkInvite(c *gin.Context) {
	resp := app.NewResponse(c)
	code := c.Query("code")
	invite, err := s.ptSrv.CheckInvite(code)
	if err != nil {
		resp.ToErrorResponse(xerror.ServerError.WithDetails(err.Error()))
		return
	}

	resp.ToResponse(gin.H{
		"code":         invite.Code,
		"inviter_name": invite.InviterName,
		"expire_at":    invite.ExpireAt,
	})
}
