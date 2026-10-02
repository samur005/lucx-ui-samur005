// Adapted from https://github.com/ildarmaga/wdtt panel/vk_creator.go / store (GPL-3.0).
package vkcreator

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	cookiesSettingKey = "lucxVkCookies"
	sessionSettingKey = "lucxVkCallSession"
	authCookieName    = "remixsid"
	cookiesValidTTL   = 3 * time.Minute
	cookiesExpiredHint = "Cookies expired — refresh remixsid (log into vk.com and save again)."
)

// CookieEntry is one browser cookie name/value pair.
type CookieEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Session is a live (or finishing) VK call created by this panel.
type Session struct {
	JoinLink  string `json:"join_link"`
	VkHash    string `json:"vk_hash"`
	CallID    string `json:"call_id,omitempty"`
	StartedAt int64  `json:"started_at"`
	Finishing bool   `json:"finishing,omitempty"`
}

var (
	cookiesValidMu      sync.Mutex
	cookiesValidAt      time.Time
	cookiesValidOK      bool
	cookiesValidateLive = func(cookieHeader string) error {
		_, err := WebToken(cookieHeader)
		return err
	}
)

func invalidateCookiesStatusCache() {
	cookiesValidMu.Lock()
	cookiesValidAt = time.Time{}
	cookiesValidMu.Unlock()
}

func loadSetting(key string) (string, error) {
	db := database.GetDB()
	if db == nil {
		return "", fmt.Errorf("database not ready")
	}
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if database.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func saveSetting(key, value string) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not ready")
	}
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	setting.Value = value
	return db.Save(setting).Error
}

func deleteSetting(key string) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not ready")
	}
	return db.Where("key = ?", key).Delete(&model.Setting{}).Error
}

// LoadCookiesJSON returns stored cookies JSON (may be empty).
func LoadCookiesJSON() ([]byte, error) {
	v, err := loadSetting(cookiesSettingKey)
	if err != nil {
		return nil, err
	}
	return []byte(v), nil
}

// HasCookies reports whether remixsid cookies are stored (not live-validated).
func HasCookies() bool {
	data, err := LoadCookiesJSON()
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return false
	}
	return validateCookiesJSON(data) == nil
}

func validateCookiesJSON(data []byte) error {
	var cookies []CookieEntry
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("invalid cookies JSON: %w", err)
	}
	if len(cookies) == 0 {
		return fmt.Errorf("cookies file empty")
	}
	for _, c := range cookies {
		if c.Name == authCookieName && strings.TrimSpace(c.Value) != "" {
			return nil
		}
	}
	return fmt.Errorf("cookies missing %s — paste remixsid from VK", authCookieName)
}

// CookieHeader builds a Cookie request header from the store.
func CookieHeader() (string, error) {
	data, err := LoadCookiesJSON()
	if err != nil {
		return "", fmt.Errorf("cookies not found: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "", fmt.Errorf("cookies not found")
	}
	var cookies []CookieEntry
	if err := json.Unmarshal(data, &cookies); err != nil {
		return "", fmt.Errorf("invalid cookies JSON: %w", err)
	}
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("cookies empty")
	}
	return strings.Join(parts, "; "), nil
}

// SaveCookies accepts JSON array or "remixsid=…; …" cookie string.
func SaveCookies(raw []byte) error {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return fmt.Errorf("cookies empty")
	}
	s := string(raw)
	if strings.HasPrefix(s, "remixsid=") || (strings.Contains(s, ";") && !strings.HasPrefix(s, "[")) {
		return saveCookieString(s)
	}
	if err := validateCookiesJSON(raw); err != nil {
		return err
	}
	invalidateCookiesStatusCache()
	return saveSetting(cookiesSettingKey, string(raw))
}

func saveCookieString(cookieStr string) error {
	parts := strings.Split(cookieStr, ";")
	var cookies []CookieEntry
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		cookies = append(cookies, CookieEntry{Name: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])})
	}
	data, err := json.Marshal(cookies)
	if err != nil {
		return err
	}
	return SaveCookies(data)
}

// ClearCookies removes stored VK cookies and invalidates the status cache.
func ClearCookies() error {
	invalidateCookiesStatusCache()
	_ = deleteSetting(sessionSettingKey)
	return deleteSetting(cookiesSettingKey)
}

func cookiesLiveValid(cookieHeader string) error {
	cookiesValidMu.Lock()
	defer cookiesValidMu.Unlock()
	if !cookiesValidAt.IsZero() && time.Since(cookiesValidAt) < cookiesValidTTL {
		if cookiesValidOK {
			return nil
		}
		return fmt.Errorf("%s", cookiesExpiredHint)
	}
	err := cookiesValidateLive(cookieHeader)
	cookiesValidAt = time.Now()
	cookiesValidOK = err == nil
	if err != nil {
		return err
	}
	return nil
}

func isTransportErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "connection") ||
		strings.Contains(s, "network") ||
		strings.Contains(s, "temporary") ||
		strings.Contains(s, "eof") ||
		strings.Contains(s, "tls") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout")
}

func truncateErr(err error, n int) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// CookiesStatus describes stored cookie health for the panel UI.
func CookiesStatus() (ok bool, hint string, present bool, expired bool) {
	data, err := LoadCookiesJSON()
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return false, "Paste remixsid / VK cookies below, then Generate vk_hash.", false, false
	}
	if err := validateCookiesJSON(data); err != nil {
		return false, err.Error(), false, false
	}
	present = true
	cookieHeader, err := CookieHeader()
	if err != nil {
		return false, err.Error(), present, false
	}
	if err := cookiesLiveValid(cookieHeader); err != nil {
		if isTransportErr(err) {
			return false, "Cookies saved, but VK check unavailable: " + err.Error(), present, false
		}
		return false, cookiesExpiredHint + " (" + truncateErr(err, 120) + ")", present, true
	}
	return true, "VK cookies valid.", present, false
}

// CookiesTextForUI returns the cookie header string for the textarea.
func CookiesTextForUI() string {
	header, err := CookieHeader()
	if err != nil || strings.TrimSpace(header) == "" {
		return ""
	}
	return header
}

func loadSession() (*Session, error) {
	v, err := loadSetting(sessionSettingKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	var s Session
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveSession(s Session) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return saveSetting(sessionSettingKey, string(raw))
}

func clearSession() error {
	return deleteSetting(sessionSettingKey)
}
