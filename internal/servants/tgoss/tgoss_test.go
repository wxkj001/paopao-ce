// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tgoss

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rocboss/paopao-ce/internal/conf"
	"github.com/rocboss/paopao-ce/pkg/telegram"
)

func TestTgossRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/botfake:token/getFile":
			resp := telegram.APIResponse[telegram.File]{
				OK: true,
				Result: telegram.File{
					FileID:   "tg_file_id_123",
					FilePath: "documents/sample.txt",
					FileSize: 12,
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/file/botfake:token/documents/sample.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello tgoss!"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tempDir := t.TempDir()

	secret := "test-router-secret"
	conf.TelegramOSSSetting = &conf.TelegramOSSConf{
		Endpoint:  mockServer.URL,
		Token:     "fake:token",
		SecretKey: secret,
		Cache: conf.TelegramCacheConf{
			Enabled:  true,
			SavePath: tempDir,
		},
	}

	client, err := telegram.NewClient(telegram.ClientConfig{
		Endpoint: mockServer.URL,
		Token:    "fake:token",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	_client = client
	_cachePath = tempDir

	token := telegram.EncodeLocator(-1001234567890, 42, "tg_file_id_123", secret)

	engine := gin.New()
	RouteTelegramOSS(engine)

	// 1. First request: cache miss, fetch from mock server, cache to disk
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/tgoss/tg/"+token+"/sample.txt", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "hello tgoss!" {
		t.Fatalf("expected 'hello tgoss!', got '%s'", w.Body.String())
	}

	// 2. Second request: should hit local cache directly!
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/tgoss/tg/"+token+"/sample.txt", nil)
	engine.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 from cache, got %d", w2.Code)
	}
	if w2.Body.String() != "hello tgoss!" {
		t.Fatalf("expected 'hello tgoss!' from cache, got '%s'", w2.Body.String())
	}

	// 3. Test HTTP Range request (video streaming capability)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/tgoss/tg/"+token+"/sample.txt", nil)
	req3.Header.Set("Range", "bytes=0-4")
	engine.ServeHTTP(w3, req3)

	if w3.Code != http.StatusPartialContent {
		t.Fatalf("expected status 206 Partial Content, got %d", w3.Code)
	}
	if w3.Body.String() != "hello" {
		t.Fatalf("expected partial 'hello', got '%s'", w3.Body.String())
	}
}
