package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

// GetPlayerDecorRPC returns placed decor slots for the current user.
func GetPlayerDecorRPC(
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

	decor, _, err := readPlayerDecor(ctx, nk, userID)
	if err != nil {
		logger.Error("read decor: %v", err)
		return "", runtime.NewError("failed to load decor", 13)
	}

	raw, err := json.Marshal(decor)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type placeDecorPayload struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

type placeDecorResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Decor     PlayerDecor     `json:"decor"`
}

type storeDecorPayload struct {
	SlotID string `json:"slotId"`
}

type storeDecorResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Decor     PlayerDecor     `json:"decor"`
}

// PlaceDecorOnSlotRPC consumes one decor item and records one decor placement.
func PlaceDecorOnSlotRPC(
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

	var body placeDecorPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerDecorKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read place decor: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	decor := defaultPlayerDecor()
	decorVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
			invVer = o.GetVersion()
		case playerDecorKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &decor); err != nil {
				return "", runtime.NewError("corrupt decor", 13)
			}
			decor.Placements = normalizeDecorPlacements(decor.Placements)
			decorVer = o.GetVersion()
		}
	}

	if !storageReadHasKey(objs, playerInventoryKey) {
		return "", runtime.NewError("inventory not initialized", 9)
	}

	invCopy := inv
	if err := ConsumeOneItem(&invCopy, body.ItemID); err != nil {
		return "", err
	}

	decorCopy := decor
	if err := decorPlaceItem(&decorCopy, body.SlotID, body.ItemID); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(normalizePlayerInventory(invCopy))
	if err != nil {
		return "", err
	}
	decorRaw, err := json.Marshal(decorCopy)
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
			Key:             playerDecorKey,
			UserID:          userID,
			Value:           string(decorRaw),
			Version:         decorVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write place decor: %v", err)
		return "", runtime.NewError("failed to save decor", 13)
	}

	raw, err := json.Marshal(placeDecorResponse{
		Inventory: normalizePlayerInventory(invCopy),
		Decor:     decorCopy,
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// StoreDecorInInventoryRPC removes one placed decor and returns it to inventory.
func StoreDecorInInventoryRPC(
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

	var body storeDecorPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
		{Collection: playerStateCollection, Key: playerDecorKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read store decor: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""
	decor := defaultPlayerDecor()
	decorVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
			invVer = o.GetVersion()
		case playerDecorKey:
			if err := json.Unmarshal([]byte(o.GetValue()), &decor); err != nil {
				return "", runtime.NewError("corrupt decor", 13)
			}
			decor.Placements = normalizeDecorPlacements(decor.Placements)
			decorVer = o.GetVersion()
		}
	}

	if !storageReadHasKey(objs, playerInventoryKey) {
		return "", runtime.NewError("inventory not initialized", 9)
	}

	decorCopy := decor
	itemID, err := decorRemoveItem(&decorCopy, body.SlotID)
	if err != nil {
		return "", err
	}

	invCopy := inv
	if err := AddInventoryItem(&invCopy, itemID, 1); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(normalizePlayerInventory(invCopy))
	if err != nil {
		return "", err
	}
	decorRaw, err := json.Marshal(decorCopy)
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
			Key:             playerDecorKey,
			UserID:          userID,
			Value:           string(decorRaw),
			Version:         decorVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write store decor: %v", err)
		return "", runtime.NewError("failed to save decor", 13)
	}

	raw, err := json.Marshal(storeDecorResponse{
		Inventory: normalizePlayerInventory(invCopy),
		Decor:     decorCopy,
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
