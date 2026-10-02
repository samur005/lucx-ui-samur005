// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

// Per-client qWDTT credentials.
//
// The qWDTT server (v1.4.x) has one owner password (-password) plus a
// passwords.json map of extra passwords. Every device that binds with a
// password gets owner_id = hex(sha256("WDTT-WRAP-ID-v1\x00"+password)[:8]),
// and the server adds the device's up/down bytes to that password's entry
// (up_bytes / down_bytes, flushed to disk about once a minute). LucX gives each
// client its own password so that per-client online state and traffic can be
// read back. The password is derived (HMAC of the inbound's owner password and
// the client's UUID) - nothing is stored, and the master and the node compute
// the same value because inbound settings and client UUIDs are synced.
//
// Registration = edit passwords.json atomically, then SIGHUP the sidecar.
// Entries managed by LucX carry the label prefix "lucx:"; every other entry
// (manual / bot-made) is left untouched. Revocation sets expires_at in the past:
// reloadDB then drops the entry and disconnects its live sessions.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// QwdttPersonalLabelPrefix marks passwords.json entries owned by LucX.
	QwdttPersonalLabelPrefix = "lucx:"
	// QwdttPersonalMaxDevices is the per-password device slot count.
	QwdttPersonalMaxDevices = 20

	qwdttPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	qwdttPasswordLen      = 16
	qwdttRevokedExpiry    = int64(1) // unix seconds, far in the past
)

// QwdttClientKey picks the stable identity a client's password is derived from.
func QwdttClientKey(uuid, email string) string {
	if u := strings.TrimSpace(uuid); u != "" {
		return u
	}
	return strings.TrimSpace(email)
}

// QwdttClientPassword derives a client's personal password (same alphabet and
// length as the server's own generator). Empty when either input is empty.
func QwdttClientPassword(ownerPassword, clientKey string) string {
	ownerPassword = strings.TrimSpace(ownerPassword)
	clientKey = strings.TrimSpace(clientKey)
	if ownerPassword == "" || clientKey == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(ownerPassword))
	mac.Write([]byte("lucx-qwdtt-client-v1\x00" + clientKey))
	sum := mac.Sum(nil)
	out := make([]byte, qwdttPasswordLen)
	for i := range out {
		out[i] = qwdttPasswordAlphabet[int(sum[i])%len(qwdttPasswordAlphabet)]
	}
	return string(out)
}

// QwdttOwnerID mirrors the server's wrapKeyID: the owner_id every device that
// bound with this password carries in passwords.json.
func QwdttOwnerID(password string) string {
	h := sha256.Sum256([]byte("WDTT-WRAP-ID-v1\x00" + password))
	return hex.EncodeToString(h[:8])
}

// QwdttUser is one client's desired registration on a sidecar.
type QwdttUser struct {
	Email    string
	Password string
	// Active false = revoke (client disabled / expired).
	Active bool
}

var qwdttUsersMu sync.Mutex

type qwdttRawDB struct {
	top       map[string]json.RawMessage
	passwords map[string]map[string]json.RawMessage
}

func loadQwdttRawDB(path string) (*qwdttRawDB, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := &qwdttRawDB{top: map[string]json.RawMessage{}, passwords: map[string]map[string]json.RawMessage{}}
	if len(strings.TrimSpace(string(b))) == 0 {
		return d, nil
	}
	if err := json.Unmarshal(b, &d.top); err != nil {
		return nil, err
	}
	if raw, ok := d.top["passwords"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &d.passwords); err != nil {
			return nil, err
		}
	}
	if d.passwords == nil {
		d.passwords = map[string]map[string]json.RawMessage{}
	}
	return d, nil
}

func (d *qwdttRawDB) save(path string) error {
	raw, err := json.Marshal(d.passwords)
	if err != nil {
		return err
	}
	d.top["passwords"] = raw
	out, err := json.MarshalIndent(d.top, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".lucx.tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func rawString(e map[string]json.RawMessage, k string) string {
	var s string
	if raw, ok := e[k]; ok {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func rawInt(e map[string]json.RawMessage, k string) int64 {
	var n int64
	if raw, ok := e[k]; ok {
		_ = json.Unmarshal(raw, &n)
	}
	return n
}

func rawBool(e map[string]json.RawMessage, k string) bool {
	var v bool
	if raw, ok := e[k]; ok {
		_ = json.Unmarshal(raw, &v)
	}
	return v
}

func setRaw(e map[string]json.RawMessage, k string, v any) {
	raw, _ := json.Marshal(v)
	e[k] = raw
}

func qwdttManaged(e map[string]json.RawMessage) bool {
	return strings.HasPrefix(rawString(e, "label"), QwdttPersonalLabelPrefix)
}

func qwdttExpired(e map[string]json.RawMessage, now time.Time) bool {
	exp := rawInt(e, "expires_at")
	return exp > 0 && exp <= now.Unix()
}

// applyQwdttUsers merges the desired users into d and reports whether anything
// changed. revokeOrphans additionally revokes LucX-managed entries that are not
// in users at all (client deleted / detached); pass false when the user list
// could be incomplete.
func applyQwdttUsers(d *qwdttRawDB, users []QwdttUser, revokeOrphans bool, now time.Time) bool {
	changed := false
	want := make(map[string]struct{}, len(users))
	for _, u := range users {
		pw := strings.TrimSpace(u.Password)
		if pw == "" {
			continue
		}
		want[pw] = struct{}{}
		e, exists := d.passwords[pw]
		if u.Active {
			label := QwdttPersonalLabelPrefix + strings.TrimSpace(u.Email)
			if !exists {
				e = map[string]json.RawMessage{}
				setRaw(e, "label", label)
				setRaw(e, "max_devices", QwdttPersonalMaxDevices)
				setRaw(e, "expires_at", 0)
				setRaw(e, "down_bytes", 0)
				setRaw(e, "up_bytes", 0)
				d.passwords[pw] = e
				changed = true
				continue
			}
			if !qwdttManaged(e) {
				// Same password already used by a manual entry - never touch.
				continue
			}
			if rawString(e, "label") != label {
				setRaw(e, "label", label)
				changed = true
			}
			if rawInt(e, "max_devices") != QwdttPersonalMaxDevices {
				setRaw(e, "max_devices", QwdttPersonalMaxDevices)
				changed = true
			}
			if rawInt(e, "expires_at") != 0 {
				setRaw(e, "expires_at", 0)
				changed = true
			}
			if rawBool(e, "is_deactivated") {
				setRaw(e, "is_deactivated", false)
				changed = true
			}
			continue
		}
		if exists && qwdttManaged(e) && !qwdttExpired(e, now) {
			setRaw(e, "expires_at", qwdttRevokedExpiry)
			changed = true
		}
	}
	if revokeOrphans {
		for pw, e := range d.passwords {
			if _, ok := want[pw]; ok || !qwdttManaged(e) || qwdttExpired(e, now) {
				continue
			}
			setRaw(e, "expires_at", qwdttRevokedExpiry)
			changed = true
		}
	}
	return changed
}

// qwdttRegistered reports whether every active user is present (and managed)
// in the on-disk database - used to detect a lost update after SIGHUP.
func qwdttRegistered(path string, users []QwdttUser) bool {
	d, err := loadQwdttRawDB(path)
	if err != nil {
		return false
	}
	for _, u := range users {
		if !u.Active || strings.TrimSpace(u.Password) == "" {
			continue
		}
		e, ok := d.passwords[u.Password]
		if !ok || !qwdttManaged(e) || rawInt(e, "expires_at") != 0 {
			return false
		}
	}
	return true
}

// SyncQwdttUsers makes passwords.json in configDir match users and asks the
// sidecar (pid) to reload it. It is idempotent: nothing is written - and the
// sidecar is not signalled - when the file already matches. A missing file
// (sidecar never started) is not an error. The server rewrites the whole file
// from memory every ~60 s, so a write that races such a save can be lost; the
// result is verified after the reload and repeated, and the next job tick heals
// anything that still slipped through.
func SyncQwdttUsers(configDir string, pid int, users []QwdttUser, revokeOrphans bool) (bool, error) {
	return syncQwdttUsers(configDir, pid, users, revokeOrphans, qwdttReload, 1500*time.Millisecond)
}

func syncQwdttUsers(configDir string, pid int, users []QwdttUser, revokeOrphans bool, reload func(int) error, verifyDelay time.Duration) (bool, error) {
	if strings.TrimSpace(configDir) == "" {
		return false, nil
	}
	path := filepath.Join(configDir, "passwords.json")
	qwdttUsersMu.Lock()
	defer qwdttUsersMu.Unlock()
	changedAny := false
	for attempt := 0; attempt < 3; attempt++ {
		d, err := loadQwdttRawDB(path)
		if err != nil {
			if os.IsNotExist(err) {
				return changedAny, nil
			}
			return changedAny, err
		}
		if !applyQwdttUsers(d, users, revokeOrphans, time.Now()) {
			return changedAny, nil
		}
		if err := d.save(path); err != nil {
			return changedAny, err
		}
		changedAny = true
		if pid <= 0 {
			return true, nil // sidecar not running: it loads the file on start
		}
		if err := reload(pid); err != nil {
			return true, err
		}
		time.Sleep(verifyDelay)
		if qwdttRegistered(path, users) {
			return true, nil
		}
	}
	return changedAny, nil
}

// QwdttEntryStats is one password entry's cumulative counters from the
// sidecar's passwords.json (up = client -> internet).
type QwdttEntryStats struct {
	Up, Down int64
}

// ReadQwdttEntryStats returns up/down byte counters for the given passwords
// that exist in configDir/passwords.json. The server flushes them about once a
// minute.
func ReadQwdttEntryStats(configDir string, passwords []string) map[string]QwdttEntryStats {
	out := map[string]QwdttEntryStats{}
	if strings.TrimSpace(configDir) == "" {
		return out
	}
	d, err := loadQwdttRawDB(filepath.Join(configDir, "passwords.json"))
	if err != nil {
		return out
	}
	for _, pw := range passwords {
		if e, ok := d.passwords[pw]; ok {
			out[pw] = QwdttEntryStats{Up: rawInt(e, "up_bytes"), Down: rawInt(e, "down_bytes")}
		}
	}
	return out
}
