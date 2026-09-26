// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/wbcreator"
)

// wbCreateRoom is swapped in tests.
var wbCreateRoom = func() (string, error) {
	room, err := wbcreator.CreateRoom(0)
	return room.RoomID, err
}

var wbHasCredentials = wbcreator.HasCredentials

// ensureOlcrtcWbRoom runs on olcRTC inbound create/update (before the
// normal olcRTC normalisation) for provider=wbstream only:
//   - a pasted https://stream.wb.ru/room/<id> or wbstream://<id> is reduced
//     to the bare id olcrtc expects;
//   - an empty roomId is filled with a freshly created WB Stream room when
//     a WB session is stored in the panel (Tunnels → olcRTC → WB Stream).
//
// Failures are logged and never block the save (the inbound simply stays
// without a room, exactly as before this hook existed). Node-sync saves are
// skipped: rooms are created on the master only.
func (s *InboundService) ensureOlcrtcWbRoom(inbound *model.Inbound) {
	if inbound == nil || inbound.Protocol != model.Olcrtc || s.FromNodeSync {
		return
	}
	raw := strings.TrimSpace(inbound.Settings)
	if raw == "" || raw == "{}" {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(raw), &settings); err != nil || settings == nil {
		return
	}
	if p, _ := settings["provider"].(string); p != "wbstream" {
		return
	}
	room, _ := settings["roomId"].(string)
	room = strings.TrimSpace(room)
	if room != "" {
		norm := wbcreator.ParseRoomID(room)
		if norm == room || !wbcreator.ValidRoomID(norm) {
			return
		}
		settings["roomId"] = norm
	} else {
		if !wbHasCredentials() {
			return
		}
		id, err := wbCreateRoom()
		if err != nil {
			logger.Warning("olcrtc: WB Stream room auto-create failed:", err)
			return
		}
		logger.Info("olcrtc: WB Stream room auto-created for inbound", inbound.Remark, ":", id)
		settings["roomId"] = id
	}
	if bs, err := json.MarshalIndent(settings, "", "  "); err == nil {
		inbound.Settings = string(bs)
	}
}
