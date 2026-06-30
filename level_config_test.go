package main

import "testing"

func TestParseLevelDefinitions(t *testing.T) {
	raw := []byte(`{"levels":[{"level":2,"expToNext":150,"rewards":[{"itemId":"seed_rose","quantity":2}]},{"level":1,"expToNext":100},{"level":3,"expToNext":0}]}`)

	defs, err := parseLevelDefinitions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 3 {
		t.Fatalf("got %d definitions", len(defs))
	}
	if defs[0].Level != 1 || defs[0].ExpToNext != 100 {
		t.Fatalf("unexpected first level %#v", defs[0])
	}
	if defs[2].Level != 3 || defs[2].ExpToNext != 0 {
		t.Fatalf("unexpected max level %#v", defs[2])
	}
	if len(defs[1].Rewards) != 1 || defs[1].Rewards[0].ItemID != "seed_rose" || defs[1].Rewards[0].Quantity != 2 {
		t.Fatalf("unexpected rewards %#v", defs[1].Rewards)
	}
}

func TestParseLevelDefinitionsRejectsGaps(t *testing.T) {
	raw := []byte(`{"levels":[{"level":1,"expToNext":100},{"level":3,"expToNext":0}]}`)

	if _, err := parseLevelDefinitions(raw); err == nil {
		t.Fatal("expected level gap to be rejected")
	}
}

func TestParseLevelDefinitionsRejectsInvalidRewards(t *testing.T) {
	raw := []byte(`{"levels":[{"level":1,"expToNext":100,"rewards":[{"itemId":"","quantity":1}]},{"level":2,"expToNext":0}]}`)

	if _, err := parseLevelDefinitions(raw); err == nil {
		t.Fatal("expected empty reward itemId to be rejected")
	}

	raw = []byte(`{"levels":[{"level":1,"expToNext":100,"rewards":[{"itemId":"seed_rose","quantity":0}]},{"level":2,"expToNext":0}]}`)
	if _, err := parseLevelDefinitions(raw); err == nil {
		t.Fatal("expected invalid reward quantity to be rejected")
	}
}

func TestDecoratePlayerResources(t *testing.T) {
	previousDefs := levelDefinitions
	previousExpToNext := expToNextByLevel
	t.Cleanup(func() {
		levelDefinitions = previousDefs
		expToNextByLevel = previousExpToNext
	})

	setLevelDefinitions([]LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 0},
	})

	resources := decoratePlayerResources(PlayerResources{Level: 1, Exp: 25})
	if resources.ExpToNextLevel != 100 {
		t.Fatalf("got expToNext %d", resources.ExpToNextLevel)
	}
	if resources.ExpProgress != 0.25 {
		t.Fatalf("got expProgress %f", resources.ExpProgress)
	}

	resources = decoratePlayerResources(PlayerResources{Level: 2, Exp: 0})
	if resources.ExpToNextLevel != 0 || resources.ExpProgress != 1 {
		t.Fatalf("unexpected max level resources %#v", resources)
	}
}
