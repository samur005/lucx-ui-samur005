// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Native VK Creator API — adapted from WDTT (https://github.com/ildarmaga/wdtt), GPL-3.0 provenance; see CREDITS.md.
package controller

import (
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/vkcreator"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

func (a *TunnelController) vkStatus(c *gin.Context) {
	jsonObj(c, vkcreator.Status(), nil)
}

func (a *TunnelController) vkSaveCookies(c *gin.Context) {
	raw, err := readCookiesPayload(c)
	if err != nil {
		jsonMsg(c, "vk: invalid cookies payload", err)
		return
	}
	if err := vkcreator.SaveCookies(raw); err != nil {
		jsonMsg(c, "vk: save cookies failed", err)
		return
	}
	jsonObj(c, vkcreator.Status(), nil)
}

func (a *TunnelController) vkClearCookies(c *gin.Context) {
	if err := vkcreator.ClearCookies(); err != nil {
		jsonMsg(c, "vk: clear cookies failed", err)
		return
	}
	jsonObj(c, vkcreator.Status(), nil)
}

func (a *TunnelController) vkCreateCall(c *gin.Context) {
	var req struct {
		Apply    bool   `json:"apply"`
		Existing string `json:"existing"` // current vkHashes to merge into
	}
	_ = c.ShouldBindJSON(&req)

	res, err := vkcreator.CreateCall()
	if err != nil {
		jsonMsg(c, "vk: create call failed", err)
		return
	}
	outHash := res.VkHash
	if req.Apply {
		outHash = vkcreator.MergeHashes(req.Existing, res.VkHash)
		// Best-effort persist: legacy tunnel blob and/or qWDTT inbound settings.
		if cfg, err := a.svc.LoadQwdttConfig(); err == nil {
			cfg.VkHashes = outHash
			_ = a.svc.SaveQwdttConfig(cfg)
		}
		_ = applyVkHashToQwdttInbound(res.VkHash)
	}
	st := vkcreator.Status()
	jsonObj(c, map[string]any{
		"join_link":  res.JoinLink,
		"vk_hash":    outHash,
		"new_hash":   res.VkHash,
		"session":    res.Session,
		"sessions":   st.Sessions,
		"cookies_ok": st.CookiesOK,
	}, nil)
}

func (a *TunnelController) vkStopCall(c *gin.Context) {
	var req struct {
		CallID string `json:"call_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := vkcreator.StopCall(req.CallID); err != nil {
		jsonMsg(c, "vk: stop call failed", err)
		return
	}
	jsonObj(c, vkcreator.Status(), nil)
}

func readCookiesPayload(c *gin.Context) ([]byte, error) {
	ct := c.ContentType()
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, err := c.FormFile("file")
		if err == nil {
			f, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return io.ReadAll(io.LimitReader(f, 1<<20))
		}
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	s := extractCookiesPayload(body)
	if s == "" {
		if v := strings.TrimSpace(c.PostForm("cookie_string")); v != "" {
			return []byte(v), nil
		}
		if v := strings.TrimSpace(c.PostForm("cookies_json")); v != "" {
			return []byte(v), nil
		}
		return nil, common.NewError("provide cookie_string or cookies_json")
	}
	return []byte(s), nil
}

// extractCookiesPayload — same contract as WDTT panel/vk_creator_handlers.go.
func extractCookiesPayload(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "remixsid=") {
		return s
	}
	if strings.HasPrefix(s, "{") {
		var req struct {
			CookiesJSON  string `json:"cookies_json"`
			CookieString string `json:"cookie_string"`
		}
		if json.Unmarshal(body, &req) == nil {
			if v := strings.TrimSpace(req.CookiesJSON); v != "" {
				return v
			}
			if v := strings.TrimSpace(req.CookieString); v != "" {
				return v
			}
		}
	}
	for _, pair := range strings.Split(s, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		if key != "cookies_json" && key != "cookie_string" {
			continue
		}
		val, err := url.QueryUnescape(kv[1])
		if err != nil {
			val = kv[1]
		}
		if v := strings.TrimSpace(val); v != "" {
			return v
		}
	}
	return ""
}

func applyVkHashToQwdttInbound(hash string) error {
	db := database.GetDB()
	if db == nil || strings.TrimSpace(hash) == "" {
		return nil
	}
	var ibs []model.Inbound
	if err := db.Where("protocol = ?", model.Qwdtt).Find(&ibs).Error; err != nil {
		return err
	}
	for i := range ibs {
		ib := &ibs[i]
		var settings map[string]any
		if strings.TrimSpace(ib.Settings) == "" {
			settings = map[string]any{}
		} else if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		existing, _ := settings["vkHashes"].(string)
		settings["vkHashes"] = vkcreator.MergeHashes(existing, hash)
		raw, err := json.Marshal(settings)
		if err != nil {
			continue
		}
		ib.Settings = string(raw)
		_ = db.Model(ib).Update("settings", ib.Settings).Error
	}
	return nil
}
