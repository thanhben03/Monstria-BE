package main

import (
	"context"
	"encoding/json"
	"github.com/heroiclabs/nakama-common/runtime"
	"testing"
)

func TestPetCatalogValidation(t *testing.T) {
	valid, err := parsePetCatalog(defaultPetCatalog)
	if err != nil || len(valid.Pets) != 2 {
		t.Fatalf("default catalog: %v", err)
	}
	for _, raw := range []string{
		`{}`, `{"pets":[]}`, `{"pets":[{"id":"pet_fox","name":"Fox","description":"Fox","rarity":"UR"}]}`,
		`{"pets":[{"id":"pot_fox","name":"Fox","description":"Fox","rarity":"R"}]}`,
		`{"pets":[{"id":"pet_fox","name":"","description":"Fox","rarity":"R"}]}`,
		`{"pets":[{"id":"pet_fox","name":"Fox","description":"Fox","rarity":"R"},{"id":"pet_fox","name":"Fox","description":"Fox","rarity":"SR"}]}`,
	} {
		if _, err := parsePetCatalog([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid catalog: %s", raw)
		}
	}
}

func TestPetCatalogRPCRequiresSession(t *testing.T) {
	if _, err := GetPetCatalogRPC(context.Background(), nil, nil, nil, ""); err == nil {
		t.Fatal("expected unauthorized")
	}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	raw, err := GetPetCatalogRPC(ctx, nil, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	var got PetCatalog
	if err := json.Unmarshal([]byte(raw), &got); err != nil || len(got.Pets) != 2 {
		t.Fatalf("catalog response: %s, %v", raw, err)
	}
}

func TestPetCopiesOnDifferentLayers(t *testing.T) {
	pets := defaultPlayerPets()
	for _, layer := range []int{0, 2} {
		if err := petPlaceItem(&pets, layer, "pet_fox", 3); err != nil {
			t.Fatal(err)
		}
	}
	if err := petPlaceItem(&pets, 2, "pet_monkey", 3); err == nil {
		t.Fatal("replaced occupied layer")
	}
	if _, err := petRemoveItem(&pets, 0); err != nil {
		t.Fatal(err)
	}
	if len(pets.Placements) != 1 || pets.Placements[0].LayerIndex != 2 || pets.Placements[0].ItemID != "pet_fox" {
		t.Fatalf("wrong remaining placement: %+v", pets)
	}
}
