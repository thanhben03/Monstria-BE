package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

func GetPlayerResourcesRPC(
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

	resources, _, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources: %v", err)
		return "", runtime.NewError("failed to load resources", 13)
	}
	resources = decoratePlayerResources(resources)

	raw, err := json.Marshal(resources)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
