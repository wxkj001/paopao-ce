// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenEncodeDecode(t *testing.T) {
	channelID := int64(-1001234567890)
	messageID := int64(12345)
	fileID := "BQACAgIAAxkBAAICamX1abc987XYZ"
	secret := "my-secret-test-key"

	// 1. With secret
	token := EncodeLocator(channelID, messageID, fileID, secret)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	locator, err := DecodeLocator(token, secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if locator.ChannelID != channelID || locator.MessageID != messageID || locator.FileID != fileID {
		t.Fatalf("mismatched locator: got %+v", locator)
	}

	// 2. Secret mismatch should fail
	_, err = DecodeLocator(token, "wrong-secret")
	if err != ErrTokenSignature {
		t.Fatalf("expected ErrTokenSignature, got %v", err)
	}

	// 3. Without secret
	plainToken := EncodeLocator(channelID, messageID, fileID, "")
	plainLocator, err := DecodeLocator(plainToken, "")
	if err != nil {
		t.Fatalf("unexpected error decoding plain token: %v", err)
	}
	if plainLocator.ChannelID != channelID || plainLocator.MessageID != messageID || plainLocator.FileID != fileID {
		t.Fatalf("mismatched plain locator: got %+v", plainLocator)
	}
}

func TestChannelPool(t *testing.T) {
	configs := []ChannelConfig{
		{ID: "-1001111111", Name: "Channel1", Weight: 1},
		{ID: "-1002222222", Name: "Channel2", Weight: 2},
	}

	pool, err := NewChannelPool(configs, StrategyRoundRobin)
	if err != nil {
		t.Fatalf("NewChannelPool error: %v", err)
	}

	// Test Round-Robin distribution
	ch1, err := pool.PickChannel()
	if err != nil {
		t.Fatalf("PickChannel error: %v", err)
	}
	ch2, err := pool.PickChannel()
	if err != nil {
		t.Fatalf("PickChannel error: %v", err)
	}
	if ch1.ID == ch2.ID {
		t.Fatalf("expected round robin to pick different channels, got same %d", ch1.ID)
	}

	// Test Cooldown
	pool.MarkCooldown(ch1.ID, 10*time.Minute)
	chAfter, err := pool.PickChannel()
	if err != nil {
		t.Fatalf("PickChannel error: %v", err)
	}
	if chAfter.ID == ch1.ID {
		t.Fatalf("expected channel in cooldown to not be picked")
	}
}

func TestClientMock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot123456:test-token/sendDocument":
			resp := APIResponse[Message]{
				OK: true,
				Result: Message{
					MessageID: 99,
					Chat: Chat{
						ID: -1001234567890,
					},
					Document: &Document{
						FileID:   "file_abc_123",
						FileSize: 1024,
						FileName: "test.png",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/bot123456:test-token/getFile":
			resp := APIResponse[File]{
				OK: true,
				Result: File{
					FileID:   "file_abc_123",
					FilePath: "documents/file_0.png",
					FileSize: 1024,
				},
			}
			json.NewEncoder(w).Encode(resp)
		case "/bot123456:test-token/deleteMessage":
			resp := APIResponse[bool]{
				OK:     true,
				Result: true,
			}
			json.NewEncoder(w).Encode(resp)
		case "/file/bot123456:test-token/documents/file_0.png":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-png-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	client, err := NewClient(ClientConfig{
		Endpoint: mockServer.URL,
		Token:    "123456:test-token",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	ctx := context.Background()

	// 1. Test SendDocument
	fileContent := []byte("hello world image data")
	msg, err := client.SendDocument(ctx, -1001234567890, "test.png", bytes.NewReader(fileContent), int64(len(fileContent)))
	if err != nil {
		t.Fatalf("SendDocument error: %v", err)
	}
	if msg.MessageID != 99 || msg.Document == nil || msg.Document.FileID != "file_abc_123" {
		t.Fatalf("unexpected msg: %+v", msg)
	}

	// 2. Test GetFile
	file, err := client.GetFile(ctx, "file_abc_123")
	if err != nil {
		t.Fatalf("GetFile error: %v", err)
	}
	if file.FilePath != "documents/file_0.png" {
		t.Fatalf("unexpected file path: %s", file.FilePath)
	}

	// 3. Test GetFileStream
	stream, size, contentType, err := client.GetFileStream(ctx, file.FilePath)
	if err != nil {
		t.Fatalf("GetFileStream error: %v", err)
	}
	defer stream.Close()
	if contentType != "image/png" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
	if size != int64(len("fake-png-data")) {
		t.Fatalf("unexpected size: %d", size)
	}

	// 4. Test DeleteMessage
	err = client.DeleteMessage(ctx, -1001234567890, 99)
	if err != nil {
		t.Fatalf("DeleteMessage error: %v", err)
	}
}
