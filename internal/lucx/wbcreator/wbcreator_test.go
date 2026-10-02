// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
package wbcreator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func memStore(t *testing.T) map[string]string {
	t.Helper()
	m := map[string]string{}
	oldL, oldS, oldD := loadSetting, saveSetting, deleteSetting
	loadSetting = func(k string) (string, error) { return m[k], nil }
	saveSetting = func(k, v string) error { m[k] = v; return nil }
	deleteSetting = func(k string) error { delete(m, k); return nil }
	t.Cleanup(func() { loadSetting, saveSetting, deleteSetting = oldL, oldS, oldD })
	return m
}

func fakeJWT(claims map[string]any) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	b, _ := json.Marshal(claims)
	return h + "." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func TestParseRoomID(t *testing.T) {
	cases := map[string]string{
		"":                                      "",
		"  abc-123  ":                           "abc-123",
		"wbstream://abc-123/":                   "abc-123",
		"https://stream.wb.ru/room/abc-123":     "abc-123",
		"https://stream.wb.ru/room/abc-123?x=1": "abc-123",
		"https://stream.wb.ru/ru/room/abc-123/": "abc-123",
		"/abc-123/":                             "abc-123",
	}
	for in, want := range cases {
		if got := ParseRoomID(in); got != want {
			t.Errorf("ParseRoomID(%q)=%q want %q", in, got, want)
		}
	}
	if !ValidRoomID("0b7d1c7e-1f2a-4b3c-9d8e-123456789abc") || ValidRoomID("a b") || ValidRoomID("x\"y") || ValidRoomID("") {
		t.Fatal("ValidRoomID mismatch")
	}
	if JoinLink("abc") != "https://stream.wb.ru/room/abc" || JoinLink("") != "" {
		t.Fatal("JoinLink mismatch")
	}
}

func TestParseCookieInputFormats(t *testing.T) {
	jwt := fakeJWT(map[string]any{"sub": "1"})
	cases := []struct {
		name, in   string
		wantNames  []string
		wantToken  string
		wantRefVal string
	}{
		{"header", "Cookie: wbx-refresh=R1; _wbauid=42; x_wbaas_token=T", []string{"wbx-refresh", "_wbauid", "x_wbaas_token"}, "", "R1"},
		{"bare pairs", "wbx-refresh=R2;_wbauid=1", []string{"wbx-refresh", "_wbauid"}, "", "R2"},
		{"json array", `[{"name":"wbx-refresh","value":"R3","domain":".wb.ru"},{"name":"wb_access_token","value":"Bearer ` + jwt + `"}]`, []string{"wbx-refresh", "wb_access_token"}, jwt, "R3"},
		{"json object", `{"wbx-refresh":"R4"}`, []string{"wbx-refresh"}, "", "R4"},
		{"json wrapped", `{"cookies":[{"name":"wbx-refresh","value":"R5"}]}`, []string{"wbx-refresh"}, "", "R5"},
		{"bearer line", "Authorization: Bearer " + jwt, []string{"wb_access_token"}, jwt, ""},
		{"bare jwt", jwt, []string{"wb_access_token"}, jwt, ""},
		{"mixed lines", "wbx-refresh=R6; a=1\nBearer " + jwt + "\nwbx-refresh=R7", []string{"wbx-refresh", "a", "wb_access_token"}, jwt, "R7"},
		{"quoted", `wbx-refresh="R8"`, []string{"wbx-refresh"}, "", "R8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCookieInput(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, c := range got {
				names = append(names, c.Name)
			}
			if strings.Join(names, ",") != strings.Join(tc.wantNames, ",") {
				t.Fatalf("names=%v want %v", names, tc.wantNames)
			}
			if v := entryValue(got, TokenEntryName); v != tc.wantToken {
				t.Fatalf("token=%q want %q", v, tc.wantToken)
			}
			if v := entryValue(got, RefreshCookieName); v != tc.wantRefVal {
				t.Fatalf("refresh=%q want %q", v, tc.wantRefVal)
			}
			if err := validateEntries(got); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
	if _, err := ParseCookieInput("   "); err == nil {
		t.Fatal("empty input must fail")
	}
	e, _ := ParseCookieInput("remixsid=abc; foo=bar")
	if validateEntries(e) == nil {
		t.Fatal("cookies without wbx-refresh/token must fail validation")
	}
}

func TestTokenExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	fresh := fakeJWT(map[string]any{"exp": now.Add(time.Hour).Unix()})
	old := fakeJWT(map[string]any{"exp": now.Add(-time.Minute).Unix()})
	noExp := fakeJWT(map[string]any{"iat": 1})
	if tokenExpired(fresh, now) || !tokenExpired(old, now) || tokenExpired(noExp, now) || tokenExpired("opaque", now) {
		t.Fatal("tokenExpired mismatch")
	}
	if _, ok := tokenExpiry(noExp); ok {
		t.Fatal("no exp claim must report ok=false")
	}
}

func TestSaveClearStatus(t *testing.T) {
	m := memStore(t)
	if HasCredentials() {
		t.Fatal("empty store must have no credentials")
	}
	if err := SaveCookies([]byte("foo=bar")); err == nil {
		t.Fatal("save without wbx-refresh/token must fail")
	}
	if err := SaveCookies([]byte("Cookie: wbx-refresh=R; _wbauid=1")); err != nil {
		t.Fatal(err)
	}
	if !HasCredentials() {
		t.Fatal("credentials expected")
	}
	st := Status()
	if !st.CookiesOK || !st.HasRefresh || st.HasToken || len(st.CookieNames) != 2 {
		t.Fatalf("status %+v", st)
	}
	raw, _ := json.Marshal(st)
	if strings.Contains(string(raw), `"R"`) || strings.Contains(string(raw), "wbx-refresh=R") {
		t.Fatal("status must not echo cookie values")
	}
	if err := ClearCookies(); err != nil {
		t.Fatal(err)
	}
	if _, ok := m[cookiesSettingKey]; ok || HasCredentials() {
		t.Fatal("clear must drop cookies")
	}
}

type fakeWB struct {
	srv          *httptest.Server
	refreshCalls atomic.Int32
	createCalls  atomic.Int32
	goodToken    string
	rotateTo     string
	refreshFails bool
}

func newFakeWB(t *testing.T) *fakeWB {
	t.Helper()
	f := &fakeWB{goodToken: fakeJWT(map[string]any{"exp": time.Now().Add(time.Hour).Unix()})}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/auth/slide-v3", func(w http.ResponseWriter, r *http.Request) {
		f.refreshCalls.Add(1)
		if f.refreshFails || !strings.Contains(r.Header.Get("Cookie"), "wbx-refresh=") || r.Header.Get("deviceId") == "" {
			// WB answers unauthorized with HTTP 200.
			fmt.Fprint(w, `{"error":"unauthorized, request_id: x","result":12}`)
			return
		}
		if f.rotateTo != "" {
			http.SetCookie(w, &http.Cookie{Name: "wbx-refresh", Value: f.rotateTo, Path: "/"})
		}
		fmt.Fprintf(w, `{"result":0,"payload":{"access_token":%q}}`, f.goodToken)
	})
	mux.HandleFunc("POST /api-room/api/v2/room", func(w http.ResponseWriter, r *http.Request) {
		f.createCalls.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+f.goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"code":16,"message":"invalid_token"}`)
			return
		}
		fmt.Fprint(w, `{"roomId":"0b7d1c7e-1f2a-4b3c-9d8e-123456789abc"}`)
	})
	f.srv = httptest.NewServer(mux)
	oldA, oldAuth, oldO := APIBase, AuthBase, Origin
	APIBase, AuthBase, Origin = f.srv.URL, f.srv.URL, "https://stream.wb.ru"
	t.Cleanup(func() {
		f.srv.Close()
		APIBase, AuthBase, Origin = oldA, oldAuth, oldO
	})
	return f
}

func TestCreateRoomRefreshAndRotate(t *testing.T) {
	memStore(t)
	f := newFakeWB(t)
	f.rotateTo = "R-new"
	if err := SaveCookies([]byte("wbx-refresh=R-old; _wbauid=1")); err != nil {
		t.Fatal(err)
	}
	room, err := CreateRoom(7)
	if err != nil {
		t.Fatal(err)
	}
	if room.RoomID != "0b7d1c7e-1f2a-4b3c-9d8e-123456789abc" || room.InboundID != 7 || !strings.HasSuffix(room.JoinLink, "/room/"+room.RoomID) {
		t.Fatalf("room %+v", room)
	}
	entries, _ := loadEntries()
	if entryValue(entries, RefreshCookieName) != "R-new" {
		t.Fatal("rotated wbx-refresh must be persisted")
	}
	if entryValue(entries, TokenEntryName) != f.goodToken {
		t.Fatal("refreshed bearer must be cached")
	}
	// Second create reuses the cached bearer (no extra slide-v3).
	if _, err := CreateRoom(0); err != nil {
		t.Fatal(err)
	}
	if f.refreshCalls.Load() != 1 || f.createCalls.Load() != 2 {
		t.Fatalf("refresh=%d create=%d", f.refreshCalls.Load(), f.createCalls.Load())
	}
	st := Status()
	if len(st.Rooms) != 2 || st.Rooms[0].InboundID != 0 || st.Rooms[1].InboundID != 7 || st.LastOKAt == 0 {
		t.Fatalf("history %+v", st.Rooms)
	}
}

func TestCreateRoomStaleTokenFallsBackToRefresh(t *testing.T) {
	memStore(t)
	f := newFakeWB(t)
	stale := fakeJWT(map[string]any{"sub": "old"}) // no exp → tried first, WB says 401
	if err := SaveCookies([]byte("wbx-refresh=R\nBearer " + stale)); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateRoom(0); err != nil {
		t.Fatal(err)
	}
	if f.refreshCalls.Load() != 1 || f.createCalls.Load() != 2 {
		t.Fatalf("refresh=%d create=%d", f.refreshCalls.Load(), f.createCalls.Load())
	}
}

func TestCreateRoomAuthFailureMarksExpired(t *testing.T) {
	memStore(t)
	f := newFakeWB(t)
	f.refreshFails = true
	if err := SaveCookies([]byte("wbx-refresh=R")); err != nil {
		t.Fatal(err)
	}
	_, err := CreateRoom(0)
	if err == nil || !IsAuthError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
	st := Status()
	if st.CookiesOK || !st.CookiesExpired || st.LastError == "" {
		t.Fatalf("status %+v", st)
	}
	// Saving a new session clears the failure flag.
	f.refreshFails = false
	if err := SaveCookies([]byte("wbx-refresh=R2")); err != nil {
		t.Fatal(err)
	}
	if st := Status(); !st.CookiesOK || st.LastError != "" {
		t.Fatalf("status after re-save %+v", st)
	}
}

func TestCreateRoomTokenOnlyExpired(t *testing.T) {
	memStore(t)
	newFakeWB(t)
	old := fakeJWT(map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})
	if err := SaveCookies([]byte("Bearer " + old)); err != nil {
		t.Fatal(err)
	}
	if st := Status(); st.CookiesOK || !st.CookiesExpired {
		t.Fatalf("expired token-only must be flagged: %+v", st)
	}
	if _, err := CreateRoom(0); err == nil || !IsAuthError(err) {
		t.Fatalf("want auth error, got %v", err)
	}
}

func TestGuestTokenRejected(t *testing.T) {
	memStore(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":3, "message":"Guests are not allowed to create room", "details":[]}`)
	}))
	defer srv.Close()
	old := APIBase
	APIBase = srv.URL
	defer func() { APIBase = old }()
	_, err := createRoomAPI("guest")
	if err == nil || !IsAuthError(err) || !strings.Contains(err.Error(), "guest") {
		t.Fatalf("want guest auth error, got %v", err)
	}
}
