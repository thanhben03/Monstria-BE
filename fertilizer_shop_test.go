package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/heroiclabs/nakama-common/api"
	"github.com/heroiclabs/nakama-common/runtime"
)

type fertilizerShopStore struct {
	runtime.NakamaModule
	level   int
	writes  []*runtime.StorageWrite
	wallets []*runtime.WalletUpdate
}

func (s *fertilizerShopStore) StorageRead(_ context.Context, reads []*runtime.StorageRead) ([]*api.StorageObject, error) {
	if reads[0].Key == playerStateKey {
		return []*api.StorageObject{{Key: playerStateKey, Value: fmt.Sprintf(`{"level":%d,"walletMigrated":true}`, s.level), Version: "resources-v1"}}, nil
	}
	return []*api.StorageObject{{Key: playerInventoryKey, Value: `{"items":[{"itemId":"fertilizer_30","quantity":2}]}`, Version: "inventory-v1"}}, nil
}

func (*fertilizerShopStore) AccountGetId(context.Context, string) (*api.Account, error) {
	return &api.Account{Wallet: `{"coin":10000,"gem":100}`}, nil
}

func (s *fertilizerShopStore) MultiUpdate(_ context.Context, _ []*runtime.AccountUpdate, writes []*runtime.StorageWrite, _ []*runtime.StorageDelete, wallets []*runtime.WalletUpdate, _ bool) ([]*api.StorageObjectAck, []*runtime.WalletUpdateResult, error) {
	s.writes, s.wallets = writes, wallets
	return nil, []*runtime.WalletUpdateResult{{Updated: map[string]int64{"coin": 10000 + wallets[0].Changeset["coin"], "gem": 100 + wallets[0].Changeset["gem"]}}}, nil
}

func TestFertilizerShopPurchaseRPC(t *testing.T) {
	old := listShopItemDefinitions()
	t.Cleanup(func() { setShopItemDefinitions(old) })
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	for _, tc := range []struct {
		name, currency   string
		level, quantity  int
		disabled, reject bool
	}{
		{"coin quantity", "coin", 5, 3, false, false},
		{"gem quantity", "gem", 5, 3, false, false},
		{"low level", "coin", 4, 3, false, true},
		{"disabled", "coin", 5, 3, true, true},
		{"invalid currency", "energy", 5, 3, false, true},
		{"negative quantity", "coin", 5, -1, false, true},
		{"too many", "coin", 5, 100, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setShopItemDefinitions([]ShopItemDefinition{{ShopItemID: "fertilizer_30", GrantType: shopGrantTypeFertilizer, GrantItemID: "fertilizer_30", Quantity: 1, CoinPrice: 250, GemPrice: 3, RequiredLevel: 5, Disabled: tc.disabled}})
			store := &fertilizerShopStore{level: tc.level}
			payload := fmt.Sprintf(`{"shopItemId":"fertilizer_30","currency":%q,"quantity":%d}`, tc.currency, tc.quantity)
			raw, err := PurchaseShopItemRPC(ctx, fertilizerLogger{}, nil, store, payload)
			if tc.reject {
				if err == nil || len(store.writes) != 0 || len(store.wallets) != 0 {
					t.Fatalf("invalid purchase committed: %s, %v", raw, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var result purchaseShopItemResponse
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			unit := 250
			if tc.currency == "gem" {
				unit = 3
			}
			if result.Purchase.Quantity != 3 || result.Purchase.Price != unit*3 || len(result.Inventory.Items) != 1 || result.Inventory.Items[0].Quantity != 5 {
				t.Fatalf("wrong purchase: %s", raw)
			}
			if len(store.writes) != 1 || store.writes[0].Version != "inventory-v1" || store.wallets[0].Changeset[tc.currency] != -int64(unit*3) {
				t.Fatal("missing atomic inventory/wallet update")
			}
		})
	}
}

func TestFertilizerShopCatalogAndDetail(t *testing.T) {
	raw, err := os.ReadFile("shop_catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	defs, err := parseShopItemDefinitions(raw)
	if err != nil {
		t.Fatal(err)
	}
	fertilizers := listShopItemDefinitionsForCategoryFrom(defs, "fertilizer")
	if len(fertilizers) != 3 {
		t.Fatalf("got %d fertilizers", len(fertilizers))
	}
	for _, def := range fertilizers {
		if _, ok := fertilizerRates[def.GrantItemID]; !ok {
			t.Fatalf("unknown fertilizer %s", def.GrantItemID)
		}
		if !def.Disabled {
			t.Fatal("sample must remain disabled until configured")
		}
		detail := shopItemDefinitionWithDetailText(def)
		if detail.DetailMainText == "" || !strings.Contains(detail.DetailSideText, fmt.Sprintf("Cấp độ yêu cầu: %d", def.RequiredLevel)) {
			t.Fatalf("missing detail: %+v", detail)
		}
	}
	old := listShopItemDefinitions()
	t.Cleanup(func() { setShopItemDefinitions(old) })
	// Only enabled offers count towards pagination, including for low-level players.
	fertilizers[1].Disabled = false
	setShopItemDefinitions(fertilizers)
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "player")
	response, err := GetShopCatalogRPC(ctx, fertilizerLogger{}, nil, &fertilizerShopStore{level: 1}, `{"category":"fertilizer","page":1,"pageSize":12}`)
	if err != nil {
		t.Fatal(err)
	}
	var catalog shopCatalogResponse
	if err := json.Unmarshal([]byte(response), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Pagination.Total != 1 || len(catalog.Items["fertilizer"]) != 1 || catalog.Items["fertilizer"][0].RequiredLevel != 5 {
		t.Fatalf("bad catalog: %s", response)
	}
}
