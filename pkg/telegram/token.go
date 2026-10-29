// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidToken   = errors.New("invalid telegram locator token")
	ErrTokenSignature = errors.New("telegram locator token signature verification failed")
)

// FileLocator contains the routing info for a Telegram-stored file
type FileLocator struct {
	ChannelID int64
	MessageID int64
	FileID    string
}

// EncodeLocator encodes channelID, messageID, and fileID into a URL-safe token.
// If secret is non-empty, a HMAC-SHA256 signature is appended to prevent tampering.
func EncodeLocator(channelID int64, messageID int64, fileID string, secret string) string {
	raw := fmt.Sprintf("%d:%d:%s", channelID, messageID, fileID)
	if secret != "" {
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(raw))
		sig := hex.EncodeToString(h.Sum(nil)[:8]) // 8 bytes = 16 hex chars
		raw = raw + ":" + sig
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeLocator decodes a URL-safe token into FileLocator.
// If secret is non-empty, it verifies the HMAC-SHA256 signature.
func DecodeLocator(token string, secret string) (*FileLocator, error) {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	parts := strings.Split(string(data), ":")
	// If secret is provided, expect 4 parts: channelID, messageID, fileID, sig
	if secret != "" {
		if len(parts) != 4 {
			return nil, ErrInvalidToken
		}
		raw := fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2])
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(raw))
		expectedSig := hex.EncodeToString(h.Sum(nil)[:8])
		if !hmac.Equal([]byte(parts[3]), []byte(expectedSig)) {
			return nil, ErrTokenSignature
		}
	} else {
		// No secret: can be 3 parts, or 4 parts (if generated with a secret)
		if len(parts) < 3 {
			return nil, ErrInvalidToken
		}
	}

	channelID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	msgID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	fileID := parts[2]
	if fileID == "" {
		return nil, ErrInvalidToken
	}

	return &FileLocator{
		ChannelID: channelID,
		MessageID: msgID,
		FileID:    fileID,
	}, nil
}
