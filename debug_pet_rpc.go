package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

type debugGrantPetPayload struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

// DebugGrantPetRPC grants a pet only when the existing debug RPC flag is enabled.
func DebugGrantPetRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	if !debugRPCsEnabled() {
		return "", runtime.NewError("debug RPCs are disabled", 7)
	}
	userID, err := petUserID(ctx)
	if err != nil {
		return "", err
	}
	var body debugGrantPetPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}
	if !isPetItemID(body.ItemID) {
		return "", runtime.NewError("invalid pet itemId", 3)
	}
	if body.Quantity < 1 || body.Quantity > 10 {
		return "", runtime.NewError("quantity must be between 1 and 10", 3)
	}
	inv, version, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory before debug grant pet: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}
	if err := AddInventoryItem(&inv, body.ItemID, body.Quantity); err != nil {
		return "", err
	}
	if err := writePlayerInventory(ctx, nk, userID, version, inv); err != nil {
		logger.Error("write inventory after debug grant pet: %v", err)
		return "", runtime.NewError("failed to grant pet (retry)", 13)
	}
	raw, err := json.Marshal(normalizePlayerInventory(inv))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
