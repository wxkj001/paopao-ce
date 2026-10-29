// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rocboss/paopao-ce/pkg/telegram"
)

func TestTelegramServant(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bottest:token/sendDocument":
			resp := telegram.APIResponse[telegram.Message]{
				OK: true,
				Result: telegram.Message{
					MessageID: 101,
					Chat: telegram.Chat{
						ID: -1001234567890,
					},
					Document: &telegram.Document{
						FileID:   "file_test_id_xyz",
						FileSize: 512,
						FileName: "image.png",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/bottest:token/getFile":
			resp := telegram.APIResponse[telegram.File]{
				OK: true,
				Result: telegram.File{
					FileID:   "file_test_id_xyz",
					FilePath: "documents/image.png",
					FileSize: 512,
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/bottest:token/deleteMessage":
			resp := telegram.APIResponse[bool]{
				OK:     true,
				Result: true,
			}
			json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	client, err := telegram.NewClient(telegram.ClientConfig{
		Endpoint: mockServer.URL,
		Token:    "test:token",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	pool, err := telegram.NewChannelPool([]telegram.ChannelConfig{
		{ID: "-1001234567890", Name: "TestChannel", Weight: 1},
	}, telegram.StrategyRoundRobin)
	if err != nil {
		t.Fatalf("NewChannelPool error: %v", err)
	}

	tempDir := t.TempDir()
	domain := "http://127.0.0.1:8008/tgoss/"
	secretKey := "test-secret-key"

	srv := newTelegramServant(client, pool, domain, secretKey, tempDir)

	// 1. Test PutObject
	fileData := []byte("dummy file content")
	reader := bytes.NewReader(fileData)
	objectKey := "public/image/202610/uuid123.png"
	urlStr, err := srv.PutObject(objectKey, reader, int64(len(fileData)), "image/png", false)
	if err != nil {
		t.Fatalf("PutObject error: %v", err)
	}
	if urlStr == "" {
		t.Fatal("expected urlStr to be non-empty")
	}

	// 2. Test ObjectKey and ObjectURL
	relativeKey := srv.ObjectKey(urlStr)
	rebuiltURL := srv.ObjectURL(relativeKey)
	if rebuiltURL != urlStr {
		t.Fatalf("mismatched URL: got %s, expected %s", rebuiltURL, urlStr)
	}

	// 3. Test IsObjectExist
	exists, err := srv.IsObjectExist(relativeKey)
	if err != nil {
		t.Fatalf("IsObjectExist error: %v", err)
	}
	if !exists {
		t.Fatal("expected object to exist")
	}

	// 4. Test SignURL
	signedURL, err := srv.SignURL(relativeKey, 60)
	if err != nil {
		t.Fatalf("SignURL error: %v", err)
	}
	if signedURL == "" {
		t.Fatal("expected signedURL to be non-empty")
	}

	// 5. Test DeleteObject
	// Simulate a cached file first
	locator, _ := telegram.DecodeLocator(srv.ObjectKey(relativeKey)[3:stringsIndex(srv.ObjectKey(relativeKey)[3:], "/")+3], secretKey)
	if locator != nil {
		cacheFilePath := filepath.Join(tempDir, locator.FileID)
		_ = os.WriteFile(cacheFilePath, []byte("cached"), 0o600)
		err = srv.DeleteObject(relativeKey)
		if err != nil {
			t.Fatalf("DeleteObject error: %v", err)
		}
		if _, err = os.Stat(cacheFilePath); !os.IsNotExist(err) {
			t.Fatal("expected cached file to be removed")
		}
	}
}

func stringsIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
