package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type harvestBugStorage struct{ fertilizerStorage }

func (s *harvestBugStorage) StorageRead(_ context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	var result []*api.StorageObject
	for _, read := range reads {
		for _, object := range s.objects {
			if object.Key == read.Key {
				result = append(result, object)
			}
		}
	}
	return result, nil
}

func (*harvestBugStorage) AccountGetId(context.Context, string) (*api.Account, error) {
	return &api.Account{Wallet: `{}`}, nil
}

func TestHarvestBugRewardAtomicAndFree(t *testing.T) {
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	for _, species := range []string{"buom", "chuonchuon"} {
		for _, mode := range []string{"success", "no bug", "dead", "not ready", "conflict"} {
			t.Run(species+"/"+mode, func(t *testing.T) {
				now := nowUnixSeconds()
				g := fertilizerGarden(now)
				plant := g.Placements[0].Plant
				plant.GrowthStartedAt = now - 120
				plant.Disease = &PlantDisease{Type: species, Severity: 1, StartedAt: now}
				switch mode {
				case "no bug":
					plant.Disease = nil
				case "dead":
					plant.Health = 0
				case "not ready":
					plant.GrowthStartedAt = now
				}
				// No pesticide in inventory: harvest must still collect the bug.
				inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: "bug_" + species, Quantity: 2}}}
				gr, _ := json.Marshal(g)
				ir, _ := json.Marshal(inv)
				store := &harvestBugStorage{fertilizerStorage{conflict: mode == "conflict", objects: []*api.StorageObject{
					{Key: playerGardenKey, Value: string(gr), Version: "g1"},
					{Key: playerInventoryKey, Value: string(ir), Version: "i1"},
					{Key: playerStateKey, Value: `{"level":1,"exp":0,"walletMigrated":true}`, Version: "r1"},
				}}}
				payload := `{"slotId":"0_0"}`
				result, err := HarvestPlantInPotRPC(ctx, fertilizerLogger{}, nil, store, payload)
				if mode == "dead" || mode == "not ready" || mode == "conflict" {
					if err == nil {
						t.Fatal("invalid harvest accepted")
					}
					if store.objects[0].Value != string(gr) || store.objects[1].Value != string(ir) {
						t.Fatal("partial commit")
					}
					if mode != "conflict" && len(store.writes) != 0 {
						t.Fatal("invalid harvest wrote state")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(store.writes) != 3 || store.writes[0].Version != "i1" || store.writes[1].Version != "g1" || store.writes[2].Version != "r1" {
					t.Fatal("missing atomic version checks")
				}
				var response harvestPlantResponse
				if err := json.Unmarshal([]byte(result), &response); err != nil {
					t.Fatal(err)
				}
				if response.Garden.Placements[0].Plant != nil {
					t.Fatal("plant not removed")
				}
				want := 3
				if mode == "no bug" {
					want = 2
					if response.BugReward != nil {
						t.Fatal("invented bug reward")
					}
				} else if response.BugReward == nil || response.BugReward.ItemID != "bug_"+species || response.BugReward.Quantity != 1 {
					t.Fatalf("missing bug reward: %s", result)
				}
				quantities := map[string]int{}
				for _, stack := range response.Inventory.Items {
					quantities[stack.ItemID] = stack.Quantity
				}
				if quantities["bug_"+species] != want || quantities["flower_rose"] != 1 {
					t.Fatalf("wrong rewards: %s", result)
				}
				persisted, err := decodePlayerInventory([]byte(store.objects[1].Value))
				if err != nil || persisted.Items[0].Quantity != want {
					t.Fatal("reward lost on reload")
				}
				store.writes = nil
				if _, err := HarvestPlantInPotRPC(ctx, fertilizerLogger{}, nil, store, payload); err == nil || len(store.writes) != 0 {
					t.Fatal("replay granted rewards")
				}
			})
		}
	}
}

func TestDestroyDeadPlantWithBugGrantsNothing(t *testing.T) {
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	g := fertilizerGarden(nowUnixSeconds())
	g.Placements[0].Plant.Health = 0
	g.Placements[0].Plant.Disease = &PlantDisease{Type: "buom", Severity: 1}
	gr, _ := json.Marshal(g)
	store := &harvestBugStorage{fertilizerStorage{objects: []*api.StorageObject{{Key: playerGardenKey, Value: string(gr), Version: "g1"}}}}
	if _, err := DestroyDeadPlantRPC(ctx, fertilizerLogger{}, nil, store, `{"slotId":"0_0"}`); err != nil {
		t.Fatal(err)
	}
	if len(store.writes) != 1 || store.writes[0].Key != playerGardenKey {
		t.Fatal("dead cleanup must only write garden")
	}
}
