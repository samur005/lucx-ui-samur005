// Copyright (c) 2025 LucX-UI Project / samur005 fork.
// Licensed under the PolyForm Noncommercial License 1.0.0.
package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func olcrtcRoom(t *testing.T, ib *model.Inbound) string {
	t.Helper()
	var s map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		t.Fatal(err)
	}
	v, _ := s["roomId"].(string)
	return v
}

func TestEnsureOlcrtcWbRoom(t *testing.T) {
	oldC, oldH := wbCreateRoom, wbHasCredentials
	t.Cleanup(func() { wbCreateRoom, wbHasCredentials = oldC, oldH })
	calls := 0
	wbHasCredentials = func() bool { return true }
	wbCreateRoom = func() (string, error) { calls++; return "room-new", nil }
	s := &InboundService{}

	ib := &model.Inbound{Protocol: model.Olcrtc, Settings: `{"provider":"wbstream","roomId":""}`}
	s.ensureOlcrtcWbRoom(ib)
	if olcrtcRoom(t, ib) != "room-new" || calls != 1 {
		t.Fatalf("empty wbstream room must be auto-created: %s", ib.Settings)
	}

	ib = &model.Inbound{Protocol: model.Olcrtc, Settings: `{"provider":"wbstream","roomId":"https://stream.wb.ru/room/abc-1"}`}
	s.ensureOlcrtcWbRoom(ib)
	if olcrtcRoom(t, ib) != "abc-1" || calls != 1 {
		t.Fatalf("room URL must be normalised without creating: %s", ib.Settings)
	}

	keep := `{"provider":"jitsi","roomId":""}`
	ib = &model.Inbound{Protocol: model.Olcrtc, Settings: keep}
	s.ensureOlcrtcWbRoom(ib)
	if ib.Settings != keep || calls != 1 {
		t.Fatal("non-wbstream providers must be untouched")
	}

	keep = `{"provider":"wbstream","roomId":"abc-2"}`
	ib = &model.Inbound{Protocol: model.Olcrtc, Settings: keep}
	s.ensureOlcrtcWbRoom(ib)
	if ib.Settings != keep {
		t.Fatal("bare room id must be untouched")
	}

	wbCreateRoom = func() (string, error) { calls++; return "", errors.New("boom") }
	keep = `{"provider":"wbstream","roomId":""}`
	ib = &model.Inbound{Protocol: model.Olcrtc, Settings: keep}
	s.ensureOlcrtcWbRoom(ib)
	if ib.Settings != keep {
		t.Fatal("failed create must leave settings untouched")
	}

	wbHasCredentials = func() bool { return false }
	before := calls
	s.ensureOlcrtcWbRoom(&model.Inbound{Protocol: model.Olcrtc, Settings: keep})
	if calls != before {
		t.Fatal("no stored WB session → no create call")
	}

	wbHasCredentials = func() bool { return true }
	sync := &InboundService{FromNodeSync: true}
	sync.ensureOlcrtcWbRoom(&model.Inbound{Protocol: model.Olcrtc, Settings: keep})
	if calls != before {
		t.Fatal("node-sync saves must not create rooms")
	}
}
