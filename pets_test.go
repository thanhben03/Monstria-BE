package main

import "testing"

func TestPetPlaceAndStore(t *testing.T) {
	pets := defaultPlayerPets()
	if err := petPlaceItem(&pets, 0, "pet_monkey", 2); err != nil {
		t.Fatalf("place pet: %v", err)
	}
	if len(pets.Placements) != 1 || pets.Placements[0] != (PetPlacement{LayerIndex: 0, ItemID: "pet_monkey"}) {
		t.Fatalf("unexpected placements: %#v", pets.Placements)
	}
	if err := petPlaceItem(&pets, 0, "pet_monkey", 2); err == nil {
		t.Fatal("expected occupied layer error")
	}
	itemID, err := petRemoveItem(&pets, 0)
	if err != nil || itemID != "pet_monkey" {
		t.Fatalf("store pet: itemID=%q err=%v", itemID, err)
	}
	if len(pets.Placements) != 0 {
		t.Fatalf("pet still placed: %#v", pets.Placements)
	}
	if _, err := petRemoveItem(&pets, 0); err == nil {
		t.Fatal("expected empty layer error")
	}
}

func TestPetPlaceRejectsLockedLayerAndNonPet(t *testing.T) {
	pets := defaultPlayerPets()
	if err := petPlaceItem(&pets, 1, "pet_monkey", 1); err == nil {
		t.Fatal("expected locked layer error")
	}
	if err := petPlaceItem(&pets, 0, "decor_chuong_gio", 1); err == nil {
		t.Fatal("expected non-pet item error")
	}
	if len(pets.Placements) != 0 {
		t.Fatalf("unexpected placements: %#v", pets.Placements)
	}
}

func TestPetPlaceAcceptsAnyPetItem(t *testing.T) {
	pets := defaultPlayerPets()
	if err := petPlaceItem(&pets, 0, "pet_dragon", 1); err != nil {
		t.Fatalf("place arbitrary pet: %v", err)
	}
	if len(pets.Placements) != 1 || pets.Placements[0].ItemID != "pet_dragon" {
		t.Fatalf("unexpected placements: %#v", pets.Placements)
	}
}

func TestNormalizePetPlacements(t *testing.T) {
	placements := normalizePetPlacements([]PetPlacement{
		{LayerIndex: 2, ItemID: "pet_monkey"},
		{LayerIndex: -1, ItemID: "pet_monkey"},
		{LayerIndex: 0, ItemID: " pet_monkey "},
		{LayerIndex: 1, ItemID: ""},
	})
	if len(placements) != 2 || placements[0].LayerIndex != 0 || placements[0].ItemID != "pet_monkey" || placements[1].LayerIndex != 2 {
		t.Fatalf("unexpected placements: %#v", placements)
	}
}
