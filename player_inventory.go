package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	playerInventoryKey = "inventory"
)

// PotStack is one stack of pots in inventory (matches client JSON).
type PotStack struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

// PlayerInventory is persisted under player_state / inventory.
type PlayerInventory struct {
	Pots []PotStack `json:"pots"`
}

func defaultPlayerInventory() PlayerInventory {
	return PlayerInventory{Pots: []PotStack{}}
}

func initPlayerInventory(ctx context.Context, nk runtime.NakamaModule, userID string) error {
	inv := defaultPlayerInventory()
	raw, err := json.Marshal(inv)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

func readPlayerInventory(ctx context.Context, nk runtime.NakamaModule, userID string) (PlayerInventory, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: playerStateCollection,
			Key:        playerInventoryKey,
			UserID:     userID,
		},
	})
	if err != nil {
		return PlayerInventory{}, "", err
	}
	if len(objs) == 0 {
		return defaultPlayerInventory(), "", nil
	}

	var inv PlayerInventory
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &inv); err != nil {
		return PlayerInventory{}, "", err
	}
	if inv.Pots == nil {
		inv.Pots = []PotStack{}
	}
	return inv, objs[0].GetVersion(), nil
}

func writePlayerInventory(ctx context.Context, nk runtime.NakamaModule, userID string, version string, inv PlayerInventory) error {
	raw, err := json.Marshal(inv)
	if err != nil {
		return err
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(raw),
			Version:         version,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	})
	return err
}

// normalizePots merges duplicate itemId, drops invalid rows, sorts by itemId for stable JSON.
func normalizePots(pots []PotStack) ([]PotStack, error) {
	byID := make(map[string]int)
	for _, p := range pots {
		id := strings.TrimSpace(p.ItemID)
		if id == "" {
			continue
		}
		if p.Quantity < 0 {
			return nil, runtime.NewError("quantity cannot be negative", 3)
		}
		if p.Quantity == 0 {
			continue
		}
		byID[id] += p.Quantity
	}
	out := make([]PotStack, 0, len(byID))
	for id, q := range byID {
		out = append(out, PotStack{ItemID: id, Quantity: q})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out, nil
}

// ConsumeOnePot removes one unit of itemID from inv (mutates inv).
func ConsumeOnePot(inv *PlayerInventory, itemID string) error {
	id := strings.TrimSpace(itemID)
	if id == "" {
		return runtime.NewError("itemId is required", 3)
	}
	for i := 0; i < len(inv.Pots); i++ {
		if inv.Pots[i].ItemID != id {
			continue
		}
		if inv.Pots[i].Quantity < 1 {
			return runtime.NewError("not enough pots in inventory", 3)
		}
		inv.Pots[i].Quantity--
		if inv.Pots[i].Quantity == 0 {
			inv.Pots = append(inv.Pots[:i], inv.Pots[i+1:]...)
		}
		sort.Slice(inv.Pots, func(a, b int) bool { return inv.Pots[a].ItemID < inv.Pots[b].ItemID })
		return nil
	}
	return runtime.NewError("not enough pots in inventory", 3)
}
