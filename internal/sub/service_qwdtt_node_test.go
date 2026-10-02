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
		{"qwdtt node, master IP stamped", s.genQwdttLink(qwdtt(masterIP+":56000", &nodeID), ""), "peer=13.143.132.172%3A56000"},
		{"qwdtt node, empty subHost", s.genQwdttLink(qwdtt("", &nodeID), ""), "peer=13.143.132.172%3A56000"},
		{"qwdtt node, explicit subHost kept", s.genQwdttLink(qwdtt("fi.example.com:56000", &nodeID), ""), "peer=fi.example.com%3A56000"},
		{"qwdtt local keeps master subHost", s.genQwdttLink(qwdtt(masterIP+":56000", nil), ""), "peer=2.27.201.120%3A56000"},
		{"csqtt node, master IP stamped", s.genCsqttLink(csqtt(masterIP, &nodeID)), "host=13.143.132.172"},
		{"csqtt node, empty subHost", s.genCsqttLink(csqtt("", &nodeID)), "host=13.143.132.172"},
	}
	for _, c := range cases {
		if !strings.Contains(c.link, c.want) {
			t.Errorf("%s: link %q missing %q", c.name, c.link, c.want)
		}
	}
}

// A known client gets its personal qWDTT password in the link; everything else
// (no email, unknown email, disabled client) keeps the shared owner password.
func TestGenQwdttLink_PersonalPassword(t *testing.T) {
	ib := &model.Inbound{
		Id: 33, Enable: true, Port: 56000, Protocol: model.Qwdtt,
		Settings: `{"listenAddr":"0.0.0.0:56000","password":"sharedSharedShar","subHost":"1.2.3.4:56000","vkHashes":"h1","workers":16,"clientPort":9000,"remark":"M"}`,
	}
	s := &SubService{}
	s.primeLinkClients(ib.Id, []model.Client{
		{ID: "uuid-a", Email: "a@x", Enable: true},
		{ID: "uuid-off", Email: "off@x", Enable: false},
	}, true)

	pass := func(link string) string {
		p, ok := qwdttProfileFromURI(link)
		if !ok {
			t.Fatalf("not a qwdtt link: %q", link)
		}
		return p.Password
	}
	shared := pass(s.genQwdttLink(ib, ""))
	if shared != "sharedSharedShar" {
		t.Fatalf("empty email must keep shared password, got %q", shared)
	}
	if got := pass(s.GetLink(ib, "ghost@x")); got != shared {
		t.Fatalf("unknown client must keep shared password, got %q", got)
	}
	if got := pass(s.GetLink(ib, "off@x")); got != shared {
		t.Fatalf("disabled client must keep shared password, got %q", got)
	}
	want := tunnel.QwdttClientPassword("sharedSharedShar", "uuid-a")
	if got := pass(s.GetLink(ib, "a@x")); got != want || want == shared {
		t.Fatalf("personal password = %q want %q (shared %q)", got, want, shared)
	}
	// Only the password differs from the shared link.
	a := strings.ReplaceAll(s.GetLink(ib, "a@x"), want, "X")
	b := strings.ReplaceAll(s.genQwdttLink(ib, ""), shared, "X")
	if a != b {
		t.Fatalf("links differ beyond the password:\n%s\n%s", a, b)
	}
}

// A node-managed inbound gets personal passwords only when its node advertises
// the capability; an older node (which never registers them) keeps the shared
// password so its profile keeps working.
func TestGenQwdttLink_PersonalPasswordNeedsCapableNode(t *testing.T) {
	nodeID := 5
	ib := &model.Inbound{
		Id: 44, Enable: true, Port: 56000, Protocol: model.Qwdtt, NodeID: &nodeID,
		Settings: `{"listenAddr":"0.0.0.0:56000","password":"sharedSharedShar","subHost":"13.143.132.172:56000","vkHashes":"h1","workers":16,"clientPort":9000,"remark":"FI"}`,
	}
	client := model.Client{ID: "uuid-a", Email: "a@x", Enable: true}
	personal := tunnel.QwdttClientPassword("sharedSharedShar", "uuid-a")
	pass := func(features string, noNode bool) string {
		s := &SubService{nodesByID: map[int]*model.Node{5: {Id: 5, Address: "13.143.132.172", Features: features}}}
		if noNode {
			s.nodesByID = nil
		}
		s.primeLinkClients(ib.Id, []model.Client{client}, true)
		p, ok := qwdttProfileFromURI(s.GetLink(ib, "a@x"))
		if !ok {
			t.Fatal("no qwdtt link")
		}
		return p.Password
	}
	if got := pass(`{"nodeType":"lucx","features":["qwdtt","qwdtt-personal"]}`, false); got != personal {
		t.Fatalf("capable node: got %q want personal", got)
	}
	for name, got := range map[string]string{
		"old node":     pass(`{"nodeType":"lucx","features":["qwdtt"]}`, false),
		"no features":  pass(``, false),
		"unknown node": pass(``, true),
	} {
		if got != "sharedSharedShar" {
			t.Fatalf("%s must keep the shared password, got %q", name, got)
		}
	}
}
