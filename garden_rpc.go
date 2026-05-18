package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// GetPlayerGardenRPC returns placements with potItemId and optional plant { seedItemId, plantedAt }.
func GetPlayerGardenRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	_ string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	g, _, err := readPlayerGarden(ctx, nk, userID)
	if err != nil {
		logger.Error("read garden: %v", err)
		return "", runtime.NewError("failed to load garden", 13)
	}

	raw, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type placePotPayload struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

// placePotResponse matches client JsonUtility nested fields.
type placePotResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

// PlacePotOnSlotRPC consumes one pot from inventory and records placement. Atomic write of inventory + garden.
func PlacePotOnSlotRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	var body placePotPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &inv); err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			if inv.Pots == nil {
				inv.Pots = []PotStack{}
			}
			if inv.Seeds == nil {
				inv.Seeds = []PotStack{}
			}
			if inv.Items == nil {
				inv.Items = []PotStack{}
			}
			invVer = o.GetVersion()
		case playerGardenKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &garden); err != nil {
				return "", runtime.NewError("corrupt garden", 13)
			}
			if garden.Placements == nil {
				garden.Placements = []SlotPlacement{}
			}
			garden.Placements = normalizeGardenPlacements(garden.Placements)
			gardenVer = o.GetVersion()
		}
	}

	if !storageReadHasKey(objs, playerInventoryKey) {
		return "", runtime.NewError("inventory not initialized", 9)
	}

	invCopy := inv
	if err := ConsumeOnePot(&invCopy, body.ItemID); err != nil {
		return "", err
	}

	gardenCopy := garden
	if err := gardenPlacePot(&gardenCopy, body.SlotID, body.ItemID); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(invCopy)
	if err != nil {
		return "", err
	}
	gardenRaw, err := json.Marshal(gardenCopy)
	if err != nil {
		return "", err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(gardenRaw),
			Version:         gardenVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write place pot: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := placePotResponse{Inventory: invCopy, Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type plantSeedPayload struct {
	SlotID     string `json:"slotId"`
	SeedItemID string `json:"seedItemId"`
}

type plantSeedResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

// PlantSeedInPotRPC consumes one seed from inventory and records plant on an existing pot (no pot consumed).
func PlantSeedInPotRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	var body plantSeedPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read plant seed: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &inv); err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			if inv.Pots == nil {
				inv.Pots = []PotStack{}
			}
			if inv.Seeds == nil {
				inv.Seeds = []PotStack{}
			}
			if inv.Items == nil {
				inv.Items = []PotStack{}
			}
			invVer = o.GetVersion()
		case playerGardenKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &garden); err != nil {
				return "", runtime.NewError("corrupt garden", 13)
			}
			if garden.Placements == nil {
				garden.Placements = []SlotPlacement{}
			}
			garden.Placements = normalizeGardenPlacements(garden.Placements)
			gardenVer = o.GetVersion()
		}
	}

	if !storageReadHasKey(objs, playerInventoryKey) {
		return "", runtime.NewError("inventory not initialized", 9)
	}

	invCopy := inv
	if err := ConsumeOneSeed(&invCopy, body.SeedItemID); err != nil {
		return "", err
	}

	gardenCopy := garden
	if err := gardenPlantSeed(&gardenCopy, body.SlotID, body.SeedItemID, nowUnixSeconds()); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(invCopy)
	if err != nil {
		return "", err
	}
	gardenRaw, err := json.Marshal(gardenCopy)
	if err != nil {
		return "", err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(gardenRaw),
			Version:         gardenVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write plant seed: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := plantSeedResponse{Inventory: invCopy, Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type waterPlantPayload struct {
	SlotID string `json:"slotId"`
}

type waterPlantResponse struct {
	Garden PlayerGarden `json:"garden"`
}

// WaterPlantInPotRPC starts plant growth. A planted seed does not grow until watered.
func WaterPlantInPotRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	var body waterPlantPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	garden, gardenVer, err := readPlayerGarden(ctx, nk, userID)
	if err != nil {
		logger.Error("storage read water plant: %v", err)
		return "", runtime.NewError("failed to load garden", 13)
	}

	gardenCopy := garden
	if err := gardenWaterPlant(&gardenCopy, body.SlotID, nowUnixSeconds()); err != nil {
		return "", err
	}

	gardenRaw, err := json.Marshal(gardenCopy)
	if err != nil {
		return "", err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(gardenRaw),
			Version:         gardenVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write water plant: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := waterPlantResponse{Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type harvestPlantPayload struct {
	SlotID string `json:"slotId"`
}

type harvestReward struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type harvestPlantResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
	Reward    harvestReward   `json:"reward"`
}

// HarvestPlantInPotRPC validates server-side growth time, clears the plant, and grants reward items.
func HarvestPlantInPotRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	var body harvestPlantPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read harvest plant: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &inv); err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			if inv.Pots == nil {
				inv.Pots = []PotStack{}
			}
			if inv.Seeds == nil {
				inv.Seeds = []PotStack{}
			}
			if inv.Items == nil {
				inv.Items = []PotStack{}
			}
			invVer = o.GetVersion()
		case playerGardenKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &garden); err != nil {
				return "", runtime.NewError("corrupt garden", 13)
			}
			if garden.Placements == nil {
				garden.Placements = []SlotPlacement{}
			}
			garden.Placements = normalizeGardenPlacements(garden.Placements)
			gardenVer = o.GetVersion()
		}
	}

	if !storageReadHasKey(objs, playerInventoryKey) {
		return "", runtime.NewError("inventory not initialized", 9)
	}

	gardenCopy := garden
	reward, err := gardenHarvestPlant(&gardenCopy, body.SlotID, nowUnixSeconds())
	if err != nil {
		return "", err
	}

	invCopy := inv
	if err := AddItem(&invCopy, reward.ItemID, reward.Quantity); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(invCopy)
	if err != nil {
		return "", err
	}
	gardenRaw, err := json.Marshal(gardenCopy)
	if err != nil {
		return "", err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(gardenRaw),
			Version:         gardenVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write harvest plant: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := harvestPlantResponse{Inventory: invCopy, Garden: gardenCopy, Reward: reward}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func storageReadHasKey(objs []*api.StorageObject, key string) bool {
	for _, o := range objs {
		if o.GetKey() == key {
			return true
		}
	}
	return false
}
