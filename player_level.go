package main

import (
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

type LevelUpResult struct {
	ExpAdded     int           `json:"expAdded"`
	LevelBefore  int           `json:"levelBefore"`
	LevelAfter   int           `json:"levelAfter"`
	ExpBefore    int           `json:"expBefore"`
	ExpAfter     int           `json:"expAfter"`
	LevelsGained int           `json:"levelsGained"`
	RewardLevels []int         `json:"rewardLevels,omitempty"`
	Rewards      []LevelReward `json:"rewards,omitempty"`
	LeveledUp    bool          `json:"leveledUp"`
}

func AddPlayerExp(resources *PlayerResources, amount int) (LevelUpResult, error) {
	if resources == nil {
		return LevelUpResult{}, runtime.NewError("resources is required", 13)
	}
	if amount < 1 {
		return LevelUpResult{}, runtime.NewError("exp amount must be positive", 3)
	}

	*resources = normalizePlayerResources(*resources)
	result := LevelUpResult{
		ExpAdded:    amount,
		LevelBefore: resources.Level,
		ExpBefore:   resources.Exp,
	}

	resources.Exp += amount
	for {
		needed := expToNextLevel(resources.Level)
		if needed <= 0 {
			resources.Exp = 0
			break
		}
		if resources.Exp < needed {
			break
		}

		resources.Exp -= needed
		resources.Level++
		result.LevelsGained++
		result.RewardLevels = append(result.RewardLevels, resources.Level)
		result.Rewards = append(result.Rewards, levelRewardsForLevel(resources.Level)...)
	}

	result.Rewards = aggregateLevelRewards(result.Rewards)
	result.LevelAfter = resources.Level
	result.ExpAfter = resources.Exp
	result.LeveledUp = result.LevelsGained > 0
	return result, nil
}

func GrantLevelUpRewards(inv *PlayerInventory, result LevelUpResult) error {
	if inv == nil {
		return runtime.NewError("inventory is required", 13)
	}
	if !result.LeveledUp {
		return nil
	}

	for _, reward := range result.Rewards {
		if err := AddInventoryItem(inv, reward.ItemID, reward.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func aggregateLevelRewards(rewards []LevelReward) []LevelReward {
	if len(rewards) == 0 {
		return nil
	}

	byID := make(map[string]int, len(rewards))
	for _, reward := range rewards {
		id := strings.TrimSpace(reward.ItemID)
		if id == "" || reward.Quantity <= 0 {
			continue
		}
		byID[id] += reward.Quantity
	}
	if len(byID) == 0 {
		return nil
	}

	out := make([]LevelReward, 0, len(byID))
	for id, quantity := range byID {
		out = append(out, LevelReward{ItemID: id, Quantity: quantity})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ItemID < out[j].ItemID
	})
	return out
}
