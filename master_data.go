package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/heroiclabs/nakama-common/runtime"
)

// Readers hold this lock only while accessing immutable catalog data.
var masterDataMu sync.RWMutex

// Serialize reads and publication so overlapping reloads cannot publish out of order.
var masterDataReloadMu sync.Mutex

var masterDataKeys = []string{"flower_catalog", "level_config", "shop_catalog", "pet_catalog"}

type masterDataReloadResult struct {
	Version string         `json:"version"`
	Counts  map[string]int `json:"counts"`
}

func authorizeMasterDataReload(ctx context.Context) error {
	userID, _ := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if userID == "" {
		return runtime.NewError("authentication required", 16)
	}
	// Temporarily allow any authenticated user; admin allowlist checking is disabled.
	return nil
}

func ReloadMasterDataRPC(ctx context.Context, logger runtime.Logger, _ *sql.DB, nk runtime.NakamaModule, _ string) (string, error) {
	if err := authorizeMasterDataReload(ctx); err != nil {
		return "", err
	}
	result, err := reloadMasterData(ctx, nk)
	if err != nil {
		logger.Error("reload master data failed: %v", err)
		return "", runtime.NewError("reload failed; active cache unchanged: "+err.Error(), 9)
	}
	logger.Info("master data reloaded: version=%s counts=%v", result.Version, result.Counts)
	raw, err := json.Marshal(result)
	return string(raw), err
}

func reloadMasterData(ctx context.Context, nk runtime.NakamaModule) (masterDataReloadResult, error) {
	masterDataReloadMu.Lock()
	defer masterDataReloadMu.Unlock()
	var result masterDataReloadResult
	reads := make([]*runtime.StorageRead, 0, len(masterDataKeys))
	for _, key := range masterDataKeys {
		reads = append(reads, &runtime.StorageRead{Collection: "catalog", Key: key, UserID: ""})
	}
	objects, err := nk.StorageRead(ctx, reads)
	if err != nil {
		return result, fmt.Errorf("read catalog storage: %w", err)
	}
	values := make(map[string][]byte, len(objects))
	for _, object := range objects {
		values[object.Key] = []byte(object.Value)
	}
	for _, key := range masterDataKeys {
		if len(values[key]) == 0 {
			return result, fmt.Errorf("missing catalog/%s", key)
		}
	}
	flowers, err := parseFlowerDefinitions(values["flower_catalog"])
	if err != nil {
		return result, fmt.Errorf("flower_catalog: %w", err)
	}
	levels, err := parseLevelDefinitions(values["level_config"])
	if err != nil {
		return result, fmt.Errorf("level_config: %w", err)
	}
	shop, err := parseShopItemDefinitions(values["shop_catalog"])
	if err != nil {
		return result, fmt.Errorf("shop_catalog: %w", err)
	}
	pets, err := parsePetCatalog(values["pet_catalog"])
	if err != nil {
		return result, fmt.Errorf("pet_catalog: %w", err)
	}
	hash := sha256.New()
	for _, key := range masterDataKeys {
		hash.Write([]byte(key))
		hash.Write([]byte{0})
		hash.Write(values[key])
		hash.Write([]byte{0})
	}
	result.Version = hex.EncodeToString(hash.Sum(nil))
	result.Counts = map[string]int{"flowers": len(flowers), "levels": len(levels), "shopItems": len(shop), "pets": len(pets.Pets)}
	// Validation finishes before any active state changes. Publish all four together.
	masterDataMu.Lock()
	setFlowerDefinitionsLocked(flowers)
	setLevelDefinitionsLocked(levels)
	setShopItemDefinitionsLocked(shop)
	petCatalog = pets
	masterDataMu.Unlock()
	return result, nil
}
