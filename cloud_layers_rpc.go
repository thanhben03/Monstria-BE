package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

type UpdateCloudLayersPayload struct {
	UnlockedCloudLayers int `json:"unlockedCloudLayers"`
}

type UpdateCloudLayersResponse struct {
	UnlockedCount int `json:"unlockedCount"`
	Status        string `json:"status"`
}

// GetPlayerCloudLayersRPC returns current cloud layers progress
func GetPlayerCloudLayersRPC(
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

	cl, _, err := readPlayerCloudLayers(ctx, nk, userID)
	if err != nil {
		logger.Error("read cloud layers: %v", err)
		return "", runtime.NewError("failed to load cloud layers", 13)
	}

	raw, err := json.Marshal(cl)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// UpdateCloudLayersRPC updates the number of unlocked cloud layers
func UpdateCloudLayersRPC(
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

	var req UpdateCloudLayersPayload
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			logger.Error("invalid JSON payload: %v", err)
			return "", runtime.NewError("invalid JSON payload", 3)
		}
	}

	// Validate input
	if req.UnlockedCloudLayers < 1 {
		req.UnlockedCloudLayers = 1
	}

	// Read current cloud layers state
	cl, version, err := readPlayerCloudLayers(ctx, nk, userID)
	if err != nil {
		logger.Error("read cloud layers before update: %v", err)
		return "", runtime.NewError("failed to load cloud layers", 13)
	}

	// Only allow increasing unlocked layers, not decreasing
	if req.UnlockedCloudLayers > cl.UnlockedCount {
		cl.UnlockedCount = req.UnlockedCloudLayers
	}

	// Write updated state
	if err := writePlayerCloudLayers(ctx, nk, userID, version, cl); err != nil {
		logger.Error("write cloud layers: %v", err)
		return "", runtime.NewError("failed to save cloud layers", 13)
	}

	logger.Info("Updated cloud layers for user %s: %d layers unlocked", userID, cl.UnlockedCount)

	resp := UpdateCloudLayersResponse{
		UnlockedCount: cl.UnlockedCount,
		Status:        "success",
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
