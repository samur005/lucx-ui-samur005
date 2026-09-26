// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const QwdttKey = "qwdtt"

func QwdttConfigFromInbound(ib *model.Inbound) (QwdttConfig, bool) {
	if ib == nil || ib.Protocol != model.Qwdtt {
		return QwdttConfig{}, false
	}
	cfg := DefaultQwdttConfig()
	if raw := strings.TrimSpace(ib.Settings); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &cfg)
		var keys map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &keys) == nil {
			if _, ok := keys["routeThroughXray"]; !ok {
				cfg.RouteThroughXray = true
			}
		}
	}
	if r := strings.TrimSpace(ib.Remark); r != "" && strings.TrimSpace(cfg.Remark) == "" {
		cfg.Remark = r
	}
	cfg.Enabled = ib.Enable
	if ib.Port > 0 {
		if host, p, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
			if p == "56000" || p == "0" {
				cfg.ListenAddr = net.JoinHostPort(host, strconv.Itoa(ib.Port))
			}
		}
	}
	if ib.NodeID != nil { // LUCX-HOOK: sidecar runs on the node, not here
		cfg.nodeManaged = true
		cfg.SubHost = NodeSidecarSubHost(cfg.SubHost)
	}
	cfg = cfg.Merge()
	if c2, err := cfg.EnsureVkHashes(); err == nil {
		cfg = c2
	}
	return cfg, true
}

func QwdttInstanceFromInbound(ib *model.Inbound) (Instance, bool) {
	cfg, ok := QwdttConfigFromInbound(ib)
	if !ok {
		return Instance{}, false
	}
	if !ib.Enable {
		return Instance{Core: Qwdtt, Key: QwdttKey, Enabled: false}, true
	}
	if err := cfg.Validate(); err != nil {
		return Instance{Core: Qwdtt, Key: QwdttKey, Enabled: false}, true
	}
	if strings.TrimSpace(cfg.Password) == "" {
		if c2, err := cfg.EnsurePassword(); err == nil {
			cfg = c2
		}
	}
	if strings.TrimSpace(cfg.ConfigDir) == "" {
		cfg.ConfigDir = dataDirFor(QwdttKey, Qwdtt)
	}
	inst := Instance{
		Core:      Qwdtt,
		Key:       QwdttKey,
		Enabled:   true,
		Args:      cfg.BuildArgs(),
		ProbePort: 0,
	}
	if cfg.RouteThroughXray {
		inst.RouteThroughXray = true
		inst.TunName = QwdttTunName(ib.Id)
		inst.RouteTable = QwdttRouteTable(ib.Id)
		inst.RouteIfaces = []string{qwdttIfaceWG, qwdttIfaceRaw}
	}
	return inst, true
}

func QwdttDTLSPort(cfg QwdttConfig) int {
	if _, port, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}
	return 56000
}
