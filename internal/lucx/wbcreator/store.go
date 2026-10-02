// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
//
// Persistent state of the WB Stream room generator: pasted stream.wb.ru
// session (cookies and/or bearer) plus a short history of created rooms.
// Everything lives in the panel settings table (same pattern as vkcreator).
package wbcreator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const (
	cookiesSettingKey = "lucxWbCookies"
	stateSettingKey   = "lucxWbState"

	// RefreshCookieName is the long-lived stream.wb.ru session cookie that
	// slide-v3 exchanges for a bearer.
	RefreshCookieName = "wbx-refresh"
	// TokenEntryName stores a pasted/refreshed bearer next to the cookies
	// (same name the whitelist-bypass creator app uses in its cookie export).
	TokenEntryName = "wb_access_token"

	maxRoomHistory = 10
	tokenSkew      = 60 * time.Second
)

// CookieEntry is one browser cookie name/value pair.
type CookieEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Room is one room created by this panel.
type Room struct {
	RoomID    string `json:"room_id"`
	JoinLink  string `json:"join_link"`
	CreatedAt int64  `json:"created_at"`
	InboundID int    `json:"inbound_id,omitempty"`
}

// State is the non-secret bookkeeping of the generator.
type State struct {
	DeviceID   string `json:"device_id,omitempty"`
	LastOKAt   int64  `json:"last_ok_at,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	AuthFailed bool   `json:"auth_failed,omitempty"`
	Rooms      []Room `json:"rooms,omitempty"`
}

// Settings backend (swapped in tests).
var (
	loadSetting   = dbLoadSetting
	saveSetting   = dbSaveSetting
	deleteSetting = dbDeleteSetting
)

func dbLoadSetting(key string) (string, error) {
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

func dbSaveSetting(key, value string) error {
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

func dbDeleteSetting(key string) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not ready")
	}
	return db.Where("key = ?", key).Delete(&model.Setting{}).Error
}

var jwtRe = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*$`)

// ParseCookieInput accepts what an operator can realistically copy:
//   - a JSON array export ([{"name":…,"value":…}, …], e.g. Cookie-Editor),
//   - a JSON object {"name":"value", …} or {"cookies":[…]},
//   - a Cookie header ("Cookie: a=b; c=d" or just "a=b; c=d"),
//   - a bearer ("Authorization: Bearer eyJ…", "Bearer eyJ…" or a bare JWT),
//
// one per line, mixed freely. Later duplicates win.
func ParseCookieInput(raw string) ([]CookieEntry, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if raw == "" {
		return nil, fmt.Errorf("cookies empty")
	}
	var entries []CookieEntry
	switch {
	case strings.HasPrefix(raw, "["):
		var arr []CookieEntry
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return nil, fmt.Errorf("invalid cookies JSON: %w", err)
		}
		entries = arr
	case strings.HasPrefix(raw, "{"):
		var wrapped struct {
			Cookies []CookieEntry `json:"cookies"`
		}
		if err := json.Unmarshal([]byte(raw), &wrapped); err == nil && len(wrapped.Cookies) > 0 {
			entries = wrapped.Cookies
			break
		}
		var flat map[string]any
		if err := json.Unmarshal([]byte(raw), &flat); err != nil {
			return nil, fmt.Errorf("invalid cookies JSON: %w", err)
		}
		for k, v := range flat {
			if s, ok := v.(string); ok {
				entries = append(entries, CookieEntry{Name: k, Value: s})
			}
		}
	default:
		for _, line := range strings.Split(raw, "\n") {
			entries = append(entries, parseCookieLine(line)...)
		}
	}
	return normalizeEntries(entries), nil
}

func parseCookieLine(line string) []CookieEntry {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "authorization:"):
		line = strings.TrimSpace(line[len("authorization:"):])
		lower = strings.ToLower(line)
	case strings.HasPrefix(lower, "cookie:"):
		line = strings.TrimSpace(line[len("cookie:"):])
		lower = strings.ToLower(line)
	}
	if strings.HasPrefix(lower, "bearer ") {
		return []CookieEntry{{Name: TokenEntryName, Value: strings.TrimSpace(line[len("bearer "):])}}
	}
	if !strings.Contains(line, "=") && jwtRe.MatchString(line) {
		return []CookieEntry{{Name: TokenEntryName, Value: line}}
	}
	var out []CookieEntry
	for _, part := range strings.Split(line, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		out = append(out, CookieEntry{Name: kv[0], Value: kv[1]})
	}
	return out
}

func normalizeEntries(in []CookieEntry) []CookieEntry {
	idx := map[string]int{}
	var out []CookieEntry
	for _, c := range in {
		name := strings.TrimSpace(c.Name)
		val := strings.Trim(strings.TrimSpace(c.Value), `"`)
		if name == "" || strings.ContainsAny(name, " \t\r\n;,") {
			continue
		}
		if name == TokenEntryName {
			val = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(val, "Bearer "), "bearer "))
		}
		if i, ok := idx[name]; ok {
			out[i].Value = val
			continue
		}
		idx[name] = len(out)
		out = append(out, CookieEntry{Name: name, Value: val})
	}
	return out
}

func entryValue(entries []CookieEntry, name string) string {
	for _, c := range entries {
		if c.Name == name {
			return strings.TrimSpace(c.Value)
		}
	}
	return ""
}

func setEntry(entries []CookieEntry, name, value string) []CookieEntry {
	for i := range entries {
		if entries[i].Name == name {
			entries[i].Value = value
			return entries
		}
	}
	return append(entries, CookieEntry{Name: name, Value: value})
}

func dropEntry(entries []CookieEntry, name string) []CookieEntry {
	out := entries[:0]
	for _, c := range entries {
		if c.Name != name {
			out = append(out, c)
		}
	}
	return out
}

func validateEntries(entries []CookieEntry) error {
	if entryValue(entries, RefreshCookieName) != "" || entryValue(entries, TokenEntryName) != "" {
		return nil
	}
	if len(entries) == 0 {
		return fmt.Errorf("no cookies found in the pasted text")
	}
	return fmt.Errorf("neither %s cookie nor a Bearer token found — copy the Cookie header of a request to auth-stream.wb.ru (or the Authorization header of a stream.wb.ru API request)", RefreshCookieName)
}

// cookieHeader renders the Cookie header for WB (bearer entry excluded).
func cookieHeader(entries []CookieEntry) string {
	parts := make([]string, 0, len(entries))
	for _, c := range entries {
		if c.Name == TokenEntryName || c.Value == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// mergeSetCookies applies cookies WB rotated during slide-v3 so the next
// refresh uses the new wbx-refresh (a stale one may already be revoked).
func mergeSetCookies(entries []CookieEntry, set []*http.Cookie) []CookieEntry {
	for _, c := range set {
		if c == nil || strings.TrimSpace(c.Name) == "" || c.Name == TokenEntryName {
			continue
		}
		if c.MaxAge < 0 || c.Value == "" {
			entries = dropEntry(entries, c.Name)
			continue
		}
		entries = setEntry(entries, c.Name, c.Value)
	}
	return entries
}

// tokenExpiry decodes the exp claim of a JWT bearer without verifying it.
// ok=false when the token is not a JWT or carries no exp (WB guest tokens don't).
func tokenExpiry(tok string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(tok), ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp json.Number `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == "" {
		return time.Time{}, false
	}
	sec, err := claims.Exp.Int64()
	if err != nil || sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

func tokenExpired(tok string, now time.Time) bool {
	exp, ok := tokenExpiry(tok)
	return ok && now.Add(tokenSkew).After(exp)
}

func loadEntries() ([]CookieEntry, error) {
	v, err := loadSetting(cookiesSettingKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	var entries []CookieEntry
	if err := json.Unmarshal([]byte(v), &entries); err != nil {
		return nil, fmt.Errorf("stored WB cookies corrupt: %w", err)
	}
	return entries, nil
}

func saveEntries(entries []CookieEntry) error {
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return saveSetting(cookiesSettingKey, string(raw))
}

func loadState() State {
	var st State
	v, err := loadSetting(stateSettingKey)
	if err != nil || strings.TrimSpace(v) == "" {
		return st
	}
	_ = json.Unmarshal([]byte(v), &st)
	return st
}

func saveState(st State) error {
	if len(st.Rooms) > maxRoomHistory {
		st.Rooms = st.Rooms[len(st.Rooms)-maxRoomHistory:]
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return saveSetting(stateSettingKey, string(raw))
}

// SaveCookies parses, validates and replaces the stored WB session.
func SaveCookies(raw []byte) error {
	entries, err := ParseCookieInput(string(raw))
	if err != nil {
		return err
	}
	if err := validateEntries(entries); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := saveEntries(entries); err != nil {
		return err
	}
	st := loadState()
	st.AuthFailed = false
	st.LastError = ""
	return saveState(st)
}

// ClearCookies removes the stored WB session (room history is kept).
func ClearCookies() error {
	mu.Lock()
	defer mu.Unlock()
	st := loadState()
	st.AuthFailed = false
	st.LastError = ""
	st.DeviceID = ""
	_ = saveState(st)
	return deleteSetting(cookiesSettingKey)
}

// HasCredentials reports whether a usable-looking session is stored
// (not live-validated).
func HasCredentials() bool {
	entries, err := loadEntries()
	if err != nil {
		return false
	}
	return validateEntries(entries) == nil
}
