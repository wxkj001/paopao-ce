// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tgoss

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rocboss/paopao-ce/internal/conf"
	"github.com/rocboss/paopao-ce/pkg/telegram"
	"github.com/sirupsen/logrus"
)

var (
	_client     *telegram.Client
	_clientOnce sync.Once
	_cachePath  string
)

func getClient() *telegram.Client {
	_clientOnce.Do(func() {
		s := conf.TelegramOSSSetting
		if s == nil {
			logrus.Fatal("tgoss: TelegramOSS configuration is nil")
		}
		var err error
		_client, err = telegram.NewClient(telegram.ClientConfig{
			Endpoint:  s.Endpoint,
			Token:     s.Token,
			Proxy:     s.Proxy,
			LocalMode: s.LocalMode,
		})
		if err != nil {
			logrus.Fatalf("tgoss: create telegram client failed: %v", err)
		}

		if s.Cache.Enabled && s.Cache.SavePath != "" {
			_cachePath, err = filepath.Abs(s.Cache.SavePath)
			if err != nil {
				logrus.Fatalf("tgoss: resolve cache path failed: %v", err)
			}
			_ = os.MkdirAll(_cachePath, 0o750)
		}
	})
	return _client
}

// RouteTelegramOSS registers TelegramOSS media streaming routes
func RouteTelegramOSS(e *gin.Engine) {
	e.GET("/tgoss/tg/:token/*filename", handleMediaStream)
	logrus.Infof("register TelegramOSS route in /tgoss/tg/:token/*filename")
}

func handleMediaStream(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	s := conf.TelegramOSSSetting
	if s == nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 1. Validate signature if query parameter expired is present
	expiredStr := c.Query("expired")
	signStr := c.Query("sign")
	if expiredStr != "" && signStr != "" {
		expiredTime, err := strconv.ParseInt(expiredStr, 10, 64)
		if err != nil || time.Now().Unix() > expiredTime {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		raw := fmt.Sprintf("%s:%d", strings.TrimPrefix(c.Request.URL.Path, "/tgoss/"), expiredTime)
		h := hmac.New(sha256.New, []byte(s.SecretKey))
		h.Write([]byte(raw))
		expectedSig := hex.EncodeToString(h.Sum(nil)[:8])
		if !hmac.Equal([]byte(signStr), []byte(expectedSig)) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}

	// 2. Decode locator token
	locator, err := telegram.DecodeLocator(token, s.SecretKey)
	if err != nil {
		logrus.Debugf("tgoss: invalid locator token %s: %v", token, err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	client := getClient()

	// 3. Check local cache
	if _cachePath != "" {
		cacheFile := filepath.Join(_cachePath, locator.FileID)
		if fi, err := os.Stat(cacheFile); err == nil && !fi.IsDir() {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(c.Writer, c.Request, cacheFile)
			return
		}
	}

	// 4. Resolve file metadata from Telegram
	ctx := c.Request.Context()
	fileMeta, err := client.GetFile(ctx, locator.FileID)
	if err != nil {
		logrus.Errorf("tgoss: getFile failed for %s: %v", locator.FileID, err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// 5. If local mode and absolute path exists on local disk, serve directly
	if client.IsLocalMode() && filepath.IsAbs(fileMeta.FilePath) {
		if fi, err := os.Stat(fileMeta.FilePath); err == nil && !fi.IsDir() {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(c.Writer, c.Request, fileMeta.FilePath)
			return
		}
	}

	// 6. Download file stream
	stream, contentLength, contentType, err := client.GetFileStream(ctx, fileMeta.FilePath)
	if err != nil {
		logrus.Errorf("tgoss: getFileStream failed for %s: %v", fileMeta.FilePath, err)
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer stream.Close()

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Accept-Ranges", "bytes")
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// 7. If caching is enabled, write to cache file while serving (or write to disk and ServeFile)
	if _cachePath != "" {
		cacheFile := filepath.Join(_cachePath, locator.FileID)
		tmpFile := cacheFile + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
		f, createErr := os.Create(tmpFile)
		if createErr == nil {
			defer func() {
				f.Close()
				_ = os.Remove(tmpFile)
			}()

			// Copy stream to cache file
			if _, copyErr := io.Copy(f, stream); copyErr == nil {
				_ = f.Close()
				_ = os.Rename(tmpFile, cacheFile)
				// Serve cached file directly to handle Range requests seamlessly
				http.ServeFile(c.Writer, c.Request, cacheFile)
				return
			}
		}
	}

	// Fallback direct streaming if caching is disabled or cache write failed
	if contentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, stream)
}
