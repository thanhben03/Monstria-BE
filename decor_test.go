package main

import "testing"

func TestNormalizeDecorPlacements(t *testing.T) {
	got := normalizeDecorPlacements([]DecorPlacement{
		{SlotID: "1_0", ItemID: "decor_b"},
		{SlotID: "", ItemID: "decor_skip"},
		{SlotID: "0_0", ItemID: "decor_a"},
		{SlotID: "0_0", ItemID: "decor_a2"},
		{SlotID: "2_0", ItemID: ""},
	})

	want := []DecorPlacement{
		{SlotID: "0_0", ItemID: "decor_a2"},
		{SlotID: "1_0", ItemID: "decor_b"},
	}

	if len(got) != len(want) {
		t.Fatalf("len got %d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d got %#v want %#v", i, got[i], want[i])
		}
	}
}

func TestDecorPlaceItemRejectsOccupiedSlot(t *testing.T) {
	decor := defaultPlayerDecor()
	if err := decorPlaceItem(&decor, "0_0", "decor_den_ngoi_sao"); err != nil {
		t.Fatalf("place decor: %v", err)
	}
	if err := decorPlaceItem(&decor, "0_0", "decor_chuong_gio"); err == nil {
		t.Fatal("expected occupied slot error")
	}
	if len(decor.Placements) != 1 || decor.Placements[0].ItemID != "decor_den_ngoi_sao" {
		t.Fatalf("unexpected decor state: %#v", decor.Placements)
	}
}

func TestDecorRemoveItemReturnsItemAndClearsSlot(t *testing.T) {
	decor := PlayerDecor{Placements: []DecorPlacement{
		{SlotID: "0_0", ItemID: "decor_chuong_gio"},
		{SlotID: "1_0", ItemID: "decor_den_ngoi_sao"},
	}}

	itemID, err := decorRemoveItem(&decor, "0_0")
	if err != nil {
		t.Fatalf("remove decor: %v", err)
	}
	if itemID != "decor_chuong_gio" {
		t.Fatalf("itemID got %q want %q", itemID, "decor_chuong_gio")
	}
	if len(decor.Placements) != 1 || decor.Placements[0].SlotID != "1_0" {
		t.Fatalf("unexpected decor state: %#v", decor.Placements)
	}
}

func TestDecorRemoveItemRejectsEmptySlot(t *testing.T) {
	decor := PlayerDecor{Placements: []DecorPlacement{
		{SlotID: "0_0", ItemID: "decor_chuong_gio"},
	}}

	if _, err := decorRemoveItem(&decor, "2_0"); err == nil {
		t.Fatal("expected empty slot error")
	}
}
func TestDecorRemoveLayerItemsReturnsItemsAndKeepsOtherLayers(t *testing.T) {
	decor := PlayerDecor{Placements: []DecorPlacement{
		{SlotID: "0_0", ItemID: "decor_chuong_gio"},
		{SlotID: "0_1", ItemID: "decor_den_ngoi_sao"},
		{SlotID: "1_0", ItemID: "decor_khung_tranh"},
	}}

	itemIDs, err := decorRemoveLayerItems(&decor, "0_4")
	if err != nil {
		t.Fatalf("remove layer decor: %v", err)
	}
	if len(itemIDs) != 2 {
		t.Fatalf("itemIDs len got %d want 2: %#v", len(itemIDs), itemIDs)
	}
	if itemIDs[0] != "decor_chuong_gio" || itemIDs[1] != "decor_den_ngoi_sao" {
		t.Fatalf("unexpected itemIDs: %#v", itemIDs)
	}
	if len(decor.Placements) != 1 || decor.Placements[0].SlotID != "1_0" {
		t.Fatalf("unexpected decor state: %#v", decor.Placements)
	}
}

func TestDecorRemoveLayerItemsRejectsEmptyLayer(t *testing.T) {
	decor := PlayerDecor{Placements: []DecorPlacement{{SlotID: "0_0", ItemID: "decor_chuong_gio"}}}
	if _, err := decorRemoveLayerItems(&decor, "2_0"); err == nil {
		t.Fatal("expected empty layer error")
	}
}
