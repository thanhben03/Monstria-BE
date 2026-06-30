package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const debugRPCsEnvKey = "ENABLE_DEBUG_RPCS"

type debugLevelUpResponse struct {
	Inventory PlayerInventory `json:"inventory"`
	Resources PlayerResources `json:"resources"`
	LevelUp   LevelUpResult   `json:"levelUp"`
}

func DebugLevelUpRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	_ string,
) (string, error) {
	if !debugRPCsEnabled() {
		return "", runtime.NewError("debug RPCs are disabled", 7)
	}

	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	inv, invVer, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory debug level up: %v", err)
		return "", runtime.NewError("failed to load inventory", 13)
	}

	resources, resourcesVer, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources debug level up: %v", err)
		return "", runtime.NewError("failed to load resources", 13)
	}

	resources = normalizePlayerResources(resources)
	expNeeded := expToNextLevel(resources.Level)
	if expNeeded <= 0 {
		return "", runtime.NewError("player is already at max level", 9)
	}

	expToAdd := expNeeded - resources.Exp
	if expToAdd < 1 {
		expToAdd = 1
	}

	levelUp, err := AddPlayerExp(&resources, expToAdd)
	if err != nil {
		return "", err
	}
	if !levelUp.LeveledUp {
		return "", runtime.NewError("failed to level up", 13)
	}

	if err := GrantLevelUpRewards(&inv, levelUp); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(normalizePlayerInventory(inv))
	if err != nil {
		return "", err
	}
	resourcesRaw, err := json.Marshal(playerResourcesForStorage(resources))
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
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(resourcesRaw),
			Version:         resourcesVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		logger.Error("storage write debug level up: %v", err)
		return "", runtime.NewError("failed to save debug level up", 13)
	}

	raw, err := json.Marshal(debugLevelUpResponse{
		Inventory: normalizePlayerInventory(inv),
		Resources: decoratePlayerResources(resources),
		LevelUp:   levelUp,
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func debugRPCsEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(debugRPCsEnvKey)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
