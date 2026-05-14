package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerGardenKey = "garden"
)

// SlotPlacement maps one logical slot (client: CloudLayer layerIndex + slot index) to a pot itemId.
type SlotPlacement struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

// PlayerGarden holds only occupied slots (empty slots are omitted).
type PlayerGarden struct {
	Placements []SlotPlacement `json:"placements"`
}

func defaultPlayerGarden() PlayerGarden {
	return PlayerGarden{Placements: []SlotPlacement{}}
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

func normalizeGardenPlacements(in []SlotPlacement) []SlotPlacement {
	by := make(map[string]string)
	for _, p := range in {
		sid := strings.TrimSpace(p.SlotID)
		if sid == "" {
			continue
		}
		iid := strings.TrimSpace(p.ItemID)
		if iid == "" {
			continue
		}
		by[sid] = iid
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SlotPlacement, 0, len(keys))
	for _, k := range keys {
		out = append(out, SlotPlacement{SlotID: k, ItemID: by[k]})
	}
	return out
}

// gardenPlaceOccupied adds or replaces occupancy for one slot. Fails if slot already has a pot.
func gardenPlaceOccupied(g *PlayerGarden, slotID, itemID string) error {
	sid := strings.TrimSpace(slotID)
	iid := strings.TrimSpace(itemID)
	if sid == "" {
		return runtime.NewError("slotId is required", 3)
	}
	if iid == "" {
		return runtime.NewError("itemId is required", 3)
	}

	by := make(map[string]string)
	for _, p := range g.Placements {
		s := strings.TrimSpace(p.SlotID)
		if s == "" {
			continue
		}
		if id := strings.TrimSpace(p.ItemID); id != "" {
			by[s] = id
		}
	}
	if existing, ok := by[sid]; ok && existing != "" {
		return runtime.NewError("slot already occupied", 3)
	}
	by[sid] = iid

	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pl := make([]SlotPlacement, 0, len(keys))
	for _, k := range keys {
		pl = append(pl, SlotPlacement{SlotID: k, ItemID: by[k]})
	}
	g.Placements = pl
	return nil
}
