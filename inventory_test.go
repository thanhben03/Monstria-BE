package main

import (
	"reflect"
	"testing"
)

func TestNormalizeInventoryStacks_mergeAndSort(t *testing.T) {
	got, err := normalizeInventoryStacks([]InventoryItemStack{
		{ItemID: "pot_gold", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 2},
		{ItemID: "pot_wood", Quantity: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []InventoryItemStack{
		{ItemID: "pot_gold", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestConsumeOneSeed(t *testing.T) {
	inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: "seed_a", Quantity: 1}}}
	if err := ConsumeOneSeed(&inv, "seed_a"); err != nil {
		t.Fatal(err)
	}
	if len(inv.Items) != 0 {
		t.Fatalf("got %#v", inv.Items)
	}
}

func TestConsumeOnePot(t *testing.T) {
	inv := PlayerInventory{Items: []InventoryItemStack{
		{ItemID: "pot_wood", Quantity: 2},
		{ItemID: "pot_gold", Quantity: 1},
	}}
	if err := ConsumeOnePot(&inv, "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOnePot(&inv, "pot_wood"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOnePot(&inv, "pot_wood"); err == nil {
		t.Fatal("expected error")
	}
}

func TestConsumeOneItem(t *testing.T) {
	inv := PlayerInventory{Items: []InventoryItemStack{
		{ItemID: "item_pesticide", Quantity: 2},
		{ItemID: "flower_rose", Quantity: 1},
	}}
	if err := ConsumeOneItem(&inv, "item_pesticide"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOneItem(&inv, "item_pesticide"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeOneItem(&inv, "item_pesticide"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAddItemMergeAndSort(t *testing.T) {
	inv := PlayerInventory{Items: []InventoryItemStack{{ItemID: "flower_sunflower", Quantity: 1}}}
	if err := AddItem(&inv, "flower_rose", 2); err != nil {
		t.Fatal(err)
	}
	if err := AddItem(&inv, "flower_rose", 3); err != nil {
		t.Fatal(err)
	}

	want := []InventoryItemStack{
		{ItemID: "flower_rose", Quantity: 5},
		{ItemID: "flower_sunflower", Quantity: 1},
	}
	if !reflect.DeepEqual(inv.Items, want) {
		t.Fatalf("got %#v want %#v", inv.Items, want)
	}
}

func TestDefaultInventoryHasItems(t *testing.T) {
	inv := defaultPlayerInventory()
	if inv.Items == nil {
		t.Fatal("expected items slice")
	}
}

func TestDecodePlayerInventoryFlattensLegacyBuckets(t *testing.T) {
	inv, err := decodePlayerInventory([]byte(`{
		"pots": [{"itemId": "pot_wood", "quantity": 2}],
		"seeds": [{"itemId": "seed_rose", "quantity": 3}],
		"items": [
			{"itemId": "item_pesticide", "quantity": 1},
			{"itemId": "seed_rose", "quantity": 4}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []InventoryItemStack{
		{ItemID: "item_pesticide", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 2},
		{ItemID: "seed_rose", Quantity: 7},
	}
	if !reflect.DeepEqual(inv.Items, want) {
		t.Fatalf("got %#v want %#v", inv.Items, want)
	}
}

func TestNormalizePlayerInventoryKeepsCommonItemsOnly(t *testing.T) {
	got := normalizePlayerInventory(PlayerInventory{
		Items: []InventoryItemStack{
			{ItemID: "item_pesticide", Quantity: 1},
			{ItemID: "seed_rose", Quantity: 4},
			{ItemID: "seed_rose", Quantity: 3},
		},
	})
	want := []InventoryItemStack{
		{ItemID: "item_pesticide", Quantity: 1},
		{ItemID: "seed_rose", Quantity: 7},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("got %#v want %#v", got.Items, want)
	}
}

func TestNormalizeInventoryStacks_dropInvalid(t *testing.T) {
	got, err := normalizeInventoryStacks([]InventoryItemStack{
		{ItemID: "", Quantity: 1},
		{ItemID: "pot_wood", Quantity: 0},
		{ItemID: "  x  ", Quantity: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []InventoryItemStack{{ItemID: "x", Quantity: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
