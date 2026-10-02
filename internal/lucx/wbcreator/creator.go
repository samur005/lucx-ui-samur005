// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
package wbcreator

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// mu serialises every read-modify-write of the stored session/state.
var mu sync.Mutex

var nowFunc = time.Now

// refreshLocked runs slide-v3 with the stored cookies and persists the new
// bearer plus any rotated cookies. Caller holds mu.
func refreshLocked(entries []CookieEntry, st *State) ([]CookieEntry, string, error) {
	if entryValue(entries, RefreshCookieName) == "" {
		return entries, "", fmt.Errorf("no %s cookie stored — cannot refresh the WB token", RefreshCookieName)
	}
	if st.DeviceID == "" {
		st.DeviceID = newUUID()
	}
	res, err := refreshAccessToken(cookieHeader(entries), st.DeviceID)
	if err != nil {
		return entries, "", err
	}
	entries = mergeSetCookies(entries, res.SetCookies)
	entries = setEntry(entries, TokenEntryName, res.AccessToken)
	if err := saveEntries(entries); err != nil {
		return entries, "", err
	}
	return entries, res.AccessToken, nil
}

// CreateRoom creates a new WB Stream room with the stored session.
// Order: cached bearer (if not expired) → on auth failure refresh via
// wbx-refresh once and retry. The room is recorded in the history;
// inboundID (0 = none) is only bookkeeping.
func CreateRoom(inboundID int) (Room, error) {
	mu.Lock()
	defer mu.Unlock()

	entries, err := loadEntries()
	if err != nil {
		return Room{}, err
	}
	if err := validateEntries(entries); err != nil {
		return Room{}, fmt.Errorf("WB session not configured: %w", err)
	}
	st := loadState()
	fail := func(err error) (Room, error) {
		st.LastError = truncate(err.Error(), 300)
		st.AuthFailed = IsAuthError(err)
		_ = saveState(st)
		return Room{}, err
	}

	hasRefresh := entryValue(entries, RefreshCookieName) != ""
	tok := entryValue(entries, TokenEntryName)
	refreshed := false
	if tok == "" || tokenExpired(tok, nowFunc()) {
		if !hasRefresh {
			return fail(&APIError{Op: "token", Status: 0, Body: "stored Bearer token expired and no wbx-refresh cookie to renew it — paste a fresh session", Auth: true})
		}
		entries, tok, err = refreshLocked(entries, &st)
		if err != nil {
			return fail(err)
		}
		refreshed = true
	}

	roomID, err := createRoomAPI(tok)
	if err != nil && IsAuthError(err) && hasRefresh && !refreshed {
		entries, tok, err = refreshLocked(entries, &st)
		if err != nil {
			return fail(err)
		}
		roomID, err = createRoomAPI(tok)
	}
	if err != nil {
		return fail(err)
	}
	room := Room{
		RoomID:    roomID,
		JoinLink:  JoinLink(roomID),
		CreatedAt: nowFunc().Unix(),
		InboundID: inboundID,
	}
	st.LastOKAt = room.CreatedAt
	st.LastError = ""
	st.AuthFailed = false
	st.Rooms = append(st.Rooms, room)
	_ = saveState(st)
	return room, nil
}

// NoteRoomApplied records which inbound a room was written to.
func NoteRoomApplied(roomID string, inboundID int) {
	mu.Lock()
	defer mu.Unlock()
	st := loadState()
	for i := len(st.Rooms) - 1; i >= 0; i-- {
		if st.Rooms[i].RoomID == roomID {
			st.Rooms[i].InboundID = inboundID
			_ = saveState(st)
			return
		}
	}
}

// StatusInfo is what the panel UI shows. Secrets are never echoed back:
// only cookie names and the bearer expiry.
type StatusInfo struct {
	CookiesOK      bool     `json:"cookies_ok"`
	CookiesPresent bool     `json:"cookies_present"`
	CookiesExpired bool     `json:"cookies_expired"`
	CookiesHint    string   `json:"cookies_hint"`
	CookieNames    []string `json:"cookie_names"`
	HasRefresh     bool     `json:"has_refresh"`
	HasToken       bool     `json:"has_token"`
	TokenExp       int64    `json:"token_exp,omitempty"`
	LastOKAt       int64    `json:"last_ok_at,omitempty"`
	LastError      string   `json:"last_error,omitempty"`
	Rooms          []Room   `json:"rooms"`
}

// Status reports the stored session health without calling WB
// (slide-v3 rotates cookies, so it only runs when a room is requested).
func Status() StatusInfo {
	mu.Lock()
	defer mu.Unlock()
	st := loadState()
	out := StatusInfo{
		LastOKAt:  st.LastOKAt,
		LastError: st.LastError,
		Rooms:     make([]Room, 0, len(st.Rooms)),
	}
	for i := len(st.Rooms) - 1; i >= 0; i-- { // newest first
		out.Rooms = append(out.Rooms, st.Rooms[i])
	}
	entries, err := loadEntries()
	if err != nil {
		out.CookiesHint = err.Error()
		return out
	}
	if len(entries) == 0 {
		out.CookiesHint = "WB session not set — paste stream.wb.ru cookies (wbx-refresh) or a Bearer token."
		return out
	}
	out.CookiesPresent = true
	for _, c := range entries {
		if c.Name != TokenEntryName {
			out.CookieNames = append(out.CookieNames, c.Name)
		}
	}
	sort.Strings(out.CookieNames)
	out.HasRefresh = entryValue(entries, RefreshCookieName) != ""
	tok := entryValue(entries, TokenEntryName)
	out.HasToken = tok != ""
	tokenUsable := false
	if out.HasToken {
		if exp, ok := tokenExpiry(tok); ok {
			out.TokenExp = exp.Unix()
			tokenUsable = !tokenExpired(tok, nowFunc())
		} else {
			tokenUsable = true
		}
	}
	switch {
	case st.AuthFailed:
		out.CookiesExpired = true
		out.CookiesHint = "WB rejected the stored session — log into stream.wb.ru again and paste fresh cookies."
	case !out.HasRefresh && !tokenUsable:
		out.CookiesExpired = true
		out.CookiesHint = "Stored Bearer token expired and there is no wbx-refresh cookie — paste a fresh session."
	default:
		out.CookiesOK = true
		if out.HasRefresh {
			out.CookiesHint = "wbx-refresh stored — the token is renewed automatically when a room is created."
		} else {
			out.CookiesHint = "Only a Bearer token is stored — it cannot be renewed; paste cookies with wbx-refresh for long-term use."
		}
	}
	return out
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
