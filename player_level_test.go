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
}

func TestAddPlayerExpLevelsUpMultipleTimes(t *testing.T) {
	withTestLevelDefinitions(t, []LevelDefinition{
		{Level: 1, ExpToNext: 100},
		{Level: 2, ExpToNext: 150},
		{Level: 3, ExpToNext: 300},
		{Level: 4, ExpToNext: 0},
	})
	resources := PlayerResources{Level: 1, Exp: 90}

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
