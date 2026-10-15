package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// SQLite's LOWER() folds ASCII only: a Cyrillic email attached to two inbounds
// looked unshared, so detaching it from one dropped its client_traffics row and
// the next node snapshot re-seeded it with the node's stale expiry/enable.
func TestEmailsUsedByOtherInboundsMatchesNonASCIIEmail(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}
	is := &InboundService{}

	exp := time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	seed := []model.Client{{ID: "id-r", Email: "Рустам", Enable: true, SubID: "s-r", ExpiryTime: exp}}
	ibA := mkInbound(t, 21201, model.VLESS, clientsSettings(t, seed))
	ibB := mkInbound(t, 21202, model.VLESS, clientsSettings(t, seed))
	for _, ib := range []*model.Inbound{ibA, ibB} {
		if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
			t.Fatalf("seed SyncInbound: %v", err)
		}
	}

	for _, probe := range []string{"Рустам", "рустам", " Рустам "} {
		shared, err := is.emailsUsedByOtherInbounds(database.GetDB(), []string{probe}, ibA.Id)
		if err != nil {
			t.Fatalf("emailsUsedByOtherInbounds(%q): %v", probe, err)
		}
		if !shared["рустам"] {
			t.Errorf("emailsUsedByOtherInbounds(%q) missed the shared Cyrillic email: %v", probe, shared)
		}
		used, err := is.emailUsedByOtherInbounds(database.GetDB(), probe, ibA.Id)
		if err != nil {
			t.Fatalf("emailUsedByOtherInbounds(%q): %v", probe, err)
		}
		if !used {
			t.Errorf("emailUsedByOtherInbounds(%q) = false, want true", probe)
		}
	}
	// Not attached anywhere else: must still report unshared.
	if used, err := is.emailUsedByOtherInbounds(database.GetDB(), "Вадим", ibA.Id); err != nil || used {
		t.Errorf("emailUsedByOtherInbounds(unknown) = %v, %v; want false", used, err)
	}

	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: ibA.Id, Email: "Рустам", Enable: true, ExpiryTime: exp,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if _, err := cs.DelInboundClientByEmail(is, ibA.Id, "Рустам", false, false); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}
	var rows []xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "Рустам").Find(&rows).Error; err != nil {
		t.Fatalf("load traffic: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("traffic rows = %d, want 1 (email still on inbound %d)", len(rows), ibB.Id)
	}
	if rows[0].ExpiryTime != exp || !rows[0].Enable {
		t.Errorf("traffic row = expiry %d enable %v, want %d true", rows[0].ExpiryTime, rows[0].Enable, exp)
	}
}

// A client_traffics row re-created from a node snapshot takes its lifecycle from
// the master client record, not from the node's stale copy.
func TestSeedNodeTrafficRowFromRecordPrefersMasterLifecycle(t *testing.T) {
	setupBulkDB(t)
	cs := &ClientService{}

	now := time.Now().UnixMilli()
	masterExp := now + 30*24*3600*1000
	staleExp := now - 11*24*3600*1000
	seed := []model.Client{
		{ID: "id-r", Email: "Рустам", Enable: true, SubID: "s-r", ExpiryTime: masterExp, TotalGB: 50 << 30},
		{ID: "id-d", Email: "delayed@x", Enable: true, SubID: "s-d", ExpiryTime: -7 * 24 * 3600 * 1000},
		{ID: "id-e", Email: "expired@x", Enable: true, SubID: "s-e", ExpiryTime: staleExp},
	}
	ib := mkInbound(t, 21211, model.VLESS, clientsSettings(t, seed))
	if err := cs.SyncInbound(nil, ib.Id, seed); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	db := database.GetDB()

	t.Run("stale node copy loses to master record", func(t *testing.T) {
		node := xray.ClientTraffic{Email: "Рустам", Enable: false, ExpiryTime: staleExp, Total: 50 << 30}
		row := node
		if err := seedNodeTrafficRowFromRecord(db, &row, node, now); err != nil {
			t.Fatal(err)
		}
		if row.ExpiryTime != masterExp || !row.Enable || row.Total != 50<<30 {
			t.Errorf("row = expiry %d enable %v total %d, want %d true %d", row.ExpiryTime, row.Enable, row.Total, masterExp, int64(50<<30))
		}
	})
	t.Run("node may activate a delayed start", func(t *testing.T) {
		activated := now + 7*24*3600*1000
		node := xray.ClientTraffic{Email: "delayed@x", Enable: true, ExpiryTime: activated}
		row := node
		if err := seedNodeTrafficRowFromRecord(db, &row, node, now); err != nil {
			t.Fatal(err)
		}
		if row.ExpiryTime != activated || !row.Enable {
			t.Errorf("row = expiry %d enable %v, want %d true", row.ExpiryTime, row.Enable, activated)
		}
	})
	t.Run("node disable on master's own limits latches", func(t *testing.T) {
		node := xray.ClientTraffic{Email: "expired@x", Enable: false, ExpiryTime: staleExp}
		row := node
		if err := seedNodeTrafficRowFromRecord(db, &row, node, now); err != nil {
			t.Fatal(err)
		}
		if row.Enable {
			t.Error("genuine node disable was lifted")
		}
	})
	t.Run("no master record keeps node values", func(t *testing.T) {
		node := xray.ClientTraffic{Email: "node-only@x", Enable: false, ExpiryTime: staleExp, Total: 7}
		row := node
		if err := seedNodeTrafficRowFromRecord(db, &row, node, now); err != nil {
			t.Fatal(err)
		}
		if row != node {
			t.Errorf("row changed without a master record: %+v", row)
		}
	})
}
