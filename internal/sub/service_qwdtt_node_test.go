// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
)

// A qWDTT/CSQTT inbound deployed to a node must advertise the node, not the
// master: before the fix the master stamped its own outbound IP into subHost
// on save, so node clients dialed the master and never connected.
func TestGenQwdttCsqttLink_NodeInboundUsesNodeAddress(t *testing.T) {
	const masterIP = "2.27.201.120"
	prev := tunnel.LocalOutboundIPv4
	tunnel.LocalOutboundIPv4 = func() string { return masterIP }
	t.Cleanup(func() { tunnel.LocalOutboundIPv4 = prev })

	nodeID := 5
	s := &SubService{
		address:   "panel.example.com",
		nodesByID: map[int]*model.Node{5: {Id: 5, Address: "13.143.132.172"}},
	}
	qwdtt := func(subHost string, node *int) *model.Inbound {
		return &model.Inbound{
			Enable: true, Port: 56000, Protocol: model.Qwdtt, NodeID: node,
			Settings: `{"listenAddr":"0.0.0.0:56000","password":"secret","subHost":"` + subHost +
				`","vkHashes":"h1","workers":16,"clientPort":9000,"remark":"FI"}`,
		}
	}
	csqtt := func(subHost string, node *int) *model.Inbound {
		return &model.Inbound{
			Enable: true, Port: 46000, Protocol: model.Csqtt, NodeID: node,
			Settings: `{"listenAddr":"0.0.0.0:46000","password":"secret","subHost":"` + subHost + `"}`,
		}
	}

	cases := []struct {
		name string
		link string
		want string
	}{
		{"qwdtt node, master IP stamped", s.genQwdttLink(qwdtt(masterIP+":56000", &nodeID)), "peer=13.143.132.172%3A56000"},
		{"qwdtt node, empty subHost", s.genQwdttLink(qwdtt("", &nodeID)), "peer=13.143.132.172%3A56000"},
		{"qwdtt node, explicit subHost kept", s.genQwdttLink(qwdtt("fi.example.com:56000", &nodeID)), "peer=fi.example.com%3A56000"},
		{"qwdtt local keeps master subHost", s.genQwdttLink(qwdtt(masterIP+":56000", nil)), "peer=2.27.201.120%3A56000"},
		{"csqtt node, master IP stamped", s.genCsqttLink(csqtt(masterIP, &nodeID)), "host=13.143.132.172"},
		{"csqtt node, empty subHost", s.genCsqttLink(csqtt("", &nodeID)), "host=13.143.132.172"},
	}
	for _, c := range cases {
		if !strings.Contains(c.link, c.want) {
			t.Errorf("%s: link %q missing %q", c.name, c.link, c.want)
		}
	}
}
