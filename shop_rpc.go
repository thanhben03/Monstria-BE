package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

type purchaseShopItemPayload struct {
	ShopItemID string `json:"shopItemId"`
	Currency   string `json:"currency"`
	Quantity   int    `json:"quantity"`
}

type getShopCatalogPayload struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Category string `json:"category"`
}

type getShopItemPayload struct {
	ShopItemID string `json:"shopItemId"`
}

type getShopItemResponse struct {
	Item ShopItemDefinition `json:"item"`
}

type shopPurchaseResult struct {
	ShopItemID       string `json:"shopItemId"`
	GrantType        string `json:"grantType"`
	GrantItemID      string `json:"grantItemId"`
	Quantity         int    `json:"quantity"`
	PurchaseQuantity int    `json:"purchaseQuantity"`
	Currency         string `json:"currency"`
	Price            int    `json:"price"`
	UnitPrice        int    `json:"unitPrice"`
}

type purchaseShopItemResponse struct {
	Resources PlayerResources    `json:"resources"`
	Inventory PlayerInventory    `json:"inventory"`
	Purchase  shopPurchaseResult `json:"purchase"`
}

func GetShopCatalogRPC(
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

	var body getShopCatalogPayload
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return "", runtime.NewError("invalid JSON payload", 3)
		}
	}

	definitions, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		logger.Error("read shop catalog storage: %v", err)
		return "", runtime.NewError("failed to load shop catalog", 13)
	}

	category := strings.TrimSpace(strings.ToLower(body.Category))
	defs := listShopItemDefinitionsForCategoryFrom(definitions, category)
	pageItems, pagination := paginateShopItemDefinitions(defs, body.Page, body.PageSize)
	resources, _, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources shop catalog: %v", err)
		return "", runtime.NewError("failed to load shop catalog", 13)
	}
	for i := range pageItems {
		pageItems[i] = shopItemDefinitionWithoutDetailText(shopItemDefinitionForPlayerLevel(pageItems[i], resources.Level))
	}
	pagination.Category = category

	raw, err := json.Marshal(shopCatalogResponse{
		Items:      groupShopItemDefinitionsByCategory(pageItems),
		Pagination: pagination,
	})
	if err != nil {
		logger.Error("marshal shop catalog: %v", err)
		return "", runtime.NewError("failed to load shop catalog", 13)
	}
	return string(raw), nil
}

func GetShopItemRPC(
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

	var body getShopItemPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	definitions, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		logger.Error("read shop item storage: %v", err)
		return "", runtime.NewError("failed to load shop item", 13)
	}

	def, err := shopItemDefinitionForIDFrom(definitions, body.ShopItemID)
	if err != nil {
		return "", err
	}
	resources, _, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources shop item detail: %v", err)
		return "", runtime.NewError("failed to load shop item", 13)
	}
	def = shopItemDefinitionForPlayerLevel(def, resources.Level)
	def = shopItemDefinitionWithDetailText(def)

	raw, err := json.Marshal(getShopItemResponse{Item: def})
	if err != nil {
		logger.Error("marshal shop item: %v", err)
		return "", runtime.NewError("failed to load shop item", 13)
	}
	return string(raw), nil
}

func PurchaseShopItemRPC(
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

	var body purchaseShopItemPayload
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return "", runtime.NewError("invalid JSON payload", 3)
	}

	definitions, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		logger.Error("read shop catalog storage purchase: %v", err)
		return "", runtime.NewError("failed to load shop catalog", 13)
	}

	def, err := shopItemDefinitionForIDFrom(definitions, body.ShopItemID)
	if err != nil {
		return "", err
	}

	purchaseQuantity, err := resolvePurchaseQuantity(body.Quantity)
	if err != nil {
		return "", err
	}

	resources, _, err := readPlayerResources(ctx, nk, userID)
	if err != nil {
		logger.Error("read resources purchase shop item: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{Collection: playerStateCollection, Key: playerInventoryKey, UserID: userID},
	})
	if err != nil {
		logger.Error("storage read purchase shop item: %v", err)
		return "", runtime.NewError("failed to load state", 13)
	}

	inv := defaultPlayerInventory()
	invVer := ""

	for _, o := range objs {
		switch o.GetKey() {
		case playerInventoryKey:
			decoded, err := decodePlayerInventory([]byte(o.GetValue()))
			if err != nil {
				return "", runtime.NewError("corrupt inventory", 13)
			}
			inv = decoded
			invVer = o.GetVersion()
		}
	}

	if requiredLevel := normalizedRequiredLevel(def.RequiredLevel); resources.Level < requiredLevel {
		return "", runtime.NewError("player level is too low", 9)
	}

	def = shopItemDefinitionForPlayerLevel(def, resources.Level)
	currency, err := resolvePurchaseCurrency(def, body.Currency)
	if err != nil {
		return "", err
	}

	unitPrice := shopItemPriceForCurrency(def, currency)
	price := unitPrice * purchaseQuantity
	walletChangeset, err := BuildPlayerCurrencySpendChangeset(currency, price)
	if err != nil {
		return "", err
	}

	invCopy := inv
	grantQuantity := def.Quantity * purchaseQuantity
	if err := grantShopItemQuantity(&invCopy, def, grantQuantity); err != nil {
		return "", err
	}

	invRaw, err := json.Marshal(invCopy)
	if err != nil {
		return "", err
	}

	_, walletResults, err := nk.MultiUpdate(ctx, nil, []*runtime.StorageWrite{
		{
			Collection:      playerStateCollection,
			Key:             playerInventoryKey,
			UserID:          userID,
			Value:           string(invRaw),
			Version:         invVer,
			PermissionRead:  1,
			PermissionWrite: 0,
		},
	}, nil, []*runtime.WalletUpdate{
		{
			UserID:    userID,
			Changeset: walletChangeset,
			Metadata: map[string]interface{}{
				"source":           "purchase_shop_item",
				"shopItemId":       def.ShopItemID,
				"grantType":        def.GrantType,
				"grantItemId":      def.GrantItemID,
				"purchaseQuantity": purchaseQuantity,
				"currency":         currency,
				"price":            price,
				"unitPrice":        unitPrice,
			},
		},
	}, true)
	if err != nil {
		if walletErr := (*runtime.WalletNegativeError)(nil); errors.As(err, &walletErr) {
			return "", runtime.NewError("not enough "+currency, 9)
		}
		logger.Error("storage write purchase shop item: %v", err)
		return "", runtime.NewError("failed to save purchase (retry)", 13)
	}

	resourcesAfter := resources
	if len(walletResults) > 0 {
		resourcesAfter = applyWalletToResources(resourcesAfter, walletResults[0].Updated)
	} else {
		resourcesAfter, _, err = readPlayerResources(ctx, nk, userID)
		if err != nil {
			logger.Error("read resources after purchase: %v", err)
			return "", runtime.NewError("failed to save purchase", 13)
		}
	}

	out := purchaseShopItemResponse{
		Resources: resourcesAfter,
		Inventory: normalizePlayerInventory(invCopy),
		Purchase: shopPurchaseResult{
			ShopItemID:       def.ShopItemID,
			GrantType:        def.GrantType,
			GrantItemID:      def.GrantItemID,
			Quantity:         grantQuantity,
			PurchaseQuantity: purchaseQuantity,
			Currency:         currency,
			Price:            price,
			UnitPrice:        unitPrice,
		},
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func resolvePurchaseCurrency(def ShopItemDefinition, requested string) (string, error) {
	currency := strings.TrimSpace(strings.ToLower(requested))
	if currency == "" {
		canUseCoin := def.CoinPrice > 0
		canUseGem := def.GemPrice > 0
		if canUseCoin && !canUseGem {
			return shopCurrencyCoin, nil
		}
		if canUseGem && !canUseCoin {
			return shopCurrencyGem, nil
		}
		return "", runtime.NewError("currency is required", 3)
	}
	if shopItemPriceForCurrency(def, currency) < 1 {
		return "", runtime.NewError("currency is not allowed for this shop item", 3)
	}
	return currency, nil
}

func resolvePurchaseQuantity(quantity int) (int, error) {
	if quantity == 0 {
		return 1, nil
	}
	if quantity < 0 {
		return 0, runtime.NewError("quantity must be positive", 3)
	}
	const maxPurchaseQuantity = 99
	if quantity > maxPurchaseQuantity {
		return 0, runtime.NewError("quantity is too large", 3)
	}
	return quantity, nil
}

func normalizedRequiredLevel(requiredLevel int) int {
	if requiredLevel < 1 {
		return 1
	}
	return requiredLevel
}

func grantShopItem(inv *PlayerInventory, def ShopItemDefinition) error {
	return grantShopItemQuantity(inv, def, def.Quantity)
}

func grantShopItemQuantity(inv *PlayerInventory, def ShopItemDefinition, quantity int) error {
	if quantity < 1 {
		return runtime.NewError("quantity must be positive", 13)
	}

	switch def.GrantType {
	case shopGrantTypePot:
		return AddPot(inv, def.GrantItemID, quantity)
	case shopGrantTypeSeed:
		return AddSeed(inv, def.GrantItemID, quantity)
	case shopGrantTypeDecor:
		return AddItem(inv, def.GrantItemID, quantity)
	case shopGrantTypeItem:
		return AddItem(inv, def.GrantItemID, quantity)
	default:
		return runtime.NewError("grantType is invalid", 13)
	}
}
