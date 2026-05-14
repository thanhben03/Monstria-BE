package main

import (
	"context"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerCloudLayersKey = "cloud_layers"
)

type PlayerCloudLayers struct {
	UnlockedCount int `json:"unlockedCount"`
}

func defaultPlayerCloudLayers() PlayerCloudLayers {
	return PlayerCloudLayers{UnlockedCount: 1}
}

func initPlayerCloudLayers(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	cl := defaultPlayerCloudLayers()
	raw, err := json.Marshal(cl)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerCloudLayersKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func readPlayerCloudLayers(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerCloudLayers, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerCloudLayersKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerCloudLayers{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerCloudLayers(), "", nil
	}

	var cl PlayerCloudLayers
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &cl); err != nil {
		return PlayerCloudLayers{}, "", err
	}

	// Đảm bảo unlockedCount ít nhất là 1
	if cl.UnlockedCount < 1 {
		cl.UnlockedCount = 1
	}

	return cl, objs[0].GetVersion(), nil
}

func writePlayerCloudLayers(ctx context.Context, nk runtime.NakamaModule, userID string, version string, cl PlayerCloudLayers) error {
	// Đảm bảo unlockedCount ít nhất là 1
	if cl.UnlockedCount < 1 {
		cl.UnlockedCount = 1
	}

	raw, err := json.Marshal(cl)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerCloudLayersKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}
