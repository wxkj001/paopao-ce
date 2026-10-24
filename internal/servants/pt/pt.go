package pt

import (
	"fmt"
	"io"
	"net/http"
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
		v1.GET("/bonus/options", s.getBonusOptions)

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
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Type", "application/x-bittorrent")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", encodedFilename, encodedFilename))
	c.Data(http.StatusOK, "application/x-bittorrent", data)
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
		resp.ToErrorResponse(xerror.UnauthorizedTokenError.WithDetails("权限不足"))
		return
	}

	existing.Name = form.Name
	existing.SmallDescr = form.SmallDescr
	existing.Descr = form.Descr
	existing.CategoryID = form.CategoryID
	existing.SpState = form.SpState
	existing.PromotionUntil = form.PromotionUntil
	existing.IsAnonymous = form.IsAnonymous
	existing.Nfo = form.Nfo

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
		// Do not leak other user's passkey
		profile.Passkey = ""
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
