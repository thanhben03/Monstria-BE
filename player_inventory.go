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

// InventoryItemStack is one stack in the common phase-5 inventory contract.
type InventoryItemStack struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type legacyPlayerInventory struct {
	Pots  []InventoryItemStack `json:"pots"`
	Seeds []InventoryItemStack `json:"seeds"`
	Items []InventoryItemStack `json:"items"`
}

// PlayerInventory is persisted under player_state / inventory.
type PlayerInventory struct {
	Items []InventoryItemStack `json:"items"`
}

func normalizePlayerInventory(inv PlayerInventory) PlayerInventory {
	items, err := normalizeInventoryStacks(inv.Items)
	if err != nil {
		items = []InventoryItemStack{}
	}
	return PlayerInventory{Items: items}
}

func decodePlayerInventory(raw []byte) (PlayerInventory, error) {
	var legacy legacyPlayerInventory
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return PlayerInventory{}, err
	}

	items := make([]InventoryItemStack, 0, len(legacy.Pots)+len(legacy.Seeds)+len(legacy.Items))
	items = append(items, legacy.Pots...)
	items = append(items, legacy.Seeds...)
	items = append(items, legacy.Items...)

	normalized, err := normalizeInventoryStacks(items)
	if err != nil {
		return PlayerInventory{}, err
	}
	return PlayerInventory{Items: normalized}, nil
}

func defaultPlayerInventory() PlayerInventory {
	return PlayerInventory{Items: []InventoryItemStack{}}
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

	inv, err := decodePlayerInventory([]byte(objs[0].GetValue()))
	if err != nil {
		return PlayerInventory{}, "", err
	}
	if inv.Items == nil {
		inv.Items = []InventoryItemStack{}
	}
	return inv, objs[0].GetVersion(), nil
}

func writePlayerInventory(ctx context.Context, nk runtime.NakamaModule, userID string, version string, inv PlayerInventory) error {
	inv = normalizePlayerInventory(inv)
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

// normalizeInventoryStacks merges duplicate itemId, drops invalid rows, sorts by itemId for stable JSON.
func normalizeInventoryStacks(stacks []InventoryItemStack) ([]InventoryItemStack, error) {
	byID := make(map[string]int)
	for _, p := range stacks {
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
	out := make([]InventoryItemStack, 0, len(byID))
	for id, q := range byID {
		out = append(out, InventoryItemStack{ItemID: id, Quantity: q})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out, nil
}

func consumeInventoryItem(inv *PlayerInventory, itemID string, quantity int, emptyMessage string) error {
	id := strings.TrimSpace(itemID)
	if id == "" {
		return runtime.NewError("itemId is required", 3)
	}
	if quantity < 1 {
		return runtime.NewError("quantity must be positive", 3)
	}
	for i := 0; i < len(inv.Items); i++ {
		if inv.Items[i].ItemID != id {
			continue
		}
		if inv.Items[i].Quantity < quantity {
			return runtime.NewError(emptyMessage, 3)
		}
		inv.Items[i].Quantity -= quantity
		if inv.Items[i].Quantity == 0 {
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
		}
		sort.Slice(inv.Items, func(a, b int) bool { return inv.Items[a].ItemID < inv.Items[b].ItemID })
		return nil
	}
	return runtime.NewError(emptyMessage, 3)
}

// ConsumeOnePot removes one unit of a pot item from the common inventory (mutates inv).
func ConsumeOnePot(inv *PlayerInventory, itemID string) error {
	return consumeInventoryItem(inv, itemID, 1, "not enough pots in inventory")
}

// ConsumeOneSeed removes one unit of a seed item from the common inventory (mutates inv).
func ConsumeOneSeed(inv *PlayerInventory, itemID string) error {
	if strings.TrimSpace(itemID) == "" {
		return runtime.NewError("seedItemId is required", 3)
	}
	return consumeInventoryItem(inv, itemID, 1, "not enough seeds in inventory")
}

// ConsumeOneItem removes one unit of itemID from the common inventory (mutates inv).
func ConsumeOneItem(inv *PlayerInventory, itemID string) error {
	return consumeInventoryItem(inv, itemID, 1, "not enough items in inventory")
}

// AddInventoryItem adds quantity units of itemID into the common inventory (mutates inv).
func AddInventoryItem(inv *PlayerInventory, itemID string, quantity int) error {
	id := strings.TrimSpace(itemID)
	if id == "" {
		return runtime.NewError("itemId is required", 3)
	}
	if quantity < 1 {
		return runtime.NewError("quantity must be positive", 3)
	}

	inv.Items = append(inv.Items, InventoryItemStack{ItemID: id, Quantity: quantity})
	items, err := normalizeInventoryStacks(inv.Items)
	if err != nil {
		return err
	}
	inv.Items = items
	return nil
}

// AddItem adds quantity units of itemID into the common inventory (mutates inv).
func AddItem(inv *PlayerInventory, itemID string, quantity int) error {
	return AddInventoryItem(inv, itemID, quantity)
}

// AddPot adds quantity units of a pot item into the common inventory (mutates inv).
func AddPot(inv *PlayerInventory, itemID string, quantity int) error {
	return AddInventoryItem(inv, itemID, quantity)
}

// AddSeed adds quantity units of a seed item into the common inventory (mutates inv).
func AddSeed(inv *PlayerInventory, itemID string, quantity int) error {
	return AddInventoryItem(inv, itemID, quantity)
}
