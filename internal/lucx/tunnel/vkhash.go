// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
package tunnel

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/vkcreator"
)

var (
	vkHashMu    sync.Mutex
	vkHashCache string
	vkHashAt    time.Time
)

const vkHashTTL = 4 * time.Hour

// EnsureVkHashes fills VkHashes when empty:
//  1. config already set
//  2. LUCX_VK_HASH env
//  3. native VK creator (panel cookies → calls.start) when cookies configured
//  4. optional external WDTT HTTP via LUCX_WDTT_URL
func (c QwdttConfig) EnsureVkHashes() (QwdttConfig, error) {
	if strings.TrimSpace(c.VkHashes) != "" {
		return c, nil
	}
	h := strings.TrimSpace(os.Getenv("LUCX_VK_HASH"))
	if h == "" {
		h = fetchVkHash()
	}
	if h != "" {
		c.VkHashes = h
	}
	return c, nil
}

func fetchVkHash() string {
	vkHashMu.Lock()
	defer vkHashMu.Unlock()
	if vkHashCache != "" && time.Since(vkHashAt) < vkHashTTL {
		return vkHashCache
	}
	if h := strings.TrimSpace(os.Getenv("LUCX_VK_HASH")); h != "" {
		vkHashCache, vkHashAt = h, time.Now()
		return h
	}
	// Prefer native in-process generator when panel cookies are present.
	if vkcreator.HasCookies() {
		if h, err := vkcreator.GenerateHash(); err == nil && strings.TrimSpace(h) != "" {
			vkHashCache, vkHashAt = h, time.Now()
			return h
		}
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("LUCX_WDTT_URL")), "/")
	if base == "" {
		return vkHashCache
	}
	h, err := wdttCreateHash(base)
	if err != nil || h == "" {
		return vkHashCache
	}
	vkHashCache, vkHashAt = h, time.Now()
	return h
}

func wdttCreateHash(base string) (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	token := strings.TrimSpace(os.Getenv("LUCX_WDTT_TOKEN"))
	cookie := ""
	if token == "" {
		user := os.Getenv("LUCX_WDTT_USER")
		pass := os.Getenv("LUCX_WDTT_PASS")
		if user != "" {
			c, err := wdttLogin(client, base, user, pass)
			if err != nil {
				return "", err
			}
			cookie = c
		}
	}
	body, _ := json.Marshal(map[string]any{
		"password": "lucx-autogen",
		"apply":    false,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/panel/api/vk/call/create", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var wrap struct {
		Success bool `json:"success"`
		Obj     struct {
			VkHash string `json:"vk_hash"`
		} `json:"obj"`
		VkHash string `json:"vk_hash"`
	}
	_ = json.Unmarshal(raw, &wrap)
	if h := strings.TrimSpace(wrap.Obj.VkHash); h != "" {
		return h, nil
	}
	return strings.TrimSpace(wrap.VkHash), nil
}

func wdttLogin(client *http.Client, base, user, pass string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"username": user,
		"password": pass,
	})
	for _, path := range []string{"/panel/api/auth/login", "/login", "/api/login"} {
		req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		parts := resp.Header.Values("Set-Cookie")
		if len(parts) == 0 {
			continue
		}
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if i := strings.IndexByte(p, ';'); i > 0 {
				p = p[:i]
			}
			out = append(out, p)
		}
		if len(out) > 0 {
			return strings.Join(out, "; "), nil
		}
	}
	return "", nil
}
