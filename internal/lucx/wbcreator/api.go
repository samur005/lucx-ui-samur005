// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
//
// WB Stream (stream.wb.ru) room API used to auto-create rooms for the olcRTC
// tunnel (provider: wbstream). ParseRoomID and the create-room request shape
// are adapted from kulikov0/whitelist-bypass relay/wbstream/api.go
// (MIT License, Copyright (c) 2026). The full MIT
// notice is reproduced in CREDITS.md and docs/WBROOM.md.
package wbcreator

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Endpoints are variables so tests can point them at httptest servers.
var (
	APIBase  = "https://stream.wb.ru"
	AuthBase = "https://auth-stream.wb.ru"
	Origin   = "https://stream.wb.ru"
)

// UserAgent mimics desktop Chrome — the WB web app is the only official client.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"

var httpClient = &http.Client{Timeout: 15 * time.Second}

// roomIDRe bounds what we accept as a room id before it lands in YAML and
// olcrtc:// links (WB ids are UUID-like).
var roomIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// APIError is a non-success answer from a WB endpoint.
type APIError struct {
	Op     string
	Status int
	Body   string
	// Auth marks "credentials rejected" answers (expired cookies/token or a
	// guest token that may not create rooms). Transport errors are not APIError.
	Auth bool
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("%s: status %d: %s", e.Op, e.Status, body)
}

// IsAuthError reports whether err means WB rejected our credentials.
func IsAuthError(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Auth
}

// ParseRoomID accepts a bare room id, a wbstream://<id> link, or a
// https://stream.wb.ru/room/<id> URL and returns the room id.
// (Adapted from kulikov0/whitelist-bypass, MIT.)
func ParseRoomID(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(trimmed, "wbstream://"); ok {
		return strings.Trim(rest, "/")
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			for i := 0; i < len(parts)-1; i++ {
				if parts[i] == "room" && parts[i+1] != "" {
					return parts[i+1]
				}
			}
		}
	}
	return strings.Trim(trimmed, "/")
}

// ValidRoomID reports whether id is safe to persist into olcRTC settings.
func ValidRoomID(id string) bool {
	return roomIDRe.MatchString(id)
}

// JoinLink is the browser URL of a room (handy for a manual check).
func JoinLink(roomID string) string {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return ""
	}
	return Origin + "/room/" + url.PathEscape(roomID)
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	req.Header.Set("Origin", Origin)
	req.Header.Set("Referer", Origin+"/")
}

// refreshResult is a fresh bearer plus any cookies WB rotated on the way.
type refreshResult struct {
	AccessToken string
	SetCookies  []*http.Cookie
}

// refreshAccessToken exchanges the stream.wb.ru session cookies (wbx-refresh
// & co.) for a short bearer via the same call the web app makes on load.
// Note: WB answers "unauthorized" with HTTP 200 + {"error":…,"result":12},
// so the body — not the status code — decides.
func refreshAccessToken(cookieHeader, deviceID string) (refreshResult, error) {
	req, err := http.NewRequest(http.MethodPost, AuthBase+"/v2/auth/slide-v3", bytes.NewReader(nil))
	if err != nil {
		return refreshResult{}, err
	}
	setBrowserHeaders(req)
	req.Header.Set("wb-apptype", "web")
	if deviceID == "" {
		deviceID = newUUID()
	}
	req.Header.Set("deviceId", deviceID)
	req.Header.Set("X-Request-ID", newUUID())
	req.Header.Set("Cookie", cookieHeader)

	resp, err := httpClient.Do(req)
	if err != nil {
		return refreshResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return refreshResult{}, &APIError{Op: "slide-v3", Status: resp.StatusCode, Body: string(raw), Auth: true}
	}
	if resp.StatusCode != http.StatusOK {
		return refreshResult{}, &APIError{Op: "slide-v3", Status: resp.StatusCode, Body: string(raw)}
	}
	var r struct {
		Result  int    `json:"result"`
		Error   string `json:"error"`
		Payload struct {
			AccessToken      string `json:"access_token"`
			AccessTokenCamel string `json:"accessToken"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return refreshResult{}, &APIError{Op: "slide-v3", Status: resp.StatusCode, Body: "decode: " + err.Error()}
	}
	tok := strings.TrimSpace(r.Payload.AccessToken)
	if tok == "" {
		tok = strings.TrimSpace(r.Payload.AccessTokenCamel)
	}
	if r.Error != "" || tok == "" {
		msg := r.Error
		if msg == "" {
			msg = "empty access_token"
		}
		auth := r.Result != 0 || strings.Contains(strings.ToLower(msg), "unauthorized")
		return refreshResult{}, &APIError{Op: "slide-v3", Status: resp.StatusCode, Body: msg, Auth: auth}
	}
	return refreshResult{AccessToken: tok, SetCookies: resp.Cookies()}, nil
}

// createRoomAPI creates a new free, all-on-screen room with a logged-in bearer.
// Guest bearers get HTTP 400 "Guests are not allowed to create room".
func createRoomAPI(accessToken string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"roomType":    "ROOM_TYPE_ALL_ON_SCREEN",
		"roomPrivacy": "ROOM_PRIVACY_FREE",
	})
	req, err := http.NewRequest(http.MethodPost, APIBase+"/api-room/api/v2/room", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	setBrowserHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", &APIError{Op: "create-room", Status: resp.StatusCode, Body: string(raw), Auth: true}
	case resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(string(raw)), "guest"):
		return "", &APIError{Op: "create-room", Status: resp.StatusCode, Body: "guest token cannot create rooms — log in on stream.wb.ru and copy the session again", Auth: true}
	default:
		return "", &APIError{Op: "create-room", Status: resp.StatusCode, Body: string(raw)}
	}
	var r struct {
		RoomID string `json:"roomId"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", &APIError{Op: "create-room", Status: resp.StatusCode, Body: "decode: " + err.Error()}
	}
	id := strings.TrimSpace(r.RoomID)
	if !ValidRoomID(id) {
		return "", &APIError{Op: "create-room", Status: resp.StatusCode, Body: "unexpected roomId " + fmt.Sprintf("%q", id)}
	}
	return id, nil
}
