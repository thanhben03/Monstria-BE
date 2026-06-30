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

	g, gardenVer, err := readPlayerGarden(ctx, nk, userID)
	if err != nil {
		logger.Error("read garden: %v", err)
		return "", runtime.NewError("failed to load garden", 13)
	}
	if updateGardenDiseaseState(&g, nowUnixSeconds()) {
		if err := writePlayerGarden(ctx, nk, userID, gardenVer, g); err != nil {
			logger.Error("write garden after disease update: %v", err)
			return "", runtime.NewError("failed to save garden", 13)
		}
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
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	updateGardenDiseaseState(&garden, nowUnixSeconds())

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

	out := placePotResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type storePotPayload struct {
	SlotID string `json:"slotId"`
}

type storePotResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

// StorePotInInventoryRPC removes an empty pot from a slot and returns it to inventory.
func StorePotInInventoryRPC(
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

	var body storePotPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read store pot: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	updateGardenDiseaseState(&garden, nowUnixSeconds())

	gardenCopy := garden
	potItemID, err := gardenRemovePot(&gardenCopy, body.SlotID)
	if err != nil {
		return "", err
	}

	invCopy := inv
	if err := AddPot(&invCopy, potItemID, 1); err != nil {
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
		logger.Error("storage write store pot: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := storePotResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type storeLayerPotsPayload struct {
	SlotID string `json:"slotId"`
}

type storeLayerPotsResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

// StoreLayerPotsInInventoryRPC removes every empty pot in the clicked pot's cloud layer.
func StoreLayerPotsInInventoryRPC(
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

	var body storeLayerPotsPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read store layer pots: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	updateGardenDiseaseState(&garden, nowUnixSeconds())

	gardenCopy := garden
	potItemIDs, err := gardenRemovePotsInLayer(&gardenCopy, body.SlotID)
	if err != nil {
		return "", err
	}

	invCopy := inv
	for _, potItemID := range potItemIDs {
		if err := AddPot(&invCopy, potItemID, 1); err != nil {
			return "", err
		}
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
		logger.Error("storage write store layer pots: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := storeLayerPotsResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
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
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	now := nowUnixSeconds()
	updateGardenDiseaseState(&garden, now)

	invCopy := inv
	if err := ConsumeOneSeed(&invCopy, body.SeedItemID); err != nil {
		return "", err
	}

	gardenCopy := garden
	if err := gardenPlantSeed(&gardenCopy, body.SlotID, body.SeedItemID, now); err != nil {
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

	out := plantSeedResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
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
	now := nowUnixSeconds()
	updateGardenDiseaseState(&gardenCopy, now)
	if err := gardenWaterPlant(&gardenCopy, body.SlotID, now); err != nil {
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
	ItemID    string `json:"itemId"`
	Quantity  int    `json:"quantity"`
	ExpReward int    `json:"expReward,omitempty"`
}

type harvestPlantResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
	Reward    harvestReward   `json:"reward"`
	Resources PlayerResources `json:"resources"`
	LevelUp   *LevelUpResult  `json:"levelUp,omitempty"`
}

type destroyDeadPlantPayload struct {
	SlotID string `json:"slotId"`
}

type destroyDeadPlantResponse struct {
	Garden PlayerGarden `json:"garden"`
}

type treatPlantDiseasePayload struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

type treatPlantDiseaseResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

// TreatPlantDiseaseRPC consumes a treatment item, clears plant disease, and adds protection time.
func TreatPlantDiseaseRPC(
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

	var body treatPlantDiseasePayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read treat plant disease: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	now := nowUnixSeconds()
	gardenCopy := garden
	updateGardenDiseaseState(&gardenCopy, now)
	if err := gardenTreatPlantDisease(&gardenCopy, body.SlotID, body.ItemID, now); err != nil {
		return "", err
	}

	invCopy := inv
	if err := ConsumeOneItem(&invCopy, body.ItemID); err != nil {
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
		logger.Error("storage write treat plant disease: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := treatPlantDiseaseResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
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
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
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

	var levelUp *LevelUpResult
	resourcesCopy, resourcesVer, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources before harvest exp: %v", err)
		return "", runtime.NewError("failed to load resources", 13)
	}
	if reward.ExpReward > 0 {
		levelUpResult, err := AddPlayerExp(&resourcesCopy, reward.ExpReward)
		if err != nil {
			return "", err
		}
		if levelUpResult.LeveledUp {
			if err := GrantLevelUpRewards(&invCopy, levelUpResult); err != nil {
				return "", err
			}
			levelUp = &levelUpResult
		}
	}

	invRaw, err := json.Marshal(invCopy)
	if err != nil {
		return "", err
	}
	gardenRaw, err := json.Marshal(gardenCopy)
	if err != nil {
		return "", err
	}
	resourcesRaw, err := json.Marshal(playerResourcesForStorage(resourcesCopy))
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
		{
			Collection:      playerStateCollection,
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(resourcesRaw),
			Version:         resourcesVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write harvest plant: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := harvestPlantResponse{
		Inventory: normalizePlayerInventory(invCopy),
		Garden:    gardenCopy,
		Reward:    reward,
		Resources: decoratePlayerResources(resourcesCopy),
		LevelUp:   levelUp,
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// DestroyDeadPlantRPC clears a dead plant from a pot without granting rewards.
func DestroyDeadPlantRPC(
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

	var body destroyDeadPlantPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	garden, gardenVer, err := readPlayerGarden(ctx, nk, userID)
	if err != nil {
		logger.Error("storage read destroy dead plant: %v", err)
		return "", runtime.NewError("failed to load garden", 13)
	}

	gardenCopy := garden
	if err := gardenDestroyDeadPlant(&gardenCopy, body.SlotID, nowUnixSeconds()); err != nil {
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
		logger.Error("storage write destroy dead plant: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := destroyDeadPlantResponse{Garden: gardenCopy}
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
