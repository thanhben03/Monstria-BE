package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

// GetPlayerInventoryRPC returns {"items":[...]}.
func GetPlayerInventoryRPC(
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

	inv, _, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}

	raw, err := json.Marshal(inv)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type setInventoryPayload struct {
	Pots  []InventoryItemStack `json:"pots"`
	Seeds []InventoryItemStack `json:"seeds"`
	Items []InventoryItemStack `json:"items"`
}

// SetPlayerInventoryRPC replaces inventory from payload. It accepts legacy buckets for dev tools but persists items[].
func SetPlayerInventoryRPC(
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

	var body setInventoryPayload
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return "", runtime.NewError("invalid JSON payload", 3)
		}
	}

	allItems := make([]InventoryItemStack, 0, len(body.Pots)+len(body.Seeds)+len(body.Items))
	allItems = append(allItems, body.Pots...)
	allItems = append(allItems, body.Seeds...)
	allItems = append(allItems, body.Items...)
	items, err := normalizeInventoryStacks(allItems)
	if err != nil {
		return "", err
	}

	inv := PlayerInventory{Items: items}

	_, version, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory before write: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}

	if err := writePlayerInventory(ctx, nk, userID, version, inv); err != nil {
		logger.Error("write inventory: %v", err)
		return "", runtime.NewError("failed to save inventory", 13)
	}

	raw, err := json.Marshal(inv)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
