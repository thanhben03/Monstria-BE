package main

import (
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
	shopCurrencyCoin = "coin"
	shopCurrencyGem  = "gem"

	shopGrantTypePot  = "pot"
	shopGrantTypeSeed = "seed"
	shopGrantTypeItem = "item"
)

type ShopItemDefinition struct {
	ShopItemID    string   `json:"shopItemId"`
	GrantType     string   `json:"grantType"`
	GrantItemID   string   `json:"grantItemId"`
	Quantity      int      `json:"quantity"`
	Currency      []string `json:"currency"`
	Price         int      `json:"price"`
	RequiredLevel int      `json:"requiredLevel,omitempty"`
	Disabled      bool     `json:"disabled,omitempty"`
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
		def.Currency = normalizeShopCurrencies(def.Currency)

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
		if len(def.Currency) == 0 {
			return nil, fmt.Errorf("items[%d].currency must contain coin or gem", i)
		}
		if def.Price < 1 {
			return nil, fmt.Errorf("items[%d].price must be greater than 0", i)
		}
		if def.RequiredLevel < 0 {
			return nil, fmt.Errorf("items[%d].requiredLevel cannot be negative", i)
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

func normalizeShopCurrencies(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, currency := range in {
		switch strings.TrimSpace(strings.ToLower(currency)) {
		case shopCurrencyCoin:
			currency = shopCurrencyCoin
		case shopCurrencyGem:
			currency = shopCurrencyGem
		default:
			continue
		}
		if _, ok := seen[currency]; ok {
			continue
		}
		seen[currency] = struct{}{}
		out = append(out, currency)
	}
	sort.Strings(out)
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
		def.Currency = normalizeShopCurrencies(def.Currency)
		out[id] = def
	}
	return out
}

func listShopItemDefinitions() []ShopItemDefinition {
	out := append([]ShopItemDefinition(nil), shopItemDefinitions...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ShopItemID < out[j].ShopItemID
	})
	return out
}

func listShopItemDefinitionsByCategory() map[string][]ShopItemDefinition {
	defs := listShopItemDefinitions()
	return groupShopItemDefinitionsByCategory(defs)
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
	category = strings.TrimSpace(strings.ToLower(category))
	if category == "" {
		return listShopItemDefinitions()
	}

	byCategory := listShopItemDefinitionsByCategory()
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
	id := strings.TrimSpace(shopItemID)
	if id == "" {
		return ShopItemDefinition{}, runtime.NewError("shopItemId is required", 3)
	}
	def, ok := shopItemDefinitionByID[id]
	if !ok || def.Disabled {
		return ShopItemDefinition{}, runtime.NewError("shop item not found", 5)
	}
	return def, nil
}

func shopItemAllowsCurrency(def ShopItemDefinition, currency string) bool {
	currency = strings.TrimSpace(strings.ToLower(currency))
	for _, allowed := range def.Currency {
		if allowed == currency {
			return true
		}
	}
	return false
}
