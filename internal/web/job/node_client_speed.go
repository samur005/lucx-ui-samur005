// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func diffSpeedSamples(totals map[string][2]int64, prev map[string]inboundSample, now int64) (map[string][2]int64, map[string]inboundSample) {
	deltas := make(map[string][2]int64)
	next := make(map[string]inboundSample, len(totals))
	for key, cur := range totals {
		old, ok := prev[key]
		if !ok {
			next[key] = inboundSample{up: cur[0], down: cur[1], at: now}
			continue
		}
		dUp := cur[0] - old.up
		dDown := cur[1] - old.down
		if dUp <= 0 && dDown <= 0 {
			if cur[0] < old.up || cur[1] < old.down {
				next[key] = inboundSample{up: cur[0], down: cur[1], at: now}
			} else {
				next[key] = old
			}
			continue
		}
		dUp = max(dUp, 0)
		dDown = max(dDown, 0)
		elapsed := max(now-old.at, nodeInboundSpeedWindowMs)
		up := dUp * nodeInboundSpeedWindowMs / elapsed
		down := dDown * nodeInboundSpeedWindowMs / elapsed
		if up > 0 || down > 0 {
			deltas[key] = [2]int64{up, down}
		}
		next[key] = inboundSample{up: cur[0], down: cur[1], at: now}
	}
	return deltas, next
}

func (j *NodeTrafficSyncJob) nodeClientSpeed(now int64) []*xray.ClientTraffic {
	totals, err := j.inboundService.GetNodeClientTrafficTotals()
	if err != nil {
		return nil
	}
	deltas, next := diffSpeedSamples(totals, j.prevClientTotals, now)
	j.prevClientTotals = next
	out := make([]*xray.ClientTraffic, 0, len(deltas))
	for email, d := range deltas {
		out = append(out, &xray.ClientTraffic{Email: email, Up: d[0], Down: d[1]})
	}
	return out
}
