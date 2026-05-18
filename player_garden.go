package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerGardenKey = "garden"
)

// PotPlant is optional flower state in a placed pot (after player uses a seed).
type PotPlant struct {
	SeedItemID      string `json:"seedItemId"`
	PlantedAt       int64  `json:"plantedAt"` // Unix seconds (server time)
	WateredAt       int64  `json:"wateredAt,omitempty"`
	GrowthStartedAt int64  `json:"growthStartedAt,omitempty"`
}

// SlotPlacement: one occupied slot — pot first, then optional plant from seed.
type SlotPlacement struct {
	SlotID    string    `json:"slotId"`
	PotItemID string    `json:"potItemId"`
	Plant     *PotPlant `json:"plant,omitempty"`
}

type slotPlacementWire struct {
	SlotID    string    `json:"slotId"`
	PotItemID string    `json:"potItemId"`
	ItemID    string    `json:"itemId"` // legacy: pot id was "itemId"
	Plant     *PotPlant `json:"plant,omitempty"`
}

func (s *SlotPlacement) UnmarshalJSON(data []byte) error {
	var w slotPlacementWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	s.SlotID = strings.TrimSpace(w.SlotID)
	s.PotItemID = strings.TrimSpace(w.PotItemID)
	if s.PotItemID == "" {
		s.PotItemID = strings.TrimSpace(w.ItemID)
	}
	s.Plant = w.Plant
	if s.Plant != nil {
		s.Plant.SeedItemID = strings.TrimSpace(s.Plant.SeedItemID)
		if s.Plant.SeedItemID == "" {
			s.Plant = nil
		}
	}
	return nil
}

func (s SlotPlacement) MarshalJSON() ([]byte, error) {
	type out struct {
		SlotID    string    `json:"slotId"`
		PotItemID string    `json:"potItemId"`
		Plant     *PotPlant `json:"plant,omitempty"`
	}
	return json.Marshal(out{
		SlotID:    s.SlotID,
		PotItemID: s.PotItemID,
		Plant:     s.Plant,
	})
}

// PlayerGarden holds only occupied slots (empty slots omitted).
type PlayerGarden struct {
	Placements []SlotPlacement `json:"placements"`
}

func defaultPlayerGarden() PlayerGarden {
	return PlayerGarden{Placements: []SlotPlacement{}}
}

func normalizeGardenPlacements(in []SlotPlacement) []SlotPlacement {
	by := make(map[string]SlotPlacement)
	for _, p := range in {
		sid := strings.TrimSpace(p.SlotID)
		if sid == "" {
			continue
		}
		pid := strings.TrimSpace(p.PotItemID)
		if pid == "" {
			continue
		}
		pl := p
		pl.SlotID = sid
		pl.PotItemID = pid
		if pl.Plant != nil {
			pl.Plant.SeedItemID = strings.TrimSpace(pl.Plant.SeedItemID)
			if pl.Plant.SeedItemID == "" {
				pl.Plant = nil
			}
		}
		by[sid] = pl
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SlotPlacement, 0, len(keys))
	for _, k := range keys {
		out = append(out, by[k])
	}
	return out
}

// gardenPlacePot adds a pot-only placement. Fails if slot already has a pot.
func gardenPlacePot(g *PlayerGarden, slotID, potItemID string) error {
	sid := strings.TrimSpace(slotID)
	pid := strings.TrimSpace(potItemID)
	if sid == "" {
		return runtime.NewError("slotId is required", 3)
	}
	if pid == "" {
		return runtime.NewError("itemId is required", 3)
	}

	for _, p := range g.Placements {
		if strings.TrimSpace(p.SlotID) == sid && strings.TrimSpace(p.PotItemID) != "" {
			return runtime.NewError("slot already occupied", 3)
		}
	}

	g.Placements = append(g.Placements, SlotPlacement{
		SlotID:    sid,
		PotItemID: pid,
		Plant:     nil,
	})
	g.Placements = normalizeGardenPlacements(g.Placements)
	return nil
}

func findPlacementIndex(g *PlayerGarden, slotID string) int {
	sid := strings.TrimSpace(slotID)
	for i := range g.Placements {
		if strings.TrimSpace(g.Placements[i].SlotID) == sid {
			return i
		}
	}
	return -1
}

// gardenPlantSeed sets plant on an existing pot; consumes no pot. slot must have pot, plant must be empty.
func gardenPlantSeed(g *PlayerGarden, slotID, seedItemID string, plantedAtUnix int64) error {
	sid := strings.TrimSpace(slotID)
	sidSeed := strings.TrimSpace(seedItemID)
	if sid == "" {
		return runtime.NewError("slotId is required", 3)
	}
	if sidSeed == "" {
		return runtime.NewError("seedItemId is required", 3)
	}

	idx := findPlacementIndex(g, sid)
	if idx < 0 {
		return runtime.NewError("no pot in this slot", 3)
	}
	if strings.TrimSpace(g.Placements[idx].PotItemID) == "" {
		return runtime.NewError("no pot in this slot", 3)
	}
	if g.Placements[idx].Plant != nil {
		return runtime.NewError("pot already has a plant", 3)
	}

	g.Placements[idx].Plant = &PotPlant{
		SeedItemID: sidSeed,
		PlantedAt:  plantedAtUnix,
	}
	g.Placements = normalizeGardenPlacements(g.Placements)
	return nil
}

func gardenWaterPlant(g *PlayerGarden, slotID string, wateredAtUnix int64) error {
	sid := strings.TrimSpace(slotID)
	if sid == "" {
		return runtime.NewError("slotId is required", 3)
	}

	idx := findPlacementIndex(g, sid)
	if idx < 0 || strings.TrimSpace(g.Placements[idx].PotItemID) == "" {
		return runtime.NewError("no pot in this slot", 3)
	}
	if g.Placements[idx].Plant == nil {
		return runtime.NewError("no plant in this slot", 3)
	}
	if g.Placements[idx].Plant.GrowthStartedAt > 0 {
		return runtime.NewError("plant already watered", 3)
	}

	g.Placements[idx].Plant.WateredAt = wateredAtUnix
	g.Placements[idx].Plant.GrowthStartedAt = wateredAtUnix
	g.Placements = normalizeGardenPlacements(g.Placements)
	return nil
}

func gardenHarvestPlant(g *PlayerGarden, slotID string, nowUnix int64) (harvestReward, error) {
	sid := strings.TrimSpace(slotID)
	if sid == "" {
		return harvestReward{}, runtime.NewError("slotId is required", 3)
	}

	idx := findPlacementIndex(g, sid)
	if idx < 0 || strings.TrimSpace(g.Placements[idx].PotItemID) == "" {
		return harvestReward{}, runtime.NewError("no pot in this slot", 3)
	}
	if g.Placements[idx].Plant == nil {
		return harvestReward{}, runtime.NewError("no plant in this slot", 3)
	}

	plant := g.Placements[idx].Plant
	def, err := flowerDefinitionForSeed(plant.SeedItemID)
	if err != nil {
		return harvestReward{}, err
	}

	if plant.GrowthStartedAt < 1 {
		return harvestReward{}, runtime.NewError("plant needs water", 3)
	}

	harvestAt := plant.GrowthStartedAt + def.GrowSeconds
	if nowUnix < harvestAt {
		return harvestReward{}, runtime.NewError("plant is not ready", 3)
	}

	g.Placements[idx].Plant = nil
	g.Placements = normalizeGardenPlacements(g.Placements)
	return harvestReward{ItemID: def.RewardItemID, Quantity: def.RewardQuantity}, nil
}

func nowUnixSeconds() int64 {
	return time.Now().Unix()
}

func initPlayerGarden(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	g := defaultPlayerGarden()
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func readPlayerGarden(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerGarden, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerGardenKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerGarden{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerGarden(), "", nil
	}

	var g PlayerGarden
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &g); err != nil {
		return PlayerGarden{}, "", err
	}
	if g.Placements == nil {
		g.Placements = []SlotPlacement{}
	}
	g.Placements = normalizeGardenPlacements(g.Placements)
	return g, objs[0].GetVersion(), nil
}

func writePlayerGarden(ctx context.Context, nk runtime.NakamaModule, userID string, version string, g PlayerGarden) error {
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerGardenKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}
