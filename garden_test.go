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
	if g.Placements[idx].Plant.GrowthStartedAt != 0 {
		t.Fatalf("plant should not grow before watering: %+v", g.Placements[idx].Plant)
	}
	if g.Placements[idx].Plant.Health != defaultPlantHealth {
		t.Fatalf("plant should start healthy: %+v", g.Placements[idx].Plant)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 200); err == nil {
		t.Fatal("expected already planted error")
	}
}

func TestGardenWaterPlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	if err := gardenWaterPlant(&g, "0_0", 120); err != nil {
		t.Fatal(err)
	}

	idx := findPlacementIndex(&g, "0_0")
	if idx < 0 || g.Placements[idx].Plant == nil {
		t.Fatalf("plant missing %+v", g.Placements)
	}
	if g.Placements[idx].Plant.WateredAt != 120 || g.Placements[idx].Plant.GrowthStartedAt != 120 {
		t.Fatalf("plant water state wrong: %+v", g.Placements[idx].Plant)
	}
	if err := gardenWaterPlant(&g, "0_0", 130); err == nil {
		t.Fatal("expected already watered error")
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

	if _, err := gardenHarvestPlant(&g, "0_0", 999); err == nil {
		t.Fatal("expected needs water error")
	}
	if err := gardenWaterPlant(&g, "0_0", 120); err != nil {
		t.Fatal(err)
	}
	if _, err := gardenHarvestPlant(&g, "0_0", 179); err == nil {
		t.Fatal("expected not ready error")
	}

	reward, err := gardenHarvestPlant(&g, "0_0", 180)
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

func TestGardenDiseaseStateCanInfectAndDamagePlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	if err := gardenWaterPlant(&g, "0_0", 100); err != nil {
		t.Fatal(err)
	}

	changed := updateGardenDiseaseStateWithRoller(&g, 100+2*secondsPerHour, func(int) int { return 0 })
	if !changed {
		t.Fatal("expected disease state to change")
	}

	plant := g.Placements[0].Plant
	if plant.Disease == nil || plant.Disease.Type != "stem_borer" {
		t.Fatalf("expected stem_borer disease, got %+v", plant.Disease)
	}
	if plant.Health != 84 {
		t.Fatalf("expected disease damage to reduce health to 84, got %+v", plant.Health)
	}
	if plant.LastCalculatedAt != 100+2*secondsPerHour {
		t.Fatalf("unexpected lastCalculatedAt: %+v", plant.LastCalculatedAt)
	}
}

func TestGardenDiseaseProtectionSkipsInfection(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	if err := gardenWaterPlant(&g, "0_0", 100); err != nil {
		t.Fatal(err)
	}
	g.Placements[0].Plant.DiseaseProtectionUntil = 100 + 2*secondsPerHour

	updateGardenDiseaseStateWithRoller(&g, 100+2*secondsPerHour, func(int) int { return 0 })
	if g.Placements[0].Plant.Disease != nil {
		t.Fatalf("expected protection to prevent disease, got %+v", g.Placements[0].Plant.Disease)
	}
}

func TestGardenDiseaseStateDoesNotReviveDeadPlant(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	g.Placements[0].Plant.Health = 0
	g.Placements[0].Plant.LastCalculatedAt = 100 + secondsPerHour

	updateGardenDiseaseStateWithRoller(&g, 100+2*secondsPerHour, func(int) int { return 0 })
	if g.Placements[0].Plant.Health != 0 {
		t.Fatalf("dead plant should stay dead, got %+v", g.Placements[0].Plant.Health)
	}
}

func TestGardenTreatPlantDisease(t *testing.T) {
	g := defaultPlayerGarden()
	if err := gardenPlacePot(&g, "0_0", "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := gardenPlantSeed(&g, "0_0", "seed_rose", 100); err != nil {
		t.Fatal(err)
	}
	g.Placements[0].Plant.Disease = &PlantDisease{Type: "leaf_spot", Severity: 1, StartedAt: 200}

	if err := gardenTreatPlantDisease(&g, "0_0", "item_pesticide", 300); err != nil {
		t.Fatal(err)
	}
	plant := g.Placements[0].Plant
	if plant.Disease != nil {
		t.Fatalf("expected disease cleared, got %+v", plant.Disease)
	}
	if plant.DiseaseProtectionUntil != 300+12*secondsPerHour {
		t.Fatalf("unexpected protection time: %+v", plant.DiseaseProtectionUntil)
	}
	if err := gardenTreatPlantDisease(&g, "0_0", "flower_rose", 300); err == nil {
		t.Fatal("expected invalid treatment item error")
	}
}
