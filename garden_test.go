package main

import (
	"encoding/json"
	"testing"
)

func TestSlotPlacementLegacyJSON(t *testing.T) {
	const legacy = `{"slotId":"0_1","itemId":"pot_wood"}`
	var p SlotPlacement
	if err := json.Unmarshal([]byte(legacy), &p); err != nil {
		t.Fatal(err)
	}
	if p.PotItemID != "pot_wood" || p.SlotID != "0_1" {
		t.Fatalf("got %+v", p)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var again SlotPlacement
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatal(err)
	}
	if again.PotItemID != "pot_wood" {
		t.Fatalf("roundtrip %+v", string(raw))
	}
}

func TestGardenPlacePotThenPlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	idx := findPlacementIndex(&g, "0_0")
	if idx < 0 || g.Placements[idx].Plant == nil || g.Placements[idx].Plant.SeedItemID != "seed_rose" {
		t.Fatalf("plant missing %+v", g.Placements)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 200); err == nil {
		t.Fatal("expected already planted error")
	}
}

func TestGardenHarvestPlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}

	if _, err := gardenHarvestPlant(&g, "0_0", 159); err == nil {
		t.Fatal("expected not ready error")
	}

	reward, err := gardenHarvestPlant(&g, "0_0", 160)
	if err != nil {
		t.Fatal(err)
	}
	if reward.ItemID != "flower_rose" || reward.Quantity != 1 {
		t.Fatalf("got reward %#v", reward)
	}

	idx := findPlacementIndex(&g, "0_0")
	if idx < 0 {
		t.Fatal("expected pot to remain after harvest")
	}
	if g.Placements[idx].PotItemID != "pot_wood" {
		t.Fatalf("pot changed after harvest: %#v", g.Placements[idx])
	}
	if g.Placements[idx].Plant != nil {
		t.Fatalf("expected plant cleared after harvest: %#v", g.Placements[idx].Plant)
	}
	if _, err := gardenHarvestPlant(&g, "0_0", 999); err == nil {
		t.Fatal("expected no plant error")
	}
}

func TestGardenHarvestRequiresPlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if _, err := gardenHarvestPlant(&g, "0_0", 100); err == nil {
		t.Fatal("expected no plant error")
	}
	if _, err := gardenHarvestPlant(&g, "0_1", 100); err == nil {
		t.Fatal("expected no pot error")
	}
}
