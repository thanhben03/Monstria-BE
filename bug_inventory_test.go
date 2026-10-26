package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func TestCollectBugRPC(t *testing.T) {
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	request := `{"slotId":"0_0","itemId":"item_pesticide"}`
	for _, species := range []string{"buom", "chuonchuon"} {
		for _, mode := range []string{"success", "no item", "conflict", "dead", "no bug", "invalid treatment"} {
			t.Run(species+"/"+mode, func(t *testing.T) {
				g := fertilizerGarden(nowUnixSeconds())
				g.Placements[0].Plant.Disease = &PlantDisease{Type: species, Severity: 3, StartedAt: nowUnixSeconds()}
				bugID := "bug_" + species
				inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: bugID, Quantity: 4}, {ItemID: "item_pesticide", Quantity: 2}}}
				payload := request
				switch mode {
				case "no item":
					inv.Items = inv.Items[:1]
				case "dead":
					g.Placements[0].Plant.Health = 0
				case "no bug":
					g.Placements[0].Plant.Disease = nil
				case "invalid treatment":
					payload = `{"slotId":"0_0","itemId":"bug_buom"}`
				}
				gr, _ := json.Marshal(g)
				ir, _ := json.Marshal(inv)
				store := &fertilizerStorage{conflict: mode == "conflict", objects: []*api.StorageObject{
					{Key: playerGardenKey, Value: string(gr), Version: "garden-v1"},
					{Key: playerInventoryKey, Value: string(ir), Version: "inventory-v1"},
				}}
				result, err := TreatPlantDiseaseRPC(ctx, fertilizerLogger{}, nil, store, payload)
				if mode != "success" {
					if err == nil {
						t.Fatal("expected rejection")
					}
					if store.objects[0].Value != string(gr) || store.objects[1].Value != string(ir) {
						t.Fatal("partial commit")
					}
					if mode != "conflict" && len(store.writes) != 0 {
						t.Fatal("invalid request wrote state")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(store.writes) != 2 || store.writes[0].Version != "inventory-v1" || store.writes[1].Version != "garden-v1" {
					t.Fatal("missing atomic version checks")
				}
				var response treatPlantDiseaseResponse
				if err := json.Unmarshal([]byte(result), &response); err != nil {
					t.Fatal(err)
				}
				if response.Garden.Placements[0].Plant.Disease != nil {
					t.Fatal("bug not cleared")
				}
				if response.BugReward == nil || response.BugReward.ItemID != bugID || response.BugReward.Quantity != 1 {
					t.Fatalf("missing confirmed bug reward for UI: %s", result)
				}
				quantities := map[string]int{}
				for _, stack := range response.Inventory.Items {
					quantities[stack.ItemID] = stack.Quantity
				}
				if quantities[bugID] != 5 || quantities["item_pesticide"] != 1 || len(quantities) != 2 {
					t.Fatalf("wrong reward: %s", result)
				}
				persisted, err := decodePlayerInventory([]byte(store.objects[1].Value))
				if err != nil || len(persisted.Items) != 2 || persisted.Items[0].ItemID != bugID || persisted.Items[0].Quantity != 5 {
					t.Fatalf("reward lost on reload: %+v, %v", persisted, err)
				}
				store.writes = nil
				if _, err := TreatPlantDiseaseRPC(ctx, fertilizerLogger{}, nil, store, request); err == nil {
					t.Fatal("replay accepted")
				}
				if len(store.writes) != 0 {
					t.Fatal("replay wrote state")
				}
			})
		}
	}
}

func TestBugTypesRemainDistinct(t *testing.T) {
	got := normalizeDiseaseTypes([]string{"borua", "buom", "bocanhcung", "chuonchuon"})
	if len(got) != 4 || got[0] != "borua" || got[1] != "buom" || got[2] != "bocanhcung" || got[3] != "chuonchuon" {
		t.Fatalf("species identities changed: %v", got)
	}
	g := fertilizerGarden(1000)
	g.Placements[0].Plant.Disease = &PlantDisease{Type: "borua", Severity: 1, StartedAt: 999}
	updateGardenDiseaseState(&g, 1000)
	if g.Placements[0].Plant.Disease.Type != "borua" {
		t.Fatal("persisted species changed")
	}
}
