// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rocboss/paopao-ce/pkg/telegram"
	"github.com/sirupsen/logrus"
)

type telegramServant struct {
	client    *telegram.Client
	pool      *telegram.ChannelPool
	domain    string
	secretKey string
	cachePath string
}

func newTelegramServant(client *telegram.Client, pool *telegram.ChannelPool, domain string, secretKey string, cachePath string) *telegramServant {
	return &telegramServant{
		client:    client,
		pool:      pool,
		domain:    domain,
		secretKey: secretKey,
		cachePath: cachePath,
	}
}

func (s *telegramServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, _persistance bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	filename := filepath.Base(objectKey)

	// Execute upload across channel pool with automatic retry & failover
	msg, channel, err := s.pool.ExecuteWithRetry(ctx, func(ch *telegram.ChannelItem) (*telegram.Message, error) {
		// Rewind reader if seeker before each attempt
		if seeker, ok := reader.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}

		res, uploadErr := s.client.SendDocument(ctx, ch.ID, filename, reader, objectSize)
		if uploadErr != nil {
			var rateErr *telegram.RateLimitError
			if errors.As(uploadErr, &rateErr) {
				s.pool.MarkCooldown(ch.ID, time.Duration(rateErr.RetryAfter)*time.Second)
			}
			return nil, uploadErr
		}
		return res, nil
	})

	// Rewind reader back to start so subsequent callers (e.g. imaging.Decode) can read from start
	if seeker, ok := reader.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	if err != nil {
		logrus.Errorf("telegramServant.PutObject failed: %v", err)
		return "", err
	}

	fileID, _ := msg.GetPrimaryFileID()
	if fileID == "" {
		return "", errors.New("telegram response did not return a valid file ID")
	}

	token := telegram.EncodeLocator(channel.ID, msg.MessageID, fileID, s.secretKey)
	relativeKey := fmt.Sprintf("tg/%s/%s", token, filename)

	return s.domain + relativeKey, nil
}

func (s *telegramServant) PersistObject(_objectKey string) error {
	// Telegram channel messages are permanent by nature, no action required
	return nil
}

func (s *telegramServant) parseTokenFromKey(objectKey string) (string, error) {
	cleanKey := strings.TrimPrefix(objectKey, "/")
	cleanKey = strings.TrimPrefix(cleanKey, "tg/")
	parts := strings.Split(cleanKey, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", telegram.ErrInvalidToken
	}
	return parts[0], nil
}

func (s *telegramServant) DeleteObject(objectKey string) error {
	token, err := s.parseTokenFromKey(objectKey)
	if err != nil {
		return err
	}

	locator, err := telegram.DecodeLocator(token, s.secretKey)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Delete message from Telegram channel
	err = s.client.DeleteMessage(ctx, locator.ChannelID, locator.MessageID)
	if err != nil {
		logrus.Warnf("telegramServant.DeleteObject: deleteMessage error: %v", err)
	}

	// 2. Delete local cache file if exists
	if s.cachePath != "" {
		cacheFile := filepath.Join(s.cachePath, locator.FileID)
		_ = os.Remove(cacheFile)
	}

	return err
}

func (s *telegramServant) DeleteObjects(objectKeys []string) (err error) {
	for _, key := range objectKeys {
		if e := s.DeleteObject(key); e != nil {
			err = e
		}
	}
	return
}

func (s *telegramServant) IsObjectExist(objectKey string) (bool, error) {
	token, err := s.parseTokenFromKey(objectKey)
	if err != nil {
		return false, err
	}

	locator, err := telegram.DecodeLocator(token, s.secretKey)
	if err != nil {
		return false, err
	}

	// 1. Check local cache first
	if s.cachePath != "" {
		cacheFile := filepath.Join(s.cachePath, locator.FileID)
		if fi, err := os.Stat(cacheFile); err == nil && !fi.IsDir() {
			return true, nil
		}
	}

	// 2. Check with Telegram getFile
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = s.client.GetFile(ctx, locator.FileID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *telegramServant) SignURL(objectKey string, expiredInSec int64) (string, error) {
	if expiredInSec <= 0 {
		return s.ObjectURL(objectKey), nil
	}

	expireTime := time.Now().Unix() + expiredInSec
	raw := fmt.Sprintf("%s:%d", objectKey, expireTime)
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(raw))
	sig := hex.EncodeToString(h.Sum(nil)[:8])

	uri := fmt.Sprintf("%s%s?expired=%d&sign=%s", s.domain, strings.TrimPrefix(objectKey, "/"), expireTime, sig)
	return uri, nil
}

func (s *telegramServant) ObjectURL(objectKey string) string {
	return s.domain + strings.TrimPrefix(objectKey, "/")
}

func (s *telegramServant) ObjectKey(objectUrl string) string {
	return strings.TrimPrefix(objectUrl, s.domain)
}

func (s *telegramServant) Name() string {
	return "TelegramOSS"
}

func (s *telegramServant) Version() *semver.Version {
	return semver.MustParse("v0.1.0")
}
