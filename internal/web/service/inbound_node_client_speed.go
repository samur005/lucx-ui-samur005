// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func (s *InboundService) GetNodeClientTrafficTotals() (map[string][2]int64, error) {
	var rows []struct {
		Email string
		Up    int64
		Down  int64
	}
	if err := database.GetDB().Model(&model.NodeClientTraffic{}).
		Select("email, SUM(up) AS up, SUM(down) AS down").
		Group("email").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string][2]int64, len(rows))
	for _, r := range rows {
		out[r.Email] = [2]int64{r.Up, r.Down}
	}
	return out, nil
}
