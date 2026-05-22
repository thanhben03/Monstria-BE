package main

import "testing"

func TestParseShopItemDefinitions(t *testing.T) {
	raw := []byte(`{
		"items": {
			"seed": [
				{
					"shopItemId": "seed_rose_pack",
					"grantType": "seed",
					"grantItemId": "seed_rose",
					"quantity": 5,
					"coinPrice": 100,
					"gemPrice": 2,
					"requiredLevel": 2
				}
			]
		}
	}`)

	defs, err := parseShopItemDefinitions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 {
		t.Fatalf("got %#v", defs)
	}
	def := defs[0]
	if def.GrantType != shopGrantTypeSeed || def.Quantity != 5 || def.RequiredLevel != 2 {
		t.Fatalf("unexpected def %#v", def)
	}
	if def.CoinPrice != 100 || def.GemPrice != 2 {
		t.Fatalf("unexpected prices %#v", def)
	}
}

func TestParseShopItemDefinitionsSupportsPotPriceByLevel(t *testing.T) {
	raw := []byte(`{
		"items": {
			"pot": [
				{
					"shopItemId": "pot_level",
					"grantType": "pot",
					"grantItemId": "pot_level",
					"quantity": 1,
					"summary": "Summary",
					"desc": "Description",
					"priceByLevel": [
						{"minLevel": 11, "maxLevel": 20, "coinPrice": 240},
						{"minLevel": 1, "maxLevel": 10, "coinPrice": 120}
					]
				}
			]
		}
	}`)

	defs, err := parseShopItemDefinitions(raw)
	if err != nil {
		t.Fatal(err)
	}
	def := defs[0]
	if def.Summary != "Summary" || def.Desc != "Description" {
		t.Fatalf("unexpected text fields %#v", def)
	}
	if len(def.PriceByLevel) != 2 || def.PriceByLevel[0].MinLevel != 1 || def.PriceByLevel[1].MinLevel != 11 {
		t.Fatalf("unexpected price levels %#v", def.PriceByLevel)
	}
}

func TestShopItemDefinitionForIDReturnsMetadata(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{
			ShopItemID:      "flower_rose",
			GrantType:       shopGrantTypeItem,
			GrantItemID:     "flower_rose",
			Quantity:        1,
			CoinPrice:       500,
			GemPrice:        5,
			GrowthTime:      "14:00:00",
			HarvestQuantity: 1,
			WaterNeed:       "ít",
			ExpReward:       10,
			AttractedBugs:   []string{"bọ rùa", "ong"},
			Desc:            "Hoa hồng test",
			GoldPerHour:     5,
		},
	})

	got, err := shopItemDefinitionForID(" flower_rose ")
	if err != nil {
		t.Fatal(err)
	}
	if got.GrowthTime != "14:00:00" || got.HarvestQuantity != 1 || got.WaterNeed != "ít" {
		t.Fatalf("unexpected metadata %#v", got)
	}
	if got.CoinPrice != 500 || got.GemPrice != 5 || got.GoldPerHour != 5 {
		t.Fatalf("unexpected prices/reward %#v", got)
	}
	if len(got.AttractedBugs) != 2 || got.AttractedBugs[0] != "bọ rùa" || got.AttractedBugs[1] != "ong" {
		t.Fatalf("unexpected bugs %#v", got.AttractedBugs)
	}
}

func TestShopItemDefinitionWithDetailTextForSeed(t *testing.T) {
	def := shopItemDefinitionWithDetailText(ShopItemDefinition{
		GrantType:       shopGrantTypeSeed,
		GrowthTime:      "14:00:00",
		RequiredLevel:   2,
		HarvestQuantity: 1,
		WaterNeed:       "Ít",
		ExpReward:       10,
		AttractedBugs:   []string{"Bọ rùa", "ong"},
		Desc:            "Hoa này dễ trồng.",
	})

	wantMain := "Tăng trưởng: <color=#00FF00>14:00:00</color>\nCấp độ: <color=#FF66FF>2</color>\nTổng thu hoạch: <color=#FFFF00>1</color>"
	wantSide := "Cần nước: <color=#66CCFF>Ít</color>\nKinh nghiệm: <color=#FFFF00>10</color>\nThu hút sâu: <color=#FF3333>Bọ rùa, ong</color>\n\n<color=#FF33CC>Hoa này dễ trồng.</color>"
	if def.DetailMainText != wantMain {
		t.Fatalf("main text:\n%s", def.DetailMainText)
	}
	if def.DetailSideText != wantSide {
		t.Fatalf("side text:\n%s", def.DetailSideText)
	}
}

func TestShopItemDefinitionWithDetailTextForPot(t *testing.T) {
	def := shopItemDefinitionWithDetailText(ShopItemDefinition{
		GrantType: shopGrantTypePot,
		Summary:   "Chậu đất nung đơn giản.",
		Desc:      "Mô tả chậu.",
		PriceByLevel: []ShopPriceLevelDefinition{
			{MinLevel: 1, MaxLevel: 10, CoinPrice: 120},
			{MinLevel: 11, MaxLevel: 20, CoinPrice: 240, GemPrice: 2},
		},
	})

	wantMain := "Chậu đất nung đơn giản."
	wantSide := "Mô tả chậu.\n\nGiá mua chậu theo cấp độ:\n1 - 10: <color=#FFFF00>120 / 0</color>\n11 - 20: <color=#FFFF00>240 / 2</color>"
	if def.DetailMainText != wantMain {
		t.Fatalf("main text:\n%s", def.DetailMainText)
	}
	if def.DetailSideText != wantSide {
		t.Fatalf("side text:\n%s", def.DetailSideText)
	}
}

func TestShopItemDefinitionForPlayerLevelUsesPotPriceByLevel(t *testing.T) {
	def := shopItemDefinitionForPlayerLevel(ShopItemDefinition{
		GrantType: shopGrantTypePot,
		CoinPrice: 120,
		PriceByLevel: []ShopPriceLevelDefinition{
			{MinLevel: 1, MaxLevel: 10, CoinPrice: 120},
			{MinLevel: 11, MaxLevel: 20, CoinPrice: 240, GemPrice: 3},
		},
	}, 12)

	if def.CoinPrice != 240 || def.GemPrice != 3 {
		t.Fatalf("unexpected price %#v", def)
	}
}

func TestShopItemDefinitionForIDRejectsDisabledItem(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{
			ShopItemID:  "disabled",
			GrantType:   shopGrantTypePot,
			GrantItemID: "pot_01",
			Quantity:    1,
			CoinPrice:   1,
			Disabled:    true,
		},
	})

	if _, err := shopItemDefinitionForID("disabled"); err == nil {
		t.Fatal("expected disabled shop item to be hidden")
	}
}

func TestParseShopItemDefinitionsSupportsLegacyArray(t *testing.T) {
	raw := []byte(`{
		"items": [
			{
				"shopItemId": "pot_wood",
				"grantType": "pot",
				"grantItemId": "pot_wood",
				"quantity": 1,
				"coinPrice": 100
			}
		]
	}`)

	defs, err := parseShopItemDefinitions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].ShopItemID != "pot_wood" {
		t.Fatalf("got %#v", defs)
	}
}

func TestParseShopItemDefinitionsRejectsInvalid(t *testing.T) {
	raw := []byte(`{"items":[{"shopItemId":"bad","grantType":"seed","grantItemId":"seed_rose","quantity":0,"coinPrice":1}]}`)
	if _, err := parseShopItemDefinitions(raw); err == nil {
		t.Fatal("expected quantity validation error")
	}
}

func TestListShopItemDefinitionsSortedCopy(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{ShopItemID: "b", GrantType: shopGrantTypePot, GrantItemID: "pot_b", Quantity: 1, CoinPrice: 1},
		{ShopItemID: "a", GrantType: shopGrantTypePot, GrantItemID: "pot_a", Quantity: 1, CoinPrice: 1},
	})

	got := listShopItemDefinitions()
	if got[0].ShopItemID != "a" || got[1].ShopItemID != "b" {
		t.Fatalf("not sorted: %#v", got)
	}
	got[0].ShopItemID = "changed"
	again := listShopItemDefinitions()
	if again[0].ShopItemID == "changed" {
		t.Fatal("listShopItemDefinitions returned mutable backing data")
	}
}

func TestListShopItemDefinitionsByCategory(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{ShopItemID: "pot_wood", GrantType: shopGrantTypePot, GrantItemID: "pot_wood", Quantity: 1, CoinPrice: 1},
		{ShopItemID: "seed_rose_pack", GrantType: shopGrantTypeSeed, GrantItemID: "seed_rose", Quantity: 5, CoinPrice: 1},
		{ShopItemID: "flower_rose", GrantType: shopGrantTypeItem, GrantItemID: "flower_rose", Quantity: 1, GemPrice: 1},
	})

	got := listShopItemDefinitionsByCategory()
	if len(got["pot"]) != 1 || got["pot"][0].ShopItemID != "pot_wood" {
		t.Fatalf("unexpected pot category %#v", got["pot"])
	}
	if len(got["seed"]) != 1 || got["seed"][0].ShopItemID != "seed_rose_pack" {
		t.Fatalf("unexpected seed category %#v", got["seed"])
	}
	if len(got["flower"]) != 1 || got["flower"][0].ShopItemID != "flower_rose" {
		t.Fatalf("unexpected flower category %#v", got["flower"])
	}
}

func TestPaginateShopItemDefinitionsDefaultsToTwelveItems(t *testing.T) {
	defs := make([]ShopItemDefinition, 13)
	for i := range defs {
		defs[i] = ShopItemDefinition{
			ShopItemID:  "item",
			GrantType:   shopGrantTypeItem,
			GrantItemID: "flower_rose",
			Quantity:    1,
			CoinPrice:   1,
		}
	}

	pageItems, pagination := paginateShopItemDefinitions(defs, 1, 0)
	if len(pageItems) != 12 {
		t.Fatalf("got %d items", len(pageItems))
	}
	if pagination.Page != 1 || pagination.PageSize != 12 || pagination.Total != 13 || pagination.TotalPages != 2 || !pagination.HasNext {
		t.Fatalf("unexpected pagination %#v", pagination)
	}
}

func TestPaginateShopItemDefinitionsCapsPageSizeAtTwelve(t *testing.T) {
	defs := make([]ShopItemDefinition, 20)
	pageItems, pagination := paginateShopItemDefinitions(defs, 1, 99)
	if len(pageItems) != 12 {
		t.Fatalf("got %d items", len(pageItems))
	}
	if pagination.PageSize != 12 {
		t.Fatalf("got page size %d", pagination.PageSize)
	}
}

func TestListShopItemDefinitionsForCategory(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{ShopItemID: "pot_wood", GrantType: shopGrantTypePot, GrantItemID: "pot_wood", Quantity: 1, CoinPrice: 1},
		{ShopItemID: "flower_rose", GrantType: shopGrantTypeItem, GrantItemID: "flower_rose", Quantity: 1, GemPrice: 1},
	})

	got := listShopItemDefinitionsForCategory("pot")
	if len(got) != 1 || got[0].ShopItemID != "pot_wood" {
		t.Fatalf("unexpected category result %#v", got)
	}
}

func TestResolvePurchaseCurrency(t *testing.T) {
	def := ShopItemDefinition{CoinPrice: 10, GemPrice: 1}
	if _, err := resolvePurchaseCurrency(def, ""); err == nil {
		t.Fatal("expected currency required for multi-currency item")
	}
	got, err := resolvePurchaseCurrency(def, " GEM ")
	if err != nil {
		t.Fatal(err)
	}
	if got != shopCurrencyGem {
		t.Fatalf("got %q", got)
	}

	def = ShopItemDefinition{CoinPrice: 10}
	got, err = resolvePurchaseCurrency(def, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != shopCurrencyCoin {
		t.Fatalf("got %q", got)
	}
	if _, err := resolvePurchaseCurrency(def, shopCurrencyGem); err == nil {
		t.Fatal("expected gem to be disabled when gemPrice is 0")
	}
}

func TestResolvePurchaseQuantity(t *testing.T) {
	got, err := resolvePurchaseQuantity(0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("got %d", got)
	}
	got, err = resolvePurchaseQuantity(3)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("got %d", got)
	}
	if _, err := resolvePurchaseQuantity(-1); err == nil {
		t.Fatal("expected negative quantity to be rejected")
	}
	if _, err := resolvePurchaseQuantity(100); err == nil {
		t.Fatal("expected large quantity to be rejected")
	}
}

func TestGrantShopItem(t *testing.T) {
	inv := defaultPlayerInventory()

	if err := grantShopItem(&inv, ShopItemDefinition{GrantType: shopGrantTypePot, GrantItemID: "pot_wood", Quantity: 2}); err != nil {
		t.Fatal(err)
	}
	if err := grantShopItem(&inv, ShopItemDefinition{GrantType: shopGrantTypeSeed, GrantItemID: "seed_rose", Quantity: 5}); err != nil {
		t.Fatal(err)
	}
	if err := grantShopItem(&inv, ShopItemDefinition{GrantType: shopGrantTypeItem, GrantItemID: "flower_rose", Quantity: 1}); err != nil {
		t.Fatal(err)
	}

	if len(inv.Pots) != 1 || inv.Pots[0].Quantity != 2 {
		t.Fatalf("unexpected pots %#v", inv.Pots)
	}
	if len(inv.Seeds) != 1 || inv.Seeds[0].Quantity != 5 {
		t.Fatalf("unexpected seeds %#v", inv.Seeds)
	}
	if len(inv.Items) != 1 || inv.Items[0].Quantity != 1 {
		t.Fatalf("unexpected items %#v", inv.Items)
	}
}

func TestGrantShopItemQuantity(t *testing.T) {
	inv := defaultPlayerInventory()

	if err := grantShopItemQuantity(&inv, ShopItemDefinition{GrantType: shopGrantTypeSeed, GrantItemID: "seed_rose", Quantity: 5}, 15); err != nil {
		t.Fatal(err)
	}
	if len(inv.Seeds) != 1 || inv.Seeds[0].ItemID != "seed_rose" || inv.Seeds[0].Quantity != 15 {
		t.Fatalf("unexpected seeds %#v", inv.Seeds)
	}
	if err := grantShopItemQuantity(&inv, ShopItemDefinition{GrantType: shopGrantTypeSeed, GrantItemID: "seed_rose", Quantity: 5}, 0); err == nil {
		t.Fatal("expected zero quantity to be rejected")
	}
}

func TestSpendPlayerCurrency(t *testing.T) {
	resources := PlayerResources{Coin: 100, Gem: 10, Level: 1, UnlockedCloudLayers: 1}
	if err := SpendPlayerCurrency(&resources, shopCurrencyCoin, 40); err != nil {
		t.Fatal(err)
	}
	if resources.Coin != 60 {
		t.Fatalf("got coin %d", resources.Coin)
	}
	if err := SpendPlayerCurrency(&resources, shopCurrencyGem, 20); err == nil {
		t.Fatal("expected not enough gem")
	}
}
