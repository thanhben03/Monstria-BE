package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const (
	bauCuaDefaultRoomID      = "default"
	bauCuaRoundSeconds       = int64(180)
	bauCuaResultLockSeconds  = int64(5)
	bauCuaStorageCollection  = "bau_cua"
	bauCuaBetsCollection     = "bau_cua_bets"
	bauCuaPhaseBettingOpen   = "betting_open"
	bauCuaPhaseWaitingResult = "waiting_result"
	bauCuaPhaseShowingResult = "showing_result"
	bauCuaItemTypePot        = "pot"
	bauCuaItemTypeSeed       = "seed"
	bauCuaItemTypeItem       = "item"
)

type bauCuaStatePayload struct {
	RoomID string `json:"roomId"`
}

type bauCuaPlaceBetPayload struct {
	RoundID  string `json:"roundId"`
	SlotID   string `json:"slotId"`
	SymbolID string `json:"symbolId"`
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
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
	ItemType string `json:"itemType,omitempty"`
}

type bauCuaInventoryItemDTO struct {
	ItemID      string `json:"itemId"`
	DisplayName string `json:"displayName"`
	Quantity    int    `json:"quantity"`
}

type bauCuaResultDTO struct {
	SymbolIDs []string `json:"symbolIds"`
}

type bauCuaPlaceBetResponse struct {
	State     bauCuaRoundStateResponse `json:"state"`
	Inventory PlayerInventory          `json:"inventory"`
}

type bauCuaRoundRecord struct {
	RoomID        string   `json:"roomId"`
	RoundID       string   `json:"roundId"`
	StartedAt     int64    `json:"startedAt"`
	BettingEndsAt int64    `json:"bettingEndsAt"`
	EndsAt        int64    `json:"endsAt"`
	Result        []string `json:"result,omitempty"`
	Resolved      bool     `json:"resolved"`
}

type bauCuaRoundBetsRecord struct {
	RoundID    string            `json:"roundId"`
	PlayerBets []bauCuaPlayerBet `json:"playerBets"`
}

type bauCuaUserBetRecord struct {
	RoundID string          `json:"roundId"`
	UserID  string          `json:"userId"`
	Slots   []bauCuaSlotBet `json:"slots"`
	Paid    bool            `json:"paid"`
	version string
}

type bauCuaPlayerBet struct {
	UserID string          `json:"userId"`
	Slots  []bauCuaSlotBet `json:"slots"`
}

type bauCuaSlotBet struct {
	SlotID   string              `json:"slotId"`
	SymbolID string              `json:"symbolId"`
	Items    []bauCuaBetStackDTO `json:"items"`
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
	if err := resolvePreviousBauCuaRound(ctx, logger, nk, roomID); err != nil {
		return "", err
	}

	round, roundVersion, err := getOrCreateCurrentBauCuaRound(ctx, nk, roomID)
	if err != nil {
		logger.Error("get current bau cua round: %v", err)
		return "", runtime.NewError("failed to load bau cua state", 13)
	}
	round, bets, err := resolveBauCuaRoundIfReady(ctx, logger, nk, round, roundVersion)
	if err != nil {
		return "", err
	}

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

	out := buildBauCuaStateResponse(round, bets, userID, buildBauCuaBetItems(inv, definitions))
	raw, err := json.Marshal(out)
	if err != nil {
		logger.Error("marshal bau cua state: %v", err)
		return "", runtime.NewError("failed to load bau cua state", 13)
	}
	return string(raw), nil
}

func BauCuaPlaceBetRPC(
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

	var body bauCuaPlaceBetPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}
	body.RoundID = strings.TrimSpace(body.RoundID)
	body.SlotID = strings.TrimSpace(body.SlotID)
	body.SymbolID = strings.TrimSpace(body.SymbolID)
	body.ItemID = strings.TrimSpace(body.ItemID)
	if body.RoundID == "" || body.SlotID == "" || body.SymbolID == "" || body.ItemID == "" {
		return "", runtime.NewError("roundId, slotId, symbolId and itemId are required", 3)
	}
	if body.Quantity < 1 {
		return "", runtime.NewError("quantity must be positive", 3)
	}

	roomID := normalizeBauCuaRoomID(roomIDFromBauCuaRoundID(body.RoundID))
	if err := resolvePreviousBauCuaRound(ctx, logger, nk, roomID); err != nil {
		return "", err
	}

	round, roundVersion, err := readBauCuaRound(ctx, nk, roomID, body.RoundID)
	if err != nil {
		logger.Error("read bau cua round: %v", err)
		return "", runtime.NewError("failed to place bet", 13)
	}
	if round.RoundID == "" {
		return "", runtime.NewError("round not found", 5)
	}
	round, _, err = resolveBauCuaRoundIfReady(ctx, logger, nk, round, roundVersion)
	if err != nil {
		return "", err
	}
	if currentBauCuaPhase(round, nowUnixSeconds()) != bauCuaPhaseBettingOpen {
		return "", runtime.NewError("betting is closed", 9)
	}
	if !isValidBauCuaSlot(body.SlotID, body.SymbolID) {
		return "", runtime.NewError("slot is invalid", 3)
	}

	inv, invVersion, err := readPlayerInventory(ctx, nk, userID)
	if err != nil {
		logger.Error("read inventory before bau cua bet: %v", err)
		return "", runtime.NewError("failed to place bet", 13)
	}
	itemType, err := consumeBauCuaBetItem(&inv, body.ItemID, body.Quantity)
	if err != nil {
		return "", err
	}

	userBet, userBetVersion, err := readBauCuaUserBet(ctx, nk, round.RoundID, userID)
	if err != nil {
		logger.Error("read user bet before bau cua bet: %v", err)
		return "", runtime.NewError("failed to place bet", 13)
	}
	addBauCuaUserBet(&userBet, body.SlotID, body.SymbolID, body.ItemID, body.Quantity, itemType)

	if err := writeBauCuaUserBetAndInventory(ctx, nk, userID, invVersion, inv, userBetVersion, userBet); err != nil {
		logger.Error("write inventory and user bet after bau cua bet: %v", err)
		return "", runtime.NewError("failed to save bet (retry)", 13)
	}

	definitions, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		logger.Warn("read shop catalog for bau cua display names: %v", err)
		definitions = nil
	}
	bets, err := readAllBauCuaBets(ctx, nk, round.RoundID)
	if err != nil {
		logger.Error("read all bets after bau cua bet: %v", err)
		return "", runtime.NewError("failed to place bet", 13)
	}
	response := bauCuaPlaceBetResponse{
		State:     buildBauCuaStateResponse(round, bets, userID, buildBauCuaBetItems(inv, definitions)),
		Inventory: normalizePlayerInventory(inv),
	}

	raw, err := json.Marshal(response)
	if err != nil {
		logger.Error("marshal bau cua place bet: %v", err)
		return "", runtime.NewError("failed to place bet", 13)
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
	symbols := []string{"bau", "cua", "tom", "ca", "ga", "cop"}
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

func buildBauCuaStateResponse(round bauCuaRoundRecord, bets bauCuaRoundBetsRecord, userID string, betItems []bauCuaInventoryItemDTO) bauCuaRoundStateResponse {
	now := nowUnixSeconds()
	out := bauCuaRoundStateResponse{
		RoomID:        round.RoomID,
		RoundID:       round.RoundID,
		Phase:         currentBauCuaPhase(round, now),
		ServerTime:    now,
		BettingEndsAt: round.BettingEndsAt,
		Slots:         buildBauCuaSlotsWithBets(bets, userID),
		BetItems:      betItems,
	}
	if len(round.Result) > 0 {
		out.Result = &bauCuaResultDTO{SymbolIDs: append([]string(nil), round.Result...)}
	}
	return out
}

func buildBauCuaSlotsWithBets(bets bauCuaRoundBetsRecord, userID string) []bauCuaSlotStateDTO {
	slots := defaultBauCuaSlots()
	playerCountBySlot := make(map[string]int)
	myBetsBySlot := make(map[string][]bauCuaBetStackDTO)

	for _, playerBet := range bets.PlayerBets {
		for _, slotBet := range playerBet.Slots {
			if len(slotBet.Items) == 0 {
				continue
			}
			playerCountBySlot[slotBet.SlotID]++
			if playerBet.UserID == userID {
				myBetsBySlot[slotBet.SlotID] = append([]bauCuaBetStackDTO(nil), slotBet.Items...)
			}
		}
	}

	for i := range slots {
		slots[i].PlayerCount = playerCountBySlot[slots[i].SlotID]
		if myBets, ok := myBetsBySlot[slots[i].SlotID]; ok {
			slots[i].MyBets = myBets
		}
	}
	return slots
}

func currentBauCuaPhase(round bauCuaRoundRecord, now int64) string {
	if len(round.Result) > 0 || round.Resolved {
		return bauCuaPhaseShowingResult
	}
	if now >= round.BettingEndsAt {
		return bauCuaPhaseWaitingResult
	}
	return bauCuaPhaseBettingOpen
}

func getOrCreateCurrentBauCuaRound(ctx context.Context, nk runtime.NakamaModule, roomID string) (bauCuaRoundRecord, string, error) {
	now := nowUnixSeconds()
	roundStart := now - (now % bauCuaRoundSeconds)
	roundID := buildBauCuaRoundID(roomID, roundStart)
	round, version, err := readBauCuaRound(ctx, nk, roomID, roundID)
	if err != nil {
		return bauCuaRoundRecord{}, "", err
	}
	if round.RoundID != "" {
		return round, version, nil
	}

	round = bauCuaRoundRecord{
		RoomID:        roomID,
		RoundID:       roundID,
		StartedAt:     roundStart,
		BettingEndsAt: roundStart + bauCuaRoundSeconds - bauCuaResultLockSeconds,
		EndsAt:        roundStart + bauCuaRoundSeconds,
		Result:        []string{},
		Resolved:      false,
	}
	if err := writeBauCuaRound(ctx, nk, "*", round); err != nil {
		existing, existingVersion, readErr := readBauCuaRound(ctx, nk, roomID, roundID)
		if readErr != nil {
			return bauCuaRoundRecord{}, "", readErr
		}
		if existing.RoundID != "" {
			return existing, existingVersion, nil
		}
		return bauCuaRoundRecord{}, "", err
	}
	created, version, err := readBauCuaRound(ctx, nk, roomID, roundID)
	if err != nil || created.RoundID == "" {
		return round, "", err
	}
	return created, version, nil
}

func resolvePreviousBauCuaRound(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, roomID string) error {
	now := nowUnixSeconds()
	cursor := ""
	prefix := normalizeBauCuaRoomID(roomID) + ":"

	for {
		objects, nextCursor, err := nk.StorageList(ctx, "", "", bauCuaStorageCollection, 100, cursor)
		if err != nil {
			logger.Error("list pending bau cua rounds: %v", err)
			return runtime.NewError("failed to resolve previous round", 13)
		}

		for _, obj := range objects {
			if !strings.HasPrefix(obj.GetKey(), prefix) {
				continue
			}

			var round bauCuaRoundRecord
			if err := json.Unmarshal([]byte(obj.GetValue()), &round); err != nil {
				logger.Error("decode pending bau cua round %s: %v", obj.GetKey(), err)
				return runtime.NewError("failed to resolve previous round", 13)
			}
			if round.RoundID == "" || round.Resolved || now < round.BettingEndsAt {
				continue
			}

			if _, _, err := resolveBauCuaRoundIfReady(ctx, logger, nk, round, obj.GetVersion()); err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == "" {
			break
		}
	}

	return nil
}

func resolveBauCuaRoundIfReady(
	ctx context.Context,
	logger runtime.Logger,
	nk runtime.NakamaModule,
	round bauCuaRoundRecord,
	roundVersion string,
) (bauCuaRoundRecord, bauCuaRoundBetsRecord, error) {
	bets, err := readAllBauCuaBets(ctx, nk, round.RoundID)
	if err != nil {
		logger.Error("read bau cua bets for resolve: %v", err)
		return round, bauCuaRoundBetsRecord{}, runtime.NewError("failed to resolve round", 13)
	}
	if round.Resolved || nowUnixSeconds() < round.BettingEndsAt {
		return round, bets, nil
	}

	if len(round.Result) == 0 {
		result, err := randomBauCuaResult()
		if err != nil {
			logger.Error("random bau cua result: %v", err)
			return round, bets, runtime.NewError("failed to resolve round", 13)
		}
		round.Result = result
	}

	if err := payoutBauCuaRound(ctx, logger, nk, round, bets); err != nil {
		return round, bets, err
	}

	round.Resolved = true
	if err := writeBauCuaRound(ctx, nk, roundVersion, round); err != nil {
		logger.Error("write resolved bau cua round: %v", err)
		return round, bets, runtime.NewError("failed to resolve round", 13)
	}
	bets, _ = readAllBauCuaBets(ctx, nk, round.RoundID)
	return round, bets, nil
}

func payoutBauCuaRound(ctx context.Context, logger runtime.Logger, nk runtime.NakamaModule, round bauCuaRoundRecord, bets bauCuaRoundBetsRecord) error {
	resultCounts := make(map[string]int)
	for _, symbolID := range round.Result {
		resultCounts[symbolID]++
	}

	for _, playerBet := range bets.PlayerBets {
		userBet, userBetVersion, err := readBauCuaUserBet(ctx, nk, round.RoundID, playerBet.UserID)
		if err != nil {
			logger.Error("read bau cua user bet for payout user %s: %v", playerBet.UserID, err)
			return runtime.NewError("failed to payout round", 13)
		}
		if userBet.Paid {
			continue
		}

		inv, invVersion, err := readPlayerInventory(ctx, nk, playerBet.UserID)
		if err != nil {
			logger.Error("read inventory for bau cua payout user %s: %v", playerBet.UserID, err)
			return runtime.NewError("failed to payout round", 13)
		}

		changed := false
		for _, slotBet := range playerBet.Slots {
			hitCount := resultCounts[slotBet.SymbolID]
			if hitCount < 1 {
				continue
			}

			for _, item := range slotBet.Items {
				rewardQuantity := item.Quantity * (hitCount + 1)
				if rewardQuantity < 1 {
					continue
				}
				if err := addBauCuaRewardItem(&inv, item.ItemID, item.ItemType, rewardQuantity); err != nil {
					return err
				}
				changed = true
			}
		}

		userBet.Paid = true
		if err := writeBauCuaPayout(ctx, nk, playerBet.UserID, invVersion, inv, userBetVersion, userBet, changed); err != nil {
			logger.Error("write payout for bau cua user %s: %v", playerBet.UserID, err)
			return runtime.NewError("failed to payout round", 13)
		}
	}
	return nil
}

func readBauCuaRound(ctx context.Context, nk runtime.NakamaModule, roomID string, roundID string) (bauCuaRoundRecord, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: bauCuaStorageCollection,
			Key:        buildBauCuaRoundStorageKey(roomID, roundID),
			UserID:     "",
		},
	})
	if err != nil {
		return bauCuaRoundRecord{}, "", err
	}
	if len(objs) == 0 {
		return bauCuaRoundRecord{}, "", nil
	}

	var round bauCuaRoundRecord
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &round); err != nil {
		return bauCuaRoundRecord{}, "", err
	}
	if round.Result == nil {
		round.Result = []string{}
	}
	return round, objs[0].GetVersion(), nil
}

func writeBauCuaRound(ctx context.Context, nk runtime.NakamaModule, version string, round bauCuaRoundRecord) error {
	raw, err := json.Marshal(round)
	if err != nil {
		return err
	}
	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      bauCuaStorageCollection,
			Key:             buildBauCuaRoundStorageKey(round.RoomID, round.RoundID),
			UserID:          "",
			Value:           string(raw),
			Version:         version,
			PermissionRead:  0,
			PermissionWrite: 0,
		},
	})
	return err
}

func readBauCuaUserBet(ctx context.Context, nk runtime.NakamaModule, roundID string, userID string) (bauCuaUserBetRecord, string, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: bauCuaBetsCollection,
			Key:        buildBauCuaUserBetStorageKey(roundID, userID),
			UserID:     "",
		},
	})
	if err != nil {
		return bauCuaUserBetRecord{}, "", err
	}
	if len(objs) == 0 {
		return bauCuaUserBetRecord{
			RoundID: roundID,
			UserID:  userID,
			Slots:   []bauCuaSlotBet{},
			Paid:    false,
		}, "", nil
	}

	var bet bauCuaUserBetRecord
	if err := json.Unmarshal([]byte(objs[0].GetValue()), &bet); err != nil {
		return bauCuaUserBetRecord{}, "", err
	}
	if bet.Slots == nil {
		bet.Slots = []bauCuaSlotBet{}
	}
	bet.RoundID = roundID
	bet.UserID = userID
	bet.version = objs[0].GetVersion()
	return bet, objs[0].GetVersion(), nil
}

func readAllBauCuaBets(ctx context.Context, nk runtime.NakamaModule, roundID string) (bauCuaRoundBetsRecord, error) {
	out := bauCuaRoundBetsRecord{RoundID: roundID, PlayerBets: []bauCuaPlayerBet{}}
	cursor := ""
	prefix := strings.TrimSpace(roundID) + ":"

	for {
		objects, nextCursor, err := nk.StorageList(ctx, "", "", bauCuaBetsCollection, 100, cursor)
		if err != nil {
			return bauCuaRoundBetsRecord{}, err
		}

		for _, obj := range objects {
			if !strings.HasPrefix(obj.GetKey(), prefix) {
				continue
			}

			var bet bauCuaUserBetRecord
			if err := json.Unmarshal([]byte(obj.GetValue()), &bet); err != nil {
				return bauCuaRoundBetsRecord{}, err
			}
			if bet.RoundID != roundID || bet.UserID == "" || len(bet.Slots) == 0 {
				continue
			}
			out.PlayerBets = append(out.PlayerBets, bauCuaPlayerBet{
				UserID: bet.UserID,
				Slots:  bet.Slots,
			})
		}

		cursor = nextCursor
		if cursor == "" {
			break
		}
	}

	sort.Slice(out.PlayerBets, func(i, j int) bool {
		return out.PlayerBets[i].UserID < out.PlayerBets[j].UserID
	})
	return out, nil
}

func writeBauCuaUserBetAndInventory(
	ctx context.Context,
	nk runtime.NakamaModule,
	userID string,
	invVersion string,
	inv PlayerInventory,
	userBetVersion string,
	userBet bauCuaUserBetRecord,
) error {
	invRaw, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	betRaw, err := json.Marshal(userBet)
	if err != nil {
		return err
	}
	if userBetVersion == "" {
		userBetVersion = "*"
	}

	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVersion,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
		{
			Collection:      bauCuaBetsCollection,
			Key:             buildBauCuaUserBetStorageKey(userBet.RoundID, userBet.UserID),
			UserID:          "",
			Value:           string(betRaw),
			Version:         userBetVersion,
			PermissionRead:  0,
			PermissionWrite: 0,
		},
	})
	return err
}

func writeBauCuaPayout(
	ctx context.Context,
	nk runtime.NakamaModule,
	userID string,
	invVersion string,
	inv PlayerInventory,
	userBetVersion string,
	userBet bauCuaUserBetRecord,
	writeInventory bool,
) error {
	betRaw, err := json.Marshal(userBet)
	if err != nil {
		return err
	}
	writes := []*runtime.StorageWrite{
		{
			Collection:      bauCuaBetsCollection,
			Key:             buildBauCuaUserBetStorageKey(userBet.RoundID, userBet.UserID),
			UserID:          "",
			Value:           string(betRaw),
			Version:         userBetVersion,
			PermissionRead:  0,
			PermissionWrite: 0,
		},
	}
	if writeInventory {
		invRaw, err := json.Marshal(inv)
		if err != nil {
			return err
		}
		writes = append(writes, &runtime.StorageWrite{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVersion,
			PermissionRead:  1,
			PermissionWrite: 0,
		})
	}
	_, err = nk.StorageWrite(ctx, writes)
	return err
}

func addBauCuaUserBet(bet *bauCuaUserBetRecord, slotID string, symbolID string, itemID string, quantity int, itemType string) {
	slotIdx := -1
	for i := range bet.Slots {
		if bet.Slots[i].SlotID == slotID {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		bet.Slots = append(bet.Slots, bauCuaSlotBet{
			SlotID:   slotID,
			SymbolID: symbolID,
			Items:    []bauCuaBetStackDTO{},
		})
		slotIdx = len(bet.Slots) - 1
	}

	items := bet.Slots[slotIdx].Items
	for i := range items {
		if items[i].ItemID == itemID && items[i].ItemType == itemType {
			items[i].Quantity += quantity
			bet.Slots[slotIdx].Items = items
			return
		}
	}
	bet.Slots[slotIdx].Items = append(items, bauCuaBetStackDTO{
		ItemID:   itemID,
		Quantity: quantity,
		ItemType: itemType,
	})
}

func consumeBauCuaBetItem(inv *PlayerInventory, itemID string, quantity int) (string, error) {
	if consumeFromBauCuaStacks(&inv.Items, itemID, quantity) {
		return bauCuaItemTypeItem, nil
	}
	return "", runtime.NewError("not enough item in inventory", 9)
}

func consumeFromBauCuaStacks(stacks *[]InventoryItemStack, itemID string, quantity int) bool {
	for i := range *stacks {
		if (*stacks)[i].ItemID != itemID || (*stacks)[i].Quantity < quantity {
			continue
		}
		(*stacks)[i].Quantity -= quantity
		if (*stacks)[i].Quantity == 0 {
			*stacks = append((*stacks)[:i], (*stacks)[i+1:]...)
		}
		return true
	}
	return false
}

func addBauCuaRewardItem(inv *PlayerInventory, itemID string, itemType string, quantity int) error {
	switch itemType {
	case bauCuaItemTypePot:
		return AddPot(inv, itemID, quantity)
	case bauCuaItemTypeSeed:
		return AddSeed(inv, itemID, quantity)
	default:
		return AddItem(inv, itemID, quantity)
	}
}

func randomBauCuaResult() ([]string, error) {
	symbols := []string{"bau", "cua", "tom", "ca", "ga", "cop"}
	out := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(symbols))))
		if err != nil {
			return nil, err
		}
		out = append(out, symbols[n.Int64()])
	}
	return out, nil
}

func isValidBauCuaSlot(slotID string, symbolID string) bool {
	for _, slot := range defaultBauCuaSlots() {
		if slot.SlotID == slotID && slot.SymbolID == symbolID {
			return true
		}
	}
	return false
}

func buildBauCuaRoundID(roomID string, roundStart int64) string {
	return fmt.Sprintf("%s_%d", normalizeBauCuaRoomID(roomID), roundStart)
}

func roomIDFromBauCuaRoundID(roundID string) string {
	idx := strings.LastIndex(roundID, "_")
	if idx <= 0 {
		return bauCuaDefaultRoomID
	}
	return roundID[:idx]
}

func buildBauCuaRoundStorageKey(roomID string, roundID string) string {
	return fmt.Sprintf("%s:%s", normalizeBauCuaRoomID(roomID), strings.TrimSpace(roundID))
}

func buildBauCuaUserBetStorageKey(roundID string, userID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(roundID), strings.TrimSpace(userID))
}

func buildBauCuaBetItems(inv PlayerInventory, definitions []ShopItemDefinition) []bauCuaInventoryItemDTO {
	displayNames := buildBauCuaDisplayNameByItemID(definitions)
	byID := make(map[string]int)

	addStacks := func(stacks []InventoryItemStack) {
		for _, stack := range stacks {
			itemID := strings.TrimSpace(stack.ItemID)
			if itemID == "" || stack.Quantity <= 0 {
				continue
			}
			byID[itemID] += stack.Quantity
		}
	}

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
