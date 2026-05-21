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
					"currency": ["gem", "coin", "coin"],
					"price": 100,
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
	if len(def.Currency) != 2 || def.Currency[0] != shopCurrencyCoin || def.Currency[1] != shopCurrencyGem {
		t.Fatalf("unexpected currencies %#v", def.Currency)
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
				"currency": ["coin"],
				"price": 100
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
	raw := []byte(`{"items":[{"shopItemId":"bad","grantType":"seed","grantItemId":"seed_rose","quantity":0,"currency":["coin"],"price":1}]}`)
	if _, err := parseShopItemDefinitions(raw); err == nil {
		t.Fatal("expected quantity validation error")
	}
}

func TestListShopItemDefinitionsSortedCopy(t *testing.T) {
	old := listShopItemDefinitions()
	defer setShopItemDefinitions(old)

	setShopItemDefinitions([]ShopItemDefinition{
		{ShopItemID: "b", GrantType: shopGrantTypePot, GrantItemID: "pot_b", Quantity: 1, Currency: []string{shopCurrencyCoin}, Price: 1},
		{ShopItemID: "a", GrantType: shopGrantTypePot, GrantItemID: "pot_a", Quantity: 1, Currency: []string{shopCurrencyCoin}, Price: 1},
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
		{ShopItemID: "pot_wood", GrantType: shopGrantTypePot, GrantItemID: "pot_wood", Quantity: 1, Currency: []string{shopCurrencyCoin}, Price: 1},
		{ShopItemID: "seed_rose_pack", GrantType: shopGrantTypeSeed, GrantItemID: "seed_rose", Quantity: 5, Currency: []string{shopCurrencyCoin}, Price: 1},
		{ShopItemID: "flower_rose", GrantType: shopGrantTypeItem, GrantItemID: "flower_rose", Quantity: 1, Currency: []string{shopCurrencyGem}, Price: 1},
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
			Currency:    []string{shopCurrencyCoin},
			Price:       1,
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
		{ShopItemID: "pot_wood", GrantType: shopGrantTypePot, GrantItemID: "pot_wood", Quantity: 1, Currency: []string{shopCurrencyCoin}, Price: 1},
		{ShopItemID: "flower_rose", GrantType: shopGrantTypeItem, GrantItemID: "flower_rose", Quantity: 1, Currency: []string{shopCurrencyGem}, Price: 1},
	})

	got := listShopItemDefinitionsForCategory("pot")
	if len(got) != 1 || got[0].ShopItemID != "pot_wood" {
		t.Fatalf("unexpected category result %#v", got)
	}
}

func TestResolvePurchaseCurrency(t *testing.T) {
	def := ShopItemDefinition{Currency: []string{shopCurrencyCoin, shopCurrencyGem}}
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

	def.Currency = []string{shopCurrencyCoin}
	got, err = resolvePurchaseCurrency(def, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != shopCurrencyCoin {
		t.Fatalf("got %q", got)
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
