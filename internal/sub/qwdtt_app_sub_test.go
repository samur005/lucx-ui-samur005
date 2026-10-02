package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// qwdttAppUA is the exact User-Agent of SpaceNeuroX SubscriptionImport.fetch.
const qwdttAppUA = "qWDTT-Subscription/1.0"

func TestIsQwdttAppClient(t *testing.T) {
	cases := map[string]bool{
		qwdttAppUA:                   true,
		"QWDTT-SUBSCRIPTION/2.0":     true,
		"  qwdtt-subscription":       true,
		"v2rayN/6.45":                false,
		"Happ/1.0":                   false,
		"":                           false,
		"Mozilla qWDTT-Subscription": false,
		"qWDTT/1.4.4":                false,
	}
	for ua, want := range cases {
		if got := IsQwdttAppClient(ua); got != want {
			t.Errorf("IsQwdttAppClient(%q) = %v, want %v", ua, got, want)
		}
	}
}

func TestQwdttProfileFromURI_RoundTripsClientURI(t *testing.T) {
	cfg := tunnel.QwdttConfig{
		Remark: "🇫🇮 qwdtt & co", SubHost: "13.143.132.172:56000", VkHashes: "h1,h2",
		Workers: 16, ClientPort: 9000, Password: "p&ss=word",
	}
	p, ok := qwdttProfileFromURI(cfg.ClientURI())
	if !ok {
		t.Fatalf("profile not parsed from %q", cfg.ClientURI())
	}
	want := tunnel.QwdttSubProfile{Name: "🇫🇮 qwdtt & co", Peer: "13.143.132.172:56000", Hashes: "h1,h2", Workers: 16, Port: 9000, Password: "p&ss=word"}
	if p != want {
		t.Fatalf("profile = %+v, want %+v", p, want)
	}
	for _, bad := range []string{"vless://x@h:1", "wdtt://1.2.3.4:56000:56001:9000:p:h", "qwdtt://config?name=x&pass=y", ""} {
		if _, ok := qwdttProfileFromURI(bad); ok {
			t.Errorf("unexpected profile from %q", bad)
		}
	}
}

func TestBuildQwdttAppSubscription(t *testing.T) {
	if _, ok := buildQwdttAppSubscription([]string{"vless://a@h:1#x", "trojan://b@h:2#y"}, "T", xray.ClientTraffic{}, time.Now()); ok {
		t.Fatal("links without qwdtt must fall back (ok=false)")
	}
	a := tunnel.QwdttConfig{Remark: "A", SubHost: "1.1.1.1:56000", Workers: 16, ClientPort: 9000, Password: "pa"}.ClientURI()
	b := tunnel.QwdttConfig{Remark: "B", SubHost: "2.2.2.2:56000", Workers: 8, ClientPort: 9001, Password: "pb"}.ClientURI()
	body, ok := buildQwdttAppSubscription([]string{"vless://a@h:1#x", a + "\n" + b}, "", xray.ClientTraffic{}, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("want ok")
	}
	var doc tunnel.QwdttSubscription
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("json: %v (%s)", err, body)
	}
	if doc.SubscriptionName != "qWDTT" || doc.Version != 1 || doc.UpdatedAt != "2026-09-27" || len(doc.Profiles) != 2 {
		t.Fatalf("unexpected doc: %+v", doc)
	}
	if doc.Profiles[0].Peer != "1.1.1.1:56000" || doc.Profiles[1].Peer != "2.2.2.2:56000" || doc.Profiles[1].Workers != 8 || doc.Profiles[1].Port != 9001 {
		t.Fatalf("unexpected profiles: %+v", doc.Profiles)
	}
	// No traffic yet and no limit: used is still sent as 0, the limit is left
	// out (unlimited).
	if !strings.Contains(string(body), `"trafficUsedMb":0,`) {
		t.Fatalf("zero usage must be sent as trafficUsedMb 0: %s", body)
	}
	if strings.Contains(string(body), "trafficLimitMb") {
		t.Fatalf("zero limit must be omitted: %s", body)
	}
	if doc.TrafficUsedMb == nil || *doc.TrafficUsedMb != 0 || doc.TrafficLimitMb != 0 {
		t.Fatalf("unexpected traffic: used=%v limit=%v", doc.TrafficUsedMb, doc.TrafficLimitMb)
	}

	// Usage that rounds to 0.00 MiB (a few hundred bytes) is also sent as 0.
	body, ok = buildQwdttAppSubscription([]string{a}, "T", xray.ClientTraffic{Down: 672, Total: 100 * 1024 * 1024 * 1024}, time.Now())
	if !ok || !strings.Contains(string(body), `"trafficUsedMb":0,"trafficLimitMb":102400,`) {
		t.Fatalf("tiny usage must be sent as trafficUsedMb 0 with the limit: ok=%v %s", ok, body)
	}

	// Non-zero usage without a limit: the value is sent, the limit is omitted.
	body, ok = buildQwdttAppSubscription([]string{a}, "T", xray.ClientTraffic{Up: 54363, Down: 91902}, time.Now())
	if !ok || !strings.Contains(string(body), `"trafficUsedMb":0.14,`) || strings.Contains(string(body), "trafficLimitMb") {
		t.Fatalf("usage 0.14 MiB without limit: ok=%v %s", ok, body)
	}
}

// The panel-UI document (QwdttConfig.Subscription) carries no traffic, so
// neither field appears there.
func TestQwdttConfigSubscriptionJSONHasNoTraffic(t *testing.T) {
	js, err := tunnel.QwdttConfig{Remark: "A", SubHost: "1.1.1.1:56000", Workers: 16, ClientPort: 9000, Password: "pa"}.SubscriptionJSON()
	if err != nil {
		t.Fatalf("SubscriptionJSON: %v", err)
	}
	if strings.Contains(js, "trafficUsedMb") || strings.Contains(js, "trafficLimitMb") {
		t.Fatalf("panel-UI subscription must not carry traffic: %s", js)
	}
}

func TestQwdttAppTrafficMb(t *testing.T) {
	const mib = 1024 * 1024
	cases := []struct {
		name        string
		traffic     xray.ClientTraffic
		used, limit float64
	}{
		{"empty", xray.ClientTraffic{}, 0, 0},
		{"unlimited", xray.ClientTraffic{Up: 54363, Down: 91902}, 0.14, 0},
		{"100GiB limit", xray.ClientTraffic{Up: 3 * mib, Down: 7*mib + mib/2, Total: 100 * 1024 * mib}, 10.5, 102400},
		{"negative ignored", xray.ClientTraffic{Up: -5, Down: 0, Total: -1}, 0, 0},
	}
	for _, tc := range cases {
		used, limit := qwdttAppTrafficMb(tc.traffic)
		if used != tc.used || limit != tc.limit {
			t.Errorf("%s: got used=%v limit=%v, want used=%v limit=%v", tc.name, used, limit, tc.used, tc.limit)
		}
	}
}

// seedQwdttAppSub seeds a master-local qWDTT inbound, a node-managed qWDTT
// inbound (empty subHost -> peer resolved from the node address) and a VLESS
// inbound, all under one subId.
func seedQwdttAppSub(t *testing.T, subId string) {
	t.Helper()
	db := database.GetDB()
	node := &model.Node{Name: "fi", Scheme: "https", Address: "13.143.132.172", Port: 7788, BasePath: "/", ApiToken: "tok", Enable: true,
		Features: `{"nodeType":"lucx","features":["qwdtt","qwdtt-personal"]}`}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	client := &model.ClientRecord{Email: "qw@e", SubID: subId, UUID: "uuid-qw", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	nodeID := node.Id
	inbounds := []*model.Inbound{
		{
			UserId: 1, Tag: "in-56000-tcp", Enable: true, Port: 56000, Protocol: model.Qwdtt, Remark: "NL", SubSortIndex: 1,
			Settings: `{"remark":"NL","listenAddr":"0.0.0.0:56000","wgPort":56001,"password":"pnl","dns":"8.8.8.8",` +
				`"subHost":"2.27.201.120:56000","vkHashes":"hnl","workers":16,"clientPort":9000}`,
		},
		{
			UserId: 1, Tag: "n5-in-56000-tcp", Enable: true, Port: 56000, Protocol: model.Qwdtt, Remark: "FI", SubSortIndex: 1, NodeID: &nodeID,
			Settings: `{"remark":"FI","listenAddr":"0.0.0.0:56000","wgPort":56001,"password":"pfi","dns":"8.8.8.8",` +
				`"subHost":"","vkHashes":"hfi","workers":16,"clientPort":9000}`,
		},
	}
	for _, ib := range inbounds {
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("seed inbound %s: %v", ib.Tag, err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
			t.Fatalf("seed client_inbound %s: %v", ib.Tag, err)
		}
	}
	seedSubInbound(t, subId, "vless-in", 8443, 2, `{"network":"tcp","security":"none"}`)
}

func qwdttAppRouter(options ...SUBControllerOption) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSUBController(router.Group("/"), append([]SUBControllerOption{WithSUBTitle("AntiBS")}, options...)...)
	return router
}

func fetchSub(t *testing.T, router *gin.Engine, subId, ua string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/"+subId, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("UA %q: status = %d, body=%s", ua, resp.Code, resp.Body.String())
	}
	return resp
}

func TestQwdttAppSubscription_TwoInboundsTwoProfiles(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")

	for _, encrypt := range []bool{true, false} {
		resp := fetchSub(t, qwdttAppRouter(WithSUBEncryption(encrypt)), "s-qw", qwdttAppUA)
		raw := resp.Body.Bytes()
		if encrypt {
			dec, err := base64.StdEncoding.DecodeString(string(raw))
			if err != nil {
				t.Fatalf("encrypted body is not base64: %v", err)
			}
			raw = dec
		} else if ct := resp.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("plain body content-type = %q, want application/json", ct)
		}
		if !strings.HasPrefix(string(raw), "{") {
			t.Fatalf("encrypt=%v: body is not a JSON object: %s", encrypt, raw)
		}
		var doc tunnel.QwdttSubscription
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("json: %v (%s)", err, raw)
		}
		if doc.SubscriptionName != "AntiBS" || doc.Version != 1 || doc.UpdatedAt == "" {
			t.Errorf("unexpected header fields: %+v", doc)
		}
		want := []tunnel.QwdttSubProfile{
			{Name: "NL", Peer: "2.27.201.120:56000", Hashes: "hnl", Workers: 16, Port: 9000, Password: tunnel.QwdttClientPassword("pnl", "uuid-qw")},
			{Name: "FI", Peer: "13.143.132.172:56000", Hashes: "hfi", Workers: 16, Port: 9000, Password: tunnel.QwdttClientPassword("pfi", "uuid-qw")},
		}
		if len(doc.Profiles) != len(want) {
			t.Fatalf("encrypt=%v: profiles = %+v, want %d", encrypt, doc.Profiles, len(want))
		}
		for i := range want {
			if doc.Profiles[i] != want[i] {
				t.Errorf("encrypt=%v: profile[%d] = %+v, want %+v", encrypt, i, doc.Profiles[i], want[i])
			}
		}
		if resp.Header().Get("Subscription-Userinfo") == "" || resp.Header().Get("Profile-Update-Interval") == "" {
			t.Errorf("common subscription headers missing: %v", resp.Header())
		}
		if got := resp.Header().Get("Profile-Title"); got != "base64:"+base64.StdEncoding.EncodeToString([]byte("AntiBS")) {
			t.Errorf("Profile-Title = %q", got)
		}
	}
}

func TestQwdttAppSubscription_OtherUserAgentsUnchanged(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")
	router := qwdttAppRouter()

	base := fetchSub(t, router, "s-qw", "v2rayN/6.45")
	for _, ua := range []string{"", "Happ/1.0", "v2rayNG/1.8.5"} {
		if got := fetchSub(t, router, "s-qw", ua).Body.String(); got != base.Body.String() {
			t.Errorf("UA %q body differs from v2rayN body", ua)
		}
	}
	dec, err := base64.StdEncoding.DecodeString(base.Body.String())
	if err != nil {
		t.Fatalf("raw body is not base64: %v", err)
	}
	lines := splitLinkLines(string(dec))
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "qwdtt://config?") || !strings.HasPrefix(lines[1], "qwdtt://config?") || !strings.HasPrefix(lines[2], "vless://") {
		t.Fatalf("raw lines = %q", lines)
	}
	if !strings.Contains(lines[1], "peer=13.143.132.172%3A56000") {
		t.Errorf("node line must advertise the node: %q", lines[1])
	}
	if !strings.HasSuffix(string(dec), "\n") {
		t.Errorf("raw body must keep the trailing newline")
	}
}

func TestQwdttAppSubscription_NoQwdttFallsBackToRaw(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s-vl", "vless-only", 8443, 1, `{"network":"tcp","security":"none"}`)
	router := qwdttAppRouter()

	app := fetchSub(t, router, "s-vl", qwdttAppUA).Body.String()
	other := fetchSub(t, router, "s-vl", "v2rayN/6.45").Body.String()
	if app != other {
		t.Fatalf("without qWDTT inbounds the app must get the regular body\napp=%q\nraw=%q", app, other)
	}
}

func TestQwdttAppSubscription_Traffic(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")
	const mib = 1024 * 1024
	if err := database.GetDB().Create(&xray.ClientTraffic{
		Email: "qw@e", Enable: true, Up: 3 * mib, Down: 7*mib + mib/2, Total: 100 * 1024 * mib,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	resp := fetchSub(t, qwdttAppRouter(WithSUBEncryption(false)), "s-qw", qwdttAppUA)
	var doc struct {
		TrafficUsedMb  float64                  `json:"trafficUsedMb"`
		TrafficLimitMb float64                  `json:"trafficLimitMb"`
		Profiles       []tunnel.QwdttSubProfile `json:"profiles"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v (%s)", err, resp.Body.String())
	}
	if doc.TrafficUsedMb != 10.5 || doc.TrafficLimitMb != 102400 {
		t.Fatalf("traffic = %v / %v MiB, want 10.5 / 102400 (%s)", doc.TrafficUsedMb, doc.TrafficLimitMb, resp.Body.String())
	}
	if len(doc.Profiles) != 2 {
		t.Fatalf("profiles = %d, want 2", len(doc.Profiles))
	}
	// Same numbers as the Subscription-Userinfo header the other clients get.
	info := resp.Header().Get("Subscription-Userinfo")
	for _, want := range []string{"upload=3145728", "download=7864320", "total=107374182400"} {
		if !strings.Contains(info, want) {
			t.Errorf("Subscription-Userinfo %q lacks %q", info, want)
		}
	}
}

func TestQwdttAppDescription(t *testing.T) {
	// 27.10.2026 20:31 UTC is already 28.10 in UTC+7.
	exp := time.Date(2026, 10, 27, 20, 31, 0, 0, time.UTC).UnixMilli()
	cases := []struct {
		name            string
		expiry, delayed int64
		want            string
	}{
		{"date in UTC+7", exp, 0, "Подписка · до 28.10.2026"},
		{"unlimited", 0, 0, "Подписка · бессрочно"},
		{"negative expiry treated as unlimited here", -1, 0, "Подписка · бессрочно"},
		{"delayed 30 days", exp, 30 * msPerDay, "Подписка · 30 дн. с первого подключения"},
		{"delayed rounds up", 0, 30*msPerDay + 1, "Подписка · 31 дн. с первого подключения"},
	}
	for _, tc := range cases {
		if got := qwdttAppDescription(tc.expiry, tc.delayed); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func fetchSubHwid(t *testing.T, router *gin.Engine, subId, ua, hwid string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/"+subId, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if hwid != "" {
		req.Header.Set("X-HWID", hwid)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func decodeQwdttApp(t *testing.T, resp *httptest.ResponseRecorder) tunnel.QwdttSubscription {
	t.Helper()
	var doc tunnel.QwdttSubscription
	if err := json.Unmarshal(resp.Body.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v (%s)", err, resp.Body.String())
	}
	return doc
}

// A device-limited client: the qWDTT app (no X-HWID) gets its JSON, every
// other client without X-HWID keeps the 404 of the HWID gate.
func TestQwdttAppSubscription_HwidLimitNoHwid(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "qw@e").Update("limit_hwid", 3).Error; err != nil {
		t.Fatalf("set limit: %v", err)
	}
	router := qwdttAppRouter(WithSUBEncryption(false))

	resp := fetchSubHwid(t, router, "s-qw", qwdttAppUA, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("qWDTT app without X-HWID: status %d, want 200", resp.Code)
	}
	if doc := decodeQwdttApp(t, resp); len(doc.Profiles) != 2 {
		t.Fatalf("profiles = %+v, want 2", doc.Profiles)
	}
	if resp.Header().Get("Subscription-Userinfo") == "" {
		t.Errorf("Subscription-Userinfo missing")
	}
	var n int64
	database.GetDB().Model(&model.ClientHwid{}).Where("sub_id = ?", "s-qw").Count(&n)
	if n != 0 {
		t.Errorf("no device must be registered without X-HWID, got %d", n)
	}
	for _, ua := range []string{"", "Happ/1.0", "v2rayNG/1.8.5"} {
		if got := fetchSubHwid(t, router, "s-qw", ua, "").Code; got != http.StatusNotFound {
			t.Errorf("UA %q without X-HWID: status %d, want 404 (HWID gate)", ua, got)
		}
	}
	// With X-HWID the app still goes through the regular gate (registers).
	if got := fetchSubHwid(t, router, "s-qw", qwdttAppUA, "device-abcdef").Code; got != http.StatusOK {
		t.Errorf("qWDTT app with X-HWID: status %d, want 200", got)
	}
	database.GetDB().Model(&model.ClientHwid{}).Where("sub_id = ?", "s-qw").Count(&n)
	if n != 1 {
		t.Errorf("X-HWID request must register one device, got %d", n)
	}
	if got := fetchSubHwid(t, router, "unknown-sub", qwdttAppUA, "").Code; got != http.StatusNotFound {
		t.Errorf("unknown subId: status %d, want 404", got)
	}
}

// Without a qWDTT profile the no-HWID shortcut must not serve anything: the
// HWID gate still answers 404 for a device-limited client.
func TestQwdttAppSubscription_HwidLimitNoQwdttStaysGated(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s-vl", "vless-only", 8443, 1, `{"network":"tcp","security":"none"}`)
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", "s-vl").Update("limit_hwid", 2).Error; err != nil {
		t.Fatalf("set limit: %v", err)
	}
	router := qwdttAppRouter()
	if got := fetchSubHwid(t, router, "s-vl", qwdttAppUA, "").Code; got != http.StatusNotFound {
		t.Fatalf("qWDTT UA, no qWDTT profile, limited client: status %d, want 404", got)
	}
}

func TestQwdttAppSubscription_Description(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")
	exp := time.Date(2026, 10, 27, 20, 31, 0, 0, time.UTC).UnixMilli()
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: "qw@e", Enable: true, ExpiryTime: exp}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	router := qwdttAppRouter(WithSUBEncryption(false))
	resp := fetchSub(t, router, "s-qw", qwdttAppUA)
	if doc := decodeQwdttApp(t, resp); doc.Description != "Подписка · до 28.10.2026" {
		t.Fatalf("description = %q (%s)", doc.Description, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `{"subscriptionName":"AntiBS","description":"Подписка · до 28.10.2026","trafficUsedMb":0,`) {
		t.Errorf("unexpected field order: %s", resp.Body.String())
	}
	// Other User-Agents: no description anywhere in the line-based body.
	raw := fetchSub(t, router, "s-qw", "v2rayN/6.45").Body.String()
	if strings.Contains(raw, "Подписка") || strings.Contains(raw, "description") {
		t.Errorf("raw body must not change: %q", raw)
	}
}

func TestQwdttAppSubscription_DescriptionDelayedStart(t *testing.T) {
	seedSubDB(t)
	seedQwdttAppSub(t, "s-qw")
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "qw@e").Update("expiry_time", -30*msPerDay).Error; err != nil {
		t.Fatalf("set delayed expiry: %v", err)
	}
	resp := fetchSub(t, qwdttAppRouter(WithSUBEncryption(false)), "s-qw", qwdttAppUA)
	if doc := decodeQwdttApp(t, resp); doc.Description != "Подписка · 30 дн. с первого подключения" {
		t.Fatalf("description = %q", doc.Description)
	}
}
