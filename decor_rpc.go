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
	Decor PlayerDecor `json:"decor"`
}

// PlaceDecorOnSlotRPC records one decor placement. Inventory is not consumed.
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

	decor, version, err := readPlayerDecor(ctx, nk, userID)
	if err != nil {
		logger.Error("read decor before place: %v", err)
		return "", runtime.NewError("failed to load decor", 13)
	}

	if err := decorPlaceItem(&decor, body.SlotID, body.ItemID); err != nil {
		return "", err
	}

	if err := writePlayerDecor(ctx, nk, userID, version, decor); err != nil {
		logger.Error("write decor: %v", err)
		return "", runtime.NewError("failed to save decor", 13)
	}

	raw, err := json.Marshal(placeDecorResponse{Decor: decor})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
