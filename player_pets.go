package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const playerPetsKey = "pets"

type PetPlacement struct {
	LayerIndex int    `json:"layerIndex"`
	ItemID     string `json:"itemId"`
}

type PlayerPets struct {
	Placements []PetPlacement `json:"placements"`
}

func defaultPlayerPets() PlayerPets {
	return PlayerPets{Placements: []PetPlacement{}}
}

func isPetItemID(itemID string) bool {
	return strings.HasPrefix(strings.TrimSpace(itemID), "pet_")
}

func normalizePetPlacements(placements []PetPlacement) []PetPlacement {
	byLayer := make(map[int]PetPlacement, len(placements))
	for _, placement := range placements {
		itemID := strings.TrimSpace(placement.ItemID)
		if placement.LayerIndex < 0 || itemID == "" {
			continue
		}
		byLayer[placement.LayerIndex] = PetPlacement{LayerIndex: placement.LayerIndex, ItemID: itemID}
	}

	layers := make([]int, 0, len(byLayer))
	for layer := range byLayer {
		layers = append(layers, layer)
	}
	sort.Ints(layers)

	out := make([]PetPlacement, 0, len(layers))
	for _, layer := range layers {
		out = append(out, byLayer[layer])
	}
	return out
}

func petPlaceItem(pets *PlayerPets, layerIndex int, itemID string, unlockedCount int) error {
	itemID = strings.TrimSpace(itemID)
	if layerIndex < 0 || layerIndex >= unlockedCount {
		return runtime.NewError("cloud layer is locked", 3)
	}
	if !isPetItemID(itemID) {
		return runtime.NewError("invalid pet itemId", 3)
	}
	for _, placement := range pets.Placements {
		if placement.LayerIndex == layerIndex {
			return runtime.NewError("pet already placed on layer", 3)
		}
	}
	pets.Placements = normalizePetPlacements(append(pets.Placements, PetPlacement{LayerIndex: layerIndex, ItemID: itemID}))
	return nil
}

func petRemoveItem(pets *PlayerPets, layerIndex int) (string, error) {
	if layerIndex < 0 {
		return "", runtime.NewError("layerIndex is invalid", 3)
	}
	for i, placement := range pets.Placements {
		if placement.LayerIndex != layerIndex {
			continue
		}
		pets.Placements = normalizePetPlacements(append(pets.Placements[:i], pets.Placements[i+1:]...))
		return placement.ItemID, nil
	}
	return "", runtime.NewError("pet layer is empty", 3)
}

func initPlayerPets(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	raw, err := json.Marshal(defaultPlayerPets())
	if err != nil {
		return err
	}
	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{{
		Collection: playerStateCollection, Key: playerPetsKey, UserID: userID,
		Value: string(raw), Version: "", PermissionRead: 1, PermissionWrite: 0,
	}})
	return err
}

func readPlayerPets(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerPets, string, error) {
	objects, err := nk.StorageRead(ctx, []*runtime.StorageRead{{
		Collection: playerStateCollection, Key: playerPetsKey, UserID: userID,
	}})
	if err != nil {
		return PlayerPets{}, "", err
	}
	if len(objects) == 0 {
		return defaultPlayerPets(), "", nil
	}
	var pets PlayerPets
	if err := json.Unmarshal([]byte(objects[0].GetValue()), &pets); err != nil {
		return PlayerPets{}, "", err
	}
	pets.Placements = normalizePetPlacements(pets.Placements)
	return pets, objects[0].GetVersion(), nil
}
