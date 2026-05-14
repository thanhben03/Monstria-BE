package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

// GetPlayerGardenRPC returns {"placements":[{"slotId":"0_0","itemId":"pot_wood"}, ...]} — only occupied slots.
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
	if err := gardenPlaceOccupied(&gardenCopy, body.SlotID, body.ItemID); err != nil {
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

func storageReadHasKey(objs []*api.StorageObject, key string) bool {
	for _, o := range objs {
		if o.GetKey() == key {
			return true
		}
	}
	return false
}
