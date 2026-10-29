// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package telegram

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ErrNoAvailableChannels = errors.New("no available telegram channels in pool")
)

type Strategy string

const (
	StrategyRoundRobin Strategy = "round-robin"
	StrategyRandom     Strategy = "random"
	StrategyWeight     Strategy = "weight"
)

// ChannelItem represents a single configured channel
type ChannelItem struct {
	ID            int64
	IDStr         string
	Name          string
	Weight        int
	cooldownUntil time.Time
}

func (c *ChannelItem) IsAvailable() bool {
	if c.cooldownUntil.IsZero() {
		return true
	}
	return time.Now().After(c.cooldownUntil)
}

func (c *ChannelItem) SetCooldown(duration time.Duration) {
	c.cooldownUntil = time.Now().Add(duration)
}

// ChannelConfig represents initial input config for a channel
type ChannelConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// ChannelPool manages a collection of Telegram channels for multi-channel storage
type ChannelPool struct {
	mu       sync.RWMutex
	channels []*ChannelItem
	strategy Strategy
	rrIndex  uint64
	rng      *rand.Rand
}

// NewChannelPool creates a new ChannelPool
func NewChannelPool(configs []ChannelConfig, strategy Strategy) (*ChannelPool, error) {
	if len(configs) == 0 {
		return nil, errors.New("telegram channel list cannot be empty")
	}

	channels := make([]*ChannelItem, 0, len(configs))
	for _, c := range configs {
		id, err := strconv.ParseInt(c.ID, 10, 64)
		if err != nil {
			return nil, errors.New("invalid telegram channel ID: " + c.ID)
		}
		weight := c.Weight
		if weight <= 0 {
			weight = 1
		}
		channels = append(channels, &ChannelItem{
			ID:     id,
			IDStr:  c.ID,
			Name:   c.Name,
			Weight: weight,
		})
	}

	if strategy == "" {
		strategy = StrategyRoundRobin
	}

	return &ChannelPool{
		channels: channels,
		strategy: strategy,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// PickChannel picks an available channel according to the configured strategy
func (p *ChannelPool) PickChannel() (*ChannelItem, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	available := make([]*ChannelItem, 0, len(p.channels))
	for _, ch := range p.channels {
		if ch.IsAvailable() {
			available = append(available, ch)
		}
	}

	if len(available) == 0 {
		// If all channels are in cooldown, pick the one with earliest cooldown expiration
		var earliest *ChannelItem
		for _, ch := range p.channels {
			if earliest == nil || ch.cooldownUntil.Before(earliest.cooldownUntil) {
				earliest = ch
			}
		}
		logrus.Warnf("all telegram channels are in cooldown, fallback to channel %s", earliest.IDStr)
		return earliest, nil
	}

	switch p.strategy {
	case StrategyRandom:
		idx := p.rng.Intn(len(available))
		return available[idx], nil

	case StrategyWeight:
		totalWeight := 0
		for _, ch := range available {
			totalWeight += ch.Weight
		}
		if totalWeight <= 0 {
			return available[0], nil
		}
		r := p.rng.Intn(totalWeight)
		curr := 0
		for _, ch := range available {
			curr += ch.Weight
			if r < curr {
				return ch, nil
			}
		}
		return available[0], nil

	case StrategyRoundRobin:
		fallthrough
	default:
		idx := atomic.AddUint64(&p.rrIndex, 1) - 1
		return available[idx%uint64(len(available))], nil
	}
}

// MarkCooldown marks a channel as cooling down (e.g. after rate limit 429)
func (p *ChannelPool) MarkCooldown(channelID int64, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ch := range p.channels {
		if ch.ID == channelID {
			ch.SetCooldown(duration)
			logrus.Warnf("channel %s (%s) set in cooldown for %v", ch.IDStr, ch.Name, duration)
			break
		}
	}
}

// ExecuteWithRetry attempts an upload operation with automatic failover across channels
func (p *ChannelPool) ExecuteWithRetry(ctx context.Context, op func(channel *ChannelItem) (*Message, error)) (*Message, *ChannelItem, error) {
	maxAttempts := len(p.channels)
	if maxAttempts > 3 {
		maxAttempts = 3
	}

	var lastErr error
	tried := make(map[int64]bool)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		ch, err := p.PickChannel()
		if err != nil {
			return nil, nil, err
		}

		if tried[ch.ID] && len(tried) < len(p.channels) {
			// Find untried channel if available
			p.mu.RLock()
			for _, c := range p.channels {
				if !tried[c.ID] && c.IsAvailable() {
					ch = c
					break
				}
			}
			p.mu.RUnlock()
		}

		tried[ch.ID] = true
		msg, err := op(ch)
		if err == nil {
			return msg, ch, nil
		}

		lastErr = err
		logrus.Warnf("telegram upload failed on channel %s (%s): %v, attempt %d/%d", ch.IDStr, ch.Name, err, attempt+1, maxAttempts)

		// Check if error is rate limit (caller can also invoke MarkCooldown)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
	}

	return nil, nil, lastErr
}
