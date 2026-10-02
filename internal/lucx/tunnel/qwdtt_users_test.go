// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQwdttClientPassword(t *testing.T) {
	a := QwdttClientPassword("owner-pass-16chr", "uuid-1")
	if len(a) != qwdttPasswordLen {
		t.Fatalf("len %d", len(a))
	}
	for _, r := range a {
		if !strings.ContainsRune(qwdttPasswordAlphabet, r) {
			t.Fatalf("char %q outside alphabet", r)
		}
	}
	if a != QwdttClientPassword("owner-pass-16chr", "uuid-1") {
		t.Fatal("not deterministic")
	}
	if a == QwdttClientPassword("owner-pass-16chr", "uuid-2") {
		t.Fatal("different clients must differ")
	}
	if a == QwdttClientPassword("other-owner-pass", "uuid-1") {
		t.Fatal("different owner passwords must differ")
	}
	if a == "owner-pass-16chr" {
		t.Fatal("personal must differ from shared")
	}
	if QwdttClientPassword("", "uuid-1") != "" || QwdttClientPassword("x", "") != "" {
		t.Fatal("empty input must give empty password")
	}
	if QwdttClientKey(" u ", "e") != "u" || QwdttClientKey("", " e ") != "e" {
		t.Fatal("client key fallback")
	}
}

// Vector produced by the server's wrapKeyID algorithm:
// hex(sha256("WDTT-WRAP-ID-v1\x00"+pw)[:8]).
func TestQwdttOwnerID(t *testing.T) {
	if got := QwdttOwnerID("abcDEF23456789xy"); got != "cd1384940e85f0b0" {
		t.Fatalf("owner id %s", got)
	}
}

func writeQwdttDB(t *testing.T, dir string, db map[string]any) string {
	t.Helper()
	b, _ := json.Marshal(db)
	p := filepath.Join(dir, "passwords.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readQwdttDB(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func entry(t *testing.T, db map[string]any, pw string) map[string]any {
	t.Helper()
	pws, _ := db["passwords"].(map[string]any)
	e, _ := pws[pw].(map[string]any)
	return e
}

func TestSyncQwdttUsers(t *testing.T) {
	dir := t.TempDir()
	p := writeQwdttDB(t, dir, map[string]any{
		"passwords": map[string]any{
			"manualPass": map[string]any{"label": "manual", "max_devices": 1, "device_ids": []string{"d1"}},
		},
		"devices": map[string]any{"d1": map[string]any{"device_id": "d1", "owner_id": "x", "up_bytes": 5}},
		"extra":   "keep-me",
	})
	reloads := 0
	reload := func(pid int) error { reloads++; return nil }
	users := []QwdttUser{
		{Email: "a@x", Password: "PassAAAAAAAAAAAA", Active: true},
		{Email: "b@x", Password: "PassBBBBBBBBBBBB", Active: true},
	}

	changed, err := syncQwdttUsers(dir, 42, users, true, reload, 0)
	if err != nil || !changed || reloads != 1 {
		t.Fatalf("first sync changed=%v err=%v reloads=%d", changed, err, reloads)
	}
	db := readQwdttDB(t, p)
	if e := entry(t, db, "PassAAAAAAAAAAAA"); e["label"] != "lucx:a@x" || e["max_devices"].(float64) != QwdttPersonalMaxDevices || e["expires_at"].(float64) != 0 {
		t.Fatalf("entry a = %v", e)
	}
	if e := entry(t, db, "manualPass"); e["label"] != "manual" || e["max_devices"].(float64) != 1 {
		t.Fatalf("manual entry touched: %v", e)
	}
	if db["extra"] != "keep-me" || db["devices"] == nil {
		t.Fatalf("unknown top-level keys must survive: %v", db)
	}

	// Idempotent: no write, no signal.
	changed, err = syncQwdttUsers(dir, 42, users, true, reload, 0)
	if err != nil || changed || reloads != 1 {
		t.Fatalf("second sync changed=%v err=%v reloads=%d", changed, err, reloads)
	}

	// Server-owned fields (counters, bound devices) survive a metadata update.
	db = readQwdttDB(t, p)
	e := entry(t, db, "PassAAAAAAAAAAAA")
	e["up_bytes"], e["down_bytes"], e["device_ids"] = 111, 222, []string{"dev-1"}
	e["max_devices"] = 1
	writeQwdttDB(t, dir, db)
	if changed, _ = syncQwdttUsers(dir, 42, users, true, reload, 0); !changed {
		t.Fatal("max_devices drift must be corrected")
	}
	e = entry(t, readQwdttDB(t, p), "PassAAAAAAAAAAAA")
	if e["up_bytes"].(float64) != 111 || e["down_bytes"].(float64) != 222 || e["max_devices"].(float64) != QwdttPersonalMaxDevices {
		t.Fatalf("counters lost or limit not restored: %v", e)
	}
	if ids, _ := e["device_ids"].([]any); len(ids) != 1 {
		t.Fatalf("device_ids lost: %v", e)
	}

	// Disabling a client revokes (expires_at in the past) but keeps the entry
	// for the server to drop and disconnect on reload.
	users[1].Active = false
	if changed, _ = syncQwdttUsers(dir, 42, users, true, reload, 0); !changed {
		t.Fatal("revoke must change the file")
	}
	if e := entry(t, readQwdttDB(t, p), "PassBBBBBBBBBBBB"); e["expires_at"].(float64) != float64(qwdttRevokedExpiry) {
		t.Fatalf("not revoked: %v", e)
	}
	if changed, _ = syncQwdttUsers(dir, 42, users, true, reload, 0); changed {
		t.Fatal("already revoked: no change expected")
	}
	// Re-enable restores it.
	users[1].Active = true
	if changed, _ = syncQwdttUsers(dir, 42, users, true, reload, 0); !changed {
		t.Fatal("re-enable must change the file")
	}
	if e := entry(t, readQwdttDB(t, p), "PassBBBBBBBBBBBB"); e["expires_at"].(float64) != 0 {
		t.Fatalf("not restored: %v", e)
	}

	// Orphans (client detached/deleted) are revoked only when the list is
	// authoritative; manual entries are never orphans.
	if changed, _ = syncQwdttUsers(dir, 42, users[:1], false, reload, 0); changed {
		t.Fatal("non-authoritative list must not revoke")
	}
	if changed, _ = syncQwdttUsers(dir, 42, users[:1], true, reload, 0); !changed {
		t.Fatal("orphan must be revoked")
	}
	db = readQwdttDB(t, p)
	if entry(t, db, "PassBBBBBBBBBBBB")["expires_at"].(float64) != float64(qwdttRevokedExpiry) {
		t.Fatal("orphan not revoked")
	}
	if entry(t, db, "manualPass")["label"] != "manual" || entry(t, db, "manualPass")["expires_at"] != nil {
		t.Fatal("manual entry must stay untouched")
	}
}

func TestSyncQwdttUsersMissingFileAndNoPid(t *testing.T) {
	dir := t.TempDir()
	users := []QwdttUser{{Email: "a@x", Password: "PassAAAAAAAAAAAA", Active: true}}
	if changed, err := syncQwdttUsers(dir, 42, users, true, func(int) error { t.Fatal("reload"); return nil }, 0); err != nil || changed {
		t.Fatalf("missing file: changed=%v err=%v", changed, err)
	}
	writeQwdttDB(t, dir, map[string]any{"passwords": map[string]any{}, "devices": map[string]any{}})
	if changed, err := syncQwdttUsers(dir, 0, users, true, func(int) error { t.Fatal("reload without pid"); return nil }, 0); err != nil || !changed {
		t.Fatalf("no pid: changed=%v err=%v", changed, err)
	}
}

// The server rewrites passwords.json from memory every minute: a write that
// races that save is lost; the post-reload verification must redo it.
func TestSyncQwdttUsersRetriesLostUpdate(t *testing.T) {
	dir := t.TempDir()
	p := writeQwdttDB(t, dir, map[string]any{"passwords": map[string]any{}, "devices": map[string]any{}})
	users := []QwdttUser{{Email: "a@x", Password: "PassAAAAAAAAAAAA", Active: true}}
	calls := 0
	reload := func(int) error {
		calls++
		if calls == 1 { // server save clobbers our edit
			writeQwdttDB(t, dir, map[string]any{"passwords": map[string]any{}, "devices": map[string]any{}})
		}
		return nil
	}
	changed, err := syncQwdttUsers(dir, 7, users, true, reload, 0)
	if err != nil || !changed || calls != 2 {
		t.Fatalf("changed=%v err=%v reload calls=%d", changed, err, calls)
	}
	if entry(t, readQwdttDB(t, p), "PassAAAAAAAAAAAA") == nil {
		t.Fatal("entry missing after retry")
	}
}

func TestReadQwdttEntryStats(t *testing.T) {
	dir := t.TempDir()
	writeQwdttDB(t, dir, map[string]any{"passwords": map[string]any{
		"P1": map[string]any{"up_bytes": 10, "down_bytes": 20},
		"P2": map[string]any{},
	}})
	got := ReadQwdttEntryStats(dir, []string{"P1", "P2", "P3"})
	if got["P1"] != (QwdttEntryStats{Up: 10, Down: 20}) || got["P2"] != (QwdttEntryStats{}) {
		t.Fatalf("stats %v", got)
	}
	if _, ok := got["P3"]; ok {
		t.Fatal("unregistered password must be absent")
	}
	if len(ReadQwdttEntryStats(filepath.Join(dir, "nope"), []string{"P1"})) != 0 {
		t.Fatal("missing dir")
	}
}

func TestFoldQwdttClients(t *testing.T) {
	m := newManager()
	byPw := map[string]string{"P1": "a@x", "P2": "b@x", "P3": "c@x"}
	pws := []string{"P1", "P2", "P3"}
	t0 := time.Unix(5000, 0)
	rows := m.foldQwdttClients("tag", pws, byPw, map[string]QwdttEntryStats{"P1": {Up: 100, Down: 200}, "P2": {Up: 5, Down: 5}}, t0)
	if len(rows) != 2 || rows[0].Up != 0 || rows[0].Online {
		t.Fatalf("baseline scrape must not count or mark online: %+v", rows)
	}
	t1 := t0.Add(10 * time.Second)
	rows = m.foldQwdttClients("tag", pws, byPw, map[string]QwdttEntryStats{
		"P1": {Up: 150, Down: 260}, "P2": {Up: 5, Down: 5}, "P3": {Up: 1, Down: 1},
	}, t1)
	if len(rows) != 3 {
		t.Fatalf("rows %+v", rows)
	}
	if r := rows[0]; r.Email != "a@x" || r.Up != 50 || r.Down != 60 || !r.Online {
		t.Fatalf("a: %+v", r)
	}
	if r := rows[1]; r.Email != "b@x" || r.Up != 0 || r.Online {
		t.Fatalf("idle b must not be online: %+v", r)
	}
	// Online persists for the grace window after the last byte movement...
	t2 := t1.Add(SidecarOnlineGrace - time.Second)
	rows = m.foldQwdttClients("tag", pws, byPw, map[string]QwdttEntryStats{"P1": {Up: 150, Down: 260}}, t2)
	if !rows[0].Online || rows[0].Up != 0 {
		t.Fatalf("grace: %+v", rows[0])
	}
	// ...then drops. A counter reset (sidecar reload lost unsaved bytes) must
	// neither go negative nor double count afterwards.
	t3 := t1.Add(SidecarOnlineGrace + time.Second)
	rows = m.foldQwdttClients("tag", pws, byPw, map[string]QwdttEntryStats{"P1": {Up: 120, Down: 250}}, t3)
	if rows[0].Online || rows[0].Up != 0 || rows[0].Down != 0 {
		t.Fatalf("after grace: %+v", rows[0])
	}
	rows = m.foldQwdttClients("tag", pws, byPw, map[string]QwdttEntryStats{"P1": {Up: 130, Down: 250}}, t3.Add(10*time.Second))
	if rows[0].Up != 10 || !rows[0].Online {
		t.Fatalf("resume after reset: %+v", rows[0])
	}
}
