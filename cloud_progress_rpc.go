package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

// GetCloudProgressRPC returns {"unlockedLayerCount":N} with N in [1, MaxCloudLayers].
func GetCloudProgressRPC(
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

	p, _, err := readPlayerCloudProgress(ctx, nk, userID)
	if err != nil {
		logger.Error("read cloud progress: %v", err)
		return "", runtime.NewError("failed to load cloud progress", 13)
	}

	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

type setCloudProgressPayload struct {
	UnlockedLayerCount int `json:"unlockedLayerCount"`
}

// SetCloudProgressRPC sets total unlocked cloud layers (1 = only first scene layer; 2 = first+second; 3+ adds prefab layers).
func SetCloudProgressRPC(
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

	var body setCloudProgressPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	n := clampCloudLayerCount(body.UnlockedLayerCount)
	p := PlayerCloudProgress{UnlockedLayerCount: n}

	_, version, err := readPlayerCloudProgress(ctx, nk, userID)
	if err != nil {
		logger.Error("read cloud progress before write: %v", err)
		return "", runtime.NewError("failed to load cloud progress", 13)
	}

	if err := writePlayerCloudProgress(ctx, nk, userID, version, p); err != nil {
		logger.Error("write cloud progress: %v", err)
		return "", runtime.NewError("failed to save cloud progress", 13)
	}

	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
