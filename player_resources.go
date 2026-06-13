package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerStateCollection = "player_state"
	playerStateKey        = "resources"
)

type PlayerResources struct {
	Coin                int     `json:"coin"`
	Gem                 int     `json:"gem"`
	Energy              int     `json:"energy"`
	Level               int     `json:"level"`
	Exp                 int     `json:"exp"`
	ExpToNextLevel      int     `json:"expToNextLevel,omitempty"`
	ExpProgress         float64 `json:"expProgress,omitempty"`
	UnlockedCloudLayers int     `json:"unlockedCloudLayers"`
	WalletMigrated      bool    `json:"walletMigrated,omitempty"`
}

func defaultPlayerResources() PlayerResources {
	return PlayerResources{
		Coin:                1000,
		Gem:                 50,
		Energy:              100,
		Level:               1,
		Exp:                 0,
		UnlockedCloudLayers: 1,
	}
}

func initPlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	defaultResources := defaultPlayerResources()
	walletChangeset := walletChangesetFromResources(defaultResources)
	defaultResources.Coin = 0
	defaultResources.Gem = 0
	defaultResources.WalletMigrated = true

	raw, err := json.Marshal(playerResourcesForStorage(defaultResources))
	if err != nil {
		return err
	}

	_, _, err = nk.MultiUpdate(ctx, nil, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	}, nil, []*runtime.WalletUpdate{
		{
			UserID:    userID,
			Changeset: walletChangeset,
			Metadata: map[string]interface{}{
				"source": "new_player_resources",
			},
		},
	}, true)
	if err != nil {
		return err
	}

	return nil
}

func normalizePlayerResources(resources PlayerResources) PlayerResources {
	if resources.Level < 1 {
		resources.Level = 1
	}
	if resources.Exp < 0 {
		resources.Exp = 0
	}
	if resources.UnlockedCloudLayers < 1 {
		resources.UnlockedCloudLayers = 1
	}
	return resources
}

func playerResourcesForStorage(resources PlayerResources) PlayerResources {
	resources = normalizePlayerResources(resources)
	resources.ExpToNextLevel = 0
	resources.ExpProgress = 0
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
		resources := defaultPlayerResources()
		wallet, err := readPlayerWallet(ctx, nk, userID)
		if err != nil {
			return PlayerResources{}, "", err
		}
		return applyWalletToResources(resources, wallet), "", nil
	}

	var resources PlayerResources
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &resources); err != nil {
		return PlayerResources{}, "", err
	}
	resources = normalizePlayerResources(resources)
	version := objs[0].GetVersion()
	if !resources.WalletMigrated {
		var err error
		resources, version, err = migratePlayerResourcesWallet(ctx, nk, userID, version, resources)
		if err != nil {
			return PlayerResources{}, "", err
		}
	}

	wallet, err := readPlayerWallet(ctx, nk, userID)
	if err != nil {
		return PlayerResources{}, "", err
	}
	return applyWalletToResources(resources, wallet), version, nil
}

func writePlayerResources(ctx context.Context, nk runtime.NakamaModule, userID string, version string, resources PlayerResources) error {
	resources = normalizePlayerResources(resources)
	resources.Coin = 0
	resources.Gem = 0
	resources.WalletMigrated = true
	raw, err := json.Marshal(playerResourcesForStorage(resources))
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

func BuildPlayerCurrencySpendChangeset(currency string, price int) (map[string]int64, error) {
	if price < 1 {
		return nil, runtime.NewError("price must be positive", 13)
	}

	switch currency {
	case shopCurrencyCoin:
		return map[string]int64{shopCurrencyCoin: -int64(price)}, nil
	case shopCurrencyGem:
		return map[string]int64{shopCurrencyGem: -int64(price)}, nil
	default:
		return nil, runtime.NewError("currency is invalid", 3)
	}
}

func walletChangesetFromResources(resources PlayerResources) map[string]int64 {
	changeset := map[string]int64{}
	if resources.Coin != 0 {
		changeset[shopCurrencyCoin] = int64(resources.Coin)
	}
	if resources.Gem != 0 {
		changeset[shopCurrencyGem] = int64(resources.Gem)
	}
	return changeset
}

func migratePlayerResourcesWallet(ctx context.Context, nk runtime.NakamaModule, userID string, version string, resources PlayerResources) (PlayerResources, string, error) {
	changeset := walletChangesetFromResources(resources)
	wallet, err := readPlayerWallet(ctx, nk, userID)
	if err != nil {
		return PlayerResources{}, "", err
	}
	if _, ok := wallet[shopCurrencyCoin]; ok {
		delete(changeset, shopCurrencyCoin)
	}
	if _, ok := wallet[shopCurrencyGem]; ok {
		delete(changeset, shopCurrencyGem)
	}

	resources.Coin = 0
	resources.Gem = 0
	resources.WalletMigrated = true

	raw, err := json.Marshal(playerResourcesForStorage(resources))
	if err != nil {
		return PlayerResources{}, "", err
	}

	walletUpdates := []*runtime.WalletUpdate(nil)
	if len(changeset) > 0 {
		walletUpdates = []*runtime.WalletUpdate{
			{
				UserID:    userID,
				Changeset: changeset,
				Metadata: map[string]interface{}{
					"source": "legacy_resources_migration",
				},
			},
		}
	}

	acks, _, err := nk.MultiUpdate(ctx, nil, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerStateKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	}, nil, walletUpdates, true)
	if err != nil {
		return PlayerResources{}, "", err
	}

	if len(acks) > 0 {
		version = acks[0].GetVersion()
	}
	return resources, version, nil
}

func readPlayerWallet(ctx context.Context, nk runtime.NakamaModule, userID string) (map[string]int64, error) {
	account, err := nk.AccountGetId(ctx, userID)
	if err != nil {
		return nil, err
	}

	wallet := map[string]int64{}
	raw := strings.TrimSpace(account.GetWallet())
	if raw == "" {
		return wallet, nil
	}
	if err := json.Unmarshal([]byte(raw), &wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

func applyWalletToResources(resources PlayerResources, wallet map[string]int64) PlayerResources {
	resources.Coin = int(wallet[shopCurrencyCoin])
	resources.Gem = int(wallet[shopCurrencyGem])
	return normalizePlayerResources(resources)
}
