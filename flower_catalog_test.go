package main

import "testing"

func TestListFlowerDefinitionsSortedCopy(t *testing.T) {
	got := listFlowerDefinitions()
	if len(got) < 2 {
		t.Fatalf("expected sample flower definitions, got %#v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].SeedItemID > got[i].SeedItemID {
			t.Fatalf("definitions are not sorted: %#v", got)
		}
	}

	got[0].SeedItemID = "changed"
	again := listFlowerDefinitions()
	if again[0].SeedItemID == "changed" {
		t.Fatal("listFlowerDefinitions returned mutable backing data")
	}
}

func TestFlowerDefinitionForSeed(t *testing.T) {
	def, err := flowerDefinitionForSeed(" seed_rose ")
	if err != nil {
		t.Fatal(err)
	}
	if def.RewardItemID != "flower_rose" || def.RewardQuantity != 1 || def.GrowSeconds != 60 || def.ExpReward != 20 {
		t.Fatalf("got %#v", def)
	}
	if len(def.Disease) != 1 || def.Disease[0] != "borua" {
		t.Fatalf("got disease config %#v", def.Disease)
	}

	if _, err := flowerDefinitionForSeed("seed_unknown"); err == nil {
		t.Fatal("expected unknown seed error")
	}
}

func TestFlowerDefinitionIncludesTuyetDuong(t *testing.T) {
	def, err := flowerDefinitionForSeed("seed_tuyetduong")
	if err != nil {
		t.Fatal(err)
	}
	if def.RewardItemID != "flower_tuyetduong" || def.RewardQuantity != 1 || def.GrowSeconds != 60 || def.ExpReward != 25 {
		t.Fatalf("got %#v", def)
	}
}
