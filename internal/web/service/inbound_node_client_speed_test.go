// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetNodeClientTrafficTotalsSumsAcrossNodes(t *testing.T) {
	db := initTrafficTestDB(t)
	rows := []model.NodeClientTraffic{
		{NodeId: 1, Email: "alice", Up: 10, Down: 20},
		{NodeId: 2, Email: "alice", Up: 1, Down: 2},
		{NodeId: 1, Email: "bob", Up: 5, Down: 6},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var svc InboundService
	got, err := svc.GetNodeClientTrafficTotals()
	if err != nil {
		t.Fatalf("GetNodeClientTrafficTotals: %v", err)
	}
	if got["alice"] != [2]int64{11, 22} {
		t.Fatalf("alice = %v, want [11 22]", got["alice"])
	}
	if got["bob"] != [2]int64{5, 6} {
		t.Fatalf("bob = %v, want [5 6]", got["bob"])
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
