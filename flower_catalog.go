package main

import (
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
	SeedItemID     string `json:"seedItemId"`
	FlowerItemID   string `json:"flowerItemId"`
	GrowSeconds    int64  `json:"growSeconds"`
	RewardItemID   string `json:"rewardItemId"`
	RewardQuantity int    `json:"rewardQuantity"`
}

const flowerCatalogFileName = "flower_catalog.json"

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
		if _, ok := seenSeeds[def.SeedItemID]; ok {
			return nil, fmt.Errorf("duplicate seedItemId %q", def.SeedItemID)
		}

		seenSeeds[def.SeedItemID] = struct{}{}
		out = append(out, def)
	}
	return out, nil
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
