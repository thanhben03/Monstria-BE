package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/heroiclabs/nakama-common/runtime"
)

const shopCatalogFileName = "shop_catalog.json"

const (
	shopCatalogStorageCollection = "catalog"
	shopCatalogStorageKey        = "shop_catalog"
	shopCatalogStorageUserID     = ""
)

const (
	shopCurrencyCoin = "coin"
	shopCurrencyGem  = "gem"

	shopGrantTypePot  = "pot"
	shopGrantTypeSeed = "seed"
	shopGrantTypeItem = "item"
)

type ShopItemDefinition struct {
	ShopItemID    string `json:"shopItemId"`
	GrantType     string `json:"grantType"`
	GrantItemID   string `json:"grantItemId"`
	Quantity      int    `json:"quantity"`
	CoinPrice     int    `json:"coinPrice,omitempty"`
	GemPrice      int    `json:"gemPrice,omitempty"`
	RequiredLevel int    `json:"requiredLevel,omitempty"`
	Disabled      bool   `json:"disabled,omitempty"`

	GrowthTime      string   `json:"growthTime,omitempty"`
	HarvestQuantity int      `json:"harvestQuantity,omitempty"`
	WaterNeed       string   `json:"waterNeed,omitempty"`
	ExpReward       int      `json:"expReward,omitempty"`
	AttractedBugs   []string `json:"attractedBugs,omitempty"`
	Desc            string   `json:"desc,omitempty"`
	GoldPerHour     int      `json:"goldPerHour,omitempty"`
}

type shopCatalogConfig struct {
	Items json.RawMessage `json:"items"`
}

type shopCatalogResponse struct {
	Items      map[string][]ShopItemDefinition `json:"items"`
	Pagination shopCatalogPagination           `json:"pagination"`
}

type shopCatalogPagination struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	Total      int    `json:"total"`
	TotalPages int    `json:"totalPages"`
	HasNext    bool   `json:"hasNext"`
	Category   string `json:"category,omitempty"`
}

var shopItemDefinitions []ShopItemDefinition
var shopItemDefinitionByID = buildShopItemDefinitionByID(shopItemDefinitions)

var errShopCatalogStorageNotFound = errors.New("shop catalog storage not found")

func init() {
	_ = loadShopItemDefinitions()
}

func loadShopItemDefinitions() error {
	path, err := resolveShopCatalogPath()
	if err != nil {
		return err
	}
	return loadShopItemDefinitionsFromFile(path)
}

func loadShopItemDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) error {
	defs, err := readShopItemDefinitionsFromStorage(ctx, nk)
	if err != nil {
		return err
	}
	setShopItemDefinitions(defs)
	return nil
}

func bootstrapShopItemDefinitionsStorage(ctx context.Context, nk runtime.NakamaModule) error {
	if err := loadShopItemDefinitionsFromStorage(ctx, nk); err == nil {
		return nil
	} else if !errors.Is(err, errShopCatalogStorageNotFound) {
		return err
	}

	path, err := resolveShopCatalogPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read shop catalog %q: %w", path, err)
	}
	if _, err := parseShopItemDefinitions(raw); err != nil {
		return fmt.Errorf("parse shop catalog %q: %w", path, err)
	}

	if _, err := nk.StorageWrite(ctx, []*runtime.StorageWrite{
		{
			Collection:      shopCatalogStorageCollection,
			Key:             shopCatalogStorageKey,
			UserID:          shopCatalogStorageUserID,
			Value:           string(raw),
			Version:         "",
			PermissionRead:  2,
			PermissionWrite: 0,
		},
	}); err != nil {
		return fmt.Errorf("write shop catalog storage: %w", err)
	}

	return loadShopItemDefinitionsFromStorage(ctx, nk)
}

func readShopItemDefinitionsFromStorage(ctx context.Context, nk runtime.NakamaModule) ([]ShopItemDefinition, error) {
	objs, err := nk.StorageRead(ctx, []*runtime.StorageRead{
		{
			Collection: shopCatalogStorageCollection,
			Key:        shopCatalogStorageKey,
			UserID:     shopCatalogStorageUserID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("read shop catalog storage: %w", err)
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("%w: %s/%s", errShopCatalogStorageNotFound, shopCatalogStorageCollection, shopCatalogStorageKey)
	}

	defs, err := parseShopItemDefinitions([]byte(objs[0].GetValue()))
	if err != nil {
		return nil, fmt.Errorf("parse shop catalog storage: %w", err)
	}
	return defs, nil
}

func resolveShopCatalogPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("SHOP_CATALOG_PATH")); path != "" {
		return path, nil
	}

	candidates := []string{
		shopCatalogFileName,
		filepath.Join("modules", shopCatalogFileName),
		filepath.Join("/nakama/data/modules", shopCatalogFileName),
		filepath.Join("/nakama/data", shopCatalogFileName),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", shopCatalogFileName)
}

func loadShopItemDefinitionsFromFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read shop catalog %q: %w", path, err)
	}
	defs, err := parseShopItemDefinitions(raw)
	if err != nil {
		return fmt.Errorf("parse shop catalog %q: %w", path, err)
	}
	setShopItemDefinitions(defs)
	return nil
}

func parseShopItemDefinitions(raw []byte) ([]ShopItemDefinition, error) {
	var config shopCatalogConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		var defs []ShopItemDefinition
		if arrayErr := json.Unmarshal(raw, &defs); arrayErr != nil {
			return nil, err
		}
		return validateShopItemDefinitions(defs)
	}
	return parseShopItemsRaw(config.Items)
}

func parseShopItemsRaw(raw json.RawMessage) ([]ShopItemDefinition, error) {
	if len(raw) == 0 {
		return nil, errors.New("items is required")
	}

	var defs []ShopItemDefinition
	if err := json.Unmarshal(raw, &defs); err == nil {
		return validateShopItemDefinitions(defs)
	}

	var byCategory map[string][]ShopItemDefinition
	if err := json.Unmarshal(raw, &byCategory); err != nil {
		return nil, err
	}

	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	for _, category := range categories {
		defs = append(defs, byCategory[category]...)
	}
	return validateShopItemDefinitions(defs)
}

func validateShopItemDefinitions(defs []ShopItemDefinition) ([]ShopItemDefinition, error) {
	if len(defs) == 0 {
		return nil, errors.New("items must contain at least one definition")
	}

	out := make([]ShopItemDefinition, 0, len(defs))
	seenIDs := make(map[string]struct{}, len(defs))
	for i, def := range defs {
		def.ShopItemID = strings.TrimSpace(def.ShopItemID)
		def.GrantType = normalizeShopGrantType(def.GrantType)
		def.GrantItemID = strings.TrimSpace(def.GrantItemID)
		def.GrowthTime = strings.TrimSpace(def.GrowthTime)
		def.WaterNeed = strings.TrimSpace(def.WaterNeed)
		def.AttractedBugs = normalizeShopTextList(def.AttractedBugs)
		def.Desc = strings.TrimSpace(def.Desc)

		if def.ShopItemID == "" {
			return nil, fmt.Errorf("items[%d].shopItemId is required", i)
		}
		if _, ok := seenIDs[def.ShopItemID]; ok {
			return nil, fmt.Errorf("duplicate shopItemId %q", def.ShopItemID)
		}
		if def.GrantType == "" {
			return nil, fmt.Errorf("items[%d].grantType is invalid", i)
		}
		if def.GrantItemID == "" {
			return nil, fmt.Errorf("items[%d].grantItemId is required", i)
		}
		if def.Quantity < 1 {
			return nil, fmt.Errorf("items[%d].quantity must be greater than 0", i)
		}
		if def.CoinPrice < 1 && def.GemPrice < 1 {
			return nil, fmt.Errorf("items[%d].coinPrice or gemPrice must be greater than 0", i)
		}
		if def.RequiredLevel < 0 {
			return nil, fmt.Errorf("items[%d].requiredLevel cannot be negative", i)
		}
		if def.CoinPrice < 0 || def.GemPrice < 0 {
			return nil, fmt.Errorf("items[%d].coinPrice and gemPrice cannot be negative", i)
		}
		if def.HarvestQuantity < 0 {
			return nil, fmt.Errorf("items[%d].harvestQuantity cannot be negative", i)
		}
		if def.ExpReward < 0 {
			return nil, fmt.Errorf("items[%d].expReward cannot be negative", i)
		}
		if def.GoldPerHour < 0 {
			return nil, fmt.Errorf("items[%d].goldPerHour cannot be negative", i)
		}

		seenIDs[def.ShopItemID] = struct{}{}
		out = append(out, def)
	}
	return out, nil
}

func normalizeShopGrantType(grantType string) string {
	switch strings.TrimSpace(strings.ToLower(grantType)) {
	case shopGrantTypePot:
		return shopGrantTypePot
	case "flower", "decoration", "decor", "tool", shopGrantTypeItem:
		return shopGrantTypeItem
	case shopGrantTypeSeed:
		return shopGrantTypeSeed
	default:
		return ""
	}
}

func normalizeShopTextList(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func setShopItemDefinitions(defs []ShopItemDefinition) {
	shopItemDefinitions = append([]ShopItemDefinition(nil), defs...)
	shopItemDefinitionByID = buildShopItemDefinitionByID(shopItemDefinitions)
}

func buildShopItemDefinitionByID(defs []ShopItemDefinition) map[string]ShopItemDefinition {
	out := make(map[string]ShopItemDefinition, len(defs))
	for _, def := range defs {
		id := strings.TrimSpace(def.ShopItemID)
		if id == "" {
			continue
		}
		def.ShopItemID = id
		def.GrantType = normalizeShopGrantType(def.GrantType)
		def.GrantItemID = strings.TrimSpace(def.GrantItemID)
		def.GrowthTime = strings.TrimSpace(def.GrowthTime)
		def.WaterNeed = strings.TrimSpace(def.WaterNeed)
		def.AttractedBugs = normalizeShopTextList(def.AttractedBugs)
		def.Desc = strings.TrimSpace(def.Desc)
		out[id] = def
	}
	return out
}

func listShopItemDefinitions() []ShopItemDefinition {
	return sortShopItemDefinitions(shopItemDefinitions)
}

func sortShopItemDefinitions(defs []ShopItemDefinition) []ShopItemDefinition {
	out := append([]ShopItemDefinition(nil), defs...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ShopItemID < out[j].ShopItemID
	})
	return out
}

func listShopItemDefinitionsByCategory() map[string][]ShopItemDefinition {
	return groupShopItemDefinitionsByCategory(listShopItemDefinitions())
}

func groupShopItemDefinitionsByCategory(defs []ShopItemDefinition) map[string][]ShopItemDefinition {
	out := make(map[string][]ShopItemDefinition)
	for _, def := range defs {
		category := shopCategoryForGrantType(def.GrantType)
		out[category] = append(out[category], def)
	}
	return out
}

func listShopItemDefinitionsForCategory(category string) []ShopItemDefinition {
	return listShopItemDefinitionsForCategoryFrom(shopItemDefinitions, category)
}

func listShopItemDefinitionsForCategoryFrom(defs []ShopItemDefinition, category string) []ShopItemDefinition {
	category = strings.TrimSpace(strings.ToLower(category))
	defs = sortShopItemDefinitions(defs)
	if category == "" {
		return defs
	}

	byCategory := groupShopItemDefinitionsByCategory(defs)
	return append([]ShopItemDefinition(nil), byCategory[category]...)
}

func paginateShopItemDefinitions(defs []ShopItemDefinition, page int, pageSize int) ([]ShopItemDefinition, shopCatalogPagination) {
	const defaultPageSize = 12
	const maxPageSize = 12

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	total := len(defs)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	start := (page - 1) * pageSize
	if start >= total {
		return []ShopItemDefinition{}, shopCatalogPagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    false,
		}
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return append([]ShopItemDefinition(nil), defs[start:end]...), shopCatalogPagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
	}
}

func shopCategoryForGrantType(grantType string) string {
	switch normalizeShopGrantType(grantType) {
	case shopGrantTypePot:
		return "pot"
	case shopGrantTypeSeed:
		return "seed"
	default:
		return "flower"
	}
}

func shopItemDefinitionForID(shopItemID string) (ShopItemDefinition, error) {
	return shopItemDefinitionForIDFrom(shopItemDefinitions, shopItemID)
}

func shopItemDefinitionForIDFrom(defs []ShopItemDefinition, shopItemID string) (ShopItemDefinition, error) {
	id := strings.TrimSpace(shopItemID)
	if id == "" {
		return ShopItemDefinition{}, runtime.NewError("shopItemId is required", 3)
	}
	def, ok := buildShopItemDefinitionByID(defs)[id]
	if !ok || def.Disabled {
		return ShopItemDefinition{}, runtime.NewError("shop item not found", 5)
	}
	return def, nil
}

func shopItemPriceForCurrency(def ShopItemDefinition, currency string) int {
	switch strings.TrimSpace(strings.ToLower(currency)) {
	case shopCurrencyCoin:
		return def.CoinPrice
	case shopCurrencyGem:
		return def.GemPrice
	}
	return 0
}
