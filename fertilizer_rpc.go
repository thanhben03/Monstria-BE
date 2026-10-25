package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/heroiclabs/nakama-common/runtime"
	"strings"
)

// Item IDs are shared with Unity FertilizerData assets. Rates are server-owned.
var fertilizerRates = map[string]float64{
	"fertilizer_15": 0.15,
	"fertilizer_30": 0.30,
	"fertilizer_50": 0.50,
}

type fertilizePlantPayload struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}
type fertilizePlantResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Garden    PlayerGarden    `json:"garden"`
}

func gardenFertilizePlant(g *PlayerGarden, slotID, itemID string, now int64) error {
	rate, ok := fertilizerRates[strings.TrimSpace(itemID)]
	if !ok {
		return runtime.NewError("unknown fertilizer", 3)
	}
	idx := findPlacementIndex(g, slotID)
	if idx < 0 || g.Placements[idx].PotItemID == "" || g.Placements[idx].Plant == nil {
		return runtime.NewError("no plant in this slot", 3)
	}
	plant := g.Placements[idx].Plant
	if plant.Health < 1 {
		return runtime.NewError("plant is dead", 3)
	}
	if plant.GrowthStartedAt < 1 {
		return runtime.NewError("plant needs water", 3)
	}
	if plant.Fertilized {
		return runtime.NewError("plant already fertilized", 3)
	}
	def, err := flowerDefinitionForSeed(plant.SeedItemID)
	if err != nil {
		return err
	}
	remaining := float64(plant.GrowthStartedAt+def.GrowSeconds-now) - plant.GrowthReductionSeconds
	if remaining <= 0 {
		return runtime.NewError("plant is already mature", 3)
	}
	plant.GrowthReductionSeconds += remaining * rate
	plant.Fertilized = true
	return nil
}

// StorageWrite atomically commits both version-checked records.
func FertilizePlantInPotRPC(
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

	var body fertilizePlantPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerGardenKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read fertilize plant: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	garden := defaultPlayerGarden()
	gardenVer := "*"

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
	if err := gardenFertilizePlant(&gardenCopy, body.SlotID, body.ItemID, now); err != nil {
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
		logger.Error("storage write fertilize plant: %v", err)
		return "", runtime.NewError("failed to save (retry)", 13)
	}

	out := fertilizePlantResponse{Inventory: normalizePlayerInventory(invCopy), Garden: gardenCopy}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
