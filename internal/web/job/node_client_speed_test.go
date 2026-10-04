// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import "testing"

func TestDiffSpeedSamples(t *testing.T) {
	t.Run("first sight yields no delta", func(t *testing.T) {
		deltas, next := diffSpeedSamples(map[string][2]int64{"a": {100, 200}}, nil, 1000)
		if len(deltas) != 0 {
			t.Fatalf("deltas = %v, want none", deltas)
		}
		if got := next["a"]; got.up != 100 || got.down != 200 || got.at != 1000 {
			t.Fatalf("baseline = %+v", got)
		}
	})

	t.Run("growth over one window is the raw delta", func(t *testing.T) {
		prev := map[string]inboundSample{"a": {up: 100, down: 200, at: 1000}}
		deltas, next := diffSpeedSamples(map[string][2]int64{"a": {600, 1200}}, prev, 6000)
		if got := deltas["a"]; got != [2]int64{500, 1000} {
			t.Fatalf("delta = %v", got)
		}
		if next["a"].up != 600 || next["a"].at != 6000 {
			t.Fatalf("next = %+v", next["a"])
		}
	})

	t.Run("gap is averaged over real elapsed time", func(t *testing.T) {
		prev := map[string]inboundSample{"a": {up: 0, down: 0, at: 0}}
		deltas, _ := diffSpeedSamples(map[string][2]int64{"a": {10000, 0}}, prev, 20000)
		if got := deltas["a"]; got != [2]int64{2500, 0} {
			t.Fatalf("delta = %v, want {2500 0}", got)
		}
	})

	t.Run("idle holds timestamp", func(t *testing.T) {
		prev := map[string]inboundSample{"a": {up: 5, down: 5, at: 1000}}
		deltas, next := diffSpeedSamples(map[string][2]int64{"a": {5, 5}}, prev, 6000)
		if len(deltas) != 0 || next["a"].at != 1000 {
			t.Fatalf("deltas=%v next=%+v", deltas, next["a"])
		}
	})

	t.Run("counter reset rebaselines", func(t *testing.T) {
		prev := map[string]inboundSample{"a": {up: 500, down: 500, at: 1000}}
		deltas, next := diffSpeedSamples(map[string][2]int64{"a": {10, 20}}, prev, 6000)
		if len(deltas) != 0 {
			t.Fatalf("deltas = %v", deltas)
		}
		if got := next["a"]; got.up != 10 || got.down != 20 || got.at != 6000 {
			t.Fatalf("next = %+v", got)
		}
	})

	t.Run("vanished key is dropped", func(t *testing.T) {
		prev := map[string]inboundSample{"gone": {up: 1, down: 1, at: 1}}
		_, next := diffSpeedSamples(map[string][2]int64{}, prev, 2)
		if len(next) != 0 {
			t.Fatalf("next = %v", next)
		}
	})
}
