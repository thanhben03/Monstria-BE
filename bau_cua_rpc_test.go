package main

import (
	"reflect"
	"testing"
)

func TestBuildBauCuaBetItemsMergesInventoryStacks(t *testing.T) {
	inv := PlayerInventory{
		Items: []InventoryItemStack{
			{ItemID: "pot_wood", Quantity: 2},
			{ItemID: "seed_rose", Quantity: 3},
			{ItemID: "flower_rose", Quantity: 1},
			{ItemID: "seed_rose", Quantity: 4},
		},
	}
	defs := []ShopItemDefinition{
		{ShopItemID: "shop_seed_rose", NameItem: "Hat hoa hong", GrantItemID: "seed_rose"},
		{ShopItemID: "shop_flower_rose", NameItem: "Hoa hong", GrantItemID: "flower_rose"},
	}

	got := buildBauCuaBetItems(inv, defs)
	want := []bauCuaInventoryItemDTO{
		{ItemID: "flower_rose", DisplayName: "Hoa hong", Quantity: 1},
		{ItemID: "pot_wood", DisplayName: "pot_wood", Quantity: 2},
		{ItemID: "seed_rose", DisplayName: "Hat hoa hong", Quantity: 7},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestDefaultBauCuaSlots(t *testing.T) {
	got := defaultBauCuaSlots()
	if len(got) != 6 {
		t.Fatalf("got %d slots", len(got))
	}
	if got[0].SlotID != "slot_1" || got[0].SymbolID != "bau" {
		t.Fatalf("unexpected first slot: %#v", got[0])
	}
	if got[5].SlotID != "slot_6" || got[5].SymbolID != "cop" {
		t.Fatalf("unexpected last slot: %#v", got[5])
	}
}
