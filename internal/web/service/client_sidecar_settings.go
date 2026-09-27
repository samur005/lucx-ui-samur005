package service

// LUCX-HOOK: client removal for share-only sidecar inbounds.
//
// qWDTT / CSQTT / olcRTC / tproxy inbounds (shareOnlySidecar) use one shared
// credential: their settings JSON has no per-client entries — often no
// "clients" key at all (node-adopted inbounds, inbounds saved before the key
// was added) — while the client membership lives in client_inbounds. The
// upstream delete path required settings.clients to be an array and failed
// with "invalid clients format in inbound settings", which blocked deleting
// any client attached to such an inbound.

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// inboundSettingsClients returns settings.clients. A missing or null key is an
// empty list; ok is false only when the key holds something other than an
// array.
func inboundSettingsClients(settings map[string]any) ([]any, bool) {
	raw, present := settings["clients"]
	if !present || raw == nil {
		return []any{}, true
	}
	clients, ok := raw.([]any)
	return clients, ok
}

// sidecarClientLinked reports whether the client with this email is linked
// (client_inbounds) to the given share-only sidecar inbound. Always false for
// other protocols, whose membership is the settings JSON itself.
func sidecarClientLinked(inbound *model.Inbound, email string) bool {
	if inbound == nil || !shareOnlySidecar(inbound.Protocol) || strings.TrimSpace(email) == "" {
		return false
	}
	var n int64
	err := database.GetDB().Model(&model.ClientInbound{}).
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("client_inbounds.inbound_id = ? AND clients.email = ?", inbound.Id, email).
		Count(&n).Error
	return err == nil && n > 0
}
