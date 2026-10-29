// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// RateLimitError indicates Telegram returned HTTP 429 Too Many Requests
type RateLimitError struct {
	RetryAfter int
	Message    string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("telegram rate limit reached: retry after %d seconds (%s)", e.RetryAfter, e.Message)
}

// ClientConfig holds configuration for Telegram Bot client
type ClientConfig struct {
	Endpoint  string
	Token     string
	Proxy     string
	LocalMode bool
	Timeout   time.Duration
}

// Client manages communication with Telegram Bot API
type Client struct {
	endpoint   string
	token      string
	localMode  bool
	httpClient *http.Client
}

// NewClient creates a new Telegram Bot API client
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Token == "" {
		return nil, errors.New("telegram bot token cannot be empty")
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://api.telegram.org"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid telegram proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &Client{
		endpoint:   endpoint,
		token:      cfg.Token,
		localMode:  cfg.LocalMode,
		httpClient: httpClient,
	}, nil
}

// IsLocalMode returns whether the client operates against a local telegram-bot-api in --local mode
func (c *Client) IsLocalMode() bool {
	return c.localMode
}

// SendDocument uploads a file to the specified channel
func (c *Client) SendDocument(ctx context.Context, channelID int64, filename string, reader io.Reader, size int64) (*Message, error) {
	apiURL := fmt.Sprintf("%s/bot%s/sendDocument", c.endpoint, c.token)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add chat_id field
	if err := writer.WriteField("chat_id", strconv.FormatInt(channelID, 10)); err != nil {
		return nil, fmt.Errorf("failed to write chat_id field: %w", err)
	}

	// Add file field
	part, err := writer.CreateFormFile("document", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create document part: %w", err)
	}

	if _, err = io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("failed to copy file data to request: %w", err)
	}

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram sendDocument request error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read telegram response: %w", err)
	}

	var apiResp APIResponse[Message]
	if err = json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal telegram response (status %d): %w", resp.StatusCode, err)
	}

	if !apiResp.OK {
		if resp.StatusCode == http.StatusTooManyRequests || (apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0) {
			retryAfter := 30
			if apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0 {
				retryAfter = apiResp.Parameters.RetryAfter
			}
			return nil, &RateLimitError{
				RetryAfter: retryAfter,
				Message:    apiResp.Description,
			}
		}
		return nil, fmt.Errorf("telegram API error (%d): %s", apiResp.ErrorCode, apiResp.Description)
	}

	return &apiResp.Result, nil
}

// GetFile requests metadata for a file
func (c *Client) GetFile(ctx context.Context, fileID string) (*File, error) {
	apiURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", c.endpoint, c.token, url.QueryEscape(fileID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create getFile request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram getFile request error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read getFile response: %w", err)
	}

	var apiResp APIResponse[File]
	if err = json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getFile response (status %d): %w", resp.StatusCode, err)
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("telegram getFile error (%d): %s", apiResp.ErrorCode, apiResp.Description)
	}

	return &apiResp.Result, nil
}

// GetFileDownloadURL returns the direct download URL for a file_path
func (c *Client) GetFileDownloadURL(filePath string) string {
	return fmt.Sprintf("%s/file/bot%s/%s", c.endpoint, c.token, filePath)
}

// GetFileStream retrieves the stream of a file
func (c *Client) GetFileStream(ctx context.Context, filePath string) (io.ReadCloser, int64, string, error) {
	// If in local mode and filePath is an existing absolute path on disk, open directly
	if c.localMode && filepath.IsAbs(filePath) {
		fi, err := os.Stat(filePath)
		if err == nil && !fi.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return nil, 0, "", err
			}
			return file, fi.Size(), "", nil
		}
	}

	downloadURL := c.GetFileDownloadURL(filePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to download file from telegram: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("telegram file download failed with status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	contentLength := resp.ContentLength
	return resp.Body, contentLength, contentType, nil
}

// DeleteMessage deletes a message in the channel
func (c *Client) DeleteMessage(ctx context.Context, channelID int64, messageID int64) error {
	apiURL := fmt.Sprintf("%s/bot%s/deleteMessage", c.endpoint, c.token)

	payload := map[string]any{
		"chat_id":    channelID,
		"message_id": messageID,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create deleteMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram deleteMessage error: %w", err)
	}
	defer resp.Body.Close()

	var apiResp APIResponse[bool]
	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("failed to decode deleteMessage response: %w", err)
	}

	if !apiResp.OK {
		logrus.Debugf("telegram deleteMessage returned error (%d): %s", apiResp.ErrorCode, apiResp.Description)
		return fmt.Errorf("telegram deleteMessage error (%d): %s", apiResp.ErrorCode, apiResp.Description)
	}

	return nil
}
