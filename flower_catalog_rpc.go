package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

type flowerCatalogResponse struct {
	Flowers []FlowerDefinition `json:"flowers"`
}

// GetFlowerCatalogRPC returns backend-owned gameplay definitions for flowers.
func GetFlowerCatalogRPC(
	ctx context.Context,
	_ runtime.Logger,
	_ *sql.DB,
	_ runtime.NakamaModule,
	_ string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	raw, err := json.Marshal(flowerCatalogResponse{Flowers: listFlowerDefinitions()})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
