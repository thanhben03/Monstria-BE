package main

import (
	"context"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerCloudProgressKey = "cloud_progress"
	// MaxCloudLayers is the upper bound for unlockedLayerCount (1 = first scene layer only).
	MaxCloudLayers = 10
)

// PlayerCloudProgress is stored under player_state / cloud_progress.
type PlayerCloudProgress struct {
	UnlockedLayerCount int `json:"unlockedLayerCount"`
}

func defaultPlayerCloudProgress() PlayerCloudProgress {
	return PlayerCloudProgress{UnlockedLayerCount: 1}
}

func initPlayerCloudProgress(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	p := defaultPlayerCloudProgress()
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerCloudProgressKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func readPlayerCloudProgress(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerCloudProgress, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerCloudProgressKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerCloudProgress{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerCloudProgress(), "", nil
	}

	var p PlayerCloudProgress
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &p); err != nil {
		return PlayerCloudProgress{}, "", err
	}
	if p.UnlockedLayerCount < 1 {
		p.UnlockedLayerCount = 1
	}
	if p.UnlockedLayerCount > MaxCloudLayers {
		p.UnlockedLayerCount = MaxCloudLayers
	}
	return p, objs[0].GetVersion(), nil
}

func writePlayerCloudProgress(ctx context.Context, nk runtime.NakamaModule, userID string, version string, p PlayerCloudProgress) error {
	if p.UnlockedLayerCount < 1 {
		p.UnlockedLayerCount = 1
	}
	if p.UnlockedLayerCount > MaxCloudLayers {
		p.UnlockedLayerCount = MaxCloudLayers
	}

	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerCloudProgressKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func clampCloudLayerCount(n int) int {
	if n < 1 {
		return 1
	}
	if n > MaxCloudLayers {
		return MaxCloudLayers
	}
	return n
}
