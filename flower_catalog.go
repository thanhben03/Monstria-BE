package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

// FlowerDefinition is backend-owned gameplay data for a seed/flower.
// Unity may mirror this by ID for visuals, but harvest validation uses this table.
type FlowerDefinition struct {
	SeedItemID     string   `json:"seedItemId"`
	FlowerItemID   string   `json:"flowerItemId"`
	GrowSeconds    int64    `json:"growSeconds"`
	RewardItemID   string   `json:"rewardItemId"`
	RewardQuantity int      `json:"rewardQuantity"`
	ExpReward      int      `json:"expReward,omitempty"`
	Disease        []string `json:"disease,omitempty"`
}

const flowerCatalogFileName = "flower_catalog.json"

const (
	flowerCatalogStorageCollection = "catalog"
	flowerCatalogStorageKey        = "flower_catalog"
	flowerCatalogStorageUserID     = ""
)

var errFlowerCatalogStorageNotFound = errors.New("flower catalog storage not found")

type flowerCatalogConfig struct {
	Flowers []FlowerDefinition `json:"flowers"`
}

var flowerDefinitions []FlowerDefinition
var flowerDefinitionBySeed = buildFlowerDefinitionBySeed(flowerDefinitions)

func init() {
	_ = loadFlowerDefinitions()
}

func loadFlowerDefinitions() error {
	path, err := resolveFlowerCatalogPath()
	if err != nil {
		return err
	}
	return loadFlowerDefinitionsFromFile(path)
}

func resolveFlowerCatalogPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("FLOWER_CATALOG_PATH")); path != "" {
		return path, nil
	}

	candidates := []string{
		flowerCatalogFileName,
		filepath.Join("modules", flowerCatalogFileName),
		filepath.Join("/nakama/data/modules", flowerCatalogFileName),
		filepath.Join("/nakama/data", flowerCatalogFileName),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", flowerCatalogFileName)
}

func loadFlowerDefinitionsFromFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read flower catalog %q: %w", path, err)
	}
	defs, err := parseFlowerDefinitions(raw)
	if err != nil {
		return fmt.Errorf("parse flower catalog %q: %w", path, err)
	}
	setFlowerDefinitions(defs)
	return nil
}

func loadFlowerDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) error {
	defs, err := readFlowerDefinitionsFromStorage(ctx, nk)
	if err != nil {
		return err
	}
	setFlowerDefinitions(defs)
	return nil
}

func bootstrapFlowerDefinitionsStorage(ctx context.Context, nk runtime.NakamaModule) error {
	if err := loadFlowerDefinitionsFromStorage(ctx, nk); err == nil {
		return nil
	} else if !errors.Is(err, errFlowerCatalogStorageNotFound) {
		return err
	}

	path, err := resolveFlowerCatalogPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read flower catalog %q: %w", path, err)
	}
	if _, err := parseFlowerDefinitions(raw); err != nil {
		return fmt.Errorf("parse flower catalog %q: %w", path, err)
	}

	if _, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      flowerCatalogStorageCollection,
			Key:             flowerCatalogStorageKey,
			UserID:          flowerCatalogStorageUserID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  2,
			PermissionWrite: 0,
		},
	}); err != nil {
		return fmt.Errorf("write flower catalog storage: %w", err)
	}

	return loadFlowerDefinitionsFromStorage(ctx, nk)
}

func readFlowerDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) ([]FlowerDefinition, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: flowerCatalogStorageCollection,
			Key:        flowerCatalogStorageKey,
			UserID:     flowerCatalogStorageUserID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("read flower catalog storage: %w", err)
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("%w: %s/%s", errFlowerCatalogStorageNotFound, flowerCatalogStorageCollection, flowerCatalogStorageKey)
	}

	defs, err := parseFlowerDefinitions([]byte(objs[0].GetValue()))
	if err != nil {
		return nil, fmt.Errorf("parse flower catalog storage: %w", err)
	}
	return defs, nil
}

func parseFlowerDefinitions(raw []byte) ([]FlowerDefinition, error) {
	var config flowerCatalogConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		var defs []FlowerDefinition
		if arrayErr := json.Unmarshal(raw, &defs); arrayErr != nil {
			return nil, err
		}
		config.Flowers = defs
	}
	return validateFlowerDefinitions(config.Flowers)
}

func validateFlowerDefinitions(defs []FlowerDefinition) ([]FlowerDefinition, error) {
	if len(defs) == 0 {
		return nil, errors.New("flowers must contain at least one definition")
	}

	out := make([]FlowerDefinition, 0, len(defs))
	seenSeeds := make(map[string]struct{}, len(defs))
	for i, def := range defs {
		def.SeedItemID = strings.TrimSpace(def.SeedItemID)
		def.FlowerItemID = strings.TrimSpace(def.FlowerItemID)
		def.RewardItemID = strings.TrimSpace(def.RewardItemID)
		def.Disease = normalizeDiseaseTypes(def.Disease)

		if def.SeedItemID == "" {
			return nil, fmt.Errorf("flowers[%d].seedItemId is required", i)
		}
		if def.FlowerItemID == "" {
			return nil, fmt.Errorf("flowers[%d].flowerItemId is required", i)
		}
		if def.GrowSeconds < 1 {
			return nil, fmt.Errorf("flowers[%d].growSeconds must be greater than 0", i)
		}
		if def.RewardItemID == "" {
			return nil, fmt.Errorf("flowers[%d].rewardItemId is required", i)
		}
		if def.RewardQuantity < 1 {
			return nil, fmt.Errorf("flowers[%d].rewardQuantity must be greater than 0", i)
		}
		if def.ExpReward < 0 {
			return nil, fmt.Errorf("flowers[%d].expReward cannot be negative", i)
		}
		if _, ok := seenSeeds[def.SeedItemID]; ok {
			return nil, fmt.Errorf("duplicate seedItemId %q", def.SeedItemID)
		}

		seenSeeds[def.SeedItemID] = struct{}{}
		out = append(out, def)
	}
	return out, nil
}

func normalizeDiseaseTypes(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, diseaseType := range in {
		diseaseType = strings.TrimSpace(diseaseType)
		if diseaseType == "" {
			continue
		}
		if _, ok := seen[diseaseType]; ok {
			continue
		}
		seen[diseaseType] = struct{}{}
		out = append(out, diseaseType)
	}
	return out
}

func setFlowerDefinitions(defs []FlowerDefinition) {
	flowerDefinitions = append([]FlowerDefinition(nil), defs...)
	flowerDefinitionBySeed = buildFlowerDefinitionBySeed(flowerDefinitions)
}

func buildFlowerDefinitionBySeed(defs []FlowerDefinition) map[string]FlowerDefinition {
	out := make(map[string]FlowerDefinition, len(defs))
	for _, def := range defs {
		seedID := strings.TrimSpace(def.SeedItemID)
		if seedID == "" {
			continue
		}
		def.SeedItemID = seedID
		def.FlowerItemID = strings.TrimSpace(def.FlowerItemID)
		def.RewardItemID = strings.TrimSpace(def.RewardItemID)
		out[seedID] = def
	}
	return out
}

func listFlowerDefinitions() []FlowerDefinition {
	out := append([]FlowerDefinition(nil), flowerDefinitions...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].SeedItemID < out[j].SeedItemID
	})
	return out
}

func flowerDefinitionForSeed(seedItemID string) (FlowerDefinition, error) {
	seedID := strings.TrimSpace(seedItemID)
	if seedID == "" {
		return FlowerDefinition{}, runtime.NewError("seedItemId is required", 3)
	}
	def, ok := flowerDefinitionBySeed[seedID]
	if !ok {
		return FlowerDefinition{}, runtime.NewError("unknown seed item", 3)
	}
	if def.GrowSeconds < 1 {
		return FlowerDefinition{}, runtime.NewError("invalid flower growSeconds", 13)
	}
	if def.RewardItemID == "" || def.RewardQuantity < 1 {
		return FlowerDefinition{}, runtime.NewError("invalid flower reward", 13)
	}
	return def, nil
}
