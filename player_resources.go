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
	Coin                int `json:"coin"`
	Gem                 int `json:"gem"`
	Energy              int `json:"energy"`
	Level               int `json:"level"`
	UnlockedCloudLayers int `json:"unlockedCloudLayers"`
}

func defaultPlayerResources() PlayerResources {
	return PlayerResources{
		Coin:                1000,
		Gem:                 50,
		Energy:              100,
		Level:               1,
		UnlockedCloudLayers: 1,
	}
}

func initPlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	defaultResources := defaultPlayerResources()

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

func normalizePlayerResources(resources PlayerResources) PlayerResources {
	if resources.Level < 1 {
		resources.Level = 1
	}
	if resources.UnlockedCloudLayers < 1 {
		resources.UnlockedCloudLayers = 1
	}
	return resources
}

func readPlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerResources, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerStateKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerResources{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerResources(), "", nil
	}

	var resources PlayerResources
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &resources); err != nil {
		return PlayerResources{}, "", err
	}
	return normalizePlayerResources(resources), objs[0].GetVersion(), nil
}

func writePlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string, version string, resources PlayerResources) error {
	resources = normalizePlayerResources(resources)
	raw, err := json.Marshal(resources)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func SpendPlayerCurrency(resources *PlayerResources, currency string, price int) error {
	if price < 1 {
		return runtime.NewError("price must be positive", 13)
	}

	switch currency {
	case shopCurrencyCoin:
		if resources.Coin < price {
			return runtime.NewError("not enough coin", 9)
		}
		resources.Coin -= price
	case shopCurrencyGem:
		if resources.Gem < price {
			return runtime.NewError("not enough gem", 9)
		}
		resources.Gem -= price
	default:
		return runtime.NewError("currency is invalid", 3)
	}
	return nil
}
