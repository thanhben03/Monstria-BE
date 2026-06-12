package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerDecorKey = "decor"
)

type DecorPlacement struct {
	SlotID string `json:"slotId"`
	ItemID string `json:"itemId"`
}

type PlayerDecor struct {
	Placements []DecorPlacement `json:"placements"`
}

func defaultPlayerDecor() PlayerDecor {
	return PlayerDecor{Placements: []DecorPlacement{}}
}

func normalizeDecorPlacements(in []DecorPlacement) []DecorPlacement {
	bySlot := make(map[string]DecorPlacement)
	for _, p := range in {
		slotID := strings.TrimSpace(p.SlotID)
		itemID := strings.TrimSpace(p.ItemID)
		if slotID == "" || itemID == "" {
			continue
		}
		bySlot[slotID] = DecorPlacement{SlotID: slotID, ItemID: itemID}
	}

	keys := make([]string, 0, len(bySlot))
	for k := range bySlot {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]DecorPlacement, 0, len(keys))
	for _, k := range keys {
		out = append(out, bySlot[k])
	}
	return out
}

func decorPlaceItem(decor *PlayerDecor, slotID string, itemID string) error {
	slotID = strings.TrimSpace(slotID)
	itemID = strings.TrimSpace(itemID)
	if slotID == "" {
		return runtime.NewError("slotId is required", 3)
	}
	if itemID == "" {
		return runtime.NewError("itemId is required", 3)
	}

	for _, p := range decor.Placements {
		if strings.TrimSpace(p.SlotID) == slotID && strings.TrimSpace(p.ItemID) != "" {
			return runtime.NewError("slot already occupied", 3)
		}
	}

	decor.Placements = append(decor.Placements, DecorPlacement{SlotID: slotID, ItemID: itemID})
	decor.Placements = normalizeDecorPlacements(decor.Placements)
	return nil
}

func initPlayerDecor(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	decor := defaultPlayerDecor()
	raw, err := json.Marshal(decor)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerDecorKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func readPlayerDecor(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerDecor, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerDecorKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerDecor{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerDecor(), "", nil
	}

	var decor PlayerDecor
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &decor); err != nil {
		return PlayerDecor{}, "", err
	}
	decor.Placements = normalizeDecorPlacements(decor.Placements)
	return decor, objs[0].GetVersion(), nil
}

func writePlayerDecor(ctx context.Context, nk runtime.NakamaModule, userID string, version string, decor PlayerDecor) error {
	decor.Placements = normalizeDecorPlacements(decor.Placements)
	raw, err := json.Marshal(decor)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerDecorKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}
