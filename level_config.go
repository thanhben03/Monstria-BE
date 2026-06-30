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

const levelConfigFileName = "level_config.json"

const (
	levelConfigStorageCollection = "catalog"
	levelConfigStorageKey        = "level_config"
	levelConfigStorageUserID     = ""
)

type LevelDefinition struct {
	Level     int           `json:"level"`
	ExpToNext int           `json:"expToNext"`
	Rewards   []LevelReward `json:"rewards,omitempty"`
}

type LevelReward struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type levelConfig struct {
	Levels []LevelDefinition `json:"levels"`
}

var levelDefinitions []LevelDefinition
var expToNextByLevel = buildExpToNextByLevel(levelDefinitions)

var errLevelConfigStorageNotFound = errors.New("level config storage not found")

func init() {
	_ = loadLevelDefinitions()
}

func loadLevelDefinitions() error {
	path, err := resolveLevelConfigPath()
	if err != nil {
		return err
	}
	return loadLevelDefinitionsFromFile(path)
}

func resolveLevelConfigPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("LEVEL_CONFIG_PATH")); path != "" {
		return path, nil
	}

	candidates := []string{
		levelConfigFileName,
		filepath.Join("modules", levelConfigFileName),
		filepath.Join("/nakama/data/modules", levelConfigFileName),
		filepath.Join("/nakama/data", levelConfigFileName),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", levelConfigFileName)
}

func loadLevelDefinitionsFromFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read level config %q: %w", path, err)
	}
	defs, err := parseLevelDefinitions(raw)
	if err != nil {
		return fmt.Errorf("parse level config %q: %w", path, err)
	}
	setLevelDefinitions(defs)
	return nil
}

func loadLevelDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) error {
	defs, err := readLevelDefinitionsFromStorage(ctx, nk)
	if err != nil {
		return err
	}
	setLevelDefinitions(defs)
	return nil
}

func bootstrapLevelDefinitionsStorage(ctx context.Context, nk runtime.NakamaModule) error {
	if debugRPCsEnabled() {
		return writeLevelDefinitionsStorageFromFile(ctx, nk)
	}

	if err := loadLevelDefinitionsFromStorage(ctx, nk); err == nil {
		return nil
	} else if !errors.Is(err, errLevelConfigStorageNotFound) {
		return err
	}

	return writeLevelDefinitionsStorageFromFile(ctx, nk)
}

func writeLevelDefinitionsStorageFromFile(ctx context.Context, nk runtime.NakamaModule) error {
	path, err := resolveLevelConfigPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read level config %q: %w", path, err)
	}
	if _, err := parseLevelDefinitions(raw); err != nil {
		return fmt.Errorf("parse level config %q: %w", path, err)
	}

	if _, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      levelConfigStorageCollection,
			Key:             levelConfigStorageKey,
			UserID:          levelConfigStorageUserID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  2,
			PermissionWrite: 0,
		},
	}); err != nil {
		return fmt.Errorf("write level config storage: %w", err)
	}

	return loadLevelDefinitionsFromStorage(ctx, nk)
}

func readLevelDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) ([]LevelDefinition, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: levelConfigStorageCollection,
			Key:        levelConfigStorageKey,
			UserID:     levelConfigStorageUserID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("read level config storage: %w", err)
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("%w: %s/%s", errLevelConfigStorageNotFound, levelConfigStorageCollection, levelConfigStorageKey)
	}

	defs, err := parseLevelDefinitions([]byte(objs[0].GetValue()))
	if err != nil {
		return nil, fmt.Errorf("parse level config storage: %w", err)
	}
	return defs, nil
}

func parseLevelDefinitions(raw []byte) ([]LevelDefinition, error) {
	var config levelConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		var defs []LevelDefinition
		if arrayErr := json.Unmarshal(raw, &defs); arrayErr != nil {
			return nil, err
		}
		config.Levels = defs
	}
	return validateLevelDefinitions(config.Levels)
}

func validateLevelDefinitions(defs []LevelDefinition) ([]LevelDefinition, error) {
	if len(defs) == 0 {
		return nil, errors.New("levels must contain at least one definition")
	}

	out := append([]LevelDefinition(nil), defs...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Level < out[j].Level
	})

	seen := make(map[int]struct{}, len(out))
	for i, def := range out {
		if def.Level < 1 {
			return nil, fmt.Errorf("levels[%d].level must be greater than 0", i)
		}
		if def.Level != i+1 {
			return nil, fmt.Errorf("levels[%d].level must be %d", i, i+1)
		}
		if _, ok := seen[def.Level]; ok {
			return nil, fmt.Errorf("duplicate level %d", def.Level)
		}
		if def.ExpToNext < 0 {
			return nil, fmt.Errorf("levels[%d].expToNext cannot be negative", i)
		}
		if i < len(out)-1 && def.ExpToNext < 1 {
			return nil, fmt.Errorf("levels[%d].expToNext must be greater than 0 before max level", i)
		}
		if err := validateLevelRewards(def.Level, def.Rewards); err != nil {
			return nil, err
		}
		seen[def.Level] = struct{}{}
	}
	return out, nil
}

func setLevelDefinitions(defs []LevelDefinition) {
	levelDefinitions = append([]LevelDefinition(nil), defs...)
	expToNextByLevel = buildExpToNextByLevel(levelDefinitions)
}

func buildExpToNextByLevel(defs []LevelDefinition) map[int]int {
	out := make(map[int]int, len(defs))
	for _, def := range defs {
		out[def.Level] = def.ExpToNext
	}
	return out
}

func validateLevelRewards(level int, rewards []LevelReward) error {
	for i, reward := range rewards {
		if strings.TrimSpace(reward.ItemID) == "" {
			return fmt.Errorf("level %d rewards[%d].itemId is required", level, i)
		}
		if reward.Quantity < 1 {
			return fmt.Errorf("level %d rewards[%d].quantity must be greater than 0", level, i)
		}
	}
	return nil
}

func levelRewardsForLevel(level int) []LevelReward {
	for _, def := range levelDefinitions {
		if def.Level != level {
			continue
		}

		rewards := append([]LevelReward(nil), def.Rewards...)
		for i := range rewards {
			rewards[i].ItemID = strings.TrimSpace(rewards[i].ItemID)
		}
		return rewards
	}
	return nil
}

func expToNextLevel(level int) int {
	if level < 1 {
		level = 1
	}
	return expToNextByLevel[level]
}

func decoratePlayerResources(resources PlayerResources) PlayerResources {
	resources = normalizePlayerResources(resources)
	resources.ExpToNextLevel = expToNextLevel(resources.Level)
	if resources.ExpToNextLevel <= 0 {
		resources.ExpProgress = 1
		return resources
	}

	resources.ExpProgress = float64(resources.Exp) / float64(resources.ExpToNextLevel)
	if resources.ExpProgress < 0 {
		resources.ExpProgress = 0
	}
	if resources.ExpProgress > 1 {
		resources.ExpProgress = 1
	}
	return resources
}
