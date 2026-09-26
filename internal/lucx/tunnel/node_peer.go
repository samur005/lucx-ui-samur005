// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"net"
	"strings"
)

// LocalOutboundIPv4 is this host's outbound IPv4 (the value EnsureSubHost
// stamps). A variable so tests can pin it without a network probe.
var LocalOutboundIPv4 = detectOutboundIPv4

// NodeSidecarSubHost returns the subHost ("host" or "host:port") to keep for a
// node-managed qWDTT/CSQTT inbound evaluated on the master panel. The sidecar
// runs on the node, so the master's own outbound IPv4 is never a valid peer:
// such a value (stamped by EnsureSubHost on the master before this fix, so
// node clients dialed the master) is dropped and "" is returned. Callers then
// resolve the node's address the way every other node inbound does (sub:
// resolveInboundAddress, panel UI: resolveAddr). Config*FromInbound also marks
// the config nodeManaged so EnsureSubHost never re-stamps the master IP. An
// explicit operator value (node IP, domain, NAT address) is kept as-is.
func NodeSidecarSubHost(subHost string) string {
	h := strings.TrimSpace(subHost)
	if h == "" {
		return ""
	}
	host := h
	if hh, _, err := net.SplitHostPort(h); err == nil {
		host = hh
	}
	if local := LocalOutboundIPv4(); local != "" && strings.EqualFold(host, local) {
		return ""
	}
	return h
}
