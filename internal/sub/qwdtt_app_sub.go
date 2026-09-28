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
//
// Description: like the WDTT panel's qWDTT JSON, "Подписка · до DD.MM.YYYY"
// (UTC+7), "Подписка · бессрочно", or "Подписка · N дн. с первого
// подключения" for a delayed-start client that has not connected yet.
//
// HWID: the app never sends X-HWID, so for its User-Agent without X-HWID the
// qWDTT JSON is served before the device-limit gate (serveQwdttAppNoHwid).
// The JSON carries only the qWDTT profiles; any other body stays gated.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
// MiB, rounded to 0.01. Used is always sent (0 included); a zero limit means
// unlimited and is left out.
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
	return buildQwdttAppSubscriptionDesc(links, title, qwdttAppDescription(traffic.ExpiryTime, 0), traffic, now)
}

// qwdttAppDescriptionZone is the owner's timezone for the expiry date on the
// app's subscription card (Asia/Novosibirsk, UTC+7, no DST — the same date
// the Telegram bot showed). Fixed zone: no tzdata dependency.
var qwdttAppDescriptionZone = time.FixedZone("UTC+7", 7*60*60)

const msPerDay = int64(24 * time.Hour / time.Millisecond)

// qwdttAppDescription renders the card's description. expiryMs is the
// subscription expiry (ms, 0 = unlimited); delayedMs > 0 is the duration of a
// delayed-start client ("days from first use") that has not started yet.
func qwdttAppDescription(expiryMs, delayedMs int64) string {
	switch {
	case delayedMs > 0:
		days := (delayedMs + msPerDay - 1) / msPerDay
		return fmt.Sprintf("Подписка · %d дн. с первого подключения", days)
	case expiryMs > 0:
		return "Подписка · до " + time.UnixMilli(expiryMs).In(qwdttAppDescriptionZone).Format("02.01.2006")
	default:
		return "Подписка · бессрочно"
	}
}

// qwdttAppDelayedStartMs returns the delayed-start duration (ms) when an
// enabled client of subId still has a negative expiry (not started yet), else 0.
// getSubs normalises such expiry to now+duration, which would drift daily.
func qwdttAppDelayedStartMs(subId string) int64 {
	db := database.GetDB()
	if db == nil || strings.TrimSpace(subId) == "" {
		return 0
	}
	var v int64
	if err := db.Model(&model.ClientRecord{}).
		Where("sub_id = ? AND enable = ? AND expiry_time < 0", subId, true).
		Select("COALESCE(MIN(expiry_time), 0)").
		Scan(&v).Error; err != nil || v >= 0 {
		return 0
	}
	return -v
}

func buildQwdttAppSubscriptionDesc(links []string, title, description string, traffic xray.ClientTraffic, now time.Time) ([]byte, bool) {
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
		Description:      description,
		TrafficUsedMb:    &used,
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
	description := qwdttAppDescription(traffic.ExpiryTime, qwdttAppDelayedStartMs(subId))
	body, ok := buildQwdttAppSubscriptionDesc(links, title, description, traffic, time.Now())
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

// serveQwdttAppNoHwid serves the qWDTT app (User-Agent qWDTT-Subscription/…)
// when it sends no X-HWID: the app cannot send one, and the device-limit gate
// would answer 404 for every client with a limit. Returns false, without
// writing anything, for other requests and when the subscription has no qWDTT
// profile — the caller then applies the regular HWID gate, so non-qWDTT bodies
// are never served ungated. Unknown subId -> 404, DB error -> 500 (as before).
func (a *SUBController) serveQwdttAppNoHwid(c *gin.Context, userAgent string) bool {
	if !IsQwdttAppClient(userAgent) || strings.TrimSpace(c.GetHeader("X-HWID")) != "" {
		return false
	}
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	subReq := a.subService.ForRequest(host)
	subReq.subscriptionBody = true
	subs, _, _, traffic, err := subReq.getSubs(subId)
	if err != nil || subs == nil {
		writeSubError(c, err)
		return true
	}
	return a.serveQwdttAppSubscription(c, subReq, subId, subs, traffic, scheme, hostWithPort)
}
