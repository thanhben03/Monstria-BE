package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/heroiclabs/nakama-common/runtime"
	"strings"
)

type PetDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rarity      string `json:"rarity"`
}
type PetCatalog struct {
	Pets []PetDefinition `json:"pets"`
}

//go:embed pet_catalog.json
var defaultPetCatalog []byte
var petCatalog, petCatalogError = parsePetCatalog(defaultPetCatalog)

func parsePetCatalog(raw []byte) (PetCatalog, error) {
	var catalog PetCatalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return catalog, err
	}
	if len(catalog.Pets) == 0 {
		return catalog, fmt.Errorf("pet catalog is empty")
	}
	seen := map[string]bool{}
	for _, pet := range catalog.Pets {
		if !strings.HasPrefix(pet.ID, "pet_") || strings.TrimSpace(pet.ID) != pet.ID || len(pet.ID) <= 4 || seen[pet.ID] || strings.TrimSpace(pet.Name) == "" || strings.TrimSpace(pet.Description) == "" {
			return catalog, fmt.Errorf("invalid or duplicate pet definition: %q", pet.ID)
		}
		if pet.Rarity != "R" && pet.Rarity != "SR" && pet.Rarity != "SSR" {
			return catalog, fmt.Errorf("invalid pet rarity: %q", pet.Rarity)
		}
		seen[pet.ID] = true
	}
	return catalog, nil
}

// Storage is authoritative after the first boot; bundled JSON seeds new deployments.
func bootstrapPetCatalog(ctx context.Context, nk runtime.NakamaModule) error {
	if petCatalogError != nil {
		return petCatalogError
	}
	objects, err := nk.StorageRead(ctx, []*runtime.StorageRead{{Collection: "catalog", Key: "pet_catalog", UserID: ""}})
	if err != nil {
		return err
	}
	if len(objects) > 0 {
		catalog, err := parsePetCatalog([]byte(objects[0].Value))
		if err != nil {
			return err
		}
		petCatalog = catalog
		return nil
	}
	_, err = nk.StorageWrite(ctx, []*runtime.StorageWrite{{Collection: "catalog", Key: "pet_catalog", UserID: "", Value: string(defaultPetCatalog), Version: "*", PermissionRead: 2, PermissionWrite: 0}})
	return err
}

func GetPetCatalogRPC(ctx context.Context, _ runtime.Logger, _ *sql.DB, _ runtime.NakamaModule, _ string) (string, error) {
	if _, err := petUserID(ctx); err != nil {
		return "", err
	}
	raw, err := json.Marshal(petCatalog)
	return string(raw), err
}
