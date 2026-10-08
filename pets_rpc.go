package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

type placePetPayload struct {
	LayerIndex *int   `json:"layerIndex"`
	ItemID     string `json:"itemId"`
}

type storePetPayload struct {
	LayerIndex *int `json:"layerIndex"`
}

type petMutationResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Pets      PlayerPets      `json:"pets"`
}

func petUserID(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}
	return userID, nil
}

func GetPlayerPetsRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, _ string) (string, error) {
	userID, err := petUserID(ctx)
	if err != nil {
		return "", err
	}
	pets, _, err := readPlayerPets(ctx, nk, userID)
	if err != nil {
		logger.Error("read pets: %v", err)
		return "", runtime.NewError("failed to load pets", 13)
	}
	raw, err := json.Marshal(pets)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func PlacePetOnLayerRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userID, err := petUserID(ctx)
	if err != nil {
		return "", err
	}
	var body placePetPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}
	if body.LayerIndex == nil {
		return "", runtime.NewError("layerIndex is required", 3)
	}
	if !isPetItemID(body.ItemID) {
		return "", runtime.NewError("invalid pet itemId", 3)
	}

	layers, _, err := readPlayerCloudLayers(ctx, nk, userID)
	if err != nil {
		logger.Error("read cloud layers before placing pet: %v", err)
		return "", runtime.NewError("failed to load cloud layers", 13)
	}
	inv, invVersion, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory before placing pet: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}
	pets, petsVersion, err := readPlayerPets(ctx, nk, userID)
	if err != nil {
		logger.Error("read pets before placing pet: %v", err)
		return "", runtime.NewError("failed to load pets", 13)
	}

	if err := petPlaceItem(&pets, *body.LayerIndex, body.ItemID, layers.UnlockedCount); err != nil {
		return "", err
	}
	if err := ConsumeOneItem(&inv, body.ItemID); err != nil {
		return "", err
	}
	return writePetMutation(ctx, logger, nk, userID, invVersion, petsVersion, inv, pets)
}

func StorePetInInventoryRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	userID, err := petUserID(ctx)
	if err != nil {
		return "", err
	}
	var body storePetPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}
	if body.LayerIndex == nil {
		return "", runtime.NewError("layerIndex is required", 3)
	}
	inv, invVersion, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory before storing pet: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}
	pets, petsVersion, err := readPlayerPets(ctx, nk, userID)
	if err != nil {
		logger.Error("read pets before storing pet: %v", err)
		return "", runtime.NewError("failed to load pets", 13)
	}
	itemID, err := petRemoveItem(&pets, *body.LayerIndex)
	if err != nil {
		return "", err
	}
	if err := AddInventoryItem(&inv, itemID, 1); err != nil {
		return "", err
	}
	return writePetMutation(ctx, logger, nk, userID, invVersion, petsVersion, inv, pets)
}

func writePetMutation(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, userID, invVersion, petsVersion string, inv PlayerInventory, pets PlayerPets) (string, error) {
	inv = normalizePlayerInventory(inv)
	pets.Placements = normalizePetPlacements(pets.Placements)
	invRaw, err := json.Marshal(inv)
	if err != nil {
		return "", err
	}
	petsRaw, err := json.Marshal(pets)
	if err != nil {
		return "", err
	}
	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID,
			Value: string(invRaw), Version: invVersion, PermissionRead: 1, PermissionWrite: 0,
		},
		{
			Collection: playerStateCollection, Key: playerPetsKey, UserID: userID,
			Value: string(petsRaw), Version: petsVersion, PermissionRead: 1, PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("write pet mutation: %v", err)
		return "", runtime.NewError("failed to save pets (retry)", 13)
	}
	raw, err := json.Marshal(petMutationResponse{Inventory: inv, Pets: pets})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
