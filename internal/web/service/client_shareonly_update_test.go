package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestUpdateAfterShareOnlyAttach(t *testing.T) {
	for _, proto := range []model.Protocol{model.Qwdtt, model.Olcrtc, model.Tproxy} {
		t.Run(string(proto), func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			source := []model.Client{{
				Email:  "fox",
				ID:     "aaaaaaaa-0000-0000-0000-0000000000aa",
				SubID:  "sub-fox",
				Enable: true,
			}}
			vless := mkInbound(t, 22101, model.VLESS, clientsSettings(t, source))
			if err := svc.SyncInbound(nil, vless.Id, source); err != nil {
				t.Fatalf("seed vless: %v", err)
			}
			share := mkInbound(t, 56001, proto, `{"remark":"share"}`)
			rec := lookupClientRecord(t, "fox")

			if _, err := svc.Attach(inboundSvc, rec.Id, []int{share.Id}); err != nil {
				t.Fatalf("Attach %s: %v", proto, err)
			}

			updated := source[0]
			updated.TotalGB = 11
			if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
				t.Fatalf("Update after %s attach: %v", proto, err)
			}
			if got := lookupClientRecord(t, "fox").TotalGB; got != 11 {
				t.Fatalf("totalGB after update = %d, want 11", got)
			}
			if got := mustInboundSettings(t, inboundSvc, share.Id); got != `{"remark":"share"}` {
				t.Fatalf("%s settings mutated: %s", proto, got)
			}
		})
	}
}

func TestUpdateShareOnlyOnlyClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true}}
	share := mkInbound(t, 56002, model.Qwdtt, `{"remark":"qwdtt"}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     source[0],
		InboundIds: []int{share.Id},
	}); err != nil {
		t.Fatalf("Create on qWDTT: %v", err)
	}
	rec := lookupClientRecord(t, "fox")
	updated := source[0]
	updated.TotalGB = 7
	updated.Comment = "ok"
	if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
		t.Fatalf("Update qWDTT-only: %v", err)
	}
	got := lookupClientRecord(t, "fox")
	if got.TotalGB != 7 || got.Comment != "ok" {
		t.Fatalf("record after update totalGB=%d comment=%q", got.TotalGB, got.Comment)
	}
}

func TestTproxyInboundSaveMustNotSyncEmptyClients(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true, ID: "aaaaaaaa-0000-0000-0000-0000000000aa"}}
	vless := mkInbound(t, 22102, model.VLESS, clientsSettings(t, source))
	if err := svc.SyncInbound(nil, vless.Id, source); err != nil {
		t.Fatal(err)
	}
	share := mkInbound(t, 443, model.Tproxy, `{"hostname":"x.example","secret":"000102030405060708090a0b0c0d0e0f"}`)
	rec := lookupClientRecord(t, "fox")
	if _, err := svc.Attach(inboundSvc, rec.Id, []int{share.Id}); err != nil {
		t.Fatal(err)
	}
	if !shareOnlySidecar(model.Tproxy) {
		t.Fatal("tproxy must be share-only: GetClients is empty and SyncInbound would drop attaches")
	}
	clients, err := inboundSvc.GetClients(share)
	if err != nil || len(clients) != 0 {
		t.Fatalf("tproxy GetClients = %d %v", len(clients), err)
	}
	got, err := svc.ListForInbound(nil, share.Id)
	if err != nil || len(got) != 1 || got[0].Email != "fox" {
		t.Fatalf("attach after tproxy save-guard: %+v %v", got, err)
	}
}

// A client that lives only on share-only sidecar inbounds (qWDTT here) must
// get its client_traffics row refreshed on edit: the subscription header reads
// expiry/total/enable from that row, so a stale one kept the old deadline
// after the date was changed in the panel.
func TestUpdateShareOnlyOnlyClientRefreshesTrafficRow(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const oldExpiry, newExpiry = int64(1791106064000), int64(1793784464000)
	source := []model.Client{{Email: "fox", SubID: "sub-fox", Enable: true, ExpiryTime: oldExpiry, TotalGB: 5}}
	share := mkInbound(t, 56003, model.Qwdtt, `{"remark":"qwdtt"}`)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client:     source[0],
		InboundIds: []int{share.Id},
	}); err != nil {
		t.Fatalf("Create on qWDTT: %v", err)
	}
	// The expiry job latched the client off at the old deadline.
	if err := database.GetDB().Model(xray.ClientTraffic{}).
		Where("email = ?", "fox").Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}

	rec := lookupClientRecord(t, "fox")
	updated := source[0]
	updated.ExpiryTime = newExpiry
	updated.TotalGB = 9
	if _, err := svc.Update(inboundSvc, rec.Id, updated, 0); err != nil {
		t.Fatalf("Update qWDTT-only: %v", err)
	}
	if got := lookupClientRecord(t, "fox").ExpiryTime; got != newExpiry {
		t.Fatalf("client record expiry = %d, want %d", got, newExpiry)
	}
	var ct xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "fox").First(&ct).Error; err != nil {
		t.Fatalf("traffic row: %v", err)
	}
	if ct.ExpiryTime != newExpiry || ct.Total != 9 || !ct.Enable {
		t.Fatalf("traffic row after update expiry=%d total=%d enable=%v, want %d/9/true",
			ct.ExpiryTime, ct.Total, ct.Enable, newExpiry)
	}
}
