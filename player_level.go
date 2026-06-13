package main

import "github.com/heroiclabs/nakama-common/runtime"

type LevelUpResult struct {
	ExpAdded     int  `json:"expAdded"`
	LevelBefore  int  `json:"levelBefore"`
	LevelAfter   int  `json:"levelAfter"`
	ExpBefore    int  `json:"expBefore"`
	ExpAfter     int  `json:"expAfter"`
	LevelsGained int  `json:"levelsGained"`
	LeveledUp    bool `json:"leveledUp"`
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
	}

	result.LevelAfter = resources.Level
	result.ExpAfter = resources.Exp
	result.LeveledUp = result.LevelsGained > 0
	return result, nil
}
