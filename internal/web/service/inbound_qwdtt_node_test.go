// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// Saving a node-managed qWDTT/CSQTT inbound on the master must not stamp the
// master's outbound IP as subHost (clients would dial the master).
func TestNormalizeSidecarSubHost_NodeInbound(t *testing.T) {
	const masterIP = "2.27.201.120"
	prev := tunnel.LocalOutboundIPv4
	tunnel.LocalOutboundIPv4 = func() string { return masterIP }
	t.Cleanup(func() { tunnel.LocalOutboundIPv4 = prev })

	s := &InboundService{}
	nodeID := 5
	// Read the persisted JSON, not a re-parse (which would mask the bug).
	subHost := func(ib *model.Inbound) string {
		var m struct {
			SubHost string `json:"subHost"`
		}
		if err := json.Unmarshal([]byte(ib.Settings), &m); err != nil {
			t.Fatalf("settings JSON: %v", err)
		}
		return m.SubHost
	}
	run := func(proto model.Protocol, sh string, node *int) string {
		ib := &model.Inbound{Protocol: proto, NodeID: node,
			Settings: `{"listenAddr":"0.0.0.0:56000","password":"p","subHost":"` + sh + `"}`}
		if proto == model.Csqtt {
			s.normalizeCsqttSettings(ib)
		} else {
			s.normalizeQwdttSettings(ib)
		}
		return subHost(ib)
	}

	if got := run(model.Qwdtt, "", &nodeID); got != "" {
		t.Errorf("qwdtt node, empty: subHost = %q, want empty (resolved to node at share time)", got)
	}
	if got := run(model.Qwdtt, masterIP+":56000", &nodeID); got != "" {
		t.Errorf("qwdtt node, master IP: subHost = %q, want cleared", got)
	}
	if got := run(model.Qwdtt, "13.143.132.172:56000", &nodeID); got != "13.143.132.172:56000" {
		t.Errorf("qwdtt node, explicit: subHost = %q, want kept", got)
	}
	if got := run(model.Qwdtt, masterIP+":56000", nil); got != masterIP+":56000" {
		t.Errorf("qwdtt local, master IP: subHost = %q, want kept (sidecar runs here)", got)
	}
	if got := run(model.Csqtt, masterIP, &nodeID); got != "" {
		t.Errorf("csqtt node, master IP: subHost = %q, want cleared", got)
	}
}
