package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

func fertilizerGarden(now int64) PlayerGarden {
	return PlayerGarden{Placements: []SlotPlacement{{SlotID: "0_0", PotItemID: "pot_wood", Plant: &PotPlant{
		SeedItemID: "seed_rose", PlantedAt: now - 40, WateredAt: now - 40, GrowthStartedAt: now - 40,
		Health: 100, LastCalculatedAt: now, DiseaseProtectionUntil: now + 10000,
	}}}}
}

func TestFertilizerRemainingTimeAndPersistence(t *testing.T) {
	def, _ := flowerDefinitionForSeed("seed_rose")
	for id, rate := range fertilizerRates {
		t.Run(id, func(t *testing.T) {
			g := fertilizerGarden(1000)
			remaining := float64(def.GrowSeconds - 40)
			if err := gardenFertilizePlant(&g, "0_0", id, 1000); err != nil {
				t.Fatal(err)
			}
			plant := g.Placements[0].Plant
			if !plant.Fertilized || math.Abs(plant.GrowthReductionSeconds-remaining*rate) > 1e-9 {
				t.Fatalf("wrong reduction: %+v", plant)
			}
			raw, _ := json.Marshal(g)
			var loaded PlayerGarden
			if err := json.Unmarshal(raw, &loaded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(g, loaded) {
				t.Fatal("fertilizer state lost on reload")
			}
			if err := gardenFertilizePlant(&loaded, "0_0", "fertilizer_50", 1001); err == nil {
				t.Fatal("repeat accepted")
			}
			readyAt := int64(math.Ceil(float64(plant.GrowthStartedAt+def.GrowSeconds) - plant.GrowthReductionSeconds))
			if _, err := gardenHarvestPlant(&loaded, "0_0", readyAt-1); err == nil {
				t.Fatal("early harvest accepted")
			}
			if _, err := gardenHarvestPlant(&loaded, "0_0", readyAt); err != nil {
				t.Fatal(err)
			}
			if err := gardenPlantSeed(&loaded, "0_0", "seed_rose", readyAt); err != nil {
				t.Fatal(err)
			}
			if loaded.Placements[0].Plant.Fertilized {
				t.Fatal("new crop inherited fertilizer")
			}
			if err := gardenWaterPlant(&loaded, "0_0", readyAt); err != nil {
				t.Fatal(err)
			}
			if err := gardenFertilizePlant(&loaded, "0_0", id, readyAt); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFertilizerRejectsInvalidPlants(t *testing.T) {
	cases := map[string]func(*PlayerGarden){
		"needs water":        func(g *PlayerGarden) { g.Placements[0].Plant.GrowthStartedAt = 0 },
		"dead":               func(g *PlayerGarden) { g.Placements[0].Plant.Health = 0 },
		"already fertilized": func(g *PlayerGarden) { g.Placements[0].Plant.Fertilized = true },
		"mature":             func(g *PlayerGarden) { g.Placements[0].Plant.GrowthStartedAt = 1 },
		"empty pot":          func(g *PlayerGarden) { g.Placements[0].Plant = nil },
		"empty slot":         func(g *PlayerGarden) { g.Placements = nil },
	}
	for name, modify := range cases {
		t.Run(name, func(t *testing.T) {
			g := fertilizerGarden(1000)
			modify(&g)
			before, _ := json.Marshal(g)
			if err := gardenFertilizePlant(&g, "0_0", "fertilizer_15", 1000); err == nil {
				t.Fatal("accepted invalid plant")
			}
			after, _ := json.Marshal(g)
			if string(before) != string(after) {
				t.Fatal("rejection mutated plant")
			}
		})
	}
	g := fertilizerGarden(1000)
	if err := gardenFertilizePlant(&g, "0_0", "item_pesticide", 1000); err == nil {
		t.Fatal("accepted non fertilizer")
	}
}

type fertilizerStorage struct {
	runtime.NakamaModule
	objects  []*api.StorageObject
	writes   []*runtime.StorageWrite
	conflict bool
}

func (s *fertilizerStorage) StorageRead(context.Context, []*runtime.StorageRead) ([]*api.StorageObject, error) {
	return s.objects, nil
}
func (s *fertilizerStorage) StorageWrite(_ context.Context, writes []*runtime.StorageWrite) ([]*api.StorageObjectAck, error) {
	s.writes = writes
	if s.conflict {
		return nil, errors.New("version conflict")
	}
	for _, w := range writes {
		for _, o := range s.objects {
			if o.Key == w.Key {
				o.Value = w.Value
				o.Version = "next"
			}
		}
	}
	return nil, nil
}

type fertilizerLogger struct{ runtime.Logger }

func (fertilizerLogger) Error(string, ...interface{}) {}

func TestFertilizerRPCAtomicInventoryAndReplay(t *testing.T) {
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	request := `{"slotId":"0_0","itemId":"fertilizer_50"}`
	for _, mode := range []string{"success", "empty inventory", "conflict", "needs water"} {
		t.Run(mode, func(t *testing.T) {
			g := fertilizerGarden(nowUnixSeconds())
			inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: "fertilizer_50", Quantity: 2}}}
			if mode == "empty inventory" {
				inv.Items = nil
			}
			if mode == "needs water" {
				g.Placements[0].Plant.GrowthStartedAt = 0
			}
			gr, _ := json.Marshal(g)
			ir, _ := json.Marshal(inv)
			store := &fertilizerStorage{conflict: mode == "conflict", objects: []*api.StorageObject{
				{Key: playerGardenKey, Value: string(gr), Version: "garden-v1"},
				{Key: playerInventoryKey, Value: string(ir), Version: "inventory-v1"},
			}}
			result, err := FertilizePlantInPotRPC(ctx, fertilizerLogger{}, nil, store, request)
			if mode != "success" {
				if err == nil {
					t.Fatal("expected rejection")
				}
				if store.objects[0].Value != string(gr) || store.objects[1].Value != string(ir) {
					t.Fatal("partial commit")
				}
				if mode != "conflict" && len(store.writes) != 0 {
					t.Fatal("invalid request attempted write")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(store.writes) != 2 || store.writes[0].Version != "inventory-v1" || store.writes[1].Version != "garden-v1" {
				t.Fatal("missing atomic version checks")
			}
			var response fertilizePlantResponse
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatal(err)
			}
			if response.Inventory.Items[0].Quantity != 1 || !response.Garden.Placements[0].Plant.Fertilized {
				t.Fatalf("bad response: %s", result)
			}
			store.writes = nil
			if _, err := FertilizePlantInPotRPC(ctx, fertilizerLogger{}, nil, store, request); err == nil {
				t.Fatal("replay consumed another item")
			}
			if len(store.writes) != 0 {
				t.Fatal("replay wrote state")
			}
		})
	}
	if _, err := FertilizePlantInPotRPC(context.Background(), nil, nil, nil, request); err == nil {
		t.Fatal("unauthorized allowed")
	}
	if _, err := FertilizePlantInPotRPC(ctx, nil, nil, nil, "{"); err == nil {
		t.Fatal("malformed JSON allowed")
	}
}
