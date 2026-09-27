package service

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInboundSettingsClients(t *testing.T) {
	arr := []any{map[string]any{"email": "a"}}
	cases := []struct {
		name     string
		settings map[string]any
		wantLen  int
		wantOK   bool
	}{
		{"missing key", map[string]any{"password": "p"}, 0, true},
		{"null", map[string]any{"clients": nil}, 0, true},
		{"array", map[string]any{"clients": arr}, 1, true},
		{"object", map[string]any{"clients": map[string]any{}}, 0, false},
		{"string", map[string]any{"clients": "x"}, 0, false},
	}
	for _, tc := range cases {
		got, ok := inboundSettingsClients(tc.settings)
		if ok != tc.wantOK || len(got) != tc.wantLen {
			t.Errorf("%s: got len=%d ok=%v, want len=%d ok=%v", tc.name, len(got), ok, tc.wantLen, tc.wantOK)
		}
	}
}

// sidecarInbound creates a share-only sidecar inbound with the given raw
// settings (no per-client entries) and links the clients via client_inbounds,
// the way qWDTT/olcRTC inbounds are stored on a master and after node adoption.
func sidecarInbound(t *testing.T, nodeID *int, port int, proto model.Protocol, settings string, clients []model.Client) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId: 1, NodeID: nodeID, Tag: fmt.Sprintf("sc-in-%d", port), Enable: true,
		Port: port, Protocol: proto, Settings: settings,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create sidecar inbound: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed SyncInbound: %v", err)
	}
	return ib
}

func inboundSettingsOf(t *testing.T, id int) string {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, id).Error; err != nil {
		t.Fatalf("read inbound %d: %v", id, err)
	}
	return ib.Settings
}

// Deleting a client attached to qWDTT/olcRTC inbounds whose settings carry no
// "clients" key (node-adopted) or an empty one used to fail with "invalid
// clients format in inbound settings". It must succeed, push the full delete
// to the node, keep the sidecar settings untouched and leave other clients
// on the same inbounds alone.
func TestDelete_SidecarInboundsWithoutClientsKey(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)

	victim := model.Client{ID: uuid.NewString(), Email: "sc-victim@x", SubID: "scvictim", Enable: true}
	keep := model.Client{ID: uuid.NewString(), Email: "sc-keep@x", SubID: "sckeep", Enable: true}

	const nodeSettings = `{"remark":"FI","password":"p","subHost":"","workers":16,"clientPort":9000}`
	const localSettings = `{"remark":"NL","password":"p","subHost":"1.2.3.4:56000","clients":[]}`
	local := sidecarInbound(t, nil, 56000, model.Qwdtt, localSettings, []model.Client{victim, keep})
	nodeIb := sidecarInbound(t, &nodeID, 56100, model.Qwdtt, nodeSettings, []model.Client{victim, keep})
	olc := sidecarInbound(t, nil, 56200, model.Olcrtc, `{"roomId":"r"}`, []model.Client{victim})

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	rec, err := svc.GetRecordByEmail(nil, victim.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.GetRecordByEmail(nil, victim.Email); err == nil {
		t.Fatalf("victim record still exists after Delete")
	}
	if got := fake.deleteClient.Load(); got != 1 {
		t.Fatalf("node full-delete RPCs = %d, want 1", got)
	}
	if got := inboundSettingsOf(t, nodeIb.Id); got != nodeSettings {
		t.Errorf("node sidecar settings rewritten:\n got %s\nwant %s", got, nodeSettings)
	}
	if got := inboundSettingsOf(t, local.Id); got != localSettings {
		t.Errorf("local sidecar settings rewritten:\n got %s\nwant %s", got, localSettings)
	}

	keepRec, err := svc.GetRecordByEmail(nil, keep.Email)
	if err != nil {
		t.Fatalf("other client lost: %v", err)
	}
	ids, err := svc.GetInboundIdsForRecord(keepRec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if !slices.Contains(ids, local.Id) || !slices.Contains(ids, nodeIb.Id) || slices.Contains(ids, olc.Id) {
		t.Errorf("other client's links changed: %v (local=%d node=%d olc=%d)", ids, local.Id, nodeIb.Id, olc.Id)
	}
}

// Detaching from a node sidecar inbound without a clients key goes through the
// per-inbound detach RPC and drops the link.
func TestDetach_SidecarNodeInboundWithoutClientsKey(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)

	c := model.Client{ID: uuid.NewString(), Email: "sc-detach@x", SubID: "scdetach", Enable: true}
	nodeIb := sidecarInbound(t, &nodeID, 56300, model.Qwdtt, `{"password":"p"}`, []model.Client{c})

	svc := &ClientService{}
	rec, err := svc.GetRecordByEmail(nil, c.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if _, err := svc.Detach(&InboundService{}, rec.Id, []int{nodeIb.Id}); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	if got := fake.deleteUser.Load(); got != 1 {
		t.Fatalf("detach RPCs = %d, want 1", got)
	}
	ids, err := svc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if slices.Contains(ids, nodeIb.Id) {
		t.Fatalf("link to sidecar inbound survived detach: %v", ids)
	}
}

// The batch detach path (BulkDetach -> delInboundClients) tolerates the
// missing clients key as well and removes the link.
func TestBulkDetach_SidecarInboundWithoutClientsKey(t *testing.T) {
	setupBulkDB(t)
	c := model.Client{ID: uuid.NewString(), Email: "sc-bulk@x", SubID: "scbulk", Enable: true}
	ib := sidecarInbound(t, nil, 56400, model.Csqtt, `{"password":"p"}`, []model.Client{c})

	svc := &ClientService{}
	if _, _, err := svc.BulkDetach(&InboundService{}, []string{c.Email}, []int{ib.Id}); err != nil {
		t.Fatalf("BulkDetach: %v", err)
	}
	rec, err := svc.GetRecordByEmail(nil, c.Email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	ids, err := svc.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		t.Fatalf("GetInboundIdsForRecord: %v", err)
	}
	if slices.Contains(ids, ib.Id) {
		t.Fatalf("link to sidecar inbound survived bulk detach: %v", ids)
	}
	if got := inboundSettingsOf(t, ib.Id); got != `{"password":"p"}` {
		t.Errorf("sidecar settings rewritten: %s", got)
	}
}

// A clients value that is present but not an array is still rejected.
func TestDelInboundClientByEmail_RejectsNonArrayClients(t *testing.T) {
	setupBulkDB(t)
	ib := &model.Inbound{UserId: 1, Tag: "bad-clients", Enable: true, Port: 56500, Protocol: model.VLESS, Settings: `{"clients":{"email":"x@x"}}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if _, err := (&ClientService{}).DelInboundClientByEmail(&InboundService{}, ib.Id, "x@x", false, true); err == nil {
		t.Fatalf("want an error for a non-array clients value")
	}
}
