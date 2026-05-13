package main

import (
	"context"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerStateCollection = "player_state"
	playerStateKey        = "resources"
)

type PlayerResources struct {
	Coin   int `json:"coin"`
	Gem    int `json:"gem"`
	Energy int `json:"energy"`
}

func initPlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	defaultResources := PlayerResources{
		Coin:   1000,
		Gem:    50,
		Energy: 100,
	}

	raw, err := json.Marshal(defaultResources)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
