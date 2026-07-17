package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type tokenManager struct {
	config  Config
	client  *http.Client
	mu      sync.RWMutex
	token   string
	expires time.Time
}

func (m *tokenManager) start(ctx context.Context) error {
	if !m.enabled() {
		return nil
	}
	if err := m.refresh(ctx); err != nil {
		return fmt.Errorf("oauth error: failed to acquire access token: %w", err)
	}
	go m.refreshLoop()
	return nil
}
func (m *tokenManager) enabled() bool { return m.config.AuthTokenURL != "" }
func (m *tokenManager) authorization() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token != "" {
		return m.config.AuthHeaderPrefix + m.token
	}
	if m.config.AuthToken != "" {
		return m.config.AuthHeaderPrefix + m.config.AuthToken
	}
	return ""
}
func (m *tokenManager) refresh(ctx context.Context) error {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {m.config.AuthClientID}, "client_secret": {m.config.AuthClientSecret}}
	if m.config.AuthScope != "" {
		form.Set("scope", m.config.AuthScope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.config.AuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("token endpoint returned %s", resp.Status)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return fmt.Errorf("token endpoint response requires access_token and expires_in")
	}
	m.mu.Lock()
	m.token, m.expires = result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn)*time.Second)
	m.mu.Unlock()
	logSafe("OAuth token acquired or refreshed")
	return nil
}
func (m *tokenManager) refreshLoop() {
	delay := time.Second
	for {
		m.mu.RLock()
		until := time.Until(m.expires)
		m.mu.RUnlock()
		wait := until - time.Minute
		if wait < time.Second {
			wait = time.Second
		}
		time.Sleep(wait)
		if err := m.refresh(context.Background()); err != nil {
			logSafe("OAuth token refresh failed: " + err.Error())
			time.Sleep(delay)
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			continue
		}
		delay = time.Second
	}
}
