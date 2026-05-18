package main

import (
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

var flowerDefinitions = []FlowerDefinition{
	{
		SeedItemID:     "seed_rose",
		FlowerItemID:   "flower_rose",
		GrowSeconds:    60,
		RewardItemID:   "flower_rose",
		RewardQuantity: 1,
	},
	{
		SeedItemID:     "seed_sunflower",
		FlowerItemID:   "flower_sunflower",
		GrowSeconds:    120,
		RewardItemID:   "flower_sunflower",
		RewardQuantity: 1,
	},
}

var flowerDefinitionBySeed = buildFlowerDefinitionBySeed(flowerDefinitions)

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
