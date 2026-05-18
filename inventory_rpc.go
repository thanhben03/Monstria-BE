package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

// GetPlayerInventoryRPC returns {"pots":[...],"seeds":[...],"items":[...]}.
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
	Pots  []PotStack `json:"pots"`
	Seeds []PotStack `json:"seeds"`
	Items []PotStack `json:"items"`
}

// SetPlayerInventoryRPC replaces pots from payload (same shape as example). Merges duplicate itemId server-side.
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

	pots, err := normalizePots(body.Pots)
	if err != nil {
		return "", err
	}
	seeds, err := normalizePots(body.Seeds)
	if err != nil {
		return "", err
	}
	items, err := normalizePots(body.Items)
	if err != nil {
		return "", err
	}

	inv := PlayerInventory{Pots: pots, Seeds: seeds, Items: items}

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
