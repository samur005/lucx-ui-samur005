// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
//
// WB Stream room generator API for the olcRTC tunnel (provider: wbstream).
// Room API adapted from kulikov0/whitelist-bypass (MIT) — see CREDITS.md.
package controller

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/wbcreator"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// wbOlcrtcInbound is a compact view of an olcRTC inbound for the WB panel.
type wbOlcrtcInbound struct {
	ID       int    `json:"id"`
	Remark   string `json:"remark"`
	Enable   bool   `json:"enable"`
	Provider string `json:"provider"`
	RoomID   string `json:"room_id"`
	Remote   bool   `json:"remote"`
}

func listOlcrtcInboundsForWb() []wbOlcrtcInbound {
	out := []wbOlcrtcInbound{}
	db := database.GetDB()
	if db == nil {
		return out
	}
	var ibs []model.Inbound
	if err := db.Where("protocol = ?", model.Olcrtc).Order("id").Find(&ibs).Error; err != nil {
		return out
	}
	for _, ib := range ibs {
		var s struct {
			Provider string `json:"provider"`
			RoomID   string `json:"roomId"`
		}
		_ = json.Unmarshal([]byte(ib.Settings), &s)
		out = append(out, wbOlcrtcInbound{
			ID:       ib.Id,
			Remark:   ib.Remark,
			Enable:   ib.Enable,
			Provider: s.Provider,
			RoomID:   s.RoomID,
			Remote:   ib.NodeID != nil,
		})
	}
	return out
}

func wbStatusPayload() map[string]any {
	st := wbcreator.Status()
	return map[string]any{
		"cookies_ok":      st.CookiesOK,
		"cookies_present": st.CookiesPresent,
		"cookies_expired": st.CookiesExpired,
		"cookies_hint":    st.CookiesHint,
		"cookie_names":    st.CookieNames,
		"has_refresh":     st.HasRefresh,
		"has_token":       st.HasToken,
		"token_exp":       st.TokenExp,
		"last_ok_at":      st.LastOKAt,
		"last_error":      st.LastError,
		"rooms":           st.Rooms,
		"inbounds":        listOlcrtcInboundsForWb(),
	}
}

func (a *TunnelController) wbStatus(c *gin.Context) {
	jsonObj(c, wbStatusPayload(), nil)
}

func (a *TunnelController) wbSaveCookies(c *gin.Context) {
	raw, err := readCookiesPayload(c)
	if err != nil {
		jsonMsg(c, "wb: invalid cookies payload", err)
		return
	}
	if err := wbcreator.SaveCookies(raw); err != nil {
		jsonMsg(c, "wb: save cookies failed", err)
		return
	}
	jsonObj(c, wbStatusPayload(), nil)
}

func (a *TunnelController) wbClearCookies(c *gin.Context) {
	if err := wbcreator.ClearCookies(); err != nil {
		jsonMsg(c, "wb: clear cookies failed", err)
		return
	}
	jsonObj(c, wbStatusPayload(), nil)
}

// wbCreateRoom creates a room; with apply+inbound_id it also writes roomId
// into that olcRTC inbound (provider must already be wbstream). The tunnel
// reconcile job restarts the olcrtc process on the next tick (config
// fingerprint changed) and the olcrtc:// link / subscription pick up the id.
func (a *TunnelController) wbCreateRoom(c *gin.Context) {
	var req struct {
		Apply     bool `json:"apply"`
		InboundID int  `json:"inbound_id"`
	}
	_ = c.ShouldBindJSON(&req)

	var target *model.Inbound
	if req.Apply && req.InboundID > 0 {
		ib, err := loadWbTargetInbound(req.InboundID)
		if err != nil {
			jsonMsg(c, "wb: create room failed", err)
			return
		}
		target = ib
	}

	room, err := wbcreator.CreateRoom(req.InboundID)
	if err != nil {
		jsonMsg(c, "wb: create room failed", err)
		return
	}
	applied := 0
	if target != nil {
		if err := applyWbRoomToOlcrtcInbound(target, room.RoomID); err != nil {
			jsonMsg(c, "wb: room "+room.RoomID+" created, but saving it into the inbound failed", err)
			return
		}
		applied = target.Id
		wbcreator.NoteRoomApplied(room.RoomID, applied)
	}
	payload := wbStatusPayload()
	payload["room_id"] = room.RoomID
	payload["join_link"] = room.JoinLink
	payload["applied_inbound_id"] = applied
	jsonObj(c, payload, nil)
}

func loadWbTargetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	if db == nil {
		return nil, common.NewError("database not ready")
	}
	ib := &model.Inbound{}
	if err := db.Where("id = ?", id).First(ib).Error; err != nil {
		return nil, common.NewErrorf("inbound %d not found", id)
	}
	if ib.Protocol != model.Olcrtc {
		return nil, common.NewErrorf("inbound %d is not an olcRTC inbound", id)
	}
	if ib.NodeID != nil {
		return nil, common.NewError("inbound runs on a remote node — create the room from the inbound form and save the inbound instead")
	}
	var s map[string]any
	if strings.TrimSpace(ib.Settings) != "" {
		if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
			return nil, common.NewErrorf("inbound %d settings are not valid JSON", id)
		}
	}
	if p, _ := s["provider"].(string); p != "wbstream" {
		return nil, common.NewErrorf("inbound %d provider is %q, not WB Stream — switch it to WB Stream first", id, p)
	}
	return ib, nil
}

func applyWbRoomToOlcrtcInbound(ib *model.Inbound, roomID string) error {
	if !wbcreator.ValidRoomID(roomID) {
		return common.NewError("invalid room id")
	}
	var s map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil || s == nil {
		return common.NewError("inbound settings are not valid JSON")
	}
	s["roomId"] = roomID
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", string(raw)).Error
}

// registerWBRoutes mounts the WB Stream room generator under /panel/api/tunnel/wb/*.
func (a *TunnelController) registerWBRoutes(g *gin.RouterGroup) {
	wb := g.Group("/wb")
	wb.GET("/status", a.wbStatus)
	wb.POST("/cookies", a.wbSaveCookies)
	wb.POST("/cookies/clear", a.wbClearCookies)
	wb.POST("/create", a.wbCreateRoom)
}
