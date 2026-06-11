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

	shopGrantTypePot   = "pot"
	shopGrantTypeSeed  = "seed"
	shopGrantTypeDecor = "decor"
	shopGrantTypeItem  = "item"
)

type ShopItemDefinition struct {
	ShopItemID    string `json:"shopItemId"`
	NameItem      string `json:"nameItem,omitempty"`
	GrantType     string `json:"grantType"`
	GrantItemID   string `json:"grantItemId"`
	Quantity      int    `json:"quantity"`
	CoinPrice     int    `json:"coinPrice,omitempty"`
	GemPrice      int    `json:"gemPrice,omitempty"`
	RequiredLevel int    `json:"requiredLevel,omitempty"`
	Disabled      bool   `json:"disabled,omitempty"`

	Summary      string                     `json:"summary,omitempty"`
	PriceByLevel []ShopPriceLevelDefinition `json:"priceByLevel,omitempty"`

	GrowthTime      string   `json:"growthTime,omitempty"`
	HarvestQuantity int      `json:"harvestQuantity,omitempty"`
	WaterNeed       string   `json:"waterNeed,omitempty"`
	ExpReward       int      `json:"expReward,omitempty"`
	AttractedBugs   []string `json:"attractedBugs,omitempty"`
	Desc            string   `json:"desc,omitempty"`
	GoldPerHour     int      `json:"goldPerHour,omitempty"`
	DetailMainText  string   `json:"detailMainText,omitempty"`
	DetailSideText  string   `json:"detailSideText,omitempty"`
	HarvestBonus    int      `json:"harvestBonus,omitempty"`
}

type ShopPriceLevelDefinition struct {
	MinLevel  int `json:"minLevel"`
	MaxLevel  int `json:"maxLevel"`
	CoinPrice int `json:"coinPrice,omitempty"`
	GemPrice  int `json:"gemPrice,omitempty"`
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
		def.NameItem = strings.TrimSpace(def.NameItem)
		def.GrantType = normalizeShopGrantType(def.GrantType)
		def.GrantItemID = strings.TrimSpace(def.GrantItemID)
		def.Summary = strings.TrimSpace(def.Summary)
		def.PriceByLevel = normalizeShopPriceLevels(def.PriceByLevel)
		def.GrowthTime = strings.TrimSpace(def.GrowthTime)
		def.WaterNeed = strings.TrimSpace(def.WaterNeed)
		def.AttractedBugs = normalizeShopTextList(def.AttractedBugs)
		def.Desc = strings.TrimSpace(def.Desc)
		def.DetailMainText = strings.TrimSpace(def.DetailMainText)
		def.DetailSideText = strings.TrimSpace(def.DetailSideText)

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
		if def.CoinPrice < 1 && def.GemPrice < 1 && !hasShopPriceLevelPrice(def.PriceByLevel) {
			return nil, fmt.Errorf("items[%d].coinPrice or gemPrice must be greater than 0", i)
		}
		if def.RequiredLevel < 0 {
			return nil, fmt.Errorf("items[%d].requiredLevel cannot be negative", i)
		}
		if def.CoinPrice < 0 || def.GemPrice < 0 {
			return nil, fmt.Errorf("items[%d].coinPrice and gemPrice cannot be negative", i)
		}
		for j, priceLevel := range def.PriceByLevel {
			if priceLevel.MinLevel < 1 {
				return nil, fmt.Errorf("items[%d].priceByLevel[%d].minLevel must be greater than 0", i, j)
			}
			if priceLevel.MaxLevel > 0 && priceLevel.MaxLevel < priceLevel.MinLevel {
				return nil, fmt.Errorf("items[%d].priceByLevel[%d].maxLevel cannot be less than minLevel", i, j)
			}
			if priceLevel.CoinPrice < 0 || priceLevel.GemPrice < 0 {
				return nil, fmt.Errorf("items[%d].priceByLevel[%d].coinPrice and gemPrice cannot be negative", i, j)
			}
			if priceLevel.CoinPrice < 1 && priceLevel.GemPrice < 1 {
				return nil, fmt.Errorf("items[%d].priceByLevel[%d].coinPrice or gemPrice must be greater than 0", i, j)
			}
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
	case shopGrantTypeSeed:
		return shopGrantTypeSeed
	case "decoration", shopGrantTypeDecor:
		return shopGrantTypeDecor
	case "flower", "tool", shopGrantTypeItem:
		return shopGrantTypeItem
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

func normalizeShopPriceLevels(in []ShopPriceLevelDefinition) []ShopPriceLevelDefinition {
	out := make([]ShopPriceLevelDefinition, 0, len(in))
	for _, priceLevel := range in {
		out = append(out, priceLevel)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MinLevel == out[j].MinLevel {
			return out[i].MaxLevel < out[j].MaxLevel
		}
		return out[i].MinLevel < out[j].MinLevel
	})
	return out
}

func hasShopPriceLevelPrice(priceLevels []ShopPriceLevelDefinition) bool {
	for _, priceLevel := range priceLevels {
		if priceLevel.CoinPrice > 0 || priceLevel.GemPrice > 0 {
			return true
		}
	}
	return false
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
		def.NameItem = strings.TrimSpace(def.NameItem)
		def.GrantType = normalizeShopGrantType(def.GrantType)
		def.GrantItemID = strings.TrimSpace(def.GrantItemID)
		def.Summary = strings.TrimSpace(def.Summary)
		def.PriceByLevel = normalizeShopPriceLevels(def.PriceByLevel)
		def.GrowthTime = strings.TrimSpace(def.GrowthTime)
		def.WaterNeed = strings.TrimSpace(def.WaterNeed)
		def.AttractedBugs = normalizeShopTextList(def.AttractedBugs)
		def.Desc = strings.TrimSpace(def.Desc)
		def.DetailMainText = strings.TrimSpace(def.DetailMainText)
		def.DetailSideText = strings.TrimSpace(def.DetailSideText)
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
	category = normalizeShopCategory(category)
	defs = sortShopItemDefinitions(defs)
	if category == "" {
		return defs
	}

	byCategory := groupShopItemDefinitionsByCategory(defs)
	return append([]ShopItemDefinition(nil), byCategory[category]...)
}

func normalizeShopCategory(category string) string {
	category = strings.TrimSpace(strings.ToLower(category))
	switch category {
	case "decoration":
		return shopGrantTypeDecor
	default:
		return category
	}
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
	case shopGrantTypeDecor:
		return "decor"
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

func shopItemDefinitionForPlayerLevel(def ShopItemDefinition, playerLevel int) ShopItemDefinition {
	if def.GrantType != shopGrantTypePot || len(def.PriceByLevel) == 0 {
		return def
	}

	level := normalizedRequiredLevel(playerLevel)
	for _, priceLevel := range def.PriceByLevel {
		if level < priceLevel.MinLevel {
			continue
		}
		if priceLevel.MaxLevel > 0 && level > priceLevel.MaxLevel {
			continue
		}
		def.CoinPrice = priceLevel.CoinPrice
		def.GemPrice = priceLevel.GemPrice
		return def
	}
	return def
}

func shopItemDefinitionWithDetailText(def ShopItemDefinition) ShopItemDefinition {
	def.DetailMainText = ""
	def.DetailSideText = ""

	switch def.GrantType {
	case shopGrantTypeSeed:
		def.DetailMainText = renderSeedShopItemMainText(def)
		def.DetailSideText = renderSeedShopItemSideText(def)
	case shopGrantTypePot:
		def.DetailMainText = strings.TrimSpace(def.Summary)
		def.DetailSideText = renderPotShopItemSideText(def)
	case shopGrantTypeDecor:
		def.DetailMainText = strings.TrimSpace(def.Summary)
		def.DetailSideText = renderDecorShopItemSideText(def)
	}
	return def
}

func renderDecorShopItemSideText(def ShopItemDefinition) string {
	lines := []string{
		fmt.Sprintf("<color=#00FF00>Mô tả: </color> %s", def.Desc),
		fmt.Sprintf("Bonus: giúp tăng <color=#FFFF00>%d</color>", def.HarvestQuantity),
	}

	return strings.Join(lines, "\n")
}

func shopItemDefinitionWithoutDetailText(def ShopItemDefinition) ShopItemDefinition {
	def.DetailMainText = ""
	def.DetailSideText = ""
	return def
}

func renderSeedShopItemMainText(def ShopItemDefinition) string {
	lines := []string{
		fmt.Sprintf("Tăng trưởng: <color=#00FF00>%s</color>", def.GrowthTime),
		fmt.Sprintf("Cấp độ: <color=#FF66FF>%d</color>", normalizedRequiredLevel(def.RequiredLevel)),
		fmt.Sprintf("Tổng thu hoạch: <color=#FFFF00>%d</color>", def.HarvestQuantity),
	}
	return strings.Join(lines, "\n")
}

func renderSeedShopItemSideText(def ShopItemDefinition) string {
	lines := []string{
		fmt.Sprintf("Cần nước: <color=#66CCFF>%s</color>", def.WaterNeed),
		fmt.Sprintf("Kinh nghiệm: <color=#FFFF00>%d</color>", def.ExpReward),
		fmt.Sprintf("Thu hút sâu: <color=#FF3333>%s</color>", strings.Join(def.AttractedBugs, ", ")),
	}
	if strings.TrimSpace(def.Desc) != "" {
		lines = append(lines, "", fmt.Sprintf("<color=#FF33CC>%s</color>", def.Desc))
	}
	return strings.Join(lines, "\n")
}

func renderPotShopItemSideText(def ShopItemDefinition) string {
	lines := make([]string, 0, len(def.PriceByLevel)+3)
	if strings.TrimSpace(def.Desc) != "" {
		lines = append(lines, strings.TrimSpace(def.Desc), "")
	}
	lines = append(lines, "Giá mua chậu theo cấp độ:")

	priceLevels := def.PriceByLevel
	if len(priceLevels) == 0 {
		priceLevels = []ShopPriceLevelDefinition{
			{
				MinLevel:  normalizedRequiredLevel(def.RequiredLevel),
				MaxLevel:  0,
				CoinPrice: def.CoinPrice,
				GemPrice:  def.GemPrice,
			},
		}
	}
	for _, priceLevel := range priceLevels {
		lines = append(lines, fmt.Sprintf(
			"%s: <color=#FFFF00>%s / %s</color>",
			formatShopLevelRange(priceLevel),
			formatShopPrice(priceLevel.CoinPrice),
			formatShopPrice(priceLevel.GemPrice),
		))
	}
	return strings.Join(lines, "\n")
}

func formatShopLevelRange(priceLevel ShopPriceLevelDefinition) string {
	if priceLevel.MaxLevel < 1 {
		return fmt.Sprintf("%d+", priceLevel.MinLevel)
	}
	return fmt.Sprintf("%d - %d", priceLevel.MinLevel, priceLevel.MaxLevel)
}

func formatShopPrice(price int) string {
	if price == 0 {
		return "0"
	}

	raw := fmt.Sprintf("%d", price)
	out := make([]byte, 0, len(raw)+len(raw)/3)
	for i, digit := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(digit))
	}
	return string(out)
}
