package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	bauCuaDefaultRoomID      = "default"
	bauCuaRoundSeconds       = int64(180)
	bauCuaResultLockSeconds  = int64(5)
	bauCuaPhaseBettingOpen   = "betting_open"
	bauCuaPhaseWaitingResult = "waiting_result"
)

type bauCuaStatePayload struct {
	RoomID string `json:"roomId"`
}

type bauCuaRoundStateResponse struct {
	RoomID        string                   `json:"roomId"`
	RoundID       string                   `json:"roundId"`
	Phase         string                   `json:"phase"`
	ServerTime    int64                    `json:"serverTime"`
	BettingEndsAt int64                    `json:"bettingEndsAt"`
	Slots         []bauCuaSlotStateDTO     `json:"slots"`
	BetItems      []bauCuaInventoryItemDTO `json:"betItems"`
	Result        *bauCuaResultDTO         `json:"result,omitempty"`
}

type bauCuaSlotStateDTO struct {
	SlotID      string              `json:"slotId"`
	SymbolID    string              `json:"symbolId"`
	PlayerCount int                 `json:"playerCount"`
	MyBets      []bauCuaBetStackDTO `json:"myBets"`
}

type bauCuaBetStackDTO struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type bauCuaInventoryItemDTO struct {
	ItemID      string `json:"itemId"`
	DisplayName string `json:"displayName"`
	Quantity    int    `json:"quantity"`
}

type bauCuaResultDTO struct {
	SymbolIDs []string `json:"symbolIds"`
}

func BauCuaGetStateRPC(
	ctx context.Context,
	logger runtime.Logger,
	_ *sql.DB,
	nk runtime.NakamaModule,
	payload string,
) (string, error) {
	userID, ok := ctx.Value(runtime.RUNTIME_CTX_USER_ID).(string)
	if !ok || userID == "" {
		return "", runtime.NewError("unauthorized", 16)
	}

	var body bauCuaStatePayload
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return "", runtime.NewError("invalid JSON payload", 3)
		}
	}

	roomID := normalizeBauCuaRoomID(body.RoomID)
	inv, _, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read bau cua inventory: %v", err)
		return "", runtime.NewError("failed to load bau cua state", 13)
	}

	definitions, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		logger.Warn("read shop catalog for bau cua display names: %v", err)
		definitions = nil
	}

	now := nowUnixSeconds()
	roundStart := now - (now % bauCuaRoundSeconds)
	bettingEndsAt := roundStart + bauCuaRoundSeconds - bauCuaResultLockSeconds
	phase := bauCuaPhaseBettingOpen
	if now >= bettingEndsAt {
		phase = bauCuaPhaseWaitingResult
	}

	out := bauCuaRoundStateResponse{
		RoomID:        roomID,
		RoundID:       fmt.Sprintf("%s_%d", roomID, roundStart),
		Phase:         phase,
		ServerTime:    now,
		BettingEndsAt: bettingEndsAt,
		Slots:         defaultBauCuaSlots(),
		BetItems:      buildBauCuaBetItems(inv, definitions),
	}

	raw, err := json.Marshal(out)
	if err != nil {
		logger.Error("marshal bau cua state: %v", err)
		return "", runtime.NewError("failed to load bau cua state", 13)
	}
	return string(raw), nil
}

func normalizeBauCuaRoomID(roomID string) string {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return bauCuaDefaultRoomID
	}
	return roomID
}

func defaultBauCuaSlots() []bauCuaSlotStateDTO {
	symbols := []string{"O1", "O2", "O3", "O4", "O5", "O6"}
	out := make([]bauCuaSlotStateDTO, 0, len(symbols))
	for i, symbolID := range symbols {
		out = append(out, bauCuaSlotStateDTO{
			SlotID:      fmt.Sprintf("slot_%d", i+1),
			SymbolID:    symbolID,
			PlayerCount: 0,
			MyBets:      []bauCuaBetStackDTO{},
		})
	}
	return out
}

func buildBauCuaBetItems(inv PlayerInventory, definitions []ShopItemDefinition) []bauCuaInventoryItemDTO {
	displayNames := buildBauCuaDisplayNameByItemID(definitions)
	byID := make(map[string]int)

	addStacks := func(stacks []PotStack) {
		for _, stack := range stacks {
			itemID := strings.TrimSpace(stack.ItemID)
			if itemID == "" || stack.Quantity <= 0 {
				continue
			}
			byID[itemID] += stack.Quantity
		}
	}

	addStacks(inv.Pots)
	addStacks(inv.Seeds)
	addStacks(inv.Items)

	ids := make([]string, 0, len(byID))
	for itemID := range byID {
		ids = append(ids, itemID)
	}
	sort.Strings(ids)

	out := make([]bauCuaInventoryItemDTO, 0, len(ids))
	for _, itemID := range ids {
		displayName := displayNames[itemID]
		if displayName == "" {
			displayName = itemID
		}
		out = append(out, bauCuaInventoryItemDTO{
			ItemID:      itemID,
			DisplayName: displayName,
			Quantity:    byID[itemID],
		})
	}
	return out
}

func buildBauCuaDisplayNameByItemID(definitions []ShopItemDefinition) map[string]string {
	out := make(map[string]string, len(definitions))
	for _, def := range definitions {
		itemID := strings.TrimSpace(def.GrantItemID)
		if itemID == "" {
			continue
		}
		displayName := strings.TrimSpace(def.NameItem)
		if displayName == "" {
			displayName = strings.TrimSpace(def.ShopItemID)
		}
		if displayName == "" {
			displayName = itemID
		}
		if _, exists := out[itemID]; !exists {
			out[itemID] = displayName
		}
	}
	return out
}
