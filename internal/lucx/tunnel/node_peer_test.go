// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeSidecarSubHost(t *testing.T) {
	prev := LocalOutboundIPv4
	LocalOutboundIPv4 = func() string { return "2.27.201.120" }
	t.Cleanup(func() { LocalOutboundIPv4 = prev })

	cases := map[string]string{
		"":                     "",
		"  ":                   "",
		"2.27.201.120:56000":   "", // master IP stamped by EnsureSubHost
		"2.27.201.120":         "", // CSQTT stores a bare host
		"13.143.132.172:56000": "13.143.132.172:56000",
		"node.example.com":     "node.example.com",
		" 9.9.9.9:56000 ":      "9.9.9.9:56000",
	}
	for in, want := range cases {
		if got := NodeSidecarSubHost(in); got != want {
			t.Errorf("NodeSidecarSubHost(%q) = %q, want %q", in, got, want)
		}
	}

	LocalOutboundIPv4 = func() string { return "" }
	if got := NodeSidecarSubHost("2.27.201.120:56000"); got != "2.27.201.120:56000" {
		t.Errorf("unknown local IP must keep subHost, got %q", got)
	}
}

func TestConfigFromInbound_NodeManagedSkipsLocalSubHost(t *testing.T) {
	prev := LocalOutboundIPv4
	LocalOutboundIPv4 = func() string { return "2.27.201.120" }
	t.Cleanup(func() { LocalOutboundIPv4 = prev })

	node := 5
	q, _ := QwdttConfigFromInbound(&model.Inbound{Protocol: model.Qwdtt, NodeID: &node,
		Settings: `{"password":"p","subHost":"2.27.201.120:56000"}`})
	if q.SubHost != "" || q.EnsureSubHost().SubHost != "" {
		t.Fatalf("node qwdtt: master subHost must be dropped and never re-stamped, got %q / %q", q.SubHost, q.EnsureSubHost().SubHost)
	}
	if bs, _ := json.Marshal(q); strings.Contains(string(bs), "nodeManaged") {
		t.Fatalf("nodeManaged must not be serialized: %s", bs)
	}
	c, _ := CsqttConfigFromInbound(&model.Inbound{Protocol: model.Csqtt, NodeID: &node,
		Settings: `{"password":"p","subHost":"2.27.201.120"}`})
	if c.SubHost != "" || c.EnsureSubHost().SubHost != "" {
		t.Fatalf("node csqtt: master subHost must be dropped, got %q", c.SubHost)
	}
	local, _ := QwdttConfigFromInbound(&model.Inbound{Protocol: model.Qwdtt,
		Settings: `{"password":"p","subHost":"2.27.201.120:56000"}`})
	if local.SubHost != "2.27.201.120:56000" {
		t.Fatalf("local qwdtt must keep subHost, got %q", local.SubHost)
	}
}
