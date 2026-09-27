package sub

// LUCX-HOOK: multi-profile subscription for the qWDTT Android app
// (SpaceNeuroX/proxy-turn-vk-android). The app fetches subscriptions with
// User-Agent "qWDTT-Subscription/1.0" and its SubscriptionImport.parsePayload
// treats a (base64-decoded) body that starts with "qwdtt://config" as ONE URI:
// with several qWDTT inbounds (master + nodes) only the first line becomes a
// profile and the rest is swallowed into its last query parameter. The same
// parser accepts a JSON document {subscriptionName, profiles:[...]} (plain or
// base64), so for that User-Agent we re-emit the already generated qwdtt://
// links as that JSON. Every other User-Agent keeps the line-based body.
//
// Traffic: the app ignores response headers (Subscription-Userinfo) and only
// reads the top-level trafficUsedMb / trafficLimitMb numbers (MiB) of the JSON
// for its subscription card, so the client's usage is emitted there too.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// qwdttAppUserAgentPrefix is the lower-cased User-Agent prefix sent by the
// qWDTT Android app when it fetches a subscription URL.
const qwdttAppUserAgentPrefix = "qwdtt-subscription"

// IsQwdttAppClient reports whether the request comes from the qWDTT Android
// app's subscription fetcher (case-insensitive prefix match).
func IsQwdttAppClient(userAgent string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(userAgent)), qwdttAppUserAgentPrefix)
}

// qwdttProfileFromURI converts one qwdtt://config?... share link (as built by
// tunnel.QwdttConfig.ClientURI) into an app profile. Non-qWDTT lines and links
// without a peer are rejected.
func qwdttProfileFromURI(line string) (tunnel.QwdttSubProfile, bool) {
	line = strings.TrimSpace(line)
	var rawQuery string
	switch {
	case strings.HasPrefix(line, "qwdtt://config?"):
		rawQuery = strings.TrimPrefix(line, "qwdtt://config?")
	case strings.HasPrefix(line, "qwdtt:config?"):
		rawQuery = strings.TrimPrefix(line, "qwdtt:config?")
	default:
		return tunnel.QwdttSubProfile{}, false
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return tunnel.QwdttSubProfile{}, false
	}
	peer := strings.TrimSpace(q.Get("peer"))
	if peer == "" {
		return tunnel.QwdttSubProfile{}, false
	}
	pass := q.Get("pass")
	if pass == "" {
		pass = q.Get("password")
	}
	name := q.Get("name")
	if strings.TrimSpace(name) == "" {
		name = "qWDTT"
	}
	workers, err := strconv.Atoi(q.Get("workers"))
	if err != nil {
		workers = 0
	}
	port, err := strconv.Atoi(q.Get("port"))
	if err != nil {
		port = 0
	}
	return tunnel.QwdttSubProfile{
		Name:     name,
		Peer:     peer,
		Hashes:   q.Get("hashes"),
		Workers:  workers,
		Port:     port,
		Password: pass,
	}, true
}

// qwdttAppProfiles collects one profile per qwdtt:// line of the generated
// subscription links, preserving their order (sub_sort_index, id).
func qwdttAppProfiles(links []string) []tunnel.QwdttSubProfile {
	var profiles []tunnel.QwdttSubProfile
	for _, link := range links {
		for _, line := range splitLinkLines(link) {
			if p, ok := qwdttProfileFromURI(line); ok {
				profiles = append(profiles, p)
			}
		}
	}
	return profiles
}

// bytesPerMiB converts the client's byte counters to the app's "Mb" unit (the
// app renders values >= 1024 as GB = value/1024).
const bytesPerMiB = 1024 * 1024

// qwdttAppTrafficMb returns the client's used (up+down) and limit traffic in
// MiB, rounded to 0.01. A zero limit means unlimited and is left out.
func qwdttAppTrafficMb(traffic xray.ClientTraffic) (used, limit float64) {
	toMb := func(b int64) float64 {
		if b <= 0 {
			return 0
		}
		return math.Round(float64(b)/bytesPerMiB*100) / 100
	}
	return toMb(traffic.Up + traffic.Down), toMb(traffic.Total)
}

// buildQwdttAppSubscription renders the app's subscription JSON for the given
// links. ok is false when the links contain no qWDTT profile, in which case the
// caller must fall back to the regular line-based body. traffic is the same
// aggregate the Subscription-Userinfo header is built from.
func buildQwdttAppSubscription(links []string, title string, traffic xray.ClientTraffic, now time.Time) ([]byte, bool) {
	profiles := qwdttAppProfiles(links)
	if len(profiles) == 0 {
		return nil, false
	}
	name := strings.TrimSpace(title)
	if name == "" {
		name = "qWDTT"
	}
	used, limit := qwdttAppTrafficMb(traffic)
	doc := tunnel.QwdttSubscription{
		SubscriptionName: name,
		TrafficUsedMb:    used,
		TrafficLimitMb:   limit,
		Version:          1,
		UpdatedAt:        now.Format("2006-01-02"),
		Profiles:         profiles,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, false
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), true
}

// serveQwdttAppSubscription answers a qWDTT-app request with the multi-profile
// JSON (base64 when subEncrypt is on — the app decodes it before parsing).
// Headers are the same as the raw branch. Returns false, without writing
// anything, when the subscription has no qWDTT profile.
func (a *SUBController) serveQwdttAppSubscription(c *gin.Context, subReq *SubService, subId string, links []string, traffic xray.ClientTraffic, scheme, hostWithPort string) bool {
	metadata := a.metadataForSubRequest(func() *SubService { return subReq }, subId, builtinProfileURL(c, scheme, hostWithPort))
	title := metadata.Title
	if strings.TrimSpace(title) == "" {
		title = a.subTitle
	}
	body, ok := buildQwdttAppSubscription(links, title, traffic, time.Now())
	if !ok {
		return false
	}
	logSubscriptionRoute(c.GetHeader("User-Agent"), "qwdtt-app")
	header := subReq.subscriptionUserinfo(traffic)
	a.ApplyCommonHeaders(c, header, a.updateInterval, metadata.Title, metadata.SupportURL, metadata.ProfileURL, metadata.Announce, a.subEnableRouting, a.subRoutingRules, a.subHideSettings)
	if a.subEncrypt {
		c.String(200, base64.StdEncoding.EncodeToString(body))
	} else {
		c.Data(200, "application/json; charset=utf-8", body)
	}
	a.recordSubscriptionFetch(c)
	return true
}
