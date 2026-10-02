// Adapted from https://github.com/ildarmaga/wdtt panel/vk_creator.go (GPL-3.0).
package vkcreator

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var creatorMu sync.Mutex

// StatusSnapshot is returned by GET /panel/api/tunnel/vk/status.
type StatusSnapshot struct {
	CookiesOK      bool      `json:"cookies_ok"`
	CookiesPresent bool      `json:"cookies_present"`
	CookiesExpired bool      `json:"cookies_expired"`
	CookiesHint    string    `json:"cookies_hint"`
	CookiesText    string    `json:"cookies_text"`
	Session        *Session  `json:"session,omitempty"`
	Sessions       []Session `json:"sessions"`
}

// Status builds the panel status payload.
func Status() StatusSnapshot {
	ok, hint, present, expired := CookiesStatus()
	out := StatusSnapshot{
		CookiesOK:      ok,
		CookiesPresent: present,
		CookiesExpired: expired,
		CookiesHint:    hint,
		CookiesText:    CookiesTextForUI(),
		Sessions:       []Session{},
	}
	s, err := loadSession()
	if err != nil || s == nil {
		return out
	}
	if s.Finishing {
		_ = clearSession()
		s.Finishing = true
		out.Session = s
		out.Sessions = []Session{*s}
		return out
	}
	if !CallAlive(s.JoinLink) {
		_ = clearSession()
		return out
	}
	out.Session = s
	out.Sessions = []Session{*s}
	return out
}

// CreateResult is returned after a successful calls.start.
type CreateResult struct {
	JoinLink string  `json:"join_link"`
	VkHash   string  `json:"vk_hash"`
	Session  Session `json:"session"`
}

// CreateCall creates a VK call using stored cookies and returns vk_hash.
func CreateCall() (CreateResult, error) {
	creatorMu.Lock()
	defer creatorMu.Unlock()

	var out CreateResult
	if ok, hint, _, _ := CookiesStatus(); !ok {
		return out, fmt.Errorf("%s", hint)
	}
	cookieHeader, err := CookieHeader()
	if err != nil {
		return out, err
	}
	created, err := CreateCallLink(cookieHeader)
	if err != nil {
		return out, err
	}
	hash := Normalize(created.JoinLink)
	if hash == "" {
		return out, fmt.Errorf("failed to extract vk hash from %q", created.JoinLink)
	}
	sess := Session{
		JoinLink:  created.JoinLink,
		VkHash:    hash,
		CallID:    created.CallID,
		StartedAt: time.Now().Unix(),
	}
	if err := saveSession(sess); err != nil {
		return out, fmt.Errorf("failed to save call session: %w", err)
	}
	out.JoinLink = created.JoinLink
	out.VkHash = hash
	out.Session = sess
	return out, nil
}

// GenerateHash is used by EnsureVkHashes: create a call and return bare hash.
// Returns "" when cookies are missing (caller may fall back to env/WDTT).
func GenerateHash() (string, error) {
	if !HasCookies() {
		return "", nil
	}
	res, err := CreateCall()
	if err != nil {
		return "", err
	}
	return res.VkHash, nil
}

// StopCall finishes the stored (or given) call via VK API.
func StopCall(callID string) error {
	creatorMu.Lock()
	defer creatorMu.Unlock()

	callID = strings.TrimSpace(callID)
	s, err := loadSession()
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("no active VK call session")
	}
	if callID != "" && s.CallID != callID {
		return fmt.Errorf("call not found")
	}
	needFinish := CallAlive(s.JoinLink)
	if needFinish {
		cookieHeader, err := CookieHeader()
		if err != nil {
			return err
		}
		_ = ForceFinishCall(cookieHeader, s.CallID)
	}
	s.Finishing = true
	return saveSession(*s)
}
