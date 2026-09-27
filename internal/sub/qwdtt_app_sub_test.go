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
	if _, ok := buildQwdttAppSubscription([]string{"vless://a@h:1#x", "trojan://b@h:2#y"}, "T", time.Now()); ok {
		t.Fatal("links without qwdtt must fall back (ok=false)")
	}
	a := tunnel.QwdttConfig{Remark: "A", SubHost: "1.1.1.1:56000", Workers: 16, ClientPort: 9000, Password: "pa"}.ClientURI()
	b := tunnel.QwdttConfig{Remark: "B", SubHost: "2.2.2.2:56000", Workers: 8, ClientPort: 9001, Password: "pb"}.ClientURI()
	body, ok := buildQwdttAppSubscription([]string{"vless://a@h:1#x", a + "\n" + b}, "", time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
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
}

// seedQwdttAppSub seeds a master-local qWDTT inbound, a node-managed qWDTT
// inbound (empty subHost -> peer resolved from the node address) and a VLESS
// inbound, all under one subId.
func seedQwdttAppSub(t *testing.T, subId string) {
	t.Helper()
	db := database.GetDB()
	node := &model.Node{Name: "fi", Scheme: "https", Address: "13.143.132.172", Port: 7788, BasePath: "/", ApiToken: "tok", Enable: true}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	client := &model.ClientRecord{Email: "qw@e", SubID: subId, Enable: true}
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
			{Name: "NL", Peer: "2.27.201.120:56000", Hashes: "hnl", Workers: 16, Port: 9000, Password: "pnl"},
			{Name: "FI", Peer: "13.143.132.172:56000", Hashes: "hfi", Workers: 16, Port: 9000, Password: "pfi"},
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
