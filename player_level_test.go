package main

import "testing"

func withTestLevelDefinitions(t *testing.T, defs []LevelDefinition) {
	t.Helper()

	previousDefs := levelDefinitions
	previousExpToNext := expToNextByLevel
	t.Cleanup(func() {
		levelDefinitions = previousDefs
		expToNextByLevel = previousExpToNext
	})

	setLevelDefinitions(defs)
}

func TestAddPlayerExpWithoutLevelUp(t *testing.T) {
	withTestLevelDefinitions(t, []LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 0},
	})
	resources := PlayerResources{Level: 1, Exp: 20}

	result, err := AddPlayerExp(&resources, 30)
	if err != nil {
		t.Fatal(err)
	}
	if resources.Level != 1 || resources.Exp != 50 {
		t.Fatalf("unexpected resources %#v", resources)
	}
	if result.LeveledUp || result.LevelsGained != 0 || result.ExpAfter != 50 {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestAddPlayerExpLevelsUpOnceWithRemainder(t *testing.T) {
	withTestLevelDefinitions(t, []LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 200},
		{Level: 3, ExpToNext: 0},
	})
	resources := PlayerResources{Level: 1, Exp: 80}
	levelDefinitions[1].Rewards = []LevelReward{{ItemID: "seed_rose", Quantity: 2}}

	result, err := AddPlayerExp(&resources, 50)
	if err != nil {
		t.Fatal(err)
	}
	if resources.Level != 2 || resources.Exp != 30 {
		t.Fatalf("unexpected resources %#v", resources)
	}
	if !result.LeveledUp || result.LevelsGained != 1 || result.LevelBefore != 1 || result.LevelAfter != 2 {
		t.Fatalf("unexpected result %#v", result)
	}
	if len(result.Rewards) != 1 || result.Rewards[0].ItemID != "seed_rose" || result.Rewards[0].Quantity != 2 {
		t.Fatalf("unexpected rewards %#v", result.Rewards)
	}
}

func TestAddPlayerExpLevelsUpMultipleTimes(t *testing.T) {
	withTestLevelDefinitions(t, []LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 150},
		{Level: 3, ExpToNext: 300},
		{Level: 4, ExpToNext: 0},
	})
	resources := PlayerResources{Level: 1, Exp: 90}
	levelDefinitions[1].Rewards = []LevelReward{{ItemID: "seed_rose", Quantity: 2}}
	levelDefinitions[2].Rewards = []LevelReward{{ItemID: "seed_rose", Quantity: 3}, {ItemID: "pot_01", Quantity: 1}}

	result, err := AddPlayerExp(&resources, 200)
	if err != nil {
		t.Fatal(err)
	}
	if resources.Level != 3 || resources.Exp != 40 {
		t.Fatalf("unexpected resources %#v", resources)
	}
	if !result.LeveledUp || result.LevelsGained != 2 || result.ExpAfter != 40 {
		t.Fatalf("unexpected result %#v", result)
	}
	if len(result.RewardLevels) != 2 || result.RewardLevels[0] != 2 || result.RewardLevels[1] != 3 {
		t.Fatalf("unexpected reward levels %#v", result.RewardLevels)
	}
	if len(result.Rewards) != 2 {
		t.Fatalf("unexpected rewards %#v", result.Rewards)
	}
	if result.Rewards[0] != (LevelReward{ItemID: "pot_01", Quantity: 1}) || result.Rewards[1] != (LevelReward{ItemID: "seed_rose", Quantity: 5}) {
		t.Fatalf("unexpected aggregated rewards %#v", result.Rewards)
	}
}

func TestAddPlayerExpClampsAtMaxLevel(t *testing.T) {
	withTestLevelDefinitions(t, []LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 0},
	})
	resources := PlayerResources{Level: 1, Exp: 90}

	result, err := AddPlayerExp(&resources, 50)
	if err != nil {
		t.Fatal(err)
	}
	if resources.Level != 2 || resources.Exp != 0 {
		t.Fatalf("unexpected resources %#v", resources)
	}
	if !result.LeveledUp || result.LevelsGained != 1 || result.ExpAfter != 0 {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestAddPlayerExpRejectsInvalidAmount(t *testing.T) {
	resources := PlayerResources{Level: 1}

	if _, err := AddPlayerExp(&resources, 0); err == nil {
		t.Fatal("expected invalid amount to be rejected")
	}
}

func TestGrantLevelUpRewards(t *testing.T) {
	inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: "seed_rose", Quantity: 1}}}
	result := LevelUpResult{
		LeveledUp: true,
		Rewards: []LevelReward{
			{ItemID: "seed_rose", Quantity: 2},
			{ItemID: "pot_01", Quantity: 1},
		},
	}

	if err := GrantLevelUpRewards(&inv, result); err != nil {
		t.Fatal(err)
	}
	want := []InventoryItemStack{
		{ItemID: "pot_01", Quantity: 1},
		{ItemID: "seed_rose", Quantity: 3},
	}
	if len(inv.Items) != len(want) || inv.Items[0] != want[0] || inv.Items[1] != want[1] {
		t.Fatalf("got %#v want %#v", inv.Items, want)
	}
}
